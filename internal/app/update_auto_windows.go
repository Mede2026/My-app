//go:build autoupdate

package app

import (
	"time"

	"github.com/mede2026/cryptobulle/internal/update"
)

// Variante compilee avec la mise a jour entierement automatique : elle
// telecharge le nouvel executable, verifie son empreinte, prend la place de
// l'ancien et redemarre.
//
// C'est le comportement qui inquiete Windows Defender. Prenez cette variante
// seulement si vous avez ajoute CryptoBulle aux exclusions de l'antivirus.

// updateActionLabel decrit ce que fait l'entree du menu dans cette variante.
const updateActionLabel = "Installer la mise à jour"

func (a *App) checkUpdates(demande bool) {
	release, plusRecente, err := update.Check(appVersion)
	if err != nil {
		if demande {
			a.showBubble("Vérification impossible", capitalize(err.Error()), kindError, -1)
		}
		return
	}
	if !plusRecente {
		if demande {
			a.showBubble("Aucune mise à jour",
				"Vous utilisez déjà la version "+appVersion+".", kindInfo, 5)
		}
		return
	}

	a.mu.Lock()
	a.latest = &release
	a.pending = &pendingRelease{version: release.Version, notes: release.Notes}
	a.mu.Unlock()

	a.showBubble("Version "+release.Version+" disponible",
		"Clic droit sur l'icône près de l'horloge, puis « Installer la mise à jour ».",
		kindInfo, 12)
}

// availableUpdate rend la version en attente d'installation, ou nil.
func (a *App) availableUpdate() *pendingRelease {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pending
}

// cleanupAfterUpdate efface l'executable precedent, laisse en place le temps
// que l'ancienne version se termine.
func cleanupAfterUpdate() { update.CleanupOld() }

// installUpdate remplace l'executable puis quitte : la nouvelle version prend
// le relais immediatement.
func (a *App) installUpdate() {
	a.mu.RLock()
	latest, _ := a.latest.(*update.Release)
	a.mu.RUnlock()
	if latest == nil {
		a.trigger(func() { a.checkUpdates(true) })
		return
	}
	release := *latest

	a.showBubble("Téléchargement en cours",
		"Version "+release.Version+", l'application redémarrera toute seule.", kindInfo, 8)

	a.trigger(func() {
		if err := update.Install(release, appVersion); err != nil {
			a.showBubble("Mise à jour impossible", capitalize(err.Error()), kindError, -1)
			return
		}
		a.post(func() {
			time.Sleep(300 * time.Millisecond)
			a.quit()
		})
	})
}
