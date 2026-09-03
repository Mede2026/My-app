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
	app        *App
	active     bool
	hook       uintptr
	stream     *crypto.Stream
	passphrase string

	// Endroit ou la frappe atterrissait au dernier caractere, et demande de
	// repartir a zero. Des que l'utilisateur change de champ, de fenetre ou
	// deplace le curseur, un nouvel en-tete doit etre pose : sans lui, ce qui
	// suit serait impossible a relire.
	place        w32.TypingPlace
	forceRestart bool
	// Entree vient d'etre pressee : reste a savoir si elle a envoye le message,
	// ce qui vide le champ, ou seulement saute une ligne.
	enterPending bool

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
	m.passphrase = passphrase
	m.pendingHigh = 0
	// L'en-tete n'est pas ecrit tout de suite : il partirait dans le champ ou se
	// trouve le curseur au moment du raccourci, qui n'est pas forcement celui ou
	// l'utilisateur va ecrire. Il sera pose avec le premier caractere tape.
	m.forceRestart = true
	m.place = w32.TypingTarget()
	m.enterPending = false
	w32.MouseClickedSince() // remet le compteur de clics a zero
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
		case w32.VK_LEFT, w32.VK_RIGHT, w32.VK_UP, w32.VK_DOWN,
			w32.VK_HOME, w32.VK_END, w32.VK_PRIOR, w32.VK_NEXT, w32.VK_TAB:
			// Le curseur bouge : la suite ne prolonge plus le texte precedent.
			m.forceRestart = true
			return passThrough(app, code, wparam, lparam)
		case w32.VK_RETURN:
			// La touche traverse telle quelle. Selon l'application, elle saute
			// une ligne ou envoie le message : la difference se lit au prochain
			// caractere, a la position du curseur.
			m.enterPending = true
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
		w32.SendString(m.header() + m.stream.Mask(letter))
		m.swallowed[event.VkCode] = true
		return 1
	}
	return passThrough(app, code, wparam, lparam)
}

// header rend l'en-tete a ecrire avant le prochain caractere, ou "" si la
// frappe continue au meme endroit.
//
// C'est ce qui repare le cas le plus courant : ecrire dans un champ, passer a un
// autre, et continuer a taper. Sans nouvel en-tete, le second champ serait
// impossible a relire.
func (m *maskTyping) header() string {
	place := w32.TypingTarget()
	moved := place.Windows != m.place.Windows || w32.MouseClickedSince() || m.forceRestart

	if !moved && m.enterPending {
		// Apres Entree, deux cas. Dans un traitement de texte, le curseur est
		// descendu d'une ligne : le texte continue. Dans une messagerie, le
		// message est parti et le champ est vide : il faut repartir a zero.
		// Les applications qui ne publient pas leur curseur, comme celles
		// batties sur un navigateur, sont traitees comme des messageries : une
		// reprise inutile ne coute que quelques caracteres, alors qu'une reprise
		// oubliee rendrait le texte illisible.
		moved = !place.HasCaret || !m.place.HasCaret || place.Caret.Top <= m.place.Caret.Top
	}

	m.place, m.forceRestart, m.enterPending = place, false, false
	if !moved {
		return ""
	}

	stream, err := crypto.NewStream(m.passphrase)
	if err != nil {
		return "" // on continue avec la suite en cours plutot que de tout perdre
	}
	m.stream = stream
	return stream.Marker()
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
