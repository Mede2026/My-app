"""Icone dans la zone de notification de Windows (a cote de l'horloge)."""

from __future__ import annotations

import threading
from typing import Callable

import pystray
from PIL import Image, ImageDraw

from . import APP_NAME, APP_VERSION


def make_icon_image(size: int = 64) -> Image.Image:
    """Dessine un petit cadenas bleu (aucun fichier image a livrer)."""
    image = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    blue, light = (63, 140, 255, 255), (233, 240, 255, 255)
    unit = size / 64

    # Anse du cadenas
    draw.arc(
        [18 * unit, 8 * unit, 46 * unit, 40 * unit],
        start=180,
        end=360,
        fill=blue,
        width=int(6 * unit),
    )
    # Corps du cadenas
    draw.rounded_rectangle(
        [12 * unit, 28 * unit, 52 * unit, 56 * unit], radius=int(8 * unit), fill=blue
    )
    # Trou de serrure
    draw.ellipse([28 * unit, 36 * unit, 36 * unit, 44 * unit], fill=light)
    draw.rectangle([30.5 * unit, 41 * unit, 33.5 * unit, 50 * unit], fill=light)
    return image


class TrayIcon:
    """Enveloppe pystray : l'icone tourne dans son propre fil d'execution."""

    def __init__(
        self,
        on_settings: Callable[[], None],
        on_encrypt: Callable[[], None],
        on_decrypt: Callable[[], None],
        on_quit: Callable[[], None],
    ) -> None:
        menu = pystray.Menu(
            pystray.MenuItem("Reglages...", lambda *_: on_settings(), default=True),
            pystray.Menu.SEPARATOR,
            pystray.MenuItem("Chiffrer la selection", lambda *_: on_encrypt()),
            pystray.MenuItem("Dechiffrer la selection", lambda *_: on_decrypt()),
            pystray.Menu.SEPARATOR,
            pystray.MenuItem("Quitter", lambda *_: on_quit()),
        )
        self.icon = pystray.Icon(
            APP_NAME, make_icon_image(), f"{APP_NAME} {APP_VERSION}", menu
        )
        self._thread: threading.Thread | None = None

    def start(self) -> None:
        self._thread = threading.Thread(target=self.icon.run, daemon=True, name="tray")
        self._thread.start()

    def notify(self, message: str, title: str = APP_NAME) -> None:
        """Petite notification systeme (utilisee pour les cas discrets)."""
        try:
            self.icon.notify(message, title)
        except Exception:
            pass

    def stop(self) -> None:
        try:
            self.icon.stop()
        except Exception:
            pass
