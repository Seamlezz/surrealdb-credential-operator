# surrealdb-credential-operator

Kubernetes operator for platform teams that want app namespaces to request scoped SurrealDB system users without receiving root credentials.

Current status: early development. API group is `surrealdb.seamlezz.com/v1alpha1`, so expect breaking changes before stable release.

## Why this exists

SurrealDB root credentials are powerful. In Kubernetes, app teams often need DB access, but they should not need root passwords or manual user creation tickets.

This operator gives platform teams three controls:

- Provider: cluster scoped SurrealDB endpoint plus admin Secret reference.
- Tenant policy: namespace scoped allow list for SurrealDB namespace, DB names, and roles.
- Credential request: app owned request that creates one SurrealDB system user and one Kubernetes Secret.

Use this when you want delegated SurrealDB credential creation inside Kubernetes. Do not use it when you cannot trust controller RBAC with Kubernetes Secret access and SurrealDB admin credentials, or when you need SurrealDB record users rather than system users.

## What it does

- Validates provider Secret and TLS references.
- Evaluates `SurrealDBCredential` requests against same namespace `SurrealDBTenantPolicy` objects.
- Defines SurrealDB users with `DEFINE USER OVERWRITE` at namespace or DB scope.
- Writes generated connection data into a Kubernetes Secret.
- Reuses existing generated passwords until rotation is requested or due.
- Removes managed SurrealDB users and owned target Secrets when credentials are deleted.

Generated Secret keys include:

- `url`
- `namespace`
- `database`
- `username`
- `password`
- `level`
- `SURREAL_URL`
- `SURREAL_NS`
- `SURREAL_DB`
- `SURREAL_USER`
- `SURREAL_PASS`

## Requirements

- Kubernetes 1.31 or newer for Helm chart installs.
- Helm 3 for chart install, or `kubectl`, `kustomize`, and `make` for local deploy.
- Reachable SurrealDB endpoint from controller Pod.
- Kubernetes Secret containing SurrealDB admin credentials.
- Go 1.24 for development.

Published release workflow targets:

- Controller image: `ghcr.io/seamlezz/surrealdb-credential-operator`
- Helm chart: `oci://ghcr.io/seamlezz/charts/surrealdb-credential-operator`

## Quick start

This path uses local chart files from this repository. Replace endpoint and credentials before applying.

Install controller:

```bash
helm install surrealdb-credential-operator \
  ./charts/surrealdb-credential-operator \
  --namespace surrealdb-credential-operator-system \
  --create-namespace
```

Create namespaces and admin Secret:

```bash
kubectl create namespace platform-secrets
kubectl create namespace demo

kubectl create secret generic surrealdb-root \
  --namespace platform-secrets \
  --from-literal=username=root \
  --from-literal=password='change-me'
```

Create provider, tenant policy, and credential request:

```bash
kubectl apply -f - <<'YAML'
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBProvider
metadata:
  name: primary
spec:
  endpoint: http://surrealdb.surrealdb.svc.cluster.local:8000
  rootCredentialRef:
    namespace: platform-secrets
    name: surrealdb-root
---
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBTenantPolicy
metadata:
  name: demo-surrealdb
  namespace: demo
spec:
  providerRef:
    name: primary
  surrealNamespace: demo
  databaseUsers:
    allowedDatabases:
      - app
    allowedRoles:
      - VIEWER
      - EDITOR
  namespaceUsers:
    allowed: false
---
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBCredential
metadata:
  name: app
  namespace: demo
spec:
  policyRef:
    name: demo-surrealdb
  level: Database
  database: app
  roles:
    - EDITOR
  targetSecret:
    name: surrealdb-app
  rotation:
    period: 720h
YAML
```

Check readiness:

```bash
kubectl wait --for=condition=Ready surrealdbprovider/primary --timeout=60s
kubectl wait --namespace demo --for=condition=Ready surrealdbtenantpolicy/demo-surrealdb --timeout=60s
kubectl wait --namespace demo --for=condition=Ready surrealdbcredential/app --timeout=60s
```

