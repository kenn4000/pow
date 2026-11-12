# Tutorial Campaigns

The CLI's **Campaigns** menu contains two sequential training campaigns.
Basic Training teaches the core rules with The Recruit; Advanced Training
introduces hero modifiers and three selectable heroes. The browser also
includes the Abandon, Ladder, Poker, Rummy, and Tug War campaigns. Campaigns
run through their existing game rules and keep scripted overrides in the
`campaigns` package.
The browser dashboard's **Campaigns** action and the terminal campaign menu
both show the same saved unlock and mission-completion progress.

## Campaign rewards

Every campaign uses the same reward tiers by mission index:

| Missions | First-completion reward |
|---|---|
| 1–5 | 350 Marks |
| 6–10 | 300 Marks and 1 Crown |
| 11 onward | 350 Marks and 2 Crowns |

The final tier also covers Puzzler's missions 16–20. Completing Basic Training
unlocks its completion achievement; completing Abandon, Ladder, Poker, Rummy,
or Tug War unlocks that campaign's achievement. Each grants an additional
500 Marks, 3 Crowns, and one Undo, Restart, Skip Mission, and Swap Card token.
These achievement rewards are one-time and are also granted to profiles that
had already completed the corresponding campaign.

The Shop sells Campaign Mission Skip tokens for 1,500 Marks or 75 Crowns.
Using one on the next playable mission advances progression without mission
Marks or completion rewards. Skipping the final mission completes the campaign
without mission or standard completion rewards, but still grants the campaign
completion achievement. Completed missions cannot be skipped.

The in-game token bar shows Restart Mission, Undo Last Move, Replace Any Card,
and (during campaigns) Campaign Mission Skip inventories. When an inventory is
empty, its button offers Crown-only quick purchases; Marks purchases are made
in the Shop. These tokens are available only in campaigns and vs-CPU games,
never in Human-vs-Human play. Replace Any Card discards a selected available
card and draws a random replacement from the remaining deck in Main, Fog,
Puzzler, Abandon, Rummy, Poker, Ladder, and Tug War, including CPU variants.
Restart returns to the mission's original state without charging Energy again;
Undo restores the state before the last move and any automatic CPU response.

## Basic Training

**ID:** `basic`  
**Prerequisite:** None  
**Description:** Learn the rules of prison-duel combat.

The player uses The Recruit in all ten missions. Missions 1-4 are scripted
single-round tutorials; missions 5-10 are three-round matches.

| # | Mission | Narrative and hint | Setup |
|---|---|---|---|
| 1 | First Blood | Kess teaches the initial deal, three-card commit, exchanges, and capture scoring. Commit three cards; higher faces win. | Player hand: R5, R6, R7, R8, R9, R1. CPU pool and leads: R2, R3, R4. Bounty 0; weights 1/1/1. |
| 2 | The Clash | Equal cards clash and both are destroyed. The hint recommends playing 3, then 5, then 7. | Player hand: R3, R5, R7. CPU leads R9, mirror, R6 and attacks all three exchanges. The middle exchange is a tutorial-only mirror clash. Bounty 0; weights 1/1/1. |
| 3 | The Cycle | Kess explains Rock beats Scissors, Scissors beats Paper, and Paper beats Rock. | Player hand: R5, P5, S5. CPU pool and leads: S9, R9, P9. The player can win 27-0 by playing R5, P5, S5. Bounty 0; weights 1/1/1. |
| 4 | The Bounty | Rock prisoners are worth more. The hint recommends Paper 4 against Rock 7, Rock 4 against Scissors 8, then Scissors 7 against Rock 8. | Player hand: P4, R4, S7. CPU leads R7, S8, R8 and attacks all three exchanges. Rock bounty +2; weights 1/1/1. Expected score: 9-7. |
| 5 | Standard | A straight match with no hero or scoring modifiers. | Three rounds; both players start with five cards; Bounty, Shift, and Favored are 0; weights 1/1/1. |
| 6 | The Reckoning | Round 3 counts triple, so the player is encouraged to save strong cards. | Three rounds; weights 1/2/3; other modifiers are 0. |
| 7 | Negative Bounty | Rock cards are worth less, so players are encouraged to consider when not to contest a capture. | Three rounds; Rock bounty -2; other modifiers are 0; weights 1/1/1. |
| 8 | Favored | Cycle wins receive an additional +2 to their doubled combat value. | Three rounds; Favored +2; other modifiers are 0; weights 1/1/1. |
| 9 | Shift | Every card receives +2 effective face value. | Three rounds; Shift +2; other modifiers are 0; weights 1/1/1. |
| 10 | Final Exam | Kess combines the modifiers and challenges the Recruit to become a Warden. | Three rounds; Rock bounty +2, Shift +1, Favored -1, weights 1/1/2. |
Basic Training missions 1–5 award 350 Marks each; missions 6–10 award 300
Marks and 1 Crown each. Completing Basic Training awards 1,000 additional
Marks, unlocks Warden, Hunter, and the Turtle, grants `badge_basic_training`,
and unlocks Advanced Training. Its completion achievement adds another 500
Marks, 3 Crowns, and the four campaign consumables.

