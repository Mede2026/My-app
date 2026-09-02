"""Tests du coeur cryptographique."""

import unittest

from cryptobulle.crypto import (
    PREFIX,
    CryptoError,
    _APP_ALPHABET,
    _STD_ALPHABET,
    decrypt,
    encrypt,
    find_token,
    looks_encrypted,
)

PASS = "batterie-metal-2022"


class TestAlphabet(unittest.TestCase):
    def test_alphabet_is_a_permutation(self):
        self.assertEqual(len(_APP_ALPHABET), 64)
        self.assertEqual(len(set(_APP_ALPHABET)), 64)
        self.assertEqual(sorted(_APP_ALPHABET), sorted(_STD_ALPHABET))


class TestRoundTrip(unittest.TestCase):
    def test_round_trip(self):
        for message in ("bonjour", "a", "x" * 5000, "accents: éàç ✓ 🥁", "ligne1\nligne2\t fin"):
            token = encrypt(message, PASS)
            self.assertTrue(token.startswith(PREFIX))
            self.assertEqual(decrypt(token, PASS), message)

    def test_output_hides_plaintext(self):
        token = encrypt("mot-de-passe-du-wifi", PASS)
        self.assertNotIn("wifi", token)
        self.assertNotIn("mot", token[len(PREFIX):])

    def test_two_encryptions_differ(self):
        first, second = encrypt("meme texte", PASS), encrypt("meme texte", PASS)
        self.assertNotEqual(first, second)  # sel et nonce aleatoires
        self.assertEqual(decrypt(first, PASS), decrypt(second, PASS))

    def test_wrong_passphrase(self):
        token = encrypt("secret", PASS)
        with self.assertRaises(CryptoError):
            decrypt(token, "mauvaise phrase")

    def test_tampered_token(self):
        token = encrypt("secret", PASS)
        flipped = "A" if token[-1] != "A" else "B"
        with self.assertRaises(CryptoError):
            decrypt(token[:-1] + flipped, PASS)

    def test_standard_base64_is_not_readable(self):
        import base64

        token = encrypt("message secret", PASS)
        raw = base64.urlsafe_b64decode(
            token[len(PREFIX):] + "=" * (-len(token[len(PREFIX):]) % 4)
        )
        self.assertNotEqual(raw[:3], b"MC1")  # l'alphabet maison brouille l'entete

    def test_empty_and_missing_passphrase(self):
        with self.assertRaises(CryptoError):
            encrypt("", PASS)
        with self.assertRaises(CryptoError):
            encrypt("texte", "")
        with self.assertRaises(CryptoError):
            decrypt("MC1~abc", "")

    def test_not_a_token(self):
        with self.assertRaises(CryptoError):
            decrypt("bonjour tout le monde", PASS)
        with self.assertRaises(CryptoError):
            decrypt(PREFIX + "trop-court", PASS)


class TestFindToken(unittest.TestCase):
    def test_token_inside_a_sentence(self):
        token = encrypt("rendez-vous a 15h", PASS)
        text = f'Salut ! Voici : "{token}" a bientot.'
        self.assertEqual(find_token(text), token)
        self.assertEqual(decrypt(find_token(text), PASS), "rendez-vous a 15h")

    def test_token_split_over_lines(self):
        token = encrypt("message coupe", PASS)
        wrapped = token[:30] + "\n" + token[30:60] + "\n " + token[60:]
        self.assertEqual(find_token(wrapped), token)

    def test_no_token(self):
        self.assertIsNone(find_token("juste du texte"))
        self.assertIsNone(find_token(""))
        self.assertFalse(looks_encrypted("MC1~court"))
        self.assertTrue(looks_encrypted(encrypt("oui", PASS)))


if __name__ == "__main__":
    unittest.main()
