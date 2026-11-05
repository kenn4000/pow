# Hero Acquisition and Campaigns

## Starter hero and roster

New profiles own only The Recruit, who is also the default hero. Recruit is
always selectable and is excluded from Hero Variety's 3/6/10 collection
thresholds. Existing profiles keep their previously unlocked heroes and gain
Recruit during the v5-to-v6 profile migration.

Warden, Hunter, and the Turtle unlock when Basic Training is completed. The other
heroes are acquired from Hero Boxes; any non-Recruit hero can also be purchased
individually for 300, 400, or 500 Crowns, depending on the hero. Marks cannot
be converted to Crowns.

## Campaigns

Choose **[9] Campaigns** from the terminal landing page. Each campaign has five
missions. A mission can be attempted until won; losses award no Marks and do
not block retries.

### Basic Training

All missions use The Recruit for both players. Each victory awards 200 Marks.
Completing all five awards another 1,000 Marks, Warden, Hunter, the Turtle, and
`badge_basic_training` (2,000 Marks total).

1. First Steps — deck, commits, and exchanges
2. The Bounty — bounty suits and scoring
3. The Cycle — suit cycle and doubling
4. The Clash — tied battles
5. The Reckoning — round weights

### Advanced Training

Unlocks after Basic Training. Choose Warden, Hunter, or the Turtle for each
mission. Each victory awards 400 Marks. Completing all five awards another
2,000 Marks, one Hero Box, and `badge_advanced_training` (4,000 Marks total).

1. The Hunter's Game — AI Hunter
2. The Treasurer — AI Treasurer
3. The Gorilla — AI Gorilla
4. The Gambler — AI Gambler
5. The Hater — AI Hater

The Curator skill increases mission Marks by 50%. Campaign Freebie makes the
first mission of each campaign cost no energy.

## Hero Boxes and direct purchase

In Shop, a Hero Box costs 8,000 Marks; Hero Broker reduces it to 7,200. Each
of the ten non-Recruit heroes has a 10% chance. If the selected hero is already
owned, the base reward is 50% of that hero's listed Crown purchase price:

| Crown price | Heroes | Base duplicate reward |
| ---: | --- | ---: |
| 300 | Warden, Hunter, Turtle, Bureaucrat | 150 Crowns |
| 400 | Treasurer, Gorilla, Diplomat | 200 Crowns |
| 500 | Gambler, Bodybuilder, Hater | 250 Crowns |

Lucky Duplicate raises duplicate compensation by 25%; Premium Duplicate
(requires Lucky Duplicate) raises it by 50% instead. Fractional Crowns are
rounded down. In Shop, `buy-hero` lists every unowned hero at its listed
300/400/500 Crown price. Paid purchases require confirmation.

Achievements award unopened Hero Boxes: Long March awards one, and badge
rewards include one additional box.

## New achievement groups

- **Bounty Connoisseur:** win with each exact active Bounty value from -3 to +3.
  Tier rewards range from 200 to 600 Marks.
- **Favored Aficionado:** win with each exact Favored value from -1 to +2.
  Tier rewards range from 400 to 600 Marks.
- **Shift Whisperer:** win with each exact Shift value from 0 to +3.
  Tier rewards range from 400 to 600 Marks.
- **Hero Variety:** own 3, 6, and 10 distinct non-Recruit heroes. Rewards are
  500 Marks, 2,000 Marks, and 5,000 Marks + 500 Crowns + the Completionist
  badge. Each completed group awards one Skill Tree Point.

Modifier tiers are independently earned: winning with a high modifier does
not automatically unlock tiers for modifiers that were not used.

## Skill tree collection branch

The six-branch, 27-node Skill Tree and its full reward effects are documented
in [economy.md](./economy.md#skills-shop-and-chromas). The Collection branch
contains Hero Broker (10% fewer Marks for Hero Boxes), Lucky Duplicate (+20%
duplicate Crown refunds), and Premium Duplicate (an additional +30% refund).
Unlocks require Skill Points: one for T1, two for T2, and three for T3. The
Shop sells one Skill Point for 300 Crowns or ten for 2,400 Crowns.

Conductor tuning is applied to the combined hero modifiers before the normal
modifier envelope clamp. It changes the shared match modifiers and is not
persisted between matches. Hero History, Hero Preview, campaign progress, and
Hero Box inventory are saved in profile schema v6.

## Terminal commands

- Landing page: **[9] Campaigns**
- Shop: `hero-box` to open a Marks-purchased box, `free-hero-box` to open an
  awarded box, and `buy-hero` for direct Crown purchases
- Heroes: choose an unlocked default hero; locked heroes show their campaign
  or acquisition hint
