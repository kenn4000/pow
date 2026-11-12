# 15-Mission Campaign Design

Each mission teaches one mechanic or strategic concept. The player picks their hero (which adjusts j/k/b/d per the rules), but the mission **overrides hand composition, opponent hand size, and/or scoring parameters** to create the lesson. The CPU opponent uses a specific algorithm tuned to the lesson.

---

## Mission 1 — "First Steps"

**Teaches:** Basic turn flow, melding, and scoring.

**Setup:** Player hero: Recruit (fixed). Opponent: Recruit. Standard rules (j=6, k=0, b=0, d=0, hand size 6).

**Player starting hand (scripted):** 3♥ 4♥ 5♥ 7♠ 7♥ 7♦ 2♠
**Opponent starting hand (scripted):** 1♠ 2♠ 3♠ 6♣ 8♣ 9♣ 4♦

**Goal:** Score ≥ +5 this hand. (Easy—the player has a ready-made run and set.)

**CPU algorithm:** **Random.** Discards randomly, never takes A. Gives the player breathing room to learn.

---

## Mission 2 — "The Take"

**Teaches:** Option A vs. Option B. The discard is a tool.

**Setup:** Player hero: Recruit. Opponent: Recruit. Standard rules.

**Player starting hand (scripted):** 3♥ 4♥ 6♥ 7♠ 7♥ 7♦ 2♠
**Opponent starting hand (scripted):** 1♠ 2♠ 3♠ 6♣ 8♣ 9♣ 5♥

**Opponent's first discard (scripted):** 5♥ (completes the player's 3♥-4♥-5♥-6♥ run)

**Goal:** Win the hand.

**Lesson:** The 5♥ is face-up on turn 1. Taking it (Option A) completes a 4-card run immediately. The player should learn that A is for completion, B is for digging.

**CPU algorithm:** **Random discard, never takes A.** Passive opponent.

---

## Mission 3 — "Dig Deep"

**Teaches:** Option B is how you build when the discard doesn't help.

**Setup:** Player hero: Recruit. Opponent: Recruit. Standard rules.

**Player starting hand (scripted):** 1♠ 3♠ 5♠ 7♠ 9♠ 2♥ 4♥
**Opponent starting hand (scripted):** Random.

**Stacked stock (scripted top 4):** 2♠ 4♠ 6♠ 8♠

**Goal:** Score ≥ +3.

**Lesson:** The hand is scattered. The discard won't help early. Drawing 2 (Option B) repeatedly is how you find the 2♠/4♠/6♠/8♠ that complete the spade run. Teaches "B is the default when nothing fits."

**CPU algorithm:** **Proximity discard, never takes A.** Passive.

---

## Mission 4 — "Deadweight"

**Teaches:** Deadwood is expensive. Dump high cards.

**Setup:** Player hero: Recruit. Opponent: Recruit. Standard rules.

**Player starting hand (scripted):** 1♥ 2♥ 3♥ 9♠ 9♥ 9♦ 8♣
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +8.

**Lesson:** The hand has a run (1-2-3♥) and a set (9s) = 9 + 6+9 = 24 melded points, but the 8♣ is deadwood. If the player doesn't dump it, they lose ~8 points. Teaches "meld everything, dump the rest."

**CPU algorithm:** **Proximity discard, takes A if completes meld.** Slightly active.

---

## Mission 5 — "The Race"

**Teaches:** The 12-turn clock. You can't wait forever.

**Setup:** Player hero: Recruit. Opponent: Recruit. Standard rules. **Reduced turns: 8 total (4 each).**

**Player starting hand (scripted):** 2♣ 3♣ 4♣ 5♦ 6♦ 9♥ 1♠
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +2.

**Lesson:** With only 4 turns each, the player must commit to a meld path *immediately*. The 2-3-4♣ run is a gift; the 5-6♦ is a trap (needs 4♦ or 7♦, unlikely in 4 turns). Teaches "when the clock is short, take the sure meld."

**CPU algorithm:** **Proximity discard, takes A if completes meld.** Same as M4.

