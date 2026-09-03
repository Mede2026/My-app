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
		visible.WriteString(stream.Mask(letter))
	}
	return visible.String()
}

func TestAlphabetsDeLaFrappeMasquee(t *testing.T) {
	// La sortie compte une place de plus : celle de l'echappement, qui annonce
	// un caractere absent de la liste (emoji, symbole rare).
	if len(outputTable) != len(inputTable)+1 {
		t.Fatalf("tailles inattendues : entree %d, sortie %d", len(inputTable), len(outputTable))
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
		aucunMarqueur(t, visible)
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
	corps := string([]rune(visible)[streamNonceChars+streamCheckChars:])

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
		visible = append(visible, []rune(stream.Mask(letter))...)
	}
	// L'utilisateur efface le « a » et tape « t » a la place : il reste « cht ».
	// Le compteur recule pour que le caractere efface et le nouveau utilisent le
	// meme octet de la suite chiffrante, sans quoi la relecture serait decalee.
	visible = visible[:len(visible)-1]
	stream.Rewind()
	visible = append(visible, []rune(stream.Mask('t'))...)

	relu, err := Decrypt(string(visible), pass)
	if err != nil {
		t.Fatal(err)
	}
	if relu != "cht" {
		t.Fatalf("apres un retour arriere : %q", relu)
	}
}

func TestEmojisEtCaracteresRares(t *testing.T) {
	textes := []string{
		"🥁",
		"salut 🥁 ça va ?",
		"ski ⛷ au Massif 🏔 samedi",
		"prix : 12 € pour 3 kg",
		"日本語",
	}
	for _, texte := range textes {
		visible := masquer(t, texte, pass)
		corps := string([]rune(visible)[streamNonceChars+streamCheckChars:])

		// Un caractere absent de l'alphabet de sortie ne peut pas s'y trouver
		// par hasard : s'il apparait, c'est qu'il est passe en clair.
		for _, letter := range corps {
			if _, connu := outputIndex[letter]; !connu {
				t.Fatalf("le caractere %q est passe en clair dans %.30q", letter, corps)
			}
		}
		if strings.Contains(corps, texte) {
			t.Fatalf("le texte tape apparait tel quel dans %.30q", corps)
		}
		relu, err := Decrypt(visible, pass)
		if err != nil {
			t.Fatalf("Decrypt(%q) : %v", texte, err)
		}
		if relu != texte {
			t.Fatalf("aller-retour casse :\n  attendu %q\n  obtenu  %q", texte, relu)
		}
	}
}

func TestUnEmojiOccupeQuatreCaracteres(t *testing.T) {
	// Un caractere hors de la liste s'ecrit en quatre : l'echappement, puis son
	// numero Unicode. C'est le seul endroit ou la correspondance n'est pas
	// exactement un pour un.
	stream, _ := NewStream(pass)
	if got := len([]rune(stream.Mask('a'))); got != 1 {
		t.Fatalf("une lettre ordinaire doit donner un caractere, obtenu %d", got)
	}
	if got := len([]rune(stream.Mask('🥁'))); got != 4 {
		t.Fatalf("un emoji doit donner quatre caracteres, obtenu %d", got)
	}
}

func TestMauvaisePhraseRefusee(t *testing.T) {
	// Les deux caracteres de controle permettent de dire « ce n'est pas pour
	// moi » plutot que de rendre du charabia.
	visible := masquer(t, "rendez-vous a 15h", pass)
	if relu, err := Decrypt(visible, "mauvaise phrase"); err == nil {
		t.Fatalf("la mauvaise phrase secrete a donne %q", relu)
	}
}

func TestDeuxActivationsDonnentDesTextesDifferents(t *testing.T) {
	if masquer(t, "meme texte", pass) == masquer(t, "meme texte", pass) {
		t.Fatal("deux activations doivent donner des textes differents")
	}
}

func TestReperageDansUneLigne(t *testing.T) {
	visible := masquer(t, "salut tout le monde", pass)
	if relu, err := DecryptText(visible, pass); err != nil || relu != "salut tout le monde" {
		t.Fatalf("ligne seule : %q, %v", relu, err)
	}

	// Deux blocs tapes separement, colles l'un sous l'autre : chacun porte son
	// propre en-tete, et la lecture repart au bon endroit.
	deuxBlocs := masquer(t, "ligne un", pass) + "\r\n" + masquer(t, "ligne deux", pass)
	relu, err := DecryptText(deuxBlocs, pass)
	if err != nil {
		t.Fatal(err)
	}
	// Le retour a la ligne traverse tel quel, « \r » compris.
	if relu != "ligne un\r\nligne deux" {
		t.Fatalf("deux blocs relus : %q", relu)
	}
}

func TestChangementDeChamp(t *testing.T) {
	// L'utilisateur tape, change de champ, et l'application repose un en-tete.
	// Le second morceau doit se relire, meme colle au premier sur une ligne.
	premier := masquer(t, "debut", pass)
	second := masquer(t, "suite", pass)

	relu, err := DecryptText(premier+second, pass)
	if err != nil {
		t.Fatal(err)
	}
	if relu != "debutsuite" {
		t.Fatalf("apres un changement de champ : %q", relu)
	}

	// Et separement, comme dans deux champs distincts.
	for texte, masque := range map[string]string{"debut": premier, "suite": second} {
		if relu, err := DecryptText(masque, pass); err != nil || relu != texte {
			t.Fatalf("champ isole %q : %q, %v", texte, relu, err)
		}
	}
}

func TestParagrapheUnSeulEnTete(t *testing.T) {
	texte := "Salut Simon,\nvoici le mot de passe : Batterie-2022.\nA demain !"

	stream, err := NewStream(pass)
	if err != nil {
		t.Fatal(err)
	}
	visible := stream.Marker()
	for _, letter := range texte {
		if letter == '\n' {
			visible += "\n" // le retour a la ligne traverse tel quel
			continue
		}
		visible += stream.Mask(letter)
	}

	// Un seul en-tete pour tout le paragraphe : les lignes suivantes commencent
	// directement par le texte masque.
	attendu := len([]rune(texte)) + StreamHeaderChars
	if got := len([]rune(visible)); got != attendu {
		t.Fatalf("%d caracteres affiches pour %d attendus", got, attendu)
	}
	relu, err := DecryptText(visible, pass)
	if err != nil || relu != texte {
		t.Fatalf("paragraphe relu : %q, %v", relu, err)
	}
}

// La frappe masquee, elle, ne porte aucun marqueur : c'est tout l'interet.
func TestFrappeMasqueeSansMarqueur(t *testing.T) {
	visible := masquer(t, "rendez-vous a 15h", pass)
	aucunMarqueur(t, visible)
	if strings.HasPrefix(visible, "MC") {
		t.Fatalf("la ligne masquee commence par un marqueur : %.20q", visible)
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
