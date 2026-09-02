"""Rangement de la phrase secrete sur le disque.

Sous Windows, on passe par DPAPI (`CryptProtectData`) : le secret est chiffre
par le systeme avec les identifiants de la session Windows. Un autre compte
Windows, ou le meme fichier copie sur une autre machine, ne peut pas le relire.

DPAPI = Data Protection API, le coffre integre a Windows.

Ailleurs (ou si DPAPI echoue), on retombe sur un simple encodage base64 : ce
n'est PAS de la securite, seulement de quoi eviter une phrase secrete en clair
a l'ecran. L'appelant est prevenu par :func:`is_secure`.
"""

from __future__ import annotations

import base64
import ctypes
import sys

_IS_WINDOWS = sys.platform == "win32"


class _DataBlob(ctypes.Structure):
    # DWORD == c_ulong ; on evite ctypes.wintypes, absent hors Windows.
    _fields_ = [("cbData", ctypes.c_ulong), ("pbData", ctypes.POINTER(ctypes.c_char))]


def _blob(data: bytes) -> "_DataBlob":
    buffer = ctypes.create_string_buffer(data, len(data))
    return _DataBlob(len(data), ctypes.cast(buffer, ctypes.POINTER(ctypes.c_char)))


def _blob_bytes(blob: "_DataBlob") -> bytes:
    return ctypes.string_at(blob.pbData, blob.cbData)


def _dpapi(protect: bool, data: bytes) -> bytes | None:
    """Appelle CryptProtectData / CryptUnprotectData. None si indisponible."""
    if not _IS_WINDOWS:
        return None
    try:
        crypt32 = ctypes.windll.crypt32
        kernel32 = ctypes.windll.kernel32
        source, result = _blob(data), _DataBlob()
        function = crypt32.CryptProtectData if protect else crypt32.CryptUnprotectData
        ok = function(
            ctypes.byref(source), "CryptoBulle", None, None, None, 0, ctypes.byref(result)
        )
        if not ok:
            return None
        try:
            return _blob_bytes(result)
        finally:
            kernel32.LocalFree(result.pbData)
    except Exception:
        return None


def is_secure() -> bool:
    """Vrai si le secret sera protege par Windows (DPAPI)."""
    return _dpapi(True, b"test") is not None


def protect(secret: str) -> str:
    """Transforme une phrase secrete en chaine rangeable dans le fichier JSON."""
    raw = secret.encode("utf-8")
    sealed = _dpapi(True, raw)
    if sealed is not None:
        return "dpapi:" + base64.b64encode(sealed).decode("ascii")
    return "plain:" + base64.b64encode(raw).decode("ascii")


def unprotect(stored: str) -> str:
    """Operation inverse de :func:`protect`. Chaine vide si illisible."""
    if not stored:
        return ""
    scheme, _, payload = stored.partition(":")
    try:
        raw = base64.b64decode(payload)
    except Exception:
        return ""
    if scheme == "dpapi":
        opened = _dpapi(False, raw)
        return opened.decode("utf-8", errors="replace") if opened else ""
    if scheme == "plain":
        return raw.decode("utf-8", errors="replace")
    return ""
