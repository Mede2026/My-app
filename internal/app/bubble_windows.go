package app

import (
	"strings"

	"golang.org/x/sys/windows"

	"github.com/mede2026/cryptobulle/internal/w32"
)

// La bulle est une petite fenetre sans bordure, toujours au-dessus des autres,
// qui apparait pres du curseur. Elle est construite une seule fois puis
// reutilisee : le deuxieme affichage est donc immediat.
//
// Tout est dessine a la main : fond, bande de couleur, boutons aux coins
// arrondis et bords lisses. Les coins de la fenetre elle-meme sont arrondis par
// Windows 11 ; sur Windows 10, ils restent droits, ce qui correspond au style
// du systeme.

const (
	bubbleClass = "CryptoBulleBubble"

	kindInfo    = "info"
	kindSuccess = "success"
	kindError   = "error"

	timerClose = 1
	timerFade  = 2

	bubbleBodyID = 100
)

// Dimensions en points logiques, converties selon la densite de l'ecran.
const (
	bubbleWidth   = 420
	accentBar     = 4
	bubblePadding = 14
	headerHeight  = 30
	footerHeight  = 38
	bodyMinHeight = 22
	bodyMaxHeight = 260
	buttonWidth   = 74
	buttonHeight  = 26
	buttonRadius  = 5
	crossSize     = 20
)

// Elements survolables de la bulle.
const (
	hoverNone = iota
	hoverCopy
	hoverClose
	hoverCross
)

type palette struct {
	background, text, title, muted, button, buttonText uintptr
	white                                              uintptr
}

var themes = map[string]palette{
	"sombre": {
		background: w32.RGB(0x1c, 0x1f, 0x26),
		text:       w32.RGB(0xe6, 0xe9, 0xef),
		title:      w32.RGB(0xff, 0xff, 0xff),
		muted:      w32.RGB(0x98, 0xa2, 0xb3),
		button:     w32.RGB(0x2a, 0x2f, 0x3a),
		buttonText: w32.RGB(0xe6, 0xe9, 0xef),
		white:      w32.RGB(0xff, 0xff, 0xff),
	},
	"clair": {
		background: w32.RGB(0xff, 0xff, 0xff),
		text:       w32.RGB(0x1f, 0x29, 0x37),
		title:      w32.RGB(0x11, 0x18, 0x27),
		muted:      w32.RGB(0x6b, 0x72, 0x80),
		button:     w32.RGB(0xef, 0xf2, 0xf7),
		buttonText: w32.RGB(0x11, 0x18, 0x27),
		white:      w32.RGB(0xff, 0xff, 0xff),
	},
}

var accents = map[string]uintptr{
	kindInfo:    w32.RGB(0x3f, 0x8c, 0xff),
	kindSuccess: w32.RGB(0x22, 0xc5, 0x5e),
	kindError:   w32.RGB(0xef, 0x44, 0x44),
}

var bubbleProc = windows.NewCallback(bubbleWndProc)

type bubbleWindow struct {
	app  *App
	hwnd w32.HWND
	body w32.HWND // champ de texte en lecture seule

	title, text string
	kind        string
	seconds     int
	hint        string

	dpi        int32
	theme      string
	palette    palette
	accent     uintptr
	background w32.HBRUSH
	titleFont  w32.HFONT
	bodyFont   w32.HFONT
	smallFont  w32.HFONT

	copyRect, closeRect, crossRect w32.RECT
	alpha                          int32
	hovered                        int
	mouseInside                    bool
}

func newBubbleWindow(app *App) *bubbleWindow {
	instance := w32.ModuleHandle()
	class := w32.WNDCLASS{
		Style:     w32.CS_DROPSHADOW, // legere ombre portee, comme les menus
		WndProc:   bubbleProc,
		Instance:  instance,
		ClassName: w32.Str(bubbleClass),
		Cursor:    w32.LoadCursor(w32.IDC_ARROW),
	}
	w32.RegisterClass(&class)

	bubble := &bubbleWindow{app: app, kind: kindInfo}
	bubble.hwnd = w32.CreateWindowEx(
		w32.WS_EX_TOPMOST|w32.WS_EX_TOOLWINDOW|w32.WS_EX_LAYERED,
		bubbleClass, appName, w32.WS_POPUP,
		0, 0, 10, 10, 0, 0, instance,
	)
	w32.RoundWindowCorners(bubble.hwnd)
	bubble.dpi = w32.DPI(bubble.hwnd)

	// Le corps du message est un champ de saisie en lecture seule : on obtient
	// gratuitement la selection a la souris et le defilement a la molette.
	bubble.body = w32.CreateWindowEx(
		0, "EDIT", "",
		w32.WS_CHILD|w32.WS_VISIBLE|w32.ES_MULTILINE|w32.ES_READONLY|0x0040, /* ES_AUTOVSCROLL */
		0, 0, 10, 10, bubble.hwnd, w32.HMENU(bubbleBodyID), instance,
	)

	bubble.applyTheme(app.config().Theme)
	bubble.makeFonts()
	return bubble
}

