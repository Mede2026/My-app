package w32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Poignees Windows, nommees pour la lisibilite du reste du code.
type (
	HWND   uintptr
	HDC    uintptr
	HICON  uintptr
	HMENU  uintptr
	HBRUSH uintptr
	HFONT  uintptr
)

// Messages de fenetre.
const (
	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	WM_QUIT           = 0x0012
	WM_ERASEBKGND     = 0x0014
	WM_SETFONT        = 0x0030
	WM_GETMINMAXINFO  = 0x0024
	WM_PAINT          = 0x000F
	WM_KEYDOWN        = 0x0100
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_RBUTTONUP      = 0x0205
	WM_MOUSELEAVE     = 0x02A3
	WM_NCLBUTTONDOWN  = 0x00A1
	WM_HOTKEY         = 0x0312
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORSTATIC = 0x0138
	WM_CTLCOLORBTN    = 0x0135
	WM_SETCURSOR      = 0x0020
	WM_SETICON        = 0x0080
	ICON_SMALL        = 0
	ICON_BIG          = 1
	WM_APP            = 0x8000

	// Messages propres a CryptoBulle.
	WM_TRAY = WM_APP + 1 // clic sur l'icone pres de l'horloge
	WM_TASK = WM_APP + 2 // « des taches attendent d'etre executees »
)

// Styles de fenetre.
const (
	WS_POPUP         = 0x80000000
	WS_VISIBLE       = 0x10000000
	WS_CHILD         = 0x40000000
	WS_TABSTOP       = 0x00010000
	WS_GROUP         = 0x00020000
	WS_VSCROLL       = 0x00200000
	WS_BORDER        = 0x00800000
	WS_CAPTION       = 0x00C00000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_EX_TOPMOST    = 0x00000008
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_LAYERED    = 0x00080000
	WS_EX_CLIENTEDGE = 0x00000200
	WS_EX_NOACTIVATE = 0x08000000
)

// Styles de controles.
const (
	BS_AUTOCHECKBOX    = 0x0003
	BS_AUTORADIOBUTTON = 0x0009
	BS_GROUPBOX        = 0x0007
	BS_PUSHBUTTON      = 0x0000
	BS_DEFPUSHBUTTON   = 0x0001
	ES_AUTOHSCROLL     = 0x0080
	ES_MULTILINE       = 0x0004
	ES_PASSWORD        = 0x0020
	ES_READONLY        = 0x0800
	ES_WANTRETURN      = 0x1000
	SS_LEFT            = 0x0000
)

// Messages envoyes aux controles.
const (
	BM_GETCHECK        = 0x00F0
	BM_SETCHECK        = 0x00F1
	EM_SETPASSWORDCHAR = 0x00CC
	EM_SETSEL          = 0x00B1
	WM_SETTEXT         = 0x000C
	WM_GETTEXT         = 0x000D
	WM_GETTEXTLENGTH   = 0x000E
	BST_CHECKED        = 1
	BST_UNCHECKED      = 0
)

// Dessin de texte.
const (
	DT_LEFT        = 0x0000
	DT_CENTER      = 0x0001
	DT_RIGHT       = 0x0002
	DT_VCENTER     = 0x0004
	DT_WORDBREAK   = 0x0010
	DT_SINGLELINE  = 0x0020
	DT_CALCRECT    = 0x0400
	DT_NOPREFIX    = 0x0800
	DT_EDITCONTROL = 0x2000
	TRANSPARENT    = 1
)

