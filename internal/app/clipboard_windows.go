package app

import (
	"errors"
	"time"

	"github.com/mede2026/cryptobulle/internal/w32"
)

// Windows ne permet pas de lire directement « le texte selectionne » : on
// simule Ctrl+C, on lit le presse-papiers, puis on remet son contenu d'origine.
//
// Le changement est repere grace au numero de sequence du presse-papiers, un
// compteur que Windows incremente a chaque modification. La reaction est donc
// immediate, sans attendre un delai fixe.

var errClipboardBusy = errors.New("presse-papiers occupé par un autre logiciel")

// openClipboard reessaie brievement : un autre programme peut le tenir.
func openClipboard() bool {
	for attempt := 0; attempt < 40; attempt++ {
		if w32.OpenClipboard() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func clipboardText() string {
	if !openClipboard() {
		return ""
	}
	defer w32.CloseClipboard()
	return w32.GetClipboardText()
}

func setClipboardText(text string) error {
	if !openClipboard() {
		return errClipboardBusy
	}
	defer w32.CloseClipboard()
	w32.EmptyClipboard()
	if !w32.SetClipboardText(text) {
		return errors.New("Windows a refusé l'écriture du presse-papiers")
	}
	return nil
}

// sendShortcut envoie Ctrl + une touche, apres avoir annonce le relachement des
// touches encore enfoncees. Au moment ou le raccourci part, l'utilisateur tient
// encore Ctrl+Alt : sans cela, l'application cible recevrait Ctrl+Alt+C.
func sendShortcut(key uint32) {
	var inputs []w32.INPUT
	for _, modifier := range []uint32{w32.VK_CONTROL, w32.VK_MENU, w32.VK_SHIFT, w32.VK_LWIN, w32.VK_RWIN} {
		if w32.KeyIsDown(modifier) {
			inputs = append(inputs, keyEvent(modifier, true))
		}
	}
	inputs = append(inputs,
		keyEvent(w32.VK_CONTROL, false),
		keyEvent(key, false),
		keyEvent(key, true),
		keyEvent(w32.VK_CONTROL, true),
	)
	w32.SendInputs(inputs)
}

func keyEvent(key uint32, released bool) w32.INPUT {
	input := w32.INPUT{Type: w32.INPUT_KEYBOARD}
	input.Ki.Vk = uint16(key)
	input.Ki.Scan = w32.MapVirtualKey(key)
	if released {
		input.Ki.Flags = w32.KEYEVENTF_KEYUP
	}
	return input
}

// readSelection copie la selection courante et renvoie (selection, ancien
// presse-papiers). La selection est vide si rien n'etait selectionne.
func readSelection() (string, string) {
	previous := clipboardText()
	before := w32.ClipboardSequence()
	sendShortcut(w32.VK_C)

	deadline := time.Now().Add(450 * time.Millisecond)
	for time.Now().Before(deadline) {
		if w32.ClipboardSequence() != before {
			if text := clipboardText(); text != "" {
				return text, previous
			}
		}
		time.Sleep(4 * time.Millisecond)
	}
	return "", previous
}

// pasteText met le texte dans le presse-papiers et l'insere avec Ctrl+V.
// Si restore n'est pas vide, l'ancien contenu revient apres le collage.
func pasteText(text, restore string) error {
	if err := setClipboardText(text); err != nil {
		return err
	}
	sendShortcut(w32.VK_V)
	if restore != "" {
		time.Sleep(200 * time.Millisecond) // laisser l'application cible lire
		_ = setClipboardText(restore)
	}
	return nil
}
