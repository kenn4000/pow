ABANDON — CAMPAIGN MISSION SPECS (15 missions)
======================================================================
Conventions:
- All cards written as (SUIT, FACE). SUITS: R=Rock, P=Paper, S=Scissors.
- FACE is 1..9.
- "Player hand" = the 5 (or fewer) private cards for the human.
- "Opponent hand" = opponent's private cards (identical to player unless specified).
- Bounty suit is FORCED per mission (overrides top-card flip).
- Standard board size 16 unless specified. Standard hand 5 unless specified.
- "Pairs" = matchups per player this mission.
- "Rounds" = 1 (single round) or 2 (seat-swap match).
- Hero: player free choice unless "HeroLock" specified.
- OpponentAlgo: NONE | GREEDY | BOUNTY_FOCUSED | TIE_PERFECT | ANTI_CONCENTRATE | ANTI_SPREAD | SMART | SMART_HIGH
- "Slack" describes how forgiving the mission is.
======================================================================

----------------------------------------------------------------------
MISSION 1 — "First Blood"
----------------------------------------------------------------------
Teaches:      Matchup basics — dominance, lower-value card is captured.
Goal:         Capture at least 1 card. (Any jail entry wins.)
Difficulty:   1/5
Board size:   6
Board cards:  (R,5) (R,3) (P,7) (S,4) (S,8) (P,2)
Player hand:  (R,6) (P,9) (S,2)     [3 cards]
BountySuit:   R
Pairs:        1
Rounds:       1
Opponent:     NONE (solo)
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        VERY HIGH. The only winning move is (R,6) vs (S,4) — R dominates S,
              R value = 2*(6+0)+0 = 12 vs S value = 4. S4 is captured.
              All other moves either tie, capture the player's card, or
              capture nothing. The player literally cannot lose.

----------------------------------------------------------------------
MISSION 2 — "Mirror"
----------------------------------------------------------------------
Teaches:      Same-suit matchups — no dominance, LOWER face is captured.
Goal:         Capture a specific card OR score >= 3.
Difficulty:   1/5
Board size:   6
Board cards:  (R,3) (R,7) (P,5) (P,9) (S,1) (S,6)
Player hand:  (R,2) (P,4) (S,8)     [3 cards]
BountySuit:   P
Pairs:        1
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        VERY HIGH. The intended play: (R,2) vs (R,7) → same suit,
              lower (R,2) is CAPTURED — but you want to CAPTURE the enemy, so
              instead play (R,2) vs... hmm. Same-suit: LOWER is captured.
              To capture (R,7) you need a LOWER face — but your (R,2) IS lower,
              so you'd be captured. Lesson: same-suit means you CAN'T
              dominate. The correct play is to avoid same-suit and use
              (P,4) vs (S,1): P beats S? No — S beats P. So S,1 dominates:
              S value = 2*(1+0)=2, P,4 = 4. S,1 CAPTURED. Good.
              Any non-same-suit move where you win. The mission teaches
              "same-suit is NOT a free win."

----------------------------------------------------------------------
MISSION 3 — "The Tie"
----------------------------------------------------------------------
Teaches:      Ties capture NOTHING (both discarded). Ties are a waste of a turn.
Goal:         Capture at least 1 card despite a tempting tie.
Difficulty:   1/5
Board size:   6
Board cards:  (R,5) (S,5) (P,3) (P,8) (R,1) (S,9)
Player hand:  (R,4) (P,2) (S,7)     [3 cards]
BountySuit:   S
Pairs:        1
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        HIGH. Trap: (R,4) vs (S,5) → R dominates S: R val = 8, S val = 5.
              S5 captured (good, not a tie). The actual TIE trap is
              (R,5) on the board vs an opponent... but in solo you only pick
              from YOUR hand + board. The tie is available if you pick
              (S,5) with (S,5)? Impossible (only one S5). So the tie here is:
              player plays (R,4) vs (R,5) same-suit → R4 lower → R4 captured.
              Then (R,5) sits on the board, unclaimed — a "tie" would be
              if the player had a 5 to match. Lesson is conceptual:
              "If faces are equal in a same-suit matchup, nothing is captured."
              Revise: add a guaranteed tie option —
              Player hand also contains (P,5). Then (P,5) vs (S,5): P vs S,
              S dominates: S val = 2*(5+0) = 10, P val = 5. S5 captured (not a tie).
              True tie: same suit same face — impossible with distinct cards.
              So Mission 3 uses a MODIFIED rule: "if dominant value ==
              subordinate value in cross-suit, BOTH discarded." The player
              should see one such matchup is available and avoid it.
              Board add: (S,4). Hand add: (R,2). Then (R,2) vs (S,4):
              R val = 2*2=4, S val=4. TIE! Both discarded. The player must
              NOT choose this (it wastes a turn). Instead (R,2) vs (P,3):
              P dominates R: P val = 6, R val = 2. R2 captured.