## Advanced Training

**ID:** `advanced`  
**Prerequisite:** Complete Basic Training  
**Description:** Learn to fight with heroes. Every hero changes the game.

Every mission is a three-round match. Before each mission, choose Warden,
Hunter, or the Turtle. The opponent's hero and AI level are fixed.

| # | Mission | Opponent | Narrative and hint |
|---|---|---|---|
| 1 | The Hunter's Game | Hunter, Greedy AI | Hunter adds +2 Bounty. Kess suggests Warden as a safe choice. |
| 2 | The Treasurer | Treasurer, Greedy AI | Treasurer subtracts 2 Bounty. Kess explains how hero bounty modifiers combine. |
| 3 | The Gorilla | Gorilla, Greedy AI | The Gorilla adds +2 Shift, strengthening every card and its cycle wins. |
| 4 | The Gambler | Gambler, Greedy AI | Gambler adds +2 Bounty and +1 Shift. Kess suggests balancing or leaning into Bounty. |
| 5 | The Hater | Hater, Greedy AI | Hater flips the opponent's Bounty contribution. Kess explains the risks for positive and negative Bounty heroes. |
| 6 | No Draft Duel | Recruit, Easy AI | Skip the draft and defend the CPU's opening attack. No draft (mode 0); both players start with five cards; CPU attacks first in Round 1. |
| 7 | Unlucky | Recruit, Easy AI | The CPU receives all three nines before the draft begins. Draft mode 2; three hole cards each; six public cards. |
| 8 | Harsh | Recruit, Easy AI | CPU drafts first, but you attack first in Round 1. Draft mode 4; one hole card each; ten public cards. |
| 9 | Blind Aggression | Recruit, Easy AI | Take the first draft pick and open every round's attack. Draft mode 3; two hole cards each; eight public cards. |
| 10 | Blind Aggression 2 | Recruit, Easy AI | CPU drafts first, but you attack first at the start of every round. Draft mode 3; two hole cards each; eight public cards. |

Advanced Training missions 1–5 award 350 Marks each; missions 6–10 award 300
Marks and 1 Crown each. Completing Advanced Training awards 2,000 additional
Marks, one Hero Box, one Skill Tree Point, and `badge_advanced_training`.

## Abandon Campaign

**ID:** `abandon`
**Missions:** 15

Missions 1–8 are fixed-board solo tutorials for matchup resolution, suit ties,
prisoner scoring, bounty, abandon penalties, flips, and hidden-card gambles.
Missions 9–15 use the existing Abandon Duel layout and engine, progressing
through alternating turns, snake order, seat-swapped rounds, and increasingly
strategic CPU policies. Mission 3 makes the equal-value tie available with
Rock 2 against Scissors 4; the tie discards both cards.

The campaign's setup and objectives are isolated from standalone Abandon
games. The supplied Mission 3 prose contains conflicting earlier examples;
the playable setup uses the final tie example in the brief.

## Ladder Campaign

**ID:** `ladder`
**Missions:** 15

