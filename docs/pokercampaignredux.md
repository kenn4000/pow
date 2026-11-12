# Full 15-Mission Campaign — Single-Player vs. Par

**Important clarification first:** You said "single player goals against a target score," so the campaign is **player vs. par**, not player vs. CPU. However, some missions work better as **player vs. a lightweight CPU that demonstrates a strategy** — the CPU is a teaching tool, not a real opponent. The CPU plays a *fixed script* or a *simple heuristic* that shows the player what a specific strategy looks like, then the player must beat the CPU's score to prove they understood.

So the campaign is **hybrid**: most missions are pure score-vs-par, but a few missions are score-vs-CPU where the CPU is a "ghost" teaching a lesson. I've marked which missions have a CPU opponent and which don't.

Here's the full spec.

---

## Campaign Structure

**Three arcs, five missions each:**

- **Arc 1 — Learn the Rules (Missions 1–5):** Scripted hands teach the mechanics. No CPU opponent. Player vs. par only.
- **Arc 2 — Learn the Strategies (Missions 6–10):** Some missions introduce a **scripted CPU ghost** that demonstrates a strategy. Player must beat both the par and the ghost.
- **Arc 3 — Master the System (Missions 11–15):** No CPU opponents. Pure score-vs-par at escalating difficulty. Player must pick the right hero for each config.

---

## Mission Data Model

```
mission = {
    "id": int,
    "name": str,
    "lesson": str | null,               # popup text (null = no popup)
    "config": {
        "nine_base": int,               # default 12
        "bounty_base": int,             # default 1
        "forced_bounty_suit": str | null,  # "rock"/"paper"/"scissors" or null (random)
        "starting_hand": list[Card] | null,
        "hand_size": int,               # default 6
    },
    "constraint": str | null,           # "no_nines", "no_straights_or_flushes", etc.
    "par": int,                         # score needed to clear
    "cpu_opponent": {
        "enabled": bool,
        "script": list[(turn, action)] | null,   # fixed-play ghost
        "heuristic": str | null,                 # "bounty"/"nine"/"pairs"/"balanced"
        "starting_hand": list[Card] | null,
        "hand_size": int,               # default 6
        "teaching_note": str | null,    # popup explaining the CPU's strategy
    } | null,
    "recommended_heroes": list[str],
    "win_condition": "score >= par" | "score > cpu_score AND score >= par",
}
```

---

## Arc 1 — Learn the Rules (Missions 1–5)

### Mission 1: *The First Hand*

**Teaching goal:** Learn how scoring works.

**Config:**
- Default (`nine_base = 12`, `bounty_base = 1`)
- Forced bounty suit: **Rock**
- Scripted starting hand: `[9-Rock, 9-Paper, 3-Scissors, 4-Rock, 5-Paper, 6-Scissors]`
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None.** Pure score-vs-par.

**Par:** 160

**Lesson popup (turn 1):**
> "Welcome to the poker mission. You'll place cards into three hands (A, B, C). Each hand scores as a poker hand: pair = 20, flush = 40, straight = 50, three of a kind = 60, straight flush = 100. Fill all three hands to maximize your score. You have a pair of 9s in your starting hand — place them together for a strong hand."

**Recommended heroes:** Hunter, Warden

**Why this works:** The player sees a guaranteed pair of 9s. They'll naturally put them in the same hand, score a pair (20) + 9-bonus (12 × 2 for Shift 0 = 12, so 32 base for that hand), and get a feel for the scoring. Par 160 is easy — it's roughly the bottom quartile of a single game.

---

### Mission 2: *Finding the Nine*

**Teaching goal:** The 9-bonus is the biggest single-hand payout.

**Config:**
- Default
- Forced bounty suit: **Paper**
- Scripted starting hand: `[9-Rock, 2-Paper, 3-Scissors, 4-Rock, 5-Paper, 6-Scissors]`
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 175

**Lesson popup (turn 1):**
> "A hand that contains at least one 9 gets a big bonus: +12 + 2×Shift. For most heroes, that's +12 to +16 per 9-hand. Chasing 9s is the most direct scoring path. You have one 9 — place it where it can anchor a strong hand."

**Recommended heroes:** Gorilla, Hater

