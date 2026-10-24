# Deployment

## Target

- URL: `https://games.markdownthing.com/prisoner-of-war/`
- k3s host: `ssh k3s_personal_box_root`
- Namespace and Helm release: `prisoner-of-war`
- Ingress: Traefik with `letsencrypt-prod` certificates
- Profile storage: 1 GiB `local-path` PVC, mounted at `/data`

Ingress sends `/prisoner-of-war/api` and `/prisoner-of-war/skins` to Go,
and the remaining `/prisoner-of-war` paths to the frontend. No Traefik
rewrite middleware is needed. Vite's base URL, nginx, and the Go `BASE_PATH`
setting agree on this prefix. nginx redirects the URL without a trailing slash.

The current game uses **one shared profile and active game for all visitors**.
The backend therefore runs exactly one replica with a `Recreate` deployment
strategy. Profile progress survives restarts; an in-progress match is in memory
and resets on restart. A deployment briefly interrupts play. The PVC is retained
on Helm uninstall. Per-user accounts and isolated matches require a separate
application change.

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

Merge requests and branch pipelines run Go race tests, the frontend build,
and Helm validation. The default branch then builds two commit-SHA-tagged
images with Kaniko (the existing Kubernetes runner is non-privileged), pushes
them to `$CI_REGISTRY_IMAGE/{backend,frontend}`, and deploys directly in-cluster.
Deployments are serialized, use Helm `--atomic --wait`, and check the public
frontend and API. Build artifacts use the GitLab-provided registry credentials.

The DNS A record already resolves to `66.63.184.194`. cert-manager issues the
host certificate when the release creates the Ingress.

## Local validation

```sh
GOWORK=off go -C backend test -race ./...
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
ssh k3s_personal_box_root 'KUBECONFIG=/etc/rancher/k3s/k3s.yaml helm -n prisoner-of-war history prisoner-of-war'
ssh k3s_personal_box_root 'KUBECONFIG=/etc/rancher/k3s/k3s.yaml helm -n prisoner-of-war rollback prisoner-of-war'
```

No private keys, registry tokens, or kubeconfigs belong in this repository.