// Divers.
const (
	SW_HIDE            = 0
	SW_SHOW            = 5
	SW_SHOWNORMAL      = 1
	SWP_NOSIZE         = 0x0001
	SWP_NOMOVE         = 0x0002
	SWP_NOACTIVATE     = 0x0010
	SWP_SHOWWINDOW     = 0x0040
	HWND_TOPMOST       = ^uintptr(0) // (HWND)-1
	MF_STRING          = 0x0000
	MF_SEPARATOR       = 0x0800
	TPM_RIGHTBUTTON    = 0x0002
	TPM_RETURNCMD      = 0x0100
	IMAGE_ICON         = 1
	LR_DEFAULTCOLOR    = 0x0000
	IDC_ARROW          = 32512
	IDC_HAND           = 32649
	COLOR_WINDOW       = 5
	COLOR_WINDOWTEXT   = 8
	COLOR_BTNFACE      = 15
	SM_CXSMICON        = 49
	SM_CYSMICON        = 50
	SPI_GETWORKAREA    = 0x0030
	MB_OK              = 0x0000
	MB_ICONERROR       = 0x0010
	MB_ICONINFORMATION = 0x0040
	MB_YESNO           = 0x0004
	IDYES              = 6
	CF_UNICODETEXT     = 13
	GMEM_MOVEABLE      = 0x0002
	INPUT_KEYBOARD     = 1
	KEYEVENTF_KEYUP    = 0x0002
	NIM_ADD            = 0
	NIM_DELETE         = 2
	NIF_MESSAGE        = 0x01
	NIF_ICON           = 0x02
	NIF_TIP            = 0x04
	HTCAPTION          = 2
	LWA_ALPHA          = 0x00000002
	VK_CONTROL         = 0x11
	VK_MENU            = 0x12
	VK_SHIFT           = 0x10
	VK_LWIN            = 0x5B
	VK_RWIN            = 0x5C
	VK_ESCAPE          = 0x1B
	VK_C               = 0x43
	VK_V               = 0x56
)

// --- structures --------------------------------------------------------------

type POINT struct{ X, Y int32 }

type RECT struct{ Left, Top, Right, Bottom int32 }

func (r RECT) Width() int32  { return r.Right - r.Left }
func (r RECT) Height() int32 { return r.Bottom - r.Top }

type MSG struct {
	Hwnd    HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	Private uint32
}

type WNDCLASS struct {
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       HICON
	Cursor     uintptr
	Background HBRUSH
	MenuName   *uint16
	ClassName  *uint16
}

type PAINTSTRUCT struct {
	Hdc         HDC
	Erase       int32
	Paint       RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}

type NOTIFYICONDATA struct {
	Size            uint32
	Wnd             HWND
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            HICON
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GuidItem        [16]byte
	BalloonIcon     HICON
}

type KEYBDINPUT struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
	_         [8]byte // complete la taille de l'union INPUT
}

type INPUT struct {
	Type uint32
	_    uint32 // alignement
	Ki   KEYBDINPUT
}

type TRACKMOUSEEVENT struct {
	Size      uint32
	Flags     uint32
	Track     HWND
	HoverTime uint32
}

const TME_LEAVE = 0x00000002

