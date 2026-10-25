# Opening Draft

Each hero has a Draft attribute. The match mode is
`clamp(3 + player Draft + opponent Draft, 1, 5)`. A zero combined modifier is
Draft 3; hero attributes are bounded from -2 to +2. Recruit and Gorilla are -1,
Gambler, Diplomat, and Turtle are +1, and other heroes have no direct modifier.
Hater negates the opponent's Draft modifier instead of adding a direct one.
Campaign missions keep their scripted hands and bypass the draft.

## Deal and picks

The 27-card deck is shuffled and one card becomes the bounty. Each player
receives the same number of private hole cards: four, three, two, one, or zero
for Draft 1–5. The public pool contains four, six, eight, ten, or twelve cards;
picks alternate starting with Y, with each player drafting one through five
cards. The two unpicked cards are removed from play. Every mode begins Round 1
with five cards per player and leaves 14 cards in the draw pile. Subsequent
rounds use the existing three-card commit and seat-based refill rules.

## Battlefield placement and privacy

The draft pool is rendered inside the existing battlefield area, not in a
separate screen or modal. The active picker sees only their own dealt and
drafted cards beside the public pool. Hot-seat matches require a pass-screen
confirmation before each private pick. In CPU matches, AI picks resolve
automatically and only the human's private cards are shown.

## AI draft policy

Easy AI drafts randomly. Normal AI chooses the greedy pick half the time and a
random pick otherwise. Hard AI always chooses greedily. The greedy value is the
card face plus two for matching the bounty suit, two per held card in that
suit, and three when the card exceeds the highest held face in that suit.
Medium CPU timeout picks use the highest face, breaking face ties by choosing
the suit with fewer cards already in hand.

## Simulation

Run the headless balance check with:

```powershell
go -C backend run ./cmd/prisoners-of-war --sim-draft 1000
```

The command reports original Seat X/Y/tie rates, mean score and score
difference, stall and clash rates, and draw-pile depletion. It exits with an
error if original Seat X's win rate is outside 46–54%. The supplied Python
comparison model tests Draft 1–5 with equal opening hands and discards unpicked
pool cards; its results are model estimates, while the Go simulator is the
regression check for the integrated game implementation.
