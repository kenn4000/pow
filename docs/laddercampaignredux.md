# Ladder — 15-Mission Campaign Design

Here's a full campaign spec you can hand to Copilot alongside the base game. Each mission has a **fixed scenario** (wild face, bounty suit, pre-scripted hands, hand sizes), a **goal**, **hero pick constraints**, and a **CPU policy**. Difficulty ramps from "learn the chain" to "beat a smart opponent with a bad hand."

I'll structure each mission as:

- **Title & premise** (one line)
- **Fixed parameters** (wild face, bounty suit, hand sizes, pre-scripted hands if any)
- **Player hero** (locked / free / restricted)
- **CPU policy** (which algorithm)
- **Goal** (win condition, sometimes with a sub-goal like "win in ≤ N turns" or "win without passing")
- **What it teaches** (the design intent)

---

## Campaign Arc Overview

| # | Title | Teaches | CPU | Difficulty |
|---|---|---|---|---|
| 1 | First Chain | Basic legal play, climbing | Random-legal | ★ |
| 2 | The Doubling Rule | RPS ×2 effective values | Greedy-lowest | ★ |
| 3 | Wild Awakening | Wild cards, wild-vs-wild | Greedy-lowest | ★★ |
| 4 | Favor of the Suit | Favor multiplier (positive) | Greedy-lowest | ★★ |
| 5 | Burden of Favor | Negative favor inversion | Greedy-lowest | ★★ |
| 6 | Shifted Ground | Shift adds to faces | Greedy-lowest | ★★ |
| 7 | The Draft Table | Draft swap (quality filter) | Greedy-lowest | ★★ |
| 8 | Initiative | Bounty-suit leader selection | Greedy-lowest | ★★★ |
| 9 | The Pass | When to pass (forced) | Greedy-lowest | ★★★ |
| 10 | Denial | Leading to deny RPS doublings | Medium | ★★★ |
| 11 | The Wild Finisher | Hoarding wild for endgame | Medium | ★★★ |
| 12 | Short Hand | Playing from behind (7 vs 10) | Medium | ★★★★ |
| 13 | Long Grind | 12-card hands, endurance | Medium | ★★★★ |
| 14 | The Mirror | Symmetric heroes, pure skill | Hard | ★★★★★ |
| 15 | The Gauntlet | Everything at once | Hard | ★★★★★ |

---

## Mission 1 — First Chain

**Premise:** "Every journey begins with a single card."

**Fixed parameters:**
- Wild face: **5** (fixed)
- Bounty suit: **rock** (fixed)
- Hand size: **5 each** (reduced from 9 for a short tutorial)
- Pre-scripted hands: **yes** — both hands fixed (see below)

**Pre-scripted hands:**
- Player (A): `2♣, 4♣, 6♣, 8♣, 9♣` (all scissors — no RPS pressure)
- CPU (B): `3♦, 5♦, 7♦, 8♦, 9♦` (all paper)

**Player hero:** **Recruit** (locked — bounty 0, favor 0, shift 0, draft −1; shared stats with a dummy opponent all zero)

**CPU policy:** **Random-legal** (picks uniformly among legal plays; never strategic)

**Goal:** Win. No turn limit.

**Teaches:** The chain is monotone — you must play a card whose effective value meets or beats the chain. With all same-suit cards and no shift/favor, this is pure face-value climbing. The player learns they can *always* play their highest card to reset, or climb gradually.

**Design note:** This is a *forced win* — the player has 9♣ and can always climb to 9. The CPU is random, so it will sometimes strand itself. The lesson is "you have legal plays; use them."

---

## Mission 2 — The Doubling Rule

**Premise:** "Rock crushes scissors. But how much?"

**Fixed parameters:**
- Wild face: **7** (fixed, but no wilds in hands)
- Bounty suit: **paper**
- Hand size: **6 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `3♠ (rock), 4♠, 5♠, 6♠, 8♠, 9♠`
- CPU (B): `4♣ (scissors), 5♣, 6♣, 7♣, 8♣, 9♣`

**Player hero:** **Recruit** (locked)

**CPU policy:** **Greedy-lowest** (always plays lowest legal non-wild)

**Goal:** Win.

