package app

import (
	"strconv"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/mede2026/cryptobulle/internal/config"
	"github.com/mede2026/cryptobulle/internal/crypto"
	"github.com/mede2026/cryptobulle/internal/hotkey"
	"github.com/mede2026/cryptobulle/internal/w32"
)

// Fenetre de reglages : phrase secrete, raccourcis, comportement, et un petit
// atelier pour chiffrer ou dechiffrer a la main.

const (
	settingsClass = "CryptoBulleSettings"

	// Taille de la zone utile, en points logiques.
	contentWidth  = 470
	contentHeight = 650
)

// Identifiants des controles.
const (
	idPassphrase = 200 + iota
	idShowPassphrase
	idHotkeyDecrypt
	idHotkeyEncrypt
	idHotkeyMask
	idCaptureDecrypt
	idCaptureEncrypt
	idCaptureMask
	idSeconds
	idAutoPaste
	idRestore
	idStartup
	idThemeDark
	idThemeLight
	idWorkshopIn
	idWorkshopOut
	idWorkshopEncrypt
	idWorkshopDecrypt
	idWorkshopCopy
	idSelfTest
	idSave
	idClose
)

// Phrase du test automatique : uniquement des minuscules et des chiffres, pour
// ne dependre d'aucune disposition clavier particuliere.
const selfTestPhrase = "bonjour ca va 123"

// timerSelfTest attend que Windows ait fini de distribuer les touches simulees.
const timerSelfTest = 1

var settingsProc = windows.NewCallback(settingsWndProc)

// control retient la place logique d'un controle, pour pouvoir le replacer
// quand la fenetre passe sur un ecran de densite differente.
type control struct {
	hwnd                w32.HWND
	x, y, width, height int32
}

type settingsWindow struct {
	app      *App
	hwnd     w32.HWND
	font     w32.HFONT
	dpi      int32
	controls []control

	passphrase, showPassphrase   w32.HWND
	hotkeyDecrypt, hotkeyEncrypt w32.HWND
	hotkeyMask                   w32.HWND
	seconds                      w32.HWND
	autoPaste, restore, startup  w32.HWND
	themeDark, themeLight        w32.HWND
	workshopIn, workshopOut      w32.HWND
	storageNote                  w32.HWND

	// capture designe le champ en attente d'une combinaison de touches.
	capture w32.HWND
}

// openSettings ouvre la fenetre de reglages, ou la ramene au premier plan.
func (a *App) openSettings() {
	if a.settings != nil && a.settings.hwnd != 0 {
		a.settings.load(a.config())
		w32.ShowWindow(a.settings.hwnd, w32.SW_SHOW)
		w32.SetForegroundWindow(a.settings.hwnd)
		return
	}
	a.settings = newSettingsWindow(a)
	a.settings.load(a.config())
	w32.ShowWindow(a.settings.hwnd, w32.SW_SHOW)
	w32.SetForegroundWindow(a.settings.hwnd)
}

func newSettingsWindow(app *App) *settingsWindow {
	instance := w32.ModuleHandle()
	class := w32.WNDCLASS{
		WndProc:    settingsProc,
		Instance:   instance,
		ClassName:  w32.Str(settingsClass),
		Cursor:     w32.LoadCursor(w32.IDC_ARROW),
		Background: w32.HBRUSH(w32.COLOR_WINDOW + 1),
		Icon:       loadIcon(32),
	}
	w32.RegisterClass(&class)

	const style = w32.WS_CAPTION | w32.WS_SYSMENU | w32.WS_MINIMIZEBOX

	settings := &settingsWindow{app: app}
	settings.hwnd = w32.CreateWindowEx(
		0, settingsClass, appName+" - réglages", style, 0, 0, 100, 100, 0, 0, instance,
	)
	// La densite depend de l'ecran ou la fenetre est apparue : on la lit apres
	// la creation, puis on donne a la fenetre sa vraie taille.
	settings.dpi = w32.DPI(settings.hwnd)
	settings.font = w32.CreateFont("Segoe UI", 9, settings.dpi, false)
	settings.resize(style)
	settings.build(instance, settings.scale)
	return settings
}

func (s *settingsWindow) scale(value int32) int32 { return value * s.dpi / 96 }

