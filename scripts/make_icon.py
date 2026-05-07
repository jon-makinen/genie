#!/usr/bin/env python3
"""Generate the Genie macOS app icon.

A pure black squircle with a white monospace "G" centered inside, plus a thin
inner border for the brutalist tech vibe. Output: build/appicon.png (1024x1024).

Run:  python3 scripts/make_icon.py
"""
from __future__ import annotations

import os
import sys
from PIL import Image, ImageDraw, ImageFont

SIZE = 1024
MARGIN = 60
CORNER_RADIUS = 220
BORDER_INSET = 18
BORDER_OPACITY = 70
GLYPH = "G"
GLYPH_RATIO = 0.62  # font-size as fraction of the inner square

FONT_CANDIDATES = [
    "/System/Library/Fonts/SFNSMono.ttf",
    "/System/Library/Fonts/Menlo.ttc",
    "/System/Library/Fonts/SFNS.ttf",
    "/System/Library/Fonts/HelveticaNeue.ttc",
]


def load_font(size: int) -> ImageFont.FreeTypeFont:
    for path in FONT_CANDIDATES:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except OSError:
                continue
    print("warning: no system font found, falling back to default bitmap font",
          file=sys.stderr)
    return ImageFont.load_default()


def draw_icon(out: str) -> None:
    img = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Squircle background.
    draw.rounded_rectangle(
        (MARGIN, MARGIN, SIZE - MARGIN, SIZE - MARGIN),
        radius=CORNER_RADIUS,
        fill=(0, 0, 0, 255),
    )

    # Glyph.
    inner = SIZE - 2 * MARGIN
    font_size = int(inner * GLYPH_RATIO)
    font = load_font(font_size)
    bbox = draw.textbbox((0, 0), GLYPH, font=font)
    glyph_w = bbox[2] - bbox[0]
    glyph_h = bbox[3] - bbox[1]
    x = (SIZE - glyph_w) / 2 - bbox[0]
    # Visually center: text bbox is tight to glyph metrics so a small lift looks right.
    y = (SIZE - glyph_h) / 2 - bbox[1] - SIZE * 0.015
    draw.text((x, y), GLYPH, font=font, fill=(245, 245, 245, 255))

    # Inner stroke.
    inset = MARGIN + BORDER_INSET
    draw.rounded_rectangle(
        (inset, inset, SIZE - inset, SIZE - inset),
        radius=CORNER_RADIUS - BORDER_INSET,
        outline=(255, 255, 255, BORDER_OPACITY),
        width=2,
    )

    os.makedirs(os.path.dirname(out) or ".", exist_ok=True)
    img.save(out)
    print(f"wrote {out}  ({SIZE}x{SIZE})")


if __name__ == "__main__":
    out = sys.argv[1] if len(sys.argv) > 1 else "build/appicon.png"
    draw_icon(out)
