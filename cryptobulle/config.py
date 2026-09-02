"""Reglages de CryptoBulle, ranges dans un fichier JSON.

Windows : %APPDATA%\\CryptoBulle\\config.json
Ailleurs : ~/.config/cryptobulle/config.json (pratique pour les tests)
"""

from __future__ import annotations

import binascii
import json
import os
import sys
from dataclasses import asdict, dataclass
from pathlib import Path

from . import APP_NAME
from . import secretstore

DEFAULT_HOTKEY_DECRYPT = "ctrl+alt+d"
DEFAULT_HOTKEY_ENCRYPT = "ctrl+alt+e"


def config_dir() -> Path:
    """Dossier ou vivent les reglages."""
    override = os.environ.get("CRYPTOBULLE_HOME")
    if override:
        return Path(override)
    if sys.platform == "win32":
        base = Path(os.environ.get("APPDATA", Path.home() / "AppData" / "Roaming"))
        return base / APP_NAME
    return Path.home() / ".config" / "cryptobulle"


def config_path() -> Path:
    return config_dir() / "config.json"


@dataclass
class Config:
    """Tous les reglages modifiables par l'utilisateur."""

    hotkey_decrypt: str = DEFAULT_HOTKEY_DECRYPT
    hotkey_encrypt: str = DEFAULT_HOTKEY_ENCRYPT
    # Phrase secrete protegee (voir secretstore) ; jamais en clair dans le JSON.
    passphrase_sealed: str = ""
    # Duree d'affichage de la bulle, en secondes (0 = jusqu'a fermeture manuelle).
    bubble_seconds: int = 12
    # Coller automatiquement le texte chiffre a la place de la selection.
    auto_paste: bool = True
    # Remettre l'ancien contenu du presse-papiers apres coup.
    restore_clipboard: bool = True
    launch_at_startup: bool = False
    # Sel personnel, tire au hasard une fois : il permet de garder la cle en
    # cache et de rendre le chiffrement instantane (voir crypto.encrypt).
    key_salt: str = ""
    theme: str = "sombre"  # "sombre" ou "clair"

    # --- phrase secrete -------------------------------------------------
    @property
    def passphrase(self) -> str:
        return secretstore.unprotect(self.passphrase_sealed)

    @passphrase.setter
    def passphrase(self, value: str) -> None:
        self.passphrase_sealed = secretstore.protect(value) if value else ""

    def has_passphrase(self) -> bool:
        return bool(self.passphrase_sealed)

    # --- sel personnel ---------------------------------------------------
    def salt(self) -> bytes:
        """Sel de l'utilisateur, cree au premier appel puis conserve."""
        from .crypto import SALT_LEN, new_salt

        try:
            raw = binascii.unhexlify(self.key_salt)
        except (binascii.Error, ValueError):
            raw = b""
        if len(raw) != SALT_LEN:
            raw = new_salt()
            self.key_salt = raw.hex()
        return raw

    # --- disque ---------------------------------------------------------
    @classmethod
    def load(cls) -> "Config":
        path = config_path()
        if not path.exists():
            return cls()
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return cls()
        known = {f for f in cls.__dataclass_fields__}
        return cls(**{k: v for k, v in data.items() if k in known})

    def save(self) -> None:
        path = config_path()
        path.parent.mkdir(parents=True, exist_ok=True)
        tmp = path.with_suffix(".json.tmp")
        tmp.write_text(json.dumps(asdict(self), indent=2, ensure_ascii=False), encoding="utf-8")
        tmp.replace(path)  # ecriture atomique : pas de fichier a moitie ecrit
        if sys.platform != "win32":
            try:
                path.chmod(0o600)
            except OSError:
                pass
