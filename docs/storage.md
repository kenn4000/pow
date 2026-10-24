# Accounts, storage, and HTTP routing

## PostgreSQL

Web mode uses PostgreSQL whenever `DATABASE_URL` or `PGHOST` is configured.
`PGHOST`, `PGPORT` (default 5432), `PGDATABASE`, `PGUSER`, and `PGPASSWORD`
are supported; `POSTGRES_PASSWORD` is a local fallback for `PGPASSWORD`.
`DATABASE_URL` takes precedence and can configure TLS for an external database.
The Helm release runs a PostgreSQL 17 StatefulSet with its own persistent volume.

Versioned SQL migrations live in `backend/storage/migrations/` and run in a
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
            ├── /admin                  admin role middleware
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
| GET | `/api/admin/users?after=<id>` | Admin-only account list, 100 per page |
| PATCH | `/api/admin/users/<id>/role` | Admin-only `{"role":"user"}` or `{"role":"admin"}` |
| GET | `/api/games?before=<id>` | Own match list, 50 per page |
| GET | `/api/games/<id>?after=<sequence>` | Own redacted transition history, 100 per page |

Lists include a `next` cursor when more records are available. Existing game
and progression endpoints keep their payloads and require a session in
PostgreSQL mode.