Slack:        HIGH. Teaches "ties exist, avoid them."

----------------------------------------------------------------------
MISSION 4 — "Count and Peak"
----------------------------------------------------------------------
Teaches:      Jail scoring = count × highest face (per suit). Multiple cards
              in one suit beat one high card in another suit.
Goal:         Score >= 12.
Difficulty:   2/5
Board size:   8
Board cards:  (R,2) (R,3) (R,5) (P,9) (P,7) (S,4) (S,6) (S,2)
Player hand:  (R,4) (R,6) (P,3) (S,1)   [4 cards]
BountySuit:   P
Pairs:        2
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        MODERATE. Two captures possible:
              Route A: capture (R,3) and (R,5) → Rock jail = {3,5}, score = 2*5 = 10.
              Route B: capture (P,9) → Paper jail = {9}, score = 9.
              Route A scores 10 + maybe another suit. Need 12. Player must
              combine: 2 Rock (10) + 1 small Scissors (say S,2 → 2) = 12.
              Teaches "count in the same suit."

----------------------------------------------------------------------
MISSION 5 — "The Bounty"
----------------------------------------------------------------------
Teaches:      Bounty-suit cards give +1+B in jail. Picking a HIGH-B hero makes
              bounty captures worth much more.
Goal:         Score >= 15.
Difficulty:   2/5
Board size:   8
Board cards:  (R,5) (R,8) (P,9) (P,3) (S,7) (S,4) (S,2) (P,6)
Player hand:  (R,6) (R,7) (S,3) (S,5)   [4 cards]
BountySuit:   R
Pairs:        2
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     none (any hero; Bodybuilder B=3 makes it trivial, Recruit B=0
              requires careful play)
Slack:        MODERATE. With B=0, best play scores ~13. With B=3, same play
              scores ~18. The mission is winnable by Recruit (needs 3 captures
              across Rock+S) but EASY for Bodybuilder. Teaches "Bounty matters."

----------------------------------------------------------------------
MISSION 6 — "Don't Abandon"
----------------------------------------------------------------------
Teaches:      Abandon penalty = (face + S) + (1+B if bounty) per unused private
              card. You MUST use your high cards or pay for them.
Goal:         Score >= 20 AND at most 1 abandoned private card.
Difficulty:   2/5
Board size:   10
Board cards:  (R,3) (R,7) (R,1) (P,4) (P,8) (P,2) (S,5) (S,9) (S,3) (P,6)
Player hand:  (R,9) (R,2) (P,5) (S,7) (S,4)    [5 cards]
BountySuit:   S
Pairs:        2
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        MODERATE. Hand size 5, pairs 2 → 4 cards used max, 1 forced
              abandoned. Player must choose WHICH to abandon carefully.
              Abandoning (R,9) costs (9+0)+(0 if not bounty) = 9 points — huge.
              Abandoning (R,2) costs 2. The mission forces the lesson.