func (b *bubbleWindow) scale(value int32) int32 { return value * b.dpi / 96 }

func (b *bubbleWindow) applyTheme(theme string) {
	chosen, ok := themes[theme]
	if !ok {
		chosen = themes["sombre"]
		theme = "sombre"
	}
	if b.background != 0 {
		w32.DeleteObject(uintptr(b.background))
	}
	b.theme = theme
	b.palette = chosen
	b.background = w32.CreateSolidBrush(chosen.background)
}

// makeFonts (re)cree les polices a la taille de l'ecran courant.
func (b *bubbleWindow) makeFonts() {
	for _, font := range []w32.HFONT{b.titleFont, b.bodyFont, b.smallFont} {
		if font != 0 {
			w32.DeleteObject(uintptr(font))
		}
	}
	b.titleFont = w32.CreateFont("Segoe UI", 10, b.dpi, true)
	b.bodyFont = w32.CreateFont("Segoe UI", 10, b.dpi, false)
	b.smallFont = w32.CreateFont("Segoe UI", 9, b.dpi, false)
	w32.SendMessage(b.body, w32.WM_SETFONT, uintptr(b.bodyFont), 1)
}

// show affiche la bulle avec un nouveau contenu.
func (b *bubbleWindow) show(title, text, kind string, seconds int) {
	if theme := b.app.config().Theme; theme != b.theme {
		b.applyTheme(theme)
	}
	b.title, b.text, b.kind, b.seconds = title, text, kind, seconds
	b.accent = accents[kind]
	if b.accent == 0 {
		b.accent = accents[kindInfo]
	}
	b.hint = "Échap pour fermer"
	b.hovered = hoverNone
	b.mouseInside = false

	w32.SetWindowText(b.body, strings.ReplaceAll(text, "\n", "\r\n"))
	b.layout()

	b.alpha = 0
	w32.SetLayeredWindowAlpha(b.hwnd, 0)
	w32.ShowWindow(b.hwnd, w32.SW_SHOW)
	w32.SetForegroundWindow(b.hwnd)
	w32.SetFocus(b.hwnd) // le focus reste a la fenetre : Echap fonctionne
	w32.InvalidateRect(b.hwnd, false)

	w32.SetTimer(b.hwnd, timerFade, 10)
	b.startCloseTimer()
}

// layout calcule la taille de la bulle et la place pres du curseur.
func (b *bubbleWindow) layout() {
	width := b.scale(bubbleWidth)
	left := b.scale(accentBar) + b.scale(bubblePadding)
	right := width - b.scale(bubblePadding)
	inner := right - left

	bodyHeight := b.measureBody(inner)
	if maximum := b.scale(bodyMaxHeight); bodyHeight > maximum {
		bodyHeight = maximum
		b.hint = "Molette pour faire défiler"
	}
	if minimum := b.scale(bodyMinHeight); bodyHeight < minimum {
		bodyHeight = minimum
	}
	height := b.scale(headerHeight) + bodyHeight + b.scale(footerHeight)

	// Placement : sous le curseur, ou au-dessus s'il n'y a pas la place.
	cursor := w32.CursorPos()
	area := w32.WorkArea()
	x := cursor.X + b.scale(16)
	if x+width > area.Right-b.scale(8) {
		x = area.Right - width - b.scale(8)
	}
	if x < area.Left+b.scale(8) {
		x = area.Left + b.scale(8)
	}
	y := cursor.Y + b.scale(20)
	if y+height > area.Bottom-b.scale(8) {
		y = cursor.Y - height - b.scale(12)
	}
	if y < area.Top+b.scale(8) {
		y = area.Top + b.scale(8)
	}
	w32.SetWindowPos(b.hwnd, w32.HWND_TOPMOST, x, y, width, height, w32.SWP_NOACTIVATE)

	top := b.scale(headerHeight)
	w32.SetWindowPos(b.body, 0, left, top, inner, bodyHeight, w32.SWP_NOACTIVATE)

	// Boutons du bas, alignes a droite.
	buttonTop := top + bodyHeight + b.scale(7)
	b.closeRect = w32.RECT{
		Left: right - b.scale(buttonWidth), Top: buttonTop,
		Right: right, Bottom: buttonTop + b.scale(buttonHeight),
	}
	b.copyRect = w32.RECT{
		Left: b.closeRect.Left - b.scale(buttonWidth+8), Top: buttonTop,
		Right: b.closeRect.Left - b.scale(8), Bottom: buttonTop + b.scale(buttonHeight),
	}
	cross := b.scale(crossSize)
	b.crossRect = w32.RECT{
		Left: right - cross, Top: b.scale(6),
		Right: right, Bottom: b.scale(6) + cross,
	}
}

