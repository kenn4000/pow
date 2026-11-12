# Prisoners of War

A two-player, three-round card game with a Go backend, Vite/React frontend,
terminal interface, and Helm deployment to
`https://games.markdownthing.com/prisoner-of-war/`.

## Repository layout

```text
backend/cmd/prisoners-of-war/ Go executable entrypoint
backend/internal/           Application, game packages, tests, storage, and artwork
frontend/                   Vite/React app and frontend container
deploy/helm/prisoner-of-war/ Helm chart for k3s
docs/                       Game systems and feature guides
compose.yaml                Local PostgreSQL container and persistent volume
.gitlab-ci.yml              Test → build images → in-cluster Helm deployment
```

Go module metadata and all Go packages live under `backend/`. HTTP routing uses
chi, PostgreSQL access uses pgx, and passwords use bcrypt. Local development
requires Go 1.21+, Node 22.12+, and Docker for PostgreSQL.
See the [backend layout and commands](backend/README.md) for source organization.
See [deployment setup](deploy/README.md) for GitLab variables and cluster commands.

The existing browser screens and renderer are preserved in
`frontend/src/game.html`. Vite extracts its CSS, static shell, and controller
into compiled modules; the React `App` mounts and manages the renderer's
lifecycle. This is an incremental React integration; the game screens still
use the existing DOM renderer.

## Rules summary

- **Shared combat:** The 27-card deck has Scissors, Paper, and Rock suits,
  numbered 1–9. Scissors beats Paper, Paper beats Rock, and Rock beats Scissors;
  a cycle win doubles the card's face before hero modifiers. One card sets the
  bounty suit. Shift changes combat faces, Favor adjusts cycle-winning cards,
  and Bounty changes the scoring face of bounty-suit cards.
- **Prisoners of War:** In CPU or online play, players draft private cards and
  alternate picks from a shared public pool, starting with Seat Y. Each round,
  secretly commit three cards; exchanges resolve in X/Y/X attack order. The
  defender sees the attack before responding. The higher effective value
  captures the opposing card, while a tie discards both. Each suit scores its
  weighted prisoner count multiplied by its highest scoring face. Ties are
  decided by distinct suits, highest scoring face, then a coin flip.
- **Fog of War:** Uses the same rules as Prisoners of War, but a hidden opposing
  card shows only a face-value range until it is revealed. Play against a CPU
  or another player online.
- **Campaigns:** All campaigns are solo against CPU opponents and use their
  variant's existing rules and layout with scripted setups and mission goals.
  Basic and Advanced Training teach combat and hero modifiers.
- **Abandon:** Flip hidden board cards, then pair two unused cards from the
  board or your private pile. Standard combat determines captures; ties capture
  nothing. Captured cards score by suit, while unused private-pile cards can
  cost points. Abandon Duel supports CPU and online opponents across two
  rounds, with a shared board, private piles, and reversed starting player in
  Round 2.
- **Ladder:** Play legal cards that meet or exceed the current chain value.
  Wild cards change the chain suit. Pass when no card is playable; the opponent
  can open a fresh chain. Empty your hand to win. Play solo against a CPU or
  online against another player.
- **Rummy:** Build sets of three equal-face cards in different suits and
  same-suit runs of three or more consecutive cards. Player 1 opens by drawing
  two, discarding one face-up and burying one. Each player gets six turns total;
  take a face-up card or draw two, then discard one face-up and bury one. The
  best melds score points and unmelded cards subtract points. Play against a
  CPU or online opponent.
- **Poker:** In the solo campaign or Poker Duel against a CPU or online
  opponent, build three balanced five-card hands. Place a card in a legal hand
  or discard it to draw a replacement. Poker Duel gives each player six cards,
  uses timed turns, and banks unused time for a bonus. Campaign missions add
  scoring targets and constraints.
- **Puzzler and Puzzle Battle:** Puzzler is a solo card-pairing challenge:
  resolve combat pairs, capture cards, and meet the puzzle's score target.
  Puzzle Battle offers solo practice against the Easy CPU or online head-to-head
  play. Submit three pairs in Round 1 and four in Round 2 before the shared
  timer ends; Round 1 scores stay hidden until both players submit.
- **Tug War:** Play hand cards against the top card of a shared pile, flip the
  deck onto a pile, or spend a limited push to move a pile. Winning clashes pull
  piles to your side; owned piles score their cards, with a bounty-suit bonus.
  Play solo against a CPU or online against another player.

