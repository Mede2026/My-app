"""AES-256-GCM, fourni de preference par Windows lui-meme.

Deux implementations possibles :

1. **CNG** (« Cryptography API: Next Generation »), le moteur cryptographique
   integre a Windows, appele via `ctypes`. Aucune bibliotheque a installer ni a
   embarquer dans l'executable : c'est l'option la plus legere et la plus rapide.
2. La bibliotheque **cryptography**, utilisee seulement si elle est installee
   (developpement et tests hors Windows).

Le choix est verifie au demarrage par un test a resultat connu : on chiffre un
message dont on connait deja le resultat exact. Si le moteur ne redonne pas ce
resultat, on ne s'en sert pas.
"""

from __future__ import annotations

import ctypes
import sys
from functools import lru_cache

TAG_LEN = 16

# Vecteur de verification (cle, nonce, donnees associees, texte -> resultat).
_KAT_KEY = bytes(range(32))
_KAT_NONCE = bytes(range(12))
_KAT_AAD = b"CryptoBulle-KAT"
_KAT_PLAIN = b"vecteur de test CryptoBulle 1234567890"
_KAT_EXPECTED = bytes.fromhex(
    "3167b56fa090b03be924b7ffd49a0c4dc0a4fe4484141d09540b80a52c5b3386"
    "342699c496f18ec8f38669645a2841d6da25c479bffe"
)


class AuthenticationError(Exception):
    """Le message a ete modifie, ou la cle n'est pas la bonne."""


class CryptoBackendError(Exception):
    """Aucun moteur AES-GCM utilisable."""


# --- moteur Windows (CNG) ---------------------------------------------------
if sys.platform == "win32":
    from ctypes import wintypes

    _bcrypt = ctypes.WinDLL("bcrypt")
    _STATUS_SUCCESS = 0
    _STATUS_AUTH_TAG_MISMATCH = ctypes.c_ulong(0xC000A002).value
    _BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO_VERSION = 1

    class _AuthInfo(ctypes.Structure):
        _fields_ = [
            ("cbSize", wintypes.ULONG),
            ("dwInfoVersion", wintypes.ULONG),
            ("pbNonce", ctypes.POINTER(ctypes.c_char)),
            ("cbNonce", wintypes.ULONG),
            ("pbAuthData", ctypes.POINTER(ctypes.c_char)),
            ("cbAuthData", wintypes.ULONG),
            ("pbTag", ctypes.POINTER(ctypes.c_char)),
            ("cbTag", wintypes.ULONG),
            ("pbMacContext", ctypes.POINTER(ctypes.c_char)),
            ("cbMacContext", wintypes.ULONG),
            ("cbAAD", wintypes.ULONG),
            ("cbData", ctypes.c_ulonglong),
            ("dwFlags", wintypes.ULONG),
        ]

    class _CngKey:
        """Cle AES prete a l'emploi, gardee en cache entre deux appels."""

        def __init__(self, handle, key_object) -> None:
            self.handle = handle
            self._key_object = key_object  # doit rester en vie avec la cle

        def __del__(self) -> None:  # pragma: no cover - libere par Windows
            try:
                _bcrypt.BCryptDestroyKey(self.handle)
            except Exception:
                pass

    @lru_cache(maxsize=1)
    def _algorithm():
        """Ouvre le fournisseur AES en mode GCM (une seule fois par processus)."""
        handle = ctypes.c_void_p()
        status = _bcrypt.BCryptOpenAlgorithmProvider(
            ctypes.byref(handle), "AES", None, 0
        )
        if status != _STATUS_SUCCESS:
            raise CryptoBackendError(f"BCryptOpenAlgorithmProvider : 0x{status & 0xFFFFFFFF:08X}")
        mode = ctypes.create_unicode_buffer("ChainingModeGCM")
        status = _bcrypt.BCryptSetProperty(
            handle, "ChainingMode", ctypes.cast(mode, ctypes.POINTER(ctypes.c_char)),
            ctypes.sizeof(mode), 0,
        )
        if status != _STATUS_SUCCESS:
            raise CryptoBackendError(f"BCryptSetProperty : 0x{status & 0xFFFFFFFF:08X}")
        return handle

    def _object_length(algorithm) -> int:
        size = wintypes.ULONG()
        written = wintypes.ULONG()
        status = _bcrypt.BCryptGetProperty(
            algorithm, "ObjectLength", ctypes.byref(size), ctypes.sizeof(size),
            ctypes.byref(written), 0,
        )
        if status != _STATUS_SUCCESS:
            raise CryptoBackendError(f"BCryptGetProperty : 0x{status & 0xFFFFFFFF:08X}")
        return size.value

    @lru_cache(maxsize=8)
    def _key(key: bytes) -> "_CngKey":
        algorithm = _algorithm()
        key_object = ctypes.create_string_buffer(_object_length(algorithm))
        handle = ctypes.c_void_p()
        status = _bcrypt.BCryptGenerateSymmetricKey(
            algorithm, ctypes.byref(handle),
            ctypes.cast(key_object, ctypes.POINTER(ctypes.c_char)), len(key_object),
            key, len(key), 0,
        )
        if status != _STATUS_SUCCESS:
            raise CryptoBackendError(f"BCryptGenerateSymmetricKey : 0x{status & 0xFFFFFFFF:08X}")
        return _CngKey(handle, key_object)

    def _auth_info(nonce: bytes, aad: bytes, tag: ctypes.Array):
        """Prepare la structure GCM ET les tampons qu'elle designe.

        Les tampons sont renvoyes avec la structure : Python ne doit surtout pas
        les liberer avant la fin de l'appel a Windows.
        """
        nonce_buffer = ctypes.create_string_buffer(nonce, len(nonce))
        aad_buffer = ctypes.create_string_buffer(aad, len(aad)) if aad else None

        info = _AuthInfo()
        info.cbSize = ctypes.sizeof(_AuthInfo)
        info.dwInfoVersion = _BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO_VERSION
        info.pbNonce = ctypes.cast(nonce_buffer, ctypes.POINTER(ctypes.c_char))
        info.cbNonce = len(nonce)
        if aad_buffer is not None:
            info.pbAuthData = ctypes.cast(aad_buffer, ctypes.POINTER(ctypes.c_char))
            info.cbAuthData = len(aad)
        info.pbTag = ctypes.cast(tag, ctypes.POINTER(ctypes.c_char))
        info.cbTag = len(tag)
        return info, (nonce_buffer, aad_buffer, tag)

    def _cng_encrypt(key: bytes, nonce: bytes, plaintext: bytes, aad: bytes) -> bytes:
        tag = ctypes.create_string_buffer(TAG_LEN)
        info, buffers = _auth_info(nonce, aad, tag)
        output = ctypes.create_string_buffer(max(1, len(plaintext)))
        written = wintypes.ULONG()
        status = _bcrypt.BCryptEncrypt(
            _key(key).handle, plaintext, len(plaintext), ctypes.byref(info), None, 0,
            output, len(plaintext), ctypes.byref(written), 0,
        )
        if status != _STATUS_SUCCESS:
            raise CryptoBackendError(f"BCryptEncrypt : 0x{status & 0xFFFFFFFF:08X}")
        result = output.raw[: written.value] + tag.raw[:TAG_LEN]
        del buffers
        return result

    def _cng_decrypt(key: bytes, nonce: bytes, payload: bytes, aad: bytes) -> bytes:
        if len(payload) < TAG_LEN:
            raise AuthenticationError("Message trop court.")
        ciphertext, tag_bytes = payload[:-TAG_LEN], payload[-TAG_LEN:]
        tag = ctypes.create_string_buffer(tag_bytes, TAG_LEN)
        info, buffers = _auth_info(nonce, aad, tag)
        output = ctypes.create_string_buffer(max(1, len(ciphertext)))
        written = wintypes.ULONG()
        status = _bcrypt.BCryptDecrypt(
            _key(key).handle, ciphertext, len(ciphertext), ctypes.byref(info), None, 0,
            output, len(ciphertext), ctypes.byref(written), 0,
        )
        if status == _STATUS_AUTH_TAG_MISMATCH:
            raise AuthenticationError("Signature du message invalide.")
        if status != _STATUS_SUCCESS:
            raise CryptoBackendError(f"BCryptDecrypt : 0x{status & 0xFFFFFFFF:08X}")
        result = output.raw[: written.value]
        del buffers
        return result