---

## Mission 6 — "Favor"

**Teaches:** Favor raises set value. High-Favor heroes want sets.

**Setup:** Player hero: **Warden** (Favor 1). Opponent: Warden. Derived: j=8, k=0, b=1, d=0.

**Player starting hand (scripted):** 3♠ 3♥ 3♦ 6♣ 6♠ 6♥ 9♦
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +20.

**Lesson:** Two sets (3s and 6s) score `(8+3) + (8+6) = 11 + 14 = 25` under j=8. Compare to j=6: `9 + 12 = 21`. The +2 Favor bonus is worth 4 points here. Teaches "Favor makes sets better; pick Warden/Vulture/Diplomat when you want set value."

**CPU algorithm:** **Proximity discard, takes A if completes meld.** Passive.

---

## Mission 7 — "Shift"

**Teaches:** Shift raises face values and deadwood. High-Shift games are swingy.

**Setup:** Player hero: **Gorilla** (Shift 2). Opponent: Gorilla. Derived: j=6, k=3, b=0, d=−1. Hand size 5.

**Player starting hand (scripted):** 7♥ 8♥ 9♥ 4♠ 4♥ 4♦ (5 cards, d=−1)
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +15.

**Lesson:** With k=3, the run scores `9+3+3 = 15` and the set scores `6+4+3 = 13`. But if either fails to meld, a 9 is `−12` deadwood. Teaches "Shift makes melds worth more but punishes deadwood harder—you must meld everything."

**CPU algorithm:** **Proximity discard, takes A if completes meld.** Same.

---

## Mission 8 — "Bounty"

**Teaches:** Bounty raises run value. High-Bounty heroes want runs.

**Setup:** Player hero: **Body** (Bounty 3). Opponent: Body. Derived: j=6, k=0, b=3, d=−1. Hand size 5.

**Player starting hand (scripted):** 4♣ 5♣ 6♣ 7♣ 2♦ (5 cards)
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +12.

**Lesson:** The 4-5-6-7♣ run scores `7 + 0 + 4 + 3 = 14` under b=3. Compare to b=0: `11`. The +3 Bounty is worth +3 on this run. Teaches "Bounty makes runs better; pick Body/Hunter/Gambler when you want run value."

**CPU algorithm:** **Proximity discard, takes A if completes meld.**

---

## Mission 9 — "Draft"

**Teaches:** Draft changes hand size. Big hands have more options but more deadwood risk.

**Setup:** Player hero: **Gambler** (Draft 1). Opponent: Gambler. Derived: j=6, k=1, b=2, d=2 → **hand size 8**.

**Player starting hand (scripted):** 2♥ 3♥ 4♥ 6♠ 6♥ 6♦ 8♣ 9♣ (8 cards)
**Opponent starting hand (scripted):** 8 cards, random.

**Goal:** Score ≥ +15.

**Lesson:** The player has 8 cards, a run (2-3-4♥) and a set (6s), but also 8♣ 9♣ deadwood. With 8 cards and only 12 turns, the player must meld *and* discard efficiently. Teaches "big hands are powerful but require discipline."

**CPU algorithm:** **Proximity discard, takes A if completes meld.** Active.

---

## Mission 10 — "The Fork"

**Teaches:** A vs. B is a real decision. The opponent's discard is a weapon.

**Setup:** Player hero: Recruit. Opponent: **Hunter** (Bounty 2, Draft 0). Derived: j=6, k=0, b=2, d=0.

**Player starting hand (scripted):** 5♥ 6♥ 7♥ 2♠ 3♠ 9♦ 1♣
**Opponent starting hand (scripted):** 4♦ 5♦ 6♦ 8♣ 8♠ 8♥ 3♥

