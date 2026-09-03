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

	// Premiere moitie d'un caractere transmis en deux morceaux, comme le sont
	// les emojis. On attend la seconde pour reconstituer le caractere entier.
	pendingHigh rune
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
		m.app.showBubble("Phrase secrète manquante",
			"Ouvrez les réglages pour choisir votre phrase secrète.", kindError, -1)
		m.app.post(m.app.openSettings)
		return
	}
	m.startWith(cfg.Passphrase())
}

// startWith allume le mode avec une phrase secrete donnee. Le test automatique
// des reglages passe par ici.
func (m *maskTyping) startWith(passphrase string) {
	stream, err := crypto.NewStream(passphrase)
	if err != nil {
		m.app.showBubble("Frappe masquée impossible", capitalize(err.Error()), kindError, -1)
		return
	}

	// Le marqueur part avant l'installation du hook : il indique ou commence le
	// texte masque, et permet de le relire plus tard.
	w32.SendString(stream.Marker())

	hook := w32.SetKeyboardHook(maskProc)
	if hook == 0 {
		m.app.showBubble("Frappe masquée impossible",
			"Windows a refusé l'interception du clavier.", kindError, -1)
		return
	}
	m.hook, m.stream, m.active = hook, stream, true
	m.pendingHigh = 0
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

	if event.FromUs() { // nos propres frappes simulees
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
			// Le retour a la ligne traverse tel quel et ne coupe pas le texte :
			// l'en-tete n'est ecrit qu'une fois, au debut de la frappe.
			return passThrough(app, code, wparam, lparam)
		}

		letter, ok := m.letterFor(event)
		if !ok {
			// Soit la touche ne produit pas de caractere (fleches, touches
			// mortes), soit c'est la premiere moitie d'un emoji : dans ce
			// dernier cas la touche a deja ete retenue et doit etre avalee.
			if m.pendingHigh != 0 {
				m.swallowed[event.VkCode] = true
				return 1
			}
			return passThrough(app, code, wparam, lparam)
		}
		w32.SendString(m.stream.Mask(letter))
		m.swallowed[event.VkCode] = true
		return 1
	}
	return passThrough(app, code, wparam, lparam)
}

// letterFor traduit une touche en caractere complet.
//
// Les emojis arrivent en deux morceaux, sous forme de touches fabriquees par
// Windows : on garde le premier de cote et on reconstitue le caractere a
// l'arrivee du second.
func (m *maskTyping) letterFor(event w32.KBDLLHOOKSTRUCT) (rune, bool) {
	letter, ok := rune(0), false
	if event.VkCode == w32.VK_PACKET {
		letter, ok = rune(event.ScanCode), true
	} else {
		letter, ok = w32.CharFromKey(event.VkCode, event.ScanCode)
	}
	if !ok {
		return 0, false
	}

	switch {
	case letter >= 0xD800 && letter <= 0xDBFF: // premiere moitie
		m.pendingHigh = letter
		return 0, false
	case letter >= 0xDC00 && letter <= 0xDFFF: // seconde moitie
		if m.pendingHigh == 0 {
			return 0, false
		}
		complete := 0x10000 + (m.pendingHigh-0xD800)<<10 + (letter - 0xDC00)
		m.pendingHigh = 0
		return complete, true
	}
	m.pendingHigh = 0
	return letter, true
}

func passThrough(app *App, code int, wparam, lparam uintptr) uintptr {
	var hook uintptr
	if app != nil && app.mask != nil {
		hook = app.mask.hook
	}
	return w32.CallNextHook(hook, code, wparam, lparam)
}
