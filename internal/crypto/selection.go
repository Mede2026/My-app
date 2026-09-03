package crypto

import (
	"regexp"
	"strings"
)

// Reperage d'un texte chiffre dans une selection.
//
// Un message ordinaire s'annonce par « MC1~ » : il est trouve tout de suite.
// Une ligne tapee en frappe masquee, elle, ne porte aucun marqueur : pour la
// relire, l'application essaie simplement de la dechiffrer. Ce n'est pas un
// pari, car deux caracteres de controle, invisibles au milieu du charabia,
// disent si le texte est bien le sien.

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

	// Si un message est bien reconnu mais refuse la cle, autant le dire
	// precisement plutot que de pretendre n'avoir rien trouve.
	recognized := false

	// Message ordinaire : le marqueur donne le depart. Les espaces sont retires,
	// car un envoi par courriel peut avoir coupe le texte en plusieurs morceaux.
	glued := spaces.Replace(selection)
	if start := strings.Index(glued, Prefix); start >= 0 {
		if token := blockRunRe.FindString(glued[start+len(Prefix):]); token != "" {
			plain, err := decryptBlock(token, passphrase)
			if err == nil {
				return plain, nil
			}
			if err == ErrWrongKey {
				return "", ErrWrongKey
			}
		}
	}

	// Chaque essai coute une derivation de cle quand le sel est inconnu : on se
	// limite aux quelques plus longues suites, ou se trouve le vrai message.
	glued = spaces.Replace(stripLegacyPrefixes(selection))
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

// stripLegacyPrefixes retire les marqueurs, pour que la recherche « a
// l'aveugle » retrouve aussi les textes qui en portent un.
func stripLegacyPrefixes(text string) string {
	text = strings.ReplaceAll(text, Prefix, "")
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
