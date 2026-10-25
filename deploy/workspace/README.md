# Shared remote OpenCode workspace

Namespace: `prisoner-of-war-dev`. Deployment: `workspace`.
The image includes OpenCode 1.19.4, Go 1.26, Node 22, GCC, Git and tmux.

| Location | User | Repository |
| --- | --- | --- |
| k3s host | `powdev` (UID/GID 1100) | `/srv/pow-dev/repo` |
| Workspace pod | `dev` (UID/GID 1100) | `/home/dev/repo` |

The repository is the same directory through a hostPath mount. Changes made
by rsync, a host shell, or OpenCode in the pod are immediately visible to the
other environments. All teammates share this working tree and tmux sessions;
coordinate concurrent edits and branch changes.

The dedicated shared private key is provisioned separately at
`keys/pow-dev_ed25519` and excluded from Git. Obtain it from the trusted team
before connecting. Its public key is tracked alongside the instructions.
This team key is separate from the administrator's SSH key.
The host account has no sudo, Docker group or Kubernetes credentials.

## Connect

From the repository root on your Mac:

```sh
./deploy/workspace/ssh.sh
```

The wrapper sets the shared key's permissions and resolves its path from any
working directory. It connects through `powdev@66.63.184.194:8888` to the pod
as `dev`. Verify host fingerprints on your first connection. The pod's host
key persists on its home PVC.

Inside the workspace:

```sh
pow-dev up
tmux new-session -A -s opencode 'cd /home/dev/repo && opencode'
```

Use OpenCode's `/connect` to authenticate your provider. Its credentials,
configuration and session history persist in `/home/dev`. Detach from tmux
with Ctrl-b then d. tmux survives SSH disconnects; after a pod restart,
reconnect, run `pow-dev up`, and start OpenCode again.

Open a non-root shell on the host instead:

```sh
./deploy/workspace/ssh.sh pow-host
```

## Sync local code

From your Mac, push local edits into the shared host directory:

```sh
./deploy/workspace/sync.sh push --dry-run --itemize-changes
./deploy/workspace/sync.sh push
```

Pull remote edits back to your local checkout:

```sh
./deploy/workspace/sync.sh pull --dry-run --itemize-changes
./deploy/workspace/sync.sh pull
```

The helpers exclude `.git`, private workspace keys, local environment files,
dependencies, build output and macOS metadata. `.env.example` is included.
They update files without deleting destination-only files; synchronize
intentional deletions separately. A push or pull can overwrite edits to the
same file, so review the dry run and coordinate with other workspace users.
Git history and branches remain local to each checkout; use GitLab to exchange
commits. The host checkout was seeded with the existing repository history.

The underlying rsync destination is:

```text
pow-host:/srv/pow-dev/repo/
```

## Preview locally

Keep the pod SSH session open, run `pow-dev up`, then visit:

```text
http://localhost:5173/prisoner-of-war/
```

Local ports 5173 and 8080 must be free. Vite provides frontend hot reload and
proxies API requests to the Go backend. The database is a separate PostgreSQL
17 sidecar with a persistent 5 GiB volume. The development login picker lets
you sign in as `admin@example.com`, or you can register another account.

After backend edits, restart its tmux session:

```sh
tmux kill-session -t pow-backend
pow-dev up
```

Run the race/integration suite, complexity lint and frontend build:

```sh
pow-dev test
```

View logs or stop the preview:

```sh
tmux attach -t pow-backend
tmux attach -t pow-frontend
pow-dev down
```

## GitLab authentication

