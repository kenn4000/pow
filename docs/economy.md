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
uses per local calendar day. The Shop sells +1 Energy for 500 Marks or 25
Crowns, and +10 Energy for 4,500 Marks or 225 Crowns. A purchase is available
only when the whole recharge fits below the current energy cap. A full refill
costs 100 Crowns. `Quick Recovery` shortens the recharge interval by 10%;
`Second Wind` supplies one free full refill per local day; `Overflow` lets
passive regeneration fill up to twice the upgraded Energy maximum. The Shop's
Energy & Items tab also sells Skill Points (1 for 300 Crowns; 10 for 2,400),
Camp Draw games (1 for
1,500 Marks or 75 Crowns; 3 for 4,000 Marks or 200 Crowns), Restart Mission
tokens (1 for 400 Marks or 20 Crowns; 6 for 2,000 Marks or 100 Crowns), Undo
Last Move tokens (1 for 250 Marks or 12 Crowns; 10 for 2,000 Marks or 100
Crowns), Replace Any Card tokens (1 for 180 Marks or 9 Crowns; 10 for 1,500
Marks or 75 Crowns), and Campaign Mission Skip tokens (1 for 1,500 Marks or
75 Crowns).
Camp Draws share the existing three-draw storage limit.

Restart, Undo, Replace Any Card, and Campaign Mission Skip tokens are
server-validated and usable only in campaigns and vs-CPU games, never in
Human-vs-Human play. The in-game token bar shows inventory and permits Crown-only
quick purchases; Marks purchases are available in the Shop. Restart returns to
the mission or match's initial state without charging Energy again. Undo
restores the previous player-action state, including undoing an automatic CPU
response. Replace Any Card discards a selected available card and draws a random
replacement from that variant's remaining deck in Main, Fog, Puzzler, Abandon,
Rummy, Poker, Ladder, and Tug War, including their CPU variants. A Campaign
Mission Skip token advances the next playable mission without mission Marks or
completion rewards.
Skipping the final mission marks the campaign complete but still withholds
completion rewards. No real-money purchases or advertising integrations are
enabled; the displayed Crowns note is informational only.

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

The ordinary match payout applies Quartermaster to base Marks, then applies
eligible daily-win and loss bonuses to the base-plus-score payout:

```text
floor((base_marks * (1 + Quartermaster_percent) + score_bonus_marks)
      * daily_win_multiplier * loss_multiplier * mode_multiplier)
      + flat_mode_rewards
```

Wins also grant one Crown, with one additional Crown for Ranked and
Friend-ranked wins. Draws and losses grant no Crowns. Crown earnings are
in-game rewards; there is no real-money purchase flow.

`Quartermaster` adds 10% to base Marks. `First Blood` and `Daily Double` each
add 50% to all Marks on the first and second win of the local day,
respectively, in any mode. `Salvager` adds 15% to loss Marks. Score bonuses
are suppressed after the first 10 daily wins in CPU and friend modes; base
Marks are still paid. Mode multipliers are 1.0 for Easy AI and casual friend
games, 1.05 for Normal AI, 1.1 for Hard AI, 1.5 for Ranked, and 1.25 for
Friend-ranked. `Field Warden` adds 15 percentage points to a ranked mode's
multiplier. `Mark of Victory` adds 20% Marks on multiplayer wins.

The first multiplayer win each day retains the 15-Crown daily reward;
`Daily Triumph` adds 5 more Crowns to it. `Bounty Board` grants 50 Marks on
the first win of each mode each day. `Streak` awards 3 Crowns at every third
consecutive ranked win across ranked modes and resets the ranked streak on a
loss. `Bounty Hunter` adds 250 Marks to each ranked best-of-three win.

Wins advance the current streak, update the best streak, and add one point to
Camp Draw progress. Losses reset the current streak; draws leave it unchanged.
Ten wins award one Camp Draw, up to three stored draws. `Camp Lore`
reduces the threshold by two wins (to a minimum of one). Daily counters reset
at local midnight.

## Camp Draw and boxes

See [campdraw.md](./campdraw.md) for the minigame, box tables,
pending seasonal boxes, simulation flag, and tuning details.

The Shop sells Wooden Boxes for 300 Marks or 13 Crowns, Silver Boxes for 600
Marks or 25 Crowns, and Gold Boxes for 1,200 Marks or 50 Crowns. Diamond Boxes
are only awarded through Camp Draw. Skin rewards already owned convert to
Marks equal to that skin's listed price.

