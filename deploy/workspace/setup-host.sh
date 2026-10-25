#!/bin/bash
set -euo pipefail
dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
test "$(id -u)" = 0
if ! id powdev >/dev/null 2>&1; then
  useradd --create-home --uid 1100 --user-group --shell /bin/bash powdev
fi
test "$(id -u powdev)" = 1100
test "$(id -g powdev)" = 1100
install -d -o powdev -g powdev -m 700 /home/powdev/.ssh
install -o powdev -g powdev -m 600 "$dir/keys/pow-dev_ed25519.pub" /home/powdev/.ssh/authorized_keys
install -d -o root -g root -m 755 /srv/pow-dev
install -d -o powdev -g powdev -m 2770 /srv/pow-dev/repo
install -o root -g root -m 644 "$dir/90-powdev.conf" /etc/ssh/sshd_config.d/90-powdev.conf
/usr/sbin/sshd -t
systemctl reload ssh
install -o root -g root -m 644 "$dir/pow-workspace-tunnel.service" /etc/systemd/system/pow-workspace-tunnel.service
systemctl daemon-reload
systemctl enable --now pow-workspace-tunnel.service