// --- bibliotheques et fonctions ----------------------------------------------

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	procRegisterClass              = user32.NewProc("RegisterClassW")
	procCreateWindowEx             = user32.NewProc("CreateWindowExW")
	procDefWindowProc              = user32.NewProc("DefWindowProcW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procUpdateWindow               = user32.NewProc("UpdateWindow")
	procGetMessage                 = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessage            = user32.NewProc("DispatchMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procPostMessage                = user32.NewProc("PostMessageW")
	procSendMessage                = user32.NewProc("SendMessageW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procSystemParametersInfo       = user32.NewProc("SystemParametersInfoW")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procCreatePopupMenu            = user32.NewProc("CreatePopupMenu")
	procAppendMenu                 = user32.NewProc("AppendMenuW")
	procTrackPopupMenu             = user32.NewProc("TrackPopupMenu")
	procDestroyMenu                = user32.NewProc("DestroyMenu")
	procRegisterHotKey             = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey           = user32.NewProc("UnregisterHotKey")
	procSendInput                  = user32.NewProc("SendInput")
	procMapVirtualKey              = user32.NewProc("MapVirtualKeyW")
	procGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procClipboardSequence          = user32.NewProc("GetClipboardSequenceNumber")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procFillRect                   = user32.NewProc("FillRect")
	procDrawText                   = user32.NewProc("DrawTextW")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procLoadCursor                 = user32.NewProc("LoadCursorW")
	procSetCursor                  = user32.NewProc("SetCursor")
	procCreateIconFromResourceEx   = user32.NewProc("CreateIconFromResourceEx")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procSetFocus                   = user32.NewProc("SetFocus")
	procTrackMouseEvent            = user32.NewProc("TrackMouseEvent")
	procMessageBox                 = user32.NewProc("MessageBoxW")
	procGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	procSetProcessDPIAware         = user32.NewProc("SetProcessDPIAware")
	procScreenToClient             = user32.NewProc("ScreenToClient")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procEnableWindow               = user32.NewProc("EnableWindow")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
	procGlobalFree   = kernel32.NewProc("GlobalFree")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procCreateFont       = gdi32.NewProc("CreateFontW")

	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
)

// Str convertit une chaine Go en pointeur UTF-16 pour Windows.
func Str(text string) *uint16 {
	pointer, err := windows.UTF16PtrFromString(text)
	if err != nil {
		empty, _ := windows.UTF16PtrFromString("")
		return empty
	}
	return pointer
}

// FromUTF16 relit une chaine renvoyee par Windows.
func FromUTF16(pointer *uint16, max int) string {
	if pointer == nil {
		return ""
	}
	return windows.UTF16ToString(unsafe.Slice(pointer, max))
}

func ModuleHandle() windows.Handle {
	var module windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &module); err != nil {
		return 0
	}
	return module
}

// --- fenetres ----------------------------------------------------------------

func RegisterClass(class *WNDCLASS) uintptr {
	atom, _, _ := procRegisterClass.Call(uintptr(unsafe.Pointer(class)))
	return atom
}

func CreateWindowEx(exStyle uint32, class, title string, style uint32, x, y, width, height int32, parent HWND, menu HMENU, instance windows.Handle) HWND {
	handle, _, _ := procCreateWindowEx.Call(
		uintptr(exStyle), uintptr(unsafe.Pointer(Str(class))), uintptr(unsafe.Pointer(Str(title))),
		uintptr(style), uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		uintptr(parent), uintptr(menu), uintptr(instance), 0,
	)
	return HWND(handle)
}

func DefWindowProc(hwnd HWND, message uint32, wparam, lparam uintptr) uintptr {
	result, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(message), wparam, lparam)
	return result
}

func DestroyWindow(hwnd HWND)           { procDestroyWindow.Call(uintptr(hwnd)) }
func ShowWindow(hwnd HWND, command int) { procShowWindow.Call(uintptr(hwnd), uintptr(command)) }
func UpdateWindow(hwnd HWND)            { procUpdateWindow.Call(uintptr(hwnd)) }

func GetMessage(message *MSG) int32 {
	result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(message)), 0, 0, 0)
	return int32(result)
}

func TranslateMessage(message *MSG) { procTranslateMessage.Call(uintptr(unsafe.Pointer(message))) }
func DispatchMessage(message *MSG)  { procDispatchMessage.Call(uintptr(unsafe.Pointer(message))) }
func PostQuitMessage(code int)      { procPostQuitMessage.Call(uintptr(code)) }

func PostMessage(hwnd HWND, message uint32, wparam, lparam uintptr) {
	procPostMessage.Call(uintptr(hwnd), uintptr(message), wparam, lparam)
}

func SendMessage(hwnd HWND, message uint32, wparam, lparam uintptr) uintptr {
	result, _, _ := procSendMessage.Call(uintptr(hwnd), uintptr(message), wparam, lparam)
	return result
}

func SetWindowPos(hwnd HWND, after uintptr, x, y, width, height int32, flags uint32) {
	procSetWindowPos.Call(uintptr(hwnd), after, uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(flags))
}

func GetSystemMetrics(index int32) int32 {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int32(value)
}

func WorkArea() RECT {
	var area RECT
	procSystemParametersInfo.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&area)), 0)
	return area
}

func CursorPos() POINT {
	var point POINT
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	return point
}

