package config

import (
	"encoding/base64"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Sous Windows, la phrase secrete est chiffree par DPAPI (Data Protection API),
// le coffre integre au systeme. Le secret est lie au compte Windows : le meme
// fichier copie ailleurs, ou lu par un autre compte, ne donne rien.

func protect(data []byte, encrypt bool) ([]byte, bool) {
	if len(data) == 0 {
		return nil, false
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	name, _ := windows.UTF16PtrFromString("CryptoBulle")

	var err error
	if encrypt {
		err = windows.CryptProtectData(&input, name, nil, 0, nil, 0, &output)
	} else {
		err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, 0, &output)
	}
	if err != nil {
		return nil, false
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))

	result := make([]byte, output.Size)
	copy(result, unsafe.Slice(output.Data, output.Size))
	return result, true
}

func storageIsSecure() bool {
	_, ok := protect([]byte("test"), true)
	return ok
}

func seal(secret string) string {
	if sealed, ok := protect([]byte(secret), true); ok {
		return "dpapi:" + base64.StdEncoding.EncodeToString(sealed)
	}
	return "plain:" + base64.StdEncoding.EncodeToString([]byte(secret))
}

func unseal(stored string) string {
	scheme, payload, found := strings.Cut(stored, ":")
	if !found {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	switch scheme {
	case "dpapi":
		if opened, ok := protect(raw, false); ok {
			return string(opened)
		}
	case "plain":
		return string(raw)
	}
	return ""
}
