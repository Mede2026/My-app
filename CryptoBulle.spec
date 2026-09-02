# -*- mode: python ; coding: utf-8 -*-
"""Recette PyInstaller : produit dist/CryptoBulle.exe (un seul fichier)."""

from pathlib import Path

block_cipher = None
icon = Path("assets/icon.ico")

a = Analysis(
    ["cryptobulle.pyw"],
    pathex=["."],
    binaries=[],
    datas=[],
    hiddenimports=["pystray._win32", "PIL._tkinter_finder"],
    hookspath=[],
    runtime_hooks=[],
    excludes=["pytest", "numpy"],
    cipher=block_cipher,
)
pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name="CryptoBulle",
    debug=False,
    strip=False,
    upx=False,
    console=False,          # pas de fenetre noire au lancement
    disable_windowed_traceback=False,
    icon=str(icon) if icon.exists() else None,
)