**Why this works:** The player has exactly one 9. Par 175 requires them to score the 9-hand AND build two other decent hands. They learn that the 9-bonus is worth about half a pair by itself.

---

### Mission 3: *Your Suit*

**Teaching goal:** Bounty bonuses reward stacking your bounty suit.

**Config:**
- Default
- Forced bounty suit: **Scissors**
- Scripted starting hand: `[2-Scissors, 5-Scissors, 8-Scissors, 3-Rock, 4-Paper, 7-Rock]`
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 185

**Lesson popup (turn 1):**
> "When a hand contains enough cards of your bounty suit, it scores bonus points. The base payout is 1 + your hero's Bounty stat. Your hero's Favor stat changes how many bounty cards you need: Favor +1 needs only 1, Favor 0 needs 2, Favor −1 needs 3. You have three Scissors cards — try to trigger bounty in at least one hand."

**Recommended heroes:** Bodyb, Hunter

**Why this works:** The player has three Scissors cards. If they put two in the same hand, they get a bounty trigger. If they pick Bodyb (Bounty +3), the payout is 4 per trigger. Par 185 requires a mix of poker hands and bounty.

---

### Mission 4: *The Even Table*

**Teaching goal:** The distribution rule forces all three hands to develop together.

**Config:**
- Default
- Forced bounty suit: **Rock**
- Scripted starting hand: `[2-Rock, 4-Paper, 6-Scissors, 3-Paper, 5-Rock, 7-Scissors]` (balanced, no obvious pairs)
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 185

**Lesson popup (turn 1):**
> "All three hands must be within 1 card of each other at all times. You can't fill one hand while ignoring the others. This means every card placement matters — you're building three hands simultaneously, not one strong hand and two dumps."

**Recommended heroes:** Recruit, Warden

**Why this works:** The player has no obvious pairs in their starting hand. They have to play every card thoughtfully. Par 185 is achievable with moderate play, but only if they route cards carefully across all three hands.

---

### Mission 5: *The Bonus Table*

**Teaching goal:** The A-best (+15) and C-worst (+5) bonuses reward deliberate hierarchy.

**Config:**
- Default
- Forced bounty suit: **Rock**
- Scripted starting hand: `[9-Rock, 3-Paper, 3-Scissors, 2-Rock, 4-Paper, 6-Scissors]`
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 195

**Lesson popup (turn 1):**
> "If Hand A scores strictly higher than B and C (using the full hand total, including 9-bonus and bounty), you get +15. If Hand C scores strictly lower than A and B, you get +5. Both bonuses can fire together if A > B > C. Make A your best hand and C your worst."

**Recommended heroes:** Gambler, Hater

**Why this works:** The player has a 9 (strong card) and a pair of 3s. They have to route the 9 to A, the pair to B or C, and the low cards to C. The hierarchy is achievable if they plan ahead. Par 195 requires both bonuses to fire.

---

## Arc 2 — Learn the Strategies (Missions 6–10)

### Mission 6: *Chase the Scissors*

**Teaching goal:** When the bounty suit is forced to a specific suit, heroes with high Bounty or Favor get the most value.

**Config:**
- Default
- Forced bounty suit: **Scissors**
- Scripted starting hand: `[9-Rock, 9-Scissors, 2-Paper, 4-Rock, 6-Scissors, 7-Paper]`
- Hand size: 6

**Constraint:** None

**CPU opponent:** **Ghost** (teaching only)
- Strategy: **Bounty-focused** — the CPU always places Scissors cards into Hand A, and other cards elsewhere
- Starting hand: `[2-Scissors, 4-Scissors, 6-Scissors, 3-Rock, 5-Paper, 7-Rock]`
- Hand size: 6
- Teaching note popup on turn 1:
  > "Watch the CPU's plays. It's chasing Scissors — every Scissors card goes into its best hand. By turn 5, you'll see the pattern. Your job: outscore it. Chase 9s instead of Scissors to exploit the difference."

**Par:** 200

**Win condition:** `score >= par AND score > cpu_score`

**Lesson popup (turn 1):**
> "The CPU is demonstrating a bounty-chasing strategy. It will stack Scissors cards into one hand to trigger bounty repeatedly. Watch how it plays, then beat it by chasing 9s instead — you have two 9s in your starting hand."

