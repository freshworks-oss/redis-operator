# Contributing

Thank you for your interest in improving the Redis Operator.

## What this project is

This Kubernetes operator creates, configures, and manages **Redis failovers**
(Redis plus Sentinel) using the `RedisFailover` custom resource. Contributions
that fit the project include bug fixes, tests, documentation, Helm chart
changes, CRD and API updates, and CI improvements.

## Where to ask questions

- **Bugs and feature requests:** [GitHub Issues](https://github.com/freshworks/redis-operator/issues).
- **General questions:** If GitHub Discussions is enabled for this repository,
  prefer Discussions for design and “how do I…?” topics; otherwise open an Issue
  and label or describe it as a question.

## Development setup

- **Go:** Use the version declared in [`go.mod`](go.mod) (including the
  `toolchain` directive if present). CI uses `go-version-file: go.mod`.
- **Container runtime:** Optional for local workflows that use the development
  image; CI runs Go targets directly on the runner without it. **Podman** works:
  if `podman` is on your `PATH`, `make` uses it automatically (otherwise Docker).
  If both are installed and you want Docker, run e.g.
  `make CONTAINER_ENGINE=docker docker-build`.
- **Additional tools:** Integration tests on CI use Minikube; Helm tests require
  Helm locally when you run `make helm-test` outside CI.

## Build and test (match CI)

The [CI workflow](.github/workflows/ci.yaml) runs lint, unit tests, integration
tests (Minikube matrix), Helm tests, and (on pull requests) a CRD/chart version
check when the CRD file changes. Pull requests also run
[`dco.yaml`](.github/workflows/dco.yaml) for commit sign-off (see below). Before
opening a PR, run the same checks you can locally:

```bash
make ci-lint
make ci-unit-test
```

Integration tests (longer; require a working Kubernetes test environment when
not relying on CI):

```bash
make ci-integration-test   # same Go invocation as CI after cluster is up
# or the scripted environment:
make integration-test
```

Helm chart validation:

```bash
make helm-test
```

To run the full local suite similar to CI (lint + unit + integration + helm):

```bash
make test
```

## Pull requests

- Prefer **small, focused PRs** that are easy to review.
- Link an **issue** when one exists so reviewers have context.
- **Sign-off (DCO):** Use `git commit -s` so every commit in the pull request
  includes a `Signed-off-by:` line (Developer Certificate of Origin, version
  1.1). CI runs [`.github/workflows/dco.yaml`](.github/workflows/dco.yaml) on
  pull requests and fails if any non-merge commit is missing it. Pull requests
  opened by Dependabot or Renovate skip that job (bot commits); use squash with
  a sign-off if your policy still requires it.
- Expect reviewers to request **tests** when behavior changes or regressions are
  plausible.

## Code review

Merges are performed by [maintainers](MAINTAINERS.md). Review turnaround is best
effort (about a week is a reasonable expectation when maintainers are
available).

## API, CRD, and Helm changes

- Changes to the `RedisFailover` API must stay consistent with the CRD under
  `manifests/databases.spotahome.com_redisfailovers.yaml` and any kustomize
  copies (for example `manifests/kustomize/base`).
- When you change the CRD YAML, CI expects the Helm chart version in
  `charts/redisoperator/Chart.yaml` to be bumped appropriately—see the
  `version-check` job in `.github/workflows/ci.yaml`.
- Regeneration helpers in the [`Makefile`](Makefile) include `update-codegen`
  and `generate-crd` (Docker-based); use them when you change API types so
  clients and manifests stay in sync.

## Security

See [SECURITY.md](SECURITY.md) for supported versions and how to report
vulnerabilities privately.
