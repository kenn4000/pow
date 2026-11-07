# Drop-in skin catalog

The browser Shop discovers skins from paired files in
`backend/internal/skins/skinshop/`: a `<family>_<color>.png` spritesheet and a
matching JSON metadata file. Family and color names use lowercase letters,
numbers, and hyphens. The JSON contains a `frames` entry for each of the 12
artwork names below, plus a `meta.skinData` object with the former `skin.json`
fields.

Example `backend/internal/skins/skinshop/gryphon_teal.json` metadata (the frame
map is abbreviated here; all 12 entries are required):

```json
{
  "frames": {
    "small_idle.png": {
      "frame": { "x": 0, "y": 0, "w": 366, "h": 512 },
      "rotated": false,
      "trimmed": false,
      "sourceSize": { "w": 366, "h": 512 }
    }
  },
  "meta": {
    "image": "gryphon_teal.png",
    "size": { "w": 1464, "h": 1536 },
    "skinData": {
      "displayName": "Gryphon (teal)",
      "hexColor": "#168f9b",
      "price": 300,
      "currency": "marks"
    }
  }
}
```

Required `meta.skinData` fields:

| Field | Requirements |
|---|---|
| `displayName` | Non-empty Shop name |
| `hexColor` | Six-digit hex color including `#`; used for card face numbers |
| `price` | Positive integer |
| `currency` | `marks` or `crowns` |

Optional fields:

| Field | Default | Purpose |
|---|---|---|
| `id` | Family and color joined with `-` | Stable chroma ID; existing skins set explicit IDs to preserve player ownership |
| `color` | Color suffix in the sheet filename | Color identity used to prevent assigning the same color to multiple suits |
| `rarityRank` | `0` | Shop tier and loot-box rarity: `0` common, `1` uncommon, `2` rare, `3` epic, `4` seasonal/premium |
| `rarity` | — | Legacy metadata; effective rarity is derived from `rarityRank` |

The Shop groups families alphabetically within each tier. If a family has
colors in multiple tiers, it has a separate tile in every tier and each tile
previews a color from that tier. Opening any tile shows all colors in the
family, their metadata prices, sprite-sheet previews, and a 20%-off full-family
purchase when all purchasable colors use the same currency and none are owned.
The discount is calculated from the sum of the individual sheet prices.

The required frame names are `small`, `med`, and `large`, each paired with
`idle`, `attack`, `win`, and `lose`:

```text
small_idle.png      med_idle.png      large_idle.png
small_attack.png    med_attack.png    large_attack.png
small_win.png       med_win.png       large_win.png
small_lose.png      med_lose.png      large_lose.png
```

The `skinData` object uses the same fields and requirements as the former
`skin.json` manifest. The backend validates every metadata-bearing spritesheet,
its PNG dimensions, and all 12 frame bounds at startup. Sheets ending in
`__scaled` are treated as auxiliary exports and ignored. Restart the Go backend
after adding files so the embedded spritesheets are rebuilt. Each card artwork
URL is cropped from its family/color sheet, so the game no longer requires a
separate PNG file for every pose and card size.
