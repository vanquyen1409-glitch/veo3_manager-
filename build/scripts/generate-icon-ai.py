"""
Generate VEO3 Manager app icon via Google Imagen 4.
Saves to build/appicon.png (overwrites the programmatic one).

Usage:
  python build/scripts/generate-icon-ai.py [--ultra|--standard|--fast] [--n 4]
"""

import argparse
import os
import sys
from pathlib import Path

from dotenv import load_dotenv
from google import genai
from google.genai import types

ROOT = Path(__file__).resolve().parents[2]
load_dotenv(ROOT / ".env")

# Tuned prompt for a flat, modern, video/play app icon.
# Imagen 4 follows specific keywords well; we lean on:
#   - explicit composition (rounded square, centered triangle)
#   - explicit colour palette (matches our app's slate-950 background)
#   - explicit style modifiers (flat, modern, premium SaaS)
#   - explicit "no text, no watermark" to avoid spurious labels
PROMPT = """A flat modern app icon, 1024x1024 pixels, perfectly square.
Rounded square shape with a smooth diagonal gradient from deep indigo (#4F46E5) to vibrant violet (#7C3AED).
A bold, clean white play triangle precisely centered, slightly offset to the right for optical balance.
Two subtle vertical film-strip notch patterns flanking the play triangle, hinting at video.
A very soft inner highlight along the top edge for depth without skeuomorphism.
Premium SaaS aesthetic, iOS 17 / macOS Sonoma app icon style, sharp geometry, vector-clean edges,
crisp anti-aliasing, professional, balanced composition.
No text, no letters, no watermark, no border, no shadow outside the square."""

NEGATIVE_PROMPT = "text, letters, words, watermark, signature, border, frame, blurry, gradient banding, noise"


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--ultra", action="store_true", help="Imagen 4 Ultra (paid tier)")
    p.add_argument("--standard", action="store_true", help="Imagen 4 standard (paid tier)")
    p.add_argument("--fast", action="store_true", help="Imagen 4 fast (paid tier)")
    p.add_argument("--free", action="store_true",
                   help="Use gemini-2.5-flash-image (free tier compatible) - default")
    p.add_argument("--n", type=int, default=4, help="Number of variants to generate (1-4)")
    p.add_argument("--out-dir", default=str(ROOT / "build" / "ai-candidates"),
                   help="Where to save candidates before picking the winner")
    args = p.parse_args()

    use_imagen = args.ultra or args.standard or args.fast
    if args.ultra:
        model = "imagen-4.0-ultra-generate-001"
    elif args.fast:
        model = "imagen-4.0-fast-generate-001"
    elif args.standard:
        model = "imagen-4.0-generate-001"
    else:
        model = "gemini-2.5-flash-image"  # free tier default

    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print("ERROR: GEMINI_API_KEY not set. Add it to .env", file=sys.stderr)
        return 1

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    client = genai.Client(api_key=api_key)

    print(f"Model: {model}")
    print(f"Generating {args.n} candidate(s)...")

    saved = []

    if use_imagen:
        # Imagen 4 batch endpoint (paid tier only)
        cfg_kwargs = {"number_of_images": args.n, "aspect_ratio": "1:1"}
        if not args.fast:
            cfg_kwargs["image_size"] = "2K" if args.ultra else "1K"
        response = client.models.generate_images(
            model=model,
            prompt=PROMPT,
            config=types.GenerateImagesConfig(**cfg_kwargs),
        )
        if not response.generated_images:
            print("ERROR: No images returned (likely safety filter)", file=sys.stderr)
            return 1
        for i, gen in enumerate(response.generated_images, start=1):
            path = out_dir / f"candidate-{i:02d}.png"
            path.write_bytes(gen.image.image_bytes)
            saved.append(path)
            print(f"  saved {path}  ({path.stat().st_size:,} bytes)")
    else:
        # gemini-2.5-flash-image: one image per call, loop for variants.
        for i in range(1, args.n + 1):
            response = client.models.generate_content(
                model=model,
                contents=PROMPT,
                config=types.GenerateContentConfig(
                    response_modalities=["Image"],
                    image_config=types.ImageConfig(aspect_ratio="1:1"),
                ),
            )
            data = None
            for part in response.candidates[0].content.parts:
                if getattr(part, "inline_data", None) and part.inline_data.data:
                    data = part.inline_data.data
                    break
            if not data:
                print(f"  candidate {i}: no image (safety filter?), skipping", file=sys.stderr)
                continue
            path = out_dir / f"candidate-{i:02d}.png"
            path.write_bytes(data)
            saved.append(path)
            print(f"  saved {path}  ({path.stat().st_size:,} bytes)")
        if not saved:
            return 1

    print(f"\n{len(saved)} candidate(s) saved to {out_dir}")
    print("Pick the best one and copy over build/appicon.png, e.g.:")
    print(f"  Copy-Item {saved[0]} build\\appicon.png -Force")
    print("Then delete build/windows/icon.ico so Wails regenerates it on next build.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
