package crypto

import (
	"regexp"
	"strings"
)

// Reperage d'un texte chiffre dans une selection, sans marqueur visible.
//
// Rien n'annonce plus le debut d'un message : le texte produit n'est qu'une
// suite de caracteres etranges. Pour le relire, l'application essaie tout
// simplement de le dechiffrer.
//
// Ce n'est pas un pari : un message MC1 porte une entete et une signature qui
// ne peuvent pas apparaitre par hasard, et un texte tape en frappe masquee
// porte deux caracteres de controle, invisibles au milieu du charabia.

var (
	// Suites de caracteres pouvant former un message ordinaire.
	blockRunRe = regexp.MustCompile("[" + tokenClass + "]{" + itoa(minBlockChars) + ",}")
	// Suites de caracteres pouvant former une ligne de frappe masquee.
	streamRunRe = regexp.MustCompile("[" + regexp.QuoteMeta(outputRunes) + "]{" +
		itoa(streamNonceChars+streamCheckChars+1) + ",}")
)

// minBlockChars est la taille minimale d'un message ordinaire une fois encode :
// entete, nonce et signature occupent deja une cinquantaine d'octets.
const minBlockChars = 40

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

// DecryptText cherche un texte chiffre dans `selection` et le relit.
//
// Les deux formats sont essayes : le message ordinaire d'abord, dont l'entete
// et la signature sont sans ambiguite, puis la frappe masquee, ligne par ligne.
func DecryptText(selection, passphrase string) (string, error) {
	if passphrase == "" {
		return "", ErrNoPassphrase
	}
	if strings.TrimSpace(selection) == "" {
		return "", ErrNotAToken
	}

	// Message ordinaire : les espaces sont retires, car un envoi par courriel
	// peut avoir coupe le texte en plusieurs morceaux.
	// Si un message est bien reconnu mais refuse la cle, autant le dire
	// precisement plutot que de pretendre n'avoir rien trouve.
	recognized := false

	// Chaque essai coute une derivation de cle quand le sel est inconnu : on se
	// limite aux quelques plus longues suites, ou se trouve le vrai message.
	glued := spaces.Replace(stripLegacyPrefixes(selection))
	for _, candidate := range firstFew(longestFirst(blockRunRe.FindAllString(glued, -1))) {
		plain, err := decryptBlock(candidate, passphrase)
		switch err {
		case nil:
			return plain, nil
		case ErrWrongKey:
			recognized = true
		}
	}

	// Frappe masquee : chaque ligne se relit toute seule.
	for _, line := range strings.Split(selection, "\n") {
		line = stripLegacyPrefixes(strings.TrimRight(line, "\r"))
		for _, candidate := range firstFew(longestFirst(streamRunRe.FindAllString(line, -1))) {
			if plain, err := decryptStream(candidate, passphrase); err == nil {
				return plain, nil
			}
		}
	}
	if recognized {
		return "", ErrWrongKey
	}
	return "", ErrNotFound
}

// stripLegacyPrefixes retire les marqueurs des premieres versions, qui
// commencaient par MC1~ ou MC2~. Les textes deja produits restent lisibles.
func stripLegacyPrefixes(text string) string {
	text = strings.ReplaceAll(text, legacyPrefixBlock, "")
	return strings.ReplaceAll(text, legacyPrefixStream, "")
}

// firstFew garde les candidats les plus prometteurs.
func firstFew(candidates []string) []string {
	const maximum = 3
	if len(candidates) > maximum {
		return candidates[:maximum]
	}
	return candidates
}

// longestFirst trie les candidats du plus long au plus court : le vrai message
// est presque toujours la plus longue suite de caracteres du bon alphabet.
func longestFirst(candidates []string) []string {
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && len(candidates[j]) > len(candidates[j-1]); j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	return candidates
}
