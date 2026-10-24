# Meta-progression economy

The progression system is available in the terminal UI. It does not change the
core card deck, exchange resolution, scoring, or AI behavior. The browser game
continues to start without reading or modifying the terminal profile.

## Profile and persistence

The profile is stored at `~/.prisoners-of-war/profile.json` (under the current
user's home directory). It includes a generated player ID, schema version,
timestamps, balances, energy/recharge state, match records, streaks, daily
limits, Camp Draw progress, unlocked skills, chroma ownership and suit-color
assignments, and achievements. The file is written through a same-directory
temporary file with owner-only permissions and renamed into place. An
unsupported schema version is reported as an error; the program does not
silently overwrite it.

New profiles start with 10/10 energy. One energy regenerates every 10 minutes,
up to the 20-energy system cap. The timer is based on elapsed wall-clock time
and is advanced on load, query, and spend. Watching an ad is simulated in the
local CLI: it takes two seconds, restores three energy, and is limited to five
uses per local calendar day. A full refill costs 100 Crowns. The `Second Wind`
skill supplies one free full refill per local day. No real-money purchases or
advertising integrations are enabled; the displayed Crowns note is
informational only.

Starting a match costs one energy, which is saved before play begins. The
profile is saved after match rewards and every shop, skill, Camp Draw, ad, and
refill action.

## Match payouts

The player is Player 1 in terminal matches. Payouts are calculated from the
completed game's final jail scores; a best-of-three payout uses the sum of the
three game scores. The current table is:

| Result | Base Marks | Marks per player score point |
|---|---:|---:|
| Win | 100 | 2 |
| Draw | 60 | 1 |
| Loss | 40 | 1 |

The formula is:

```text
floor((base_marks * (1 + Quartermaster_percent)
       * (1 + first_win_percent_if_eligible)
       + score_bonus_marks) * mode_multiplier)
```

Wins also grant one Crown, with one additional Crown for Ranked and
Friend-ranked wins. Draws and losses grant no Crowns. Crown earnings are
in-game rewards; there is no real-money purchase flow.

`Quartermaster` adds 10% to the base. `First Blood` adds 50% to the base on
the first win of the local day. Score bonuses are suppressed after the first
10 daily wins in CPU and friend modes; base Marks are still paid. Mode
multipliers are 1.0 for Easy AI and casual friend games, 1.05 for Normal AI,
1.1 for Hard AI, 1.5 for local Ranked, and 1.25 for Friend-ranked.
`Field Warden` adds five percentage points to the Ranked multiplier.

Wins advance the current streak, update the best streak, and add one point to
Camp Draw progress. Losses reset the current streak; draws leave it unchanged.
Ten wins award one Camp Draw, up to three stored draws. `Fortunes Favor`
reduces the threshold by two wins (to a minimum of one). Daily counters reset
at local midnight.

## Camp Draw

See [campdraw.md](./campdraw.md) for the minigame, box tables,
pending seasonal boxes, simulation flag, and tuning details.

## Skills, shop, and chromas

Skills have Crown costs and explicit prerequisites. Their effects are
aggregated from unlocked nodes and applied by the payout, energy, and Camp
Draw services. The CLI skill tree shows availability, costs, and prerequisites.

The chroma catalog includes rarity, color, price, currency, and optional bundle
membership. Purchases validate balances and ownership; repeated bundle
purchases are idempotent. New profiles own Tribal (red), Tribal (blue), and
Tribal (yellow). The four Conquistador chromas (lime green, gold, pink, and sky
blue) cost 100 Marks each. Browser suit choices show only owned chromas; each
can be assigned to Scissors (`B`), Paper (`F`), or Rock (`S`) as long as no
other suit uses the same color. The selected chroma controls the deck artwork
and face-number color. Initial assignments are Tribal (blue) for Scissors,
Tribal (yellow) for Paper, and Tribal (red) for Rock.

## CLI

```powershell
GOWORK=off go -C backend run ./cmd/prisoners-of-war --ui terminal
```

The terminal opens on a dashboard with balances, energy and recharge timing,
featured-chroma placeholder, quick actions, login streak, Camp Draw progress,
and unseen-achievement count. The dashboard is redrawn after each sub-screen.

| Key | Action |
|---|---|
| `1` | Play; opens CPU, Ranked, and local friend mode choices |
| `2` | Camp Draw; shown only when a draw is available |
| `3` | Shop |
| `4` | Skill Tree; shows available Skill Tree Points |
| `5` | Achievements; marks newly unlocked entries as viewed |
| `6` | Profile |
| `7` | Settings |
| `W` | Watch an ad for energy; shown only when energy and daily ad capacity remain |
| `R` | Open Crown and daily-free energy refill options |
| `Q` | Save and quit; confirmation can be toggled in Settings |

Settings persist color output and quit-confirmation preferences. The settings
screen displays the profile path and offers profile deletion after two
confirmations. Profiles are schema version 4; older profile data is migrated
without losing existing settings. The featured chroma currently uses the
allowed static “none today” placeholder.

Ranked modes are local adapters around completed games; there is no server,
online matchmaking, or network friend system.

To inspect the current economy values, skill tree, catalog, and bundles as
JSON:

```powershell
GOWORK=off go -C backend run ./cmd/prisoners-of-war --dump-economy
```

For development, set the profile directory through `HOME`/`USERPROFILE`, or
call `profile.LoadProfileFrom` and `Profile.SaveTo` with an explicit test path.

For the complete achievement catalog, set rewards, match history, and gallery
behavior, see [achievements.md](./achievements.md).
