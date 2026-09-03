// Frappe masquee : chiffrement au fil de la frappe, un caractere a la fois.
//
// Format « MC2 » : MC2~<nonce sur 8 caracteres><caracteres masques...>
//
// Contrairement au format MC1, qui chiffre un message entier d'un bloc, celui-ci
// doit produire un caractere visible des que l'utilisateur en tape un. C'est donc
// un chiffrement par flux : AES en mode compteur fabrique une suite d'octets
// imprevisible, et chaque octet decale la lettre tapee dans l'alphabet.
//
// Consequences a connaitre :
//   - la longueur du texte reste visible ;
//   - il n'y a pas de signature : un mauvais mot de passe donne du charabia au
//     lieu d'une erreur claire ;
//   - c'est fait pour qu'un voisin ne lise pas l'ecran, pas pour resister a une
//     analyse serieuse. Pour cela, le mode MC1 reste le bon choix.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

// Prefix2 marque le debut d'un texte tape en frappe masquee.
const Prefix2 = "MC2~"

const (
	streamNonceLen   = 6 // octets tires au hasard a chaque activation
	streamNonceChars = 8 // leur ecriture dans l'alphabet maison
)

// Sel fixe : le nonce, lui, change a chaque activation. Un sel constant permet
// a deux personnes ayant la meme phrase secrete de se relire sans echanger
// autre chose que le texte visible.
var streamSalt = []byte("CryptoBulle-flux")

// inputRunes : ce que l'utilisateur peut taper. outputRunes : ce qui s'affiche.
// Les deux ensembles font la meme taille, ce qui garantit une correspondance
// exacte, un caractere pour un caractere. L'ensemble de sortie ne contient ni
// espace ni retour a la ligne, pour que le texte masque reste un seul bloc
// facile a selectionner.
const (
	inputRunes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 " +
		".,;:!?'\"-()[]{}/\\@#&+=%*_<>|~^$" +
		"éèêëàâäùûüçîïôöÿœæÉÈÊÀÂÙÛÇÎÔŒÆ°§µ€"
	outputRunes = "!\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`" +
		"abcdefghijklmnopqrstuvwxyz{|}~" +
		"éèêëàâäùûüçîïôöÿœæÉÈÊÀÂÙÛÇÎÔŒÆ°§µ€"
)

var (
	inputTable  = []rune(inputRunes)
	outputTable = []rune(outputRunes)
	inputIndex  = map[rune]int{}
	outputIndex = map[rune]int{}
	alphabetLen int

	streamRe = regexp.MustCompile(
		regexp.QuoteMeta(Prefix2) +
			"[" + tokenClass + "]{" + fmt.Sprint(streamNonceChars) + "}" +
			"[" + regexp.QuoteMeta(outputRunes) + "]+")
)

func init() {
	alphabetLen = len(inputTable)
	if len(outputTable) != alphabetLen {
		panic("crypto : les alphabets de la frappe masquee n'ont pas la meme taille")
	}
	for index, letter := range inputTable {
		inputIndex[letter] = index
	}
	for index, letter := range outputTable {
		outputIndex[letter] = index
	}
	if len(inputIndex) != alphabetLen || len(outputIndex) != alphabetLen {
		panic("crypto : un caractere est repete dans un alphabet de la frappe masquee")
	}
}

// Stream masque les caracteres au fil de la frappe.
//
// Il n'est utilise que par le fil de l'interface : aucune protection contre les
// acces concurrents n'est necessaire.
type Stream struct {
	counter   cipher.Stream
	keystream []byte // suite deja calculee, pour pouvoir revenir en arriere
	position  int
	marker    string
}

// NewStream prepare une frappe masquee. Le texte renvoye est le marqueur a
// ecrire avant les caracteres masques.
func NewStream(passphrase string) (*Stream, error) {
	nonce := make([]byte, streamNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("source d'alea indisponible : %w", err)
	}
	stream, err := newStreamWithNonce(passphrase, nonce)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func newStreamWithNonce(passphrase string, nonce []byte) (*Stream, error) {
	key, err := DeriveKey(passphrase, streamSalt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("AES : %w", err)
	}
	iv := make([]byte, block.BlockSize())
	copy(iv, nonce)
	return &Stream{
		counter: cipher.NewCTR(block, iv),
		marker:  Prefix2 + encoding.EncodeToString(nonce),
	}, nil
}

// Marker est le texte a inserer avant les caracteres masques.
func (s *Stream) Marker() string { return s.marker }

// byteAt rend l'octet de la suite chiffrante a la position demandee, en la
// prolongeant si besoin. Les octets deja calcules sont conserves pour que le
// retour arriere puisse reutiliser exactement le meme.
func (s *Stream) byteAt(position int) byte {
	for len(s.keystream) <= position {
		block := make([]byte, 64)
		s.counter.XORKeyStream(block, block)
		s.keystream = append(s.keystream, block...)
	}
	return s.keystream[position]
}

// Mask rend le caractere a afficher a la place de celui qui vient d'etre tape.
// Le second resultat est faux si le caractere n'est pas dans l'alphabet : il
// doit alors etre laisse tel quel.
func (s *Stream) Mask(typed rune) (rune, bool) {
	index, known := inputIndex[typed]
	if !known {
		return typed, false
	}
	shift := int(s.byteAt(s.position))
	s.position++
	return outputTable[(index+shift)%alphabetLen], true
}

// Rewind revient d'un caractere en arriere, apres un retour arriere au clavier.
func (s *Stream) Rewind() {
	if s.position > 0 {
		s.position--
	}
}

// Position est le nombre de caracteres deja masques.
func (s *Stream) Position() int { return s.position }

// decryptStream relit un texte tape en frappe masquee.
func decryptStream(token, passphrase string) (string, error) {
	body := token[len(Prefix2):]
	if len(body) < streamNonceChars {
		return "", ErrDamaged
	}
	nonce, err := encoding.DecodeString(body[:streamNonceChars])
	if err != nil || len(nonce) != streamNonceLen {
		return "", ErrDamaged
	}

	stream, err := newStreamWithNonce(passphrase, nonce)
	if err != nil {
		return "", err
	}

	var plain strings.Builder
	for _, letter := range body[streamNonceChars:] {
		index, known := outputIndex[letter]
		if !known {
			plain.WriteRune(letter) // laisse tel quel, comme au chiffrement
			continue
		}
		shift := int(stream.byteAt(stream.position))
		stream.position++
		plain.WriteRune(inputTable[((index-shift)%alphabetLen+alphabetLen)%alphabetLen])
	}
	return plain.String(), nil
}

// findStreamToken cherche un texte masque, ligne par ligne.
//
// Les espaces ne sont pas retires ici, contrairement au format MC1 : chaque
// ligne est un texte independant, avec son propre marqueur.
func findStreamToken(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if start := strings.Index(line, Prefix2); start >= 0 {
			if token := streamRe.FindString(line[start:]); token != "" {
				return token
			}
		}
	}
	return ""
}