# --- moteur de repli : bibliotheque cryptography -----------------------------
def _library_backend():
    from cryptography.exceptions import InvalidTag
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    def encrypt(key: bytes, nonce: bytes, plaintext: bytes, aad: bytes) -> bytes:
        return AESGCM(key).encrypt(nonce, plaintext, aad)

    def decrypt(key: bytes, nonce: bytes, payload: bytes, aad: bytes) -> bytes:
        try:
            return AESGCM(key).decrypt(nonce, payload, aad)
        except InvalidTag as exc:
            raise AuthenticationError("Signature du message invalide.") from exc

    return "cryptography", encrypt, decrypt


def _check(encrypt) -> bool:
    """Test a resultat connu : le moteur donne-t-il le bon chiffre ?"""
    try:
        return encrypt(_KAT_KEY, _KAT_NONCE, _KAT_PLAIN, _KAT_AAD) == _KAT_EXPECTED
    except Exception:
        return False


@lru_cache(maxsize=1)
def _backend():
    """Choisit le moteur une fois pour toutes : (nom, chiffrer, dechiffrer)."""
    errors = []
    if sys.platform == "win32":
        if _check(_cng_encrypt):
            return "windows-cng", _cng_encrypt, _cng_decrypt
        errors.append("moteur Windows CNG indisponible")
    try:
        backend = _library_backend()
    except Exception as exc:
        errors.append(f"cryptography : {exc}")
    else:
        if _check(backend[1]):
            return backend
        errors.append("cryptography : test a resultat connu echoue")
    raise CryptoBackendError("Aucun moteur AES-GCM utilisable (" + " ; ".join(errors) + ")")


def backend_name() -> str:
    """Nom du moteur reellement utilise (utile dans l'onglet Aide)."""
    return _backend()[0]


def encrypt(key: bytes, nonce: bytes, plaintext: bytes, aad: bytes = b"") -> bytes:
    """Chiffre et signe : renvoie le chiffre suivi de la signature (16 octets)."""
    return _backend()[1](key, nonce, plaintext, aad)


def decrypt(key: bytes, nonce: bytes, payload: bytes, aad: bytes = b"") -> bytes:
    """Verifie la signature et dechiffre. Leve AuthenticationError si non conforme."""
    return _backend()[2](key, nonce, payload, aad)