**Teaches:** The ×2 multiplier. When the CPU leads scissors and the player holds rock, a **3-rock (effective 6) beats a 5-scissors**. The player learns that *low rock cards are bombs against scissors chains* — and that the CPU's greedy-lowest policy will happily walk into it. The pre-scripted hands ensure the player *has* a rock bomb when the CPU leads scissors.

**Design note:** The CPU's greedy-lowest policy means it plays its lowest legal card. If the player leads scissors, the CPU (holding scissors) will play its lowest scissors — setting up the player's rock doubling. This is a *coached* moment.

---

## Mission 3 — Wild Awakening

**Premise:** "One face can ignore the chain entirely."

**Fixed parameters:**
- Wild face: **4** (fixed)
- Bounty suit: **scissors**
- Hand size: **6 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♠, 3♠, 4♠ (wild), 6♠, 8♠, 9♠`
- CPU (B): `2♣, 3♣, 4♣ (wild), 6♣, 8♣, 9♣`

**Player hero:** **Recruit** (locked)

**CPU policy:** **Greedy-lowest** (uses wild only when no other legal play — matches base game)

**Goal:** Win.

**Teaches:** Wilds are always legal on a numeric chain. The player should learn to **hold the wild** until the CPU strands them at a high chain value (e.g., CPU plays 9, player has no legal non-wild, plays 4♠ wild). The player also learns **wild-vs-wild**: if the player plays the wild first, the CPU can only follow with a *better-suited* wild (rock beats scissors, but scissors does not beat rock — so the player's 4♠ rock wild is unbeatable by the CPU's 4♣ scissors wild). This is a *coached wild dominance* moment.

**Design note:** The pre-scripted hands guarantee both players have their wild. The lesson is "wilds are tempo, not bombs — use them when you're stuck, or use them to *strand* the opponent."

---

## Mission 4 — Favor of the Suit

**Premise:** "The gods smile on some duels."

**Fixed parameters:**
- Wild face: **9** (fixed, no wilds in hands)
- Bounty suit: **rock**
- Hand size: **6 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♠, 3♠, 5♠, 6♠, 8♠, 9♠` (rocks)
- CPU (B): `2♣, 3♣, 5♣, 6♣, 8♣, 9♣` (scissors)

**Player hero:** **Warden** (locked — favor +1)

**CPU hero:** **Recruit** (dummy — favor 0)

**Shared favor:** **+1** (Warden +1 + Recruit 0)

**Shared shift:** 0
**Shared bounty:** 0
**Shared draft:** 0 (Warden 0 + Recruit −1 → max(0, −1) = 0)

**CPU policy:** **Greedy-lowest**

**Goal:** Win.

**Teaches:** Positive favor makes RPS wins stronger. A 5-rock vs scissors = `5×2 + 1 = 11` (beats a 9-scissors at 9). The player learns that **favor +1 makes mid-rock cards into high-card beaters**. The pre-scripted hands ensure the player has rock and the CPU has scissors, so the favor bonus is *live* on every RPS win.

**Design note:** This is the "favor is good" tutorial. Mission 5 inverts it.

---

## Mission 5 — Burden of Favor

**Premise:** "When the gods frown, winning the duel costs you."