Hero Boxes cost 8,000 Marks (7,200 with Hero Broker). `Veteran's Discount`
reduces Marks prices for Shop chests by 5%; `Thrifty Quartermaster` reduces
Marks prices for consumables by 10%. Each of the ten
non-Recruit heroes has an equal 10% chance. A duplicate awards 50% of that
hero's listed Crown purchase price. `Lucky Duplicate` adds 20% to that refund;
`Premium Duplicate` adds a further 30%.

## Skills, shop, and chromas

The 27-node tree has six branches: Daily Wins, Multiplayer, General Rewards,
Energy, Collection, and Camp. T1 nodes cost 1 Skill Point, T2 nodes cost 2,
and T3 nodes cost 3. Prerequisites must be unlocked first; Crowns cannot be
used to unlock nodes. Skill Points can be earned through progression rewards
or purchased in the Shop for 300 Crowns each or 2,400 Crowns for ten.

| Branch | Nodes and effects |
| --- | --- |
| Daily Wins | First Blood: +50% Marks on the first daily win. Daily Double: +50% on the second daily win (requires First Blood). Rations & Readiness: first daily campaign win grants one Restart token, one Undo token, and one Energy (requires First Blood). Daily Triumph: +5 Crowns on the first daily multiplayer win (requires Daily Double). |
| Multiplayer | Streak: +3 Crowns for every three consecutive ranked wins across modes. Field Warden: +15 percentage points to ranked payout multipliers (requires Streak). Mark of Victory: +20% Marks on multiplayer wins (requires Streak). Bounty Hunter: +250 Marks on ranked best-of-three wins (requires Mark of Victory). |
| General Rewards | Quartermaster: +10% base match Marks. Salvager: +15% Marks on losses. Curator: +50% campaign Marks (requires Quartermaster). Bounty Board: +50 Marks on the first win in each mode per day (requires Salvager). |
| Energy | Expanded Rations: +2 maximum Energy. Quick Recovery: 10% faster recharge (requires Expanded Rations). Stockpile: +4 maximum Energy (requires Expanded Rations). Second Wind: one daily free refill (requires Quick Recovery). Overflow: passive regeneration up to twice maximum Energy (requires Stockpile). |
| Collection | Thrifty Quartermaster: consumables cost 10% fewer Marks. Hero Broker: Hero Boxes cost 10% fewer Marks (requires Thrifty Quartermaster). Veteran's Discount: Shop chests cost 5% fewer Marks (requires Thrifty Quartermaster). Lucky Duplicate: +20% duplicate hero Crown refunds (requires Thrifty Quartermaster). Premium Duplicate: +30% additional duplicate refund (requires Lucky Duplicate). |
| Camp | Camp Lore: Camp Draws arrive two wins sooner. Reroll: once per Camp Draw, open two prize boxes and choose which reward to keep (requires Camp Lore). Loaded Deck: draw one extra card in Camp Draw (requires Camp Lore). The Conductor: tune one hero modifier by ±1 (requires Camp Lore). Master Conductor: tune one modifier by up to ±2 (requires The Conductor). |

Existing matching unlocks are retained during migration. Retired nodes are
removed and each grants one compensating Skill Point.

The chroma catalog reads tier, color, price, and currency from each skin
spritesheet's metadata. Tier ranks map to Common (0), Uncommon (1), Rare (2),
Epic (3), and Seasonal/Premium (4). The browser displays family tiles
alphabetically within each tier; a family with colors in multiple tiers has a
separate tile in every tier it spans. Opening any tile previews all family
colors and allows an individual purchase or a full-family purchase at 20% off
the sum of listed prices. The family discount requires a shared currency and
no already-owned colors. Partially owned families can be completed by buying
remaining colors individually at their listed prices. Browser suit
choices show only owned chromas; each can be assigned to Scissors (`B`),
Paper (`F`), or Rock (`S`) as long as no other suit uses the same color. The
selected chroma controls the deck artwork and face-number color.

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
confirmations. Profiles are schema version 13; older profile data is migrated
without losing existing settings. The featured chroma currently uses the
allowed static “none today” placeholder.

Ranked modes are local adapters around completed games; there is no server,
online matchmaking, or network friend system.

To inspect the current economy values, skill tree, and catalog as JSON:

```powershell
GOWORK=off go -C backend run ./cmd/prisoners-of-war --dump-economy
```

For development, set the profile directory through `HOME`/`USERPROFILE`, or
call `profile.LoadProfileFrom` and `Profile.SaveTo` with an explicit test path.

For the complete achievement catalog, set rewards, match history, and gallery
behavior, see [achievements.md](./achievements.md).