**Scripted opponent first discard:** 3♥ (completes the player's 1♣-2♠-3♠? No—3♥ is hearts, not spades. It's a *trap*: looks like it fits, but doesn't complete anything. The player should choose B.)

**Goal:** Score ≥ +5.

**Lesson:** The face-up discard is bait. Taking it (A) grows your hand by 1 but doesn't meld. Drawing 2 (B) finds real meld material. Teaches "A is for completion, not for hoping."

**CPU algorithm:** **Meld-completion A, proximity discard.** Standard.

---

## Mission 11 — "Pressure"

**Teaches:** The opponent is actively taking your discards. You must think about what you're giving away.

**Setup:** Player hero: Recruit. Opponent: **Gambler** (Draft 1, Shift 1, Bounty 2). Derived: j=6, k=1, b=2, d=1. Opponent hand size 7.

**Player starting hand (scripted):** 3♠ 4♠ 5♠ 8♥ 8♦ 8♣ 1♦
**Opponent starting hand (scripted):** 2♠ 6♠ 9♥ 9♦ 9♣ 4♥ 5♥

**Goal:** Score ≥ +5.

**Lesson:** The opponent has a strong hand and will take any discard that completes a meld. The player must **avoid discarding 6♠ or 2♠** (which complete the opponent's spade run) and **avoid discarding 9s** (which complete the opponent's set). Teaches "discard what your opponent can't use."

**CPU algorithm:** **Meld-completion A, proximity discard, but aggressive:** takes A if the discard completes *any* meld, even a 3-card set. Standard CPU.

---

## Mission 12 — "The Comeback"

**Teaches:** A bad start doesn't mean a lost hand. Reshuffle and recovery.

**Setup:** Player hero: Recruit. Opponent: Recruit. Standard rules.

**Player starting hand (scripted):** 1♠ 3♠ 5♠ 7♠ 9♠ 2♥ 4♥ (scattered, like M3 but worse)
**Opponent starting hand (scripted):** 2♣ 3♣ 4♣ 6♦ 6♥ 6♠ 8♥ (opponent has a run and set—strong start)

**Goal:** **Win** the hand (not just score ≥ X).

**Lesson:** The opponent starts strong. The player must draw aggressively (B), find melds, and outscore. Teaches "the game isn't over until turn 12."

**CPU algorithm:** **Meld-completion A, proximity discard.** Standard, but the opponent's strong hand means it will meld fast and score high.

---

## Mission 13 — "The Set Trap"

**Teaches:** Sets are rarer than runs (only 3 suits). A near-set is a trap—don't over-invest.

**Setup:** Player hero: **Vulture** (Favor 1, Bounty −2). Opponent: Vulture. Derived: j=7, k=0, b=−2, d=0.

**Player starting hand (scripted):** 5♠ 5♥ 6♦ 7♦ 8♦ 9♣ 1♣
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +10.

**Lesson:** The player has two 5s (5♠ 5♥)—looks like a set forming. But the third 5 (5♦) may never come. Meanwhile the 6-7-8♦ run is complete. Teaches "don't hold a pair hoping for the third; meld what you have." Under Vulture's b=−2, runs are penalized, so the player must weigh set potential vs. run penalty.

**CPU algorithm:** **Meld-completion A, proximity discard.**

---

## Mission 14 — "The Long Game"

**Teaches:** Extended turns (16 total) change strategy. More agency, more deadwood risk.

**Setup:** Player hero: Recruit. Opponent: Recruit. Standard rules but **16 total turns (8 each)**.

**Player starting hand (scripted):** Random.
**Opponent starting hand (scripted):** Random.

**Goal:** Score ≥ +10.

**Lesson:** With 8 turns each, the player has time to build a big hand—but also time to accumulate deadwood. Teaches "longer games reward planning, but punish greed."

**CPU algorithm:** **Meld-completion A, proximity discard.** Standard.

**Note:** Reshuffle may trigger in this mission (stock drains by 16 from 13–17 start). Implement the forced-A rule: when stock < 2, B is illegal.

---

## Mission 15 — "The Final Test"

**Teaches:** Everything at once. No scripted hands, no training wheels.

**Setup:** Player hero: **Player's choice** (any hero). Opponent: **Random hero** (rolled at mission start). Derived params from the matchup. Standard rules (12 turns).

**Player starting hand:** Random.
**Opponent starting hand:** Random.

**Goal:** Win the match (two hands, sum scores). Opponent plays a **strong CPU** (see below).

**Lesson:** The player must pick a hero that suits their playstyle, adapt to the opponent's hero, and execute across two hands.

**CPU algorithm:** **Full lookahead** (the "Hard" CPU from the strategy spec):
- Option A: compares `hand_quality(hand + discard)` vs. expected B value over 10 samples.
- Discard: exhaustive pair search (drop the 2 cards that leave the best hand).
- Takes A whenever it improves the hand, even without a completed meld.

---

## CPU Algorithm Summary

| Algorithm | Behavior | Used in |
|---|---|---|
| **Random** | Random discard, never takes A | M1 |
| **Passive** | Proximity discard, never takes A | M2, M3 |
| **Standard** | Proximity discard, takes A if completes meld | M4–M13 |
| **Aggressive** | Same as standard but prioritizes A more | M11 (opponent) |
| **Hard** | Full lookahead, exhaustive discard | M15 |

---

## Difficulty Curve

- **M1–M3:** Learn the basics. Passive CPU. Scripted hands teach one concept each.
- **M4–M5:** Scoring and the clock. Standard CPU.
- **M6–M9:** Hero parameters. Each mission isolates one stat (Favor, Shift, Bounty, Draft). CPU standard.
- **M10–M11:** Strategic depth. Bait discards, opponent pressure. CPU standard/aggressive.
- **M12–M13:** Recovery and traps. Standard CPU, harder scenarios.
- **M14:** Extended game variant. Standard CPU.
- **M15:** Final test. Hard CPU, random matchup.

---

## Implementation Notes for Copilot

1. **Scripted hands:** Each mission has a `player_hand` and `opponent_hand` list. Deal these instead of random. The stock is the remaining 27 − dealt cards, shuffled.
2. **Scripted discards:** Some missions script the opponent's first discard (M2, M10). Force it on the opponent's first B turn.
3. **Scripted stock:** M3 stacks the top 4 stock cards. Ensure they're drawn in order.
4. **Turn count override:** M5 uses 8 total turns, M14 uses 16. All others use 12.
5. **Hand size override:** M9 uses d=2 (hand size 8). M15 uses standard 6+d.
6. **Goal checking:** Each mission has a goal (score ≥ X, or win). Check at hand end. If met, mission complete; if not, retry.
7. **Hero lock:** M1–M3 lock the player to Recruit. M6–M9 lock to a specific hero (Warden, Gorilla, Body, Gambler). M15 lets the player choose any hero.
8. **CPU algorithm selection:** Each mission specifies which CPU algorithm to use. Implement all five per the strategy spec.
9. **Opponent hero:** M1–M14 fix the opponent's hero (usually mirroring the player's for fairness). M15 rolls a random opponent hero at mission start. 

---

## What each mission teaches (quick reference)

| # | Mission | Teaches | CPU |
|---|---|---|---|
| 1 | First Steps | Basic flow, melding | Random |
| 2 | The Take | Option A for completion | Passive |
| 3 | Dig Deep | Option B for digging | Passive |
| 4 | Deadweight | Dump high deadwood | Standard |
| 5 | The Race | Short clock = commit early | Standard |
| 6 | Favor | Favor raises set value | Standard |
| 7 | Shift | Shift raises face value + deadwood | Standard |
| 8 | Bounty | Bounty raises run value | Standard |
| 9 | Draft | Big hand = more options, more risk | Standard |
| 10 | The Fork | A vs. B is a real decision | Standard |
| 11 | Pressure | Opponent takes your discards | Aggressive |
| 12 | The Comeback | Bad start ≠ lost hand | Standard |
| 13 | The Set Trap | Don't over-invest in near-sets | Standard |
| 14 | The Long Game | Extended turns = more agency + risk | Standard |
| 15 | The Final Test | Everything, no training wheels | Hard |

---
 