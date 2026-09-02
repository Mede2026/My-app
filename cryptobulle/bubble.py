"""La petite bulle qui apparait a l'ecran.

Une bulle est une fenetre Tkinter sans bordure, toujours au-dessus des autres,
placee pres du curseur de la souris. Elle se ferme toute seule apres quelques
secondes, sauf si la souris est dessus.
"""

from __future__ import annotations

import tkinter as tk
from dataclasses import dataclass

MAX_WIDTH = 520
MIN_WIDTH = 260
MAX_TEXT_LINES = 14


@dataclass(frozen=True)
class Palette:
    background: str
    border: str
    title: str
    text: str
    muted: str
    button: str
    button_text: str


THEMES = {
    "sombre": Palette(
        background="#1c1f26",
        border="#3f8cff",
        title="#ffffff",
        text="#e6e9ef",
        muted="#98a2b3",
        button="#2a2f3a",
        button_text="#e6e9ef",
    ),
    "clair": Palette(
        background="#ffffff",
        border="#2563eb",
        title="#111827",
        text="#1f2937",
        muted="#6b7280",
        button="#eef2f7",
        button_text="#111827",
    ),
}

ACCENTS = {"info": "#3f8cff", "success": "#22c55e", "error": "#ef4444"}
ICONS = {"info": "\U0001F513", "success": "\U0001F512", "error": "⚠"}


