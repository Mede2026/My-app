package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

// aucunMarqueur verifie qu'aucun mot reconnaissable n'annonce le texte chiffre.
//
// Les caracteres pris un a un, eux, peuvent apparaitre : c'est le propre du
// charabia. Seules les suites completes des anciens marqueurs sont interdites.
func aucunMarqueur(t *testing.T, texte string) {
	t.Helper()
	for _, marqueur := range []string{"MC1~", "MC2~"} {
		if strings.Contains(texte, marqueur) {
			t.Fatalf("le texte produit contient le marqueur %q : %.30q", marqueur, texte)
		}
	}
}

const pass = "batterie-metal-2022"

// Jeton produit par la premiere version (ecrite en Python), marqueur compris.
// Le format n'ayant pas change, il doit rester lisible.
const pythonToken = "MC1~UZgbAzkdfBpOmBiKoUQ5b0pfhpzwi5CZFsCwIO85HMFHwZpipPP6tp06OhG18WmnpWT6HmLSEsCp0y4HvNUQBVSk1pBd0iqU5lGt59nmJR"

func TestCompatibleAvecLaVersionPython(t *testing.T) {
	got, err := Decrypt(pythonToken, pass)
	if err != nil {
		t.Fatalf("Decrypt : %v", err)
	}
	if want := "Rendez-vous au Massif samedi 9h"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAllerRetour(t *testing.T) {
	messages := []string{"bonjour", "a", strings.Repeat("x", 5000), "accents : éàç ✓ 🥁", "ligne1\nligne2\tfin"}
	for _, message := range messages {
		token, err := Encrypt(message, pass, nil)
		if err != nil {
			t.Fatalf("Encrypt(%.10q) : %v", message, err)
		}
		aucunMarqueur(t, token)
		got, err := Decrypt(token, pass)
		if err != nil {
			t.Fatalf("Decrypt : %v", err)
		}
		if got != message {
			t.Fatalf("aller-retour casse pour %.20q", message)
		}
	}
}

func TestDeuxChiffrementsDifferent(t *testing.T) {
	salt := NewSalt()
	first, _ := Encrypt("meme texte", pass, salt)
	second, _ := Encrypt("meme texte", pass, salt)
	if first == second {
		t.Fatal("deux chiffrements identiques : le nonce ne change pas")
	}
	for _, token := range []string{first, second} {
		if got, err := Decrypt(token, pass); err != nil || got != "meme texte" {
			t.Fatalf("Decrypt : %q, %v", got, err)
		}
	}
}

func TestMauvaisePhraseSecrete(t *testing.T) {
	token, _ := Encrypt("secret", pass, nil)
	if _, err := Decrypt(token, "mauvaise phrase"); err != ErrWrongKey {
		t.Fatalf("erreur attendue ErrWrongKey, obtenu %v", err)
	}
}

func TestMessageModifie(t *testing.T) {
	token, _ := Encrypt("secret", pass, nil)
	modified := token[:len(token)-1] + "A"
	if modified == token {
		modified = token[:len(token)-1] + "B"
	}
	if _, err := Decrypt(modified, pass); err == nil {
		t.Fatal("un message modifie a ete accepte")
	}
}

func TestLeTexteNApparaitPas(t *testing.T) {
	token, _ := Encrypt("mot-de-passe-du-wifi", pass, nil)
	if strings.Contains(token, "wifi") {
		t.Fatal("le texte clair se retrouve dans le jeton")
	}
}

func TestBase64StandardNeDonneRien(t *testing.T) {
	token, _ := Encrypt("message secret", pass, nil)
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err == nil && len(raw) >= 3 && string(raw[:3]) == magic {
		t.Fatal("un decodeur base64 standard retrouve l'entete")
	}
}

func TestEntreesInvalides(t *testing.T) {
	if _, err := Encrypt("", pass, nil); err != ErrEmpty {
		t.Fatalf("texte vide : %v", err)
	}
	if _, err := Encrypt("texte", "", nil); err != ErrNoPassphrase {
		t.Fatalf("phrase vide : %v", err)
	}
	if _, err := Decrypt("bonjour tout le monde", pass); err != ErrNotFound {
		t.Fatalf("texte ordinaire : %v", err)
	}
	if _, err := Decrypt("AAAA", pass); err == nil {
		t.Fatal("jeton trop court accepte")
	}
}

func TestReperageSansMarqueur(t *testing.T) {
	token, _ := Encrypt("rendez-vous a 15h", pass, nil)

	entoure := "Salut ! Voici : \"" + token + "\" a bientot."
	if relu, err := DecryptText(entoure, pass); err != nil || relu != "rendez-vous a 15h" {
		t.Fatalf("message non repere au milieu d'une phrase : %q, %v", relu, err)
	}

	coupe := token[:30] + "\n" + token[30:60] + "\r\n " + token[60:]
	if relu, err := DecryptText(coupe, pass); err != nil || relu != "rendez-vous a 15h" {
		t.Fatalf("message coupe en plusieurs lignes non repere : %q, %v", relu, err)
	}
}

func TestTexteOrdinaireRefuse(t *testing.T) {
	textes := []string{
		"", "   ", "juste du texte",
		"Bonjour, comment vas-tu aujourd'hui ? On se voit demain.",
		strings.Repeat("motsanslespaces", 20),
	}
	for _, texte := range textes {
		if relu, err := DecryptText(texte, pass); err == nil {
			t.Fatalf("texte ordinaire accepte : %.20q a donne %q", texte, relu)
		}
	}
}

func TestLooksEncrypted(t *testing.T) {
	token, _ := Encrypt("deja chiffre", pass, nil)
	if !LooksEncrypted(token, pass) {
		t.Fatal("un message chiffre devrait etre reconnu")
	}
	if !LooksEncrypted("voici : "+token, pass) {
		t.Fatal("un message chiffre au milieu d'une phrase devrait etre reconnu")
	}
	for _, texte := range []string{"", "bonjour", "MC1~court", strings.Repeat("abc", 40)} {
		if LooksEncrypted(texte, pass) {
			t.Fatalf("faux positif sur %.20q", texte)
		}
	}
}

func TestAucunDebutReconnaissable(t *testing.T) {
	// Deux messages du meme texte ne doivent partager aucun debut commun :
	// sinon, ce debut serait lui-meme un marqueur.
	first, _ := Encrypt("bonjour", pass, nil)
	second, _ := Encrypt("bonjour", pass, nil)
	if first[:4] == second[:4] {
		t.Fatalf("les messages commencent tous par %q", first[:4])
	}
}

func TestSelPersonnel(t *testing.T) {
	salt := NewSalt()
	if len(salt) != SaltLen {
		t.Fatalf("taille de sel inattendue : %d", len(salt))
	}
	token, err := Encrypt("message rapide", pass, salt)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Decrypt(token, pass); got != "message rapide" {
		t.Fatal("aller-retour casse avec un sel impose")
	}
	// Un sel de mauvaise taille est remplace, pas refuse.
	if token, err = Encrypt("texte", pass, []byte("trop court")); err != nil {
		t.Fatal(err)
	}
	if got, _ := Decrypt(token, pass); got != "texte" {
		t.Fatal("sel invalide mal gere")
	}
}

func BenchmarkEncrypt(b *testing.B) {
	salt := NewSalt()
	if _, err := DeriveKey(pass, salt); err != nil { // chauffe la cle
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Encrypt("Rendez-vous au Massif samedi 9h", pass, salt); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	salt := NewSalt()
	token, _ := Encrypt("Rendez-vous au Massif samedi 9h", pass, salt)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Decrypt(token, pass); err != nil {
			b.Fatal(err)
		}
	}
}
