#!/bin/bash
set -euo pipefail
mkdir -p /home/dev/.ssh /run/sshd
if [ ! -f /home/dev/.uid-1100-migrated ]; then
  chown -R dev:dev /home/dev
  touch /home/dev/.uid-1100-migrated
fi
chmod 700 /home/dev /home/dev/.ssh
install -o dev -g dev -m 600 /etc/workspace-ssh/authorized_keys /home/dev/.ssh/authorized_keys
if [ ! -f /home/dev/.ssh/ssh_host_ed25519_key ]; then
  ssh-keygen -q -t ed25519 -N '' -f /home/dev/.ssh/ssh_host_ed25519_key
fi
umask 077
for name in PGPASSWORD PGHOST PGUSER PGDATABASE PUBLIC_URL COOKIE_SECURE BOOTSTRAP_ADMIN_EMAIL BOOTSTRAP_ADMIN_PASSWORD DEVELOPMENT_LOGIN GOWORK; do
  printf 'export %s=%q\n' "$name" "${!name}"
done > /run/pow-dev.env
chown root:dev /run/pow-dev.env
chmod 640 /run/pow-dev.env
printf '%s\n' 'export PATH=/usr/local/go/bin:/home/dev/go/bin:$PATH' '. /run/pow-dev.env' > /etc/profile.d/pow-dev.sh
chmod 644 /etc/profile.d/pow-dev.sh
exec /usr/sbin/sshd -D -e -f /dev/null \
  -o Port=2222 \
  -o ListenAddress=127.0.0.1 \
  -o HostKey=/home/dev/.ssh/ssh_host_ed25519_key \
  -o AuthorizedKeysFile=/home/dev/.ssh/authorized_keys \
  -o PasswordAuthentication=no \
  -o KbdInteractiveAuthentication=no \
  -o PermitRootLogin=no \
  -o AllowUsers=dev \
  -o AllowTcpForwarding=local \
  -o AllowAgentForwarding=yes \
  -o X11Forwarding=no \
  -o PermitUserEnvironment=no \
  -o UsePAM=no
