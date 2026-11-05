# Achievements, Milestones, and Match History

Achievements are stored with the local player profile and evaluated by the
terminal progression flow. They do not change card resolution, scoring, the
deck, or AI decisions. See [economy.md](./economy.md) for the
broader progression economy.

## Progress and rewards

The profile schema is version 4. Loading older profiles initializes
the newer fields without dropping existing profile data. Profiles newer than
the supported schema fail to load. Match history is newest-first and bounded
to 50 records.

Each achievement is evaluated from lifetime profile statistics, the current
completed-game summary, or both. Tiered achievements grant every reward for
tiers crossed in one evaluation; their stored tier makes subsequent
evaluations idempotent. Each listed set grants one Skill Tree Point exactly
once when all of its members are complete. Collector III grants one
Pick-Any-Chroma token. Tier capstones can award badges.

The role identifiers remain the game’s existing suit values. Display names
and descriptions use **Scissors**, **Paper**, and **Rock**, never the current
skin colors. The gallery displays each role’s current color as a swatch.

## Achievement catalog

| ID | Achievement | Set | Requirement | Rewards |
|---|---|---|---|---|
| `sparse` | Sparse I | Sparse | Win with exactly two suits represented in your jail | 500 Marks |
| `sparse` | Sparse II | Sparse | Win with exactly two suits represented across both jails | 1,000 Marks |
| `sparse` | Sparse III | Sparse | Win with exactly one suit represented in your jail | 1,500 Marks |
| `sparse` | Sparse IV | Sparse | Win a best-of-three match where every game you won had at most two suits in your jail | 2,500 Marks, 50 Crowns |
| `efficient` | Efficient I | Efficient | Win with at least 7 prisoners | 500 Marks |
| `efficient` | Efficient II | Efficient | Win with at least 8 prisoners | 1,000 Marks |
| `efficient` | Efficient III | Efficient | Win with all 9 prisoners and no clashes | 2,500 Marks, 50 Crowns |
| `bounty_hunter` | Bounty Hunter | Bounty | Win with at least 3 prisoners matching the bounty role | 1,000 Marks |
| `bounty_denied` | Bounty Denied | Bounty | Win with no prisoners matching the bounty role | 1,500 Marks |
| `bounty_decided` | Bounty Decided | Bounty | Win a game where the non-bounty score does not beat the opponent, but the final score does | 2,500 Marks, 50 Crowns |
| `aggressor` | Aggressor I | Aggressor | Win 1 game as Seat X | 200 Marks |
| `aggressor` | Aggressor II | Aggressor | Win 10 games as Seat X | 1,000 Marks |
| `aggressor` | Aggressor III | Aggressor | Win 100 games as Seat X | 5,000 Marks, 100 Crowns, `badge_aggressor` |
| `defender` | Defender I | Defender | Win 1 game as Seat Y | 200 Marks |
| `defender` | Defender II | Defender | Win 10 games as Seat Y | 1,000 Marks |
| `defender` | Defender III | Defender | Win 100 games as Seat Y | 5,000 Marks, 100 Crowns, `badge_defender` |
| `streak_easy` | Easy Streak | Win Streaks | Win 3 consecutive matches against Easy CPU | 250 Marks |
| `streak_medium` | Medium Streak | Win Streaks | Win 3 consecutive matches against Normal CPU | 500 Marks |
| `streak_hard` | Hard Streak | Win Streaks | Win 3 consecutive matches against Hard CPU | 1,000 Marks |
| `streak_ranked` | Ranked Streak | Win Streaks | Win 3 consecutive Ranked matches | 1,500 Marks, 50 Crowns |
| `comeback` | Comeback I | Comeback | Win 1 game after trailing at the end of Round 2 | 500 Marks |
| `comeback` | Comeback II | Comeback | Win 10 such games | 1,500 Marks |
| `comeback` | Comeback III | Comeback | Win 50 such games | 4,000 Marks, 50 Crowns, `badge_resilient` |
| `first_clash` | First Clash | Ties | Tie an exchange | 100 Marks |
| `clash_master` | Clash Master | Ties | Tie at least 3 exchanges in one game | 500 Marks |
| `dead_heat` | Dead Heat | Ties | Finish a match with equal final scores | 1,500 Marks, 50 Crowns |
| `collector` | Collector I | Collector | Purchase 3 chromas of any rarity | 500 Marks |
| `collector` | Collector II | Collector | Purchase 3 rare chromas | 1,500 Marks |
| `collector` | Collector III | Collector | Purchase 3 super-rare or seasonal chromas | 1 Pick-Any-Chroma token |
| `long_march` | Long March | Dedication | Play 100 matches | 2,000 Marks |
| `hard_way` | The Hard Way | Dedication | Win 10 matches against the Hard CPU, including local Ranked matches | 2,500 Marks, 100 Crowns, `badge_determined` |
| `login_streak` | Login Streak I | Login Streak | Log in for the first time | 100 Marks |
| `login_streak` | Login Streak II | Login Streak | Log in 7 consecutive days | 750 Marks |
| `login_streak` | Login Streak III | Login Streak | Log in 30 consecutive days | 3,000 Marks, 50 Crowns, `badge_devoted` |
| `first_blood` | First Blood | — | Win your first match | 200 Marks |
| `both_sides` | Both Sides | — | Win a match as Seat X and a match as Seat Y | 300 Marks |
| `the_cycle` | The Cycle | — | Win a game with a lane won by suit doubling | 200 Marks |
| `master_both` | Master Both | — | Win 3 consecutive matches as Seat X and 3 consecutive matches as Seat Y | 1,500 Marks |
| `photo_finish` | Photo Finish | — | Win a match by exactly 1 point | 1,500 Marks, 50 Crowns |
| `no_not_like_that` | No, Not Like That | — | Lose a game with at least 9 points from one suit | 1,000 Marks, 50 Crowns |

