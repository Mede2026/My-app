"""Genere assets/icon.ico a partir du dessin de la zone de notification."""

from pathlib import Path

from cryptobulle.tray import make_icon_image

TARGET = Path(__file__).resolve().parent.parent / "assets" / "icon.ico"


def main() -> None:
    TARGET.parent.mkdir(parents=True, exist_ok=True)
    image = make_icon_image(256)
    image.save(TARGET, sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)])
    print(f"Icone ecrite : {TARGET}")


if __name__ == "__main__":
    main()
