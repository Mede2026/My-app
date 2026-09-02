// Package crypto contient le format « MC1 », propre a CryptoBulle.
//
// Un message chiffre ressemble a : MC1~<charge utile encodee>
//
// La charge utile binaire est :
//
//	MAGIC(3) | VERSION(1) | SEL(16) | NONCE(12) | CHIFFRE+SIGNATURE(...)
//
// La cle vient de scrypt (phrase secrete + « poivre » propre a l'application),
// le chiffrement est AES-256-GCM, et l'encodage final utilise un alphabet
// base64 permute, specifique a CryptoBulle. Le format est identique a celui de
// la premiere version : les anciens messages restent lisibles.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/crypto/scrypt"
)

const (
	// Prefix marque le debut d'un message CryptoBulle.
	Prefix = "MC1~"
	// SaltLen est la taille du sel, en octets.
	SaltLen = 16

	magic     = "MC1"
	version   = 1
	nonceLen  = 12
	keyLen    = 32
	headerLen = len(magic) + 1 + SaltLen + nonceLen

	// scrypt : environ 16 Mo de memoire et ~50 ms de calcul.
	scryptN = 1 << 14
	scryptR = 8
	scryptP = 1
)

// appPepper est melange a la phrase secrete : une meme phrase ne donne donc
// pas la meme cle dans un autre logiciel.
var appPepper = []byte("CryptoBulle-v1|poivre-applicatif|8f2c1d9a4b6e")

// appAlphabet est une permutation des 64 caracteres du base64 « urlsafe ».
const appAlphabet = "AfhmdLEr3GMxg2S_a91UZnTlHOI5KYzevFCwNQ-P4tWV7cpXRbDuykjJsoi6q80B"

var (
	encoding = base64.NewEncoding(appAlphabet).WithPadding(base64.NoPadding)
	// Le tiret doit etre echappe : dans une classe [...], il signifierait
	// « intervalle de caracteres ».
	tokenClass = strings.ReplaceAll(regexp.QuoteMeta(appAlphabet), "-", `\-`)
	tokenRe    = regexp.MustCompile(regexp.QuoteMeta(Prefix) + "[" + tokenClass + "]{40,}")
	spaces     = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "\v", "", "\f", "")
)

// Erreurs presentees telles quelles a l'utilisateur.
var (
	ErrNoPassphrase = errors.New("aucune phrase secrete n'est configuree")
	ErrEmpty        = errors.New("il n'y a rien a chiffrer")
	ErrNotAToken    = errors.New("ce texte ne contient pas de message CryptoBulle")
	ErrDamaged      = errors.New("message CryptoBulle incomplet ou abime")
	ErrWrongKey     = errors.New("dechiffrement impossible : phrase secrete differente ou message modifie")
)

// --- cache de cles ----------------------------------------------------------

var keyCache = struct {
	sync.Mutex
	entries map[string][]byte
}{entries: make(map[string][]byte)}

// DeriveKey calcule la cle AES-256 pour une phrase secrete et un sel donnes.
//
// Le resultat est garde en memoire : le premier appel coute ~50 ms, les
// suivants sont instantanes. C'est ce qui permet de chauffer la cle au
// demarrage pour que le raccourci clavier ne fasse jamais attendre.
func DeriveKey(passphrase string, salt []byte) ([]byte, error) {
	cacheKey := passphrase + "\x00" + string(salt)

	keyCache.Lock()
	if key, ok := keyCache.entries[cacheKey]; ok {
		keyCache.Unlock()
		return key, nil
	}
	keyCache.Unlock()

	material := append([]byte(passphrase), '|')
	material = append(material, appPepper...)
	key, err := scrypt.Key(material, salt, scryptN, scryptR, scryptP, keyLen)
	if err != nil {
		return nil, fmt.Errorf("derivation de la cle : %w", err)
	}

	keyCache.Lock()
	if len(keyCache.entries) > 64 { // borne simple : on repart de zero
		keyCache.entries = make(map[string][]byte)
	}
	keyCache.entries[cacheKey] = key
	keyCache.Unlock()
	return key, nil
}

// NewSalt tire un sel aleatoire.
func NewSalt() []byte {
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		panic("source d'alea indisponible : " + err.Error())
	}
	return salt
}

// --- chiffrement ------------------------------------------------------------

// Encrypt chiffre plaintext et renvoie un jeton « MC1~... ».
//
// salt peut etre le sel personnel de l'utilisateur, conserve dans ses reglages :
// la cle correspondante etant deja en cache, le chiffrement devient instantane.
// La securite ne change pas, car le nonce, lui, est tire au hasard a chaque
// message.
func Encrypt(plaintext, passphrase string, salt []byte) (string, error) {
	if passphrase == "" {
		return "", ErrNoPassphrase
	}
	if plaintext == "" {
		return "", ErrEmpty
	}
	if len(salt) != SaltLen {
		salt = NewSalt()
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("source d'alea indisponible : %w", err)
	}

	header := make([]byte, 0, headerLen)
	header = append(header, magic...)
	header = append(header, version)
	header = append(header, salt...)
	header = append(header, nonce...)

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	// Le header sert de donnees associees : il est authentifie, pas chiffre.
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), header)
	return Prefix + encoding.EncodeToString(append(header, sealed...)), nil
}

// Decrypt verifie et dechiffre un jeton « MC1~... ».
func Decrypt(token, passphrase string) (string, error) {
	if passphrase == "" {
		return "", ErrNoPassphrase
	}
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, Prefix) {
		return "", ErrNotAToken
	}

	raw, err := encoding.DecodeString(token[len(Prefix):])
	if err != nil {
		return "", ErrNotAToken
	}
	if len(raw) <= headerLen || string(raw[:3]) != magic {
		return "", ErrDamaged
	}
	if raw[3] != version {
		return "", fmt.Errorf("message cree avec une version plus recente (v%d)", raw[3])
	}

	header := raw[:headerLen]
	salt := raw[4 : 4+SaltLen]
	nonce := raw[4+SaltLen : headerLen]

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, raw[headerLen:], header)
	if err != nil {
		return "", ErrWrongKey
	}
	return string(plaintext), nil
}

func newGCM(passphrase string, salt []byte) (cipher.AEAD, error) {
	key, err := DeriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES : %w", err)
	}
	return cipher.NewGCM(block)
}

// --- reperage dans un texte -------------------------------------------------

// FindToken renvoie le premier jeton CryptoBulle contenu dans text, ou "".
//
// Les espaces et retours a la ligne sont retires au prealable : un jeton coupe
// sur plusieurs lignes par un logiciel de courriel reste donc reconnu.
func FindToken(text string) string {
	if text == "" {
		return ""
	}
	return tokenRe.FindString(spaces.Replace(text))
}

// LooksEncrypted indique si le texte contient un message CryptoBulle.
func LooksEncrypted(text string) bool { return FindToken(text) != "" }
