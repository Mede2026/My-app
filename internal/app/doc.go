// Package app assemble CryptoBulle : raccourcis clavier, presse-papiers, bulle
// d'affichage, fenetre de reglages et icone pres de l'horloge.
//
// Le coeur du programme est une unique boucle de messages Windows, sur le fil
// principal. Les actions longues (copier la selection, chiffrer, coller) partent
// dans une goroutine et renvoient leur resultat par PostMessage, ce qui garde
// l'interface toujours reactive.
package app

// appName est le nom affiche partout dans l'interface.
const appName = "CryptoBulle"

// appVersion est remplacee a la compilation par le numero de la publication
// (voir scripts/build.sh et le workflow GitHub). La valeur ecrite ici sert aux
// constructions locales.
var appVersion = "3.4.0"

// pendingRelease decrit une version plus recente reperee sur GitHub.
//
// Le type vit ici, et non dans le paquet update, pour que la version compilee
// sans la mise a jour automatique n'embarque ni le telechargement ni le
// remplacement de l'executable.
type pendingRelease struct {
	version string
	notes   string
}
