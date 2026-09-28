# Social assets

| File | What it is |
|---|---|
| `yard-share-card.png` | 1200x630 light card: README hero and Open Graph image |
| `yard-share-card-dark.png` | 1200x630 dark card: README hero when the reader's theme is dark (`<picture>`) |
| `build-share-cards.py` | Generates both PNGs from one palette |

## Rebuild

```bash
python3 docs/social/build-share-cards.py
```

Needs `rsvg-convert` (librsvg, `brew install librsvg`). The script embeds the real console screenshot
`docs/ux/00-overview.png` as a data URI (librsvg refuses relative image paths that leave the SVG's
directory) and writes the PNGs next to itself. To change the copy or the chips, edit the script.

## Palette

The apple.com blue and white system shared by the Zyvor repos.

| Role | Light | Dark |
|---|---|---|
| Background | `#ffffff` to `#f5f5f7`, faint blue wash | `#000000` to `#0b0b0f`, blue wash |
| Ink / secondary | `#1d1d1f` / `#6e6e73` | `#f5f5f7` / `#a1a1a6` |
| Hairline / card | `#d2d2d7` / `#ffffff` | `#3a3a3c` / `#1c1c1e` |
| Blue | `#0071e3` to `#2997ff` | `#0a84ff` to `#5eb0ff` |

Type is Helvetica Neue with Menlo for code-ish labels. The Zyvor "Z" mark is drawn in blue. The only orange
(`#ff6a2a`) is the single dot in the caption; the small orange logo inside the screenshot is part of the real
console capture.

Copy is verified against `README.md` and `go.mod` (standalone, Apache-2.0, Go 1.27+). The caption is true
because `docs/ux/*.png` are captures from a lab deployment.

## GitHub Social preview

The repository's Social preview cannot be set through the API or `gh`. After merging, upload
`yard-share-card.png` by hand under Settings, Social preview.
