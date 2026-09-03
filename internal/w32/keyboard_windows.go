package w32

import (
	"fmt"
	"unsafe"
)

// Interception du clavier pour la frappe masquee.
//
// Un « hook bas niveau » demande a Windows de prevenir l'application avant que
// chaque touche n'atteigne le programme au premier plan. La touche peut alors
// etre avalee et remplacee par une autre. C'est le seul moyen de changer ce qui
// s'affiche dans un logiciel quelconque.
//
// Ce hook n'est installe que pendant que le mode est actif, et rien n'est
// enregistre : les touches sont transformees puis oubliees.

const (
	WH_KEYBOARD_LL = 13

	WM_KEYUP      = 0x0101
	WM_SYSKEYDOWN = 0x0104
	WM_SYSKEYUP   = 0x0105

	LLKHF_INJECTED = 0x00000010

	KEYEVENTF_UNICODE = 0x0004

	VK_BACK    = 0x08
	VK_TAB     = 0x09
	VK_RETURN  = 0x0D
	VK_LMENU   = 0xA4 // Alt de gauche
	VK_RMENU   = 0xA5 // Alt de droite, « AltGr » sur les claviers francais
	VK_CAPITAL = 0x14
	// VK_PACKET : une touche « fabriquee » par un programme, qui transporte
	// directement un caractere. C'est ainsi qu'arrivent les emojis choisis dans
	// le panneau de Windows.
	VK_PACKET = 0xE7

	// Signature attachee a nos propres frappes simulees, pour les reconnaitre
	// quand elles repassent par le hook.
	injectedSignature = 0x43427542 // « CBuB »
)

