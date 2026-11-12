# Achievements, Milestones, and Match History

Achievements are stored in the local player profile and evaluated from match,
campaign, collection, and account progression. They do not change card
resolution, scoring, decks, or AI decisions. See [economy.md](./economy.md) for
the broader progression economy.

## Progress and rewards

Achievements are grouped into four categories:

- **Campaigns:** seven one-time campaign completion achievements.
- **Feats:** nine one-time match feats, each granting one Skill Point.
- **Per-Mode:** eight achievements with three cumulative win milestones each.
- **Meta:** eleven achievements with three cumulative or best-record milestones
  each.

Tiered achievements grant the rewards for every newly crossed tier. Stored
achievement state makes repeated evaluation idempotent. Campaign and Feat
rewards are also granted only once. Sets are gallery groupings and do not
grant rewards.

Rewards include Marks, Crowns, boxes, consumable tokens, and Skill Points.
Skill Points from achievements come only from the nine Feats.

## Tier reward ladder

All Per-Mode and Meta achievements use the same tier payout. Per-Mode capstone
boxes vary by game type; Meta capstones grant a Gold Box. Capstone token types
rotate across achievements.

| Level | Marks | Crowns | Capstone box | Capstone token |
|---|---:|---:|---|---|
| I | 400 | — | — | — |
| II | 800 | 1 | — | — |
| III | 1,200 | 2 | Per-Mode: Wooden or Silver; Meta: Gold | One token |

## Campaigns

Complete every mission in the named campaign for the one-time reward.
`camp_pow` currently maps to the Basic campaign.

| ID | Campaign | Reward |
|---|---|---|
| `camp_pow` | Prisoners of War (Basic) | 3,000 Marks + 10 Crowns |
| `camp_abandon` | Abandon | 3,000 Marks + 10 Crowns |
| `camp_ladder` | Ladder | 3,000 Marks + 10 Crowns |
| `camp_poker` | Poker | 3,000 Marks + 10 Crowns |
| `camp_rummy` | Rummy | 3,000 Marks + 10 Crowns |
| `camp_puzzler` | Puzzler | 3,000 Marks + 10 Crowns |
| `camp_tug_war` | Tug War | 3,000 Marks + 10 Crowns |

## Feats

Each Feat grants one Skill Point when its requirement is met.

| ID | Feat | Requirement |
|---|---|---|
| `sparse` | Sparse | Win with exactly two suits represented in your jail. |
| `efficient` | Efficient | Win with all 9 prisoners and no clashes. |
| `bounty_hunter` | Bounty Hunter | Win with at least 3 prisoners matching the bounty role. |
| `bounty_denied` | Bounty Denied | Win with no prisoners matching the bounty role. |
| `bounty_decided` | Bounty Decided | Win when your non-bounty score does not beat the opponent, but your final score does. |
| `comeback` | Comeback | Win a game after trailing at the end of Round 2. |
| `clash_master` | Clash Master | Tie at least 3 exchanges in one game. |
| `dead_heat` | Dead Heat | Finish a match with equal final scores. |
| `photo_finish` | Photo Finish | Win a match by exactly 1 point. |

## Per-Mode

Win each game type 1, 10, and 30 times. Each milestone pays the standard
ladder above; the box and token listed are awarded at level III.

| ID | Game type | Level III box | Level III token |
|---|---|---|---|
| `mode_main` | Prisoners of War | Silver | Undo |
| `mode_fog` | Fog of War | Silver | Undo |
| `mode_abandon` | Abandon | Wooden | Restart |
| `mode_poker` | Poker | Silver | Restart |
| `mode_rummy` | Rummy | Wooden | Restart |
| `mode_puzzler` | Puzzler | Wooden | Replace Card |
| `mode_ladder` | Ladder | Wooden | Skip Level |
| `mode_tug-war` | Tug War | Wooden | Skip Level |

## Meta

Each milestone pays the standard ladder above; every level III milestone also
grants one Gold Box and the listed token.

| ID | Achievement | I / II / III requirements | Level III token |
|---|---|---|---|
| `chroma_collector` | Chroma Collector | Own 5 / 20 / 50 chromas | Replace Card |
| `lucky_draw` | Lucky Draw | Open 5 / 20 / 50 boxes | Skip Level |
| `win_streak_humans` | Win Streak (Humans) | Win 3 / 6 / 12 consecutive matches against human opponents | Undo |
| `login_streak` | Login Streak | Log in on 1 / 10 / 30 consecutive days | Restart |
| `long_march` | Long March | Win 20 / 50 / 100 matches | Undo |
| `hero_variety` | Hero Variety | Win at least 10 matches while winning with 3 / 6 / 10 different heroes | Replace Card |
| `branch_master` | Branch Master | Fully unlock 1 / 3 / 6 Skill Tree branches | Skip Level |
| `tinkerer` | Tinkerer | Use 10 / 50 / 200 consumables | Restart |
| `marathon` | Marathon | Play 10 / 30 / 60 matches in a single week | Undo |
| `mode_tourist` | Mode Tourist | Win in 2 / 4 / 7 different game types | Replace Card |
| `ranked_regular` | Ranked Regular | Play 10 / 50 / 200 ranked matches | Skip Level |

## Sets

The gallery groups achievements into Campaigns, Bounty Feats, Other Feats,
Per-Mode, and Meta. Completing a set grants no additional reward.

## Totals

These totals assume every achievement tier is earned.

| Source | Marks | Crowns | Skill Points | Boxes | Tokens |
|---|---:|---:|---:|---:|---:|
| Campaigns (7) | 21,000 | 70 | — | — | — |
| Feats (9) | — | — | 9 | — | — |
| Per-Mode (8 × 3 tiers) | 19,200 | 24 | — | 8 | 8 |
| Meta (11 × 3 tiers) | 26,400 | 33 | — | 11 | 11 |
| **Total** | **66,600** | **127** | **9** | **19** | **19** |

## Gallery and integration

The browser and terminal galleries show categories, requirements, progress,
tier rewards, and set groupings. Rewards are credited automatically; no claim
action is required.

Match outcomes update mode wins, total wins, hero wins, ranked matches,
human win streaks, and weekly match counts. Opening Wooden, Silver, Gold, or
Hero Boxes advances Lucky Draw; using Undo, Restart, Skip Level, or Replace
Card advances Tinkerer. Login events update Login Streak. These progression
updates do not alter game rules.

Achievement definitions are included in the economy JSON output:

```powershell
go -C backend run ./cmd/prisoners-of-war --dump-economy
```
