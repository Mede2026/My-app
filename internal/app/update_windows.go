package app

import (
	"time"

	"github.com/mede2026/cryptobulle/internal/update"
)

// Mises a jour : l'application demande a GitHub la derniere version publiee,
// et propose de l'installer. C'est le seul moment ou elle se connecte a
// Internet, et cela peut se couper dans les reglages.

// checkUpdates interroge GitHub. `demande` distingue la verification lancee par
// l'utilisateur, qui merite toujours une reponse, de celle du demarrage, qui
// reste silencieuse quand tout va bien.
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
	a.pending = &release
	a.mu.Unlock()

	notes := release.Notes
	if notes != "" {
		notes = "\n\n" + notes
	}
	a.showBubble("Version "+release.Version+" disponible",
		"Clic droit sur l'icône près de l'horloge, puis « Installer la mise à jour »."+notes,
		kindInfo, 12)
}

// availableUpdate rend la version en attente d'installation, ou nil.
func (a *App) availableUpdate() *update.Release {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pending
}

// installUpdate remplace l'executable puis quitte : la nouvelle version prend
// le relais immediatement.
func (a *App) installUpdate() {
	release := a.availableUpdate()
	if release == nil {
		a.trigger(func() { a.checkUpdates(true) })
		return
	}

	a.showBubble("Téléchargement en cours",
		"Version "+release.Version+", l'application redémarrera toute seule.", kindInfo, 8)

	a.trigger(func() {
		if err := update.Install(*release, appVersion); err != nil {
			a.showBubble("Mise à jour impossible", capitalize(err.Error()), kindError, -1)
			return
		}
		// La nouvelle version est lancee : celle-ci doit liberer la place.
		a.post(func() {
			time.Sleep(300 * time.Millisecond)
			a.quit()
		})
	})
}
