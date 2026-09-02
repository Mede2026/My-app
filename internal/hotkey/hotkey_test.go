package hotkey

import "testing"

func TestParseSimple(t *testing.T) {
	combo, err := Parse("ctrl+alt+d")
	if err != nil {
		t.Fatal(err)
	}
	if combo.Modifiers != ModControl|ModAlt || combo.Key != 0x44 {
		t.Fatalf("obtenu %+v", combo)
	}
}

func TestOrdreEtCasseSansImportance(t *testing.T) {
	first, _ := Parse("Alt+CTRL+d")
	second, _ := Parse("ctrl+alt+d")
	if first != second {
		t.Fatal("l'ordre ou la casse change le resultat")
	}
	if got, _ := Normalize("Alt+Ctrl+D"); got != "ctrl+alt+d" {
		t.Fatalf("Normalize = %q", got)
	}
}

func TestTousLesModificateurs(t *testing.T) {
	combo, err := Parse("ctrl+alt+shift+win+f5")
	if err != nil {
		t.Fatal(err)
	}
	if combo.Modifiers != ModControl|ModAlt|ModShift|ModWin {
		t.Fatalf("modificateurs %04x", combo.Modifiers)
	}
	if combo.Key != 0x74 {
		t.Fatalf("F5 devrait valoir 0x74, obtenu 0x%02x", combo.Key)
	}
}

func TestSynonymes(t *testing.T) {
	cases := map[string]string{
		"control+maj+echap": "ctrl+shift+escape",
		"ctrl+entree":       "ctrl+enter",
		"windows+espace":    "win+space",
		"ctrl+suppr":        "ctrl+delete",
	}
	for input, want := range cases {
		if got, err := Normalize(input); err != nil || got != want {
			t.Fatalf("Normalize(%q) = %q, %v ; attendu %q", input, got, err, want)
		}
	}
}

func TestOrdreCanonique(t *testing.T) {
	if got, _ := Normalize("win+alt+space"); got != "alt+win+space" {
		t.Fatalf("ordre canonique casse : %q", got)
	}
}

func TestAllerRetour(t *testing.T) {
	for _, combo := range []string{"ctrl+alt+d", "ctrl+shift+f12", "alt+win+space", "ctrl+delete", "ctrl+7"} {
		parsed, err := Parse(combo)
		if err != nil {
			t.Fatalf("Parse(%q) : %v", combo, err)
		}
		if parsed.String() != combo {
			t.Fatalf("aller-retour : %q -> %q", combo, parsed.String())
		}
	}
}

func TestCodeBrut(t *testing.T) {
	// Une touche sans nom connu est ecrite en hexadecimal, et doit se relire.
	combo := Combo{Modifiers: ModControl, Key: 0xDB}
	parsed, err := Parse(combo.String())
	if err != nil || parsed != combo {
		t.Fatalf("Parse(%q) = %+v, %v", combo.String(), parsed, err)
	}
}

func TestEntreesRefusees(t *testing.T) {
	for _, combo := range []string{"", "d", "ctrl", "ctrl+alt", "ctrl+a+b", "ctrl+touchemagique"} {
		if _, err := Parse(combo); err == nil {
			t.Fatalf("le raccourci %q aurait du etre refuse", combo)
		}
	}
}

func TestPretty(t *testing.T) {
	if got := Pretty("ctrl+alt+d"); got != "Ctrl+Alt+D" {
		t.Fatalf("Pretty = %q", got)
	}
	if got := Pretty("ctrl+shift+f9"); got != "Ctrl+Shift+F9" {
		t.Fatalf("Pretty = %q", got)
	}
	if got := Pretty("n'importe quoi"); got != "n'importe quoi" {
		t.Fatalf("un raccourci illisible doit etre rendu tel quel, obtenu %q", got)
	}
}