// resize donne a la fenetre la taille utile voulue, bordures comprises, et la
// centre sur la zone de travail.
func (s *settingsWindow) resize(style uint32) {
	client := w32.RECT{Right: s.scale(contentWidth), Bottom: s.scale(contentHeight)}
	w32.AdjustWindowRect(&client, style)
	area := w32.WorkArea()
	w32.SetWindowPos(s.hwnd, 0,
		area.Left+(area.Width()-client.Width())/2,
		area.Top+(area.Height()-client.Height())/3,
		client.Width(), client.Height(), w32.SWP_NOACTIVATE)
}

// relayout replace tous les controles apres un changement de densite d'ecran.
func (s *settingsWindow) relayout() {
	if s.font != 0 {
		w32.DeleteObject(uintptr(s.font))
	}
	s.font = w32.CreateFont("Segoe UI", 9, s.dpi, false)
	for _, item := range s.controls {
		w32.SendMessage(item.hwnd, w32.WM_SETFONT, uintptr(s.font), 1)
		w32.SetWindowPos(item.hwnd, 0, s.scale(item.x), s.scale(item.y),
			s.scale(item.width), s.scale(item.height), w32.SWP_NOACTIVATE)
	}
	w32.InvalidateRect(s.hwnd, true)
}

// build cree tous les controles. Les coordonnees sont en points logiques,
// converties par `scale` pour s'adapter aux ecrans haute densite.
func (s *settingsWindow) build(instance windows.Handle, scale func(int32) int32) {
	label := func(text string, x, y, width, height int32) w32.HWND {
		return s.add("STATIC", text, w32.WS_CHILD|w32.WS_VISIBLE, x, y, width, height, 0, instance, scale)
	}
	group := func(text string, y, height int32) {
		s.add("BUTTON", text, w32.WS_CHILD|w32.WS_VISIBLE|w32.BS_GROUPBOX,
			14, y, 442, height, 0, instance, scale)
	}
	button := func(text string, id uintptr, x, y, width int32) w32.HWND {
		return s.add("BUTTON", text, w32.WS_CHILD|w32.WS_VISIBLE|w32.WS_TABSTOP|w32.BS_PUSHBUTTON,
			x, y, width, 24, id, instance, scale)
	}
	check := func(text string, id uintptr, x, y, width int32) w32.HWND {
		return s.add("BUTTON", text, w32.WS_CHILD|w32.WS_VISIBLE|w32.WS_TABSTOP|w32.BS_AUTOCHECKBOX,
			x, y, width, 20, id, instance, scale)
	}
	edit := func(id uintptr, style uint32, x, y, width, height int32) w32.HWND {
		return s.add("EDIT", "", w32.WS_CHILD|w32.WS_VISIBLE|w32.WS_TABSTOP|w32.WS_BORDER|style,
			x, y, width, height, id, instance, scale)
	}

	// --- phrase secrete
	group("Phrase secrète", 8, 92)
	s.passphrase = edit(idPassphrase, w32.ES_AUTOHSCROLL|w32.ES_PASSWORD, 28, 30, 300, 22)
	s.showPassphrase = check("Afficher", idShowPassphrase, 340, 31, 100)
	s.storageNote = label("", 28, 58, 410, 16)
	label("Vos correspondants doivent avoir exactement la même phrase.", 28, 74, 410, 16)

	// --- raccourcis
	group("Raccourcis clavier", 108, 114)
	label("Déchiffrer la sélection", 28, 134, 160, 20)
	s.hotkeyDecrypt = edit(idHotkeyDecrypt, w32.ES_READONLY|w32.ES_AUTOHSCROLL, 190, 132, 130, 22)
	button("Enregistrer...", idCaptureDecrypt, 330, 131, 110)
	label("Chiffrer la sélection", 28, 164, 160, 20)
	s.hotkeyEncrypt = edit(idHotkeyEncrypt, w32.ES_READONLY|w32.ES_AUTOHSCROLL, 190, 162, 130, 22)
	button("Enregistrer...", idCaptureEncrypt, 330, 161, 110)
	label("Frappe masquée", 28, 194, 160, 20)
	s.hotkeyMask = edit(idHotkeyMask, w32.ES_READONLY|w32.ES_AUTOHSCROLL, 190, 192, 130, 22)
	button("Enregistrer...", idCaptureMask, 330, 191, 110)

	// --- comportement
	group("Comportement", 230, 106)
	label("Durée de la bulle (secondes, 0 = manuel)", 28, 254, 250, 20)
	s.seconds = edit(idSeconds, w32.ES_AUTOHSCROLL, 284, 252, 50, 22)
	s.autoPaste = check("Coller automatiquement le texte chiffré", idAutoPaste, 28, 278, 260)
	s.restore = check("Remettre l'ancien presse-papiers ensuite", idRestore, 28, 300, 260)
	s.startup = check("Lancer au démarrage de Windows", idStartup, 28, 322, 260)
	s.themeDark = s.add("BUTTON", "Bulle sombre",
		w32.WS_CHILD|w32.WS_VISIBLE|w32.WS_GROUP|w32.BS_AUTORADIOBUTTON, 300, 278, 130, 20,
		idThemeDark, instance, scale)
	s.themeLight = s.add("BUTTON", "Bulle claire",
		w32.WS_CHILD|w32.WS_VISIBLE|w32.BS_AUTORADIOBUTTON, 300, 300, 130, 20,
		idThemeLight, instance, scale)

	// --- atelier
	group("Atelier", 344, 210)
	label("Texte à chiffrer, ou message chiffré à relire :", 28, 364, 400, 18)
	s.workshopIn = edit(idWorkshopIn, w32.ES_MULTILINE|w32.ES_WANTRETURN|w32.WS_VSCROLL, 28, 382, 410, 60)
	button("Chiffrer", idWorkshopEncrypt, 28, 448, 90)
	button("Déchiffrer", idWorkshopDecrypt, 124, 448, 90)
	button("Copier", idWorkshopCopy, 220, 448, 70)
	button("Tester la frappe masquée", idSelfTest, 296, 448, 142)
	s.workshopOut = edit(idWorkshopOut, w32.ES_MULTILINE|w32.ES_READONLY|w32.WS_VSCROLL, 28, 478, 410, 62)

	// --- aide et boutons finaux
	label("Frappe masquée : tout ce que vous tapez s'affiche chiffré, en direct. "+
		"Échap ou le même raccourci pour en sortir.", 16, 558, 440, 32)
	label(appName+" "+appVersion+" - AES-256-GCM, clé dérivée par scrypt.", 16, 590, 440, 18)
	button("Enregistrer", idSave, 240, 614, 105)
	button("Fermer", idClose, 351, 614, 105)
}