## Sets

| Set | Members | Completion reward |
|---|---|---|
| Sparse | Sparse | 1 Skill Tree Point |
| Efficient | Efficient | 1 Skill Tree Point |
| Bounty | Bounty Hunter, Bounty Denied, Bounty Decided | 1 Skill Tree Point |
| Aggressor | Aggressor | 1 Skill Tree Point |
| Defender | Defender | 1 Skill Tree Point |
| Win Streaks | Easy, Medium, Hard, and Ranked Streak | 1 Skill Tree Point |
| Comeback | Comeback | 1 Skill Tree Point |
| Ties | First Clash, Clash Master, Dead Heat | 1 Skill Tree Point |
| Collector | Collector | 1 Skill Tree Point |
| Dedication | Long March, The Hard Way | 1 Skill Tree Point |
| Login Streak | Login Streak | 1 Skill Tree Point |

The detailed list defines 11 sets and therefore 11 Skill Tree Points. Although
the source specification mentions 12 sets in its introduction, it lists 11
sets and sets the tuning target to 11 Skill Tree Points; this implementation
follows the named sets and tuning target. Summing the listed achievement
rewards yields 53,300 Marks and 700 Crowns, above the specification’s
approximate targets of 52,000 Marks and 600 Crowns. The individual rewards are
kept as specified rather than silently adjusted.

## Gallery and integration

In the terminal UI, choose **Achievements** to browse categories, progress,
status, tier rewards, current role-color swatches, and set completion. Newly
earned achievements and sets are reported after a match or progression event.
Skill Points are the only currency for skill unlocks; the Shop sells one for
300 Crowns or ten for 2,400 Crowns.
Collector tokens can be redeemed from the chroma shop by selecting `pick` and
then an unowned chroma.

Match results, Camp Draws, chroma purchases, profile startup, and login streak
updates feed the achievement evaluator. The match result summaries are
progression data adapters; they do not change game rules. Achievement and set
definitions are also included in the JSON output from:

```powershell
GOWORK=off go -C backend run ./cmd/prisoners-of-war --dump-economy
```
