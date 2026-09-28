# Copyright 2026 Zyvor AI Labs · https://zyvor.dev
# SPDX-License-Identifier: Apache-2.0
"""Render the Yard share cards (light + dark) from one palette.

    python3 docs/social/build-share-cards.py

Needs `rsvg-convert` (librsvg). The real console screenshot docs/ux/00-overview.png
is embedded as a data URI at build time (librsvg refuses relative image paths that
leave the SVG's directory), and the PNGs are written next to this script:

    docs/social/yard-share-card.png        light, 1200x630 (README hero, site OG)
    docs/social/yard-share-card-dark.png   dark,  1200x630 (README dark mode)
"""
import base64
import pathlib
import subprocess
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
SHOT = HERE.parent / "ux" / "00-overview.png"
SHOT_W, SHOT_H = 1568, 727  # pixel size of the screenshot

LIGHT = dict(
    bg0="#ffffff", bg1="#f5f5f7", wash="#0071e3", wash_op="0.10", wash2_op="0.05",
    ink="#1d1d1f", sec="#6e6e73", card="#ffffff", card_stroke="#d2d2d7", shadow_op="0.16",
    blue0="#0071e3", blue1="#2997ff", chip_bg="#e8f1fd", chip_stroke="#b9d6f7", chip_ink="#0058b0",
    bar="#f2f2f5", dot="#d2d2d7", hair="#e5e5ea")
DARK = dict(
    bg0="#000000", bg1="#0b0b0f", wash="#2997ff", wash_op="0.20", wash2_op="0.08",
    ink="#f5f5f7", sec="#a1a1a6", card="#1c1c1e", card_stroke="#3a3a3c", shadow_op="0.70",
    blue0="#0a84ff", blue1="#5eb0ff", chip_bg="#10233d", chip_stroke="#1f4a80", chip_ink="#7cc0ff",
    bar="#242426", dot="#48484a", hair="#2c2c2e")

SANS = "'Helvetica Neue',Helvetica,Arial,sans-serif"
MONO = "'Menlo','JetBrains Mono',monospace"

# Verified against README.md / go.mod: standalone, Apache-2.0, Go 1.27+.
CHIPS = [("RUNS STANDALONE", True), ("APACHE-2.0", False), ("GO 1.27+", False)]


def chips(p):
    out, x = [], 72
    for label, filled in CHIPS:
        w = int(len(label) * 8.7 + 40)
        if filled:
            out.append(f'<rect x="{x}" y="452" width="{w}" height="38" rx="19" fill="url(#blue)"/>'
                       f'<text x="{x + w / 2}" y="476" text-anchor="middle" font-family="{MONO}" '
                       f'font-size="13.5" font-weight="700" letter-spacing="0.6" fill="#ffffff">{label}</text>')
        else:
            out.append(f'<rect x="{x}" y="452" width="{w}" height="38" rx="19" fill="{p["chip_bg"]}" '
                       f'stroke="{p["chip_stroke"]}"/>'
                       f'<text x="{x + w / 2}" y="476" text-anchor="middle" font-family="{MONO}" '
                       f'font-size="13.5" font-weight="700" letter-spacing="0.6" fill="{p["chip_ink"]}">{label}</text>')
        x += w + 12
    return "\n    ".join(out)


