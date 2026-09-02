"""Coeur cryptographique de CryptoBulle.

Format « MC1 » (propre a l'application) :

    MC1~<charge utile encodee dans l'alphabet maison>

La charge utile binaire est :

    MAGIC(3) | VERSION(1) | SEL(16) | NONCE(12) | CHIFFRE+TAG(...)

- La cle est derivee avec scrypt a partir de la phrase secrete de l'utilisateur
  ET d'un « poivre » constant propre a l'application.
- Le chiffrement est AES-256-GCM : il chiffre ET authentifie (toute
  modification d'un seul caractere rend le dechiffrement impossible).
- La sortie est ensuite encodee avec un alphabet base64 permute, specifique a
  CryptoBulle : un base64 standard ne donnera donc rien de lisible.
"""

from __future__ import annotations

import base64
import hashlib
import os
import re
from functools import lru_cache

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

MAGIC = b"MC1"
VERSION = 1
PREFIX = "MC1~"

SALT_LEN = 16
NONCE_LEN = 12
KEY_LEN = 32
HEADER_LEN = len(MAGIC) + 1 + SALT_LEN + NONCE_LEN

# Parametres scrypt : ~16 Mo de memoire, environ 50 ms sur une machine moderne.
SCRYPT_N = 2 ** 14
SCRYPT_R = 8
SCRYPT_P = 1

# Constante propre a l'application : melangee a la phrase secrete, elle fait
# qu'une meme phrase secrete ne donne pas la meme cle ailleurs.
APP_PEPPER = b"CryptoBulle-v1|poivre-applicatif|8f2c1d9a4b6e"

# Alphabet maison (permutation des 64 caracteres du base64 « urlsafe »).
_STD_ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
_APP_ALPHABET = "AfhmdLEr3GMxg2S_a91UZnTlHOI5KYzevFCwNQ-P4tWV7cpXRbDuykjJsoi6q80B"

_ENCODE_TABLE = str.maketrans(_STD_ALPHABET, _APP_ALPHABET)
_DECODE_TABLE = str.maketrans(_APP_ALPHABET, _STD_ALPHABET)

# Un jeton complet dans un texte : le prefixe suivi d'au moins 40 caracteres.
TOKEN_RE = re.compile(re.escape(PREFIX) + "[" + re.escape(_APP_ALPHABET) + "]{40,}")


class CryptoError(Exception):
    """Erreur de chiffrement ou de dechiffrement lisible par l'utilisateur."""


def _encode(data: bytes) -> str:
    """Encode des octets dans l'alphabet maison (sans caractere de bourrage)."""
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=").translate(_ENCODE_TABLE)


def _decode(text: str) -> bytes:
    """Operation inverse de :func:`_encode`."""
    standard = text.translate(_DECODE_TABLE)
    padding = "=" * (-len(standard) % 4)
    try:
        return base64.urlsafe_b64decode(standard + padding)
    except Exception as exc:  # binascii.Error et compagnie
        raise CryptoError("Le texte n'est pas un message CryptoBulle valide.") from exc


@lru_cache(maxsize=64)
def _derive_key(passphrase: str, salt: bytes) -> bytes:
    """Derive une cle AES-256 a partir de la phrase secrete et du sel.

    Le resultat est mis en cache : dechiffrer plusieurs fois le meme message
    reste instantane, alors qu'un scrypt complet coute ~50 ms.
    """
    material = passphrase.encode("utf-8") + b"|" + APP_PEPPER
    return hashlib.scrypt(
        material,
        salt=salt,
        n=SCRYPT_N,
        r=SCRYPT_R,
        p=SCRYPT_P,
        maxmem=64 * 1024 * 1024,
        dklen=KEY_LEN,
    )


def encrypt(plaintext: str, passphrase: str) -> str:
    """Chiffre `plaintext` et renvoie un jeton `MC1~...`."""
    if not passphrase:
        raise CryptoError("Aucune phrase secrete n'est configuree.")
    if plaintext == "":
        raise CryptoError("Il n'y a rien a chiffrer.")

    salt = os.urandom(SALT_LEN)
    nonce = os.urandom(NONCE_LEN)
    header = MAGIC + bytes([VERSION]) + salt + nonce
    key = _derive_key(passphrase, salt)
    # Le header sert de donnees associees : il est authentifie, pas chiffre.
    ciphertext = AESGCM(key).encrypt(nonce, plaintext.encode("utf-8"), header)
    return PREFIX + _encode(header + ciphertext)


def decrypt(token: str, passphrase: str) -> str:
    """Dechiffre un jeton `MC1~...` et renvoie le texte d'origine."""
    if not passphrase:
        raise CryptoError("Aucune phrase secrete n'est configuree.")

    token = token.strip()
    if not token.startswith(PREFIX):
        raise CryptoError("Ce texte ne contient pas de message CryptoBulle.")

    raw = _decode(token[len(PREFIX):])
    if len(raw) <= HEADER_LEN or raw[:3] != MAGIC:
        raise CryptoError("Message CryptoBulle incomplet ou abime.")
    if raw[3] != VERSION:
        raise CryptoError(f"Message cree avec une version plus recente (v{raw[3]}).")

    header = raw[:HEADER_LEN]
    salt = raw[4:4 + SALT_LEN]
    nonce = raw[4 + SALT_LEN:HEADER_LEN]
    key = _derive_key(passphrase, salt)
    try:
        plaintext = AESGCM(key).decrypt(nonce, raw[HEADER_LEN:], header)
    except InvalidTag as exc:
        raise CryptoError(
            "Dechiffrement impossible : phrase secrete differente ou message modifie."
        ) from exc
    return plaintext.decode("utf-8", errors="replace")


def find_token(text: str) -> str | None:
    """Retourne le premier jeton CryptoBulle trouve dans `text`, sinon None.

    Utile quand l'utilisateur selectionne un peu de texte autour du message
    (guillemets, espaces, ligne de courriel...).
    """
    if not text:
        return None
    cleaned = "".join(text.split())  # un jeton coupe sur plusieurs lignes reste valide
    match = TOKEN_RE.search(cleaned)
    return match.group(0) if match else None


def looks_encrypted(text: str) -> bool:
    """Vrai si le texte contient un jeton CryptoBulle."""
    return find_token(text) is not None
