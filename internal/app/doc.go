// Package app assemble CryptoBulle : raccourcis clavier, presse-papiers, bulle
// d'affichage, fenetre de reglages et icone pres de l'horloge.
//
// Le coeur du programme est une unique boucle de messages Windows, sur le fil
// principal. Les actions longues (copier la selection, chiffrer, coller) partent
// dans une goroutine et renvoient leur resultat par PostMessage, ce qui garde
// l'interface toujours reactive.
package app

// Nom et version affiches dans l'interface.
const (
	appName    = "CryptoBulle"
	appVersion = "2.5.0"
)
