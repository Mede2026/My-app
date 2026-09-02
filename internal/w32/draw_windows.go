package w32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// Dessin fin : coins arrondis lisses, sans bibliotheque graphique.
//
// Windows sait remplir un rectangle, pas un rectangle aux coins arrondis avec
// des bords lisses. GDI+ le ferait, mais ses fonctions attendent des nombres a
// virgule, que Go ne peut pas transmettre a une DLL sur les processeurs 64 bits.
//
// On dessine donc nous-memes dans une petite image en memoire : pour chaque
// pixel, on regarde quelle part est a l'interieur de la forme (echantillonnage
// en 4x4) et on melange la couleur du bouton avec celle du fond. Le resultat est
// parfaitement lisse, et le cout est negligeable a ces tailles.

const (
	CS_DROPSHADOW  = 0x00020000
	WM_DPICHANGED  = 0x02E0
	SRCCOPY        = 0x00CC0020
	BI_RGB         = 0
	DIB_RGB_COLORS = 0

	// Coins arrondis natifs de Windows 11.
	dwmaWindowCornerPreference = 33
	dwmCornerRound             = 2
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

var (
	gdi32DIB               = gdi32.NewProc("CreateDIBSection")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procBitBlt             = gdi32.NewProc("BitBlt")

	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	comctl32                 = windows.NewLazySystemDLL("comctl32.dll")
	procInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
)

// InitCommonControls charge la version moderne des controles Windows.
// Le manifeste de l'application demande deja la version 6 ; cet appel garantit
// que la bibliotheque est bien initialisee avant la premiere fenetre.
func InitCommonControls() {
	var controls struct {
		Size uint32
		ICC  uint32
	}
	controls.Size = uint32(unsafe.Sizeof(controls))
	controls.ICC = 0x0000FFFF // toutes les classes standard
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&controls)))
}

// RoundWindowCorners demande a Windows 11 d'arrondir les coins de la fenetre.
// Sans effet sur Windows 10, ou la fenetre garde ses coins droits.
func RoundWindowCorners(hwnd HWND) {
	preference := int32(dwmCornerRound)
	procDwmSetWindowAttribute.Call(
		uintptr(hwnd), dwmaWindowCornerPreference,
		uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference),
	)
}

// splitColor separe une couleur Windows (COLORREF, ordre bleu-vert-rouge).
func splitColor(color uintptr) (red, green, blue float64) {
	return float64(color & 0xFF), float64((color >> 8) & 0xFF), float64((color >> 16) & 0xFF)
}

// coverage indique quelle proportion du pixel (x, y) est a l'interieur du
// rectangle arrondi, entre 0 et 1.
func coverage(x, y, width, height, radius float64) float64 {
	const samples = 4
	inside := 0.0
	for subY := 0; subY < samples; subY++ {
		for subX := 0; subX < samples; subX++ {
			pointX := x + (float64(subX)+0.5)/samples
			pointY := y + (float64(subY)+0.5)/samples

			// On ramene le point dans le quart de coin le plus proche.
			nearestX := pointX
			if pointX < radius {
				nearestX = radius
			} else if pointX > width-radius {
				nearestX = width - radius
			}
			nearestY := pointY
			if pointY < radius {
				nearestY = radius
			} else if pointY > height-radius {
				nearestY = height - radius
			}

			deltaX, deltaY := pointX-nearestX, pointY-nearestY
			if deltaX*deltaX+deltaY*deltaY <= radius*radius {
				inside++
			}
		}
	}
	return inside / (samples * samples)
}

// FillRoundRect peint un rectangle aux coins arrondis et aux bords lisses.
// `background` est la couleur deja presente derriere : elle sert au melange.
func FillRoundRect(dc HDC, rect RECT, radius int32, color, background uintptr) {
	width, height := rect.Width(), rect.Height()
	if width <= 0 || height <= 0 {
		return
	}
	if maximum := min32(width, height) / 2; radius > maximum {
		radius = maximum
	}

	memoryDC, _, _ := procCreateCompatibleDC.Call(uintptr(dc))
	if memoryDC == 0 {
		return
	}
	defer procDeleteDC.Call(memoryDC)

	header := bitmapInfoHeader{
		Width: width, Height: -height, // hauteur negative : premiere ligne en haut
		Planes: 1, BitCount: 32, Compression: BI_RGB,
	}
	header.Size = uint32(unsafe.Sizeof(header))

	var pixels unsafe.Pointer
	bitmap, _, _ := gdi32DIB.Call(
		memoryDC, uintptr(unsafe.Pointer(&header)), DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&pixels)), 0, 0,
	)
	if bitmap == 0 || pixels == nil {
		return
	}
	defer DeleteObject(bitmap)

	shapeRed, shapeGreen, shapeBlue := splitColor(color)
	backRed, backGreen, backBlue := splitColor(background)
	buffer := unsafe.Slice((*uint32)(pixels), int(width*height))
	radiusF := float64(radius)
	widthF, heightF := float64(width), float64(height)

	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			part := coverage(float64(x), float64(y), widthF, heightF, radiusF)
			red := backRed + (shapeRed-backRed)*part
			green := backGreen + (shapeGreen-backGreen)*part
			blue := backBlue + (shapeBlue-backBlue)*part
			// Format du DIB : 0x00RRGGBB
			buffer[y*width+x] = uint32(red+0.5)<<16 | uint32(green+0.5)<<8 | uint32(blue+0.5)
		}
	}

	previous := SelectObject(HDC(memoryDC), bitmap)
	procBitBlt.Call(
		uintptr(dc), uintptr(rect.Left), uintptr(rect.Top), uintptr(width), uintptr(height),
		memoryDC, 0, 0, SRCCOPY,
	)
	SelectObject(HDC(memoryDC), previous)
}

// RectAt relit un rectangle transmis par Windows dans un message (WM_DPICHANGED).
func RectAt(address uintptr) RECT {
	pointer := *(*unsafe.Pointer)(unsafe.Pointer(&address))
	if pointer == nil {
		return RECT{}
	}
	return *(*RECT)(pointer)
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// Blend melange deux couleurs Windows. `ratio` va de 0 (premiere couleur) a 1.
func Blend(first, second uintptr, ratio float64) uintptr {
	firstRed, firstGreen, firstBlue := splitColor(first)
	secondRed, secondGreen, secondBlue := splitColor(second)
	mix := func(a, b float64) uintptr {
		return uintptr(a + (b-a)*ratio + 0.5)
	}
	return mix(firstRed, secondRed) | mix(firstGreen, secondGreen)<<8 | mix(firstBlue, secondBlue)<<16
}
