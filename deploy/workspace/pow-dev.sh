#!/bin/bash
set -euo pipefail
export PATH=/usr/local/go/bin:/home/dev/go/bin:$PATH
. /run/pow-dev.env
cd /home/dev/repo
case "${1:-up}" in
  up)
    pg_isready -h "$PGHOST" -U "$PGUSER" -d "$PGDATABASE"
    if [ ! -d frontend/node_modules ]; then npm --prefix frontend ci; fi
    tmux has-session -t pow-backend 2>/dev/null || tmux new-session -d -s pow-backend 'bash -lc "cd /home/dev/repo && go -C backend run ./cmd/prisoners-of-war --listen 127.0.0.1:8080"'
    tmux has-session -t pow-frontend 2>/dev/null || tmux new-session -d -s pow-frontend 'bash -lc "cd /home/dev/repo && npm --prefix frontend run dev -- --host 127.0.0.1 --port 5173 --strictPort"'
    tmux list-sessions
    ;;
  down)
    tmux kill-session -t pow-frontend 2>/dev/null || true
    tmux kill-session -t pow-backend 2>/dev/null || true
    ;;
  test)
    TEST_DATABASE_URL="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:5432/${PGDATABASE}?sslmode=disable" go -C backend test -p 2 -race ./...
    make -C backend lint
    npm --prefix frontend ci
    npm --prefix frontend run build
    ;;
  *) exit 2 ;;
esac
