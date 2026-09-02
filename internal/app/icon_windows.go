package app

import (
	_ "embed"
	"encoding/binary"

	"github.com/mede2026/cryptobulle/internal/w32"
)

// L'icone est integree au binaire : aucun fichier a livrer a cote.
//
//go:embed assets/icon.ico
var iconFile []byte

// loadIcon extrait du fichier .ico l'image la plus proche de la taille voulue
// et la transforme en icone utilisable par Windows.
func loadIcon(size int32) w32.HICON {
	image := bestImage(size)
	if image == nil {
		return 0
	}
	return w32.CreateIconFromICO(image, size, size)
}

// bestImage lit l'en-tete du fichier .ico et rend l'image la mieux adaptee.
func bestImage(wanted int32) []byte {
	const headerLen, entryLen = 6, 16
	if len(iconFile) < headerLen {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(iconFile[4:6]))

	var best []byte
	bestDistance := int32(1 << 30)
	for index := 0; index < count; index++ {
		start := headerLen + index*entryLen
		if start+entryLen > len(iconFile) {
			break
		}
		entry := iconFile[start : start+entryLen]
		width := int32(entry[0])
		if width == 0 {
			width = 256 // 0 signifie 256 dans ce format
		}
		size := int32(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int32(binary.LittleEndian.Uint32(entry[12:16]))
		if offset < 0 || size <= 0 || int(offset+size) > len(iconFile) {
			continue
		}
		distance := width - wanted
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestDistance = distance
			best = iconFile[offset : offset+size]
		}
	}
	return best
}
