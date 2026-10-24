# Deployment

## Target

- URL: `https://games.markdownthing.com/prisoner-of-war/`
- k3s host: `ssh k3s_personal_box_root`
- Namespace and Helm release: `prisoner-of-war`
- Ingress: Traefik with `letsencrypt-prod` certificates
- Database: PostgreSQL 17 StatefulSet with a 5 GiB `local-path` PVC

Ingress sends `/prisoner-of-war/api` and `/prisoner-of-war/skins` to Go,
and the remaining `/prisoner-of-war` paths to the frontend. No Traefik
rewrite middleware is needed. Vite's base URL, nginx, and the Go `BASE_PATH`
setting agree on this prefix. nginx redirects the URL without a trailing slash.

Accounts, sessions, profiles, active games, match history, and state transitions
are stored in PostgreSQL. Each account is isolated; restarting the backend
resumes games from committed snapshots. The chart uses a PostgreSQL StatefulSet
and a stable Secret for its generated database password. Subsequent Helm
upgrades reuse that password; `postgres.existingSecret` can supply your own
Secret with a `password` key. The database Secret and StatefulSet data volume
are retained on uninstall. Back up PostgreSQL before destructive database or
storage maintenance; a local-path volume is tied to this node.

The previous `prisoner-of-war-profile` PVC is retained as a legacy archive.
The new release does not automatically assign its shared profile to a new
account. `persistence.enabled=false` stops mounting it on the backend.

## GitLab CI/CD setup

Under **Settings → CI/CD → Variables**, create these protected variables and
protect the default branch:

| Variable | Type | Value |
| --- | --- | --- |
| `REGISTRY_DEPLOY_USER` | Variable | GitLab deploy-token username with `read_registry` |
| `REGISTRY_DEPLOY_PASSWORD` | Masked variable | Matching long-lived deploy token |

Use a deploy token for cluster image pulls: the pipeline's `CI_JOB_TOKEN`
expires and cannot support future pod restarts. The pipeline maintains the
`gitlab-registry` image-pull secret in the application namespace.

CI runs inside the cluster. Helm and kubectl authenticate directly to the
Kubernetes API using the job pod's mounted service-account token. No SSH key
or kubeconfig CI variable is needed.

Bootstrap the namespace and namespaced deployment permissions once from your
workstation:

```sh
ssh k3s_personal_box_root 'kubectl apply -f -' < deploy/ci-rbac.yaml
```

The current runner configuration starts job pods in `gitlab-runner` with its
`default` service account. `ci-rbac.yaml` binds that account to deployment
permissions only in `prisoner-of-war`, including Helm release secrets and
registry credentials. Jobs sharing this runner service account share those
namespace permissions. If a dedicated project runner is introduced, update
the RoleBinding subject to its job service account.

Reapply `ci-rbac.yaml` before the first database-enabled deployment: its Role
now includes StatefulSets. Go CI tests use a PostgreSQL service and exercise
real migrations, accounts, role checks, state isolation, restart recovery,
rollback behavior, and match history.

## Initial administrator

Create a Secret containing `email` and `password` in the application namespace.
The password must be 12–72 bytes. From an interactive shell:

```sh
read -r ADMIN_EMAIL
read -rs ADMIN_PASSWORD
printf '\n'
kubectl -n prisoner-of-war create secret generic prisoner-of-war-bootstrap-admin \
  --from-literal=email="$ADMIN_EMAIL" --from-literal=password="$ADMIN_PASSWORD"
unset ADMIN_PASSWORD
```

Run kubectl through your configured cluster context, or perform this setup on
`ssh k3s_personal_box_root`. Set the GitLab CI variable
`BOOTSTRAP_ADMIN_SECRET=prisoner-of-war-bootstrap-admin`, then deploy. Manual
Helm invocations can instead set
`--set bootstrapAdmin.existingSecret=prisoner-of-war-bootstrap-admin`.
No account becomes an admin simply by registering first. Existing bootstrap
accounts are not reset on restart. Remove the bootstrap Secret reference once
an admin is established; the account remains in PostgreSQL.

Merge requests and branch pipelines run the enforced Go complexity linter,
Go race tests, the frontend build,
and Helm validation. The default branch then builds two commit-SHA-tagged
images with Kaniko (the existing Kubernetes runner is non-privileged), pushes
them to `$CI_REGISTRY_IMAGE/{backend,frontend}`, and deploys directly in-cluster.
Deployments are serialized, use Helm `--atomic --wait`, and check the public
frontend and confirms that unauthenticated game/account requests return 401.
Build artifacts use the GitLab-provided registry credentials.

The DNS A record already resolves to `66.63.184.194`. cert-manager issues the
host certificate when the release creates the Ingress.

## Local validation

```sh
GOWORK=off go -C backend test -race ./...
make -C backend lint
GOWORK=off go -C backend build ./...
npm --prefix frontend ci
npm --prefix frontend run build
helm lint deploy/helm/prisoner-of-war --strict
helm template prisoner-of-war deploy/helm/prisoner-of-war |
  ssh k3s_personal_box_root 'kubectl apply --dry-run=server -n default -f -'
docker build -f backend/Dockerfile -t prisoner-of-war-backend .
docker build -f frontend/Dockerfile -t prisoner-of-war-frontend .
```

`GOWORK=off` avoids interference from a parent directory's Go workspace.

## Operations

```sh
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war get pods,svc,ingress,pvc'
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war logs deployment/prisoner-of-war-backend'
ssh k3s_personal_box_root 'kubectl -n prisoner-of-war logs statefulset/prisoner-of-war-postgres'
ssh k3s_personal_box_root 'KUBECONFIG=/etc/rancher/k3s/k3s.yaml helm -n prisoner-of-war history prisoner-of-war'
ssh k3s_personal_box_root 'KUBECONFIG=/etc/rancher/k3s/k3s.yaml helm -n prisoner-of-war rollback prisoner-of-war'
```

No private keys, registry tokens, or kubeconfigs belong in this repository.
