"""
Clean an AI-generated app icon: detect the rounded-square subject, clip everything
outside it to alpha=0, resize/center to a clean 1024x1024 with iOS-HIG margins.

Strategy
--------
1. Convert to HSV. The icon's rounded square is highly saturated (gradient violet).
   The background (white OR black OR dark glow) has low saturation.
2. Build a binary mask of "saturated subject" pixels.
3. Take the bounding box of that mask -> that's our rounded square's footprint.
4. Build a fresh rounded-square alpha mask matching that bbox.
5. Composite the original RGB through that mask onto a transparent canvas,
   then pad to 1024x1024 with the standard iOS app-icon margin (~10%).

Usage
-----
    python build/scripts/clip-icon.py [input.png]
    # default input: build/ai-candidates/from-user.png
    # default output: build/appicon.png  (also writes a side-by-side preview)
"""

from __future__ import annotations

import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

ROOT = Path(__file__).resolve().parents[2]
DEFAULT_IN = ROOT / "build" / "ai-candidates" / "from-user.png"
DEFAULT_OUT = ROOT / "build" / "appicon.png"
PREVIEW = ROOT / "build" / "ai-candidates" / "preview-before-after.png"

FINAL_SIZE = 1024
ICON_MARGIN = 0.04   # 4% padding on all sides (iOS HIG actually uses 0%, but
                      # taskbar/Linux launchers crop slightly so a tiny margin helps)
CORNER_RADIUS_PCT = 0.22  # 22% of the square's side, matches iOS app icons


def detect_subject_bbox(img: Image.Image) -> tuple[int, int, int, int]:
    """Bounding box of non-transparent pixels.

    If the image already has a proper alpha channel (most AI image gen output
    these days), use it directly. Otherwise fall back to saturation detection.
    """
    if img.mode == "RGBA":
        a = img.split()[3]
        # Threshold alpha at 32 to reject very faint glow halos.
        mask = a.point(lambda p: 255 if p > 32 else 0, mode="L")
        bbox = mask.getbbox()
        if bbox is not None:
            return bbox
    # Fallback: saturated subject (for images without alpha)
    hsv = img.convert("HSV")
    _, s, _ = hsv.split()
    sat_mask = s.point(lambda p: 255 if p > 30 else 0, mode="L")
    bbox = sat_mask.getbbox()
    if bbox is None:
        raise RuntimeError("Could not detect any subject in the image")
    return bbox


def make_rounded_mask(size: tuple[int, int], radius: int) -> Image.Image:
    """An anti-aliased rounded-rectangle mask the size of the subject bbox."""
    # Render at 4x for AA, then downscale.
    sx, sy = size
    big = Image.new("L", (sx * 4, sy * 4), 0)
    d = ImageDraw.Draw(big)
    d.rounded_rectangle((0, 0, sx * 4 - 1, sy * 4 - 1),
                        radius=radius * 4, fill=255)
    return big.resize((sx, sy), Image.LANCZOS)


def main() -> int:
    inp = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_IN
    if not inp.exists():
        print(f"ERROR: input not found: {inp}", file=sys.stderr)
        return 1

    src = Image.open(inp).convert("RGBA")
    print(f"Loaded {inp}  ({src.width} x {src.height})")

    # --- 1) Detect the rounded square's bounding box --------------------
    bbox = detect_subject_bbox(src)
    bx, by, bx2, by2 = bbox
    print(f"Subject bbox: ({bx},{by}) -> ({bx2},{by2})  size {bx2-bx} x {by2-by}")

    # Force the bbox to be square (use the larger side, recenter)
    bw, bh = bx2 - bx, by2 - by
    side = max(bw, bh)
    cx, cy = (bx + bx2) // 2, (by + by2) // 2
    half = side // 2
    bx, by, bx2, by2 = cx - half, cy - half, cx + half, cy + half
    bx = max(0, bx); by = max(0, by)
    bx2 = min(src.width, bx2); by2 = min(src.height, by2)
    bw, bh = bx2 - bx, by2 - by
    print(f"Squared bbox: ({bx},{by}) -> ({bx2},{by2})  size {bw} x {bh}")

    # --- 2) Crop, then apply rounded-rectangle alpha mask ----------------
    cropped = src.crop((bx, by, bx2, by2)).convert("RGBA")
    radius = int(min(cropped.size) * CORNER_RADIUS_PCT)
    mask = make_rounded_mask(cropped.size, radius)
    # Slight inward erosion so the corner anti-aliasing eats any 1-pixel halo
    mask = mask.filter(ImageFilter.MinFilter(3))

    clipped = Image.new("RGBA", cropped.size, (0, 0, 0, 0))
    clipped.paste(cropped, (0, 0), mask)

    # --- 3) Resize to FINAL_SIZE with margin -----------------------------
    inner = int(FINAL_SIZE * (1 - 2 * ICON_MARGIN))
    clipped_resized = clipped.resize((inner, inner), Image.LANCZOS)
    out = Image.new("RGBA", (FINAL_SIZE, FINAL_SIZE), (0, 0, 0, 0))
    pad = (FINAL_SIZE - inner) // 2
    out.paste(clipped_resized, (pad, pad), clipped_resized)

    # --- 4) Save ---------------------------------------------------------
    DEFAULT_OUT.parent.mkdir(parents=True, exist_ok=True)
    out.save(DEFAULT_OUT, format="PNG", optimize=True)
    print(f"Wrote {DEFAULT_OUT}  ({DEFAULT_OUT.stat().st_size:,} bytes)")

    # --- 5) Side-by-side preview against checker pattern -----------------
    PREVIEW.parent.mkdir(parents=True, exist_ok=True)
    checker = make_checker(FINAL_SIZE * 2 + 40, FINAL_SIZE)
    composite = Image.alpha_composite(checker.convert("RGBA"),
                                      paste_pair(src, out, FINAL_SIZE))
    composite.save(PREVIEW, format="PNG", optimize=True)
    print(f"Wrote preview {PREVIEW}")
    return 0


def make_checker(w: int, h: int, sq: int = 32) -> Image.Image:
    """Light-grey checker pattern so transparency shows clearly."""
    img = Image.new("RGB", (w, h), (240, 240, 240))
    d = ImageDraw.Draw(img)
    for y in range(0, h, sq):
        for x in range(0, w, sq):
            if (x // sq + y // sq) % 2 == 0:
                d.rectangle((x, y, x + sq, y + sq), fill=(208, 208, 208))
    return img


def paste_pair(left: Image.Image, right: Image.Image, size: int) -> Image.Image:
    """Two icons side by side on a single transparent canvas, both at `size`."""
    canvas = Image.new("RGBA", (size * 2 + 40, size), (0, 0, 0, 0))
    l = left.convert("RGBA").resize((size, size), Image.LANCZOS)
    r = right.convert("RGBA").resize((size, size), Image.LANCZOS)
    canvas.paste(l, (0, 0), l)
    canvas.paste(r, (size + 40, 0), r)
    return canvas


if __name__ == "__main__":
    sys.exit(main())