**Fixed parameters:**
- Wild face: **9** (fixed, no wilds)
- Bounty suit: **rock**
- Hand size: **6 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♠, 3♠, 5♠, 6♠, 8♠, 9♠` (rocks)
- CPU (B): `2♣, 3♣, 5♣, 6♣, 8♣, 9♣` (scissors)

**Player hero:** **Turtle** (locked — favor −1)

**CPU hero:** **Turtle** (dummy — favor −1)

**Shared favor:** **−2** (Turtle −1 + Turtle −1)

**Shared shift:** 0, **bounty** 0, **draft** 0

**CPU policy:** **Greedy-lowest**

**Goal:** Win.

**Teaches:** **Negative favor inverts the RPS incentive.** A 5-rock vs scissors = `5×2 − 2 = 8`, which is *worse* than a same-suit 5 (effective 5) — wait, no: 8 > 5, so it's still better. Let me recompute: 5-rock vs scissors = `10 − 2 = 8`. A same-suit 5 = 5. So the RPS win is still *better* (8 > 5), but *less* better than with favor 0 (where it would be 10). The inversion is at **low cards**: a 1-rock vs scissors = `2 − 2 = 0`, which is *worse* than a same-suit 1 (effective 1). So **low cards actively avoid RPS wins under negative favor**.

The player learns: **with negative favor, don't lead low rocks into scissors — save your rocks for high cards, or play same-suit to avoid the penalty.**

**Design note:** The pre-scripted hands have low rocks (2, 3) where the inversion bites. The player must learn to *not* play 2-rock on a scissors chain (effective 2, worse than a 3-scissors at 3) — instead play 5-rock or higher where the doubling still wins.

---

## Mission 6 — Shifted Ground

**Premise:** "The ground itself rises."

**Fixed parameters:**
- Wild face: **7** (fixed, no wilds)
- Bounty suit: **paper**
- Hand size: **6 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `1♠, 2♠, 4♠, 6♠, 8♠, 9♠` (rocks — note the 1, which becomes 3 with shift 2)
- CPU (B): `3♣, 4♣, 5♣, 7♣, 8♣, 9♣` (scissors)

**Player hero:** **Gorilla** (locked — shift 2)

**CPU hero:** **Gorilla** (locked — shift 2)

**Shared shift:** **2** (Gorilla 2 + Gorilla 2, floor((2+2)/2) = 2)

**Shared favor:** 0, **bounty** 0, **draft** −2 → max(0, −2) = 0

**CPU policy:** **Greedy-lowest**

**Goal:** Win.

**Teaches:** **Shift raises every face.** The player's 1-rock is now a 3-rock (effective 3). The 9-scissors is now 11. The player learns that **shift compresses the relative value of low cards** — a 1 is no longer the worst card, and the effective ceiling is now `(9+2)×2 = 22`. The pre-scripted 1-rock teaches "even a 1 is playable when shifted."

**Design note:** The CPU's 9-scissors at effective 11 is a *bomb* the player must plan around. The player's 9-rock at effective 11 (or 22 vs scissors) is the answer.

---

## Mission 7 — The Draft Table

**Premise:** "Choose your opening carefully."

**Fixed parameters:**
- Wild face: **6** (fixed, no wilds)
- Bounty suit: **scissors**
- Hand size: **7 each** (before draft)
- Pre-scripted hands: **yes**

**Pre-scripted hands (before draft):**
- Player (A): `1♠, 2♠, 3♠, 7♠, 8♠, 9♠, 9♣` (two 9s, low rocks, one scissors)
- CPU (B): `1♣, 2♣, 3♣, 7♣, 8♣, 9♣, 9♠`

**Player hero:** **Gambler** (locked — draft +1)

**CPU hero:** **Gambler** (locked — draft +1)

**Shared draft:** **2** (Gambler +1 + Gambler +1)

**Shared bounty:** 2 (Gambler 2 + Gambler 2, floor(4/2) = 2 → hand size 11 — wait, that's too big)

**Correction:** Gambler bounty is +2, so shared bounty = floor((2+2)/2) = 2, hand size = 9 + 2 = 11. That's too big for a tutorial. **Override the hand size to 7 for this mission** (campaign allows hand-size override).

**Shared shift:** 1 (Gambler 1 + Gambler 1, floor(2/2) = 1)

**CPU policy:** **Greedy-lowest**

**Goal:** Win.

**Teaches:** **Draft swaps bad cards for good.** The player has two 9s and low rocks — they should pass *one of the 9s* (or a low rock) and receive one of the CPU's cards. The lesson is "draft is a quality filter — dump your worst card, receive a random one." The pre-scripted hands are deliberately lopsided (player has a duplicate 9) so the swap matters.

**Design note:** The CPU also drafts 2 cards (draft 2 each way). Both players swap simultaneously. The player learns that draft is **symmetric** — they give and receive equally, so it's a net quality change based on what they give up vs what they get.

---

## Mission 8 — Initiative

**Premise:** "The weakest servant of the bounty suit leads."

**Fixed parameters:**
- Wild face: **8** (fixed)
- Bounty suit: **paper** (fixed)
- Hand size: **7 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♣ (scissors), 3♦ (paper), 5♦, 7♦, 8♦, 9♦, 9♣`
- CPU (B): `1♦ (paper), 2♦, 4♦, 6♦, 8♦, 9♦, 8♣`

