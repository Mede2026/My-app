"""Lecture et ecriture des combinaisons de touches ("ctrl+alt+d").

Ce module ne contient que de la logique pure : il traduit un texte en couple
(modificateurs, code de touche) comprehensible par `RegisterHotKey`, la
fonction de Windows qui reserve un raccourci pour toute la session.

`RegisterHotKey` est bien plus leger qu'un « hook » clavier : Windows ne
previent CryptoBulle que pour les deux combinaisons demandees, au lieu de lui
faire examiner chaque touche tapee sur la machine.
"""

from __future__ import annotations

import sys

MOD_ALT = 0x0001
MOD_CONTROL = 0x0002
MOD_SHIFT = 0x0004
MOD_WIN = 0x0008
MOD_NOREPEAT = 0x4000

_MODIFIERS = {
    "ctrl": MOD_CONTROL, "control": MOD_CONTROL, "ctl": MOD_CONTROL,
    "alt": MOD_ALT, "altgr": MOD_ALT,
    "shift": MOD_SHIFT, "maj": MOD_SHIFT,
    "win": MOD_WIN, "windows": MOD_WIN, "cmd": MOD_WIN, "super": MOD_WIN,
}
_MODIFIER_ORDER = (("ctrl", MOD_CONTROL), ("alt", MOD_ALT), ("shift", MOD_SHIFT), ("win", MOD_WIN))

# Touches nommees : nom canonique -> code de touche virtuelle Windows.
_NAMED_KEYS = {
    "backspace": 0x08, "tab": 0x09, "enter": 0x0D, "escape": 0x1B, "space": 0x20,
    "pageup": 0x21, "pagedown": 0x22, "end": 0x23, "home": 0x24,
    "left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
    "insert": 0x2D, "delete": 0x2E,
    "plus": 0xBB, "minus": 0xBD, "comma": 0xBC, "period": 0xBE,
}
_NAMED_KEYS.update({f"f{index}": 0x6F + index for index in range(1, 25)})  # F1 = 0x70

# Synonymes acceptes a la saisie (anglais, francais, abreviations).
_KEY_ALIASES = {
    "esc": "escape", "echap": "escape", "entree": "enter", "return": "enter",
    "espace": "space", "suppr": "delete", "inser": "insert", "retour": "backspace",
    "haut": "up", "bas": "down", "gauche": "left", "droite": "right",
    "fin": "end", "debut": "home", "pgup": "pageup", "pgdn": "pagedown",
    "pagesuivante": "pagedown", "pageprecedente": "pageup",
    "tabulation": "tab", "virgule": "comma", "point": "period",
}
_VK_TO_NAME = {code: name for name, code in _NAMED_KEYS.items()}


class HotkeyError(Exception):
    """Combinaison invalide, ou refusee par Windows."""


def _key_code(token: str) -> int:
    """Code de touche virtuelle pour un morceau de combinaison."""
    token = _KEY_ALIASES.get(token, token)
    if token in _NAMED_KEYS:
        return _NAMED_KEYS[token]
    if len(token) == 1:
        if token.isdigit():
            return 0x30 + int(token)
        if "a" <= token <= "z":
            return 0x41 + (ord(token) - ord("a"))
        if sys.platform == "win32":  # caracteres propres a la disposition clavier
            from .winapi import user32

            scan = user32.VkKeyScanW(ord(token))
            if scan != -1:
                return scan & 0xFF
    raise HotkeyError(f"Touche inconnue : « {token} »")


def parse(combo: str) -> tuple[int, int]:
    """Traduit "ctrl+alt+d" en (modificateurs, code de touche)."""
    tokens = [part.strip().lower() for part in (combo or "").split("+") if part.strip()]
    if not tokens:
        raise HotkeyError("Raccourci vide.")

    modifiers = 0
    keys = []
    for token in tokens:
        if token in _MODIFIERS:
            modifiers |= _MODIFIERS[token]
        else:
            keys.append(token)

    if len(keys) != 1:
        raise HotkeyError("Un raccourci doit contenir exactement une touche principale.")
    if not modifiers:
        raise HotkeyError("Ajoutez au moins Ctrl, Alt, Maj ou Windows.")
    return modifiers, _key_code(keys[0])


def normalize(combo: str) -> str:
    """Forme canonique d'un raccourci : "Alt+ctrl+D" devient "ctrl+alt+d"."""
    modifiers, key = parse(combo)
    return from_parts(modifiers, key)


def from_parts(modifiers: int, key: int) -> str:
    """Ecrit un raccourci a partir des codes Windows."""
    parts = [name for name, flag in _MODIFIER_ORDER if modifiers & flag]
    if key in _VK_TO_NAME:
        parts.append(_VK_TO_NAME[key])
    elif 0x30 <= key <= 0x39:
        parts.append(chr(key))
    elif 0x41 <= key <= 0x5A:
        parts.append(chr(key).lower())
    else:
        parts.append(f"0x{key:02x}")
    return "+".join(parts)


def pretty(combo: str) -> str:
    """Version affichable : "ctrl+alt+d" devient "Ctrl+Alt+D"."""
    try:
        combo = normalize(combo)
    except HotkeyError:
        pass
    return "+".join(part.capitalize() if len(part) > 1 else part.upper() for part in combo.split("+"))
