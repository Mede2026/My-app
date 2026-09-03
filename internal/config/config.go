// Package config lit et ecrit les reglages de CryptoBulle.
//
// Le fichier est le meme que celui de la premiere version :
// %APPDATA%\CryptoBulle\config.json. Les reglages sont donc conserves.
package config

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mede2026/cryptobulle/internal/crypto"
)

// Raccourcis proposes par defaut.
const (
	DefaultHotkeyDecrypt = "ctrl+alt+d"
	DefaultHotkeyEncrypt = "ctrl+alt+e"
	DefaultHotkeyMask    = "ctrl+alt+m"
)

// Config regroupe tous les reglages modifiables par l'utilisateur.
type Config struct {
	HotkeyDecrypt string `json:"hotkey_decrypt"`
	HotkeyEncrypt string `json:"hotkey_encrypt"`
	// Raccourci du mode frappe masquee.
	HotkeyMask string `json:"hotkey_mask"`
	// Phrase secrete protegee (voir seal) ; jamais en clair dans le fichier.
	PassphraseSealed string `json:"passphrase_sealed"`
	// Duree d'affichage de la bulle, en secondes (0 = fermeture manuelle).
	BubbleSeconds int `json:"bubble_seconds"`
	// Coller automatiquement le texte chiffré a la place de la selection.
	AutoPaste bool `json:"auto_paste"`
	// Remettre l'ancien contenu du presse-papiers apres coup.
	RestoreClipboard bool   `json:"restore_clipboard"`
	LaunchAtStartup  bool   `json:"launch_at_startup"`
	Theme            string `json:"theme"` // "sombre" ou "clair"
	// Sel personnel, tire une fois : il permet de garder la cle en cache et
	// rend le chiffrement instantane (voir crypto.Encrypt).
	KeySalt string `json:"key_salt"`
}

// Default renvoie les reglages d'usine.
func Default() Config {
	return Config{
		HotkeyDecrypt:    DefaultHotkeyDecrypt,
		HotkeyEncrypt:    DefaultHotkeyEncrypt,
		HotkeyMask:       DefaultHotkeyMask,
		BubbleSeconds:    12,
		AutoPaste:        true,
		RestoreClipboard: true,
		Theme:            "sombre",
	}
}

// Dir est le dossier des reglages.
func Dir() string {
	if override := os.Getenv("CRYPTOBULLE_HOME"); override != "" {
		return override
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "CryptoBulle")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "cryptobulle")
}

// Path est le chemin complet du fichier de reglages.
func Path() string { return filepath.Join(Dir(), "config.json") }

// Load lit les reglages. Un fichier absent ou abime redonne les valeurs
// d'usine plutot qu'une erreur : l'application doit toujours pouvoir demarrer.
func Load() Config {
	config := Default()
	data, err := os.ReadFile(Path())
	if err != nil {
		return config
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return Default()
	}
	if config.HotkeyDecrypt == "" {
		config.HotkeyDecrypt = DefaultHotkeyDecrypt
	}
	if config.HotkeyEncrypt == "" {
		config.HotkeyEncrypt = DefaultHotkeyEncrypt
	}
	if config.HotkeyMask == "" {
		config.HotkeyMask = DefaultHotkeyMask
	}
	if config.Theme == "" {
		config.Theme = "sombre"
	}
	return config
}

// Save ecrit les reglages sur le disque, sans risque de fichier a moitie ecrit.
func (c *Config) Save() error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	temporary := Path() + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, Path()) // remplacement atomique
}

// --- phrase secrete ---------------------------------------------------------

// Passphrase renvoie la phrase secrete en clair (vide si illisible).
func (c *Config) Passphrase() string { return unseal(c.PassphraseSealed) }

// SetPassphrase range la phrase secrete sous forme protegee.
func (c *Config) SetPassphrase(passphrase string) {
	if passphrase == "" {
		c.PassphraseSealed = ""
		return
	}
	c.PassphraseSealed = seal(passphrase)
}

// HasPassphrase indique si une phrase secrete est enregistree.
func (c *Config) HasPassphrase() bool { return c.PassphraseSealed != "" }

// SecureStorage indique si le systeme protege reellement la phrase secrete.
func SecureStorage() bool { return storageIsSecure() }

// --- sel personnel ----------------------------------------------------------

// Salt renvoie le sel de l'utilisateur, cree au premier appel.
// L'appelant doit enregistrer les reglages si le sel vient d'etre cree.
func (c *Config) Salt() []byte {
	raw, err := hex.DecodeString(c.KeySalt)
	if err != nil || len(raw) != crypto.SaltLen {
		raw = crypto.NewSalt()
		c.KeySalt = hex.EncodeToString(raw)
	}
	return raw
}
