// Package hotkey lit et ecrit les combinaisons de touches, par exemple
// « ctrl+alt+d ».
//
// Le resultat est un couple (modificateurs, code de touche) directement
// utilisable par RegisterHotKey, la fonction de Windows qui reserve un
// raccourci pour toute la session. Contrairement a un « hook » clavier,
// RegisterHotKey ne fait examiner a l'application que les combinaisons
// demandees, jamais l'ensemble des touches tapees sur la machine.
package hotkey

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Modificateurs, tels que Windows les attend.
const (
	ModAlt      = 0x0001
	ModControl  = 0x0002
	ModShift    = 0x0004
	ModWin      = 0x0008
	ModNoRepeat = 0x4000
)

// Combo est une combinaison analysee.
type Combo struct {
	Modifiers uint32
	Key       uint32
}

var errNoKey = errors.New("un raccourci doit contenir exactement une touche principale")

var modifiers = map[string]uint32{
	"ctrl": ModControl, "control": ModControl, "ctl": ModControl,
	"alt": ModAlt, "altgr": ModAlt,
	"shift": ModShift, "maj": ModShift,
	"win": ModWin, "windows": ModWin, "cmd": ModWin, "super": ModWin,
}

var modifierOrder = []struct {
	name string
	flag uint32
}{
	{"ctrl", ModControl}, {"alt", ModAlt}, {"shift", ModShift}, {"win", ModWin},
}

// namedKeys associe un nom canonique au code de touche virtuelle de Windows.
var namedKeys = map[string]uint32{
	"backspace": 0x08, "tab": 0x09, "enter": 0x0D, "escape": 0x1B, "space": 0x20,
	"pageup": 0x21, "pagedown": 0x22, "end": 0x23, "home": 0x24,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
	"insert": 0x2D, "delete": 0x2E,
	"plus": 0xBB, "minus": 0xBD, "comma": 0xBC, "period": 0xBE,
}

// aliases accepte a la saisie l'anglais, le francais et quelques abreviations.
var aliases = map[string]string{
	"esc": "escape", "echap": "escape", "entree": "enter", "return": "enter",
	"espace": "space", "suppr": "delete", "inser": "insert", "retour": "backspace",
	"haut": "up", "bas": "down", "gauche": "left", "droite": "right",
	"fin": "end", "debut": "home", "pgup": "pageup", "pgdn": "pagedown",
	"tabulation": "tab", "virgule": "comma", "point": "period",
}

var keyNames = map[uint32]string{}

func init() {
	for index := 1; index <= 24; index++ { // F1 = 0x70
		namedKeys["f"+strconv.Itoa(index)] = uint32(0x6F + index)
	}
	for name, code := range namedKeys {
		keyNames[code] = name
	}
}

// Parse traduit « ctrl+alt+d » en modificateurs et code de touche.
func Parse(combo string) (Combo, error) {
	var parsed Combo
	var keys []string

	for _, part := range strings.Split(strings.ToLower(combo), "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if flag, ok := modifiers[part]; ok {
			parsed.Modifiers |= flag
			continue
		}
		keys = append(keys, part)
	}

	if len(keys) == 0 && parsed.Modifiers == 0 {
		return Combo{}, errors.New("raccourci vide")
	}
	if len(keys) != 1 {
		return Combo{}, errNoKey
	}
	if parsed.Modifiers == 0 {
		return Combo{}, errors.New("ajoutez au moins Ctrl, Alt, Maj ou Windows")
	}

	key, err := keyCode(keys[0])
	if err != nil {
		return Combo{}, err
	}
	parsed.Key = key
	return parsed, nil
}

func keyCode(token string) (uint32, error) {
	if canonical, ok := aliases[token]; ok {
		token = canonical
	}
	if code, ok := namedKeys[token]; ok {
		return code, nil
	}
	if strings.HasPrefix(token, "0x") { // code brut, produit par String()
		if code, err := strconv.ParseUint(token[2:], 16, 32); err == nil {
			return uint32(code), nil
		}
	}
	if len(token) == 1 {
		char := token[0]
		switch {
		case char >= '0' && char <= '9':
			return uint32(char), nil // '0' vaut deja 0x30
		case char >= 'a' && char <= 'z':
			return uint32(char - 'a' + 0x41), nil
		}
	}
	return 0, fmt.Errorf("touche inconnue : « %s »", token)
}

// String ecrit la combinaison sous sa forme canonique : « ctrl+alt+d ».
func (c Combo) String() string {
	var parts []string
	for _, modifier := range modifierOrder {
		if c.Modifiers&modifier.flag != 0 {
			parts = append(parts, modifier.name)
		}
	}
	parts = append(parts, keyName(c.Key))
	return strings.Join(parts, "+")
}

// Pretty ecrit la combinaison pour l'affichage : « Ctrl+Alt+D ».
func (c Combo) Pretty() string {
	parts := strings.Split(c.String(), "+")
	for index, part := range parts {
		if len(part) == 1 {
			parts[index] = strings.ToUpper(part)
		} else {
			parts[index] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "+")
}

func keyName(key uint32) string {
	if name, ok := keyNames[key]; ok {
		return name
	}
	switch {
	case key >= '0' && key <= '9':
		return string(rune(key))
	case key >= 0x41 && key <= 0x5A:
		return strings.ToLower(string(rune(key)))
	}
	return fmt.Sprintf("0x%02x", key)
}

// Normalize remet un raccourci sous sa forme canonique.
func Normalize(combo string) (string, error) {
	parsed, err := Parse(combo)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

// Pretty est la version affichable d'un raccourci ecrit sous forme de texte.
// Un raccourci illisible est rendu tel quel, sans erreur.
func Pretty(combo string) string {
	parsed, err := Parse(combo)
	if err != nil {
		return combo
	}
	return parsed.Pretty()
}

// KnownKeys liste les noms de touches acceptes (pour la documentation).
func KnownKeys() []string {
	names := make([]string, 0, len(namedKeys))
	for name := range namedKeys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
