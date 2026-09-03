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
	"strings"
)

// legacyPrefixStream etait ecrit devant les textes des premieres versions. Il
// n'est plus produit, mais reste accepte a la relecture.
const legacyPrefixStream = "MC2~"

const (
	streamNonceLen   = 6 // octets tires au hasard a chaque activation
	streamNonceChars = 8 // leur ecriture dans l'alphabet maison
	// Deux caracteres de controle, calcules a partir de la cle et du tirage.
	// Ils ne se voient pas : ils ressemblent au reste du charabia. Ils
	// permettent a l'application de reconnaitre son propre texte, et de dire
	// « ce n'est pas pour moi » plutot que de rendre n'importe quoi.
	streamCheckChars = 2
	// Les deux premiers octets de la suite chiffrante servent au controle : le
	// texte lui-meme commence apres.
	streamTextStart = 2
)

// Sel fixe : le nonce, lui, change a chaque activation. Un sel constant permet
// a deux personnes ayant la meme phrase secrete de se relire sans echanger
// autre chose que le texte visible.
var streamSalt = []byte("CryptoBulle-flux")

// inputRunes : ce que l'utilisateur peut taper. outputRunes : ce qui s'affiche.
// L'ensemble de sortie ne contient ni espace ni retour a la ligne, pour que le
// texte masque reste un bloc facile a selectionner.
//
// La sortie compte une place de plus que l'entree : cette place supplementaire
// sert d'echappement. Elle annonce un caractere qui n'est pas dans la liste,
// emoji compris ; son numero Unicode suit alors sur trois caracteres masques.
// Rien ne sort donc jamais en clair.
const (
	inputRunes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 " +
		".,;:!?'\"-()[]{}/\\@#&+=%*_<>|~^$" +
		"éèêëàâäùûüçîïôöÿœæÉÈÊÀÂÙÛÇÎÔŒÆ°§µ"
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
)

// escapeIndex est la place reservee a l'echappement, juste apres les
// caracteres ordinaires.
var escapeIndex int

func init() {
	alphabetLen = len(outputTable)
	escapeIndex = len(inputTable)
	if escapeIndex != alphabetLen-1 {
		panic("crypto : la sortie doit compter exactement une place de plus que l'entree")
	}
	for index, letter := range inputTable {
		inputIndex[letter] = index
	}
	for index, letter := range outputTable {
		outputIndex[letter] = index
	}
	if len(inputIndex) != escapeIndex || len(outputIndex) != alphabetLen {
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
	stream := &Stream{counter: cipher.NewCTR(block, iv), position: streamTextStart}
	// L'en-tete invisible : le tirage aleatoire, puis les deux caracteres de
	// controle tires de la suite chiffrante.
	header := encoding.EncodeToString(nonce)
	for index := 0; index < streamCheckChars; index++ {
		header += string(outputTable[int(stream.byteAt(index))%alphabetLen])
	}
	stream.marker = header
	return stream, nil
}

// Marker est l'en-tete a ecrire avant les caracteres masques. Il ne contient
// aucun mot reconnaissable : il se lit comme la suite du charabia.
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

// Mask rend ce qu'il faut afficher a la place du caractere qui vient d'etre
// tape : un seul caractere en general, quatre pour ceux qui sortent de la liste
// (emoji, symboles rares). Rien n'est jamais laisse en clair.
func (s *Stream) Mask(typed rune) string {
	if index, known := inputIndex[typed]; known {
		return string(s.maskIndex(index))
	}

	// Echappement : la place reservee, puis le numero Unicode sur trois
	// morceaux de sept bits.
	code := uint32(typed)
	masked := []rune{
		s.maskIndex(escapeIndex),
		s.maskIndex(int(code >> 14 & 0x7F)),
		s.maskIndex(int(code >> 7 & 0x7F)),
		s.maskIndex(int(code & 0x7F)),
	}
	return string(masked)
}

// maskIndex chiffre une position de l'alphabet et avance d'un cran.
func (s *Stream) maskIndex(index int) rune {
	shift := int(s.byteAt(s.position))
	s.position++
	return outputTable[(index+shift)%alphabetLen]
}

// Rewind revient d'un caractere en arriere, apres un retour arriere au clavier.
func (s *Stream) Rewind() {
	if s.position > streamTextStart {
		s.position--
	}
}

// Position est le nombre de caracteres deja masques.
func (s *Stream) Position() int { return s.position - streamTextStart }

// unmask retrouve la position d'origine d'un caractere affiche.
func (s *Stream) unmask(letter rune) (int, bool) {
	index, known := outputIndex[letter]
	if !known {
		return 0, false
	}
	shift := int(s.byteAt(s.position))
	s.position++
	return ((index-shift)%alphabetLen + alphabetLen) % alphabetLen, true
}

// decryptStream relit un texte tape en frappe masquee.
//
// L'erreur ErrNotFound signale que ce texte n'en est pas un : les deux
// caracteres de controle ne correspondent pas.
func decryptStream(token, passphrase string) (string, error) {
	if passphrase == "" {
		return "", ErrNoPassphrase
	}
	letters := []rune(strings.TrimPrefix(token, legacyPrefixStream))
	if len(letters) <= streamNonceChars+streamCheckChars {
		return "", ErrNotFound
	}

	nonce, err := encoding.DecodeString(string(letters[:streamNonceChars]))
	if err != nil || len(nonce) != streamNonceLen {
		return "", ErrNotFound
	}
	stream, err := newStreamWithNonce(passphrase, nonce)
	if err != nil {
		return "", err
	}
	if string(letters[streamNonceChars:streamNonceChars+streamCheckChars]) !=
		stream.marker[streamNonceChars:] {
		return "", ErrNotFound // pas notre texte, ou pas la bonne phrase secrete
	}

	body := letters[streamNonceChars+streamCheckChars:]
	var plain strings.Builder
	for position := 0; position < len(body); position++ {
		index, known := stream.unmask(body[position])
		if !known {
			plain.WriteRune(body[position]) // caractere etranger au texte masque
			continue
		}
		if index != escapeIndex {
			plain.WriteRune(inputTable[index])
			continue
		}
		// Echappement : les trois caracteres suivants portent le numero Unicode.
		if position+3 >= len(body) {
			break
		}
		code := uint32(0)
		for morceau := 0; morceau < 3; morceau++ {
			position++
			digit, known := stream.unmask(body[position])
			if !known {
				return plain.String(), nil
			}
			code = code<<7 | uint32(digit&0x7F)
		}
		plain.WriteRune(rune(code))
	}
	return plain.String(), nil
}