**Player hero:** **Recruit** (locked)

**CPU hero:** **Recruit** (locked)

**Shared stats:** all zero (bounty 0, draft −2 → 0, shift 0, favor 0)

**Leader:** The bounty suit is **paper (♦)**. Player's lowest paper is 3♦; CPU's lowest paper is 1♦. **CPU leads** (holds 1♦ < 3♦).

**CPU policy:** **Greedy-lowest**

**Goal:** Win.

**Teaches:** **Leader determination.** The CPU leads because it holds the lowest bounty-suit card. The player learns that the leader is *not* random — it's determined by hand composition. The player also learns that **holding a low card of the bounty suit is a disadvantage** (you lead, which is a tempo cost under the current balance at ~52% — actually a slight advantage overall, but under this mission's scripted hands, the CPU leading gives the player the information advantage).

**Design note:** The pre-scripted hands make it *obvious* who leads (CPU has 1♦, player has 3♦). The player learns to *count the bounty suit* to predict who leads.

---

## Mission 9 — The Pass

**Premise:** "Knowing when to yield is a weapon."

**Fixed parameters:**
- Wild face: **3** (fixed, no wilds)
- Bounty suit: **rock**
- Hand size: **6 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♣, 3♣, 4♣, 5♣, 6♣, 9♣` (all scissors, ascending)
- CPU (B): `1♠, 2♠, 4♠, 6♠, 7♠, 8♠` (all rocks)

**Player hero:** **Recruit** (locked)

**CPU hero:** **Recruit** (locked)

**Shared stats:** all zero

**Leader:** bounty suit rock, player's lowest rock = none (player has no rocks!), CPU's lowest rock = 1♠. **CPU leads.**

**CPU policy:** **Greedy-lowest** — but with a **twist**: the CPU is allowed to *voluntarily pass* if its legal plays would all be bad (i.e., if the only legal play is a card that sets up a player doubling). This is a "smart pass" policy.

**Goal:** Win.

**Teaches:** **Passing is a real move.** The player learns that when they have no legal play, they pass and the opponent leads fresh. But also — the CPU *chose* to pass in this mission (forced by the policy), showing that passing can be strategic. The player learns to *force* the CPU to pass by leading high cards they can't beat.

**Design note:** The player has only scissors; the CPU has only rocks. If the player leads scissors, the CPU (holding rock) can double. If the player leads *high* scissors (9), the CPU must either play a rock (doubling to 18+) or pass. The player learns "lead high to strand them." But if the CPU doubles with a rock, the player must pass (no legal scissors beats 18). So the player learns the *pass economy*: forcing passes is a tempo win.

---

## Mission 10 — Denial

**Premise:** "Deny them the doubling."

**Fixed parameters:**
- Wild face: **9** (fixed, no wilds)
- Bounty suit: **scissors**
- Hand size: **7 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♠, 3♠, 4♠, 5♠, 6♠, 8♠, 9♠` (rocks)
- CPU (B): `2♣, 3♣, 4♣, 5♣, 6♣, 8♣, 9♣` (scissors)

**Player hero:** **Recruit** (locked)

**CPU hero:** **Warden** (locked — favor +1)

**Shared favor:** **+1** (Recruit 0 + Warden +1)

**Shared shift:** 0, **bounty** 1 (0 + 1 → floor(1/2) = 0), **draft** −1 → 0

**CPU policy:** **Medium** — "denial-aware": if the player holds rocks and the chain is scissors, the CPU will *avoid leading scissors* (which would let the player double) and instead lead **rocks** (denying the player's doubling) or **same-suit** cards.

**Goal:** Win.

**Teaches:** **Denial.** The CPU actively denies the player's RPS doublings by leading same-suit or winning-suit cards. The player must learn to **create their own doubling opportunities** by leading scissors (to bait the CPU into a rock response they can then double with... wait, no — if the player leads scissors, the CPU's rock doubles against it). Let me re-think: the player wants to lead a *suit the CPU will respond to with the player's winning suit*. The player holds rocks; they want the chain to be scissors so their rocks double. The CPU holds scissors; it wants to avoid setting scissors chains. So the player's job is to **force the chain to scissors** by playing scissors themselves (sacrificing a card) — but the player has no scissors. So the player must **bait the CPU into leading scissors** by playing rocks (which the CPU, holding scissors, might respond to with... scissors loses to rock, so no doubling). Hmm.