Read generated connection data:

```bash
kubectl get secret surrealdb-app \
  --namespace demo \
  -o jsonpath='{.data.SURREAL_USER}' | base64 --decode
```

Expected result: a generated SurrealDB username beginning with `k8s_demo_app_`, plus Secret keys listed above.

## Usage

### Install from OCI chart

For released versions, install from GHCR:

```bash
helm install surrealdb-credential-operator \
  oci://ghcr.io/seamlezz/charts/surrealdb-credential-operator \
  --version 0.1.0 \
  --namespace surrealdb-credential-operator-system \
  --create-namespace
```

### Install with Kustomize

```bash
IMG=ghcr.io/seamlezz/surrealdb-credential-operator:v0.1.0 make deploy
```

### Request namespace level user

Policy must allow namespace users:

```yaml
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBTenantPolicy
metadata:
  name: demo-surrealdb
  namespace: demo
spec:
  providerRef:
    name: primary
  surrealNamespace: demo
  namespaceUsers:
    allowed: true
    allowedRoles:
      - VIEWER
```

Credential omits `database`:

```yaml
apiVersion: surrealdb.seamlezz.com/v1alpha1
kind: SurrealDBCredential
metadata:
  name: namespace-reader
  namespace: demo
spec:
  policyRef:
    name: demo-surrealdb
  level: Namespace
  roles:
    - VIEWER
  targetSecret:
    name: surrealdb-namespace-reader
```

### Rotate password manually

Change `surrealdb.seamlezz.com/rotate-at` to any new value:

```bash
kubectl annotate surrealdbcredential app \
  --namespace demo \
  surrealdb.seamlezz.com/rotate-at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --overwrite
```

### Configure scheduled rotation

Set `spec.rotation.period` on `SurrealDBCredential`:

```yaml
spec:
  rotation:
    period: 720h
```

### Delete credential

```bash
kubectl delete surrealdbcredential app --namespace demo
```

Normal deletion removes the managed SurrealDB user and owned target Secret before removing the finalizer. If SurrealDB is unreachable and cleanup must continue, annotate the deleting credential with force cleanup before retrying. Force cleanup skips remote user removal, attempts owned target Secret deletion, logs/statuses any local Secret cleanup failure, and removes the finalizer anyway. Use it only after deciding the orphaned SurrealDB user risk is acceptable; owned Kubernetes Secrets also have owner references and are normally garbage-collected when the credential is deleted.

```bash
kubectl annotate surrealdbcredential app \
  --namespace demo \
  surrealdb.seamlezz.com/force-cleanup=true \
  --overwrite
```

## Core concepts

### `SurrealDBProvider`

Cluster scoped object owned by platform team. It points to a SurrealDB endpoint and a Kubernetes Secret with admin credentials. Default Secret keys are `username` and `password`.

Provider endpoints accept `http`, `https`, `ws`, and `wss`. When no custom TLS config is set, HTTP endpoints are converted to matching WebSocket endpoints for SurrealDB session behavior. Custom TLS config currently supports HTTP and HTTPS endpoints only.

### `SurrealDBTenantPolicy`

Namespace scoped allow list owned by platform team. It connects one Kubernetes namespace to one SurrealDB namespace and limits which DB names and roles app requests may use.

Supported roles are `VIEWER`, `EDITOR`, and `OWNER`.

### `SurrealDBCredential`

Namespace scoped request usually owned by app team. It references a policy in the same namespace, requests a namespace or DB level system user, and names the target Secret.

Generated username is deterministic from Kubernetes namespace, credential name, provider, SurrealDB namespace, DB, and level. Password is generated with `crypto/rand`, length 48, URL safe alphabet.

## Configuration

Helm values live in `charts/surrealdb-credential-operator/values.yaml`.

Common values:

- `image.repository`: controller image repository.
- `image.tag`: image tag. Defaults to chart app version when empty.
- `image.pullPolicy`: image pull policy.
- `replicaCount`: controller replicas.
- `leaderElection.enabled`: controller runtime leader election.
- `metrics.bindAddress`: metrics listen address.
- `metrics.secure`: secure metrics endpoint toggle.
- `health.probeBindAddress`: health probe listen address.
- `resources`: Pod resource requests and limits.
- `nodeSelector`, `tolerations`, `affinity`: scheduling controls.

Provider TLS options:

```yaml
spec:
  tls:
    caSecretRef:
      namespace: platform-secrets
      name: surrealdb-ca
      key: ca.crt
    clientCertificateRef:
      namespace: platform-secrets
      name: surrealdb-client-cert
      certKey: tls.crt
      keyKey: tls.key
```

Do not use `insecureSkipVerify: true` outside local development.

## Troubleshooting

### `root credential Secret platform-secrets/surrealdb-root not found`

Meaning: provider references a Secret that does not exist.

Fix: create Secret in referenced namespace or update `spec.rootCredentialRef`.

Check:

```bash
kubectl get secret surrealdb-root --namespace platform-secrets
```

### `root credential Secret platform-secrets/surrealdb-root missing key "username"`

Meaning: admin Secret is present, but expected key is missing or empty.

Fix: add `username` and `password`, or set `usernameKey` and `passwordKey` in provider spec.

### `database "app" not allowed by policy`

Meaning: credential requested DB not listed in `spec.databaseUsers.allowedDatabases`.

Fix: change credential DB or update tenant policy.

### `roles [OWNER] not allowed database-level users`

Meaning: requested role is absent from policy allow list.

Fix: request allowed role or update platform policy.

### `target Secret demo/surrealdb-app already exists and is not owned by SurrealDBCredential demo/app`

Meaning: operator refuses to overwrite a Secret it does not own.

Fix: choose another `targetSecret.name`, or delete existing Secret after confirming it is safe.

### `custom TLS config is currently supported only for http/https endpoints`

Meaning: provider uses `ws` or `wss` with `spec.tls`.

Fix: use `http` or `https` endpoint when custom TLS config is needed.

### Credential never becomes ready

Check controller logs and conditions:

```bash
kubectl describe surrealdbcredential app --namespace demo
kubectl logs \
  --namespace surrealdb-credential-operator-system \
  deploy/surrealdb-credential-operator-controller-manager
```

## Development

Install Go 1.24, Docker, `kubectl`, and Kind for end to end tests.

Common commands:

```bash
make help
make test
make lint
make build
make test-e2e
```

Dagger workflows mirror CI:

```bash
dagger call check
```

Useful local deploy commands:

```bash
make install
IMG=controller:latest make deploy
make undeploy
make uninstall
```

## Security

- Store SurrealDB admin credentials in Kubernetes Secrets, not manifests.
- Limit who can read provider Secret namespaces and generated app Secret namespaces.
- Controller RBAC includes Secret access because it validates provider refs and writes target Secrets.
- Generated passwords are written to Kubernetes Secrets and should be treated as credentials.
- Avoid posting Secrets, controller logs with credentials, or SurrealDB admin details in public issues.
- Report security issues privately to project maintainers through GitHub repository owner contact until a dedicated security policy exists.

## Uninstall

Remove Helm release:

```bash
helm uninstall surrealdb-credential-operator \
  --namespace surrealdb-credential-operator-system
```

Helm does not remove CRDs from chart `crds` directory. Delete CRDs only after backing up or deleting custom resources, because CRD deletion removes stored custom resources.

```bash
kubectl delete crd \
  surrealdbcredentials.surrealdb.seamlezz.com \
  surrealdbtenantpolicies.surrealdb.seamlezz.com \
  surrealdbproviders.surrealdb.seamlezz.com
```

## License

Apache 2.0. See [LICENSE](LICENSE).
