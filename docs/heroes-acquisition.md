# Hero Acquisition and Campaigns

## Starter hero and roster

New profiles own only The Recruit, who is also the default hero. Recruit is
always selectable and is excluded from Hero Variety's 3/6/10 collection
thresholds. Existing profiles keep their previously unlocked heroes and gain
Recruit during the v5-to-v6 profile migration.

Warden, Hunter, and Anchor unlock when Basic Training is completed. The other
heroes are acquired from Hero Boxes; any non-Recruit hero can also be purchased
individually for 1,200 Crowns. Marks cannot be converted to Crowns.

## Campaigns

Choose **[9] Campaigns** from the terminal landing page. Each campaign has five
missions. A mission can be attempted until won; losses award no Marks and do
not block retries.

### Basic Training

All missions use The Recruit for both players. Each victory awards 200 Marks.
Completing all five awards another 1,000 Marks, Warden, Hunter, Anchor, and
`badge_basic_training` (2,000 Marks total).

1. First Steps — deck, commits, and exchanges
2. The Bounty — bounty suits and scoring
3. The Cycle — suit cycle and doubling
4. The Clash — tied battles
5. The Reckoning — round weights

### Advanced Training

Unlocks after Basic Training. Choose Warden, Hunter, or Anchor for each
mission. Each victory awards 400 Marks. Completing all five awards another
2,000 Marks, one Hero Box, and `badge_advanced_training` (4,000 Marks total).

1. The Hunter's Game — AI Hunter
2. The Treasurer — AI Treasurer
3. The Ascendant — AI Ascendant
4. The Gambler — AI Gambler
5. The Hater — AI Hater

The Curator skill increases mission Marks by 50%. Campaign Freebie makes the
first mission of each campaign cost no energy.

## Hero Boxes and direct purchase

In Shop, select `hero-box` to open a Hero Box. The regular cost is 3,000 Marks;
Hero Broker reduces it to 2,700. Any free boxes from campaigns or achievements
are opened before Marks are spent.

| Tier | Chance | Possible heroes | Duplicate reward |
| --- | ---: | --- | ---: |
| Early | 50% | Treasurer, Ascendant | 100 Crowns |
| Mid | 30% | Gambler, Diplomat, Ascetic | 150 Crowns |
| Late | 20% | Herald, Hater | 200 Crowns |

Lucky Duplicate raises duplicate compensation by 25%; Premium Duplicate
(requires Lucky Duplicate) raises it by 50% instead. Fractional Crowns are
rounded down. In Shop, `buy-hero` lists every unowned hero for direct purchase
at 1,200 Crowns each. Both paid actions require confirmation.

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

## Skill tree additions

The current project had nine skill nodes before this expansion. Per the agreed
scope, the ten specified additions bring the tree to 19 nodes:

| Node | Cost | Effect |
| --- | ---: | --- |
| Conductor I | 500 Crowns | Tune one modifier by +1 or -1 |
| Conductor II | 1,200 Crowns | Tune two different modifiers; requires Conductor I |
| Hero Broker | 300 Crowns | Hero Boxes cost 10% fewer Marks |
| Curator | 400 Crowns | Campaign mission Marks +50% |
| Lucky Duplicate | 400 Crowns | Duplicate Crowns +25% |
| Premium Duplicate | 800 Crowns | Duplicate Crowns +50%; requires Lucky Duplicate |
| Campaign Freebie | 300 Crowns | First mission in each campaign costs no energy |
| Daily Rations | 500 Crowns | Gain 2 energy on first login each day |
| Hero History | 400 Crowns | Show the opponent's last two hero picks |
| Hero Preview | 600 Crowns | Preview modifiers during hero selection; requires Hero History |

Conductor tuning is applied to the combined hero modifiers before the normal
modifier envelope clamp. It changes the shared match modifiers and is not
persisted between matches. Hero History, Hero Preview, campaign progress, and
Hero Box inventory are saved in profile schema v6.

## Terminal commands

- Landing page: **[9] Campaigns**
- Shop: `hero-box` to open a free or Marks-purchased box; `buy-hero` for direct
  Crown purchases
- Heroes: choose an unlocked default hero; locked heroes show their campaign
  or acquisition hint