----------------------------------------------------------------------
MISSION 7 — "Flip and Find"
----------------------------------------------------------------------
Teaches:      The free flip (1 board card per turn) reveals information.
Goal:         Score >= 18.
Difficulty:   2/5
Board size:   8
Board cards:  (R,5) (R,9) (P,3) (P,7) (S,2) (S,8) (P,4) (S,6)
Board visibility at start:
              Bounty suit = R. Visible bounty: (R,5) only.
              (R,9) is the hidden top bounty (highest R face). Face-down.
              All non-bounty cards: face-down.
              So at start: 1 visible (R,5), 7 face-down.
Player hand:  (R,4) (S,3) (P,6) (S,1)   [4 cards]
BountySuit:   R
Pairs:        2
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        MODERATE. Best capture is (R,9) — the hidden top bounty.
              Player must FLIP the (R,9) to find it, then capture it with
              a higher R or a dominant suit. Or gamble on it. The lesson:
              "Flip before committing; the best card is often hidden."

----------------------------------------------------------------------
MISSION 8 — "Blind Gamble"
----------------------------------------------------------------------
Teaches:      Sometimes gambling on a face-down card is the optimal play.
Goal:         Score >= 25.
Difficulty:   2/5
Board size:   10
Board cards:  (R,7) (R,2) (R,4) (P,5) (P,8) (P,1) (S,3) (S,9) (S,5) (P,3)
Board visibility at start:
              BountySuit = S. Visible bounty: (S,3), (S,5) — 2 visible.
              (S,9) is the hidden top bounty.
              All non-bounty: face-down (7 cards).
Player hand:  (R,9) (P,2) (S,4) (R,1)   [4 cards]
BountySuit:   S
Pairs:        2
Rounds:       1
Opponent:     NONE
OpponentAlgo: NONE
HeroLock:     Recruit
Slack:        MODERATE. All visible bounty (S3, S5) are LOW. Capturing them
              scores 3+5 = 8. The 9 is hidden. The player must either flip it
              (if a flip comes) or gamble on a face-down card hoping for high
              non-bounty faces. Expected value of gambling > visible plays.
              Teaches "gamble when the visible board is thin."

----------------------------------------------------------------------
MISSION 9 — "The Mirror Match"
----------------------------------------------------------------------
Teaches:      2P basics — reading the opponent, alternating turns.
Goal:         Beat the opponent (score higher).
Difficulty:   2/5
Board size:   12
Board cards:  RANDOM per game (dealt from deck; bounty suit determined by
              flipped top card).
Player hand:  RANDOM per game (5 cards, guaranteed 1 bounty).
Opponent hand: MIRROR of player hand (same 5 cards).
BountySuit:   RANDOM (top card).
Pairs:        2 each
Rounds:       1
Opponent:     GREEDY CPU (no lookahead, no expectation; best immediate capture).
OpponentAlgo: GREEDY
Turn order:   P1 = player. Snake NOT used — strict alternation (P1, P2, P1, P2).
Slack:        HIGH. The greedy opponent is weak; any competent player wins.

----------------------------------------------------------------------
MISSION 10 — "Snake Eyes"
----------------------------------------------------------------------
Teaches:      Snake turn order — grabbing the best bounty before the opponent.
Goal:         Win AND capture at least 2 bounty-suit cards.
Difficulty:   3/5
Board size:   12
Board cards:  PRESET (not random). Ensure bounty cards are clustered:
              (R,9) (R,8) (R,7) (R,6) (P,4) (P,2) (P,5) (S,3) (S,1) (S,6) (P,3) (S,4)
              BountySuit = R. Hidden top bounty: (R,9) is highest R → face-down.
              Visible R: (R,8) (R,7) (R,6).
Player hand:  PRESET: (P,8) (P,7) (S,5) (S,8) (R,2)  [5 cards]
Opponent hand: MIRROR (same cards).
BountySuit:   R (forced)
Pairs:        3 each
Rounds:       1
Turn order:   SNAKE (P1, P2, P2, P1, P1, P2 for pairs=3: 6 turns → P1 leads
              blocks 1, P2 leads block 2, P1 leads block 3 → P1 has 2 leads,
              P2 has 1 lead. Total: each player gets 3 turns.)
