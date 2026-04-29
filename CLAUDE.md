# CLAUDE.md

Kubernetes **Redis Operator** ([`github.com/freshworks/redis-operator`](https://github.com/freshworks/redis-operator)): manages **Redis + Sentinel** via the **`RedisFailover`** CRD (`databases.spotahome.com/v1`). Controller code is **Go**; reconciliation uses **Kooper** with leader election and label-scoped operator groups.

## Hard constraints (non-negotiable)

- **`OPERATOR_GROUP_ID`** must be non-empty at runtime; reconciliation is scoped by **`redis-failover.freshworks.com/operator-group`** on each `RedisFailover`.
- Do not remove or bypass **`redis-failover.freshworks.com/skip-reconcile`** handling without an explicit product decision and tests.
- **Generated code:** never hand-edit `client/k8s/**` or `api/redisfailover/v1/zz_generated.deepcopy.go`—use `Makefile` `update-codegen` / `generate-crd`.
- **CRD + chart:** if `manifests/databases.spotahome.com_redisfailovers.yaml` changes, bump **`charts/redisoperator/Chart.yaml`** (CI enforces).

## Rules

### Always

- Prefer facts from this repo (`go.mod`, `Makefile`, `operator/`, `service/`) over assumptions about upstream Spotahome behavior unless you are explicitly porting a change.
- Keep changes small and reviewable; match existing patterns in `service/k8s` and `operator/redisfailover/service`.
- Run **`make ci-lint`** and **`make ci-unit-test`** before considering work done for Go changes.

### When changing behavior

- Add or extend **unit tests** in the same package (`*_test.go`); use **`test/integration/`** with build tag **`integration`** for multi-resource flows.
- Update **examples** or **docs** when user-visible behavior or defaults change.

### Before opening or updating a PR

- Sign commits with **`git commit -s`** (DCO) unless the PR is from an exempt bot workflow.
- Confirm Helm/CRD impact if you touched `manifests/`, `charts/`, or `api/`.

## Project structure

```text
cmd/redisoperator/     # process entry
cmd/utils/             # CLI flags, Kubernetes REST config
api/redisfailover/v1/  # types, validation, defaults, register
operator/redisfailover/
  factory.go           # Kooper controller + retriever + leader election
  handler.go           # Handle() → check / heal / ensure
  ensurer.go           # Ensure* resource orchestration
  checker.go           # pre-heal checks
operator/redisfailover/service/
  client.go            # Ensure* Kubernetes objects from spec
  generator.go         # desired manifests / specs
  check.go, heal.go    # Redis/Sentinel health and repair
service/k8s/           # typed K8s helpers (STS, Deploy, SVC, CM, …)
service/redis/         # Redis client calls used by checks
client/k8s/            # generated clientset / informers / listers
manifests/             # CRD + kustomize
charts/redisoperator/  # Helm
test/integration/      # integration tests
```

## Context retrieval

<context-sources>
  <area name="reconcile-entry">
    <triggers>reconcile, handler, controller, kooper, leader election</triggers>
    <start-with>operator/redisfailover/factory.go</start-with>
    <then>operator/redisfailover/handler.go</then>
    <then>operator/redisfailover/ensurer.go</then>
  </area>
  <area name="redisfailover-spec">
    <triggers>CRD, API, validation, defaults, RedisFailover spec</triggers>
    <start-with>api/redisfailover/v1/types.go</start-with>
    <then>api/redisfailover/v1/validate.go</then>
    <then>api/redisfailover/v1/defaults.go</then>
  </area>
  <area name="kubernetes-objects">
    <triggers>StatefulSet, Deployment, Service, ConfigMap, PDB, ensure</triggers>
    <start-with>operator/redisfailover/service/client.go</start-with>
    <then>operator/redisfailover/service/generator.go</then>
    <then>service/k8s/</then>
  </area>
  <area name="health-heal">
    <triggers>check, heal, sentinel, master, slave, redis ping</triggers>
    <start-with>operator/redisfailover/service/check.go</start-with>
    <then>operator/redisfailover/service/heal.go</then>
    <then>service/redis/client.go</then>
  </area>
  <area name="main-and-flags">
    <triggers>startup, metrics, pprof, OPERATOR_GROUP_ID, flags</triggers>
    <start-with>cmd/redisoperator/main.go</start-with>
    <then>cmd/utils/flags.go</then>
    <then>cmd/utils/k8s.go</then>
  </area>
  <area name="integration-tests">
    <triggers>integration, minikube, e2e</triggers>
    <start-with>test/integration/redisfailover/</start-with>
  </area>
</context-sources>

## Related docs

- [AGENTS.md](AGENTS.md) — unified agent context (build, CI, checklist)
- [CONTRIBUTING.md](CONTRIBUTING.md) — contribution process
- [SECURITY.md](SECURITY.md) — vulnerability reporting