// measureBody demande a Windows la hauteur du texte pour une largeur donnee.
func (b *bubbleWindow) measureBody(width int32) int32 {
	dc := w32.GetDC(b.hwnd)
	defer w32.ReleaseDC(b.hwnd, dc)

	previous := w32.SelectObject(dc, uintptr(b.bodyFont))
	rect := w32.RECT{Right: width}
	w32.DrawText(dc, b.text, &rect, w32.DT_CALCRECT|w32.DT_WORDBREAK|w32.DT_NOPREFIX|w32.DT_EDITCONTROL)
	w32.SelectObject(dc, previous)
	return rect.Height() + b.scale(6)
}

func (b *bubbleWindow) startCloseTimer() {
	w32.KillTimer(b.hwnd, timerClose)
	if b.seconds > 0 && !b.mouseInside {
		w32.SetTimer(b.hwnd, timerClose, uint32(b.seconds*1000))
	}
}

func (b *bubbleWindow) hide() {
	w32.KillTimer(b.hwnd, timerClose)
	w32.KillTimer(b.hwnd, timerFade)
	w32.ShowWindow(b.hwnd, w32.SW_HIDE)
	b.mouseInside = false
	b.hovered = hoverNone
}

func (b *bubbleWindow) paint() {
	var paint w32.PAINTSTRUCT
	dc := w32.BeginPaint(b.hwnd, &paint)
	defer w32.EndPaint(b.hwnd, &paint)

	client := w32.ClientRect(b.hwnd)
	background := b.palette.background

	// Fond de la carte.
	w32.FillRect(dc, client, b.background)

	// Bande verticale coloree a gauche : elle indique le type de message.
	bar := w32.RECT{Left: 0, Top: 0, Right: b.scale(accentBar), Bottom: client.Bottom}
	accentBrush := w32.CreateSolidBrush(b.accent)
	w32.FillRect(dc, bar, accentBrush)
	w32.DeleteObject(uintptr(accentBrush))

	// Filet d'un pixel sur les trois autres cotes, a peine visible.
	edge := w32.CreateSolidBrush(w32.Blend(background, b.palette.muted, 0.35))
	thickness := int32(1)
	w32.FillRect(dc, w32.RECT{Left: bar.Right, Top: 0, Right: client.Right, Bottom: thickness}, edge)
	w32.FillRect(dc, w32.RECT{Left: client.Right - thickness, Top: 0, Right: client.Right, Bottom: client.Bottom}, edge)
	w32.FillRect(dc, w32.RECT{Left: bar.Right, Top: client.Bottom - thickness, Right: client.Right, Bottom: client.Bottom}, edge)
	w32.DeleteObject(uintptr(edge))

	w32.SetBkTransparent(dc)
	left := b.scale(accentBar) + b.scale(bubblePadding)

	// Titre.
	previous := w32.SelectObject(dc, uintptr(b.titleFont))
	w32.SetTextColor(dc, b.palette.title)
	titleRect := w32.RECT{
		Left: left, Top: 0, Right: b.crossRect.Left - b.scale(4), Bottom: b.scale(headerHeight),
	}
	w32.DrawText(dc, b.title, &titleRect, w32.DT_LEFT|w32.DT_VCENTER|w32.DT_SINGLELINE|w32.DT_NOPREFIX)

	// Croix de fermeture, plus claire au survol.
	w32.SelectObject(dc, uintptr(b.smallFont))
	crossColor := b.palette.muted
	if b.hovered == hoverCross {
		crossColor = b.palette.title
	}
	w32.SetTextColor(dc, crossColor)
	cross := b.crossRect
	w32.DrawText(dc, "✕", &cross, w32.DT_CENTER|w32.DT_VCENTER|w32.DT_SINGLELINE)

	// Note discrete en bas a gauche.
	w32.SetTextColor(dc, b.palette.muted)
	hint := w32.RECT{
		Left: left, Top: b.copyRect.Top, Right: b.copyRect.Left, Bottom: b.copyRect.Bottom,
	}
	w32.DrawText(dc, b.hint, &hint, w32.DT_LEFT|w32.DT_VCENTER|w32.DT_SINGLELINE|w32.DT_NOPREFIX)

	// Boutons : « Copier » est l'action principale, dans la couleur du message.
	b.paintButton(dc, b.copyRect, "Copier", b.accent, b.palette.white, b.hovered == hoverCopy)
	b.paintButton(dc, b.closeRect, "Fermer", b.palette.button, b.palette.buttonText, b.hovered == hoverClose)
	w32.SelectObject(dc, previous)
}

