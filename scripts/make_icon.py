"""Genere assets/icon.ico. Utilise Pillow, uniquement au moment de la construction.

L'application finale ne depend pas de Pillow : elle charge simplement l'icone
deja integree a l'executable.
"""

from pathlib import Path

from PIL import Image, ImageDraw

TARGET = Path(__file__).resolve().parent.parent / "assets" / "icon.ico"
SIZES = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]


def draw_padlock(size: int = 256) -> Image.Image:
    """Dessine un cadenas bleu sur fond transparent."""
    image = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    blue, light = (63, 140, 255, 255), (233, 240, 255, 255)
    unit = size / 64

    draw.arc(
        [18 * unit, 8 * unit, 46 * unit, 40 * unit],
        start=180, end=360, fill=blue, width=int(6 * unit),
    )
    draw.rounded_rectangle(
        [12 * unit, 28 * unit, 52 * unit, 56 * unit], radius=int(8 * unit), fill=blue
    )
    draw.ellipse([28 * unit, 36 * unit, 36 * unit, 44 * unit], fill=light)
    draw.rectangle([30.5 * unit, 41 * unit, 33.5 * unit, 50 * unit], fill=light)
    return image


def main() -> None:
    TARGET.parent.mkdir(parents=True, exist_ok=True)
    draw_padlock(256).save(TARGET, sizes=SIZES)
    print(f"Icone ecrite : {TARGET}")


if __name__ == "__main__":
    main()
