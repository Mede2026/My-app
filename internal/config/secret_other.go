//go:build !windows

package config

import (
	"encoding/base64"
	"strings"
)

// Hors Windows (developpement et tests), il n'y a pas de coffre systeme : on se
// contente d'un encodage, ce qui n'est PAS de la securite. SecureStorage()
// renvoie false pour que l'interface le dise clairement a l'utilisateur.

func storageIsSecure() bool { return false }

func seal(secret string) string {
	return "plain:" + base64.StdEncoding.EncodeToString([]byte(secret))
}

func unseal(stored string) string {
	scheme, payload, found := strings.Cut(stored, ":")
	if !found || scheme != "plain" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}
