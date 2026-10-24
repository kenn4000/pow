# Backend

```text
cmd/prisoners-of-war/  Executable entrypoint
internal/app/         HTTP routes, accounts, admin features, game sessions, and terminal orchestration
internal/game/        Card rules and game state
internal/ai/          Opponent strategies
internal/storage/     PostgreSQL connection and versioned SQL migrations
internal/profile/     Player profiles and local JSON persistence
internal/skins/       Embedded card artwork
internal/             Other game and progression packages
Dockerfile            Production image build
Makefile              Run, build, test, and complexity lint commands
go.mod, go.sum        Go module metadata
```

All implementation packages are internal to this module. Tests live alongside
the code they exercise. The command delegates to `internal/app.Run`, which
selects web, terminal, or simulation mode using the existing flags.

From the repository root:

```sh
make -C backend run
make -C backend run ARGS="--ui terminal"
make -C backend build
make -C backend test
make -C backend lint
make -C backend clean
```

The build writes `backend/bin/prisoners-of-war`, which is ignored by Git and
Docker. To use Go directly:

```sh
GOWORK=off go -C backend run ./cmd/prisoners-of-war
GOWORK=off go -C backend test -race ./...
```

PostgreSQL integration tests require `TEST_DATABASE_URL`; CI supplies a
PostgreSQL service automatically. See [local development](../README.md),
[storage and routing](../docs/storage.md), and [deployment](../deploy/README.md)
for environment variables and application setup.