class Bubble:
    """Une fenetre-bulle unique."""

    def __init__(
        self,
        master: tk.Tk,
        title: str,
        body: str,
        kind: str = "info",
        seconds: int = 12,
        theme: str = "sombre",
        show_now: bool = True,
    ) -> None:
        self.master = master
        self.body = body
        self.kind = kind
        self.seconds = seconds
        self.palette = THEMES.get(theme, THEMES["sombre"])
        accent = ACCENTS.get(kind, ACCENTS["info"])
        self._timer: str | None = None
        self._drag_origin: tuple[int, int] | None = None

        self.window = tk.Toplevel(master)
        self.window.withdraw()
        self.window.overrideredirect(True)   # pas de barre de titre Windows
        self.window.attributes("-topmost", True)
        self.window.configure(background=accent)
        try:
            self.window.attributes("-alpha", 0.0)  # pour le fondu d'apparition
        except tk.TclError:
            pass

        # Le fond colore du Toplevel sert de bordure de 2 pixels.
        frame = tk.Frame(self.window, background=self.palette.background)
        frame.pack(fill="both", expand=True, padx=2, pady=2)

        header = tk.Frame(frame, background=self.palette.background)
        header.pack(fill="x", padx=12, pady=(10, 4))
        self.title_label = tk.Label(
            header,
            text=f"{ICONS.get(kind, '')}  {title}",
            background=self.palette.background,
            foreground=self.palette.title,
            font=("Segoe UI Semibold", 10),
            anchor="w",
        )
        self.title_label.pack(side="left")
        close = tk.Label(
            header,
            text="✕",
            background=self.palette.background,
            foreground=self.palette.muted,
            font=("Segoe UI", 10),
            cursor="hand2",
        )
        close.pack(side="right")
        close.bind("<Button-1>", lambda _event: self.close())

        lines = body.count("\n") + 1
        longest = max((len(line) for line in body.split("\n")), default=1)
        height = max(1, min(MAX_TEXT_LINES, lines + longest // 60))
        self.text = tk.Text(
            frame,
            height=height,
            width=48,
            wrap="word",
            relief="flat",
            background=self.palette.background,
            foreground=self.palette.text,
            insertbackground=self.palette.text,
            selectbackground=accent,
            font=("Segoe UI", 10),
            borderwidth=0,
            highlightthickness=0,
            padx=12,
            pady=4,
        )
        self.text.insert("1.0", body)
        self.text.configure(state="disabled")  # lecture seule, selection possible
        self.text.pack(fill="both", expand=True)

        footer = tk.Frame(frame, background=self.palette.background)
        footer.pack(fill="x", padx=12, pady=(6, 10))
        self.hint = tk.Label(
            footer,
            text="Echap pour fermer",
            background=self.palette.background,
            foreground=self.palette.muted,
            font=("Segoe UI", 8),
        )
        self.hint.pack(side="left")
        self._button(footer, "Fermer", self.close).pack(side="right")
        self._button(footer, "Copier", self._copy).pack(side="right", padx=(0, 6))

        for widget in (self.window, frame, header, footer, self.text):
            widget.bind("<Enter>", self._pause_timer)
            widget.bind("<Leave>", self._resume_timer)
        header.bind("<Button-1>", self._drag_start)
        header.bind("<B1-Motion>", self._drag_move)
        self.window.bind("<Escape>", lambda _event: self.close())
        self.window.bind("<Control-c>", lambda _event: self._copy())

        if show_now:
            self._place_near_pointer()
            self._fade_in()
            self._start_timer()

    # --- apparence ------------------------------------------------------
    def _button(self, parent: tk.Widget, label: str, command) -> tk.Button:
        return tk.Button(
            parent,
            text=label,
            command=command,
            background=self.palette.button,
            foreground=self.palette.button_text,
            activebackground=self.palette.button,
            activeforeground=self.palette.button_text,
            relief="flat",
            borderwidth=0,
            padx=10,
            pady=3,
            cursor="hand2",
            font=("Segoe UI", 9),
        )

    def _place_near_pointer(self) -> None:
        self.window.update_idletasks()
        width = max(MIN_WIDTH, min(MAX_WIDTH, self.window.winfo_reqwidth()))
        height = self.window.winfo_reqheight()
        pointer_x, pointer_y = self.master.winfo_pointerxy()
        screen_w = self.master.winfo_screenwidth()
        screen_h = self.master.winfo_screenheight()
        x = min(max(8, pointer_x + 16), screen_w - width - 8)
        y = pointer_y + 20
        if y + height > screen_h - 8:  # pas de place en bas : on passe au-dessus
            y = max(8, pointer_y - height - 12)
        self.window.geometry(f"{width}x{height}+{int(x)}+{int(y)}")

    def _fade_in(self, alpha: float = 0.0) -> None:
        self.window.deiconify()
        try:
            self.window.attributes("-alpha", alpha)
        except tk.TclError:
            return
        if alpha < 1.0:
            self.window.after(12, self._fade_in, min(1.0, alpha + 0.12))
        else:
            self.window.focus_force()

    def update_content(self, title: str, body: str, kind: str, seconds: int) -> None:
        """Reutilise la fenetre existante : affichage instantane, sans clignotement."""
        self._pause_timer()
        self.body, self.kind, self.seconds = body, kind, seconds
        accent = ACCENTS.get(kind, ACCENTS["info"])
        self.window.configure(background=accent)
        self.text.configure(selectbackground=accent)
        self.title_label.configure(text=f"{ICONS.get(kind, '')}  {title}")

        lines = body.count("\n") + 1
        longest = max((len(line) for line in body.split("\n")), default=1)
        self.text.configure(state="normal", height=max(1, min(MAX_TEXT_LINES, lines + longest // 60)))
        self.text.delete("1.0", "end")
        self.text.insert("1.0", body)
        self.text.configure(state="disabled")
        self.hint.configure(text="Echap pour fermer")

        self._place_near_pointer()
        self.window.deiconify()
        try:
            self.window.attributes("-alpha", 1.0)
        except tk.TclError:
            pass
        self.window.lift()
        self.window.focus_force()
        self._start_timer()

    def alive(self) -> bool:
        try:
            return bool(self.window.winfo_exists())
        except tk.TclError:
            return False

    # --- comportement ---------------------------------------------------
    def _copy(self) -> None:
        self.master.clipboard_clear()
        self.master.clipboard_append(self.body)
        self.hint.configure(text="Copie dans le presse-papiers")

    def _drag_start(self, event: tk.Event) -> None:
        self._drag_origin = (event.x_root, event.y_root)

    def _drag_move(self, event: tk.Event) -> None:
        if not self._drag_origin:
            return
        dx = event.x_root - self._drag_origin[0]
        dy = event.y_root - self._drag_origin[1]
        self._drag_origin = (event.x_root, event.y_root)
        self.window.geometry(f"+{self.window.winfo_x() + dx}+{self.window.winfo_y() + dy}")

    def _start_timer(self) -> None:
        if self.seconds > 0:
            self._timer = self.window.after(self.seconds * 1000, self.close)

    def _pause_timer(self, _event: tk.Event | None = None) -> None:
        if self._timer is not None:
            self.window.after_cancel(self._timer)
            self._timer = None

    def _resume_timer(self, _event: tk.Event | None = None) -> None:
        if self._timer is None:
            self._start_timer()

    def close(self) -> None:
        """Masque la bulle. La fenetre reste prete pour le prochain affichage."""
        self._pause_timer()
        try:
            self.window.withdraw()
        except tk.TclError:
            pass

    def destroy(self) -> None:
        self._pause_timer()
        try:
            self.window.destroy()
        except tk.TclError:
            pass


class BubbleManager:
    """Garde une seule bulle, construite une fois puis reutilisee.

    Creer une fenetre Tkinter coute quelques dizaines de millisecondes ; la
    reutiliser en coute zero. La bulle apparait donc instantanement des le
    deuxieme raccourci.
    """

    def __init__(self, root: tk.Tk, theme: str = "sombre") -> None:
        self.root = root
        self.theme = theme
        self._current: Bubble | None = None
        self._built_theme: str | None = None

    def show(self, title: str, body: str, kind: str = "info", seconds: int = 12) -> Bubble:
        reusable = (
            self._current is not None
            and self._current.alive()
            and self._built_theme == self.theme
        )
        if reusable:
            self._current.update_content(title, body, kind, seconds)
            return self._current
        if self._current is not None:
            self._current.destroy()
        self._current = Bubble(self.root, title, body, kind, seconds, self.theme)
        self._built_theme = self.theme
        return self._current

    def prepare(self) -> None:
        """Construit la fenetre a l'avance, hors du chemin critique."""
        if self._current is None or not self._current.alive():
            self._current = Bubble(self.root, "", "", "info", 0, self.theme, show_now=False)
            self._built_theme = self.theme

    def close(self) -> None:
        if self._current is not None:
            self._current.close()
