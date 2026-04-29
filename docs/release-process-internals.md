# Release process (internals)

This document describes **how automation is wired**, what to **verify** when you change workflows or release steps, and how to **test** end-to-end without mixing that into the [releasing playbook](releasing.md).

For **step-by-step “what do I do for a new release?”** (operator-only vs CRD), use [releasing.md](releasing.md) only.

---

## Workflows (what runs when)

| Workflow | File | Trigger (summary) | What it does |
|----------|------|---------------------|--------------|
| **Create a release** | [.github/workflows/release.yml](../.github/workflows/release.yml) | `release: types: [published]` | On a **published** GitHub Release, checks out the tag, runs `make image-release` and pushes **multi-arch operator images** to `ghcr.io` (see [Makefile](../Makefile) `image-release`, `REPOSITORY` / `IMAGE_NAME`). **Does not** create tags; you create the tag and Release. |
| **Release Charts** | [.github/workflows/helm.yml](../.github/workflows/helm.yml) | **Push to `master`** with paths under `charts/**` only (not the workflow file alone) | `helm package` for `charts/redisoperator`, `helm registry login` to GHCR, `helm push` to `oci://ghcr.io/<repository_owner>/charts`. Chart **version** is whatever is in [Chart.yaml](charts/redisoperator/Chart.yaml) at that commit. |
| **CI** (incl. version check) | [.github/workflows/ci.yaml](../.github/workflows/ci.yaml) | **Pull requests** to `master` (and push per file) | Lint, unit tests, optional integration, Helm `lint`/`template`, and **if the CRD manifest file changed**, **version-check** enforces a bump to [Chart.yaml](charts/redisoperator/Chart.yaml) `version` vs the base branch. |
| **Release Drafter** | [.github/workflows/draft_release.yml](../.github/workflows/draft_release.yml) + [.github/release-drafter.yml](../.github/release-drafter.yml) | Pushes to `main` / `master` | Updates a **draft** release body from merged PRs and labels. Does **not** change repo versions or push images. |

**Important:** A **PR branch** does not run the **Helm OCI** push; that only runs after merge to `master` and a change under `charts/**`.

**Makefile (operator tag selection):** `image-release` tags images with `$(TAG)` derived from git: latest tag if `HEAD` matches that tag, else commit SHA (and `-dirty` if the tree is not clean). The `release` workflow runs on the tagged tree, so the tag is usually a SemVer `v*`.

---

## When you change the release process (what to test)

If you modify [.github/workflows/release.yml](../.github/workflows/release.yml), [helm.yml](../.github/workflows/helm.yml), [ci.yaml](../.github/workflows/ci.yaml), or the Makefile’s image/push targets:

1. **Run local equivalents** on a branch: `make ci-lint`, `make ci-unit-test`, `make helm-test`.  
2. **Operator image path** — in a **fork** (or disposable repo with Actions enabled), create a **test tag** and a **pre-release** GitHub Release, confirm the workflow completes and:  
   `docker pull ghcr.io/<org>/redis-operator:<test-tag>`.  
3. **Helm OCI path** — merge a change under `charts/**` to the fork’s `master` (or `helm package` + `helm registry login` + `helm push` manually) and run:  
   `helm show chart oci://ghcr.io/<org>/charts/redis-operator --version <Chart version>`.  
4. **Version-check** — if you still change the CRD file in a PR, confirm the `version-check` job passes only when [Chart.yaml](charts/redisoperator/Chart.yaml) `version` is bumped correctly, and fails when it is not.

---

## Pre-merge and fork testing (PR validation)

- **On the PR to upstream:** default CI runs lint, tests, and (per path filters) integration and chart checks. **version-check** runs if the CRD file changed.  
- **What the PR does not do:** it does not run `helm push` to production GHCR until you merge.  
- **To validate OCI / Actions before merge to the main org:** use a **fork**, enable **Actions**, push/merge to the fork’s `master` with `charts/**` changes so [helm.yml](../.github/workflows/helm.yml) runs against your `repository_owner`. For the operator image, use a **test tag** and **pre-release** on the fork.  
- **Pushing the same chart `version` again:** a second `helm push` of the same chart `version` may be accepted (overwrite) or rejected by the registry, depending on GHCR/OCI policy—always **bump [Chart.yaml](charts/redisoperator/Chart.yaml) `version`** for a new chart you intend to be a distinct published version.

**Local `helm` without a registry** — install from path for smoke tests:  
`helm install testrel ./charts/redisoperator` (suitable for Kind/minikube; uninstall when done).

---

## Cross-links

- [releasing.md](releasing.md) — maintainer release playbook.  
- [README: deployment](../README.md#operator-deployment-on-kubernetes) — `kubectl`, Kustomize, and Helm OCI for users.
