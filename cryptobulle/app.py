"""Chef d'orchestre : raccourcis, presse-papiers, bulles et icone de notification.

Trois fils d'execution seulement :

- le fil principal, qui appartient a Tkinter (toute l'interface) ;
- le fil « winui », qui heberge la boucle de messages Windows (raccourcis et
  icone), endormi tant qu'il ne se passe rien ;
- un fil temporaire par action, pour ne jamais bloquer les deux autres.

Les echanges entre fils passent par :meth:`CryptoBulleApp.post`, qui depose la
tache dans une file lue par le fil principal.
"""

from __future__ import annotations

import ctypes
import queue
import sys
import threading
import tkinter as tk
from tkinter import messagebox

from . import APP_NAME, APP_VERSION
from .bubble import BubbleManager
from .clipboard import ClipboardError, get_clipboard, paste_text, read_selection, set_clipboard
from .config import Config
from .crypto import CryptoError, decrypt, derive_key, encrypt, find_token, looks_encrypted
from .hotkeys import HotkeyError, pretty

MUTEX_NAME = "Global\\CryptoBulle-single-instance"
ERROR_ALREADY_EXISTS = 183


def _claim_single_instance() -> bool:
    """Vrai si aucune autre copie de CryptoBulle ne tourne deja."""
    if sys.platform != "win32":
        return True
    kernel32 = ctypes.windll.kernel32
    kernel32.CreateMutexW(None, False, MUTEX_NAME)
    return kernel32.GetLastError() != ERROR_ALREADY_EXISTS