func SetForegroundWindow(hwnd HWND) { procSetForegroundWindow.Call(uintptr(hwnd)) }
func SetFocus(hwnd HWND)            { procSetFocus.Call(uintptr(hwnd)) }
func EnableWindow(hwnd HWND, enabled bool) {
	value := uintptr(0)
	if enabled {
		value = 1
	}
	procEnableWindow.Call(uintptr(hwnd), value)
}

func ClientRect(hwnd HWND) RECT {
	var rect RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	return rect
}

func WindowRect(hwnd HWND) RECT {
	var rect RECT
	procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	return rect
}

func ScreenToClient(hwnd HWND, point *POINT) {
	procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(point)))
}

func InvalidateRect(hwnd HWND, erase bool) {
	value := uintptr(0)
	if erase {
		value = 1
	}
	procInvalidateRect.Call(uintptr(hwnd), 0, value)
}

func SetTimer(hwnd HWND, id uintptr, milliseconds uint32) {
	procSetTimer.Call(uintptr(hwnd), id, uintptr(milliseconds), 0)
}

func KillTimer(hwnd HWND, id uintptr) { procKillTimer.Call(uintptr(hwnd), id) }

func TrackMouseLeave(hwnd HWND) {
	event := TRACKMOUSEEVENT{Flags: TME_LEAVE, Track: hwnd}
	event.Size = uint32(unsafe.Sizeof(event))
	procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&event)))
}

func LoadCursor(id uintptr) uintptr {
	cursor, _, _ := procLoadCursor.Call(0, id)
	return cursor
}

func SetCursor(cursor uintptr) { procSetCursor.Call(cursor) }

func SetLayeredWindowAlpha(hwnd HWND, alpha byte) {
	procSetLayeredWindowAttributes.Call(uintptr(hwnd), 0, uintptr(alpha), LWA_ALPHA)
}

func MessageBox(hwnd HWND, text, title string, flags uint32) int32 {
	result, _, _ := procMessageBox.Call(
		uintptr(hwnd), uintptr(unsafe.Pointer(Str(text))),
		uintptr(unsafe.Pointer(Str(title))), uintptr(flags),
	)
	return int32(result)
}

// DPI est le nombre de points par pouce de l'ecran (96 = echelle 100 %).
func DPI(hwnd HWND) int32 {
	if procGetDpiForWindow.Find() == nil {
		if dpi, _, _ := procGetDpiForWindow.Call(uintptr(hwnd)); dpi >= 72 {
			return int32(dpi)
		}
	}
	return 96
}

// EnableDPIAwareness evite que Windows agrandisse la fenetre a posteriori,
// ce qui rendrait le texte flou sur un ecran haute densite.
func EnableDPIAwareness() { procSetProcessDPIAware.Call() }

// --- menus --------------------------------------------------------------------

func CreatePopupMenu() HMENU {
	menu, _, _ := procCreatePopupMenu.Call()
	return HMENU(menu)
}

func AppendMenu(menu HMENU, flags uint32, id uintptr, label string) {
	var text uintptr
	if label != "" {
		text = uintptr(unsafe.Pointer(Str(label)))
	}
	procAppendMenu.Call(uintptr(menu), uintptr(flags), id, text)
}

func TrackPopupMenu(menu HMENU, flags uint32, x, y int32, hwnd HWND) int32 {
	choice, _, _ := procTrackPopupMenu.Call(
		uintptr(menu), uintptr(flags), uintptr(x), uintptr(y), 0, uintptr(hwnd), 0,
	)
	return int32(choice)
}

func DestroyMenu(menu HMENU) { procDestroyMenu.Call(uintptr(menu)) }

// --- raccourcis et frappes ------------------------------------------------------

func RegisterHotKey(hwnd HWND, id int32, modifiers, key uint32) bool {
	ok, _, _ := procRegisterHotKey.Call(uintptr(hwnd), uintptr(id), uintptr(modifiers), uintptr(key))
	return ok != 0
}

