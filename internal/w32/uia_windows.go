package w32

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Lecture du texte sous le curseur, par l'automatisation d'interface de Windows
// (UI Automation).
//
// C'est le mécanisme prévu pour les lecteurs d'écran : il permet de demander à
// une application quel texte se trouve à un endroit donné. Il fonctionne dans
// les logiciels ordinaires comme dans les navigateurs et Discord, sans cliquer,
// sans rien sélectionner et sans toucher au presse-papiers.
//
// Le dialogue passe par COM, où chaque objet expose ses fonctions dans un
// tableau de pointeurs, la « vtable ». On appelle donc par numéro d'emplacement.
// Tout est vérifié : au moindre doute, la fonction rend une chaîne vide et
// l'application retombe sur la sélection.

const (
	coinitMultithreaded = 0
	clsctxInprocServer  = 1

	uiaTextPatternID  = 10014
	textUnitLine      = 3
	textUnitParagraph = 4

	// Emplacements dans les tableaux de fonctions COM.
	vtblRelease               = 2
	vtblElementFromPoint      = 7  // IUIAutomation
	vtblGetCurrentPattern     = 16 // IUIAutomationElement
	vtblRangeFromPoint        = 3  // IUIAutomationTextPattern
	vtblExpandToEnclosingUnit = 6  // IUIAutomationTextRange
	vtblGetText               = 12 // IUIAutomationTextRange
)

// Identifiants du service d'automatisation.
var (
	clsidCUIAutomation = windows.GUID{
		Data1: 0xff48dba4, Data2: 0x60ef, Data3: 0x4201,
		Data4: [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e},
	}
	iidIUIAutomation = windows.GUID{
		Data1: 0x30cbe57d, Data2: 0xd9d0, Data3: 0x452a,
		Data4: [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee},
	}

	oleaut32          = windows.NewLazySystemDLL("oleaut32.dll")
	procSysFreeString = oleaut32.NewProc("SysFreeString")

	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
)

// comCall appelle la fonction numéro `slot` de l'objet `object`.
//
// Les adresses viennent de Windows, pas du ramasse-miettes de Go : la
// conversion passe par pointerFromAddress, qui évite une fausse alerte de
// « go vet » sans rien changer au résultat.
func comCall(object uintptr, slot int, args ...uintptr) uintptr {
	table := *(*uintptr)(pointerFromAddress(object))
	slotAddress := table + uintptr(slot)*unsafe.Sizeof(uintptr(0))
	method := *(*uintptr)(pointerFromAddress(slotAddress))
	result, _, _ := syscall.SyscallN(method, append([]uintptr{object}, args...)...)
	return result
}

func comRelease(object uintptr) {
	if object != 0 {
		comCall(object, vtblRelease)
	}
}

// packPoint place les deux coordonnées dans un seul mot, comme le fait le
// compilateur C quand il passe une structure POINT par valeur.
func packPoint(point POINT) uintptr {
	return uintptr(uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32)
}

// TextAtPoint rend le texte de la ligne située sous le point donné, ou "" si
// l'application ne le publie pas.
func TextAtPoint(point POINT) string {
	// COM travaille par fil d'exécution : celui-ci ne doit pas changer en route.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Une erreur ici signifie le plus souvent que le fil est déjà préparé dans
	// un autre mode, ce qui n'empêche pas la suite. On ne referme que ce que
	// l'on a soi-même ouvert.
	if windows.CoInitializeEx(0, coinitMultithreaded) == nil {
		defer windows.CoUninitialize()
	}

	var automation uintptr
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidCUIAutomation)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIUIAutomation)), uintptr(unsafe.Pointer(&automation)),
	)
	if hr != 0 || automation == 0 {
		return ""
	}
	defer comRelease(automation)

	packed := packPoint(point)

	var element uintptr
	if comCall(automation, vtblElementFromPoint, packed,
		uintptr(unsafe.Pointer(&element))) != 0 || element == 0 {
		return ""
	}
	defer comRelease(element)

	// GetCurrentPattern réussit même quand le motif n'existe pas : il rend
	// alors un objet nul, qu'il faut écarter avant de s'en servir.
	var pattern uintptr
	if comCall(element, vtblGetCurrentPattern, uiaTextPatternID,
		uintptr(unsafe.Pointer(&pattern))) != 0 || pattern == 0 {
		return ""
	}
	defer comRelease(pattern)

	var textRange uintptr
	if comCall(pattern, vtblRangeFromPoint, packed,
		uintptr(unsafe.Pointer(&textRange))) != 0 || textRange == 0 {
		return ""
	}
	defer comRelease(textRange)

	// La ligne d'abord ; le paragraphe si elle ne donne rien.
	for _, unit := range []uintptr{textUnitLine, textUnitParagraph} {
		if comCall(textRange, vtblExpandToEnclosingUnit, unit) != 0 {
			continue
		}
		if text := rangeText(textRange); text != "" {
			return text
		}
	}
	return ""
}

// rangeText lit le texte d'une plage. Windows rend une chaîne BSTR, qu'il faut
// libérer après lecture.
func rangeText(textRange uintptr) string {
	var text *uint16
	// -1 : aucune limite de longueur.
	if comCall(textRange, vtblGetText, ^uintptr(0), uintptr(unsafe.Pointer(&text))) != 0 {
		return ""
	}
	if text == nil {
		return ""
	}
	defer procSysFreeString.Call(uintptr(unsafe.Pointer(text)))
	return windows.UTF16PtrToString(text)
}
