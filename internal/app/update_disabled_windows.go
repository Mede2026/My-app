//go:build noupdate

package app

// Variante compilee sans la mise a jour automatique.
//
// Telecharger un executable et remplacer le sien est le comportement type d'un
// logiciel malveillant : certains antivirus s'en alarment, meme quand tout est
// honnete. Cette version ne contient donc ni telechargement ni remplacement, et
// il faut passer par la page des publications pour changer de version.

// updateActionLabel decrit ce que fait l'entree du menu dans cette variante.
const updateActionLabel = "Mise à jour"

// availableUpdate ne rend jamais rien : cette version ne cherche pas de mise a jour.
func (a *App) availableUpdate() *pendingRelease { return nil }

func cleanupAfterUpdate() {}

func (a *App) checkUpdates(demande bool) {
	if !demande {
		return
	}
	a.showBubble("Mise à jour manuelle",
		"Cette version ne se met pas à jour toute seule.\n"+
			"Rendez-vous sur la page des publications du dépôt pour prendre la dernière.",
		kindInfo, 10)
}

func (a *App) installUpdate() { a.checkUpdates(true) }
