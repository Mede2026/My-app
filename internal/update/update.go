// Package update compare les versions et lit la reponse de GitHub.
//
// Cette partie ne depend d'aucune fonction de Windows : elle est donc testee
// comme le reste. Le telechargement et le remplacement de l'executable vivent
// dans update_windows.go.
package update

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Repository est le depot d'ou viennent les mises a jour.
const Repository = "Mede2026/My-app"

// LatestURL est l'adresse qui decrit la derniere version publiee.
const LatestURL = "https://api.github.com/repos/" + Repository + "/releases/latest"

// PageURL est la page a ouvrir dans le navigateur pour telecharger a la main.
const PageURL = "https://github.com/" + Repository + "/releases/latest"

// Release decrit une version publiee.
type Release struct {
	Version string // « 3.0.1 », sans le v
	Notes   string
	Asset   Asset
}

// Asset est le fichier a telecharger pour cette version.
type Asset struct {
	Name string
	URL  string
	Size int64
	// SHA256 vient de GitHub quand il le publie : il permet de verifier que le
	// fichier telecharge est bien celui annonce.
	SHA256 string
}

// ErrNoAsset signale une version publiee sans executable utilisable.
var ErrNoAsset = errors.New("cette version ne contient pas d'exécutable pour Windows")

// reponse de l'interface de GitHub, reduite a ce qui nous sert.
type githubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

// ParseRelease lit la reponse de GitHub et retient l'executable qui correspond
// a l'architecture demandee (« amd64 » ou « arm64 »).
func ParseRelease(data []byte, architecture string) (Release, error) {
	var raw githubRelease
	if err := json.Unmarshal(data, &raw); err != nil {
		return Release{}, errors.New("réponse de GitHub illisible")
	}
	release := Release{
		Version: strings.TrimPrefix(raw.TagName, "v"),
		Notes:   strings.TrimSpace(raw.Body),
	}
	if release.Version == "" {
		return Release{}, errors.New("aucune version publiée")
	}

	for _, asset := range raw.Assets {
		if !strings.HasSuffix(asset.Name, ".exe") {
			continue
		}
		// « CryptoBulle-arm64.exe » pour l'ARM, « CryptoBulle.exe » sinon.
		forArm := strings.Contains(asset.Name, "arm64")
		if forArm != (architecture == "arm64") {
			continue
		}
		release.Asset = Asset{
			Name:   asset.Name,
			URL:    asset.URL,
			Size:   asset.Size,
			SHA256: strings.TrimPrefix(asset.Digest, "sha256:"),
		}
		return release, nil
	}
	return release, ErrNoAsset
}

// Newer indique si `candidate` est plus recente que `current`.
//
// Les versions s'ecrivent « 3.1.2 ». Une version inconnue, comme celle d'une
// compilation locale, est toujours consideree comme depassee : cela permet
// d'essayer la mise a jour plutot que de la refuser.
func Newer(candidate, current string) bool {
	left, right := parseVersion(candidate), parseVersion(current)
	for index := range left {
		switch {
		case left[index] > right[index]:
			return true
		case left[index] < right[index]:
			return false
		}
	}
	return false
}

func parseVersion(version string) [3]int {
	var numbers [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	for index, part := range strings.SplitN(version, ".", 3) {
		if index > 2 {
			break
		}
		// « 1.2.3-beta » : on ne garde que les chiffres du debut.
		digits := part
		if cut := strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }); cut >= 0 {
			digits = part[:cut]
		}
		numbers[index], _ = strconv.Atoi(digits)
	}
	return numbers
}
