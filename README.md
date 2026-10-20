# Prisoners of War

A two-player, three-round card game with a local browser UI and a terminal
fallback. It uses only the Go standard library and targets Go 1.21 or newer.

## Rules summary

- The 27-card deck contains faces 1–9 in Blade (B), Fang (F), and Shield (S).
- Blade beats Fang, Fang beats Shield, and Shield beats Blade. A card that
  beats its opponent has its face value doubled for that exchange.
- At setup, one card is removed as the bounty; each card of its suit captured
  by a player is worth one additional point.
- Players are randomly assigned seats. X receives six cards and Y receives
  five. Each round, both players secretly commit three cards, then resolve
  three exchanges in X/Y/X attack order. The defender sees the attack card
  before choosing a response.
- The player with the higher effective value captures the opposing card; the
  winner's card is discarded. A tie is a clash, and both cards are discarded.
  The third battle automatically plays each player's last pile card.
- After Round 1 the seats swap before hands refill to the new seat targets.
  Before Round 3, the lower-scoring player chooses their seat; tied scores
  decide the chooser by coin flip. There is no refill after Round 3.
- Each suit scores its jail count multiplied by that suit's highest face.
  Add one point per captured bounty-suit card. Ties are broken by distinct
  suits, then highest single face, then coin flip.

## Run the graphical UI

```powershell
go run .
```

The local browser UI opens at `http://127.0.0.1:8080`. It draws the bounty and
nine battle cells, capture borders, live score, current battle, piles, and
hands. Cards use red numbers for Blade, yellow for Fang, and blue for Shield;
the bounty has a gold border, while captured cards have a White or Black
border. Choose cards by clicking them. Human-vs-Human mode uses a pass-screen
prompt to hide hands during commitments and handoffs. Play Again starts a
fresh game and randomizes which player is White.

The UI offers Human vs Human and Human vs AI at Easy, Normal, or Hard. A mode
can be preselected:

```powershell
go run . --mode hvh
go run . --mode easy
go run . --mode normal
go run . --mode hard
```

To choose a different local address, use `--listen 127.0.0.1:8081`.

## Terminal fallback

```powershell
go run . --ui terminal
go run . --ui terminal --mode hard
```

At commit, enter exactly three distinct hand numbers separated by spaces or
commas. During a battle, enter the number of a pile card. At the start of
Round 3, the eligible player enters `X` or `Y`. Invalid entries are rejected.
ANSI suit colors can be disabled with `--no-color` or the `NO_COLOR`
environment variable.

## AI

- **Easy** chooses random legal cards and seats.
- **Normal** chooses commits using immediate score value, attacks randomly,
  and makes greedy defenses.
- **Hard** evaluates commits and seat choices against sampled unseen cards,
  uses a sampled worst-case attack choice, and makes greedy defenses. It only
  samples cards that are not known to the AI.

## Tests and simulation

Run the unit and integration tests with:

```powershell
go test ./...
```

Run a headless Hard-vs-Hard simulation and print summary statistics with:

```powershell
go run . --sim 10000
```
