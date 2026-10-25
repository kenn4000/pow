#!/bin/sh
set -eu
POW_WORKSPACE_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
export POW_WORKSPACE_DIR
chmod 600 "$POW_WORKSPACE_DIR/keys/pow-dev_ed25519"
if [ "$#" -eq 0 ]; then set -- pow-dev; fi
exec ssh -F "$POW_WORKSPACE_DIR/ssh_config" "$@"
