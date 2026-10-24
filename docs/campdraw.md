# Camp Draws and Pending Boxes

Camp Draws are an optional terminal minigame. They use the existing card
resolution rules through `game.Resolve`; the game deck, rules, and AI are not
modified. A Camp Draw consumes one available minigame after all three
exchanges complete.

## Minigame

The player receives three random cards and the CPU receives four. The exchanges
alternate as follows:

1. CPU attacks; the player sees the CPU card and chooses a defense.
2. Player attacks blind; the CPU sees that card and chooses a defense.
3. CPU attacks; the player sees the CPU card and chooses a defense.

The CPU attacks with its highest-face card (ties use suit order). For defense,
it chooses the lowest-face card that wins, otherwise the lowest-face tie, or
the lowest-face loss. The player earns one win whenever their card captures
the CPU card.

| Player wins | Box |
|---:|---|
| 0 | Wooden |
| 1 | Silver |
| 2 | Gold |
| 3 | Diamond |

## Box prize tables

Prize weights below are percentages. Internally weights use integer
hundredths so the half-percent Gold prizes are exact.

| Box | Prize | Chance |
|---|---|---:|
| Wooden | 200 / 500 / 1,000 / 2,000 / 5,000 Marks | 30% / 25% / 15% / 10% / 5% |
| Wooden | 50 Crowns | 10% |
| Wooden | Common chroma | 5% |
| Silver | 800 / 2,000 / 4,000 / 8,000 Marks | 25% / 20% / 15% / 8% |
| Silver | 80 / 150 Crowns | 15% / 5% |
| Silver | Common / Uncommon / Rare chroma | 8% / 3% / 1% |
| Gold | 4,000 / 8,000 / 12,000 / 20,000 Marks | 20% / 18% / 12% / 8% |
| Gold | 150 / 400 Crowns | 12% / 5% |
| Gold | Common / Uncommon / Rare / Epic / Seasonal chroma | 10% / 8% / 5% / 1.5% / 0.5% |
| Diamond | 100 / 200 / 300 / 500 / 800 Crowns | 25% / 20% / 15% / 10% / 5% |
| Diamond | Common / Uncommon / Rare / Epic / Seasonal chroma | 8% / 8% / 6% / 2% / 1% |

`Lucky Find` (the `fortune` skill node) shifts five percentage points from
the highest-value currency prize to the Rare chroma prize when the table
contains a Rare chroma prize. `Curator` rerolls once if a chroma roll selects
an already-owned chroma; a second owned result converts to Marks. Without
Curator, duplicate chromas are granted as owned duplicates for now; this is a
future hook for disenchanting duplicates into Marks. A rolled chroma rarity
with no catalog entries converts to its configured Marks fallback. Seasonal
Ember Dog is event-only and cannot be purchased directly:
5,000 Common, 12,000 Uncommon, 25,000 Rare, 50,000 Epic, or 30,000 Seasonal.

The specified approximate design values are Wooden ~900 Marks, Silver ~3,200,
Gold ~5,300, Diamond ~8,300, and ~3,685 Marks per draw (~$0.74). These are
design estimates, not guaranteed rates; Crown and chroma value depends on the
economy’s valuation assumptions.

## Pending boxes and grants

Profile schema v4 stores pending box counts by type: `wooden`, `silver`,
`gold`, and `diamond`. The landing page shows the total and a Camp Draw action
when either a minigame or pending box is available. Pending boxes skip the
minigame; choose a type, confirm, and one box is opened immediately.

`Profile.GrantBox` adds boxes and validates the type. `lootbox.OpenPendingBox`
validates availability, rolls the table, applies rewards, and decrements the
count. Daily login, challenge, battle-pass, and event grant functions are
currently stubs in `seasons`; they return zero until those systems are built.

## Tuning and simulation

The tuning constants and prize tables live in
[`backend/internal/lootbox/lootbox_tables.go`](../backend/internal/lootbox/lootbox_tables.go):

| Constant | Value |
|---|---:|
| CPU hand size | 4 |
| Wins per Camp Draw | 10 |
| Maximum stored draws | 3 |
| Base threshold with Fortunes Favor | 8 |

The CPU hand size of four follows the attached Python simulation, which
compared CPU hands of three, four, and five cards. Fortunes Favor reduces the
base ten-win threshold by two.

Run headless simulations without loading or saving a player profile:

```powershell
GOWORK=off go -C backend run ./cmd/prisoners-of-war --sim-campdraw 5000
```

The output includes win-count and box distributions plus average Marks,
Crowns, and chromas per draw. Increase the draw count for smoother estimates.
