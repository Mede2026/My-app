"""Presse-papiers et frappes simulees, directement via l'API de Windows.

Windows ne permet pas de lire « le texte selectionne » : on envoie donc un
Ctrl+C, on lit le presse-papiers, puis on remet son contenu d'origine.

Le changement est detecte grace au *numero de sequence* du presse-papiers, un
compteur que Windows incremente a chaque modification. C'est immediat, alors
qu'une comparaison de textes obligerait a relire le contenu en boucle.
"""

from __future__ import annotations

import ctypes
import time

from .winapi import (
    CF_UNICODETEXT,
    GMEM_MOVEABLE,
    INPUT,
    INPUT_KEYBOARD,
    KEYEVENTF_KEYUP,
    VK_CONTROL,
    VK_LWIN,
    VK_MENU,
    VK_RWIN,
    VK_SHIFT,
    kernel32,
    user32,
)

VK_C = 0x43
VK_V = 0x56
_MODIFIER_KEYS = (VK_CONTROL, VK_MENU, VK_SHIFT, VK_LWIN, VK_RWIN)


class ClipboardError(Exception):
    """Le presse-papiers est reste inaccessible."""


def _open(timeout: float = 0.4) -> bool:
    """Ouvre le presse-papiers, en reessayant : un autre logiciel peut le tenir."""
    deadline = time.monotonic() + timeout
    while True:
        if user32.OpenClipboard(None):
            return True
        if time.monotonic() >= deadline:
            return False
        time.sleep(0.005)


def sequence_number() -> int:
    """Compteur incremente par Windows a chaque changement du presse-papiers."""
    return user32.GetClipboardSequenceNumber()


def get_clipboard() -> str:
    """Texte present dans le presse-papiers ("" s'il n'y en a pas)."""
    if not _open():
        return ""
    try:
        handle = user32.GetClipboardData(CF_UNICODETEXT)
        if not handle:
            return ""
        pointer = kernel32.GlobalLock(handle)
        if not pointer:
            return ""
        try:
            return ctypes.wstring_at(pointer)
        finally:
            kernel32.GlobalUnlock(handle)
    finally:
        user32.CloseClipboard()


def set_clipboard(text: str) -> None:
    """Remplace le contenu du presse-papiers par `text`."""
    if not _open():
        raise ClipboardError("Presse-papiers occupe par un autre logiciel.")
    try:
        user32.EmptyClipboard()
        data = ctypes.create_unicode_buffer(text)
        size = ctypes.sizeof(data)
        handle = kernel32.GlobalAlloc(GMEM_MOVEABLE, size)
        if not handle:
            raise ClipboardError("Memoire insuffisante pour le presse-papiers.")
        pointer = kernel32.GlobalLock(handle)
        if not pointer:
            kernel32.GlobalFree(handle)
            raise ClipboardError("Presse-papiers verrouille.")
        ctypes.memmove(pointer, data, size)
        kernel32.GlobalUnlock(handle)
        if not user32.SetClipboardData(CF_UNICODETEXT, handle):
            kernel32.GlobalFree(handle)
            raise ClipboardError("Windows a refuse l'ecriture du presse-papiers.")
        # Succes : Windows devient proprietaire du bloc, il ne faut pas le liberer.
    finally:
        user32.CloseClipboard()


def _key_event(virtual_key: int, released: bool) -> INPUT:
    event = INPUT()
    event.type = INPUT_KEYBOARD
    event.u.ki.wVk = virtual_key
    event.u.ki.wScan = user32.MapVirtualKeyW(virtual_key, 0)
    event.u.ki.dwFlags = KEYEVENTF_KEYUP if released else 0
    return event


def _send(events: list[INPUT]) -> None:
    array = (INPUT * len(events))(*events)
    user32.SendInput(len(events), array, ctypes.sizeof(INPUT))


def send_shortcut(virtual_key: int) -> None:
    """Envoie Ctrl + `virtual_key`, apres avoir relache les touches tenues.

    Au moment ou le raccourci se declenche, l'utilisateur tient encore Ctrl+Alt :
    si on n'annonce pas leur relachement, l'application cible recevrait
    Ctrl+Alt+C au lieu de Ctrl+C.
    """
    events = [
        _key_event(key, released=True)
        for key in _MODIFIER_KEYS
        if user32.GetAsyncKeyState(key) & 0x8000
    ]
    events += [
        _key_event(VK_CONTROL, False),
        _key_event(virtual_key, False),
        _key_event(virtual_key, True),
        _key_event(VK_CONTROL, True),
    ]
    _send(events)


def read_selection(timeout: float = 0.45) -> tuple[str, str]:
    """Copie la selection courante.

    Renvoie `(texte_selectionne, ancien_presse_papiers)`. Le texte selectionne
    est vide si rien n'etait selectionne.
    """
    previous = get_clipboard()
    before = sequence_number()
    send_shortcut(VK_C)

    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if sequence_number() != before:
            text = get_clipboard()
            if text:
                return text, previous
        time.sleep(0.004)
    return "", previous


def paste_text(text: str, restore: str | None = None, delay: float = 0.2) -> None:
    """Met `text` dans le presse-papiers et l'insere avec Ctrl+V."""
    set_clipboard(text)
    send_shortcut(VK_V)
    if restore is not None:
        time.sleep(delay)  # laisser l'application cible lire le presse-papiers
        try:
            set_clipboard(restore)
        except ClipboardError:
            pass
