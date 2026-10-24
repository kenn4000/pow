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

- The 27-card deck contains faces 1–9 in Scissors (S), Paper (P), and Rock (R).
- Scissors beats Paper, Paper beats Rock, and Rock beats Scissors. A card that
  beats its opponent has its face value doubled for that exchange.
- At setup, one card is removed as the bounty. Hero modifiers affect combat
  and scoring: Shift changes every face, Favored adjusts cycle-winning cards,
  and Bounty changes the scoring face of cards matching the bounty suit.
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
- Each suit scores its captured-card count multiplied by that suit's highest
  effective scoring face. Reckoning can weight captures by the round in which
  they were won. Ties are broken by distinct suits, then highest effective
  scoring face, then coin flip.

## Run the graphical UI

Copy `.env.example` to `.env`, replace the database and admin passwords, then
start PostgreSQL and the backend from the repository root:

```sh
docker compose up -d --wait postgres
set -a
. ./.env
set +a
GOWORK=off go -C backend run ./cmd/prisoners-of-war
```

In another terminal, start Vite:

```sh
npm --prefix frontend ci
npm --prefix frontend run dev
```

Open `http://localhost:5173/prisoner-of-war/`. Vite proxies API and artwork
requests to the Go server at `127.0.0.1:8080`. Sign up or sign in with the
bootstrap admin account configured in `.env`. Administrators have an **Admin**
page for listing users and changing their roles. **Saved games** lists your
active, completed, and abandoned matches with their state-transition history.
An unfinished match resumes after refreshing or restarting the backend.

The development login page also has a **Sign in as any user** side panel.
It loads every account, including administrators, supports email/role search,
and opens a real session for the selected account without a password. The
development endpoints are `GET /api/auth/users` (cursor-paginated) and
`POST /api/auth/assume` with `{"userId":"…"}`. This workspace intentionally
enables this account-switching workflow while the game is being developed.
The picker is off unless `DEVELOPMENT_LOGIN=true` (set in `.env.example`, and by
the Helm value `developmentLogin.enabled`). When off, both endpoints return 404 and
the sign-in page hides the panel. Never enable it on a host other people can reach:
it signs anyone in as any account, including administrators.

Screens have shareable hash routes, such as `#/campaigns`, `#/shop/boxes`,
`#/skills/energy`, `#/saved-games`, and `#/password`. Refresh and browser
Back/Forward preserve the selected destination. **Cards & rules** is available
to every signed-in player, and the rules overview is also on the login page.
Commander Record lets players save a display name.

Campaigns can launch their scripted missions in the browser, including hero
selection, prerequisites, energy costs, progress and first-completion rewards.
Completed missions offer free practice replays without repeated progression
rewards. Missions and their hidden state persist through refresh like ordinary
matches. **Choose match mode** can replace an active match with another mode;
**Restart this mode** restarts its current configuration. The unfinished match
is archived in Saved games. Battles wait for **Continue** by default, with an
optional auto-advance setting. Mobile play places the current hand first.

Use **Password** to change your password after confirming the current one.
The current session is rotated and stays signed in; all other sessions are
revoked. Players also have a **Cards & rules** page: its contact sheet covers
the 27-card deck and all 72 embedded artwork files. Clicking a thumbnail opens
a detail panel with suit rules, color/pose variants, and image metadata.

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

The UI offers Human vs Human and Human vs AI at Easy, Normal, or Hard. A mode
can be preselected:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode hvh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode easy
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode normal
GOWORK=off go -C backend run ./cmd/prisoners-of-war --mode hard
```

Each match selects heroes before play. The browser offers hero choices when
starting a match; Human-vs-AI uses your saved default and lets the AI counter
your last opponent. The terminal [8] Heroes screen lets you set a default and
view hero records. The active modifiers and round weights are shown throughout
the game. See [heroes](docs/heroes.md) for the roster, rules, and
simulator details, and [hero acquisition](docs/heroes-acquisition.md)
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

The terminal starts with a progression menu for CPU games, local Ranked and
friend matches, profile, heroes, campaigns, shop, skills, Camp Draws, and
energy actions. See [campaigns](docs/campaigns.md) for the full
mission list, scripted setups, progression rules, and rewards. The Shop offers
chromas, Hero Boxes, and direct hero purchases.
At commit, enter exactly three distinct hand numbers separated by spaces or
commas. During a battle, enter the number of a pile card. At the start of
Round 3, the eligible player enters `X` or `Y`. Invalid entries are rejected.
ANSI suit colors can be disabled with `--no-color` or the `NO_COLOR`
environment variable. Progression details and payout rules are in
[economy](docs/economy.md).

Print the economy configuration, including payouts, skills, chromas and
bundles, as JSON:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --dump-economy
```

## AI

- **Easy** chooses random legal cards and seats.
- **Normal** chooses commits using immediate score value, attacks randomly,
  and makes greedy defenses.
- **Hard** evaluates commits and seat choices against sampled unseen cards,
  uses a sampled worst-case attack choice, and makes greedy defenses. It only
  samples cards that are not known to the AI.

## Tests and simulation

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

Run a headless Hard-vs-Hard simulation and print summary statistics with:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --sim 10000
```

Run all 66 hero pairings with greedy-vs-greedy and greedy-vs-random samples:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war --sim-heroes 100
```
