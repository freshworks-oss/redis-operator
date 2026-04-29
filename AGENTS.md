# AGENTS: context for agentic tools

This file gives AI agents and automation enough context to work productively in this repository: what the operator does, where code lives, how to build and test, and guardrails that match [CONTRIBUTING.md](CONTRIBUTING.md).

## Repository overview

**Module:** `github.com/freshworks/redis-operator`  
**Role:** Kubernetes operator that reconciles the **`RedisFailover`** custom resource: Redis (StatefulSet), Sentinel (Deployment), Services, ConfigMaps, PodDisruptionBudgets, and related objects.

**Lineage:** Fork of Spotahome’s redis-operator. The CRD API group is still **`databases.spotahome.com`**; the kind is **`RedisFailover`**. The Go import path does **not** match the API group name—do not “fix” one to match the other without an explicit API migration plan.

**Primary language:** Go (`go.mod` / `toolchain` drive CI via `go-version-file`).

## Architecture quick facts

| Topic | Detail |
|--------|--------|
| Controller framework | [Kooper](https://github.com/spotahome/kooper) `controller.Controller` with leader election |
| Entrypoint | `cmd/redisoperator/main.go` |
| Reconcile pipeline | `handler.go` → validate → check → heal → `ensurer.go` / `operator/redisfailover/service/` |
| K8s API layer | `service/k8s/` |
| CRD types | `api/redisfailover/v1/` |
| Generated clients / deepcopy | `client/k8s/`, `api/redisfailover/v1/zz_generated.deepcopy.go` — **do not hand-edit** |

## Non-negotiable runtime behavior

- **`OPERATOR_GROUP_ID`** must be set and non-empty; the process exits otherwise.
- Only `RedisFailover` objects with label **`redis-failover.freshworks.com/operator-group=<OPERATOR_GROUP_ID>`** are reconciled.
- Annotation **`redis-failover.freshworks.com/skip-reconcile: "true"`** skips reconciliation for that CR.

## Build and test (agents)

Commands that mirror CI (see [`.github/workflows/ci.yaml`](.github/workflows/ci.yaml)):

```bash
make ci-lint
make ci-unit-test
make ci-integration-test   # needs a cluster; CI uses Minikube
make helm-test
make test                  # full local suite (heavy)
```

**Code generation:** `Makefile` targets `update-codegen` and `generate-crd` (Dockerized). Run after API type changes.

**Container workflows (Podman or Docker):** `make docker-build`, `make shell`, `make build`, `make image` (see `CONTAINER_ENGINE` in the `Makefile`).

## Contribution requirements (summary)

- Follow [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
- **DCO:** commits on pull requests need `Signed-off-by:` (`git commit -s`); CI enforces via [`.github/workflows/dco.yaml`](.github/workflows/dco.yaml) (Dependabot/Renovate actors exempt).
- **API / CRD / Helm:** changing `manifests/databases.spotahome.com_redisfailovers.yaml` requires bumping `charts/redisoperator/Chart.yaml` (see CI `version-check` job).

## Guardrails

- Do not commit secrets, kubeconfig contents, or registry credentials.
- Do not commit generated trees under `client/k8s/` or `zz_generated.deepcopy.go` without running the repo’s generators.
- Prefer extending `service/k8s/` and `operator/redisfailover/service/` instead of duplicating raw client patterns.
- Keep label and annotation keys consistent with existing constants (`operator/redisfailover/service/constants.go`, `handler.go`).

## Project structure (essential)

```text
cmd/redisoperator/       # main
cmd/utils/               # flags, k8s client bootstrap
api/redisfailover/v1/    # CRD Go types + validation
operator/redisfailover/  # controller, handler, ensurer, checker
operator/redisfailover/service/
service/k8s/             # Kubernetes resource helpers
service/redis/           # Redis/Sentinel client usage
client/k8s/              # generated clientset / informers / listers
manifests/               # CRD YAML, kustomize bases
charts/redisoperator/    # Helm chart
example/                 # sample manifests
test/integration/        # integration tests (build tag: integration)
.github/workflows/      # CI, DCO, staleissues, release
```

## GitHub Actions (beyond `ci.yaml`)

| Workflow | Purpose |
|----------|---------|
| [`ci.yaml`](.github/workflows/ci.yaml) | Lint, unit tests, integration matrix, Helm test, CRD/chart version check |
| [`dco.yaml`](.github/workflows/dco.yaml) | DCO `Signed-off-by` on PR commits |
| [`staleissues.yaml`](.github/workflows/staleissues.yaml) | Mark/close inactive issues (scheduled) |
| [`release.yml`](.github/workflows/release.yml), [`draft_release.yml`](.github/workflows/draft_release.yml), [`helm.yml`](.github/workflows/helm.yml) | Releases and charts |

**Repo settings:** enable [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) to match `SECURITY.md`. Optionally enable **Dependency graph** and add [dependency review](https://docs.github.com/en/code-security/supply-chain-security/understanding-your-software-supply-chain/about-dependency-review) as a workflow if the org supports it.

## Quality gates (agent checklist)

- [ ] `make ci-lint` and `make ci-unit-test` pass for your change.
- [ ] Integration or Helm impact considered; run `make ci-integration-test` / `make helm-test` when you touch reconcile paths, manifests, or charts.
- [ ] API or CRD changes include manifests + chart version bump when required.
- [ ] No generated client/deepcopy edits without regeneration.
- [ ] No secrets in commits.
- [ ] DCO sign-off on commits (`git commit -s`) for human-authored PRs.

## If you need more

- [README.md](README.md) — install and usage
- [CONTRIBUTING.md](CONTRIBUTING.md) — full PR and development policy
- [CLAUDE.md](CLAUDE.md) — additional agent-oriented constraints and context map
- [docs/](docs/) — deeper documentation where present