See [Opening Draft](README-draft.md) for the standard opening deal and pick
sequence. Multiplayer matches are played online by friend invitation or
matchmaking in supported modes.

## Run the game with Docker Compose

Docker Compose runs PostgreSQL, the Go API, and the built mobile-friendly web
app together. Copy `.env.example` to `.env` (`Copy-Item .env.example .env` in
PowerShell or `cp .env.example .env` on macOS/Linux), then set unique values
for `POSTGRES_PASSWORD` and `BOOTSTRAP_ADMIN_PASSWORD`.

Build and start the stack:

```sh
docker compose up --build -d
docker compose ps
```

Open `http://localhost:8088/prisoner-of-war/` on the computer. To play on a
phone, connect it to the same trusted Wi-Fi and open
`http://<computer-LAN-IP>:8088/prisoner-of-war/`. `APP_PORT` in `.env` changes
the host port. Only the web app and PostgreSQL's loopback-only development
port are published; the API is private to the Compose network. PostgreSQL data
persists in the `postgres-data` volume across restarts. `docker compose down`
stops the app without deleting that data.

Sign up or sign in with the bootstrap admin account configured in `.env`.
Administrators have an **Admin** page for listing users and changing their
roles. **Saved games** lists active, completed, and abandoned matches with
their state-transition history. An unfinished match resumes after refreshing
or restarting the backend.

To inspect logs or stop the stack:

```sh
docker compose logs -f backend frontend postgres
docker compose down
```

For frontend hot reload, use the same `.env` and run the backend and Vite from
the host instead. Start the database with `docker compose up -d postgres`, then
load `.env` into the shell before starting the Go server:

```sh
set -a
. ./.env
set +a
GOWORK=off go -C backend run ./cmd/prisoners-of-war
```

In another terminal, run `npm --prefix frontend ci` and
`npm --prefix frontend run dev`. Open `http://localhost:5173/prisoner-of-war/`.

The development login page also has a **Sign in as any user** side panel.
It loads every account, including administrators, supports email/role search,
and opens a real session for the selected account without a password. The
development endpoints are `GET /api/auth/users` (cursor-paginated) and
`POST /api/auth/assume` with `{"userId":"…"}`. The picker is off by default
(`DEVELOPMENT_LOGIN=false` in `.env.example` and
`developmentLogin.enabled=false` in Helm). Set it to `true` only on a private
development environment. When off, both endpoints return 404 and the sign-in
page hides the panel. Never enable it on a host other people can reach: it
signs anyone in as any account, including administrators.

Screens have shareable hash routes, such as `#/campaigns`,
`#/shop/items/crowns`, `#/shop/skins/common`,
`#/shop/heroes-boxes/box-shop`, `#/skills/energy`, `#/saved-games`, and `#/password`. Refresh and browser
Back/Forward preserve the selected destination. **Cards & rules** is available
to every signed-in player, and the rules overview is also on the login page.
Commander Record lets players save a display name.

Campaigns can launch their scripted missions in the browser, including hero
selection, prerequisites, energy costs, progress and first-completion rewards.
Every campaign mission start, including a replay, costs 1 Energy; replays do
not award progression rewards again. Single matches cost 1 Energy and ranked
best-of-three matches cost 2 Energy. Energy recharges one point every ten
minutes unless a Skill Tree modifier changes the rate. Missions and their
hidden state persist through refresh like ordinary
matches. The 15-mission Poker campaign teaches scoring and strategy with two
visible teaching ghosts; the 15-mission Tug War campaign includes scripted
setups, hidden-information challenges, and mirror/gauntlet series. **Choose
match mode** can replace an active match with another mode;
**Restart this mode** restarts its current configuration. The unfinished match
is archived in Saved games. Battles wait for **Continue** by default, with an
optional auto-advance setting. Mobile play places the current hand first.

Use **Password** to change your password after confirming the current one.
The current session is rotated and stays signed in; all other sessions are
revoked. Players also have a **Cards & rules** page: its contact sheet covers
the 27-card deck and all 72 embedded artwork files. Clicking a thumbnail opens
a detail panel with suit rules, color/pose variants, and image metadata.

