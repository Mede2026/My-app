//go:build !windows

package app

import "errors"

// Run n'existe que pour permettre la compilation hors Windows : les raccourcis
// globaux, DPAPI et l'icone de notification n'ont pas d'equivalent portable.
func Run() error {
	return errors.New("CryptoBulle fonctionne uniquement sous Windows")
}
