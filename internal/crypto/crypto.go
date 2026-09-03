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
	// legacyPrefixBlock etait ecrit devant les messages des premieres versions.
	// Il n'est plus produit, mais reste accepte a la relecture.
	legacyPrefixBlock = "MC1~"
	// SaltLen est la taille du sel, en octets.
	SaltLen = 16

	// magic n'apparait plus que dans les messages des premieres versions.
	magic     = "MC1"
	version   = 2
	nonceLen  = 12
	keyLen    = 32
	headerLen = SaltLen + nonceLen
	// Disposition des messages produits avant le retrait du marqueur.
	legacyHeaderLen = len(magic) + 1 + SaltLen + nonceLen

	// scrypt : environ 16 Mo de memoire et ~50 ms de calcul.
	scryptN = 1 << 14
	scryptR = 8
	scryptP = 1

	// Taille de la signature ajoutee par AES-GCM.
	tagLen = 16
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
	spaces     = strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "\v", "", "\f", "")
)

// Erreurs presentees telles quelles a l'utilisateur.
var (
	ErrNoPassphrase = errors.New("aucune phrase secrete n'est configuree")
	ErrEmpty        = errors.New("il n'y a rien a chiffrer")
	ErrNotAToken    = errors.New("ce texte ne contient pas de message CryptoBulle")
	ErrNotFound     = errors.New("aucun message CryptoBulle dans la selection")
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
	header = append(header, salt...)
	header = append(header, nonce...)

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	// Le sel et le nonce sont authentifies sans etre chiffres ; le numero de
	// version, lui, voyage dans la partie chiffree pour ne rien laisser
	// paraitre. Le message commence donc par du hasard pur.
	sealed := gcm.Seal(nil, nonce, append([]byte{version}, plaintext...), header)
	return encoding.EncodeToString(append(header, sealed...)), nil
}

// Decrypt relit un texte chiffre, dans l'un ou l'autre des deux formats.
//
// Pour une selection prise a la souris, prefere DecryptText, qui sait retrouver
// le texte chiffre au milieu d'autre chose.
func Decrypt(token, passphrase string) (string, error) {
	return DecryptText(token, passphrase)
}

// decryptBlock relit un message ordinaire deja isole.
func decryptBlock(token, passphrase string) (string, error) {
	if passphrase == "" {
		return "", ErrNoPassphrase
	}
	token = strings.TrimPrefix(strings.TrimSpace(token), legacyPrefixBlock)

	raw, err := encoding.DecodeString(token)
	if err != nil {
		return "", ErrNotAToken
	}
	if len(raw) > legacyHeaderLen && string(raw[:len(magic)]) == magic {
		return decryptLegacyBlock(raw, passphrase)
	}
	if len(raw) <= headerLen+tagLen {
		return "", ErrNotAToken
	}

	header := raw[:headerLen]
	salt, nonce := raw[:SaltLen], raw[SaltLen:headerLen]

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	opened, err := gcm.Open(nil, nonce, raw[headerLen:], header)
	if err != nil {
		return "", ErrWrongKey
	}
	if opened[0] != version {
		return "", fmt.Errorf("message cree avec une version plus recente (v%d)", opened[0])
	}
	return string(opened[1:]), nil
}

// decryptLegacyBlock relit les messages produits avant le retrait du marqueur,
// dont l'entete voyageait en clair.
func decryptLegacyBlock(raw []byte, passphrase string) (string, error) {
	if raw[len(magic)] != 1 {
		return "", fmt.Errorf("message cree avec une version plus recente (v%d)", raw[len(magic)])
	}
	header := raw[:legacyHeaderLen]
	salt := raw[len(magic)+1 : len(magic)+1+SaltLen]
	nonce := raw[len(magic)+1+SaltLen : legacyHeaderLen]

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, raw[legacyHeaderLen:], header)
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

// LooksEncrypted indique si le texte est deja un message CryptoBulle.
//
// Plus rien ne depasse en clair : la seule facon de le savoir est d'essayer de
// le relire. Cela sert a ne pas chiffrer deux fois le meme texte.
func LooksEncrypted(text, passphrase string) bool {
	if text == "" || passphrase == "" {
		return false
	}
	_, err := DecryptText(text, passphrase)
	return err == nil
}