Git operations on the host and in the pod use the dedicated GitLab account
[`pow-workspace`](https://gitlabs.markdownthing.com/pow-workspace), with
Developer access to `kenn/prisoners-of-war`. The shared public key is registered
to that account. Git commits use `Prisoners of War Workspace` and
`pow-workspace@markdownthing.com`.

The private key and pinned GitLab host key are installed in
`~/.ssh/pow-gitlab/` on both accounts. The shared checkout's `core.sshCommand`
selects that configuration; SSH-agent forwarding is unnecessary. GitLab's
host key was verified against the GitLab container's own public host key.

Inside the pod:

```sh
cd /home/dev/repo
git fetch origin
git switch -c feature/my-change
git push -u origin feature/my-change
```

`main` remains protected. Push feature branches and open merge requests.
To reinstall Git credentials after provisioning a new home volume, run from
your Mac:

```sh
./deploy/workspace/setup-git-access.sh
```

The setup updates only the dedicated SSH configuration and this checkout's
Git identity. The private key persists in the host home and pod home PVC.

## Transport and persistence

The root-managed `pow-workspace-tunnel.service` forwards host loopback port
22220 to pod loopback port 2222. It reconnects after pod replacements. The
host's SSH configuration permits `powdev` TCP forwarding only to that loopback
port. SSH, Vite, Go and PostgreSQL listen on pod loopback; no public preview
ingress or NodePort is created. The pod's SSH daemon accepts only `dev`;
development sessions are non-root and privilege escalation is disabled.

The home PVC stores OpenCode state and caches; the database uses its own PVC.
The repo hostPath and local-path PVCs are tied to node `server`. They persist
through pod replacement, but are not backups. The original repo snapshot on
the home PVC is retained underneath the new repo mount.

## Administrator setup

The following commands require `k3s_personal_box_root`. Team members need only
the shared key and the connection scripts.

Run from the repository root. Bootstrap the host account, public key, data
directory, SSH restrictions and loopback tunnel:

```sh
COPYFILE_DISABLE=1 tar -C deploy/workspace -czf - setup-host.sh 90-powdev.conf pow-workspace-tunnel.service keys/pow-dev_ed25519.pub |
  ssh k3s_personal_box_root 'umask 077; staging=$(mktemp -d /tmp/pow-workspace-host.XXXXXX) && tar -xzf - -C "$staging" && bash "$staging/setup-host.sh"'
```

Build and import the image into this single node's containerd store:

```sh
COPYFILE_DISABLE=1 tar -C deploy/workspace -czf - Dockerfile entrypoint.sh pow-dev.sh |
  ssh k3s_personal_box_root 'docker build -t pow-workspace:1.19.4-4 - && docker save pow-workspace:1.19.4-4 | k3s ctr images import -'
```

Create credentials and deploy:

```sh
ssh k3s_personal_box_root 'kubectl create namespace prisoner-of-war-dev --dry-run=client -o yaml | kubectl apply -f -'
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war-dev create secret generic workspace-ssh --from-file=authorized_keys=/dev/stdin --dry-run=client -o yaml | kubectl apply -f -' < deploy/workspace/keys/pow-dev_ed25519.pub
ssh k3s_personal_box_root 'if ! kubectl -n prisoner-of-war-dev get secret workspace-passwords >/dev/null 2>&1; then kubectl -n prisoner-of-war-dev create secret generic workspace-passwords --from-literal=postgres-password="$(openssl rand -hex 24)" --from-literal=admin-password="$(openssl rand -hex 24)"; fi'
ssh k3s_personal_box_root 'kubectl apply -f - && kubectl -n prisoner-of-war-dev rollout status deployment/workspace --timeout=180s' < deploy/workspace/workspace.yaml
```

For a new empty data directory only, seed the repository and Git history:

```sh
git ls-files -z | COPYFILE_DISABLE=1 tar --null -T - -czf - .git |
  ./deploy/workspace/ssh.sh pow-host 'tar -xzf - -C /srv/pow-dev/repo'
./deploy/workspace/sync.sh push
```

Use a new image tag and update `workspace.yaml` for future image changes.
`Recreate` prevents simultaneous pods sharing PostgreSQL and home storage.
An image update disconnects SSH and stops tmux; allow a few seconds for the
host tunnel to reconnect, then reconnect and run `pow-dev up`.

## Operations

```sh
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war-dev get pods,pvc'
ssh k3s_personal_box_root 'systemctl status pow-workspace-tunnel --no-pager'
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war-dev logs deployment/workspace -c workspace'
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war-dev logs deployment/workspace -c postgres'
```
