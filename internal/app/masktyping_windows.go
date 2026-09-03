package app

import (
	"golang.org/x/sys/windows"

	"github.com/mede2026/cryptobulle/internal/crypto"
	"github.com/mede2026/cryptobulle/internal/w32"
)

// Frappe masquee : tant que le mode est actif, chaque touche tapee est avalee
// et remplacee a l'ecran par un caractere chiffre. Personne ne peut lire ce qui
// est ecrit, meme en regardant l'ecran pendant la frappe.
//
// Le texte produit se relit avec le raccourci de dechiffrement habituel.
//
// Rien n'est enregistre : la touche est traduite puis oubliee. Le hook clavier
// n'existe que pendant que le mode est allume, et l'icone pres de l'horloge
// change de couleur pour que l'etat soit toujours visible.

var maskProc = windows.NewCallback(maskHookProc)

type maskTyping struct {
	app    *App
	active bool
	hook   uintptr
	stream *crypto.Stream

	// Touches dont l'appui a ete avale : leur relachement doit l'etre aussi.
	swallowed map[uint32]bool
}

func newMaskTyping(app *App) *maskTyping {
	return &maskTyping{app: app, swallowed: map[uint32]bool{}}
}

// toggle allume ou eteint le mode. Appele depuis le fil de l'interface.
func (m *maskTyping) toggle() {
	if m.active {
		m.stop()
		return
	}
	m.start()
}

func (m *maskTyping) start() {
	cfg := m.app.config()
	if !cfg.HasPassphrase() {
		m.app.showBubble("Phrase secrete manquante",
			"Ouvrez les reglages pour choisir votre phrase secrete.", kindError, -1)
		m.app.post(m.app.openSettings)
		return
	}

	stream, err := crypto.NewStream(cfg.Passphrase())
	if err != nil {
		m.app.showBubble("Frappe masquee impossible", capitalize(err.Error()), kindError, -1)
		return
	}

	// Le marqueur part avant l'installation du hook : il indique ou commence le
	// texte masque, et permet de le relire plus tard.
	w32.SendString(stream.Marker())

	hook := w32.SetKeyboardHook(maskProc)
	if hook == 0 {
		m.app.showBubble("Frappe masquee impossible",
			"Windows a refuse l'interception du clavier.", kindError, -1)
		return
	}
	m.hook, m.stream, m.active = hook, stream, true
	clear(m.swallowed)
	m.app.setTrayState(true)
}

func (m *maskTyping) stop() {
	if !m.active {
		return
	}
	w32.RemoveKeyboardHook(m.hook)
	m.hook, m.stream, m.active = 0, nil, false
	clear(m.swallowed)
	m.app.setTrayState(false)
}

// newLine termine la ligne courante et en commence une nouvelle, avec son
// propre marqueur : chaque ligne se relit ainsi toute seule.
func (m *maskTyping) newLine() {
	w32.SendKey(w32.VK_RETURN)
	cfg := m.app.config()
	stream, err := crypto.NewStream(cfg.Passphrase())
	if err != nil {
		m.stop()
		return
	}
	m.stream = stream
	w32.SendString(stream.Marker())
}

// maskHookProc recoit chaque touche avant l'application au premier plan.
//
// Renvoyer 1 avale la touche ; passer la main a CallNextHook la laisse suivre
// son chemin normal. Le traitement doit rester tres court, sans quoi Windows
// desactive le hook.
func maskHookProc(code int, wparam, lparam uintptr) uintptr {
	app := currentApp
	if app == nil || app.mask == nil || !app.mask.active || code < 0 {
		return passThrough(app, code, wparam, lparam)
	}
	m := app.mask
	event := w32.KeyEventAt(lparam)

	if event.IsInjected() { // nos propres frappes simulees
		return passThrough(app, code, wparam, lparam)
	}

	switch uint32(wparam) {
	case w32.WM_KEYUP, w32.WM_SYSKEYUP:
		if m.swallowed[event.VkCode] {
			delete(m.swallowed, event.VkCode)
			return 1 // l'appui a ete avale : le relachement doit l'etre aussi
		}
		return passThrough(app, code, wparam, lparam)

	case w32.WM_KEYDOWN, w32.WM_SYSKEYDOWN:
		// Les raccourcis (Ctrl+S, Alt+Tab, touche Windows) doivent continuer a
		// fonctionner. AltGr fait exception : c'est une touche d'ecriture.
		altGr := w32.KeyIsDown(w32.VK_RMENU)
		if (w32.KeyIsDown(w32.VK_CONTROL) && !altGr) ||
			w32.KeyIsDown(w32.VK_LMENU) ||
			w32.KeyIsDown(w32.VK_LWIN) || w32.KeyIsDown(w32.VK_RWIN) {
			return passThrough(app, code, wparam, lparam)
		}

		switch event.VkCode {
		case w32.VK_ESCAPE: // sortie de secours
			m.stop()
			m.swallowed[event.VkCode] = true
			return 1
		case w32.VK_BACK:
			m.stream.Rewind() // le caractere efface libere sa place
			return passThrough(app, code, wparam, lparam)
		case w32.VK_RETURN:
			m.newLine()
			m.swallowed[event.VkCode] = true
			return 1
		}

		letter, ok := w32.CharFromKey(event.VkCode, event.ScanCode)
		if !ok {
			return passThrough(app, code, wparam, lparam)
		}
		masked, changed := m.stream.Mask(letter)
		if !changed { // caractere hors alphabet : laisse tel quel
			return passThrough(app, code, wparam, lparam)
		}
		w32.SendRune(masked)
		m.swallowed[event.VkCode] = true
		return 1
	}
	return passThrough(app, code, wparam, lparam)
}

func passThrough(app *App, code int, wparam, lparam uintptr) uintptr {
	var hook uintptr
	if app != nil && app.mask != nil {
		hook = app.mask.hook
	}
	return w32.CallNextHook(hook, code, wparam, lparam)
}
