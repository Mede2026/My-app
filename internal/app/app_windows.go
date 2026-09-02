package app

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/mede2026/cryptobulle/internal/config"
	"github.com/mede2026/cryptobulle/internal/crypto"
	"github.com/mede2026/cryptobulle/internal/hotkey"
	"github.com/mede2026/cryptobulle/internal/w32"
)

const (
	serviceClass = "CryptoBulleService"
	mutexName    = "Global\\CryptoBulle-single-instance"

	trayIconID = 1

	hotkeyDecrypt = 1
	hotkeyEncrypt = 2

	menuSettings = 1
	menuEncrypt  = 2
	menuDecrypt  = 3
	menuQuit     = 4
)

// App tient l'etat de l'application. Une seule instance vit par processus.
type App struct {
	mu  sync.RWMutex
	cfg config.Config

	hwnd     w32.HWND
	trayIcon w32.HICON
	dpi      int32

	bubble   *bubbleWindow
	settings *settingsWindow

	tasks chan func()
	busy  atomic.Bool
}

var (
	currentApp  *App
	serviceProc = windows.NewCallback(serviceWndProc)
)

// Run demarre CryptoBulle et ne rend la main qu'a la fermeture.
func Run() error {
	// La boucle de messages doit rester sur le meme fil systeme du debut a la fin.
	runtime.LockOSThread()

	if !claimSingleInstance() {
		w32.MessageBox(0, appName+" est deja lance (icone pres de l'horloge).",
			appName, w32.MB_OK|w32.MB_ICONINFORMATION)
		return nil
	}
	// Le manifeste demande deja les controles modernes et la gestion fine de la
	// densite d'ecran ; ces deux appels servent de filet si le manifeste manque.
	w32.EnableDPIAwareness()
	w32.InitCommonControls()

	app := &App{cfg: config.Load(), tasks: make(chan func(), 64)}
	currentApp = app

	// Le sel personnel est cree au premier lancement, puis conserve.
	if app.cfg.KeySalt == "" {
		app.cfg.Salt()
		_ = app.cfg.Save()
	}

	if err := app.createServiceWindow(); err != nil {
		return err
	}
	app.dpi = w32.DPI(app.hwnd)
	app.trayIcon = loadIcon(w32.GetSystemMetrics(w32.SM_CXSMICON))
	app.addTrayIcon()
	app.bubble = newBubbleWindow(app)

	if message := app.applyHotkeys(); message != "" {
		w32.MessageBox(0, message+"\n\nChoisissez d'autres raccourcis dans les reglages.",
			appName, w32.MB_OK|w32.MB_ICONERROR)
	}

	cfg := app.config()
	go warmUp(cfg.Passphrase(), cfg.Salt())

	if !cfg.HasPassphrase() {
		app.openSettings()
	} else {
		app.bubble.show(appName+" est actif",
			hotkey.Pretty(cfg.HotkeyEncrypt)+" : chiffrer la selection\n"+
				hotkey.Pretty(cfg.HotkeyDecrypt)+" : dechiffrer la selection",
			kindInfo, 6)
	}

	app.loop()
	return nil
}

// claimSingleInstance renvoie faux si une autre copie tourne deja.
func claimSingleInstance() bool {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return true
	}
	_, err = windows.CreateMutex(nil, false, name)
	return err != windows.ERROR_ALREADY_EXISTS
}

// warmUp calcule la cle a l'avance : le premier raccourci n'attend pas scrypt.
func warmUp(passphrase string, salt []byte) {
	if passphrase == "" {
		return
	}
	_, _ = crypto.DeriveKey(passphrase, salt)
}

// --- reglages partages entre fils -------------------------------------------

func (a *App) config() config.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

func (a *App) setConfig(updated config.Config) {
	a.mu.Lock()
	a.cfg = updated
	a.mu.Unlock()
}

// post fait executer une fonction par le fil de l'interface.
func (a *App) post(task func()) {
	select {
	case a.tasks <- task:
		w32.PostMessage(a.hwnd, w32.WM_TASK, 0, 0)
	default: // file pleine : on prefere perdre la tache plutot que bloquer
	}
}

func (a *App) runTasks() {
	for {
		select {
		case task := <-a.tasks:
			task()
		default:
			return
		}
	}
}

// trigger lance une action en arriere-plan, une seule a la fois.
func (a *App) trigger(action func()) {
	if !a.busy.CompareAndSwap(false, true) {
		return // une action est deja en cours : on ignore le second appui
	}
	go func() {
		defer a.busy.Store(false)
		action()
	}()
}

