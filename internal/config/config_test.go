package config

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mede2026/cryptobulle/internal/crypto"
)

func useTempDir(t *testing.T) {
	t.Helper()
	t.Setenv("CRYPTOBULLE_HOME", t.TempDir())
}

func TestValeursParDefaut(t *testing.T) {
	useTempDir(t)
	config := Load()
	if config.HotkeyDecrypt != "ctrl+alt+d" || config.HotkeyEncrypt != "ctrl+alt+e" {
		t.Fatalf("raccourcis par defaut : %+v", config)
	}
	if config.HasPassphrase() {
		t.Fatal("aucune phrase secrete ne devrait exister au depart")
	}
	if !config.AutoPaste || !config.RestoreClipboard || config.BubbleSeconds != 12 {
		t.Fatalf("reglages par defaut inattendus : %+v", config)
	}
}

func TestEcritureEtRelecture(t *testing.T) {
	useTempDir(t)
	config := Default()
	config.SetPassphrase("ski au Massif")
	config.BubbleSeconds = 5
	if err := config.Save(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Massif") {
		t.Fatal("la phrase secrete se retrouve en clair dans le fichier")
	}

	reloaded := Load()
	if reloaded.Passphrase() != "ski au Massif" {
		t.Fatalf("phrase secrete relue : %q", reloaded.Passphrase())
	}
	if reloaded.BubbleSeconds != 5 {
		t.Fatalf("duree relue : %d", reloaded.BubbleSeconds)
	}
}

func TestFichierAbimeRedonneLesDefauts(t *testing.T) {
	useTempDir(t)
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte("{ ceci n'est pas du JSON"), 0o600); err != nil {
		t.Fatal(err)
	}
	if Load().HotkeyDecrypt != "ctrl+alt+d" {
		t.Fatal("un fichier abime devrait redonner les reglages d'usine")
	}
}

func TestReglagesInconnusIgnores(t *testing.T) {
	useTempDir(t)
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"hotkey_encrypt": "ctrl+f9", "vieux_reglage": 1}`
	if err := os.WriteFile(Path(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config := Load()
	if config.HotkeyEncrypt != "ctrl+f9" {
		t.Fatalf("reglage connu perdu : %q", config.HotkeyEncrypt)
	}
	if config.HotkeyDecrypt != "ctrl+alt+d" {
		t.Fatal("les reglages absents devraient reprendre la valeur d'usine")
	}
}

func TestSelPersonnelStable(t *testing.T) {
	useTempDir(t)
	config := Default()
	first := config.Salt()
	if len(first) != crypto.SaltLen {
		t.Fatalf("taille du sel : %d", len(first))
	}
	if hex.EncodeToString(first) != config.KeySalt {
		t.Fatal("le sel n'a pas ete conserve dans les reglages")
	}
	if second := config.Salt(); string(second) != string(first) {
		t.Fatal("le sel change d'un appel a l'autre")
	}
}

func TestDossierParDefaut(t *testing.T) {
	t.Setenv("CRYPTOBULLE_HOME", "")
	if Dir() == "" || filepath.Base(Path()) != "config.json" {
		t.Fatalf("chemin inattendu : %q", Path())
	}
}