// add cree un controle enfant et lui donne la police de l'interface.
func (s *settingsWindow) add(class, text string, style uint32, x, y, width, height int32, id uintptr, instance windows.Handle, scale func(int32) int32) w32.HWND {
	created := w32.CreateWindowEx(0, class, text, style,
		scale(x), scale(y), scale(width), scale(height),
		s.hwnd, w32.HMENU(id), instance)
	w32.SendMessage(created, w32.WM_SETFONT, uintptr(s.font), 1)
	s.controls = append(s.controls, control{created, x, y, width, height})
	return created
}

// load remplit les controles a partir des reglages.
func (s *settingsWindow) load(cfg config.Config) {
	w32.SetWindowText(s.passphrase, cfg.Passphrase())
	w32.SetWindowText(s.hotkeyDecrypt, hotkey.Pretty(cfg.HotkeyDecrypt))
	w32.SetWindowText(s.hotkeyEncrypt, hotkey.Pretty(cfg.HotkeyEncrypt))
	w32.SetWindowText(s.hotkeyMask, hotkey.Pretty(cfg.HotkeyMask))
	w32.SetWindowText(s.seconds, strconv.Itoa(cfg.BubbleSeconds))
	w32.SetChecked(s.autoPaste, cfg.AutoPaste)
	w32.SetChecked(s.restore, cfg.RestoreClipboard)
	w32.SetChecked(s.startup, startupEnabled())
	w32.SetChecked(s.themeDark, cfg.Theme != "clair")
	w32.SetChecked(s.themeLight, cfg.Theme == "clair")
	w32.SetChecked(s.showPassphrase, false)
	w32.SendMessage(s.passphrase, w32.EM_SETPASSWORDCHAR, uintptr('•'), 0)

	note := "Attention : sans protection du système, la phrase est simplement encodée."
	if config.SecureStorage() {
		note = "Phrase secrète protégée par Windows (DPAPI)."
	}
	w32.SetWindowText(s.storageNote, note)
}