The campaign uses the existing Ladder rules and layout. Its scenarios teach
legal climbing, suit doubling, wilds, Favor, Shift, Draft, initiative, passing,
and denial before moving into asymmetric and open-hand challenges. Missions
14 and 15 use full-information minimax; the Gauntlet reveals its randomly
selected CPU hero before the player selects a hero. Campaign setups and CPU
policies do not change standalone Ladder games.

## Rummy Campaign

**ID:** `rummy`
**Missions:** 15

Missions progress from scripted runs and sets through discards, turn limits,
hero modifiers, larger hands, opponent pressure, and a two-hand final match.
Cards use the existing three-suit Rock/Paper/Scissors deck so campaign hands
match the multiplayer Rummy engine. Treasurer stands in for the brief's
unavailable Vulture hero (Favor +1, Bounty −2). Long-game stock recycling is
campaign-only; standalone multiplayer Rummy retains its existing draw behavior.

## Tug War Campaign

**ID:** `tug-war`
**Missions:** 15

The campaign reuses the Tug War board, card, push, flip, and scoring rules.
Its missions teach basic hand play, pushes, bounty denial, board control, fog
of war, hero modifiers, constrained pushes, and strategic CPU policies before
three series missions. Scripted presets use distinct physical cards across
hands, piles, and decks. The campaign-specific setup and AI do not alter
standalone multiplayer matches.

Each of these five 15-mission campaigns awards 350 Marks for missions 1–5,
300 Marks and 1 Crown for missions 6–10, then 350 Marks and 2 Crowns for
missions 11–15. Its completion achievement grants the additional rewards
listed above.

## Poker Campaign

**ID:** `poker`
**Prerequisite:** None
**Description:** Learn poker scoring and strategy in 15 single-player missions.

The campaign is player-versus-par, not a conventional CPU match. Missions 6
and 7 add visible, deterministic teaching ghosts; both player and ghost build
independent tables, and the player must meet par and outscore the ghost. Ghosts
use the selected hero's modifiers, draw from a stable deck order, and never
redraw. All other missions are score-vs-par only. Mission lessons appear at
the start of play, and suggested heroes are recommendations rather than
restrictions. Mission 15 intentionally has no lesson or hero suggestions.

Suit names are Rock, Paper, and Scissors. A mission may force the bounty suit
or select it randomly at game start. Missions 1-9 have scripted six-card
opening hands; later missions use random hands. Draft sets the minimum value
in the remaining deck to `2 - Draft`, while the scripted opening hand is
preserved exactly.

The player receives six cards. Each turn, place one legal card into one of
three five-card hands or discard and redraw it. Hand sizes must remain within
one card of each other; after an action, the hand refills to six while cards
remain in the draw pile. The game ends when all hands are full, there is no
legal placement after the draw pile empties, or 30 turns have passed. A hand
with fewer than five cards is void and scores zero. In Silent Nines, 9s cannot
be placed and may instead be redrawn or held.

| # | Mission | Setup or lesson | Constraint / opponent | Par |
|---|---|---|---|---:|
| 1 | The First Hand | Rock bounty; pair of 9s | — | 160 |
| 2 | Finding the Nine | Paper bounty; one 9 | — | 175 |
| 3 | Your Suit | Scissors bounty; three Scissors | — | 185 |
| 4 | The Even Table | Rock bounty; balanced opening hand | — | 185 |
| 5 | The Bonus Table | Rock bounty; 9 and pair of 3s | — | 195 |
| 6 | Chase the Scissors | Scissors bounty; 9-focused counter | Bounty ghost; beat its score | 200 |
| 7 | Chase the Paper | Paper bounty; Paper-focused counter | 9 ghost; beat its score | 200 |
| 8 | Silent Nines | Rock bounty | No 9s may be placed | 175 |
| 9 | The Wall | Paper bounty | Straights and flushes score zero base | 195 |
| 10 | Free Play | Random bounty and hand | — | 205 |
| 11 | High Bar | Rock bounty; random hand | — | 210 |
| 12 | Consistency | Random bounty and hand | Three consecutive games at 175+ | 175 |
| 13 | Doubled Bounty | Nine base 8; bounty base 3; Scissors bounty | — | 210 |
| 14 | Nine Storm | Nine base 16; Paper bounty | — | 215 |
| 15 | The Crown | Default scoring; random bounty and hand | No hints or recommendations | 235 |