The **Social** tab provides public handles, ELO and leaderboard display,
friend requests, blocking, and direct messages between friends. The displayed
friendly-wager limits are informational; matchmaking, wagering, and gifting
are not yet connected.

The UI draws the bounty and
nine battle cells, capture borders, live score, current battle, piles, and
hands. Cards use configurable colors for Scissors, Paper, and Rock. Click a
suit circle to select one of eight colors; each suit must use a different
color. The color assignments are saved in the browser. The card face shows
only its number, colored by suit. The bounty has a gold border, while captured
cards have a White or Black border. Choose cards by clicking them.
Human-vs-Human mode uses a pass-screen prompt to hide hands during commitments
and handoffs. Play Again starts a fresh game and randomizes which player is
White.

Battle also offers **Poker Duel**, a randomly seeded standalone mirror match.
Choose an Easy, Medium, or Hard CPU opponent, invite a friend, or use random
matchmaking. Each player selects a hero; hands stay private, turns are timed,
and the heroes' strongest modifiers are shared. Multiplayer games do not
affect campaign progress or rewards.

Human-vs-human battles are online only: connect through a friend invitation or
random matchmaking. Human-vs-AI matches can be preselected:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode easy
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode normal
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode hard
```

Each match selects heroes before play. The browser offers hero choices when
starting a match; Human-vs-AI uses your saved default and lets the AI counter
your last opponent. The terminal [8] Heroes screen lets you set a default and
view hero records. The active modifiers and round weights are shown throughout
the game. See [heroes](docs/heroes.md) for the roster and rules, and
[hero acquisition](docs/heroes-acquisition.md)
for hero acquisition, campaigns, achievements, and skill tree additions.

The browser dashboard also provides quick access to the Skill Tree, hero
selection, and Shop from its bottom navigation. Its commander card and Match
History use saved player statistics; the Campaigns view shows actual mission
progress. The dashboard's daily shop feature is a preview of a catalog item
at its regular listed price and does not change purchase pricing.

To choose a different local address, use `--listen 127.0.0.1:8081`.

## Terminal fallback

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --ui terminal
GOWORK=off go -C backend run ./cmd/prisoners-of-war --ui terminal --mode hard
```

The terminal starts with a progression menu for CPU games, Ranked best-of-three
against the Hard AI, profile, heroes, campaigns, shop, skills, Camp Draws, and
energy actions. Human-vs-human matches are available in the web UI through
friend invitations or random matchmaking. See [campaigns](docs/campaigns.md) for the full
mission list, scripted setups, progression rules, and rewards. The Shop offers chromas, Hero Boxes, direct hero purchases, and Crown pack
price previews. Crown packs are not purchasable until real-money checkout is
integrated.
At commit, enter exactly three distinct hand numbers separated by spaces or
commas. During a battle, enter the number of a pile card. At the start of
Round 3, seats are assigned automatically from the current score. ANSI suit
colors can be disabled with `--no-color` or the `NO_COLOR`
environment variable. Progression details and payout rules are in
[economy](docs/economy.md).

Print the economy configuration, including payouts, skills, and chromas, as
JSON:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --dump-economy
```

## AI

- **Easy** chooses random legal cards.
- **Normal** chooses commits using immediate score value, attacks randomly,
  and makes greedy defenses.
- **Hard** evaluates commits against sampled unseen cards,
  uses a sampled worst-case attack choice, and makes greedy defenses. It only
  samples cards that are not known to the AI.

## Tests

Enforce a maximum cyclomatic complexity of 20 per production Go function:

```sh
make -C backend lint
```

This runs gocyclo pinned to v0.6.0 and is enforced by the GitLab `backend-lint`
job. Test files are excluded from the complexity metric; no production files
or existing functions are exempted.

Run the unit and integration tests with:

```sh
GOWORK=off go -C backend test ./...
```

The PostgreSQL integration test runs when `TEST_DATABASE_URL` is set. CI runs
it against a PostgreSQL service; locally, point it at your Docker database:

```sh
TEST_DATABASE_URL="postgres://prisonersofwar:${POSTGRES_PASSWORD}@127.0.0.1:5432/prisonersofwar?sslmode=disable" GOWORK=off go -C backend test -race ./...
```

The test creates and removes its own database schema. Connection URLs must
percent-encode special characters in passwords. See [storage and API](docs/storage.md)
for the data model, authentication, router groups, and deployment bootstrap.
