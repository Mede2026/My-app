"""Tests des reglages et du rangement de la phrase secrete."""

import os
import tempfile
import unittest
from pathlib import Path


class TestConfig(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        os.environ["CRYPTOBULLE_HOME"] = self.temp.name

    def tearDown(self):
        os.environ.pop("CRYPTOBULLE_HOME", None)
        self.temp.cleanup()

    def test_defaults(self):
        from cryptobulle.config import Config

        config = Config()
        self.assertEqual(config.hotkey_decrypt, "ctrl+alt+d")
        self.assertEqual(config.hotkey_encrypt, "ctrl+alt+e")
        self.assertFalse(config.has_passphrase())

    def test_save_and_load(self):
        from cryptobulle.config import Config, config_path

        config = Config()
        config.passphrase = "ski au Massif"
        config.bubble_seconds = 5
        config.save()

        raw = Path(config_path()).read_text(encoding="utf-8")
        self.assertNotIn("Massif", raw)  # jamais en clair sur le disque

        loaded = Config.load()
        self.assertEqual(loaded.passphrase, "ski au Massif")
        self.assertEqual(loaded.bubble_seconds, 5)

    def test_unknown_keys_are_ignored(self):
        from cryptobulle.config import Config, config_path

        path = Path(config_path())
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text('{"hotkey_encrypt": "ctrl+f9", "vieux_reglage": 1}', encoding="utf-8")
        self.assertEqual(Config.load().hotkey_encrypt, "ctrl+f9")

    def test_broken_file_falls_back_to_defaults(self):
        from cryptobulle.config import Config, config_path

        path = Path(config_path())
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text("{ ceci n'est pas du JSON", encoding="utf-8")
        self.assertEqual(Config.load().hotkey_decrypt, "ctrl+alt+d")


if __name__ == "__main__":
    unittest.main()