// save relit les controles, verifie tout, puis enregistre.
func (s *settingsWindow) save() {
	updated := s.app.config()

	passphrase := w32.WindowText(s.passphrase)
	if passphrase == "" {
		s.alert("La phrase secrète est obligatoire.", w32.MB_ICONERROR)
		return
	}
	decryptCombo, err := hotkey.Normalize(w32.WindowText(s.hotkeyDecrypt))
	if err != nil {
		s.alert(capitalize(err.Error()), w32.MB_ICONERROR)
		return
	}
	encryptCombo, err := hotkey.Normalize(w32.WindowText(s.hotkeyEncrypt))
	if err != nil {
		s.alert(capitalize(err.Error()), w32.MB_ICONERROR)
		return
	}
	maskCombo, err := hotkey.Normalize(w32.WindowText(s.hotkeyMask))
	if err != nil {
		s.alert(capitalize(err.Error()), w32.MB_ICONERROR)
		return
	}
	if decryptCombo == encryptCombo || decryptCombo == maskCombo || encryptCombo == maskCombo {
		s.alert("Les trois raccourcis doivent être différents.", w32.MB_ICONERROR)
		return
	}
	if updated.HasPassphrase() && passphrase != updated.Passphrase() {
		answer := w32.MessageBox(s.hwnd,
			"Changer la phrase secrète rendra illisibles les messages déjà chiffrés "+
				"avec l'ancienne.\n\nContinuer ?", appName, w32.MB_YESNO|w32.MB_ICONERROR)
		if answer != w32.IDYES {
			return
		}
	}

	seconds, err := strconv.Atoi(strings.TrimSpace(w32.WindowText(s.seconds)))
	if err != nil || seconds < 0 {
		s.alert("La durée doit être un nombre de secondes (0 ou plus).", w32.MB_ICONERROR)
		return
	}

	updated.SetPassphrase(passphrase)
	updated.HotkeyDecrypt = decryptCombo
	updated.HotkeyEncrypt = encryptCombo
	updated.HotkeyMask = maskCombo
	updated.BubbleSeconds = seconds
	updated.AutoPaste = w32.Checked(s.autoPaste)
	updated.RestoreClipboard = w32.Checked(s.restore)
	updated.Theme = "sombre"
	if w32.Checked(s.themeLight) {
		updated.Theme = "clair"
	}
	updated.LaunchAtStartup = setStartup(w32.Checked(s.startup))
	w32.SetChecked(s.startup, updated.LaunchAtStartup)

	s.app.setConfig(updated)
	if err := updated.Save(); err != nil {
		s.alert("Enregistrement impossible : "+err.Error(), w32.MB_ICONERROR)
		return
	}
	if message := s.app.applyHotkeys(); message != "" {
		s.alert(message, w32.MB_ICONERROR)
		return
	}
	go warmUp(updated.Passphrase(), updated.Salt())
	s.alert("Réglages enregistrés.", w32.MB_ICONINFORMATION)
}

// startSelfTest verifie la frappe masquee de bout en bout : chiffrement,
// interception du clavier, injection des caracteres, puis relecture.
//
// Les touches sont simulees sans notre signature : le hook les traite donc
// comme de vraies frappes.
func (s *settingsWindow) startSelfTest() {
	passphrase := w32.WindowText(s.passphrase)
	if passphrase == "" {
		s.report("La phrase secrète est obligatoire pour lancer le test.")
		return
	}

	// 1. Le chiffrement lui-même, sans rien demander à Windows.
	stream, err := crypto.NewStream(passphrase)
	if err != nil {
		s.report("Chiffrement interne : ÉCHEC (" + err.Error() + ")")
		return
	}
	attendu := stream.Marker()
	for _, letter := range selfTestPhrase {
		attendu += stream.Mask(letter)
	}
	relu, err := crypto.DecryptText(attendu, passphrase)
	if err != nil || relu != selfTestPhrase {
		s.report("Chiffrement interne : ÉCHEC\r\nrelu : " + relu + "\r\n" + errorText(err))
		return
	}

	// Deuxieme verification : deux morceaux tapes dans deux champs differents,
	// donc deux en-tetes, colles bout a bout.
	deuxChamps := attendu + attendu
	if relu, err := crypto.DecryptText(deuxChamps, passphrase); err != nil ||
		relu != selfTestPhrase+selfTestPhrase {
		s.report("Changement de champ : ÉCHEC\r\nrelu : " + relu + "\r\n" + errorText(err))
		return
	}

	// 2. L'interception du clavier, dans le champ de l'atelier.
	w32.SetWindowText(s.workshopIn, "")
	w32.SetWindowText(s.workshopOut, "Test en cours...")
	w32.SetFocus(s.workshopIn)

	s.app.mask.startWith(passphrase)
	if !s.app.mask.active {
		s.report("Chiffrement interne : OK\r\n" +
			"Interception du clavier : ÉCHEC, Windows a refusé le hook.")
		return
	}
	w32.SendUserKeys(selfTestPhrase)
	w32.SetTimer(s.hwnd, timerSelfTest, 600)
}

