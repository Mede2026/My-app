// Package w32 regroupe les appels aux API de Windows dont CryptoBulle a besoin.
//
// Le contenu n'existe que sur Windows (fichiers *_windows.go) ; ce fichier
// permet simplement au paquet d'exister sur les autres systemes, pour que
// « go build ./... » fonctionne partout.
package w32
