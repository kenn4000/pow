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
| Wooden | 55 / 115 / 240 / 500 / 1,000 Marks + 1 Crown | 38% / 34% / 17% / 3% / 1% |
| Wooden | 1 Energy + 1 Crown | 5% |
| Wooden | Random common skin | 2% |
| Silver | 105 / 210 / 420 / 850 / 1,500 Marks + 2 Crowns | 32% / 32% / 18% / 4% / 2% |
| Silver | 1 Energy + 2 Crowns | 8% |
| Silver | Random common / uncommon skin | 3% / 1% |
| Gold | 115 / 240 / 490 / 1,000 / 2,200 Marks + 4 Crowns | 32% / 27% / 18% / 4% / 1% |
| Gold | 2 Energy + 4 Crowns | 8% |
| Gold | Random common / uncommon / rare skin | 5% / 3% / 2% |
| Diamond | 500 / 1,000 / 2,000 / 4,000 Marks + 8 Crowns | 50% / 25% / 9% / 5% |
| Diamond | 10 Energy + 8 Crowns | 2% |
| Diamond | Random rare / epic / seasonal or premium skin | 5% / 3% / 1% |

Wooden Boxes cost 300 Marks or 13 Crowns, Silver Boxes cost 600 Marks or 25
Crowns, and Gold Boxes cost 1,200 Marks or 50 Crowns. Diamond Boxes are
available only from Camp Draw wins.

`Lucky Find` (the `fortune` skill node) shifts up to five percentage points from
the highest-value currency prize to the Rare skin prize when the table
contains a Rare skin prize. If a box selects a skin the player already owns,
the player receives Marks equal to that skin's listed price. A rolled skin
rarity with no catalog entries converts to its configured Marks fallback:
5,000 Common, 12,000 Uncommon, 25,000 Rare, 50,000 Epic, or 30,000 Seasonal
and Premium.

## Pending boxes and grants

Profile schema v11 stores pending box counts by type: `wooden`, `silver`,
`gold`, and `diamond`. The landing page shows the total and a Camp Draw action
when either a minigame or pending box is available. Pending boxes skip the
minigame; choose a type, confirm, and one box is opened immediately.
`Profile.GrantBox` adds boxes and validates the type. `lootbox.OpenPendingBox`
validates availability, rolls the table, applies rewards, and decrements the
count. The Shop sells one Camp Draw for 1,500 Marks or 75 Crowns, or three for
4,000 Marks or 200 Crowns. Purchased draws use the same three-draw storage cap
as draws earned through wins. Daily login, challenge, battle-pass, and event
grant functions are currently stubs in `seasons`; they return zero until those
systems are built.

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