// --- fenetre de service ------------------------------------------------------

func (a *App) createServiceWindow() error {
	instance := w32.ModuleHandle()
	class := w32.WNDCLASS{
		WndProc:   serviceProc,
		Instance:  instance,
		ClassName: w32.Str(serviceClass),
	}
	w32.RegisterClass(&class)

	// Fenetre jamais affichee : elle sert de boite aux lettres pour les
	// raccourcis, l'icone de notification et les taches de fond.
	a.hwnd = w32.CreateWindowEx(0, serviceClass, appName, 0, 0, 0, 0, 0, 0, 0, instance)
	if a.hwnd == 0 {
		return windows.GetLastError()
	}
	return nil
}

func serviceWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	app := currentApp
	if app == nil {
		return w32.DefWindowProc(w32.HWND(hwnd), uint32(message), wparam, lparam)
	}

	switch uint32(message) {
	case w32.WM_TASK:
		app.runTasks()
		return 0

	case w32.WM_HOTKEY:
		switch wparam {
		case hotkeyDecrypt:
			app.trigger(app.actionDecrypt)
		case hotkeyEncrypt:
			app.trigger(app.actionEncrypt)
		}
		return 0

	case w32.WM_TRAY:
		switch uint32(lparam) & 0xFFFF {
		case w32.WM_RBUTTONUP, w32.WM_LBUTTONUP:
			app.showTrayMenu()
		case w32.WM_LBUTTONDBLCLK:
			app.openSettings()
		}
		return 0

	case w32.WM_DESTROY:
		app.removeTrayIcon()
		w32.PostQuitMessage(0)
		return 0
	}
	return w32.DefWindowProc(w32.HWND(hwnd), uint32(message), wparam, lparam)
}

func (a *App) loop() {
	var message w32.MSG
	for w32.GetMessage(&message) > 0 {
		w32.TranslateMessage(&message)
		w32.DispatchMessage(&message)
	}
}

func (a *App) quit() {
	w32.UnregisterHotKey(a.hwnd, hotkeyDecrypt)
	w32.UnregisterHotKey(a.hwnd, hotkeyEncrypt)
	w32.DestroyWindow(a.hwnd)
}

// --- icone pres de l'horloge --------------------------------------------------

func (a *App) trayData() w32.NOTIFYICONDATA {
	data := w32.NOTIFYICONDATA{Wnd: a.hwnd, ID: trayIconID}
	data.Size = uint32(unsafe.Sizeof(data))
	return data
}

func (a *App) addTrayIcon() {
	data := a.trayData()
	data.Flags = w32.NIF_ICON | w32.NIF_MESSAGE | w32.NIF_TIP
	data.CallbackMessage = w32.WM_TRAY
	data.Icon = a.trayIcon
	copy(data.Tip[:], windows.StringToUTF16(appName+" "+appVersion))
	w32.ShellNotifyIcon(w32.NIM_ADD, &data)
}

func (a *App) removeTrayIcon() {
	data := a.trayData()
	w32.ShellNotifyIcon(w32.NIM_DELETE, &data)
}

func (a *App) showTrayMenu() {
	menu := w32.CreatePopupMenu()
	w32.AppendMenu(menu, w32.MF_STRING, menuSettings, "Reglages...")
	w32.AppendMenu(menu, w32.MF_SEPARATOR, 0, "")
	w32.AppendMenu(menu, w32.MF_STRING, menuEncrypt, "Chiffrer la selection")
	w32.AppendMenu(menu, w32.MF_STRING, menuDecrypt, "Dechiffrer la selection")
	w32.AppendMenu(menu, w32.MF_SEPARATOR, 0, "")
	w32.AppendMenu(menu, w32.MF_STRING, menuQuit, "Quitter")

	point := w32.CursorPos()
	// Sans cet avant-plan, le menu resterait affiche apres un clic ailleurs.
	w32.SetForegroundWindow(a.hwnd)
	choice := w32.TrackPopupMenu(menu, w32.TPM_RIGHTBUTTON|w32.TPM_RETURNCMD, point.X, point.Y, a.hwnd)
	w32.DestroyMenu(menu)
	w32.PostMessage(a.hwnd, 0, 0, 0) // WM_NULL : referme proprement le menu

	switch choice {
	case menuSettings:
		a.openSettings()
	case menuEncrypt:
		a.trigger(a.actionEncrypt)
	case menuDecrypt:
		a.trigger(a.actionDecrypt)
	case menuQuit:
		a.quit()
	}
}

