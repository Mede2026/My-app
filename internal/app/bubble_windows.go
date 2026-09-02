package app

import (
	"strings"

	"golang.org/x/sys/windows"

	"github.com/mede2026/cryptobulle/internal/w32"
)

// La bulle est une petite fenetre sans bordure, toujours au-dessus des autres,
// qui apparait pres du curseur. Elle est construite une seule fois puis
// reutilisee : le deuxieme affichage est donc immediat.

const (
	bubbleClass = "CryptoBulleBubble"

	kindInfo    = "info"
	kindSuccess = "success"
	kindError   = "error"

	timerClose = 1
	timerFade  = 2

	bubbleBodyID = 100
)

// Dimensions en points « logiques » (mises a l'echelle selon l'ecran).
const (
	bubbleWidth   = 400
	bubbleBorder  = 2
	bubblePadding = 12
	headerHeight  = 26
	footerHeight  = 32
	bodyMinHeight = 22
	bodyMaxHeight = 260
	buttonWidth   = 66
	buttonHeight  = 22
)

type palette struct {
	background, text, title, muted, button, buttonText uintptr
}

var themes = map[string]palette{
	"sombre": {
		background: w32.RGB(0x1c, 0x1f, 0x26),
		text:       w32.RGB(0xe6, 0xe9, 0xef),
		title:      w32.RGB(0xff, 0xff, 0xff),
		muted:      w32.RGB(0x98, 0xa2, 0xb3),
		button:     w32.RGB(0x2a, 0x2f, 0x3a),
		buttonText: w32.RGB(0xe6, 0xe9, 0xef),
	},
	"clair": {
		background: w32.RGB(0xff, 0xff, 0xff),
		text:       w32.RGB(0x1f, 0x29, 0x37),
		title:      w32.RGB(0x11, 0x18, 0x27),
		muted:      w32.RGB(0x6b, 0x72, 0x80),
		button:     w32.RGB(0xee, 0xf2, 0xf7),
		buttonText: w32.RGB(0x11, 0x18, 0x27),
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

	palette    palette
	accent     uintptr
	background w32.HBRUSH
	titleFont  w32.HFONT
	bodyFont   w32.HFONT
	smallFont  w32.HFONT

	copyRect, closeRect, crossRect w32.RECT
	alpha                          int32
	hovered                        bool
	visible                        bool
}

func newBubbleWindow(app *App) *bubbleWindow {
	instance := w32.ModuleHandle()
	class := w32.WNDCLASS{
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

	// Le corps du message est un champ de saisie en lecture seule : on obtient
	// gratuitement la selection a la souris et le defilement a la molette.
	bubble.body = w32.CreateWindowEx(
		0, "EDIT", "",
		w32.WS_CHILD|w32.WS_VISIBLE|w32.ES_MULTILINE|w32.ES_READONLY|0x0040, /* ES_AUTOVSCROLL */
		0, 0, 10, 10, bubble.hwnd, w32.HMENU(bubbleBodyID), instance,
	)

	bubble.applyTheme(app.config().Theme)
	return bubble
}

func (b *bubbleWindow) scale(value int32) int32 { return value * b.app.dpi / 96 }

func (b *bubbleWindow) applyTheme(theme string) {
	chosen, ok := themes[theme]
	if !ok {
		chosen = themes["sombre"]
	}
	if b.background != 0 {
		w32.DeleteObject(uintptr(b.background))
	}
	b.palette = chosen
	b.background = w32.CreateSolidBrush(chosen.background)

	if b.titleFont == 0 {
		dpi := b.app.dpi
		b.titleFont = w32.CreateFont("Segoe UI", 10, dpi, true)
		b.bodyFont = w32.CreateFont("Segoe UI", 10, dpi, false)
		b.smallFont = w32.CreateFont("Segoe UI", 8, dpi, false)
		w32.SendMessage(b.body, w32.WM_SETFONT, uintptr(b.bodyFont), 1)
	}
}

// show affiche la bulle avec un nouveau contenu.
func (b *bubbleWindow) show(title, text, kind string, seconds int) {
	if theme := b.app.config().Theme; themes[theme].background != b.palette.background {
		b.applyTheme(theme)
	}
	b.title, b.text, b.kind, b.seconds = title, text, kind, seconds
	b.accent = accents[kind]
	if b.accent == 0 {
		b.accent = accents[kindInfo]
	}
	b.hint = "Echap pour fermer"

	w32.SetWindowText(b.body, strings.ReplaceAll(text, "\n", "\r\n"))
	b.layout()

	b.alpha = 0
	w32.SetLayeredWindowAlpha(b.hwnd, 0)
	w32.ShowWindow(b.hwnd, w32.SW_SHOW)
	w32.SetForegroundWindow(b.hwnd)
	w32.SetFocus(b.hwnd) // le focus reste a la fenetre : Echap fonctionne
	w32.InvalidateRect(b.hwnd, true)
	b.visible = true

	w32.SetTimer(b.hwnd, timerFade, 12)
	b.startCloseTimer()
}

// layout calcule la taille de la bulle et la place pres du curseur.
func (b *bubbleWindow) layout() {
	width := b.scale(bubbleWidth)
	inner := width - 2*b.scale(bubbleBorder) - 2*b.scale(bubblePadding)

	bodyHeight := b.measureBody(inner)
	if maximum := b.scale(bodyMaxHeight); bodyHeight > maximum {
		bodyHeight = maximum
		b.hint = "Molette pour faire defiler"
	}
	if minimum := b.scale(bodyMinHeight); bodyHeight < minimum {
		bodyHeight = minimum
	}

	height := 2*b.scale(bubbleBorder) + b.scale(headerHeight) + bodyHeight + b.scale(footerHeight)

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

	// Le champ de texte occupe la zone centrale.
	left := b.scale(bubbleBorder) + b.scale(bubblePadding)
	top := b.scale(bubbleBorder) + b.scale(headerHeight)
	w32.SetWindowPos(b.body, 0, left, top, inner, bodyHeight, w32.SWP_NOACTIVATE)

	// Boutons du bas, alignes a droite.
	buttonTop := top + bodyHeight + b.scale(5)
	right := width - b.scale(bubbleBorder) - b.scale(bubblePadding)
	b.closeRect = w32.RECT{
		Left: right - b.scale(buttonWidth), Top: buttonTop,
		Right: right, Bottom: buttonTop + b.scale(buttonHeight),
	}
	b.copyRect = w32.RECT{
		Left: b.closeRect.Left - b.scale(buttonWidth) - b.scale(6), Top: buttonTop,
		Right: b.closeRect.Left - b.scale(6), Bottom: buttonTop + b.scale(buttonHeight),
	}
	cross := b.scale(18)
	b.crossRect = w32.RECT{
		Left: right - cross, Top: b.scale(bubbleBorder) + b.scale(4),
		Right: right, Bottom: b.scale(bubbleBorder) + b.scale(4) + cross,
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
	if b.seconds > 0 && !b.hovered {
		w32.SetTimer(b.hwnd, timerClose, uint32(b.seconds*1000))
	}
}

func (b *bubbleWindow) hide() {
	w32.KillTimer(b.hwnd, timerClose)
	w32.KillTimer(b.hwnd, timerFade)
	w32.ShowWindow(b.hwnd, w32.SW_HIDE)
	b.visible = false
	b.hovered = false
}

func (b *bubbleWindow) paint() {
	var paint w32.PAINTSTRUCT
	dc := w32.BeginPaint(b.hwnd, &paint)
	defer w32.EndPaint(b.hwnd, &paint)

	client := w32.ClientRect(b.hwnd)
	border := w32.CreateSolidBrush(b.accent)
	w32.FillRect(dc, client, border) // le fond accentue fait office de bordure
	w32.DeleteObject(uintptr(border))

	edge := b.scale(bubbleBorder)
	inside := w32.RECT{
		Left: client.Left + edge, Top: client.Top + edge,
		Right: client.Right - edge, Bottom: client.Bottom - edge,
	}
	w32.FillRect(dc, inside, b.background)
	w32.SetBkTransparent(dc)

	padding := b.scale(bubblePadding)

	// Titre, avec une pastille de couleur en guise d'icone.
	titleRect := w32.RECT{
		Left: inside.Left + padding, Top: inside.Top + b.scale(6),
		Right: b.crossRect.Left - b.scale(4), Bottom: inside.Top + b.scale(headerHeight),
	}
	previous := w32.SelectObject(dc, uintptr(b.titleFont))
	w32.SetTextColor(dc, b.palette.title)
	w32.DrawText(dc, b.title, &titleRect, w32.DT_LEFT|w32.DT_SINGLELINE|w32.DT_NOPREFIX)

	// Croix de fermeture.
	w32.SelectObject(dc, uintptr(b.smallFont))
	w32.SetTextColor(dc, b.palette.muted)
	cross := b.crossRect
	w32.DrawText(dc, "✕", &cross, w32.DT_CENTER|w32.DT_VCENTER|w32.DT_SINGLELINE)

	// Note du bas.
	hint := w32.RECT{
		Left: inside.Left + padding, Top: b.copyRect.Top,
		Right: b.copyRect.Left, Bottom: b.copyRect.Bottom,
	}
	w32.DrawText(dc, b.hint, &hint, w32.DT_LEFT|w32.DT_VCENTER|w32.DT_SINGLELINE|w32.DT_NOPREFIX)

	b.paintButton(dc, b.copyRect, "Copier")
	b.paintButton(dc, b.closeRect, "Fermer")
	w32.SelectObject(dc, previous)
}

func (b *bubbleWindow) paintButton(dc w32.HDC, rect w32.RECT, label string) {
	brush := w32.CreateSolidBrush(b.palette.button)
	w32.FillRect(dc, rect, brush)
	w32.DeleteObject(uintptr(brush))

	w32.SelectObject(dc, uintptr(b.smallFont))
	w32.SetTextColor(dc, b.palette.buttonText)
	box := rect
	w32.DrawText(dc, label, &box, w32.DT_CENTER|w32.DT_VCENTER|w32.DT_SINGLELINE|w32.DT_NOPREFIX)
}

func inRect(rect w32.RECT, x, y int32) bool {
	return x >= rect.Left && x < rect.Right && y >= rect.Top && y < rect.Bottom
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
		if y < b.scale(headerHeight) && !inRect(b.crossRect, x, y) {
			// Deplacement a la souris : Windows s'en charge pour nous.
			w32.SendMessage(b.hwnd, w32.WM_NCLBUTTONDOWN, w32.HTCAPTION, 0)
		}
		return 0

	case w32.WM_LBUTTONUP:
		switch {
		case inRect(b.crossRect, x, y), inRect(b.closeRect, x, y):
			b.hide()
		case inRect(b.copyRect, x, y):
			if err := setClipboardText(b.text); err == nil {
				b.hint = "Copie dans le presse-papiers"
				w32.InvalidateRect(b.hwnd, false)
			}
		}
		return 0

	case w32.WM_MOUSEMOVE:
		if !b.hovered {
			b.hovered = true
			w32.KillTimer(b.hwnd, timerClose) // la bulle attend tant qu'on la survole
			w32.TrackMouseLeave(b.hwnd)
		}
		return 0

	case w32.WM_MOUSELEAVE:
		b.hovered = false
		b.startCloseTimer()
		return 0

	case w32.WM_KEYDOWN:
		if wparam == w32.VK_ESCAPE {
			b.hide()
		}
		return 0

	case w32.WM_TIMER:
		switch wparam {
		case timerClose:
			b.hide()
		case timerFade:
			b.alpha += 45
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