func UnregisterHotKey(hwnd HWND, id int32) {
	procUnregisterHotKey.Call(uintptr(hwnd), uintptr(id))
}

func SendInputs(inputs []INPUT) {
	if len(inputs) == 0 {
		return
	}
	procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
}

func MapVirtualKey(key uint32) uint16 {
	scan, _, _ := procMapVirtualKey.Call(uintptr(key), 0)
	return uint16(scan)
}

func KeyIsDown(key uint32) bool {
	state, _, _ := procGetAsyncKeyState.Call(uintptr(key))
	return state&0x8000 != 0
}

// --- presse-papiers ---------------------------------------------------------------

func OpenClipboard() bool {
	ok, _, _ := procOpenClipboard.Call(0)
	return ok != 0
}

func CloseClipboard() { procCloseClipboard.Call() }
func EmptyClipboard() { procEmptyClipboard.Call() }

func ClipboardSequence() uint32 {
	value, _, _ := procClipboardSequence.Call()
	return uint32(value)
}

// pointerFromAddress convertit une adresse renvoyee par Windows en pointeur.
//
// La memoire vient du systeme (GlobalAlloc), pas du ramasse-miettes de Go : la
// conversion est donc sure, mais « go vet » ne peut pas le deviner et signale
// une conversion uintptr -> pointeur. Cette reinterpretation lui evite une
// fausse alerte sans rien changer au resultat.
func pointerFromAddress(address uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&address))
}

func GetClipboardText() string {
	handle, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if handle == 0 {
		return ""
	}
	pointer, _, _ := procGlobalLock.Call(handle)
	if pointer == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(handle)
	return windows.UTF16PtrToString((*uint16)(pointerFromAddress(pointer)))
}

// SetClipboardText suppose le presse-papiers deja ouvert et vide.
func SetClipboardText(text string) bool {
	encoded := windows.StringToUTF16(text)
	size := uintptr(len(encoded) * 2)
	handle, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if handle == 0 {
		return false
	}
	pointer, _, _ := procGlobalLock.Call(handle)
	if pointer == 0 {
		procGlobalFree.Call(handle)
		return false
	}
	copy(unsafe.Slice((*uint16)(pointerFromAddress(pointer)), len(encoded)), encoded)
	procGlobalUnlock.Call(handle)

	if result, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, handle); result == 0 {
		procGlobalFree.Call(handle)
		return false
	}
	// Succes : Windows devient proprietaire du bloc, il ne faut pas le liberer.
	return true
}

// --- dessin -----------------------------------------------------------------------

func BeginPaint(hwnd HWND, paint *PAINTSTRUCT) HDC {
	dc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(paint)))
	return HDC(dc)
}

func EndPaint(hwnd HWND, paint *PAINTSTRUCT) {
	procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(paint)))
}

// RGB compose une couleur au format attendu par Windows (bleu-vert-rouge).
func RGB(red, green, blue byte) uintptr {
	return uintptr(red) | uintptr(green)<<8 | uintptr(blue)<<16
}

func CreateSolidBrush(color uintptr) HBRUSH {
	brush, _, _ := procCreateSolidBrush.Call(color)
	return HBRUSH(brush)
}

func FillRect(dc HDC, rect RECT, brush HBRUSH) {
	procFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&rect)), uintptr(brush))
}

func DeleteObject(object uintptr) { procDeleteObject.Call(object) }

func SelectObject(dc HDC, object uintptr) uintptr {
	previous, _, _ := procSelectObject.Call(uintptr(dc), object)
	return previous
}

func SetBkTransparent(dc HDC)            { procSetBkMode.Call(uintptr(dc), TRANSPARENT) }
func SetTextColor(dc HDC, color uintptr) { procSetTextColor.Call(uintptr(dc), color) }

func DrawText(dc HDC, text string, rect *RECT, format uint32) int32 {
	encoded := windows.StringToUTF16(text)
	height, _, _ := procDrawText.Call(
		uintptr(dc), uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1),
		uintptr(unsafe.Pointer(rect)), uintptr(format),
	)
	return int32(height)
}