**Recommended heroes:** Gorilla, Hater, Gambler

**Why this works:** The player has two 9s and two Scissors. The CPU is stacking Scissors. If the player stacks 9s, they win the 9-bonus contest. If the player competes on Scissors, they tie the CPU and lose on the par. The lesson is "different axes of scoring are more effective than competing on the same one."

---

### Mission 7: *Chase the Paper*

**Teaching goal:** Same lesson as Mission 6, flipped. The CPU chases 9s; the player should chase bounty.

**Config:**
- Default
- Forced bounty suit: **Paper**
- Scripted starting hand: `[2-Paper, 5-Paper, 8-Paper, 3-Rock, 4-Scissors, 6-Rock]` (three Paper)
- Hand size: 6

**Constraint:** None

**CPU opponent:** **Ghost** (teaching only)
- Strategy: **9-focused** — the CPU always places 9s into Hand A and builds around them
- Starting hand: `[9-Rock, 2-Paper, 4-Scissors, 5-Rock, 6-Scissors, 8-Paper]`
- Hand size: 6
- Teaching note popup on turn 1:
  > "The CPU is chasing 9s. You have three Paper cards and a forced Paper bounty suit. Chase Paper bounty instead of 9s — the two strategies are equally strong, but only one of you is playing each."

**Par:** 200

**Win condition:** `score >= par AND score > cpu_score`

**Lesson popup (turn 1):**
> "The CPU is demonstrating a 9-chasing strategy. It will place its 9 into Hand A immediately and build everything else around that hand. Beat it by chasing Paper bounty instead — you have three Paper cards ready."

**Recommended heroes:** Bodyb, Hunter, Gambler

**Why this works:** Mirror of Mission 6. The player learns that the counter to "chase the forced suit" is "chase 9s" and vice versa. Both strategies are viable; the CPU demonstrates one so the player can practice the other.

---

### Mission 8: *Silent Nines*

**Teaching goal:** Constraints change the scoring math. When 9s are banned, the game shifts to pairs, straights, and bounty.

**Config:**
- Default
- Forced bounty suit: **Rock**
- Scripted starting hand: `[2-Rock, 3-Paper, 4-Scissors, 5-Rock, 6-Paper, 7-Scissors]` (no 9s)
- Hand size: 6

**Constraint:** `no_nines` (no completed hand may contain a 9)

**CPU opponent:** **None** (the constraint is the challenge)

**Par:** 175

**Lesson popup (turn 1):**
> "This mission bans 9s. You cannot place a 9 into any hand, and any hand that contains a 9 fails the mission. Your scoring options are limited to pairs (20), straights (50), flushes (40), and bounty. Build hands without relying on the 9-bonus."

**Recommended heroes:** Bodyb, Hunter, Warden

**Why this works:** The player has spent 7 missions learning "chase 9s." Now they can't. Par drops to 175 because the 9-bonus is gone (that's a ~15-point loss per 9-hand). The player must lean on bounty and poker hand type. The recommended heroes have strong bounty stats or Favor +1 to compensate.

**Implementation note:** When the player draws a 9, they cannot place it. They must discard it (via redraw) or hold it until end of game (it counts as a void card if unplayed — no penalty, but no bonus either). The CPU also cannot place 9s.

---

### Mission 9: *The Wall*

**Teaching goal:** When straights and flushes are banned, you're forced into pairs + 9s + bounty.

**Config:**
- Default
- Forced bounty suit: **Paper**
- Scripted starting hand: `[9-Rock, 9-Paper, 3-Scissors, 3-Rock, 4-Paper, 5-Scissors]` (pair of 9s + pair of 3s)
- Hand size: 6

**Constraint:** `no_straights_or_flushes` (straights, flushes, and straight flushes score 0 base)

**CPU opponent:** **None**

**Par:** 195

**Lesson popup (turn 1):**
> "No straights or flushes. Any hand that scores as a straight (50) or flush (40) instead scores 0 for its base. Your best paths are pairs (20), three of a kind (60), 9-bonus, and bounty. You have a pair of 9s and a pair of 3s — place them carefully."

**Recommended heroes:** Hunter, Warden

**Why this works:** The player has two pairs in hand. They can put one pair in A (getting a 9-hand with a pair, high base + 9-bonus), and the other in B or C. Par 195 is achievable with two pairs and a decent third hand. The constraint makes the game about pairs, not poker hand types.

