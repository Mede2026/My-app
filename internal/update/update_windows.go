package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mede2026/cryptobulle/internal/w32"
)

// Mise a jour de l'executable lui-meme.
//
// Windows interdit d'ecraser un fichier en cours d'execution, mais autorise a
// le renommer. La manoeuvre est donc :
//
//	CryptoBulle.exe      -> CryptoBulle.exe.ancien
//	CryptoBulle.exe.neuf -> CryptoBulle.exe
//
// puis on lance la nouvelle version et on quitte. Le fichier « ancien » est
// efface au demarrage suivant.

const (
	suffixNew = ".neuf"
	suffixOld = ".ancien"
)

// Check demande a GitHub la derniere version publiee.
//
// Le second resultat est vrai si elle est plus recente que celle qui tourne.
func Check(currentVersion string) (Release, bool, error) {
	data, err := w32.HTTPGet(LatestURL, "CryptoBulle/"+currentVersion,
		"application/vnd.github+json")
	if err != nil {
		return Release{}, false, err
	}
	release, err := ParseRelease(data, runtime.GOARCH)
	if err != nil {
		return Release{}, false, err
	}
	return release, Newer(release.Version, currentVersion), nil
}

// Install telecharge la nouvelle version, la verifie, prend la place de
// l'ancienne et relance l'application. L'appelant doit ensuite quitter.
func Install(release Release, currentVersion string) error {
	if release.Asset.URL == "" {
		return ErrNoAsset
	}

	binary, err := w32.HTTPGet(release.Asset.URL, "CryptoBulle/"+currentVersion,
		"application/octet-stream")
	if err != nil {
		return err
	}
	if err := verify(binary, release.Asset); err != nil {
		return err
	}

	executable, err := os.Executable()
	if err != nil {
		return errors.New("impossible de retrouver l'exécutable en cours")
	}

	nouveau, ancien := executable+suffixNew, executable+suffixOld
	if err := os.WriteFile(nouveau, binary, 0o755); err != nil {
		return fmt.Errorf("écriture impossible dans %s : le dossier est-il protégé ?",
			trimPath(executable))
	}
	_ = os.Remove(ancien)
	if err := os.Rename(executable, ancien); err != nil {
		_ = os.Remove(nouveau)
		return errors.New("remplacement refusé par Windows : déplacez CryptoBulle " +
			"dans un dossier où vous pouvez écrire, par exemple le Bureau")
	}
	if err := os.Rename(nouveau, executable); err != nil {
		_ = os.Rename(ancien, executable) // on remet tout comme avant
		return errors.New("remplacement interrompu, rien n'a été modifié")
	}

	if err := exec.Command(executable).Start(); err != nil {
		return errors.New("la nouvelle version est installée, mais n'a pas pu être lancée")
	}
	return nil
}

// verify controle la taille et l'empreinte annoncees par GitHub.
func verify(binary []byte, asset Asset) error {
	if asset.Size > 0 && int64(len(binary)) != asset.Size {
		return errors.New("fichier téléchargé incomplet")
	}
	if asset.SHA256 == "" {
		return nil // GitHub ne publie pas toujours l'empreinte
	}
	sum := sha256.Sum256(binary)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), asset.SHA256) {
		return errors.New("le fichier téléchargé ne correspond pas à celui annoncé")
	}
	return nil
}

// CleanupOld efface la version precedente, laissee sur le disque le temps que
// l'ancienne application se termine.
func CleanupOld() {
	if executable, err := os.Executable(); err == nil {
		_ = os.Remove(executable + suffixOld)
	}
}

func trimPath(path string) string {
	if cut := strings.LastIndexAny(path, `\/`); cut >= 0 {
		return path[:cut]
	}
	return path
}