// CreateFont fabrique une police. `size` est en points, mise a l'echelle du DPI.
func CreateFont(name string, size, dpi int32, bold bool) HFONT {
	weight := int32(400)
	if bold {
		weight = 600
	}
	height := -(size * dpi / 72)
	font, _, _ := procCreateFont.Call(
		uintptr(height), 0, 0, 0, uintptr(weight), 0, 0, 0,
		1 /* DEFAULT_CHARSET */, 0, 0, 4 /* CLEARTYPE_QUALITY */, 0,
		uintptr(unsafe.Pointer(Str(name))),
	)
	return HFONT(font)
}

// --- icone pres de l'horloge --------------------------------------------------------

func ShellNotifyIcon(action uint32, data *NOTIFYICONDATA) bool {
	ok, _, _ := procShellNotifyIcon.Call(uintptr(action), uintptr(unsafe.Pointer(data)))
	return ok != 0
}

// CreateIconFromICO fabrique une icone a partir d'une image contenue dans un
// fichier .ico deja charge en memoire.
func CreateIconFromICO(image []byte, width, height int32) HICON {
	if len(image) == 0 {
		return 0
	}
	icon, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&image[0])), uintptr(len(image)), 1,
		0x00030000, uintptr(width), uintptr(height), LR_DEFAULTCOLOR,
	)
	return HICON(icon)
}

// --- contextes de dessin et texte des controles ------------------------------

var (
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procSetBkColor          = gdi32.NewProc("SetBkColor")
)

var procAdjustWindowRect = user32.NewProc("AdjustWindowRect")

// AdjustWindowRect calcule la taille exterieure d'une fenetre a partir de la
// taille utile voulue (barre de titre et bordures comprises).
func AdjustWindowRect(rect *RECT, style uint32) {
	procAdjustWindowRect.Call(uintptr(unsafe.Pointer(rect)), uintptr(style), 0)
}

var (
	procGetSysColor      = user32.NewProc("GetSysColor")
	procGetSysColorBrush = user32.NewProc("GetSysColorBrush")
)

// SysColor rend une couleur du theme courant de Windows (fond de fenetre,
// couleur du texte...). La respecter, plutot que d'ecrire du blanc en dur,
// garde l'application lisible avec les themes a fort contraste.
func SysColor(index int32) uintptr {
	color, _, _ := procGetSysColor.Call(uintptr(index))
	return color
}

// SysColorBrush rend le pinceau correspondant. Il appartient au systeme :
// il ne faut jamais le detruire.
func SysColorBrush(index int32) HBRUSH {
	brush, _, _ := procGetSysColorBrush.Call(uintptr(index))
	return HBRUSH(brush)
}

func GetDC(hwnd HWND) HDC {
	dc, _, _ := procGetDC.Call(uintptr(hwnd))
	return HDC(dc)
}

func ReleaseDC(hwnd HWND, dc HDC) { procReleaseDC.Call(uintptr(hwnd), uintptr(dc)) }

func SetBkColor(dc HDC, color uintptr) { procSetBkColor.Call(uintptr(dc), color) }

// WindowText lit le contenu d'un controle (champ de saisie, bouton...).
func WindowText(hwnd HWND) string {
	length, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, length+1)
	procGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buffer[0])), length+1)
	return windows.UTF16ToString(buffer)
}

// SetWindowText remplace le contenu d'un controle.
func SetWindowText(hwnd HWND, text string) {
	procSetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(Str(text))))
}

// Checked indique si une case a cocher est cochee.
func Checked(hwnd HWND) bool { return SendMessage(hwnd, BM_GETCHECK, 0, 0) == BST_CHECKED }

// SetChecked coche ou decoche une case.
func SetChecked(hwnd HWND, checked bool) {
	state := uintptr(BST_UNCHECKED)
	if checked {
		state = BST_CHECKED
	}
	SendMessage(hwnd, BM_SETCHECK, state, 0)
}
