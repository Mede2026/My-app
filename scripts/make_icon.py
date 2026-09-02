"""Genere assets/icon.ico : un cadenas bleu, dessine sans aucune bibliotheque.

Seule la bibliotheque standard de Python est utilisee (zlib pour la compression
PNG, binascii pour les sommes de controle). Le fichier produit contient des
images PNG, ce que Windows accepte depuis Vista, ce qui le garde tres petit.

Lancer : python3 scripts/make_icon.py
"""

from __future__ import annotations

import binascii
import struct
import zlib
from pathlib import Path

TARGET = Path(__file__).resolve().parent.parent / "internal" / "app" / "assets" / "icon.ico"
SIZES = (16, 24, 32, 48, 64, 128, 256)

BLUE = (63, 140, 255)
LIGHT = (233, 240, 255)
SUPERSAMPLE = 4  # on dessine en plus grand puis on reduit : bords lisses


def _shape_alpha(x: float, y: float) -> tuple[tuple[int, int, int], float]:
    """Couleur et opacite du cadenas au point (x, y), en coordonnees 0-64."""
    # Anse : anneau ouvert vers le bas.
    ring_x, ring_y, radius, thickness = 32.0, 24.0, 14.0, 3.2
    distance = ((x - ring_x) ** 2 + (y - ring_y) ** 2) ** 0.5
    if y <= ring_y and abs(distance - radius) <= thickness:
        return BLUE, 1.0

    # Corps : rectangle aux coins arrondis.
    left, top, right, bottom, corner = 12.0, 28.0, 52.0, 56.0, 8.0
    if left <= x <= right and top <= y <= bottom:
        inside_x = min(max(x, left + corner), right - corner)
        inside_y = min(max(y, top + corner), bottom - corner)
        if ((x - inside_x) ** 2 + (y - inside_y) ** 2) ** 0.5 <= corner:
            # Trou de serrure : rond prolonge d'une fente.
            hole = ((x - 32.0) ** 2 + (y - 40.0) ** 2) ** 0.5 <= 4.2
            slot = 30.6 <= x <= 33.4 and 40.0 <= y <= 50.0
            return (LIGHT if hole or slot else BLUE), 1.0
    return BLUE, 0.0


def render(size: int) -> bytes:
    """Dessine l'icone en RGBA brut, avec lissage par sur-echantillonnage."""
    rows = []
    step = 64.0 / (size * SUPERSAMPLE)
    for pixel_y in range(size):
        row = bytearray()
        for pixel_x in range(size):
            red = green = blue = alpha = 0.0
            for sub_y in range(SUPERSAMPLE):
                for sub_x in range(SUPERSAMPLE):
                    x = (pixel_x * SUPERSAMPLE + sub_x + 0.5) * step
                    y = (pixel_y * SUPERSAMPLE + sub_y + 0.5) * step
                    color, opacity = _shape_alpha(x, y)
                    if opacity:
                        red += color[0]
                        green += color[1]
                        blue += color[2]
                        alpha += 1.0
            if alpha:
                row += bytes((round(red / alpha), round(green / alpha), round(blue / alpha)))
                row.append(round(255 * alpha / (SUPERSAMPLE * SUPERSAMPLE)))
            else:
                row += b"\x00\x00\x00\x00"
        rows.append(bytes(row))
    return b"".join(rows)


def _chunk(kind: bytes, payload: bytes) -> bytes:
    return (
        struct.pack(">I", len(payload))
        + kind
        + payload
        + struct.pack(">I", binascii.crc32(kind + payload) & 0xFFFFFFFF)
    )


def to_png(size: int, pixels: bytes) -> bytes:
    """Encode une image RGBA en PNG (sans dependance externe)."""
    stride = size * 4
    scanlines = b"".join(b"\x00" + pixels[y * stride : (y + 1) * stride] for y in range(size))
    header = struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)  # 8 bits, RGBA
    return (
        b"\x89PNG\r\n\x1a\n"
        + _chunk(b"IHDR", header)
        + _chunk(b"IDAT", zlib.compress(scanlines, 9))
        + _chunk(b"IEND", b"")
    )


def build_ico(images: dict[int, bytes]) -> bytes:
    """Assemble les images PNG dans un fichier .ico."""
    header = struct.pack("<HHH", 0, 1, len(images))  # 1 = icone
    offset = len(header) + 16 * len(images)
    entries, payload = b"", b""
    for size, data in sorted(images.items()):
        dimension = 0 if size >= 256 else size  # 0 signifie 256 dans le format
        entries += struct.pack(
            "<BBBBHHII", dimension, dimension, 0, 0, 1, 32, len(data), offset
        )
        payload += data
        offset += len(data)
    return header + entries + payload


def main() -> None:
    images = {size: to_png(size, render(size)) for size in SIZES}
    TARGET.parent.mkdir(parents=True, exist_ok=True)
    TARGET.write_bytes(build_ico(images))
    print(f"Icone ecrite : {TARGET} ({TARGET.stat().st_size / 1024:.1f} Ko)")


if __name__ == "__main__":
    main()