Hand base scores are: straight flush 100, three of a kind 60, straight 50,
flush 40, a pair 20, and high card 0. Unlisted pair patterns use the listed
pair or three-of-a-kind category; four of a kind and full house have no
separate category. A hand containing a 9 adds `nine_base + 2 × Shift`. A hand
with at least `2 - Favor` bounty-suit cards adds
`max(0, bounty_base + Bounty)`. A zero-base completed hand loses 10 points.
If all three hands have a base score of at least 20, add a 15-point table
bonus. Mission constraints are checked before scoring; a failed constraint
fails the attempt without reporting a score.

When all three hands are complete, add 15 points if Hand A's score is strictly
higher than both other hands, and add 5 points if Hand C's score is strictly
lower than both other hands. Neither bonus applies if any hand is void.

The ghosts in Mission 6 and 7 prioritize bounty-suit cards or 9s, respectively,
and place those cards in Hand A whenever the distribution rule permits. If no
priority card is available, they place the highest face card; replacement
draws are deterministic and visible through their three hands.

Mission 12 repeats games within one attempt. A score of at least 175 advances
the streak; a lower score resets it to zero. Switching heroes also resets the
streak. The bounty suit is rerolled for each game. Poker uses the shared
campaign reward tiers above; practice replays do not award progression rewards.

## Mission behavior

- `SingleRound` missions deal one three-exchange round. `ThreeRound` missions
  use the normal three-round deal, refill, seat-change, and scoring rules unless
  a mission explicitly sets an opening draft or attack order.
- A `DraftMode` of 0 skips the draft and deals five cards to each player.
  Modes 1-5 use the normal opening draft with the mission's configured first
  picker. Fixed CPU hole cards are removed from the bounty and public pool.
- Missions with a forced player-first attack order start each round with the
  player attacking, followed by the CPU, then the player again; normal seat
  rotation and refills are otherwise preserved.
- Fixed player hands, CPU pools, scripted CPU leads, and modifier overrides
  are declared on each `Mission`. A nil modifier override leaves the
  hero-combined or normally rolled value in place.
- CPU scripted leads are used only when the CPU is the attacker. The CPU uses
  its configured AI for defense. In the all-attacks tutorials, the CPU leads
  each exchange.
- `AIEasy` chooses random or greedy play with 70%/30% weights. `AINormal` uses
  random-ish attacks and greedy defense. `AIGreedy` uses greedy play for both.
- The tutorial mirror consumes a real CPU pool card but displays a virtual
  card matching the player's selection. The engine discards both physical
  cards without creating a duplicate in the game state.
- A mission is won only if the player's score is strictly higher. Ties require
  a retry. A loss permits an immediate retry; completed missions can be
  replayed for practice without additional Marks or progression rewards.
- Missions unlock in order. Advanced Training remains locked until Basic
  Training is complete. Campaign completion rewards and achievement events
  are applied by the existing profile and achievement flows.
- Completing Advanced Training grants one Skill Tree Point. Players who had
  already completed its original five missions can play the five additions;
  their earlier completion rewards are not granted again.

## Adding a campaign

1. Define the campaign and its missions in a campaign-specific file such as
   `backend/internal/campaigns/new_campaign.go`.
2. Set each mission's sequential `Index`, type, briefing and hint, AI level,
   allowed hero IDs, and the shared mission reward tier.
3. For scripted missions, ensure each physical card appears only once across
   the player hand, CPU pool, bounty card, and remaining deck. Scripted CPU
   leads should be members of the CPU pool; the mirror exchange is the sole
   virtual exception.
4. Set `Requires` to the preceding campaign ID, or leave it empty for an
   introductory campaign. Add completion rewards to the campaign definition.
5. Add mission-runner and progression tests, update this document, then run
   `GOWORK=off go -C backend test ./...` and `GOWORK=off go -C backend build ./...` from the repository root.