Opponent:     BOUNTY_FOCUSED CPU. Algorithm: greedy, but +100 bonus for any move
              that captures a bounty-suit card.
OpponentAlgo: BOUNTY_FOCUSED
Slack:        MODERATE. Player must grab the (R,8) and (R,7) early before the
              opponent does. If the opponent grabs them first, the player must
              find another route. Teaches "initiative = bounty priority."

----------------------------------------------------------------------
MISSION 11 — "Two Rounds"
----------------------------------------------------------------------
Teaches:      Seat swap — 2 rounds, each player leads one round, balanced match.
Goal:         Win the MATCH (sum of round 1 score + round 2 score, higher wins).
Difficulty:   3/5
Round 1:
  Board size:   14
  Board cards:  RANDOM but BIASED: more Rock cards (high bounty) than average.
  Player hand:  RANDOM (5, 1+ bounty).
  Opponent hand: MIRROR.
  BountySuit:   R (forced)
  Pairs:        3 each
  Turn order:   P1 = player leads (alternation, not snake).
Round 2:
  Board size:   14
  Board cards:  RANDOM (fresh deal; different board).
  Player hand:  RANDOM (fresh).
  Opponent hand: MIRROR.
  BountySuit:   RANDOM (different from round 1 ideally).
  Pairs:        3 each
  Turn order:   P1 = OPPONENT leads (swapped).
Opponent:     GREEDY
OpponentAlgo: GREEDY
Rounds:       2 (seat swap)
Slack:        MODERATE. Each round is P1-favored (seat); the swap balances.
              Teaches "seat swap = fair match."

----------------------------------------------------------------------
MISSION 12 — "All-In"
----------------------------------------------------------------------
Teaches:      Suit concentration — stacking one suit dominates scoring.
Goal:         Score >= 40.
Difficulty:   4/5
Board size:   14
Board cards:  PRESET (Rock-heavy):
              (R,9) (R,8) (R,7) (R,6) (R,5) (R,4) (P,3) (P,5) (P,1) (P,7)
              (S,2) (S,4) (S,8) (S,6)
              BountySuit = R. (R,9) hidden (top bounty).
Player hand:  PRESET: (P,9) (P,8) (S,9) (S,7) (R,3)  [5 cards]
Opponent hand: RANDOM (not mirror — opponent gets random 5).
BountySuit:   R
Pairs:        3 each
Rounds:       1
Turn order:   SNAKE.
Opponent:     ANTI_CONCENTRATE CPU — avoids stacking its own jail in one suit,
              but does NOT actively prevent the player from doing so.
OpponentAlgo: ANTI_CONCENTRATE
Slack:        LOW. Target 40 is HIGH. Requires capturing 3-4 Rock cards.
              Rock jail (e.g. {R5, R6, R7, R8}) = 4 × 8 = 32, plus another suit.
              Need spread or careful stacking. Teaches "concentration is powerful."

----------------------------------------------------------------------
MISSION 13 — "Spread"
----------------------------------------------------------------------
Teaches:      Suit spread — jail in all 3 suits beats a single pile.
Goal:         Score >= 30 AND have jail in all 3 suits.
Difficulty:   4/5
Board size:   14
Board cards:  PRESET (balanced):
              (R,7) (R,2) (P,8) (P,3) (P,5) (S,9) (S,4) (S,6) (R,5) (P,1)
              (S,2) (R,4) (P,6) (S,3)
              BountySuit = P. (P,8) hidden top bounty.
Player hand:  PRESET: (R,9) (R,6) (P,9) (S,7) (S,8)  [5 cards]
Opponent hand: RANDOM.
BountySuit:   P
Pairs:        3 each
Rounds:       1
Turn order:   SNAKE.
Opponent:     ANTI_SPREAD CPU — tries to grab high cards before the player
              can spread them across suits.
