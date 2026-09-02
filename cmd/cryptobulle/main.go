// Commande cryptobulle : chiffrer et dechiffrer du texte partout dans Windows,
// avec deux raccourcis clavier et une bulle d'affichage.
package main

import (
	"fmt"
	"os"

	"github.com/mede2026/cryptobulle/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