func (b *bubbleWindow) paintButton(dc w32.HDC, rect w32.RECT, label string, fill, textColor uintptr, hovered bool) {
	if hovered {
		fill = w32.Blend(fill, b.palette.white, 0.16) // legerement eclairci
	}
	w32.FillRoundRect(dc, rect, b.scale(buttonRadius), fill, b.palette.background)

	w32.SelectObject(dc, uintptr(b.smallFont))
	w32.SetTextColor(dc, textColor)
	box := rect
	w32.DrawText(dc, label, &box, w32.DT_CENTER|w32.DT_VCENTER|w32.DT_SINGLELINE|w32.DT_NOPREFIX)
}

func inRect(rect w32.RECT, x, y int32) bool {
	return x >= rect.Left && x < rect.Right && y >= rect.Top && y < rect.Bottom
}

// hoverAt indique quel element se trouve sous le curseur.
func (b *bubbleWindow) hoverAt(x, y int32) int {
	switch {
	case inRect(b.crossRect, x, y):
		return hoverCross
	case inRect(b.copyRect, x, y):
		return hoverCopy
	case inRect(b.closeRect, x, y):
		return hoverClose
	}
	return hoverNone
}

func bubbleWndProc(hwnd, message, wparam, lparam uintptr) uintptr {
	app := currentApp
	if app == nil || app.bubble == nil {
		return w32.DefWindowProc(w32.HWND(hwnd), uint32(message), wparam, lparam)
	}
	b := app.bubble
	x, y := int32(int16(lparam&0xFFFF)), int32(int16((lparam>>16)&0xFFFF))

	switch uint32(message) {
	case w32.WM_PAINT:
		b.paint()
		return 0

	case w32.WM_ERASEBKGND:
		return 1 // tout est peint dans WM_PAINT : evite le clignotement

	case w32.WM_CTLCOLOREDIT, w32.WM_CTLCOLORSTATIC:
		w32.SetTextColor(w32.HDC(wparam), b.palette.text)
		w32.SetBkColor(w32.HDC(wparam), b.palette.background)
		return uintptr(b.background)

	case w32.WM_LBUTTONDOWN:
		if y < b.scale(headerHeight) && b.hoverAt(x, y) == hoverNone {
			// Deplacement a la souris : Windows s'en charge pour nous.
			w32.SendMessage(b.hwnd, w32.WM_NCLBUTTONDOWN, w32.HTCAPTION, 0)
		}
		return 0

	case w32.WM_LBUTTONUP:
		switch b.hoverAt(x, y) {
		case hoverCross, hoverClose:
			b.hide()
		case hoverCopy:
			if err := setClipboardText(b.text); err == nil {
				b.hint = "Copié dans le presse-papiers"
				w32.InvalidateRect(b.hwnd, false)
			}
		}
		return 0

	case w32.WM_MOUSEMOVE:
		if !b.mouseInside {
			b.mouseInside = true
			w32.KillTimer(b.hwnd, timerClose) // la bulle attend tant qu'on la survole
			w32.TrackMouseLeave(b.hwnd)
		}
		if hovered := b.hoverAt(x, y); hovered != b.hovered {
			b.hovered = hovered
			w32.InvalidateRect(b.hwnd, false)
		}
		return 0

	case w32.WM_SETCURSOR:
		// Sans ce traitement, Windows remettrait la fleche a chaque mouvement.
		if b.hovered != hoverNone {
			w32.SetCursor(w32.LoadCursor(w32.IDC_HAND))
			return 1
		}

	case w32.WM_MOUSELEAVE:
		b.mouseInside = false
		if b.hovered != hoverNone {
			b.hovered = hoverNone
			w32.InvalidateRect(b.hwnd, false)
		}
		b.startCloseTimer()
		return 0

	case w32.WM_KEYDOWN:
		if wparam == w32.VK_ESCAPE {
			b.hide()
		}
		return 0

	case w32.WM_DPICHANGED:
		// L'utilisateur a change d'ecran : on refait les polices a la bonne taille.
		b.dpi = int32(wparam & 0xFFFF)
		b.makeFonts()
		b.layout()
		w32.InvalidateRect(b.hwnd, true)
		return 0

	case w32.WM_TIMER:
		switch wparam {
		case timerClose:
			b.hide()
		case timerFade:
			b.alpha += 51
			if b.alpha >= 255 {
				b.alpha = 255
				w32.KillTimer(b.hwnd, timerFade)
			}
			w32.SetLayeredWindowAlpha(b.hwnd, byte(b.alpha))
		}
		return 0

	case w32.WM_CLOSE:
		b.hide()
		return 0
	}
	return w32.DefWindowProc(w32.HWND(hwnd), uint32(message), wparam, lparam)
}
