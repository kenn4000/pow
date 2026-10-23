# Heroes and Reckoning

Heroes are selected before a match. Their contributions combine into the
match's Bounty, Shift, and Favored modifiers. A fresh Reckoning roll is made
for every game, including each game in a best-of-three match.

## Roster

| Hero | Bounty | Shift | Favored | Unlock |
| --- | ---: | ---: | ---: | --- |
| The Recruit | 0 | 0 | 0 | Always owned |
| The Warden | +1 | 0 | +1 | Basic Training or 1,200 Crowns |
| The Hunter | +2 | 0 | 0 | Basic Training or 1,200 Crowns |
| The Treasurer | -2 | 0 | +1 | Hero Box or 1,200 Crowns |
| The Ascendant | 0 | +2 | 0 | Hero Box or 1,200 Crowns |
| The Anchor | -1 | 0 | -1 | Basic Training or 1,200 Crowns |
| The Gambler | +2 | +1 | 0 | Hero Box or 1,200 Crowns |
| The Diplomat | -2 | +1 | +1 | Hero Box or 1,200 Crowns |
| The Ascetic | -1 | +1 | -1 | Hero Box or 1,200 Crowns |
| The Herald | +3 | 0 | -1 | Hero Box or 1,200 Crowns |
| The Hater | +1 | 0 | +1 | Hero Box or 1,200 Crowns |

The Hater flips the other hero's Bounty contribution only; it never changes
Shift or Favored. The Hero Box acquisition rules and Campaign missions are
documented in [README-heroes-acquisition.md](./README-heroes-acquisition.md).

## Combining heroes

Add both heroes' contributions, then clamp Bounty to -3..+3, Shift to 0..+3,
and Favored to -1..+2. The default Recruit-vs-Recruit matchup receives a +1
Bounty. If one hero is the Hater, negate the other hero's Bounty before adding.
If both heroes are Haters, negate both Bounty contributions.

Examples:

- Hunter + Hater: `Bounty = -2 + 1 = -1`.
- Warden + Hater: Warden's `+1` is flipped, so `Bounty = -1 + 1 = 0`;
  Favored remains `+1 + +1 = +2`.
- Hater + Hater: `Bounty = -1 + -1 = -2`; Favored remains `+2`.

## Applying modifiers

- **Combat:** `base = max(1, face + Shift)`. If the card's suit beats its
  opponent's suit, its value is `max(1, base * 2 + Favored)`; otherwise its
  value is `base`.
- **Scoring face:** `max(1, face + Shift + Bounty)` for cards matching the
  bounty suit; other cards use `max(1, face + Shift)`.
- **Per-suit score:** multiply the maximum effective scoring face captured
  in that suit by the sum of the Reckoning weights for each captured card's
  round. The total is the sum of the three suit scores.

## Reckoning

One of four round-weight sets is rolled per game:

| Weights (Rounds 1 / 2 / 3) | Probability |
| --- | ---: |
| 1 / 1 / 1 | 40% |
| 1 / 1 / 2 | 25% |
| 1 / 1 / 3 | 20% |
| 1 / 2 / 3 | 15% |

An even `1 / 1 / 1` roll has no round weighting. The modifier HUD displays
the selected hero pair, nonzero modifiers, bounty suit, and round weights.

## Selection and progression

- CPU and Ranked terminal matches use the profile's default hero. New profiles
  start with The Recruit. The AI counter-picks from the player's unlocked
  heroes using the last-opponent hint, or picks randomly when no hint is
  available.
- Local Human-vs-Human matches ask each player to select an unlocked hero.
- The terminal `[8] Heroes` screen shows hero contributions and records,
  sets a default, and shows hints for locked heroes. Hero play/win counts are
  stored in the profile.
- Browser matches use the profile's default hero for the player unless a
  different unlocked hero is selected before starting. Human-vs-Human allows
  both local players to select a hero.

## Hero matchup simulator

Run every unordered hero pair (including mirror matches) with greedy-vs-greedy
and greedy-vs-random games. The 11-hero roster has 66 pairings:

```powershell
go run . --sim-heroes 100
```

The report flags pairings against these targets:

- Greedy-vs-greedy P1 win rate: 46–54%.
- Greedy-vs-random P1 win rate: at least 65%.
- Mean absolute score difference: at most 42.
- Stall rate (rounds ending with one or zero captures): at most 5%.
- Clash rate: at most 6% of battles.

The regular `--sim N` simulation also rolls Reckoning once per game.
