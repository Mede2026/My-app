package update

import "testing"

func TestComparaisonDeVersions(t *testing.T) {
	plusRecentes := [][2]string{
		{"3.0.0", "2.9.0"},
		{"2.10.0", "2.9.9"},
		{"2.9.1", "2.9.0"},
		{"v3.0.0", "2.9.0"},
		{"1.0.0", "dev"},
	}
	for _, paire := range plusRecentes {
		if !Newer(paire[0], paire[1]) {
			t.Fatalf("%q devrait être plus récente que %q", paire[0], paire[1])
		}
	}

	pasPlusRecentes := [][2]string{
		{"2.9.0", "2.9.0"},
		{"2.9.0", "3.0.0"},
		{"2.9.0", "2.9.1"},
		{"2.9.0-beta", "2.9.0"},
	}
	for _, paire := range pasPlusRecentes {
		if Newer(paire[0], paire[1]) {
			t.Fatalf("%q ne devrait pas être plus récente que %q", paire[0], paire[1])
		}
	}
}

const exemple = `{
  "tag_name": "v3.1.0",
  "name": "CryptoBulle 3.1.0",
  "body": "Corrige la frappe masquée.",
  "draft": false,
  "prerelease": false,
  "assets": [
    {"name": "CryptoBulle-arm64.exe", "browser_download_url": "https://example.com/arm.exe",
     "size": 2800000, "digest": "sha256:aaaa"},
    {"name": "CryptoBulle.exe", "browser_download_url": "https://example.com/amd.exe",
     "size": 2900000, "digest": "sha256:bbbb"}
  ]
}`

func TestLectureDeLaPublication(t *testing.T) {
	release, err := ParseRelease([]byte(exemple), "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "3.1.0" {
		t.Fatalf("version lue : %q", release.Version)
	}
	if release.Notes != "Corrige la frappe masquée." {
		t.Fatalf("notes lues : %q", release.Notes)
	}
	if release.Asset.Name != "CryptoBulle.exe" || release.Asset.SHA256 != "bbbb" {
		t.Fatalf("fichier choisi : %+v", release.Asset)
	}

	surArm, err := ParseRelease([]byte(exemple), "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if surArm.Asset.Name != "CryptoBulle-arm64.exe" {
		t.Fatalf("fichier choisi sur ARM : %+v", surArm.Asset)
	}
}

func TestPublicationSansExecutable(t *testing.T) {
	if _, err := ParseRelease([]byte(`{"tag_name":"v1.0.0","assets":[]}`), "amd64"); err != ErrNoAsset {
		t.Fatalf("erreur attendue ErrNoAsset, obtenu %v", err)
	}
	if _, err := ParseRelease([]byte("pas du JSON"), "amd64"); err == nil {
		t.Fatal("une réponse illisible devrait être refusée")
	}
}
