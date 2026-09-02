"""Boucle de messages Windows : raccourcis globaux + icone de notification.

Un seul fil d'execution, une seule fenetre cachee, aucune bibliotheque externe.
Windows reveille ce fil uniquement quand un raccourci est presse ou quand on
clique sur l'icone : au repos, CryptoBulle ne consomme pas de processeur.
"""

from __future__ import annotations

import ctypes
import queue
import sys
import threading
from ctypes import wintypes
from pathlib import Path
from typing import Callable

from . import hotkeys as hk
from .winapi import (
    IDI_APPLICATION,
    IMAGE_ICON,
    LR_DEFAULTSIZE,
    LR_LOADFROMFILE,
    MF_SEPARATOR,
    MF_STRING,
    NIF_ICON,
    NIF_MESSAGE,
    NIF_TIP,
    NIM_ADD,
    NIM_DELETE,
    NOTIFYICONDATA,
    TPM_RETURNCMD,
    TPM_RIGHTBUTTON,
    WM_CLOSE,
    WM_DESTROY,
    WM_HOTKEY,
    WM_LBUTTONDBLCLK,
    WM_LBUTTONUP,
    WM_RBUTTONUP,
    WM_RUN_TASK,
    WM_TRAY,
    WNDCLASS,
    WNDPROC,
    kernel32,
    shell32,
    user32,
)

MenuItem = tuple[str, Callable[[], None]] | None


def load_icon() -> int:
    """Icone de l'application : celle de l'executable, sinon assets/icon.ico."""
    if getattr(sys, "frozen", False):
        handle = shell32.ExtractIconW(None, sys.executable, 0)
        if handle and handle > 1:
            return handle
    icon_file = Path(__file__).resolve().parent.parent / "assets" / "icon.ico"
    if icon_file.exists():
        handle = user32.LoadImageW(
            None, str(icon_file), IMAGE_ICON, 0, 0, LR_LOADFROMFILE | LR_DEFAULTSIZE
        )
        if handle:
            return handle
    return user32.LoadIconW(None, ctypes.c_wchar_p(IDI_APPLICATION))


