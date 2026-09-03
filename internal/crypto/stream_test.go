package crypto

import (
	"strings"
	"testing"
)

// masquer simule la frappe : chaque caractere passe un par un, comme au clavier.
func masquer(t *testing.T, texte, phrase string) string {
	t.Helper()
	stream, err := NewStream(phrase)
	if err != nil {
		t.Fatal(err)
	}
	var visible strings.Builder
	visible.WriteString(stream.Marker())
	for _, letter := range texte {
		masked, _ := stream.Mask(letter)
		visible.WriteRune(masked)
	}
	return visible.String()
}

func TestAlphabetsDeLaFrappeMasquee(t *testing.T) {
	if len(inputTable) != len(outputTable) {
		t.Fatalf("tailles differentes : %d et %d", len(inputTable), len(outputTable))
	}
	if len(inputIndex) != len(inputTable) || len(outputIndex) != len(outputTable) {
		t.Fatal("un caractere est repete dans un alphabet")
	}
	for _, interdit := range []rune{' ', '\n', '\r', '\t'} {
		if _, present := outputIndex[interdit]; present {
			t.Fatalf("l'alphabet de sortie ne doit pas contenir %q", interdit)
		}
	}
	if _, present := inputIndex[' ']; !present {
		t.Fatal("l'espace doit pouvoir etre tape")
	}
}

func TestAllerRetourFrappeMasquee(t *testing.T) {
	textes := []string{
		"bonjour",
		"Rendez-vous a 15h30 !",
		"j'ai reussi l'examen, ça va très bien",
		"MOT DE PASSE : Wifi-2026#",
		strings.Repeat("abc ", 300),
	}
	for _, texte := range textes {
		visible := masquer(t, texte, pass)
		if !strings.HasPrefix(visible, Prefix2) {
			t.Fatalf("marqueur absent pour %.20q", texte)
		}
		relu, err := Decrypt(visible, pass)
		if err != nil {
			t.Fatalf("Decrypt(%.20q) : %v", texte, err)
		}
		if relu != texte {
			t.Fatalf("aller-retour casse :\n  attendu %q\n  obtenu  %q", texte, relu)
		}
	}
}

func TestLongueurConserveeEtTexteCache(t *testing.T) {
	texte := "mot de passe du wifi"
	visible := masquer(t, texte, pass)
	corps := visible[len(Prefix2)+streamNonceChars:]

	if len([]rune(corps)) != len([]rune(texte)) {
		t.Fatalf("un caractere tape doit donner un caractere affiche : %d contre %d",
			len([]rune(corps)), len([]rune(texte)))
	}
	if strings.Contains(corps, "wifi") || strings.Contains(corps, "passe") {
		t.Fatal("le texte tape apparait a l'ecran")
	}
	if strings.ContainsAny(corps, " \t\n\r") {
		t.Fatal("le texte masque ne doit contenir aucune espace")
	}
}

func TestRetourArriere(t *testing.T) {
	stream, err := NewStream(pass)
	if err != nil {
		t.Fatal(err)
	}
	visible := []rune(stream.Marker())

	for _, letter := range "cha" {
		masked, _ := stream.Mask(letter)
		visible = append(visible, masked)
	}
	// L'utilisateur efface le « a » et tape « t » a la place : il reste « cht ».
	// Le compteur recule pour que le caractere efface et le nouveau utilisent le
	// meme octet de la suite chiffrante, sans quoi la relecture serait decalee.
	visible = visible[:len(visible)-1]
	stream.Rewind()
	masked, _ := stream.Mask('t')
	visible = append(visible, masked)

	relu, err := Decrypt(string(visible), pass)
	if err != nil {
		t.Fatal(err)
	}
	if relu != "cht" {
		t.Fatalf("apres un retour arriere : %q", relu)
	}
}

func TestCaracteresHorsAlphabetLaissesTelsQuels(t *testing.T) {
	stream, _ := NewStream(pass)
	if _, masque := stream.Mask('🥁'); masque {
		t.Fatal("un caractere hors alphabet ne devrait pas etre masque")
	}
	if stream.Position() != 0 {
		t.Fatal("un caractere laisse tel quel ne doit pas avancer le compteur")
	}
}

func TestMauvaisePhraseDonneDuCharabia(t *testing.T) {
	// Un chiffrement par flux n'a pas de signature : avec la mauvaise phrase, on
	// obtient du charabia plutot qu'une erreur. C'est une limite assumee.
	visible := masquer(t, "rendez-vous a 15h", pass)
	relu, err := Decrypt(visible, "mauvaise phrase")
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if relu == "rendez-vous a 15h" {
		t.Fatal("la mauvaise phrase secrete a quand meme donne le bon texte")
	}
}

func TestDeuxActivationsDonnentDesTextesDifferents(t *testing.T) {
	if masquer(t, "meme texte", pass) == masquer(t, "meme texte", pass) {
		t.Fatal("deux activations doivent donner des textes differents")
	}
}

func TestReperageDansUneLigne(t *testing.T) {
	visible := masquer(t, "salut", pass)
	entoure := "Il a ecrit " + visible + " juste avant"
	if got := FindToken(entoure); got != visible {
		t.Fatalf("jeton mal repere :\n  attendu %q\n  obtenu  %q", visible, got)
	}

	// Deux lignes : la premiere doit etre rendue en entier, sans deborder.
	deuxLignes := masquer(t, "ligne un", pass) + "\r\n" + masquer(t, "ligne deux", pass)
	relu, err := Decrypt(FindToken(deuxLignes), pass)
	if err != nil {
		t.Fatal(err)
	}
	if relu != "ligne un" {
		t.Fatalf("premiere ligne relue : %q", relu)
	}
}

func TestLesDeuxFormatsCoexistent(t *testing.T) {
	bloc, err := Encrypt("message ordinaire", pass, nil)
	if err != nil {
		t.Fatal(err)
	}
	if relu, err := Decrypt(bloc, pass); err != nil || relu != "message ordinaire" {
		t.Fatalf("format MC1 casse : %q, %v", relu, err)
	}
	visible := masquer(t, "frappe masquee", pass)
	if relu, err := Decrypt(visible, pass); err != nil || relu != "frappe masquee" {
		t.Fatalf("format MC2 casse : %q, %v", relu, err)
	}
}