// --- raccourcis ----------------------------------------------------------------

// applyHotkeys (re)declare les deux raccourcis. Renvoie un message d'erreur
// lisible, ou "" si tout va bien.
func (a *App) applyHotkeys() string {
	cfg := a.config()
	w32.UnregisterHotKey(a.hwnd, hotkeyDecrypt)
	w32.UnregisterHotKey(a.hwnd, hotkeyEncrypt)

	for _, entry := range []struct {
		id    int32
		combo string
	}{
		{hotkeyDecrypt, cfg.HotkeyDecrypt},
		{hotkeyEncrypt, cfg.HotkeyEncrypt},
	} {
		parsed, err := hotkey.Parse(entry.combo)
		if err != nil {
			return err.Error()
		}
		if !w32.RegisterHotKey(a.hwnd, entry.id, parsed.Modifiers|hotkey.ModNoRepeat, parsed.Key) {
			return "Windows refuse « " + parsed.Pretty() + " » : ce raccourci est deja pris " +
				"par un autre logiciel."
		}
	}
	return ""
}

// --- actions --------------------------------------------------------------------

// selectedText renvoie le texte selectionne (ou, a defaut, le presse-papiers)
// ainsi que l'ancien contenu du presse-papiers.
func selectedText() (string, string) {
	selection, previous := readSelection()
	if selection == "" {
		selection = previous
	}
	return selection, previous
}

func (a *App) actionDecrypt() {
	cfg := a.config()
	if !a.requirePassphrase(cfg) {
		return
	}

	text, previous := selectedText()
	if cfg.RestoreClipboard && previous != "" {
		_ = setClipboardText(previous)
	}

	token := crypto.FindToken(text)
	if token == "" {
		a.showBubble("Rien a dechiffrer",
			"Aucun message CryptoBulle dans la selection.\nSelectionnez un texte qui commence par MC1~.",
			kindError, -1)
		return
	}
	plaintext, err := crypto.Decrypt(token, cfg.Passphrase())
	if err != nil {
		a.showBubble("Dechiffrement impossible", capitalize(err.Error()), kindError, -1)
		return
	}
	a.showBubble("Texte dechiffre", plaintext, kindSuccess, -1)
}

func (a *App) actionEncrypt() {
	cfg := a.config()
	if !a.requirePassphrase(cfg) {
		return
	}

	text, previous := selectedText()
	text = strings.TrimSpace(text)
	if text == "" {
		a.showBubble("Rien a chiffrer",
			"Selectionnez d'abord du texte, puis refaites le raccourci.", kindError, -1)
		return
	}
	if crypto.LooksEncrypted(text) {
		a.showBubble("Deja chiffre", "Ce texte est deja un message CryptoBulle.", kindError, -1)
		return
	}

	token, err := crypto.Encrypt(text, cfg.Passphrase(), cfg.Salt())
	if err != nil {
		a.showBubble("Chiffrement impossible", capitalize(err.Error()), kindError, -1)
		return
	}

	restore := ""
	if cfg.RestoreClipboard {
		restore = previous
	}
	message := "Le texte chiffre est dans le presse-papiers (Ctrl+V pour le coller)."
	if cfg.AutoPaste {
		err = pasteText(token, restore)
		message = "Le texte chiffre a remplace votre selection."
	} else {
		err = setClipboardText(token)
	}
	if err != nil {
		a.showBubble("Erreur", capitalize(err.Error()), kindError, -1)
		return
	}
	a.showBubble("Texte chiffre", message, kindSuccess, 4)
}

func (a *App) requirePassphrase(cfg config.Config) bool {
	if cfg.HasPassphrase() {
		return true
	}
	a.showBubble("Phrase secrete manquante",
		"Ouvrez les reglages pour choisir votre phrase secrete.", kindError, -1)
	a.post(a.openSettings)
	return false
}

// showBubble affiche une bulle depuis n'importe quel fil d'execution.
// Une duree negative signifie « celle des reglages ».
func (a *App) showBubble(title, body, kind string, seconds int) {
	if seconds < 0 {
		seconds = a.config().BubbleSeconds
	}
	a.post(func() { a.bubble.show(title, body, kind, seconds) })
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}
