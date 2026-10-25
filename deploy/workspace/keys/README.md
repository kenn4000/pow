# Shared workspace key

`pow-dev_ed25519` is the dedicated team private key for the `powdev` host
account and `dev` workspace account. `pow-dev_ed25519.pub` is its public key.
This is separate from the server administrator's key.

The private key is provisioned separately to trusted teammates at this path
and is excluded from Git. The public key and setup instructions are tracked.

The key is shared by the trusted team; every holder has the same identity and
access. The host account has no sudo, Docker group, or Kubernetes credentials.
It can edit `/srv/pow-dev/repo` and connect to the workspace SSH tunnel.

Run `../ssh.sh` to set the private key's local permissions before connecting.
To revoke shared access, replace this key and its authorized public key on the
host and in the `workspace-ssh` Kubernetes Secret, update the GitLab user's SSH
key, rerun `../setup-git-access.sh`, then restart the deployment.
