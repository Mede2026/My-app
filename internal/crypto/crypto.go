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
	// Prefix annonce un message ordinaire.
	Prefix = "MC1~"
	// SaltLen est la taille du sel, en octets.
	SaltLen = 16

	magic   = "MC1"
	version = 1
	// Certaines versions ont brievement produit des messages sans marqueur, avec
	// le numero de version range dans la partie chiffree. Ils restent lisibles.
	bareVersion = 2
	nonceLen    = 12
	keyLen      = 32
	headerLen   = len(magic) + 1 + SaltLen + nonceLen
	// Disposition des messages sans marqueur : ni entete magique, ni version.
	bareHeaderLen = SaltLen + nonceLen

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
	header = append(header, magic...)
	header = append(header, version)
	header = append(header, salt...)
	header = append(header, nonce...)

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	// L'entete sert de donnees associees : elle est authentifiee, pas chiffree.
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), header)
	return Prefix + encoding.EncodeToString(append(header, sealed...)), nil
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
	raw, err := encoding.DecodeString(strings.TrimPrefix(strings.TrimSpace(token), Prefix))
	if err != nil {
		return "", ErrNotAToken
	}

	if len(raw) > headerLen+tagLen && string(raw[:len(magic)]) == magic {
		return openBlock(raw, passphrase, len(magic)+1, headerLen, false)
	}
	// Messages sans marqueur produits par une version intermediaire.
	if len(raw) > bareHeaderLen+tagLen {
		return openBlock(raw, passphrase, 0, bareHeaderLen, true)
	}
	return "", ErrNotAToken
}

// openBlock ouvre une charge utile deja decodee.
//
// `saltAt` est la position du sel, `headerEnd` la fin de l'entete authentifiee.
// Quand `versionInside` est vrai, le numero de version est le premier octet du
// texte dechiffre plutot qu'un octet de l'entete.
func openBlock(raw []byte, passphrase string, saltAt, headerEnd int, versionInside bool) (string, error) {
	if !versionInside && raw[len(magic)] != version {
		return "", fmt.Errorf("message cree avec une version plus recente (v%d)", raw[len(magic)])
	}
	salt := raw[saltAt : saltAt+SaltLen]
	nonce := raw[saltAt+SaltLen : headerEnd]

	gcm, err := newGCM(passphrase, salt)
	if err != nil {
		return "", err
	}
	opened, err := gcm.Open(nil, nonce, raw[headerEnd:], raw[:headerEnd])
	if err != nil {
		return "", ErrWrongKey
	}
	if !versionInside {
		return string(opened), nil
	}
	if len(opened) == 0 || opened[0] != bareVersion {
		return "", ErrNotAToken
	}
	return string(opened[1:]), nil
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

// LooksEncrypted indique si le texte contient deja un message ordinaire.
//
// Aucune cle n'est necessaire : le marqueur et l'entete voyagent encodes, mais
// non chiffres. Cela sert uniquement a eviter de chiffrer deux fois.
func LooksEncrypted(text string) bool {
	if text == "" {
		return false
	}
	glued := spaces.Replace(text)
	if strings.Contains(glued, strings.TrimSuffix(Prefix, "~")) {
		return true
	}
	for _, candidate := range blockRunRe.FindAllString(glued, -1) {
		raw, err := encoding.DecodeString(candidate)
		if err == nil && len(raw) > headerLen && string(raw[:len(magic)]) == magic {
			return true
		}
	}
	return false
}
