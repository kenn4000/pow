#!/bin/sh
set -eu
dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH='' cd -- "$dir/../.." && pwd)
case "${1:-push}" in
  push) source="$root/"; target='pow-host:/srv/pow-dev/repo/' ;;
  pull) source='pow-host:/srv/pow-dev/repo/'; target="$root/" ;;
  *) exit 2 ;;
esac
if [ "$#" -gt 0 ]; then shift; fi
exec rsync -az --omit-dir-times --no-perms \
  --exclude='.git/' --include='.env.example' --exclude='.env' --exclude='.env.*' \
  --exclude='node_modules/' --exclude='dist/' --exclude='.DS_Store' --exclude='._*' \
  --exclude='/backend/bin/' --exclude='/frontend/.vite/' \
  --exclude='/deploy/workspace/keys/' \
  -e "\"$dir/ssh.sh\"" "$@" "$source" "$target"
