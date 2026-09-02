"""Tests du moteur AES-GCM, quel que soit celui retenu sur la machine."""

import unittest

from cryptobulle import aesgcm


class TestBackend(unittest.TestCase):
    def test_known_answer(self):
        """Le moteur retenu doit reproduire exactement le vecteur de reference."""
        result = aesgcm.encrypt(
            aesgcm._KAT_KEY, aesgcm._KAT_NONCE, aesgcm._KAT_PLAIN, aesgcm._KAT_AAD
        )
        self.assertEqual(result, aesgcm._KAT_EXPECTED)

    def test_backend_name(self):
        self.assertIn(aesgcm.backend_name(), {"windows-cng", "cryptography"})

    def test_round_trip(self):
        key, nonce = bytes(range(32)), bytes(range(12))
        for message in (b"", b"court", b"x" * 100000):
            payload = aesgcm.encrypt(key, nonce, message, b"entete")
            self.assertEqual(len(payload), len(message) + aesgcm.TAG_LEN)
            self.assertEqual(aesgcm.decrypt(key, nonce, payload, b"entete"), message)

    def test_wrong_associated_data_is_rejected(self):
        key, nonce = bytes(32), bytes(12)
        payload = aesgcm.encrypt(key, nonce, b"secret", b"entete")
        with self.assertRaises(aesgcm.AuthenticationError):
            aesgcm.decrypt(key, nonce, payload, b"autre entete")

    def test_modified_payload_is_rejected(self):
        key, nonce = bytes(32), bytes(12)
        payload = bytearray(aesgcm.encrypt(key, nonce, b"secret", b""))
        payload[0] ^= 0x01
        with self.assertRaises(aesgcm.AuthenticationError):
            aesgcm.decrypt(key, nonce, bytes(payload), b"")

    def test_truncated_payload_is_rejected(self):
        with self.assertRaises(aesgcm.AuthenticationError):
            aesgcm.decrypt(bytes(32), bytes(12), b"court", b"")


if __name__ == "__main__":
    unittest.main()