class CryptoBulleApp:
    def __init__(self) -> None:
        self.config = Config.load()
        self.root = tk.Tk()
        self.root.withdraw()
        self.root.title(APP_NAME)
        self.bubbles = BubbleManager(self.root, self.config.theme)
        self.windows = None  # WindowsUI, cree au demarrage
        self.settings = None  # SettingsWindow, creee a la demande
        self._tasks: queue.Queue = queue.Queue()
        self._busy = threading.Lock()
        self._stopping = False

    # --- passerelle entre fils d'execution -------------------------------
    def post(self, function, *args) -> None:
        """Fait executer `function` par le fil principal (celui de Tkinter)."""
        self._tasks.put((function, args))

    def _pump(self) -> None:
        while True:
            try:
                function, args = self._tasks.get_nowait()
            except queue.Empty:
                break
            try:
                function(*args)
            except Exception as exc:  # une action ratee ne doit pas tuer l'appli
                self._show("Erreur", str(exc), "error")
        if not self._stopping:
            # 50 ms : assez rapide pour que la bulle paraisse immediate, assez
            # lent pour que le processeur reste pratiquement inactif au repos.
            self.root.after(50, self._pump)

    def _show(self, title: str, body: str, kind: str = "info", seconds: int | None = None) -> None:
        self.bubbles.theme = self.config.theme
        duration = self.config.bubble_seconds if seconds is None else seconds
        self.bubbles.show(title, body, kind, duration)

    # --- actions ---------------------------------------------------------
    def trigger(self, action) -> None:
        """Lance une action dans un fil de fond, une seule a la fois."""
        if not self._busy.acquire(blocking=False):
            return  # une action est deja en cours : on ignore le second appui

        def worker() -> None:
            try:
                action()
            except Exception as exc:  # filet de securite
                self.post(self._show, "Erreur", str(exc), "error")
            finally:
                self._busy.release()

        threading.Thread(target=worker, daemon=True, name="action").start()

    def _selected_text(self) -> tuple[str, str]:
        """Texte selectionne (ou, a defaut, presse-papiers) et ancien contenu."""
        selection, previous = read_selection()
        return (selection or previous or get_clipboard()), previous

    def action_decrypt(self) -> None:
        if not self._require_passphrase():
            return
        try:
            text, previous = self._selected_text()
        except ClipboardError as exc:
            self.post(self._show, "Erreur", str(exc), "error")
            return

        if self.config.restore_clipboard and previous:
            try:
                set_clipboard(previous)
            except ClipboardError:
                pass

        token = find_token(text)
        if token is None:
            self.post(
                self._show,
                "Rien a dechiffrer",
                "Aucun message CryptoBulle dans la selection.\n"
                "Selectionnez un texte qui commence par MC1~.",
                "error",
            )
            return
        try:
            plaintext = decrypt(token, self.config.passphrase)
        except CryptoError as exc:
            self.post(self._show, "Dechiffrement impossible", str(exc), "error")
            return
        self.post(self._show, "Texte dechiffre", plaintext, "success")

    def action_encrypt(self) -> None:
        if not self._require_passphrase():
            return
        try:
            text, previous = self._selected_text()
        except ClipboardError as exc:
            self.post(self._show, "Erreur", str(exc), "error")
            return

        text = text.strip()
        if not text:
            self.post(
                self._show,
                "Rien a chiffrer",
                "Selectionnez d'abord du texte, puis refaites le raccourci.",
                "error",
            )
            return
        if looks_encrypted(text):
            self.post(
                self._show, "Deja chiffre", "Ce texte est deja un message CryptoBulle.", "error"
            )
            return

        try:
            token = encrypt(text, self.config.passphrase, self.config.salt())
        except CryptoError as exc:
            self.post(self._show, "Chiffrement impossible", str(exc), "error")
            return

        restore = previous if self.config.restore_clipboard else None
        try:
            if self.config.auto_paste:
                paste_text(token, restore=restore)
                message = "Le texte chiffre a remplace votre selection."
            else:
                set_clipboard(token)
                message = "Le texte chiffre est dans le presse-papiers (Ctrl+V pour le coller)."
        except ClipboardError as exc:
            self.post(self._show, "Erreur", str(exc), "error")
            return
        self.post(self._show, "Texte chiffre", message, "success", 4)

    def _require_passphrase(self) -> bool:
        if self.config.has_passphrase():
            return True
        self.post(
            self._show,
            "Phrase secrete manquante",
            "Ouvrez les reglages pour choisir votre phrase secrete.",
            "error",
        )
        self.post(self.open_settings)
        return False

    def warm_up(self) -> None:
        """Calcule la cle a l'avance pour que le premier raccourci soit immediat.

        scrypt prend une cinquantaine de millisecondes ; autant les depenser au
        demarrage plutot qu'au moment ou l'utilisateur attend son texte.
        """
        if not self.config.has_passphrase():
            return
        try:
            derive_key(self.config.passphrase, self.config.salt())
        except Exception:
            pass

    # --- reglages --------------------------------------------------------
    def open_settings(self) -> None:
        from .ui_settings import SettingsWindow  # charge Tkinter/ttk a la demande

        if self.settings is not None and self.settings.window.winfo_exists():
            self.settings.window.deiconify()
            self.settings.window.lift()
            self.settings.window.focus_force()
            return
        self.settings = SettingsWindow(self.root, self.config, self.apply_config)

    def apply_config(self) -> str | None:
        """Reactive les raccourcis apres un changement. Renvoie l'erreur eventuelle."""
        self.bubbles.theme = self.config.theme
        threading.Thread(target=self.warm_up, daemon=True).start()
        try:
            self.windows.set_hotkey(
                "decrypt", self.config.hotkey_decrypt, lambda: self.trigger(self.action_decrypt)
            )
            self.windows.set_hotkey(
                "encrypt", self.config.hotkey_encrypt, lambda: self.trigger(self.action_encrypt)
            )
        except HotkeyError as exc:
            return str(exc)
        return None

    def quit(self) -> None:
        self._stopping = True
        if self.windows is not None:
            self.windows.stop()
        try:
            self.root.destroy()
        except tk.TclError:
            pass

    # --- demarrage -------------------------------------------------------
    def run(self) -> int:
        from .winui import WindowsUI

        self.windows = WindowsUI(
            app_name=APP_NAME,
            tooltip=f"{APP_NAME} {APP_VERSION}",
            menu=[
                ("Reglages...", lambda: self.post(self.open_settings)),
                None,
                ("Chiffrer la selection", lambda: self.trigger(self.action_encrypt)),
                ("Dechiffrer la selection", lambda: self.trigger(self.action_decrypt)),
                None,
                ("Quitter", lambda: self.post(self.quit)),
            ],
            on_default=lambda: self.post(self.open_settings),
        )
        self.windows.start()

        error = self.apply_config()
        if error:
            messagebox.showerror(
                APP_NAME, f"{error}\n\nChoisissez d'autres raccourcis dans les reglages."
            )

        self.root.after(50, self._pump)
        self.root.after(60, self.bubbles.prepare)  # fenetre prete avant le besoin
        if not self.config.has_passphrase():
            self.root.after(150, self.open_settings)
        else:
            self.root.after(
                250,
                lambda: self._show(
                    f"{APP_NAME} est actif",
                    f"{pretty(self.config.hotkey_encrypt)} : chiffrer la selection\n"
                    f"{pretty(self.config.hotkey_decrypt)} : dechiffrer la selection",
                    "info",
                    6,
                ),
            )
        self.root.mainloop()
        return 0


def main() -> int:
    if sys.platform != "win32":
        print("CryptoBulle fonctionne uniquement sous Windows.", file=sys.stderr)
        return 1
    if not _claim_single_instance():
        messagebox.showinfo(APP_NAME, f"{APP_NAME} est deja lance (icone pres de l'horloge).")
        return 0

    from .winapi import enable_dpi_awareness

    enable_dpi_awareness()  # a faire avant la creation de la premiere fenetre
    return CryptoBulleApp().run()


if __name__ == "__main__":
    sys.exit(main())
