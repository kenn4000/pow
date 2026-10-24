# Tutorial Campaigns

The CLI's **Campaigns** menu contains two sequential training campaigns.
Basic Training teaches the core rules with The Recruit; Advanced Training
introduces hero modifiers and three selectable heroes. Both campaigns run
through the same public game rules and keep their scripted card overrides in
the `campaigns` package.
The browser dashboard's **Campaigns** action shows the same saved unlock and
mission-completion progress; campaign missions are still played from the
terminal campaign menu.

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
| 5 | Standard | A straight match with no hero or scoring modifiers. | Three rounds; Bounty, Shift, and Favored are 0; weights 1/1/1. |
| 6 | The Reckoning | Round 3 counts triple, so the player is encouraged to save strong cards. | Three rounds; weights 1/2/3; other modifiers are 0. |
| 7 | Negative Bounty | Rock cards are worth less, so players are encouraged to consider when not to contest a capture. | Three rounds; Rock bounty -2; other modifiers are 0; weights 1/1/1. |
| 8 | Favored | Cycle wins receive an additional +2 to their doubled combat value. | Three rounds; Favored +2; other modifiers are 0; weights 1/1/1. |
| 9 | Shift | Every card receives +2 effective face value. | Three rounds; Shift +2; other modifiers are 0; weights 1/1/1. |
| 10 | Final Exam | Kess combines the modifiers and challenges the Recruit to become a Warden. | Three rounds; Rock bounty +2, Shift +1, Favored -1, weights 1/1/2. |

Each mission awards 100 Marks. Completing Basic Training awards 1,000
additional Marks, unlocks Warden, Hunter, and the Turtle, grants
`badge_basic_training`, and unlocks Advanced Training. Total campaign Marks:
2,000.

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

Each mission awards 400 Marks. Completing Advanced Training awards 2,000
additional Marks, one Hero Box, and `badge_advanced_training`. Total campaign
Marks: 4,000.

## Mission behavior

- `SingleRound` missions deal one three-exchange round. `ThreeRound` missions
  use the normal three-round deal, refill, seat-change, and scoring rules.
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

## Adding a campaign

1. Define the campaign and its missions in a campaign-specific file such as
   `backend/internal/campaigns/new_campaign.go`.
2. Set each mission's sequential `Index`, type, briefing and hint, AI level,
   allowed hero IDs, and mission Marks.
3. For scripted missions, ensure each physical card appears only once across
   the player hand, CPU pool, bounty card, and remaining deck. Scripted CPU
   leads should be members of the CPU pool; the mirror exchange is the sole
   virtual exception.
4. Set `Requires` to the preceding campaign ID, or leave it empty for an
   introductory campaign. Add completion rewards to the campaign definition.
5. Add mission-runner and progression tests, update this document, then run
   `GOWORK=off go -C backend test ./...` and `GOWORK=off go -C backend build ./...` from the repository root.