class WindowsUI:
    """Fenetre cachee qui porte les raccourcis et l'icone pres de l'horloge."""

    def __init__(self, app_name: str, tooltip: str, menu: list[MenuItem], on_default) -> None:
        self.app_name = app_name
        self.tooltip = tooltip[:127]
        self.menu = menu
        self.on_default = on_default

        self.hwnd = None
        self._thread: threading.Thread | None = None
        self._ready = threading.Event()
        self._tasks: queue.Queue = queue.Queue()
        self._hotkeys: dict[str, tuple[int, Callable[[], None]]] = {}
        self._by_id: dict[int, Callable[[], None]] = {}
        self._next_id = 1
        self._wndproc = WNDPROC(self._on_message)  # garde la reference vivante
        self._icon_added = False

    # --- cycle de vie ---------------------------------------------------
    def start(self, timeout: float = 5.0) -> None:
        self._thread = threading.Thread(target=self._run, name="winui", daemon=True)
        self._thread.start()
        if not self._ready.wait(timeout):
            raise RuntimeError("La fenetre de service Windows n'a pas demarre.")

    def stop(self) -> None:
        if self.hwnd:
            user32.PostMessageW(self.hwnd, WM_CLOSE, 0, 0)

    def _run(self) -> None:
        instance = kernel32.GetModuleHandleW(None)
        class_name = f"{self.app_name}Window"

        window_class = WNDCLASS()
        window_class.lpfnWndProc = ctypes.cast(self._wndproc, ctypes.c_void_p)
        window_class.hInstance = instance
        window_class.lpszClassName = class_name
        user32.RegisterClassW(ctypes.byref(window_class))

        # Fenetre jamais affichee : elle sert uniquement de boite aux lettres.
        self.hwnd = user32.CreateWindowExW(
            0, class_name, self.app_name, 0, 0, 0, 0, 0, None, None, instance, None
        )
        if not self.hwnd:
            self._ready.set()
            raise ctypes.WinError(ctypes.get_last_error())

        self._add_tray_icon()
        self._ready.set()

        message = wintypes.MSG()
        while user32.GetMessageW(ctypes.byref(message), None, 0, 0) > 0:
            user32.TranslateMessage(ctypes.byref(message))
            user32.DispatchMessageW(ctypes.byref(message))

    # --- appels depuis les autres fils -----------------------------------
    def call(self, function: Callable[[], object]):
        """Fait executer `function` par le fil de la boucle et rend son resultat."""
        if threading.current_thread() is self._thread:
            return function()
        answer: queue.Queue = queue.Queue(maxsize=1)

        def wrapper() -> None:
            try:
                answer.put((True, function()))
            except Exception as exc:
                answer.put((False, exc))

        self._tasks.put(wrapper)
        user32.PostMessageW(self.hwnd, WM_RUN_TASK, 0, 0)
        succeeded, value = answer.get(timeout=5)
        if not succeeded:
            raise value
        return value

    # --- raccourcis -------------------------------------------------------
    def set_hotkey(self, name: str, combo: str, callback: Callable[[], None]) -> None:
        """(Re)lie une action a une combinaison. Leve HotkeyError si refusee."""
        modifiers, key = hk.parse(combo)
        self.remove_hotkey(name)

        def register() -> None:
            identifier = self._next_id
            self._next_id += 1
            if not user32.RegisterHotKey(self.hwnd, identifier, modifiers | hk.MOD_NOREPEAT, key):
                raise hk.HotkeyError(
                    f"Windows refuse « {hk.pretty(combo)} » : "
                    "ce raccourci est deja pris par un autre logiciel."
                )
            self._hotkeys[name] = (identifier, callback)
            self._by_id[identifier] = callback

        self.call(register)

    def remove_hotkey(self, name: str) -> None:
        entry = self._hotkeys.pop(name, None)
        if entry is None:
            return
        identifier, _ = entry
        self._by_id.pop(identifier, None)
        self.call(lambda: user32.UnregisterHotKey(self.hwnd, identifier))

    # --- icone de notification --------------------------------------------
    def _icon_data(self) -> NOTIFYICONDATA:
        data = NOTIFYICONDATA()
        data.cbSize = ctypes.sizeof(NOTIFYICONDATA)
        data.hWnd = self.hwnd
        data.uID = 1
        return data

    def _add_tray_icon(self) -> None:
        data = self._icon_data()
        data.uFlags = NIF_ICON | NIF_MESSAGE | NIF_TIP
        data.uCallbackMessage = WM_TRAY
        data.hIcon = load_icon()
        data.szTip = self.tooltip
        self._icon_added = bool(shell32.Shell_NotifyIconW(NIM_ADD, ctypes.byref(data)))

    def _remove_tray_icon(self) -> None:
        if self._icon_added:
            shell32.Shell_NotifyIconW(NIM_DELETE, ctypes.byref(self._icon_data()))
            self._icon_added = False

    def _show_menu(self) -> None:
        menu = user32.CreatePopupMenu()
        actions: dict[int, Callable[[], None]] = {}
        for index, item in enumerate(self.menu, start=1):
            if item is None:
                user32.AppendMenuW(menu, MF_SEPARATOR, 0, None)
            else:
                label, callback = item
                user32.AppendMenuW(menu, MF_STRING, index, label)
                actions[index] = callback

        point = wintypes.POINT()
        user32.GetCursorPos(ctypes.byref(point))
        # Sans cet avant-plan, le menu resterait affiche apres un clic ailleurs.
        user32.SetForegroundWindow(self.hwnd)
        chosen = user32.TrackPopupMenu(
            menu, TPM_RIGHTBUTTON | TPM_RETURNCMD, point.x, point.y, 0, self.hwnd, None
        )
        user32.DestroyMenu(menu)
        user32.PostMessageW(self.hwnd, 0, 0, 0)  # WM_NULL : referme proprement
        callback = actions.get(chosen)
        if callback is not None:
            callback()

    # --- traitement des messages -------------------------------------------
    def _on_message(self, hwnd, message, wparam, lparam):
        if message == WM_RUN_TASK:
            while True:
                try:
                    task = self._tasks.get_nowait()
                except queue.Empty:
                    break
                task()
            return 0
        if message == WM_HOTKEY:
            callback = self._by_id.get(int(wparam))
            if callback is not None:
                callback()
            return 0
        if message == WM_TRAY:
            event = lparam & 0xFFFF
            if event in (WM_RBUTTONUP, WM_LBUTTONUP):
                self._show_menu()
            elif event == WM_LBUTTONDBLCLK:
                self.on_default()
            return 0
        if message == WM_DESTROY:
            self._remove_tray_icon()
            user32.PostQuitMessage(0)
            return 0
        return user32.DefWindowProcW(hwnd, message, wparam, lparam)
