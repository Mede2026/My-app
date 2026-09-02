"""Lancement automatique de CryptoBulle au demarrage de Windows.

On ecrit une valeur dans la base de registre, sous la cle « Run » de
l'utilisateur courant (aucun droit administrateur necessaire).
"""

from __future__ import annotations

import sys
from pathlib import Path

from . import APP_NAME

_RUN_KEY = r"Software\Microsoft\Windows\CurrentVersion\Run"


def _winreg():
    if sys.platform != "win32":
        return None
    import winreg

    return winreg


def command_line() -> str:
    """Commande a lancer au demarrage, guillemets compris."""
    executable = Path(sys.executable).resolve()
    if getattr(sys, "frozen", False):  # binaire construit par PyInstaller
        return f'"{executable}"'
    script = Path(__file__).resolve().parent.parent / "cryptobulle.pyw"
    return f'"{executable}" "{script}"'


def is_enabled() -> bool:
    winreg = _winreg()
    if winreg is None:
        return False
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, _RUN_KEY) as key:
            value, _ = winreg.QueryValueEx(key, APP_NAME)
            return bool(value)
    except OSError:
        return False


def set_enabled(enabled: bool) -> bool:
    """Active ou desactive le demarrage automatique. Renvoie l'etat obtenu."""
    winreg = _winreg()
    if winreg is None:
        return False
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, _RUN_KEY, 0, winreg.KEY_SET_VALUE) as key:
            if enabled:
                winreg.SetValueEx(key, APP_NAME, 0, winreg.REG_SZ, command_line())
            else:
                try:
                    winreg.DeleteValue(key, APP_NAME)
                except FileNotFoundError:
                    pass
    except OSError:
        return is_enabled()
    return enabled