// finishSelfTest lit ce qui est arrivé dans le champ et rend son verdict.
func (s *settingsWindow) finishSelfTest() {
	w32.KillTimer(s.hwnd, timerSelfTest)
	s.app.mask.stop()

	recu := w32.WindowText(s.workshopIn)
	passphrase := w32.WindowText(s.passphrase)
	relu, err := crypto.DecryptText(recu, passphrase)

	lignes := []string{
		"Chiffrement interne : OK",
		"Changement de champ : OK",
		"Interception du clavier : OK",
		"Disposition clavier : " + w32.KeyboardLayoutName(),
		"Tapé      : " + selfTestPhrase,
		"Affiché   : " + recu,
		"Attendu   : " + strconv.Itoa(len([]rune(selfTestPhrase))+crypto.StreamHeaderChars) +
			" caractères, reçu " + strconv.Itoa(len([]rune(recu))),
	}
	switch {
	case relu == selfTestPhrase:
		lignes = append(lignes, "Relecture : OK", "", "Tout fonctionne.")
	case recu == "":
		lignes = append(lignes,
			"Relecture : ÉCHEC",
			"",
			"Rien n'est arrivé dans le champ. Les touches ont été avalées mais",
			"jamais réinjectées, ou une autre application les a interceptées avant.")
	case recu == selfTestPhrase:
		lignes = append(lignes,
			"Relecture : ÉCHEC",
			"",
			"Le texte est passé en clair : le hook n'a pas avalé les touches.",
			"Un antivirus ou une application privilégiée peut le bloquer.")
	default:
		lignes = append(lignes,
			"Relecture : ÉCHEC ("+errorText(err)+")",
			"Relu      : "+relu,
			"",
			"Les caractères sont arrivés, mais pas dans le bon ordre ou pas tous.",
			"Copiez ce rapport pour le transmettre.")
	}
	s.report(strings.Join(lignes, "\r\n"))
	w32.SetFocus(s.hwnd)
}

// report affiche le rapport de test dans le champ de résultat.
func (s *settingsWindow) report(text string) {
	w32.SetWindowText(s.workshopOut, text)
}

func errorText(err error) string {
	if err == nil {
		return "aucune erreur signalée"
	}
	return err.Error()
}

func (s *settingsWindow) alert(message string, icon uint32) {
	w32.MessageBox(s.hwnd, message, appName, w32.MB_OK|icon)
}

// startCapture met un champ de raccourci en attente d'une combinaison.
func (s *settingsWindow) startCapture(field w32.HWND) {
	s.capture = field
	w32.SetWindowText(field, "Appuyez sur les touches...")
	w32.SetFocus(s.hwnd) // le clavier arrive alors a la fenetre, pas au controle
}

