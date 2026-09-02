"""Fenetre de reglages et petit atelier de chiffrement manuel."""

from __future__ import annotations

import tkinter as tk
from tkinter import messagebox, ttk

from . import APP_NAME, APP_VERSION, startup
from .config import Config
from .crypto import CryptoError, decrypt, encrypt, find_token
from .hotkeys import HotkeyError, normalize, pretty
from .secretstore import is_secure

# Touches Tkinter -> noms compris par cryptobulle.hotkeys
_KEYSYM_ALIASES = {
    "Return": "enter", "BackSpace": "backspace", "Escape": "escape", "space": "space",
    "Prior": "pageup", "Next": "pagedown", "Delete": "delete", "Insert": "insert",
    "Home": "home", "End": "end", "Left": "left", "Right": "right", "Up": "up",
    "Down": "down", "Tab": "tab", "plus": "plus", "minus": "minus",
    "comma": "comma", "period": "period",
}
_MODIFIER_KEYSYMS = ("Control", "Alt", "Shift", "Win", "Super", "Meta")


class SettingsWindow:
    """Fenetre unique de reglages (une seule instance a la fois)."""

    def __init__(self, root: tk.Tk, config: Config, on_apply) -> None:
        self.root = root
        self.config = config
        self.on_apply = on_apply

        self.window = tk.Toplevel(root)
        self.window.title(f"{APP_NAME} - reglages")
        self.window.resizable(False, False)
        self.window.protocol("WM_DELETE_WINDOW", self.close)

        self.var_passphrase = tk.StringVar(value=config.passphrase)
        self.var_decrypt = tk.StringVar(value=config.hotkey_decrypt)
        self.var_encrypt = tk.StringVar(value=config.hotkey_encrypt)
        self.var_seconds = tk.IntVar(value=config.bubble_seconds)
        self.var_auto_paste = tk.BooleanVar(value=config.auto_paste)
        self.var_restore = tk.BooleanVar(value=config.restore_clipboard)
        self.var_startup = tk.BooleanVar(value=startup.is_enabled())
        self.var_theme = tk.StringVar(value=config.theme)
        self.var_show_pass = tk.BooleanVar(value=False)

        notebook = ttk.Notebook(self.window)
        notebook.pack(fill="both", expand=True, padx=10, pady=10)
        notebook.add(self._tab_general(notebook), text="Reglages")
        notebook.add(self._tab_workshop(notebook), text="Atelier")
        notebook.add(self._tab_about(notebook), text="Aide")

        bar = ttk.Frame(self.window)
        bar.pack(fill="x", padx=10, pady=(0, 10))
        ttk.Button(bar, text="Fermer", command=self.close).pack(side="right")
        ttk.Button(bar, text="Enregistrer", command=self.save).pack(side="right", padx=(0, 6))

        self.window.update_idletasks()
        self._center()
        self.window.lift()
        self.window.focus_force()

    # --- onglets --------------------------------------------------------
    def _tab_general(self, parent: ttk.Notebook) -> ttk.Frame:
        tab = ttk.Frame(parent, padding=12)

        secret = ttk.LabelFrame(tab, text="Phrase secrete", padding=10)
        secret.pack(fill="x")
        self.entry_pass = ttk.Entry(secret, textvariable=self.var_passphrase, show="•", width=42)
        self.entry_pass.grid(row=0, column=0, sticky="we")
        ttk.Checkbutton(
            secret, text="Afficher", variable=self.var_show_pass, command=self._toggle_show
        ).grid(row=0, column=1, padx=(8, 0))
        note = (
            "Protegee par Windows (DPAPI)."
            if is_secure()
            else "Attention : stockage simple, sans protection systeme."
        )
        ttk.Label(secret, text=note, foreground="#6b7280").grid(
            row=1, column=0, columnspan=2, sticky="w", pady=(6, 0)
        )
        ttk.Label(
            secret,
            text="Les personnes avec qui vous echangez doivent avoir la meme phrase.",
            foreground="#6b7280",
        ).grid(row=2, column=0, columnspan=2, sticky="w")

        keys = ttk.LabelFrame(tab, text="Raccourcis clavier", padding=10)
        keys.pack(fill="x", pady=(12, 0))
        self._hotkey_row(keys, 0, "Dechiffrer la selection", self.var_decrypt)
        self._hotkey_row(keys, 1, "Chiffrer la selection", self.var_encrypt)

        options = ttk.LabelFrame(tab, text="Comportement", padding=10)
        options.pack(fill="x", pady=(12, 0))
        ttk.Label(options, text="Duree de la bulle (secondes, 0 = manuel)").grid(
            row=0, column=0, sticky="w"
        )
        ttk.Spinbox(options, from_=0, to=120, width=5, textvariable=self.var_seconds).grid(
            row=0, column=1, sticky="e", padx=(10, 0)
        )
        ttk.Checkbutton(
            options, text="Coller automatiquement le texte chiffre", variable=self.var_auto_paste
        ).grid(row=1, column=0, columnspan=2, sticky="w", pady=(6, 0))
        ttk.Checkbutton(
            options, text="Remettre l'ancien presse-papiers ensuite", variable=self.var_restore
        ).grid(row=2, column=0, columnspan=2, sticky="w")
        ttk.Checkbutton(
            options, text="Lancer au demarrage de Windows", variable=self.var_startup
        ).grid(row=3, column=0, columnspan=2, sticky="w")
        ttk.Label(options, text="Theme de la bulle").grid(row=4, column=0, sticky="w", pady=(6, 0))
        theme_box = ttk.Frame(options)
        theme_box.grid(row=4, column=1, sticky="e", pady=(6, 0))
        for value, label in (("sombre", "Sombre"), ("clair", "Clair")):
            ttk.Radiobutton(theme_box, text=label, value=value, variable=self.var_theme).pack(
                side="left"
            )
        options.columnconfigure(0, weight=1)
        return tab

    def _hotkey_row(self, parent: ttk.LabelFrame, row: int, label: str, var: tk.StringVar) -> None:
        ttk.Label(parent, text=label).grid(row=row, column=0, sticky="w", pady=3)
        ttk.Entry(parent, textvariable=var, width=18).grid(
            row=row, column=1, padx=(10, 6), pady=3
        )
        ttk.Button(parent, text="Enregistrer...", command=lambda: self._capture(var)).grid(
            row=row, column=2, pady=3
        )
        parent.columnconfigure(0, weight=1)

    def _tab_workshop(self, parent: ttk.Notebook) -> ttk.Frame:
        tab = ttk.Frame(parent, padding=12)
        ttk.Label(tab, text="Texte a chiffrer ou message MC1~ a dechiffrer :").pack(anchor="w")
        self.workshop_in = tk.Text(tab, height=5, width=64, wrap="word")
        self.workshop_in.pack(fill="both", pady=(4, 8))
        row = ttk.Frame(tab)
        row.pack(fill="x")
        ttk.Button(row, text="Chiffrer", command=lambda: self._workshop(True)).pack(side="left")
        ttk.Button(row, text="Dechiffrer", command=lambda: self._workshop(False)).pack(
            side="left", padx=6
        )
        ttk.Button(row, text="Copier le resultat", command=self._workshop_copy).pack(side="left")
        ttk.Label(tab, text="Resultat :").pack(anchor="w", pady=(10, 0))
        self.workshop_out = tk.Text(tab, height=5, width=64, wrap="char")
        self.workshop_out.pack(fill="both", pady=(4, 0))
        return tab

    def _tab_about(self, parent: ttk.Notebook) -> ttk.Frame:
        tab = ttk.Frame(parent, padding=12)
        try:
            from .aesgcm import backend_name

            engine = backend_name()
        except Exception as exc:  # pragma: no cover - depend de la machine
            engine = f"indisponible ({exc})"
        text = (
            f"{APP_NAME} {APP_VERSION}\n\n"
            "1. Selectionnez du texte n'importe ou dans Windows.\n"
            f"2. {pretty(self.config.hotkey_encrypt)} : le texte est chiffre puis colle a la place.\n"
            f"3. {pretty(self.config.hotkey_decrypt)} : la bulle affiche le texte d'origine.\n\n"
            "Chiffrement : AES-256-GCM, cle derivee par scrypt a partir de votre\n"
            "phrase secrete et d'une constante propre a l'application.\n"
            "Le resultat est encode dans un alphabet maison : sans CryptoBulle et\n"
            "sans la bonne phrase secrete, le message reste illisible.\n\n"
            f"Moteur cryptographique : {engine}\n"
            "Aucune bibliotheque externe : tout passe par Windows et la\n"
            "bibliotheque standard de Python."
        )
        ttk.Label(tab, text=text, justify="left").pack(anchor="w")
        return tab

    # --- actions --------------------------------------------------------
    def _toggle_show(self) -> None:
        self.entry_pass.configure(show="" if self.var_show_pass.get() else "•")

    def _capture(self, var: tk.StringVar) -> None:
        """Ouvre une petite fenetre et attend une combinaison de touches."""
        dialog = tk.Toplevel(self.window)
        dialog.title("Nouveau raccourci")
        dialog.resizable(False, False)
        dialog.transient(self.window)
        ttk.Label(
            dialog,
            text="Appuyez sur la combinaison souhaitee\n(au moins Ctrl, Alt, Maj ou Windows).",
            justify="center",
            padding=16,
        ).pack()
        ttk.Button(dialog, text="Annuler", command=dialog.destroy).pack(pady=(0, 12))

        def on_key(event: tk.Event) -> str:
            keysym = event.keysym
            if keysym.startswith(_MODIFIER_KEYSYMS):
                return "break"
            token = _KEYSYM_ALIASES.get(keysym, keysym.lower())
            combo = "+".join(self._pressed_modifiers() + [token])
            try:
                var.set(normalize(combo))
            except HotkeyError as exc:
                messagebox.showerror(APP_NAME, str(exc), parent=dialog)
                return "break"
            dialog.destroy()
            return "break"

        dialog.bind("<KeyPress>", on_key)
        dialog.grab_set()      # la fenetre capte tout le clavier
        dialog.focus_force()

    @staticmethod
    def _pressed_modifiers() -> list[str]:
        """Modificateurs physiquement enfonces, demandes a Windows."""
        from .winapi import VK_CONTROL, VK_LWIN, VK_MENU, VK_RWIN, VK_SHIFT, user32

        pressed = []
        for name, key in (
            ("ctrl", VK_CONTROL), ("alt", VK_MENU), ("shift", VK_SHIFT),
        ):
            if user32.GetAsyncKeyState(key) & 0x8000:
                pressed.append(name)
        if (user32.GetAsyncKeyState(VK_LWIN) | user32.GetAsyncKeyState(VK_RWIN)) & 0x8000:
            pressed.append("win")
        return pressed

    def _workshop(self, do_encrypt: bool) -> None:
        source = self.workshop_in.get("1.0", "end").strip()
        passphrase = self.var_passphrase.get()
        try:
            if do_encrypt:
                result = encrypt(source, passphrase, self.config.salt())
            else:
                token = find_token(source)
                if token is None:
                    raise CryptoError("Aucun message CryptoBulle dans ce texte.")
                result = decrypt(token, passphrase)
        except CryptoError as exc:
            result = f"[erreur] {exc}"
        self.workshop_out.delete("1.0", "end")
        self.workshop_out.insert("1.0", result)

    def _workshop_copy(self) -> None:
        self.root.clipboard_clear()
        self.root.clipboard_append(self.workshop_out.get("1.0", "end").strip())

    def save(self) -> None:
        try:
            decrypt_combo = normalize(self.var_decrypt.get())
            encrypt_combo = normalize(self.var_encrypt.get())
        except HotkeyError as exc:
            messagebox.showerror(APP_NAME, str(exc), parent=self.window)
            return
        if decrypt_combo == encrypt_combo:
            messagebox.showerror(
                APP_NAME, "Les deux raccourcis doivent etre differents.", parent=self.window
            )
            return
        passphrase = self.var_passphrase.get()
        if not passphrase:
            messagebox.showerror(APP_NAME, "La phrase secrete est obligatoire.", parent=self.window)
            return
        if passphrase != self.config.passphrase and self.config.has_passphrase():
            if not messagebox.askyesno(
                APP_NAME,
                "Changer la phrase secrete rendra illisibles les messages deja chiffres "
                "avec l'ancienne. Continuer ?",
                parent=self.window,
            ):
                return

        self.config.hotkey_decrypt = decrypt_combo
        self.config.hotkey_encrypt = encrypt_combo
        self.config.passphrase = passphrase
        self.config.bubble_seconds = max(0, int(self.var_seconds.get()))
        self.config.auto_paste = bool(self.var_auto_paste.get())
        self.config.restore_clipboard = bool(self.var_restore.get())
        self.config.theme = self.var_theme.get()
        self.config.launch_at_startup = startup.set_enabled(bool(self.var_startup.get()))
        self.var_startup.set(self.config.launch_at_startup)
        self.config.save()

        error = self.on_apply()
        if error:
            messagebox.showerror(APP_NAME, error, parent=self.window)
        else:
            messagebox.showinfo(APP_NAME, "Reglages enregistres.", parent=self.window)

    def _center(self) -> None:
        width = self.window.winfo_width()
        height = self.window.winfo_height()
        x = (self.window.winfo_screenwidth() - width) // 2
        y = (self.window.winfo_screenheight() - height) // 3
        self.window.geometry(f"+{x}+{y}")

    def close(self) -> None:
        try:
            self.window.destroy()
        except tk.TclError:
            pass