**Implementation note:** Under this constraint, a straight or flush scores 0 base (not 50/40), but the 9-bonus and bounty still apply on top. A straight with a 9 scores 0 + 12 = 12 (plus bounty if applicable), not 50 + 12.

---

### Mission 10: *Free Play*

**Teaching goal:** No scripts, no constraints, no teaching. Just play and score.

**Config:**
- Default
- Forced bounty suit: **Random** (rolled at game start)
- Starting hand: **Random**
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 205

**Lesson popup (turn 1):**
> "No scripts, no forced suits, no constraints. This is the default game. Score 205 to advance. Everything you've learned so far is fair game."

**Recommended heroes:** Gambler, Hater, Hunter

**Why this works:** This is the transition mission. The player has learned all the mechanics (9-bonus, bounty, distribution, split bonus, constraints). Now they play a default game and must hit 205, which is about the 60th percentile. If they can clear 205 consistently, they're ready for Arc 3.

---

## Arc 3 — Master the System (Missions 11–15)

### Mission 11: *High Bar*

**Teaching goal:** At high pars, hero choice matters more than play skill.

**Config:**
- Default
- Forced bounty suit: **Rock**
- Starting hand: **Random**
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 210

**Lesson popup (turn 1):**
> "Par 210 is high. The average game scores ~193; only heroes with strong scoring modifiers clear 210 consistently. Hater (mean 198), Gambler (mean 198), and Hunter (mean 196) are the best picks. Recruit (mean 189) can clear 210 but needs a strong draw."

**Recommended heroes:** Hater, Gambler, Hunter

**Why this works:** The player has all the mechanics from Arc 2. Par 210 requires them to pick a top-tier hero. This teaches that "playing well" isn't enough at high pars — hero choice matters.

---

### Mission 12: *Consistency*

**Teaching goal:** Consistency is a different skill than peak performance.

**Config:**
- Default
- Forced bounty suit: **Random** (re-rolled each game)
- Starting hand: **Random**
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 175 per game × 3 consecutive games

**Special rule:** The player must score ≥ 175 in **three consecutive games**. If any game scores below 175, the streak resets to 0. The player can change heroes between games, but switching heroes also resets the streak to 0.

**Lesson popup (turn 1):**
> "Three consecutive games ≥ 175. Pick a hero whose scores are stable, not spiky. Gambler (IQR 50, mean 198) and Hunter (IQR 47, mean 196) are the most consistent top-tier heroes. Turtle (IQR 52) is stable but low-scoring. Recruit (IQR 50) is a safe pick if you can hit 175."

**Recommended heroes:** Gambler, Hunter, Recruit

**Why this works:** A player who scores 220 sometimes and 160 other times fails. A player who scores 180 consistently clears. This teaches that variance is the enemy of consistency challenges, and that hero choice should account for variance.

**Implementation note:** Track streak across attempts. Display "Game 2/3 — need 175+" between games. Reset streak on failure or hero switch.

---

### Mission 13: *Doubled Bounty*

**Teaching goal:** When the config shifts toward bounty, bounty-heavy heroes become S-tier.

**Config:**
- `nine_base = 8` (9-bonus is weak)
- `bounty_base = 3` (bounty is dominant)
- Forced bounty suit: **Scissors**
- Starting hand: **Random**
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 210

**Lesson popup (turn 1):**
> "This mission triples the bounty payout. A bounty trigger pays `max(0, 3 + Bounty)` instead of `max(0, 1 + Bounty)`. Bodyb's Bounty +3 makes him S-tier: 6 points per trigger × ~1.4 triggers = ~8 bonus points from bounty alone. Meanwhile, the 9-bonus is only 8 + 2×Shift (vs. 12 + 2×Shift normally), so Shift-heavy heroes are weaker."

**Recommended heroes:** Bodyb, Hunter, Gambler

**Why this works:** The config strongly favors bounty heroes. Bodyb's mean under this config is ~+8 over baseline. The player learns that config changes shift the optimal hero choice.

---

### Mission 14: *Nine Storm*

**Teaching goal:** When the config shifts toward 9s, Shift-heavy heroes become S-tier.

