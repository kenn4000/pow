# Drop-in skin catalog

The browser Shop discovers card skins from
`backend/internal/skins/shop/<family>/<color>/`. A skin needs no Go or
frontend catalog entry: add one directory, a `skin.json` manifest, and the 12
artwork PNGs listed below. The family and color folders must use lowercase
letters, numbers, and hyphens.

Example `backend/internal/skins/shop/gryphon/teal/skin.json`:

```json
{
  "displayName": "Gryphon (teal)",
  "hexColor": "#168f9b",
  "price": 300,
  "currency": "marks"
}
```

Required manifest fields:

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
| `color` | Color folder name | Color identity used to prevent assigning the same color to multiple suits |
| `rarity` | `common` | Loot-box and Shop rarity |

The artwork filenames are `small`, `med`, and `large`, each paired with
`idle`, `attack`, `win`, and `lose`:

```text
small_idle.png      med_idle.png      large_idle.png
small_attack.png    med_attack.png    large_attack.png
small_win.png       med_win.png       large_win.png
small_lose.png      med_lose.png      large_lose.png
```

After adding a skin, restart the Go backend so it rebuilds and embeds the new
files. On startup, every skin manifest and all 12 artwork files are validated;
invalid metadata or missing art is reported with the affected skin path.
Valid skins appear automatically in the Shop, use the manifest color for
card numbers, and can be assigned to a suit after purchase.