def svg(p, shot_uri):
    win_x, win_w = 600, 548
    pad = 10
    bar_h = 34
    img_w = win_w - 2 * pad
    img_h = round(img_w * SHOT_H / SHOT_W)
    win_h = bar_h + img_h + pad
    win_y = 300 - win_h // 2 - 14
    img_x, img_y = win_x + pad, win_y + bar_h
    cap_y = win_y + win_h + 44
    label = ("Yard — open asset and operations platform: devices, sites, telemetry, incidents "
             "and work orders. Runs standalone.")
    return f'''<!-- Copyright 2026 Zyvor AI Labs · https://zyvor.dev -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="1200" height="630" viewBox="0 0 1200 630" role="img" aria-label="{label}">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="630" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="{p['bg0']}"/>
      <stop offset="1" stop-color="{p['bg1']}"/>
    </linearGradient>
    <radialGradient id="wash" cx="900" cy="300" r="540" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="{p['wash']}" stop-opacity="{p['wash_op']}"/>
      <stop offset="0.6" stop-color="{p['wash']}" stop-opacity="{p['wash2_op']}"/>
      <stop offset="1" stop-color="{p['wash']}" stop-opacity="0"/>
    </radialGradient>
    <linearGradient id="blue" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="{p['blue0']}"/>
      <stop offset="1" stop-color="{p['blue1']}"/>
    </linearGradient>
    <linearGradient id="blueText" x1="72" y1="0" x2="330" y2="0" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="{p['blue0']}"/>
      <stop offset="1" stop-color="{p['blue1']}"/>
    </linearGradient>
    <filter id="shadow" x="-10%" y="-15%" width="120%" height="140%" color-interpolation-filters="sRGB">
      <feGaussianBlur in="SourceAlpha" stdDeviation="16"/>
      <feOffset dy="14" result="b"/>
      <feColorMatrix in="b" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 {p['shadow_op']} 0" result="s"/>
      <feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
    <clipPath id="shotClip"><rect x="{img_x}" y="{img_y}" width="{img_w}" height="{img_h}" rx="9"/></clipPath>
  </defs>

  <rect width="1200" height="630" fill="url(#bg)"/>
  <rect width="1200" height="630" fill="url(#wash)"/>

  <!-- Zyvor mark (blue) -->
  <rect x="72" y="56" width="56" height="56" rx="13" fill="url(#blue)"/>
  <path d="M86.5 70 113.5 70 86.5 98 113.5 98" fill="none" stroke="#ffffff" stroke-width="7"
        stroke-linecap="round" stroke-linejoin="round"/>

  <!-- wordmark + tagline -->
  <text x="68" y="262" font-family="{SANS}" font-size="132" font-weight="700" letter-spacing="-4" fill="{p['ink']}">Yard</text>
  <g font-family="{SANS}" font-size="38" font-weight="600" letter-spacing="-0.6">
    <text x="72" y="332" fill="{p['sec']}">Register assets. See health.</text>
    <text x="72" y="380" fill="url(#blueText)">Close the work.</text>
  </g>

  <!-- chips -->
  <g>
    {chips(p)}
  </g>

  <!-- footer -->
  <line x1="72" y1="546" x2="536" y2="546" stroke="{p['hair']}" stroke-width="1"/>
  <text x="72" y="584" font-family="{MONO}" font-size="17" fill="{p['blue0']}">zyvorai.github.io/yard</text>
  <text x="536" y="584" text-anchor="end" font-family="{SANS}" font-size="16" fill="{p['sec']}">Zyvor</text>

  <!-- the real console, in a window frame -->
  <g filter="url(#shadow)">
    <rect x="{win_x}" y="{win_y}" width="{win_w}" height="{win_h}" rx="16" fill="{p['card']}" stroke="{p['card_stroke']}"/>
  </g>
  <path d="M{win_x} {win_y + 16} a16 16 0 0 1 16 -16 h{win_w - 32} a16 16 0 0 1 16 16 v{bar_h - 16} h-{win_w} z" fill="{p['bar']}"/>
  <g fill="{p['dot']}">
    <circle cx="{win_x + 22}" cy="{win_y + 17}" r="5"/>
    <circle cx="{win_x + 40}" cy="{win_y + 17}" r="5"/>
    <circle cx="{win_x + 58}" cy="{win_y + 17}" r="5"/>
  </g>
  <image x="{img_x}" y="{img_y}" width="{img_w}" height="{img_h}" preserveAspectRatio="xMidYMid slice"
         clip-path="url(#shotClip)" xlink:href="{shot_uri}"/>
  <rect x="{img_x}" y="{img_y}" width="{img_w}" height="{img_h}" rx="9" fill="none" stroke="{p['card_stroke']}" stroke-opacity="0.6"/>

  <!-- caption: the single orange dot -->
  <circle cx="{win_x + 8}" cy="{cap_y - 5}" r="5" fill="#ff6a2a"/>
  <text x="{win_x + 24}" y="{cap_y}" font-family="{SANS}" font-size="17" fill="{p['sec']}">Captured against a live lab deployment, not a mockup.</text>
</svg>
'''


def main():
    uri = "data:image/png;base64," + base64.b64encode(SHOT.read_bytes()).decode()
    for name, palette in (("yard-share-card", LIGHT), ("yard-share-card-dark", DARK)):
        with tempfile.TemporaryDirectory() as tmp:
            src = pathlib.Path(tmp) / f"{name}.svg"
            src.write_text(svg(palette, uri))
            out = HERE / f"{name}.png"
            subprocess.run(["rsvg-convert", "-w", "1200", str(src), "-o", str(out)], check=True)
            print("wrote", out, f"({out.stat().st_size // 1024} KB)")


if __name__ == "__main__":
    main()
