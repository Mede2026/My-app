"""Gestion des raccourcis clavier globaux (actifs dans toutes les applications)."""

from __future__ import annotations

from typing import Callable

import keyboard


class HotkeyError(Exception):
    """Raccourci invalide ou deja pris par une autre application."""


def normalize(combo: str) -> str:
    """Verifie un raccourci et renvoie sa forme canonique ("ctrl+alt+d")."""
    combo = (combo or "").strip().lower()
    if not combo:
        raise HotkeyError("Raccourci vide.")
    try:
        keyboard.parse_hotkey(combo)
    except Exception as exc:
        raise HotkeyError(f"Raccourci invalide : « {combo} »") from exc
    return combo


class HotkeyManager:
    """Enregistre les raccourcis et permet de les remplacer a chaud."""

    def __init__(self) -> None:
        self._handles: dict[str, object] = {}
        self._combos: dict[str, str] = {}

    def register(self, name: str, combo: str, callback: Callable[[], None]) -> None:
        """(Re)lie l'action `name` au raccourci `combo`."""
        combo = normalize(combo)
        self.unregister(name)
        try:
            # suppress=True : la combinaison n'est pas transmise a l'application
            # sous le curseur, elle est « avalee » par CryptoBulle.
            self._handles[name] = keyboard.add_hotkey(
                combo, callback, suppress=True, trigger_on_release=True
            )
        except Exception as exc:
            raise HotkeyError(f"Impossible d'activer « {combo} » : {exc}") from exc
        self._combos[name] = combo

    def unregister(self, name: str) -> None:
        handle = self._handles.pop(name, None)
        self._combos.pop(name, None)
        if handle is not None:
            try:
                keyboard.remove_hotkey(handle)
            except (KeyError, ValueError):
                pass

    def unregister_all(self) -> None:
        for name in list(self._handles):
            self.unregister(name)

    def combo(self, name: str) -> str:
        return self._combos.get(name, "")