**Config:**
- `nine_base = 16` (9-bonus is huge)
- `bounty_base = 1` (bounty is normal)
- Forced bounty suit: **Paper**
- Starting hand: **Random**
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 215

**Lesson popup (turn 1):**
> "This mission raises the 9-bonus to 16 + 2×Shift. Gorilla's Shift 2 gives him 20 points per 9-hand (vs. 16 normally). Three 9-hands = 60 points from the 9-bonus alone. Meanwhile, bounty is unchanged, so bounty-focused heroes are weaker relative to the 9-chasers."

**Recommended heroes:** Gorilla, Hater, Gambler

**Why this works:** The mirror of Mission 13. The player learns that the 9-bonus config favors Shift heroes. Par 215 is achievable with Gorilla, harder with Bodyb.

---

### Mission 15: *The Crown*

**Teaching goal:** Everything you've learned, in one final test.

**Config:**
- Default (`nine_base = 12`, `bounty_base = 1`)
- Forced bounty suit: **Random**
- Starting hand: **Random**
- Hand size: 6

**Constraint:** None

**CPU opponent:** **None**

**Par:** 235

**Lesson popup:** **None** (no hints, no hints popup, no recommended heroes visible)

**Recommended heroes:** None (the player must decide)

**Why this works:** Par 235 is above the 90th percentile for most heroes (~225) and near the 95th percentile for the top heroes (Gambler 235, Hater 255). The player must pick a top hero, play well, and have a strong draw. There's no single trick — everything they've learned is tested at once.

---

## CPU Ghost Implementation

Missions 6 and 7 have CPU ghost opponents. The ghost is **not a real opponent** — it plays a fixed strategy and is there to demonstrate a technique. Implementation:

### CPU ghost model

```
cpu_ghost = {
    "strategy": "bounty" | "nine" | "pairs" | "balanced",
    "starting_hand": [cards],
    "hand_size": 6,
    "visible_to_player": true,   # the player can see the ghost's plays
}
```

The ghost plays its turns using a **simplified heuristic**:

- **Bounty strategy:** Always place a bounty-suit card if one is in hand; otherwise place the highest base-score card. Never redraw.
- **Nine strategy:** Always place a 9 if one is in hand; otherwise place the highest base-score card. Never redraw.
- **Pairs strategy:** Always place a card that would complete a pair if possible; otherwise place the highest base-score card. Never redraw.
- **Balanced:** Use the standard Medium heuristic.

**The ghost plays on its own turn** (alternating with the player). The player sees the ghost's placements as they happen. The ghost's score is computed normally at end of game.

**Win condition:** `player_score >= par AND player_score > ghost_score`

**If the player beats par but loses to the ghost:** the mission fails, but the player can retry. The ghost's strategy is deterministic, so the player can learn from the loss.

**The ghost is not a real opponent** — there's no timer, no visibility rules, no Hand A public. The ghost is just a scoreboard. The player plays their game normally; the ghost plays its game normally; both scores are computed; the player must exceed both par and ghost.

---

## Summary Table

| # | Name | Arc | Config Change | Constraint | CPU Ghost | Par | Win Condition |
|---|---|---|---|---|---|---|---|
| 1 | The First Hand | 1 | Rock forced, scripted hand | — | No | 160 | Score ≥ 160 |
| 2 | Finding the Nine | 1 | Paper forced, scripted hand | — | No | 175 | Score ≥ 175 |
| 3 | Your Suit | 1 | Scissors forced, scripted hand | — | No | 185 | Score ≥ 185 |
| 4 | The Even Table | 1 | Rock forced, scripted hand | — | No | 185 | Score ≥ 185 |
| 5 | The Bonus Table | 1 | Rock forced, scripted hand | — | No | 195 | Score ≥ 195 |
| 6 | Chase the Scissors | 2 | Scissors forced, scripted hand | — | **Yes, bounty** | 200 | Score ≥ 200 AND > ghost |
| 7 | Chase the Paper | 2 | Paper forced, scripted hand | — | **Yes, nine** | 200 | Score ≥ 200 AND > ghost |
| 8 | Silent Nines | 2 | Rock forced, scripted hand | no_nines | No | 175 | Score ≥ 175, constraint pass |
| 9 | The Wall | 2 | Paper forced, scripted hand | no_straights_or_flushes | No | 195 | Score ≥ 195, constraint pass |
| 10 | Free Play | 2 | Random, random hand | — | No | 205 | Score ≥ 205 |
| 11 | High Bar | 3 | Rock forced, random hand | — | No | 210 | Score ≥ 210 |
| 12 | Consistency | 3 | Random, random hand | — | No | 175 × 3 games | Streak of 3 games ≥ 175 |
| 13 | Doubled Bounty | 3 | nb=8, bb=3, Scissors forced | — | No | 210 | Score ≥ 210 |
| 14 | Nine Storm | 3 | nb=16, bb=1, Paper forced | — | No | 215 | Score ≥ 215 |
| 15 | The Crown | 3 | Default, random suit | — | No | 235 | Score ≥ 235 |