**Correction:** The player holds rocks, the CPU holds scissors. Rock beats scissors. So the player *always* wants the chain to be scissors (their rocks double) and the CPU *always* wants the chain to be rocks (their scissors... lose, so no doubling for CPU, but also no doubling for player). Actually the CPU wants the chain to be **scissors** so its own scissors don't double against... no, scissors don't beat scissors. The CPU wants the chain to be **paper** (scissors beat paper) but neither player has paper. So the CPU's best denial is to keep the chain on **scissors** (its own suit, where it can play same-suit without giving the player a doubling) or **rocks** (where the player's rocks are same-suit, no doubling).

So the CPU's denial policy is: **lead scissors** (same suit as its hand, denies the player's rock doubling because scissors chain means player's rock would double — wait, that *gives* the player a doubling). Hmm, I have the RPS backwards.

Let me be precise: **Rock beats Scissors.** If chain is Scissors, a Rock play doubles. If chain is Rock, a Scissors play *loses* (no double), a Paper play *wins* (doubles, but nobody has paper). If chain is Paper, a Scissors play doubles.

Player holds Rocks. Player doubles when chain is **Scissors**.
CPU holds Scissors. CPU doubles when chain is **Paper**.

So the player wants chain = Scissors, CPU wants chain = Paper. Neither can force Paper (nobody has it). The CPU's denial is: **avoid leading Scissors** (which would let the player double) and instead **lead Rocks** (same-suit as player, no doubling) or **lead Paper** (nobody has it, so the player must respond with Rock — same-suit-ish, no doubling, or pass).

**The CPU's medium policy:** "If the player holds a suit that beats mine, lead my *losing* suit instead of my winning suit to deny them the doubling." In this case, CPU leads Rocks (its losing suit) to keep the chain on Rock, where the player's Rocks are same-suit (no double). The player must then **break the chain** by playing a Rock (same-suit, no double) or a Scissors (loses, no double) — neither doubles. The player's only doubling opportunity is if the chain becomes Scissors, which the CPU avoids.

**The lesson:** The player must learn to **create a Scissors chain** by... they can't, they have no scissors. So the lesson is actually: **sometimes you can't double, and you must win on raw face value**. The player learns that denial is real and that the CPU will actively avoid giving them the doubling.

**Design note:** This is a *hard* mission because the player's doubling is denied. The player must win on efficient climbing, not on RPS bombs.

---

## Mission 11 — The Wild Finisher

**Premise:** "Save the wild for the end."

**Fixed parameters:**
- Wild face: **2** (fixed)
- Bounty suit: **rock**
- Hand size: **8 each**
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `1♠, 2♠ (wild), 3♠, 4♠, 5♠, 6♠, 7♠, 8♠` (all rocks, one wild)
- CPU (B): `1♣, 2♣ (wild), 3♣, 4♣, 5♣, 6♣, 7♣, 8♣` (all scissors, one wild)

**Player hero:** **Recruit** (locked)

**CPU hero:** **Recruit** (locked)

**Shared stats:** all zero

**Leader:** bounty suit rock, player's lowest rock = 1♠, CPU has no rocks → **player leads**.

**CPU policy:** **Medium** — "wild finisher": the CPU *hoards* its wild (never plays it until it has ≤2 cards left), and otherwise plays greedy-lowest. This forces the player to deal with the CPU's wild as a *late-game bomb*.

**Goal:** Win.

**Teaches:** **Wilds are finishers, not openers.** The player must learn to *hoard their wild* as well — if the player plays their 2♠ wild early, they lose the tempo tool. The pre-scripted hands ensure both players have a wild and equal card counts, so the wild is the *deciding* card. The player learns that the wild's value is in *holding it*, not playing it.