// KBDLLHOOKSTRUCT decrit la touche transmise par Windows au hook.
type KBDLLHOOKSTRUCT struct {
	VkCode    uint32
	ScanCode  uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

var (
	procSetWindowsHookEx         = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx           = user32.NewProc("CallNextHookEx")
	procToUnicodeEx              = user32.NewProc("ToUnicodeEx")
	procGetKeyboardLayout        = user32.NewProc("GetKeyboardLayout")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetKeyState              = user32.NewProc("GetKeyState")
	procVkKeyScan                = user32.NewProc("VkKeyScanW")
)

// SetKeyboardHook installe le hook clavier. Rend 0 en cas d'echec.
func SetKeyboardHook(callback uintptr) uintptr {
	handle, _, _ := procSetWindowsHookEx.Call(
		WH_KEYBOARD_LL, callback, uintptr(ModuleHandle()), 0,
	)
	return handle
}

// RemoveKeyboardHook retire le hook.
func RemoveKeyboardHook(handle uintptr) {
	if handle != 0 {
		procUnhookWindowsHookEx.Call(handle)
	}
}

// CallNextHook laisse la touche suivre son chemin normal.
func CallNextHook(handle uintptr, code int, wparam, lparam uintptr) uintptr {
	result, _, _ := procCallNextHookEx.Call(handle, uintptr(code), wparam, lparam)
	return result
}

// KeyEventAt relit la description de touche transmise au hook.
func KeyEventAt(lparam uintptr) KBDLLHOOKSTRUCT {
	pointer := *(*unsafe.Pointer)(unsafe.Pointer(&lparam))
	if pointer == nil {
		return KBDLLHOOKSTRUCT{}
	}
	return *(*KBDLLHOOKSTRUCT)(pointer)
}

// FromUs indique que la touche est l'une de nos propres frappes simulees.
// Celles-la doivent traverser sans etre retouchees, sinon elles seraient
// masquees a l'infini.
func (k KBDLLHOOKSTRUCT) FromUs() bool { return k.ExtraInfo == injectedSignature }

// IsInjected indique que la touche vient d'un programme, pas d'un vrai clavier.
func (k KBDLLHOOKSTRUCT) IsInjected() bool { return k.Flags&LLKHF_INJECTED != 0 }

// foregroundLayout rend la disposition clavier de la fenetre active : sans
// elle, un clavier francais serait lu comme un clavier americain.
func foregroundLayout() uintptr {
	window, _, _ := procGetForegroundWindow.Call()
	thread, _, _ := procGetWindowThreadProcessId.Call(window, 0)
	layout, _, _ := procGetKeyboardLayout.Call(thread)
	return layout
}

func keyToggled(key uint32) bool {
	state, _, _ := procGetKeyState.Call(uintptr(key))
	return state&1 != 0
}

// CharFromKey traduit une touche en caractere, selon la disposition clavier et
// les touches de modification enfoncees. Le deuxieme resultat est faux pour les
// touches qui ne produisent pas un caractere unique (fleches, touches mortes).
func CharFromKey(virtualKey, scanCode uint32) (rune, bool) {
	var state [256]byte
	if KeyIsDown(VK_SHIFT) {
		state[VK_SHIFT] = 0x80
	}
	if keyToggled(VK_CAPITAL) {
		state[VK_CAPITAL] = 0x01
	}
	if KeyIsDown(VK_RMENU) { // AltGr : Windows le voit comme Ctrl + Alt
		state[VK_CONTROL] = 0x80
		state[VK_MENU] = 0x80
	}

	var buffer [8]uint16
	// Le drapeau 0x4 empeche ToUnicodeEx de modifier l'etat des touches mortes.
	count, _, _ := procToUnicodeEx.Call(
		uintptr(virtualKey), uintptr(scanCode), uintptr(unsafe.Pointer(&state[0])),
		uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0x4, foregroundLayout(),
	)
	if int32(count) != 1 {
		return 0, false
	}
	return rune(buffer[0]), true
}

// SendRune ecrit un caractere dans la fenetre active, quelle que soit la
// disposition du clavier.
func SendRune(letter rune) {
	if letter > 0xFFFF { // hors du plan de base : non gere, et jamais produit ici
		return
	}
	down := INPUT{Type: INPUT_KEYBOARD}
	down.Ki.Scan = uint16(letter)
	down.Ki.Flags = KEYEVENTF_UNICODE
	down.Ki.ExtraInfo = injectedSignature

	up := down
	up.Ki.Flags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP

	SendInputs([]INPUT{down, up})
}

// SendString ecrit un texte dans la fenetre active.
func SendString(text string) {
	inputs := make([]INPUT, 0, len(text)*2)
	for _, letter := range text {
		if letter > 0xFFFF {
			continue
		}
		down := INPUT{Type: INPUT_KEYBOARD}
		down.Ki.Scan = uint16(letter)
		down.Ki.Flags = KEYEVENTF_UNICODE
		down.Ki.ExtraInfo = injectedSignature
		up := down
		up.Ki.Flags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP
		inputs = append(inputs, down, up)
	}
	SendInputs(inputs)
}

// SendKey rejoue une touche ordinaire (Entree, par exemple) que nous avons avalee.
func SendKey(virtualKey uint32) {
	down := INPUT{Type: INPUT_KEYBOARD}
	down.Ki.Vk = uint16(virtualKey)
	down.Ki.Scan = MapVirtualKey(virtualKey)
	down.Ki.ExtraInfo = injectedSignature
	up := down
	up.Ki.Flags = KEYEVENTF_KEYUP
	SendInputs([]INPUT{down, up})
}

// SendUserKeys simule une frappe ordinaire, comme si elle venait du clavier.
//
// Contrairement a SendRune et SendString, ces evenements ne portent pas notre
// signature : le hook les traite donc comme de vraies touches. C'est ce qui
// permet a l'application de se tester elle-meme.
func SendUserKeys(text string) {
	var inputs []INPUT
	press := func(virtualKey uint32, released bool) INPUT {
		event := INPUT{Type: INPUT_KEYBOARD}
		event.Ki.Vk = uint16(virtualKey)
		event.Ki.Scan = MapVirtualKey(virtualKey)
		if released {
			event.Ki.Flags = KEYEVENTF_KEYUP
		}
		return event
	}

	for _, letter := range text {
		scan, _, _ := procVkKeyScan.Call(uintptr(uint16(letter)))
		if int16(scan) == -1 {
			continue // ce caractere n'existe pas sur la disposition courante
		}
		virtualKey := uint32(scan & 0xFF)
		withShift := scan&0x100 != 0

		if withShift {
			inputs = append(inputs, press(VK_SHIFT, false))
		}
		inputs = append(inputs, press(virtualKey, false), press(virtualKey, true))
		if withShift {
			inputs = append(inputs, press(VK_SHIFT, true))
		}
	}
	SendInputs(inputs)
}

// KeyboardLayoutName rend l'identifiant de la disposition clavier active, utile
// pour comprendre un probleme de saisie.
func KeyboardLayoutName() string {
	layout := foregroundLayout()
	return fmt.Sprintf("0x%08X", uint32(layout))
}