---

## Implementation Checklist

```
[ ] Single-player game loop
[ ] Mission config loading (nine_base, bounty_base, forced_bounty_suit, starting_hand, hand_size)
[ ] Scripted starting hand dealing (bypass shuffle for initial deal, shuffle rest)
[ ] Constraint checking (no_nines, no_straights_or_flushes)
[ ] Par comparison: score >= par clears mission
[ ] CPU ghost support for missions 6-7:
    [ ] Ghost plays on alternating turns (player-ghost-player-ghost...)
    [ ] Ghost uses simplified heuristic (bounty, nine, pairs, balanced)
    [ ] Ghost's plays visible to player
    [ ] Ghost's score computed normally
    [ ] Win condition: player_score >= par AND player_score > ghost_score
[ ] Mission 12 streak tracking:
    [ ] Track streak across attempts
    [ ] Reset on sub-175 score or hero switch
    [ ] Display current streak between games
[ ] Teaching popups (mission.lesson on turn 1)
[ ] Recommended heroes as hints (not enforced)
[ ] Campaign state: current_mission, cleared_missions, best_scores, mission_12_streak
[ ] Retry: reload mission config, reset game state

VERIFICATION
[ ] Mission 1 clears at 160 with pair of 9s
[ ] Mission 6 ghost plays bounty strategy (puts Scissors in Hand A)
[ ] Mission 7 ghost plays nine strategy (puts 9s in Hand A)
[ ] Mission 8 constraint: 9 in any hand fails the mission
[ ] Mission 9 constraint: straight/flush scores 0 base, not 50/40
[ ] Mission 12 streak resets on sub-175 game
[ ] Mission 12 streak resets on hero switch
[ ] All pars clearable with recommended heroes (simulate to verify)
[ ] All pars clearable with Recruit (weakest free hero) at least 15% of the time
```

---

## Hero Recommendations Per Mission

For display on the mission intro screen (as hints, not rules):

| # | Recommended Heroes |
|---|---|
| 1 | Hunter, Warden |
| 2 | Gorilla, Hater |
| 3 | Bodyb, Hunter |
| 4 | Recruit, Warden |
| 5 | Gambler, Hater |
| 6 | Gorilla, Hater, Gambler |
| 7 | Bodyb, Hunter, Gambler |
| 8 | Bodyb, Hunter, Warden |
| 9 | Hunter, Warden |
| 10 | Gambler, Hater, Hunter |
| 11 | Hater, Gambler, Hunter |
| 12 | Gambler, Hunter, Recruit |
| 13 | Bodyb, Hunter, Gambler |
| 14 | Gorilla, Hater, Gambler |
| 15 | None (hidden) |

---

## What's New vs. Previous Versions

**Removed:**
- CPU opponent with difficulty levels (Easy/Medium/Hard)
- CPU focus modifiers (bounty/nine/pairs/balanced)
- Timer / time bank
- Hand A visibility
- Two-player rules

**Added:**
- CPU ghost for missions 6–7 (deterministic, teaching-only, not a real opponent)
- Mission 12 single-player streak tracking
- Explicit win conditions per mission

**Kept:**
- Scripted starting hands for missions 1–9
- Forced bounty suits
- Constraints for missions 8–9
- Config shifts for missions 13–14
- Teaching popups
- Recommended heroes
- Par escalation

This is the final spec. Your copilot has everything needed to implement the campaign as a single-player experience with two teaching-ghost missions.