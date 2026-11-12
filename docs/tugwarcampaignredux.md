```markdown
# Tug War — Campaign Mission Specifications (v2)

Format per mission:
- **Setup:** player hand, CPU hand, board, deck, bounty suit, pushes, win condition, deck behavior
- **CPU hero:** fixed hero the CPU plays (determines CPU's contribution to s/f/b)
- **Expected stat range:** the s/f/b values that result from CPU hero + typical player hero choices
- **Recommended hero:** the hero the mission is designed around (player may still pick any hero)
- **CPU Algorithm:** the CPU's policy for this mission
- **Intent:** the lesson the mission teaches

Card notation: `SuitFace` — e.g., `R7` = Rock 7, `S3` = Scissors 3, `P9` = Paper 9. Suits: R beats S, S beats P, P beats R.

Stats reminder: `s = min(playerShift + cpuShift, 3)`, `f = playerFavor + cpuFavor`, `b = clamp(playerBounty + cpuBounty, -1, 3)`, pushes set explicitly by mission.

---

## Act 1 — Teach the verbs

### M1 — First Steps
**Setup**
- Player hand: `R2, R3, S5, P6, S8, R9`
- CPU hand: `P1, S2, R4, P5, S7, P8`
- Board (3 piles, all neutral):
  - Pile A: top `P1`
  - Pile B: top `R2`
  - Pile C: top `S3`
- Deck: empty (no flips possible)
- Bounty suit: **S** (fixed)
- Pushes: Player 2, CPU 0
- Win: Player score > 0
- Deck behavior: no draws, no refills

**CPU hero:** Recruit
**Expected stat range:** s ∈ [0,2], f ∈ [0,1], b ∈ [−1,1]
**Recommended hero:** Recruit

**CPU Algorithm**
- No pushes. No flips. Only hand plays.
- Play the lowest face card that beats any pile top. Tie: prefer tallest pile. Tie: furthest pile.
- If no legal play: pass.

**Intent:** Player learns the verbs. CPU is a punching bag so the player can experiment.

---

### M2 — Push It
**Setup**
- Player hand: `R3, R4, R5, P2, S2, P4`
- CPU hand: `S8, P8, R8, S6, P6, R6`
- Board (all neutral):
  - Pile A: top `R8`
  - Pile B: top `S9`
  - Pile C: top `P7`
- Deck (fixed order): `R1, S1, P1, R2, S2`
- Bounty suit: **P** (fixed)
- Pushes: Player 3, CPU 0
- Win: Player has all 3 piles in their zone within 3 rounds
- Deck behavior: no refills; deck is only for CPU flips and player flips

**CPU hero:** Recruit
**Expected stat range:** s ∈ [0,2], f ∈ [0,1], b ∈ [−1,1]
**Recommended hero:** Recruit

**CPU Algorithm**
- No pushes. Flips whenever it can't beat a pile by hand.
- Flip target: tallest pile.
- Will flip through `R1, S1, P1` in order — all fail, become new tops.

**Intent:** Player cannot win by hand (their hand is all low vs. high tops). Only path is 3 pushes. The puzzle is realizing "I have a tool the CPU doesn't, and it's the only tool that works."

---

### M3 — Denial
**Setup**
- Player hand: `S4, S5, S7, R3, P2, R6` (3 bounty-suit S cards, 3 non-bounty)
- CPU hand: `S2, S3, S6, S9, R4, P5` (4 bounty-suit S cards)
- Board (all neutral):
  - Pile A: top `R5` (CPU will feed this pile)
  - Pile B: top `P3`
  - Pile C: top `S8`
- Deck (fixed order): `R7, P6, S1, R8, P4, S1`
- Bounty suit: **S** (fixed)
- Pushes: Player 2, CPU 2
- Win: Player score > 0 AND at least one pile in player's zone
- Deck behavior: standard (refill to 6 each round while deck allows)

**CPU hero:** Turtle
**Expected stat range:** s ∈ [0,1], f ∈ [−2,1], b ∈ [−3,2] → banded b ∈ [−1,2]
**Recommended hero:** Vulture or Turtle (any bounty-negative hero makes the poison stronger; any bounty-positive hero makes it neutral and the mission easier)

**CPU Algorithm**
- Bounty-averse policy:
  - Hand play: prefers playing a bounty-suit card onto a pile it does NOT control (neutral or opponent zone). Specifically: if a bounty-suit card can be played onto a pile such that the pile would move toward the player, do that.
  - If no such play: play the highest-value non-bounty card onto a pile it controls (protect it).
  - Push: if a pile with ≥2 bounty-suit cards is neutral or in CPU zone, push it toward the player. Otherwise save pushes.
  - Flip: only if no legal hand play.
- Goal: hand the player the poison pile.

**Intent:** Player learns bounty cards can be liabilities. Winning the poison pile loses the game. The tell is CPU keeps playing S cards onto the same pile.

---

## Act 2 — Teach the board

### M4 — The High Ground
**Setup**
- Player hand: `R6, R7, S5, P8, S9, P3`
- CPU hand: `P6, P5, R4, S4, P2, R3`
- Board:
  - Pile A: 4 cards in CPU's zone, top `P8`, stack `R5, S6, P4, P8`
  - Pile B: neutral, top `S3`
  - Pile C: neutral, top `R6`
- Deck (fixed order): `R9, S7, P7, S8, R8, P1, S1, R1`
- Bounty suit: **R** (fixed)
- Pushes: Player 3, CPU 0
- Win: Pile A is in player's zone within 4 rounds (piles B and C are decoys)
- Deck behavior: refill to 6 while deck allows

**CPU hero:** Hunter
**Expected stat range:** s ∈ [0,2], f ∈ [0,1], b ∈ [1,3] → banded b ∈ [1,3]
**Recommended hero:** Bodybuilder or Hunter (bounty heroes — the bounty pile is valuable, so b > 1 makes the recovery worth more points)

**CPU Algorithm**
- No pushes (CPU has the strong board, weak tools).
- Hand play: lowest winning card onto pile A if possible. Otherwise to pile B or C (distraction).
- Flip: only if no legal play, targets tallest pile.
- CPU cannot yank pile A back (no pushes). Its only defense is adding cards to pile A to make it taller.

**Intent:** Player has 3 pushes — the only way to recover pile A. Puzzle is spending all 3 pushes on pile A while using hand plays on B/C to stall. Sequencing the pushes is the solution.

---

### M5 — The Siege
**Setup**
- Player hand: `R4, S7, P5, R8, S3, P9`
- CPU hand: `P7, R6, S8, P4, R2, S5`
- Board (all neutral):
  - Pile A: top `R8`
  - Pile B: top `S5`
  - Pile C: top `P2`
- Deck: empty
- Bounty suit: **S** (fixed)
- Pushes: Player 2, CPU 2
- Win: Player has all 3 piles in their zone at end of round 4
- Deck behavior: no draws, no refills

**CPU hero:** Gorilla
**Expected stat range:** s ∈ [2,3] (Gorilla 2 + player shift; caps at 3), f ∈ [−1,1], b ∈ [−1,3]
**Recommended hero:** Recruit or Warden (shift-0 heroes — the mission is designed for "you're playing against a high-shift opponent and must use pushes to compensate")

**CPU Algorithm**
- Spreader policy:
  - Each turn, plays the lowest winning card onto a pile it doesn't control.
  - Pushes: if it can lock a pile into its zone with a push, does so.
  - Tie-break: prefer pile with fewest CPU cards.
- The CPU will try to lock piles B and C by round 3.

**Intent:** Player must race. They cannot win all 3 piles by hand — they need pushes on A and B, and card plays on C. The puzzle is "which 2 piles can I afford to lose?"

---

### M6 — Fog of War
**Setup**
- Player hand: `R5, S4, P6, R7, S8, P3`
- CPU hand: `P5, R4, S7, P2, R6, S3`
- Board (all neutral, tops face-down):
  - Pile A: top hidden (revealed as `R7`)
  - Pile B: top hidden (revealed as `S4`)
  - Pile C: top hidden (revealed as `P9`)
- Deck (fixed order): `S2, P7, R3, S5, P1, R8, S6, P4`
- Bounty suit: **P** (hidden until first flip)
- Pushes: Player 2, CPU 2
- Win: Player score > CPU score
- Deck behavior: standard refill

**CPU hero:** Vulture
**Expected stat range:** s ∈ [0,1], f ∈ [0,2], b ∈ [−3,1] → banded b ∈ [−1,1]
**Recommended hero:** Any (the mission teaches hidden information — hero choice doesn't trivialize it, but bounty-negative heroes make the bounty pile a potential trap)

**CPU Algorithm**
- Info-gatherer policy:
  - Rounds 1–2: flip aggressively (even if a hand play exists) to reveal pile tops. Prefer flipping the pile with the highest stack.
  - Once all 3 pile tops are revealed: switch to standard heuristic.
- After reveal: standard hand-play and push heuristics.

**Intent:** Player must flip to learn the board. CPU shows the value of scouting by flipping first. Player learns "flips are intel, not just board control."

---

## Act 3 — Teach the stats

### M7 — The Favor Economy
**Setup**
- Player hand: `R2, R3, S9, P9, S4, P2`
- CPU hand: `R5, R6, R7, R4, R8, R9`
- Board (all neutral):
  - Pile A: top `R8`
  - Pile B: top `S7`
  - Pile C: top `P6`
- Deck: empty
- Bounty suit: **R** (fixed)
- Pushes: Player 2, CPU 2
- Win: Player score > CPU score
- Deck behavior: no draws, no refills

**CPU hero:** Warden
**Expected stat range:** s ∈ [0,1], f ∈ [1,3], b ∈ [0,3]
**Recommended hero:** Warden, Vulture, or Diplomat (favor-heavy heroes — mission is designed for f ≥ 3; low-favor heroes will find this very hard)

**CPU Algorithm**
- Dominant-suit hoarder:
  - Always plays dominant-suit cards (R). Prefers to lock piles it can beat by playing R cards.
  - Non-dominant cards are never played (they sit in hand).
  - Pushes: only on piles it's about to lock.
- The CPU's R cards will dominate every pile.

**Intent:** Player learns "in a high-favor world, only one suit matters." The puzzle is finding the dominant cards and using them efficiently. Non-dominant cards are dead weight. CPU's R9 cannot be beaten by any card play except another R (impossible) — the player must push, not play.

---

### M8 — Overdrive
**Setup**
- Player hand: `S2, P3, R4, S3, P2, R3`
- CPU hand: `R7, R8, S8, S9, R9, S7`
- Board (all neutral):
  - Pile A: top `R7`
  - Pile B: top `S9`
  - Pile C: top `P9`
- Deck: `R1, S1, P1`
- Bounty suit: **S** (fixed)
- Pushes: Player 1, CPU 3
- Win: Player has at least 1 pile in their zone at end of round 4 (survival)
- Deck behavior: no refills; 3 flips available

**CPU hero:** Gorilla
**Expected stat range:** s ∈ [2,3], f ∈ [−1,1], b ∈ [−1,3]
**Recommended hero:** Turtle or Recruit (low-shift heroes — the mission is designed for "you're out-gunned")

**CPU Algorithm**
- Blitz policy:
  - Spends every push immediately on the highest-value pile (highest stack, tie: furthest from CPU).
  - Plays highest card to lock piles. Does not preserve cards.
  - Goal: lock 2 piles by end of round 3.

**Intent:** Player learns "sometimes you don't win — you survive." Hand is nearly unplayable, 1 push. Goal is to have one pile at end of round 4. Puzzle is picking the pile the CPU is least likely to contest.

---

### M9 — The Bodybuilder's Game
**Setup**
- Player hand: `R5, S6, S7, P4, R8, P7`
- CPU hand: `S3, S4, S5, S8, S9, R2`
- Board (all neutral):
  - Pile A: top `R6` (CPU ignores)
  - Pile B: top `P5` (bounty pile — CPU will feed S cards)
  - Pile C: top `S3` (CPU ignores)
- Deck: `R1, P1, S1`
- Bounty suit: **S** (fixed)
- Pushes: Player 3, CPU 3
- Win: Player wins more piles than CPU (2 of 3)
- Deck behavior: no refills; 3 flips available

**CPU hero:** Bodybuilder
**Expected stat range:** s ∈ [0,2], f ∈ [−1,1], b ∈ [2,3] → banded b = 3 (CPU already maxes b)
**Recommended hero:** Bodybuilder or Hunter (bounty heroes — but note CPU already maxes b, so any hero works; the puzzle is about pile selection, not stat choice)

**CPU Algorithm**
- Tunnel-vision policy:
  - Only plays cards to pile B. Never plays to pile A or C.
  - Pushes only pile B. Flips only onto pile B.
  - All resources committed to the bounty pile.

**Intent:** Player learns "don't fight where the CPU is strongest." CPU will win pile B every game. Player must win piles A and C. This mission teaches selective engagement — winning the game, not the pile.

---

## Act 4 — Teach the constraints

### M10 — No Pushes
**Setup**
- Player hand: `R4, S6, P5, R7, S3, P8`
- CPU hand: `P6, R5, S7, P4, R6, S4`
- Board (all neutral):
  - Pile A: top `R5`
  - Pile B: top `S5`
  - Pile C: top `P5`
- Deck: empty
- Bounty suit: **R** (fixed)
- Pushes: Player 0, CPU 0
- Win: Player score > CPU score
- Deck behavior: no draws, no refills

**CPU hero:** Hater
**Expected stat range:** s ∈ [1,3], f ∈ [−1,2], b ∈ [0,3]
**Recommended hero:** Any (mission is about pure card play — hero choice affects the exact stats but not the puzzle structure)

**CPU Algorithm**
- Efficient policy: standard heuristic, no pushes.
- Plays the lowest winning card. Tie: tallest pile. Tie: furthest.
- No flips (deck empty).

**Intent:** Player learns pushes are optional. Pure card play. CPU's hand is slightly stronger — player must use favor doubling to win by suit choice, not card quality.

---

### M11 — One-Way Pushes
**Setup**
- Player hand: `R3, R7, S6, P8, S8, P4`
- CPU hand: `S4, P5, R6, S5, P3, R8`
- Board:
  - Pile A: in CPU's zone, top `R4`, stack `S2, P3, R4`
  - Pile B: neutral, top `S6`
  - Pile C: neutral, top `P5`
- Deck: `R1, S1, P1, R2`
- Bounty suit: **P** (fixed)
- Pushes: Player 2 (one-way — can only move piles toward player, cannot yank from CPU zone), CPU 3 (normal)
- Win: Player has 2 of 3 piles in their zone at end
- Deck behavior: no refills; 4 flips available

**CPU hero:** Diplomat
**Expected stat range:** s ∈ [0,1], f ∈ [1,3], b ∈ [−3,1] → banded b ∈ [−1,1]
**Recommended hero:** Diplomat or Warden (favor heroes — the mission is about exploiting suit dominance, and favor helps)

**CPU Algorithm**
- Exploitative policy:
  - If player has a pile in their zone: use push to yank it back to neutral (or into CPU zone if possible).
  - Otherwise: standard heuristic — play to lock CPU piles.
  - Pushes prioritized on yanking the player's piles.
- The CPU plays reactively.

**Intent:** Player learns asymmetric rules. They cannot claw back pile A — they must win piles B and C. Puzzle: "which two piles can I win if I can't undo the CPU's yanks?"

---

### M12 — The Mirror
**Setup**
- Both players pick the same hero (player picks, CPU uses same).
- Player hand: random fair deal
- CPU hand: random fair deal
- Board: standard 3 neutral piles, random tops
- Deck: standard shuffle
- Bounty suit: random (standard flip)
- Pushes: standard from draft
- Win: Best-of-3 games, first to 2 wins
- Deck behavior: standard

**CPU hero:** Same as player (forced)
**Expected stat range:** standard derivation (both players have identical stats)
**Recommended hero:** Any (both players play the same hero)

**CPU Algorithm**
- Adaptive policy:
  - Standard heuristic for rounds 1–2.
  - From round 3 onward: estimate player's aggression by counting pushes spent and flips taken. If player is aggressive (>50% turns spent a push or flip), CPU switches to defensive policy (defensive bias 0.9, aggression 0.3, prioritize yanking). If player is passive, CPU goes aggressive (aggression 0.9, defensive bias 0.3).
  - CPU "reads" the player and counters.

**Intent:** Player learns the CPU is playing them. Puzzle is hiding your strategy — playing a mix that doesn't reveal whether you're aggressive or passive.

---

### M13 — The Gambler's Gambit
**Setup**
- Player hand: `R7, S9, P5, R4, S3, P8`
- CPU hand: `P9, R8, S5, P4, R6, S7`
- Board:
  - Pile A: in CPU's zone, top `P6`, stack `R5, S4, P6`
  - Pile B: neutral, top `S8`
  - Pile C: neutral, top `R3`
- Deck: `S1, R1, P1, S2, R2`
- Bounty suit: **R** (fixed)
- Pushes: Player 2, CPU 2
- Win: Player score > (CPU score + 5) — margin win
- Deck behavior: no refills; 5 flips available

**CPU hero:** Gambler
**Expected stat range:** s ∈ [1,3], f ∈ [−1,2], b ∈ [1,3] → banded b ∈ [1,3]
**Recommended hero:** Bodybuilder or Hater (high-bounty heroes — the margin win is easier with a hero that lets you score big)

**CPU Algorithm**
- Bluffer policy:
  - 30% of turns: play a suboptimal card onto pile B or C (bait — makes player think CPU is committed there).
  - 70% of turns: standard heuristic, but prioritizes locking pile A (own pile) and pile C (the one it's baiting toward).
  - Pushes: 50% on pile A (protect), 50% on pile C (bait).
  - The CPU deliberately lets the player overextend on pile B, then pivots to lock A and C.

**Intent:** Player learns the CPU can bluff. Pile B looks contestable but is a trap — winning it costs cards needed for A and C. The margin win (+5) means the player must win decisively, not just win.

---

## Act 5 — Finale

### M14 — The Gauntlet
**Setup**
- Player keeps same hero across 3 games.
- Each game uses standard hands, standard deck, standard stats (fair play).
- Three CPU opponents in sequence:
  - Game 1: Recruit with M1's passive policy
  - Game 2: Warden with M2's patient policy
  - Game 3: Gorilla with M5's spreader policy
- Pushes: standard from draft in each game.
- Win: player wins 2 of 3 games.
- Deck behavior: standard per game.

**CPU heroes:** Recruit (game 1), Warden (game 2), Gorilla (game 3)
**Expected stat range:** standard derivation per game
**Recommended hero:** Any (this is the test of hero flexibility — pick a hero that works against three different stat ranges)

**CPU Algorithm**
- Each game uses its hero's associated mission policy (M1 passive, M2 patient, M5 spreader).
- No custom stacking or overrides — fair play across all three.

**Intent:** Player demonstrates adaptability. Same hero must work against 3 CPU personalities. Tests everything learned under fair-play rules.

---

### M15 — Tug of War (Boss)
**Setup**
- Player hand: `R8, S7, P6, R5, S6, P8`
- CPU hand: `S9, S8, R9, P7, S5, R7`
- Board (all neutral):
  - Pile A: top `R7`
  - Pile B: top `P8`
  - Pile C: top `S6`
- Deck: `R1, S1, P1, R2, S2, P2, R3, S3`
- Bounty suit: **S** (fixed)
- Pushes: Player 2, CPU 3 (boss bonus)
- Win: Player score > CPU score
- Deck behavior: standard refill to 6 while deck allows

**CPU hero:** Hater
**Expected stat range:** s ∈ [1,3], f ∈ [−1,2], b ∈ [0,3]
**Recommended hero:** Any (but note CPU has +1 push — pick a hero with high push count or strong stats to compensate)

**CPU Algorithm**
- Everything policy:
  - Aggression 0.9: spends pushes eagerly (low delta threshold).
  - Defensive bias 0.7: yanks player piles to neutral when possible.
  - End-game awareness 1.0: in last 2 rounds, prioritizes locking piles (never pulls own piles out of zone).
  - Suit preference: dominant. Prefers dominant-suit cards.
  - Push priority: 60% lock own pile, 40% yank player pile.
  - Flip priority: only when no legal hand play.
  - Bait: 10% chance of suboptimal play (less than M13).

**Intent:** Final exam. Every lesson: aggression, defense, stat exploitation, end-game timing, recognizing bait. CPU has +1 push and a policy that uses it well. Player must play their best game.

---

## Mission Summary Table

| Mission | Genre | Player Pushes | CPU Pushes | CPU Hero | Recommended Player Hero | Win Condition |
|---|---|---|---|---|---|---|
| M1 | Hand puzzle | 2 | 0 | Recruit | Recruit | Score > 0 |
| M2 | Hand puzzle | 3 | 0 | Recruit | Recruit | All 3 piles within 3 rounds |
| M3 | Scripted style | 2 | 2 | Turtle | Vulture/Turtle | Score > 0 + 1 pile |
| M4 | Comeback | 3 | 0 | Hunter | Bodybuilder/Hunter | Pile A back within 4 rounds |
| M5 | Puzzle-lock | 2 | 2 | Gorilla | Recruit/Warden | All 3 piles by round 4 |
| M6 | Hidden info | 2 | 2 | Vulture | Any | Score > CPU |
| M7 | Scripted style | 2 | 2 | Warden | Warden/Vulture/Diplomat | Score > CPU |
| M8 | Survival | 1 | 3 | Gorilla | Turtle/Recruit | 1 pile at end of round 4 |
| M9 | Scripted style | 3 | 3 | Bodybuilder | Bodybuilder/Hunter | 2 of 3 piles |
| M10 | Fair (no pushes) | 0 | 0 | Hater | Any | Score > CPU |
| M11 | Asymmetric | 2 (one-way) | 3 | Diplomat | Diplomat/Warden | 2 of 3 piles |
| M12 | Adaptive CPU | standard | standard | = player | Any | Best of 3 |
| M13 | Puzzle-lock | 2 | 2 | Gambler | Bodybuilder/Hater | Score > CPU + 5 |
| M14 | Gauntlet | standard | standard | Recruit/Warden/Gorilla | Any | Win 2 of 3 |
| M15 | Boss | 2 | 3 | Hater | Any | Score > CPU |

## Implementation notes for Copilot

- Deck order in campaign missions is the exact list given. Draw from front, no shuffle.
- Hand order doesn't matter (hands are unordered sets), but contents matter.
- Piles are specified as `top` and optionally `initial stack`. If no initial stack, pile starts with just the top.
- Stacked deck: if the deck is empty, draws/flips are unavailable. If the deck is shorter than the refill needs, hands just don't refill to 6 (they stay at whatever size).
- Push count is fixed per side; mission config overrides draft computation.
- CPU policy is described per mission — implement as a weighted score overlay on the base heuristic.
- Missions do not need to be fair or winnable in all cases. They are puzzles. Some require specific play sequences; retry is expected.
- M6 (hidden tops): implement `pile.hidden` flag. Flip or play reveals it. Bounty suit is hidden until the first deck flip.
- M12 (adaptive): CPU tracks player action history (pushes spent, flips taken). At start of round 3, compute player aggression and set policy.
- M14 (gauntlet): implement as a sequence of 3 standard games. Player hero persists. Track wins, end after game 3 or when someone has 2 wins.
- M11 (one-way pushes): add `pushOneWay` flag. When true, player's pushes may only move piles toward player, never away.

## Stat derivation reminder

Missions cannot force s/f/b to specific values because the player picks their hero. The mission only controls:
- CPU hero (and thus CPU's contribution to s/f/b)
- Board, hands, deck, bounty suit
- Push counts for both sides (explicitly, overriding draft)
- Win condition, end condition, hidden-info flags
- CPU policy
