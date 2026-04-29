# Releasing (playbook)

For **maintainers**: what to do when you cut a release. **SemVer and chart versions are edited by hand**—there is no automatic version bump in this repository.

**Process details** (workflows, CI behavior, and how to test a change to that process): [release-process-internals.md](release-process-internals.md)

---

## What to bump in each situation

| What changed | Bump Git tag (operator) | Bump [Chart.yaml](charts/redisoperator/Chart.yaml) `version` | Update chart [values](charts/redisoperator/values.yaml) `image.tag` (and often `appVersion`) |
|--------------|-------------------------|-------------------------------------------------------------|-----------------------------------------------------------------------------------------------|
| **Operator code / behavior only** (no CRD YAML) | **Yes** — new `vX.Y.Z` for the image | **Only if** you publish a **new** Helm chart (see below) | **Yes** if the chart’s default image should follow the new operator |
| **CRD** ([manifests/databases…](../manifests/databases.spotahome.com_redisfailovers.yaml) and [chart copy](../charts/redisoperator/crds/)) | Yes, when you release that operator | **Yes** — CI on PRs will fail without a valid chart `version` bump | Usually yes so defaults stay consistent |
| **Helm templates / values only** (operator binary unchanged) | No new tag required unless you also ship a new image | **Yes** for any new chart you `helm push` | Only if the image reference changes |

The Kubernetes **API group** stays `databases.spotahome.com` in normal releases (see [README](../README.md)).

---

## A. Operator-only release (no CRD file change)

Typical case: changes under `operator/`, `cmd/`, `service/`, etc. **The CRD file is untouched.**

1. **Merge** your work to `master` when CI is green. Local sanity: `make ci-lint`, `make ci-unit-test`, and `make helm-test` from the repo root.
2. **Tag** the commit you are releasing (e.g. `v1.4.0`) using your SemVer policy.
3. **Create a GitHub Release** from that tag and **publish** it. That triggers the **Create a release** workflow, which builds and pushes the **operator image** to GHCR. Confirm the run succeeds and the image is listed under the org’s packages.
4. **Chart and Helm on GHCR**  
   - Pushing a **new** OCI chart to GHCR only happens when there is a **push to `master` that changes something under** `charts/**` (see [release-process-internals.md](release-process-internals.md#workflows)).  
   - If users should `helm install` a chart whose **defaults** use the new image: in the same release cycle (or a follow-up PR to `master`), bump [Chart.yaml](charts/redisoperator/Chart.yaml) `version`, set [values.yaml](charts/redisoperator/values.yaml) `image.tag` (and `appVersion` / `app.kubernetes.io` labels if you use them) to match the new tag, then **merge** so the chart workflow runs.  
   - If you **do not** change `charts/**`, the operator image is still available by tag, but the published Helm chart in GHCR (if any) is unchanged—users can override `image.tag` or wait for a later chart update.

5. **Notes / changelog** — update [CHANGELOG.md](../CHANGELOG.md) and release notes to match your project policy (Release Drafter can pre-fill drafts).

---

## B. CRD change (or you ship a new operator that includes CRD updates)

**Any change** to the canonical CRD, e.g. [manifests/databases.spotahome.com_redisfailovers.yaml](../manifests/databases.spotahome.com_redisfailovers.yaml), also requires:

1. **Keep the chart copy in sync** — the chart bundles CRDs under [charts/redisoperator/crds/](../charts/redisoperator/crds/); follow the same content as the manifest the project treats as source of truth (and [kustomize base](../manifests/kustomize/base) if that is your flow).
2. **Bump the Helm chart `version`** in [Chart.yaml](charts/redisoperator/Chart.yaml) in the **same PR** (valid patch/minor/major per policy). A PR that changes the CRD file without a chart version bump will **fail** the **version-check** job in CI.
3. **Operator image** — as in section A: tag, GitHub Release, confirm image in GHCR.
4. **Clusters** — document or perform **CRD application** (often `kubectl replace` / `kubectl apply` of the CRD) per [README](../README.md#update-helm-chart) so existing clusters pick up the new schema. Communicate any required user actions in release notes.
5. **Merge to `master`**; if `charts/**` changed, the **Release Charts** workflow publishes the new chart OCI version to GHCR.

---

## Short checklists

**Before tagging an operator release**

- [ ] Tests and lint pass on the commit you are tagging.  
- [ ] CRD and chart `crds/` are in sync if you touched the CRD.  
- [ ] [Chart.yaml](charts/redisoperator/Chart.yaml) `version` is bumped on the PR if the CRD file changed.  
- [ ] Release notes / CHANGELOG ready (or use Release Drafter as a start).

**After the GitHub Release is published**

- [ ] Operator image exists in GHCR for the new tag.  
- [ ] If the chart was updated: new chart `version` appears in GHCR under GitHub Packages; `helm show chart` / `helm install … --version` work as in the README.

---

## Further reading (end users and install commands)

- [README: operator deployment and Helm OCI](../README.md#operator-deployment-on-kubernetes)  
- [Helm: Chart `version` vs `appVersion`](https://helm.sh/docs/topics/charts/#the-chartyaml-file)
