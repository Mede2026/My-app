# -*- mode: python ; coding: utf-8 -*-
"""Recette PyInstaller.

Par defaut : un dossier `dist\\CryptoBulle\\` qui demarre en un instant.
Avec la variable d'environnement CRYPTOBULLE_ONEFILE=1 : un unique .exe, plus
pratique a transporter mais plus lent au lancement (Windows doit le decompresser
a chaque demarrage).
"""

import os
from pathlib import Path

ONEFILE = os.environ.get("CRYPTOBULLE_ONEFILE") == "1"
icon = Path("assets/icon.ico")

# Modules jamais utilises a l'execution : les exclure allege nettement le resultat.
EXCLUDES = [
    "PIL", "cryptography", "numpy", "pytest", "setuptools", "pip",
    "unittest", "doctest", "pydoc", "email", "http", "html", "xml",
    "xmlrpc", "sqlite3", "bz2", "lzma", "multiprocessing", "asyncio",
    "distutils", "pkg_resources",
]

a = Analysis(
    ["cryptobulle.pyw"],
    pathex=["."],
    binaries=[],
    datas=[],
    hiddenimports=[],
    hookspath=[],
    runtime_hooks=[],
    excludes=EXCLUDES,
    noarchive=False,
)
pyz = PYZ(a.pure)

common = dict(
    name="CryptoBulle",
    debug=False,
    strip=False,
    upx=False,
    console=False,  # pas de fenetre noire au lancement
    icon=str(icon) if icon.exists() else None,
)

if ONEFILE:
    exe = EXE(pyz, a.scripts, a.binaries, a.datas, [], **common)
else:
    exe = EXE(pyz, a.scripts, [], exclude_binaries=True, **common)
    coll = COLLECT(exe, a.binaries, a.datas, strip=False, upx=False, name="CryptoBulle")
