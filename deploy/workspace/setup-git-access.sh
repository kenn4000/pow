#!/bin/sh
set -eu
dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
for target in pow-host pow-dev; do
  case "$target" in
    pow-host) repo=/srv/pow-dev/repo ;;
    pow-dev) repo=/home/dev/repo ;;
  esac
  COPYFILE_DISABLE=1 tar -C "$dir" -czf - gitlab_ssh_config gitlab_known_hosts -C "$dir/keys" pow-dev_ed25519 |
    "$dir/ssh.sh" -T -o RemoteCommand=none -o ClearAllForwardings=yes "$target" "
      set -eu
      umask 077
      mkdir -p ~/.ssh/pow-gitlab
      chmod 700 ~/.ssh ~/.ssh/pow-gitlab
      tar -xzf - -C ~/.ssh/pow-gitlab
      chmod 600 ~/.ssh/pow-gitlab/*
      git -C '$repo' config core.sshCommand 'ssh -F ~/.ssh/pow-gitlab/gitlab_ssh_config'
      git -C '$repo' config user.name 'Prisoners of War Workspace'
      git -C '$repo' config user.email 'pow-workspace@markdownthing.com'
    "
done