**Design note:** The CPU's "wild finisher" policy means it will strand the player at a high chain value late, then play its wild to win the trick and lead off, shedding its last cards. The player must *match* this — hold their wild, wait for the CPU to strand them, use it to win the trick, then shed. This is a *timing* lesson.

---

## Mission 12 — Short Hand

**Premise:** "Seven cards against ten. Make them count."

**Fixed parameters:**
- Wild face: **5** (fixed)
- Bounty suit: **paper**
- **Hand size: Player 7, CPU 10** (asymmetric — override)
- Pre-scripted hands: **yes**

**Pre-scripted hands:**
- Player (A): `2♣, 4♣, 6♣, 7♣, 8♣, 9♣, 9♠` (7 cards)
- CPU (B): `1♦, 2♦, 3♦, 4♦, 5♦, 6♦, 7♦, 8♦, 9♦, 9♣` (10 cards)

**Player hero:** **Recruit** (locked)

**CPU hero:** **Recruit** (locked)

**Shared stats:** all zero (override hand size to 7 vs 10)

**Leader:** bounty suit paper, player has no paper, CPU's lowest paper = 1♦ → **CPU leads**.

**CPU policy:** **Medium** — "attrition": plays greedy-lowest but *prioritizes emptying its hand* (never voluntarily passes, always plays if legal). The CPU has more cards, so it must be aggressive.

**Goal:** Win.

**Teaches:** **Efficiency under card deficit.** The player has 7 cards vs 10. They must win on *tempo* — force the CPU to pass (denying the CPU a shed) and shed efficiently. The player's 9♠ (scissors) is a *doubling bomb* against the CPU's paper (♦) chain — the player must use it to win a critical trick and lead off. The lesson: **when you have fewer cards, every doubling matters**.

**Design note:** The CPU's 10-card hand means it will likely win the *card-count* race unless the player forces passes. The player's 9♠ vs paper is the key tempo play.

---

## Mission 13 — Long Grind

**Premise:** "Twelve cards each. Endurance."

**Fixed parameters:**
- Wild face: **9** (fixed)
- Bounty suit: **scissors**
- **Hand size: 12 each** (override — Bodybuilder mirror)
- Pre-scripted hands: **no** (random deal, but seeded for reproducibility)

**Player hero:** **Bodybuilder** (locked — bounty +3)

**CPU hero:** **Bodybuilder** (locked — bounty +3)

**Shared stats:** bounty +3 → hand size 12; draft −2 → 0; shift 0; favor 0

**Leader:** determined by lowest scissors (random deal).

**CPU policy:** **Medium** — "attrition": greedy-lowest, but *saves one high card* (9) as a finisher. Tracks its hand count and plays more conservatively when ahead.

**Goal:** Win.

**Teaches:** **Long games reward planning.** With 12 cards, the chain runs long and every card matters. The player learns that *hoarding a 9* as a finisher is essential — the CPU will do the same. The lesson is **endurance and finisher management**: don't dump your highest card early; save it to break a stalemate.

**Design note:** This is a *long* mission (~40–60 turns). The difficulty is stamina and consistency, not a specific trick.

---

## Mission 14 — The Mirror

**Premise:** "The same hero, the same hand. Only skill separates you."

**Fixed parameters:**
- Wild face: **7** (fixed)
- Bounty suit: **rock**
- **Hand size: 9 each** (standard)
- Pre-scripted hands: **yes — identical hands** (mirror)

**Pre-scripted hands (identical):**
- Both players: `1♠, 2♠, 3♠, 4♣, 5♣, 6♣, 7♦, 8♦, 9♦`

**Player hero:** **Warden** (locked — favor +1)

**CPU hero:** **Warden** (locked — favor +1)

**Shared stats:** bounty 1 → hand 9 (floor(2/2)=1, 9+1=10 — override to 9); draft 0; shift 0; favor +2

**Leader:** bounty suit rock, both have 1♠ and 2♠ and 3♠. Lowest rock is 1♠, held by *both* — but each face appears once per suit, so only *one* player has 1♠. Wait, the hands are *identical*, so both have 1♠ — that's impossible with a real deck (only one 1♠). **Correction:** the mirror hands must be *different* (each player has a distinct 1♠, etc.). So the "mirror" is *statistically symmetric* (same hero, same hand *size*, same suit distribution) but the actual cards differ. Let me re-specify:

**Pre-scripted hands (symmetric, distinct cards):**
- Player (A): `1♠, 2♠, 3♠, 4♣, 5♣, 6♣, 7♦, 8♦, 9♦`
- CPU (B): `4♠, 5♠, 6♠, 1♣, 2♣, 3♣, 7♦, 8♦, 9♦`

Both have 3 rocks, 3 scissors, 3 paper — symmetric suit distribution, distinct cards. Bounty suit rock → player's lowest rock is 1♠, CPU's is 4♠ → **player leads**.

**CPU policy:** **Hard** — full minimax (open-hand, since campaign can be open-hand). The CPU plays *optimally* given the player's known hand.

**Goal:** Win.

**Teaches:** **Pure skill.** With symmetric stats and symmetric suit distribution, the game is decided by *who plays better*. The CPU is optimal (minimax), so the player must play optimally too. This is the "final exam" of the tutorial — every mechanic is live, and the opponent is perfect.

**Design note:** This is *very* hard. The player should only attempt it after mastering the previous 13. The CPU's minimax is tractable (9 cards each, perfect info) and will exploit any mistake.

---

## Mission 15 — The Gauntlet

**Premise:** "Everything at once. No second chances."

**Fixed parameters:**
- Wild face: **random** (flipped at setup, not pre-scripted — but the campaign *reveals* it to the player)
- Bounty suit: **random** (revealed)
- **Hand size: 11 each** (asymmetric bounty override — one hero high bounty, one low)
- Pre-scripted hands: **no** (random deal, seeded)

**Player hero:** **free choice** from all 11 (the player *picks* their hero)
**CPU hero:** **random** from all 11 (revealed at start)

**Shared stats:** computed from the pair (all four stats live)

**Leader:** determined by bounty-suit rule

**CPU policy:** **Hard** — full minimax (open-hand, 11 cards each — feasible with memoization).

**Goal:** Win.

**Teaches:** **Full synthesis.** Every mechanic is live (wilds, favor, shift, draft, bounty, initiative), and the opponent is optimal. The player must *choose their hero* wisely (the campaign reveals the CPU hero first, so the player can pick a counter). This is the campaign's *boss fight* — a single game against a perfect opponent with random parameters, where hero choice is the player's only lever.

**Design note:** The "pick your hero after seeing the CPU's hero" is the campaign's *reward* for completing missions 1–14 — it's the player's chance to use everything they've learned. The random wild/bounty means no two attempts are identical.

---

## Campaign Implementation Notes (for Copilot)

### Mission data structure

Each mission should be a data record (Go struct or JSON) with:

```go
type Mission struct {
    ID              int
    Title           string
    Premise         string

    // Fixed parameters (nil / zero = use default)
    WildFace        *int              // fixed wild face, or nil for random
    BountySuit      *Suit             // fixed bounty suit, or nil for random
    PlayerHandSize  *int              // override, or nil for 9+shared_bounty
    CPUHandSize     *int              // override, or nil for 9+shared_bounty
    PlayerHand      []Card            // pre-scripted, or nil for random deal
    CPUHand         []Card            // pre-scripted, or nil for random deal

    // Hero constraints
    PlayerHero      HeroChoice        // Locked(hero) | Free | Restricted(set)
    CPUHero         HeroChoice        // Locked(hero) | Random | Restricted(set)
    RevealCPUHero   bool              // if true, player sees CPU hero before choosing theirs

    // CPU policy
    CPUPolicy       PolicyType        // RandomLegal | GreedyLowest | MediumDenial | MediumWildFinisher | MediumAttrition | HardMinimax

    // Goal
    Goal            GoalType          // Win | WinInNTurns(n) | WinWithoutPassing | ...
    TurnLimit       *int              // nil = no limit

    // Metadata
    Teaches         string
    Difficulty      int               // 1..5
}
```

### Setup overrides

The campaign needs to **bypass the standard setup** in specific ways:

1. **Pre-scripted hands:** if `PlayerHand` / `CPUHand` are set, use them instead of dealing. The wild face and bounty suit are still determined from a flipped bounty card *unless* `WildFace` / `BountySuit` are set, in which case they're forced (and the bounty card is *not* flipped — instead, a card matching the forced wild face + bounty suit is conceptually "flipped" but removed from the deck).
2. **Hand-size override:** if `PlayerHandSize` / `CPUHandSize` are set, use them instead of `9 + shared_bounty`. This requires the deck to have enough cards — if the sum exceeds 27 minus the bounty card, the mission is *invalid* and should error at load time.
3. **Hero lock/free/restricted:** the UI must enforce this. `Locked(hero)` means the player *cannot* choose. `Free` means any of the 11. `Restricted(set)` means a subset. The CPU hero is set by the mission (or random/restricted).
4. **Reveal CPU hero:** if `RevealCPUHero` is true, the player sees the CPU's hero *before* choosing their own (mission 15).

### CPU policy implementations

Each policy is a function `(g *LadderGame, legal []Card, player PlayerID) Card`:

- **`RandomLegal`:** pick uniformly from `legal`. (Mission 1)
- **`GreedyLowest`:** pick the lowest-face non-wild in `legal`, else lowest wild. (Missions 2–9)
- **`MediumDenial`:** if the player holds a suit that beats the current chain suit, *avoid leading that suit* (lead same-suit or losing-suit instead). Otherwise greedy-lowest. (Mission 10)
- **`MediumWildFinisher`:** hoard wild until hand size ≤ 2, then play it. Otherwise greedy-lowest. (Mission 11)
- **`MediumAttrition`:** greedy-lowest, but never voluntarily pass; if ahead on card count, save the highest non-wild as a finisher. (Missions 12–13)
- **`HardMinimax`:** full minimax with memoization over `(chainValue, chainSuit, chainIsWild, handA, handB)`. Open-hand (sees player's hand). Tractable for ≤12 cards each. (Missions 14–15)

### UI requirements

- **Mission select screen:** 15 missions in a list, with title, difficulty (★–★★★★★), and a "locked/unlocked" state. Mission 1 unlocked; each subsequent mission unlocks on completing the previous.
- **Pre-mission briefing:** shows the premise, the fixed parameters (wild face, bounty suit, hand sizes), the hero constraint, and the goal. For mission 15, shows the CPU hero *before* the player chooses theirs.
- **In-game:** shows the player's hand, the chain, the wild face, the bounty suit, and the CPU's hand size (but *not* the CPU's cards, unless the mission is open-hand — missions 14–15 are open-hand for the minimax CPU, so the player sees the CPU's hand too).
- **Post-mission:** if the player wins, show a "lesson learned" recap (the `Teaches` string) and unlock the next mission. If the player loses, offer a retry with the same seed (for pre-scripted missions) or a new seed (for random missions).

### Tuning notes

- **Missions 1–3 are forced wins** — the player *cannot* lose if they play reasonably. The CPU is random or greedy-lowest, and the pre-scripted hands guarantee the player has an answer. These are *tutorials*, not challenges.
- **Missions 4–9 are winnable but require understanding** — the player must use the specific mechanic (favor, shift, draft, initiative, pass). The CPU is greedy-lowest, so it's beatable with correct play.
- **Missions 10–13 are genuine challenges** — the CPU is Medium, and the player must play well. Winrate under the greedy policy is ~52%, so a human player should win 60–70% with understanding.
- **Missions 14–15 are *hard*** — the CPU is minimax. The player should expect to lose 30–50% of attempts. Mission 15's hero-pick-after-reveal is the player's compensation.
- **All missions should be *seeded* for reproducibility** — pre-scripted missions use the scripted hands (no RNG). Random missions (13, 15) should use a fixed seed per attempt, re-rolled on retry.

### What NOT to implement in the campaign

- **No mission scoring or stars** — win/lose only. (The base game has no score; the campaign shouldn't invent one.)
- **No mission-specific rule changes** — the campaign uses the *same rules* as the base game, only with different *setup parameters* and *CPU policies*. If a mission needs a rule change, it's a sign the mission is too complex — simplify the mission instead.
- **No "hint" system** — the pre-mission briefing and the in-game state display are the only teaching tools. If a mission is unclear, rewrite the briefing.

--- 