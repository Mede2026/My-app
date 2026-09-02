"""Tests de la lecture des combinaisons de touches (logique pure)."""

import unittest

from cryptobulle.hotkeys import (
    MOD_ALT,
    MOD_CONTROL,
    MOD_SHIFT,
    MOD_WIN,
    HotkeyError,
    from_parts,
    normalize,
    parse,
    pretty,
)


class TestParse(unittest.TestCase):
    def test_simple(self):
        self.assertEqual(parse("ctrl+alt+d"), (MOD_CONTROL | MOD_ALT, 0x44))

    def test_order_and_case_do_not_matter(self):
        self.assertEqual(parse("Alt+CTRL+d"), parse("ctrl+alt+d"))
        self.assertEqual(normalize("Alt+Ctrl+D"), "ctrl+alt+d")

    def test_all_modifiers(self):
        modifiers, key = parse("ctrl+alt+shift+win+f5")
        self.assertEqual(modifiers, MOD_CONTROL | MOD_ALT | MOD_SHIFT | MOD_WIN)
        self.assertEqual(key, 0x74)  # F5

    def test_synonyms(self):
        self.assertEqual(normalize("control+maj+echap"), "ctrl+shift+escape")
        self.assertEqual(normalize("ctrl+entree"), "ctrl+enter")
        self.assertEqual(normalize("windows+espace"), "win+space")

    def test_digits_and_letters(self):
        self.assertEqual(parse("ctrl+7")[1], 0x37)
        self.assertEqual(parse("ctrl+z")[1], 0x5A)

    def test_canonical_order(self):
        """L'ecriture canonique suit toujours l'ordre ctrl, alt, shift, win."""
        self.assertEqual(normalize("win+alt+space"), "alt+win+space")

    def test_round_trip(self):
        for combo in ("ctrl+alt+d", "ctrl+shift+f12", "alt+win+space", "ctrl+delete"):
            self.assertEqual(from_parts(*parse(combo)), combo)

    def test_rejects_bad_input(self):
        for combo in ("", "d", "ctrl", "ctrl+alt", "ctrl+a+b", "ctrl+touchemagique"):
            with self.assertRaises(HotkeyError, msg=combo):
                parse(combo)

    def test_pretty(self):
        self.assertEqual(pretty("ctrl+alt+d"), "Ctrl+Alt+D")
        self.assertEqual(pretty("ctrl+shift+f9"), "Ctrl+Shift+F9")


if __name__ == "__main__":
    unittest.main()
