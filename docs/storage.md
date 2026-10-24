# Accounts, storage, and HTTP routing

## PostgreSQL

Web mode uses PostgreSQL whenever `DATABASE_URL` or `PGHOST` is configured.
`PGHOST`, `PGPORT` (default 5432), `PGDATABASE`, `PGUSER`, and `PGPASSWORD`
are supported; `POSTGRES_PASSWORD` is a local fallback for `PGPASSWORD`.
`DATABASE_URL` takes precedence and can configure TLS for an external database.
The Helm release runs a PostgreSQL 17 StatefulSet with its own persistent volume.

Versioned SQL migrations live in `backend/internal/storage/migrations/` and run in a
transaction under an advisory lock before the server starts accepting requests.

| Table | Contents |
| --- | --- |
| `users` | Normalized unique email, bcrypt password hash, user/admin role |
| `auth_sessions` | SHA-256 hashes of random session tokens, expiry, account ownership |
| `auth_attempts` | Per-email authentication attempt windows |
| `player_data` | Per-account profile and versioned active-game/Camp Draw snapshots |
| `games` | All active, completed, and abandoned matches and final records |
| `game_events` | Ordered actions, private authoritative snapshots, and redacted player views |
| `role_audit` | Administrator, target account, old role, new role, timestamp |

Each game request locks only the account's `player_data` row. Profile changes,
game snapshots, events, and match outcomes commit together. The HTTP response is
buffered until commit succeeds. Failed actions roll back. Different users have
independent profiles and games; simultaneous requests from one account are
serialized in PostgreSQL. Clients can send an `If-Match` header using the
`X-State-Version` response header to reject stale actions with HTTP 409.

Private snapshots preserve hands, deck order, phase, active actor, pending
handoffs, battle results, payouts, and Camp Draw progress. RNG is freshly seeded
per request; subsequent random choices are recorded in the committed state.
Historical APIs return redacted views, never private snapshots or another
account's games. The profile retains its latest 50 match summaries for the
existing dashboard; the `games` table retains the complete history and has
cursor-based pagination.

Without database configuration, standalone web/terminal mode still uses the
original local JSON profile. This local mode has no account system. Existing
shared production profile files are not assigned to any newly registered user;
the previous profile PVC is retained for a deliberate data import later.

## Authentication and roles

Signup always creates a `user`, even if a request supplies a role. Passwords
must be 12–72 bytes and are stored as bcrypt hashes. Seven-day sessions use a
random token in an HttpOnly, SameSite=Lax cookie. Production sets
`COOKIE_SECURE=true` and `PUBLIC_URL=https://games.markdownthing.com`.
Mutation requests with foreign origins are rejected. Login/signup attempts
are limited per email to 15 in a 15-minute window. Signing out deletes the
server-side session. Roles are read from PostgreSQL on every request.

The `user` role can access its own game/profile/history. The `admin` role also
lists accounts and changes roles. Role changes are audited. Concurrent role
changes are serialized, and the last administrator cannot be demoted.

Authenticated users can change their password through **Password** or
`POST /api/account/password` with `currentPassword` and `newPassword`.
The current password must be correct and the new one must differ and meet
the 12–72 byte policy. The hash update, revocation of all previous sessions,
and creation of a replacement session commit atomically. Only after commit
does the response set the replacement HttpOnly cookie. Password changes are
rate-limited with login/signup attempts. Login and password change both lock
the account row, so a concurrent old-password login cannot leave a valid
session behind after the change. Bootstrap does not reset a changed password.

Create the initial admin with `BOOTSTRAP_ADMIN_EMAIL` and
`BOOTSTRAP_ADMIN_PASSWORD` at startup. A public signup is never automatically
made an administrator. Repeated bootstrap does not reset passwords or override
role changes on existing accounts. Do not keep the bootstrap password in Git.

## Nested chi groups

```text
root                                    panic recovery
├── GET /healthz                        process health
├── GET /readyz                         PostgreSQL readiness
└── BASE_PATH                           inherited application prefix
    ├── GET /
    ├── /skins                          public assets
    │   └── GET /*
    └── /api                            shared origin validation
        ├── /auth                       public login/signup endpoints
        │   └── GET/POST /{authAction}
        └── protected group             session authentication
            ├── /account
            │   └── POST /password
            ├── /admin                  admin role middleware
            │   ├── GET /cards          deck and artwork metadata
            │   └── /users
            │       ├── GET /
            │       └── PATCH /{userID}/role
            ├── /games                  account-owned history
            │   ├── GET /
            │   └── GET /{gameID}
            └── game group              per-user PostgreSQL transaction
                ├── GET /state
                ├── POST /new
                ├── POST /action
                └── progression endpoints
```

Every subrouter registers relative routes and inherits its parents' prefixes.
`BASE_PATH=/prisoner-of-war` is mounted once; no prefix-stripping router is used.
Identity and the request-scoped game server are passed through request context.
Authentication, role checks, and game transactions run at their respective
group scopes. Health checks and artwork remain public.

## API

All paths below are relative to `BASE_PATH` and return JSON.

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/auth/signup` | Register with `email` and `password`; set session cookie |
| POST | `/api/auth/login` | Sign in with `email` and `password` |
| POST | `/api/auth/logout` | Revoke the current session |
| GET | `/api/auth/me` | Current account and role, or HTTP 401 |
| POST | `/api/account/password` | Confirm `currentPassword`, set `newPassword`, rotate session and revoke others |
| GET | `/api/admin/users?after=<id>` | Admin-only account list, 100 per page |
| PATCH | `/api/admin/users/<id>/role` | Admin-only `{"role":"user"}` or `{"role":"admin"}` |
| GET | `/api/admin/cards` | Admin-only 27-card deck and all embedded artwork with dimensions, size, tier, pose, and color |
| GET | `/api/games?before=<id>` | Own match list, 50 per page |
| GET | `/api/games/<id>?after=<sequence>` | Own redacted transition history, 100 per page |

Lists include a `next` cursor when more records are available. Existing game
and progression endpoints keep their payloads and require a session in
PostgreSQL mode.

## Admin card contact sheet

The **Cards** page has deck and artwork views, color selection, search, and a
keyboard-accessible detail panel. The deck view includes every suit/face pair;
the artwork view includes all PNG files discovered in the embedded skin
filesystem. Dimensions are read from PNG headers without decoding full images.
The panel displays all available color/pose variations for the selected tier,
and its Close button or Escape key returns focus to the selected thumbnail.
Image requests remain under the inherited application base path.

## Complexity enforcement

`make -C backend lint` runs gocyclo v0.6.0 with `-over 20` across every
production Go package. Any function above 20 fails the command and the
GitLab test stage. Only `_test.go` files are excluded. Existing violations
were refactored into validation, rendering, game-phase, and progression
helpers rather than allowlisted.