// finishCapture transforme la touche recue en raccourci lisible.
func (s *settingsWindow) finishCapture(key uint32) {
	switch key {
	case w32.VK_CONTROL, w32.VK_MENU, w32.VK_SHIFT, w32.VK_LWIN, w32.VK_RWIN:
		return // on attend la touche principale
	}

	var modifiers uint32
	if w32.KeyIsDown(w32.VK_CONTROL) {
		modifiers |= hotkey.ModControl
	}
	if w32.KeyIsDown(w32.VK_MENU) {
		modifiers |= hotkey.ModAlt
	}
	if w32.KeyIsDown(w32.VK_SHIFT) {
		modifiers |= hotkey.ModShift
	}
	if w32.KeyIsDown(w32.VK_LWIN) || w32.KeyIsDown(w32.VK_RWIN) {
		modifiers |= hotkey.ModWin
	}

	field := s.capture
	s.capture = 0
	combo := hotkey.Combo{Modifiers: modifiers, Key: key}
	if _, err := hotkey.Parse(combo.String()); err != nil {
		w32.SetWindowText(field, "")
		s.alert(capitalize(err.Error()), w32.MB_ICONERROR)
		s.load(s.app.config())
		return
	}
	w32.SetWindowText(field, combo.Pretty())
}

func (s *settingsWindow) workshop(encrypting bool) {
	cfg := s.app.config()
	source := strings.TrimSpace(w32.WindowText(s.workshopIn))
	passphrase := w32.WindowText(s.passphrase)

	var result string
	var err error
	if encrypting {
		result, err = crypto.Encrypt(source, passphrase, cfg.Salt())
	} else {
		result, err = crypto.DecryptText(source, passphrase)
	}
	if err != nil {
		result = "[erreur] " + err.Error()
	}
	w32.SetWindowText(s.workshopOut, strings.ReplaceAll(result, "\n", "\r\n"))
}

func settingsWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	app := currentApp
	if app == nil || app.settings == nil {
		return w32.DefWindowProc(w32.HWND(hwnd), uint32(message), wparam, lparam)
	}
	s := app.settings

	switch uint32(message) {
	case w32.WM_CTLCOLORSTATIC, w32.WM_CTLCOLORBTN:
		// Sans cela, Windows peint le fond des libelles, des cadres et des cases
		// a cocher en gris, ce qui laisserait des rectangles ternes sur la
		// fenetre blanche. On reprend les couleurs du theme du systeme.
		w32.SetBkColor(w32.HDC(wparam), w32.SysColor(w32.COLOR_WINDOW))
		w32.SetTextColor(w32.HDC(wparam), w32.SysColor(w32.COLOR_WINDOWTEXT))
		return uintptr(w32.SysColorBrush(w32.COLOR_WINDOW))

	case w32.WM_COMMAND:
		s.command(uint32(wparam) & 0xFFFF)
		return 0

	case w32.WM_KEYDOWN:
		if s.capture != 0 {
			s.finishCapture(uint32(wparam))
		}
		return 0

	case w32.WM_TIMER:
		if wparam == timerSelfTest {
			s.finishSelfTest()
		}
		return 0

	case w32.WM_DPICHANGED:
		s.dpi = int32(wparam & 0xFFFF)
		suggested := w32.RectAt(lparam)
		w32.SetWindowPos(s.hwnd, 0, suggested.Left, suggested.Top,
			suggested.Width(), suggested.Height(), w32.SWP_NOACTIVATE)
		s.relayout()
		return 0

	case w32.WM_CLOSE:
		s.capture = 0
		w32.ShowWindow(s.hwnd, w32.SW_HIDE)
		return 0
	}
	return w32.DefWindowProc(w32.HWND(hwnd), uint32(message), wparam, lparam)
}

func (s *settingsWindow) command(id uint32) {
	switch id {
	case idShowPassphrase:
		character := uintptr('•')
		if w32.Checked(s.showPassphrase) {
			character = 0
		}
		w32.SendMessage(s.passphrase, w32.EM_SETPASSWORDCHAR, character, 0)
		w32.InvalidateRect(s.passphrase, true)
	case idCaptureDecrypt:
		s.startCapture(s.hotkeyDecrypt)
	case idCaptureEncrypt:
		s.startCapture(s.hotkeyEncrypt)
	case idCaptureMask:
		s.startCapture(s.hotkeyMask)
	case idWorkshopEncrypt:
		s.workshop(true)
	case idWorkshopDecrypt:
		s.workshop(false)
	case idSelfTest:
		s.startSelfTest()
	case idWorkshopCopy:
		_ = setClipboardText(strings.TrimSpace(w32.WindowText(s.workshopOut)))
	case idSave:
		s.save()
	case idClose:
		w32.ShowWindow(s.hwnd, w32.SW_HIDE)
	}
}
