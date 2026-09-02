"""Lecture de la selection et collage, via le presse-papiers de Windows.

Windows ne permet pas de lire directement « le texte selectionne » : on simule
donc Ctrl+C, on lit le presse-papiers, puis on remet son ancien contenu.
"""

from __future__ import annotations

import time

import keyboard
import pyperclip

# Touches modificatrices possiblement encore enfoncees quand le raccourci part.
_MODIFIERS = ("ctrl", "alt", "shift", "left windows", "right windows")


class ClipboardError(Exception):
    """Le presse-papiers n'a pas repondu."""


def get_clipboard() -> str:
    """Contenu texte du presse-papiers ("" si autre chose ou s'il est vide)."""
    for attempt in range(5):  # un autre programme peut le verrouiller un instant
        try:
            return pyperclip.paste() or ""
        except Exception:
            time.sleep(0.03 * (attempt + 1))
    return ""


def set_clipboard(text: str) -> None:
    """Ecrit du texte dans le presse-papiers."""
    last = None
    for attempt in range(5):
        try:
            pyperclip.copy(text)
            return
        except Exception as exc:
            last = exc
            time.sleep(0.03 * (attempt + 1))
    raise ClipboardError(f"Presse-papiers inaccessible : {last}")


def release_modifiers() -> None:
    """Relache Ctrl/Alt/Maj pour que le Ctrl+C simule parte proprement."""
    for key in _MODIFIERS:
        try:
            keyboard.release(key)
        except Exception:
            pass
    time.sleep(0.04)


def read_selection(timeout: float = 0.6) -> tuple[str, str]:
    """Copie la selection courante.

    Retourne `(texte_selectionne, ancien_presse_papiers)`. Le texte est vide
    si rien n'etait selectionne.
    """
    previous = get_clipboard()
    sentinel = "\x00cryptobulle\x00"
    try:
        set_clipboard(sentinel)
    except ClipboardError:
        sentinel = previous  # tant pis, on comparera avec l'ancien contenu

    release_modifiers()
    keyboard.send("ctrl+c")

    deadline = time.time() + timeout
    while time.time() < deadline:
        time.sleep(0.03)
        current = get_clipboard()
        if current and current != sentinel:
            return current, previous
    return "", previous


def paste_text(text: str, restore: str | None = None, delay: float = 0.25) -> None:
    """Met `text` dans le presse-papiers et l'insere avec Ctrl+V.

    Si `restore` est fourni, l'ancien contenu revient apres le collage.
    """
    set_clipboard(text)
    release_modifiers()
    keyboard.send("ctrl+v")
    if restore is not None:
        time.sleep(delay)  # laisser l'application cible lire le presse-papiers
        try:
            set_clipboard(restore)
        except ClipboardError:
            pass