OpponentAlgo: ANTI_SPREAD
Slack:        LOW. Target 30 + all 3 suits forces spread. Player needs
              ~3 captures in each suit. Teaches "spread beats concentration
              when the board is balanced."

----------------------------------------------------------------------
MISSION 14 — "Tempo"
----------------------------------------------------------------------
Teaches:      Timing — lock good visible captures early, gamble late when
              visible options dry up.
Goal:         Score >= 35.
Difficulty:   4/5
Board size:   16
Board cards:  PRESET: designed so early turns have STRONG visible captures,
              late turns have WEAK visible captures (forcing a gamble):
              Early-visible (bounty suit = R, high faces):
              (R,9) (R,8) (P,7) (P,6) (S,5) (S,4) (P,3) (P,2)
              Late-hidden (face-down, low faces):
              (R,3) (R,2) (P,5) (P,4) (S,9) (S,8) (S,7) (S,6)
              BountySuit = R. (R,9) hidden top bounty.
Player hand:  PRESET: (P,9) (P,8) (S,3) (S,2) (R,4)  [5 cards]
Opponent hand: RANDOM.
BountySuit:   R
Pairs:        4 each
Rounds:       1
Turn order:   SNAKE.
Opponent:     SMART CPU (full algorithm, ALPHA=0.3).
OpponentAlgo: SMART
Slack:        LOW. Early turns: grab (R,8) and (R,7). Late turns: visible is
              thin; the player must gamble on face-down high cards. Teaches
              "adapt your play to the stage of the game."

----------------------------------------------------------------------
MISSION 15 — "Abandon"
----------------------------------------------------------------------
Teaches:      Full game synthesis. Standard rules.
Goal:         Win the MATCH (sum of two rounds with seat swap).
Difficulty:   5/5
Round 1:
  Board size:   16
  Board cards:  RANDOM.
  Player hand:  RANDOM (5).
  Opponent hand: MIRROR (same 5 as player).
  BountySuit:   RANDOM.
  Pairs:        6 each
  Turn order:   SNAKE. Player leads first block.
Round 2:
  Board size:   16
  Board cards:  RANDOM (fresh).
  Player hand:  RANDOM (fresh).
  Opponent hand: MIRROR.
  BountySuit:   RANDOM.
  Pairs:        6 each
  Turn order:   SNAKE. Opponent leads first block.
Opponent:     SMART_HIGH CPU (full algorithm, ALPHA=0.5 — more future-focused).
OpponentAlgo: SMART_HIGH
Rounds:       2 (seat swap)
Slack:        NONE. Standard game, smart opponent, 6 pairs. Player must use
              everything learned: matchup basics, same-suit, ties, jail scoring,
              bounty bonus, abandon penalty, flips, gambling, snake, seat swap,
              concentration vs. spread, tempo, and reading the opponent.

======================================================================
END OF MISSION SPECS
======================================================================

CAMPAIGN SUMMARY:
| Mission | Teaches | Goal | Opponent |
|---------|---------|------|----------|
| 1 | Matchup basics | Capture 1 card | None |
| 2 | Same-suit (lower captured) | Capture 1 card | None |
| 3 | Ties capture nothing | Capture 1 despite trap | None |
| 4 | Jail count × highest | Score 12+ | None |
| 5 | Bounty bonus | Score 15+ | None |
| 6 | Abandon penalty | Score 20+, ≤1 abandoned | None |
| 7 | Free flip | Score 18+ | None |
| 8 | Gambling on hidden | Score 25+ | None |
| 9 | 2P basics | Beat opponent | GREEDY |
| 10 | Snake initiative | Win + 2 bounty captures | BOUNTY_FOCUSED |
| 11 | Seat swap | Win match (2 rounds) | GREEDY |
| 12 | Suit concentration | Score 40+ | ANTI_CONCENTRATE |
| 13 | Suit spread | Score 30+ + all 3 suits | ANTI_SPREAD |
| 14 | Tempo | Score 35+ | SMART |
| 15 | Full synthesis | Win match | SMART_HIGH |