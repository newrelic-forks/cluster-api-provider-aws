# CAPA Fork — Build and Release

End-to-end runbook for cutting an `nrN` release of `cluster-api-provider-aws`
from this fork. Covers:

1. Pulling from upstream and applying fork patches
2. Building and pushing multi-arch container images
3. Tagging, creating the GitHub release, and uploading the artifacts the
   InfrastructureProvider operator consumes
4. Validating on a test cell and rolling out fleet-wide

The fork follows a **release-branch-only** model:

- `main` tracks **upstream** (no nr patches). It exists as a clean reference
  point and as the home for this runbook + `AGENTS.md`. Don't merge release
  branches into main.
- Each release lives on its own `release-vX.Y.Z-nrN` branch, branched from the
  upstream tag and tagged at the tip with `vX.Y.Z-nrN`.
- Hotfixes on an older release land on that release's branch as a new
  `-nrN+1` tag — no need to touch main or newer release branches.

This runbook lives on `main` so a new engineer who clones the fork sees the
entry point immediately. The release branches each carry their own copy of
this file as it stood when that release was cut; treat main's copy as the
source of truth.

---

## Patch Inventory

Every nr release carries the same set of fork patches on top of the upstream
release tag. Keep this list in sync with what's actually on the latest
`release-*-nrN` branch.

| # | Patch | Why | Files |
|---|-------|-----|-------|
| A | Update CRDs to serve v1beta1 | Older CAPA consumers in the fleet still read v1beta1; upstream dropped `served: true`. We re-enable it via kustomize patches. | `config/crd/kustomization.yaml`, `config/crd/patches/enable_v1beta1_served.yaml` |
| B | Skip NAT gateways with no SubnetId | Upstream panics when an existing NAT gateway has no SubnetId (deleted-subnet edge case). We skip instead. | `pkg/cloud/services/network/natgateways.go` (+ tests) |
| C | Skip instance refresh when RefreshPreferences is not configured | Upstream triggers a default instance refresh on every launch-template change, rolling our nodes unnecessarily. We only refresh when the user opts in. | `exp/controllers/awsmachinepool_controller.go` |
| D | IAM auth: dispatch GetInstanceProfile vs GetRole | (nr2+) Upstream calls `IAM.GetRole` with names that are actually instance profile names from `AWSMachinePool` / `AWSMachineTemplate`. EKS clusters using instance profiles fail aws-auth reconciliation. Fix: tag each discovered name as `asInstanceProfile` or `asRole` and dispatch to the correct IAM API. | `pkg/cloud/services/iamauth/reconcile.go`, `pkg/cloud/services/iamauth/service.go`, `pkg/cloud/services/iamauth/reconcile_test.go`, mocks |

The current `nr1` branch carries A+B+C. `nr2` adds D.

> **Out-of-tree prerequisite for Patch D**: the cell's bootstrap IAM policy
> must grant `iam:GetInstanceProfile`. Lives in
> `cell-foundation-module/modules/cf_cell_foundation_v2/iam.tf` →
> `aws_iam_policy.bootstrap_policy` (named
> `aws-bootstrap-k8-${var.cell_account_name}`). Without this, nr2 emits
> `AccessDenied` on every reconcile instead of working. Land that change
> before (or in lock-step with) rolling out nr2 to a cell.
>
> The grant propagates per **AWS account**, not per cell — cells that share an
> account (e.g. `test-angry-zebra` and `test-basic-boy` both run in
> `cell-test-weak-yak`) get the policy update from a single Atlantis apply.
> The reference PR for nr2 was [cell-foundation-module#236](https://source.datanerd.us/container-fabric/cell-foundation-module/pull/236).

---

## Prerequisites

- Go 1.24 on PATH: `export PATH=/opt/homebrew/opt/go@1.24/bin:$PATH`
- Docker / Rancher Desktop running with buildx + a multi-arch builder
  (`docker buildx ls` should list at least one builder supporting both
  `linux/amd64` and `linux/arm64`)
- `crane` (`brew install crane`)
- `gh` CLI authenticated against `github.com/newrelic-forks`
- Logged in to the container-fabric registry: `docker login cf-registry.nr-ops.net`
- `kubebuilder` envtest assets installed: `make setup-envtest`
- Working tree clean

---

## Step 1 — Pull from upstream

Fetch all upstream tags so you can branch from the right base.

```bash
cd ~/Repos/newrelic-forks/cluster-api-provider-aws
git fetch upstream --tags --prune
git fetch origin --tags --prune
```

Identify the upstream release tag you want to base on (e.g. `v2.11.1`). Confirm
it exists locally:

```bash
git tag --list 'v2.11.*'
```

---

## Step 2 — Create the release branch

There are two cases.

### Case A: New upstream version (most common)

You're cutting a release on a new upstream tag (e.g. moving from `v2.11.1` to
`v2.12.0`). Branch directly from the upstream tag and cherry-pick the fork
patches onto it.

```bash
UPSTREAM_TAG=v2.12.0
NEW_BRANCH=release-v2.12.0-nr1

git checkout -b "$NEW_BRANCH" "$UPSTREAM_TAG"
git push -u origin "$NEW_BRANCH"
```

Then cherry-pick patches A/B/C/D in order — see "Cherry-picking fork patches"
in Step 3. Find the source SHAs by reading `git log` on the most recent
`release-*-nrN` branch (don't hard-code SHAs in this doc — they go stale).

### Case B: Additional patch on an existing release line

You're adding a new patch on top of an already-shipped fork release (e.g.
adding Patch D to make `nr2` from `nr1`). Branch from the previous nr tag so
the existing patches are already in place.

```bash
PREV_TAG=v2.11.1-nr1
NEW_BRANCH=release-v2.11.1-nr2

git checkout -b "$NEW_BRANCH" "$PREV_TAG"
git push -u origin "$NEW_BRANCH"
```

Apply only the new delta in Step 3.

---

## Step 3 — Apply the new patch(es)

For each new fix:

1. Edit code, regenerate as needed (`make generate`).
2. Add unit tests.
3. `make lint && make test` must be clean.
4. Commit with DCO sign-off, one logical change per commit:

   ```bash
   git commit -s -m "iamauth: dispatch GetInstanceProfile for instance profile names"
   ```

Do **not** open a PR — fork releases land directly on the release branch by
convention (see `AGENTS.md`).

### Cherry-picking fork patches onto a fresh upstream base

If you started from a clean upstream tag (Case A in Step 2), cherry-pick each
fork patch in order. **Don't trust SHAs written here** — they go stale every
release. Find the current SHAs by reading the most recent `release-*-nrN`
branch's history:

```bash
# Latest fork release branch is the source of truth for patch SHAs
LATEST_NR=release-v2.11.1-nr2
git log --oneline "$UPSTREAM_TAG..origin/$LATEST_NR" -- \
  config/crd \
  pkg/cloud/services/network/natgateways.go \
  exp/controllers/awsmachinepool_controller.go \
  pkg/cloud/services/iamauth/
```

Identify the commit for each patch (A: CRD v1beta1 served, B: NAT gateway
SubnetId, C: skip instance refresh, D: IAM auth dispatch) and cherry-pick in
order:

```bash
git cherry-pick <patch-A-sha>
git cherry-pick <patch-B-sha>
git cherry-pick <patch-C-sha>
git cherry-pick <patch-D-sha>
```

Resolve conflicts, re-run `make generate && make lint && make test`, then
proceed.

---

## Step 4 — Build multi-arch images

We build natively on the host arch via `docker-build-all`. The Makefile's
default `ALL_ARCH` is five architectures (`amd64 arm arm64 ppc64le s390x`);
we override to just the two we run.

Trying `docker buildx build --platform linux/amd64` on Apple Silicon crashes
the Go toolchain under QEMU (see "Multi-arch on Apple Silicon" below), so we
build natively per arch instead.

```bash
export REGISTRY=cf-registry.nr-ops.net/container-fabric
export TAG=v2.11.1-nr2
export ALL_ARCH="amd64 arm64"

docker buildx use rancher-desktop   # or whichever local builder has both archs

make docker-build-all REGISTRY=$REGISTRY TAG=$TAG ALL_ARCH="$ALL_ARCH"
```

The Makefile names produced images as `$(REGISTRY)/$(CORE_IMAGE_NAME)-$(ARCH):$(TAG)`,
so you should see two locally:

```bash
docker images | grep "cluster-api-aws-controller" | grep "$TAG"
# expect:
#   $REGISTRY/cluster-api-aws-controller-amd64   $TAG ...
#   $REGISTRY/cluster-api-aws-controller-arm64   $TAG ...
```

> **Note on version stamps**: the build's `LDFLAGS` include
> `gitVersion=$(git describe --abbrev=0)`. If you build *before* tagging
> `v2.11.1-nr2` locally, the binary self-reports as `v2.11.1-nr1-dirty`. The
> code is still correct — only the version string is off. For final
> production images, build *after* you tag locally so the stamp matches.

---

## Step 5 — Push images and create the fat manifest

```bash
make docker-push-all REGISTRY=$REGISTRY TAG=$TAG ALL_ARCH="$ALL_ARCH"
```

`docker-push-all` pushes each arch image and then chains into
`docker-push-core-manifest`, which auto-runs:

1. `docker manifest create --amend $(REGISTRY)/cluster-api-aws-controller:$(TAG) <each arch>`
2. `docker manifest annotate --arch <arch>` per child
3. `docker manifest push --purge`

So a single `make docker-push-all` should leave you with three tags in the
registry: `-amd64`, `-arm64`, and the fat manifest with no arch suffix.

> **Watch for transient network errors.** `docker manifest create` fetches
> each child manifest from the registry; a `connection reset by peer` mid-fetch
> silently produces a partial manifest list, and the subsequent `manifest push`
> still succeeds (with only the children that survived). If the build/push log
> shows fewer "Pushed ref" lines than `ALL_ARCH` entries, retry the manifest
> step manually:
>
> ```bash
> docker manifest rm $REGISTRY/cluster-api-aws-controller:$TAG 2>/dev/null || true
> docker manifest create $REGISTRY/cluster-api-aws-controller:$TAG \
>   --amend $REGISTRY/cluster-api-aws-controller-amd64:$TAG \
>   --amend $REGISTRY/cluster-api-aws-controller-arm64:$TAG
> docker manifest annotate $REGISTRY/cluster-api-aws-controller:$TAG \
>   $REGISTRY/cluster-api-aws-controller-amd64:$TAG --arch amd64 --os linux
> docker manifest annotate $REGISTRY/cluster-api-aws-controller:$TAG \
>   $REGISTRY/cluster-api-aws-controller-arm64:$TAG --arch arm64 --os linux
> docker manifest push --purge $REGISTRY/cluster-api-aws-controller:$TAG
> ```

---

## Step 6 — MANDATORY: Fix manifest platform metadata (Apple Silicon only)

When `docker-build-all` runs on an arm64 host, both arch images get stamped
with `architecture: arm64` in their **config blob**, even though the amd64
image contains an amd64 binary. The Makefile's `docker manifest annotate`
patches the **manifest list** entry's platform field but does **not** touch
the underlying config blob, so:

- `docker manifest inspect <fat>` looks correct ✓
- `crane config <amd64-image>` still says `arm64` ✗ — fails image scanners,
  Trivy, Kyverno-style admission policies that read config blobs

`crane mutate` rewrites only the config blob (no rebuild, no layer push):

```bash
crane mutate $REGISTRY/cluster-api-aws-controller-amd64:$TAG \
  --set-platform linux/amd64 \
  --tag $REGISTRY/cluster-api-aws-controller-amd64:$TAG

crane mutate $REGISTRY/cluster-api-aws-controller-arm64:$TAG \
  --set-platform linux/arm64 \
  --tag $REGISTRY/cluster-api-aws-controller-arm64:$TAG
```

The mutated images have **new digests**. The fat manifest still points at the
old (wrong) child digests, so re-create and re-push it:

```bash
docker manifest rm $REGISTRY/cluster-api-aws-controller:$TAG 2>/dev/null || true
docker manifest create $REGISTRY/cluster-api-aws-controller:$TAG \
  --amend $REGISTRY/cluster-api-aws-controller-amd64:$TAG \
  --amend $REGISTRY/cluster-api-aws-controller-arm64:$TAG
docker manifest annotate $REGISTRY/cluster-api-aws-controller:$TAG \
  $REGISTRY/cluster-api-aws-controller-amd64:$TAG --arch amd64 --os linux
docker manifest annotate $REGISTRY/cluster-api-aws-controller:$TAG \
  $REGISTRY/cluster-api-aws-controller-arm64:$TAG --arch arm64 --os linux
docker manifest push --purge $REGISTRY/cluster-api-aws-controller:$TAG
```

Verify both layers are correct:

```bash
crane config $REGISTRY/cluster-api-aws-controller-amd64:$TAG | jq '{architecture, os, variant}'
# expect: {"architecture":"amd64","os":"linux","variant":null}
crane config $REGISTRY/cluster-api-aws-controller-arm64:$TAG | jq '{architecture, os, variant}'
# expect: {"architecture":"arm64","os":"linux","variant":null}

docker manifest inspect $REGISTRY/cluster-api-aws-controller:$TAG | jq '.manifests[].platform'
# expect: linux/amd64 and linux/arm64, no spurious variant fields
```

Skip this step only if the build host is real `linux/amd64` (CI, Linux box).

---

## Step 7 — Verify the published image

The simplest end-to-end check: pull the **fat manifest** as if you were a node,
extract the binary, and look for the symbols you expect.

```bash
docker pull --platform linux/amd64 $REGISTRY/cluster-api-aws-controller:$TAG
CID=$(docker create --platform linux/amd64 $REGISTRY/cluster-api-aws-controller:$TAG)
docker cp $CID:/manager /tmp/manager-amd64
docker rm $CID >/dev/null

# Confirm ELF arch
file /tmp/manager-amd64
# expect: ELF 64-bit LSB executable, x86-64, ...

# (nr2+) Confirm new IAM auth code is compiled in
strings /tmp/manager-amd64 | grep -E "iamauth.*\.(getARNForInstanceProfile|resolveRoleARN|getARNForRole)" | sort -u
# expect three distinct symbols

grep -aoE "instance profile %s (has no role attached|has no ARN)" /tmp/manager-amd64 | sort -u
# expect both error format strings (proves the new function is reachable)

rm -f /tmp/manager-amd64
```

---

## Step 8 — Tag and create the GitHub release (with assets)

The InfrastructureProvider operator on each cell fetches release artifacts
from `https://github.com/newrelic-forks/cluster-api-provider-aws/releases/...`
when it reconciles. Without `metadata.yaml` and `infrastructure-components.yaml`
attached, every cell sees `release not found for version v2.11.1-nrN, ... 404
Not Found`.

```bash
git tag -a $TAG -m "Release $TAG"   # drop -s if you don't have a GPG key set up
git push origin $TAG
```

Render the manifests for the new tag:

```bash
make release-manifests \
  CORE_CONTROLLER_IMG=$REGISTRY/cluster-api-aws-controller \
  RELEASE_TAG=$TAG \
  PULL_POLICY=IfNotPresent

# Sanity check
grep -E "image:.*cluster-api-aws-controller" out/infrastructure-components.yaml | head -3
# expect: image: $REGISTRY/cluster-api-aws-controller:$TAG
```

Create the GitHub release and upload the two artifacts:

```bash
gh release create $TAG \
  --repo newrelic-forks/cluster-api-provider-aws \
  --target release-$TAG \
  --title $TAG \
  --notes "Release notes here..."

gh release upload $TAG \
  --repo newrelic-forks/cluster-api-provider-aws \
  out/metadata.yaml \
  out/infrastructure-components.yaml
```

> **CRITICAL — verify the release is published, not draft.** `gh release create`
> can land in draft state depending on flags / org settings, and a draft
> release is invisible to the operator's tag-based fetch URL. The cell will
> then emit `release not found for version <TAG>, ... 404 Not Found` even though
> the assets are attached.
>
> ```bash
> gh release view $TAG --repo newrelic-forks/cluster-api-provider-aws \
>   --json isDraft,url,assets | jq '{isDraft, url, assetCount: (.assets | length)}'
> ```
>
> If `isDraft: true`, publish it:
>
> ```bash
> gh release edit $TAG \
>   --repo newrelic-forks/cluster-api-provider-aws \
>   --draft=false
> ```
>
> Confirm the release URL is now `releases/tag/$TAG` (not `releases/tag/untagged-<hash>`).

> **`metadata.yaml` `contract:` field**: at the time of v2.11.1, upstream still
> declares `contract: v1beta1` for the `minor: 11` series. Don't "fix" this in
> the fork — match upstream. The CRD-level `served: true` for v1beta1 (Patch A)
> is a separate concern.

> **Other release assets** (clusterawsadm binaries, cluster-template-*.yaml,
> AWSIAMManagedPolicy*.json) — nr1 ships them as a courtesy, but the
> InfrastructureProvider operator doesn't consume them. Skip unless someone
> in CF actually depends on them.

---

## Step 9 — Validate on a test cell

The deployment is **operator-driven**, not via `kubectl set image` — manual
patches will be reverted by the CAPI operator within seconds.

The flow on each cell:

```
cluster-operator-configs (Helm values pin the InfrastructureProvider)
       │
       ▼
  InfrastructureProvider/aws (.spec.version = "$TAG", fetchConfig.url points at GitHub release)
       │
       ▼
  CAPI operator (cluster-api-operator pod in capi-operator-system)
       │ polls every ~30–60s, downloads metadata.yaml + infrastructure-components.yaml
       ▼
  Applies infrastructure-components.yaml → updates capa-controller-manager Deployment
       │
       ▼
  CAPA controller pod rolls to the new image tag
```

To validate on `test-angry-zebra` (or any test cell):

1. **Point the cell's InfrastructureProvider at the new tag.** Update
   `spec.version` (and `spec.deployment.containers[].imageUrl` if the cell pins
   the image directly) to `$TAG`. This typically lives in the
   `cluster-operator-configs` repo's per-cell values file.
2. **Wait for the CAPI operator to reconcile.** It will fetch the release
   assets and apply them. No manual `kubectl apply` needed.
3. **Confirm the controller rolled:**

   ```bash
   kubectl -n capa-system get deploy capa-controller-manager \
     -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].image}{"\n"}'
   # expect: $REGISTRY/cluster-api-aws-controller:$TAG

   kubectl -n capa-system get pods -l control-plane=capa-controller-manager -o wide
   # expect: Running 1/1, RESTARTS 0, fresh AGE

   kubectl get infrastructureprovider -A -o json \
     | jq -r '.items[] | "\(.metadata.namespace)/\(.metadata.name)\tspec=\(.spec.version)\tinstalled=\(.status.installedVersion)"'
   # expect: spec == installed == $TAG
   ```

4. **For Patch D (nr2+), watch the AMCP IAM condition:**

   ```bash
   kubectl get awsmanagedcontrolplane -A -o json | \
     jq -r '.items[] | .status.conditions[]? | select(.type=="IAMAuthenticatorConfigured") | "\(.status)\t\(.reason // "-")\t\((.message // "")[:200])"'
   ```

Three possible post-swap states:

| State | Meaning | Action |
|---|---|---|
| `IAMAuthenticatorConfigured: True` | Patch D + IAM permission both in place | ✅ Done, proceed |
| `False` with `unable to get instance profile: ... AccessDenied` | Patch D landed (calling the right API) but the bootstrap policy lacks `iam:GetInstanceProfile` | Land the cell-foundation-module IAM grant for this cell (see Patch D prerequisite above). Once Atlantis applies, AMCP flips to True within ~3 minutes — no controller restart needed |
| `False` with `unable to get role: ... NoSuchEntity` | The image swap didn't take — operator is still running an older version | Re-check the operator's `installedVersion`. Common cause: GitHub release is in draft state; see Step 8 |

**Reference timeline** from the nr2 validation:

| Time (UTC) | Event |
|---|---|
| 09:30 | nr2 image rolled by operator, pod up |
| 09:32 | Code patch firing; error pattern switched from `GetRole NoSuchEntity` to `GetInstanceProfile AccessDenied` (proves dispatch works) |
| 09:38:33 | Atlantis applied IAM policy update (cell-foundation-module#236 → policy v36→v37) |
| 09:41:14 | First successful `Reconciled aws-iam-authenticator configuration` log line; AMCP `Ready: True` |

Proceed past this point only once the image is healthy on a real cell **and**
`IAMAuthenticatorConfigured` flips to `True`.

### Sanity checks on `aws-auth`

Patch D's reconcile is **append-only with dedup** in `iamauth/configmap.go`. Post-swap, the `aws-auth` ConfigMap should be **unchanged** if the resolved role ARN was already present (which it usually is — bootstrap added it on the cell's first install). Useful to verify nothing broke:

```bash
kubectl -n kube-system get configmap aws-auth -o jsonpath='{.data.mapRoles}' | grep -B 1 "system:nodes"
# expect: same entries as before the swap (no removals, possibly one new append)
```

---

## Step 10 — Roll out fleet-wide

Order matters. The IAM permission must reach a cell **before** that cell upgrades
to nr2; otherwise nr2 emits AccessDenied during the propagation lag. The IAM
grant is harmless on cells still running pre-nr2 images (they don't call
`GetInstanceProfile`), so it can land freely ahead of the controller bump.

1. **Land cell-foundation-module IAM grant fleet-wide.** PR adding
   `iam:GetInstanceProfile` to `aws_iam_policy.bootstrap_policy` merges →
   Atlantis applies per workspace → every cell's `aws-bootstrap-k8-<account>`
   policy version bumps. Verify a few sample cells with:

   ```bash
   aws --profile <profile> iam get-policy \
     --policy-arn arn:aws:iam::<account>:policy/aws-bootstrap-k8-<cell-account-name> \
     | jq -r '.Policy.DefaultVersionId'
   # then list-actions on that version, grep for iam:GetInstanceProfile
   ```

2. **Bump CAPA version in cluster-operator-configs** (or wherever the
   InfrastructureProvider's `spec.version` lives). This is what each cell's
   CAPI operator picks up.

3. **Argo / pipeline fan-out** rolls per-cell at whatever cadence the
   `cluster-operator-configs` deploy pipeline uses.

4. **Watch the rollout** per cell. The 3-state matrix in Step 9 applies to
   each cell. Most cells should land in `IAMAuthenticatorConfigured: True`
   within minutes of the operator picking up the new tag, since the IAM grant
   went out first.

5. **Decommission per-cell workarounds.** Cells that previously had ad-hoc
   instance profiles created with role-name suffixes (test-tan-spot pattern)
   no longer need them. Remove via the same Terraform path that created them.

---

## Multi-arch on Apple Silicon — postmortem

We hit two failure modes building amd64 images on M-series Macs:

1. `docker buildx build --platform linux/amd64` runs the Go toolchain under
   QEMU. The Go runtime crashes mid-build with `runtime: lfstack.push invalid
   packing` and `SIGSEGV` in `runtime.netpoll`. This is a known QEMU+Go issue
   and is not fixable from our side.

2. `make docker-build-all` (which `go build`s natively per arch and uses
   buildx to stage layers) succeeds, but Docker stamps the OCI config's
   `architecture` field from the **host** arch. The amd64 image ends up with
   `architecture: arm64` in its config. Kubernetes' image admission then
   rejects scheduling on amd64 nodes.

`crane mutate --set-platform` rewrites only the config blob (no rebuild, no
layer changes), which is fast and deterministic. It's the documented working
fix until builds move off laptops.

If you build on a real `linux/amd64` host, neither problem applies — Step 6
becomes a no-op.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `make generate` fails with `go.mod requires go >= 1.24.0` | System Go is older | `export PATH=/opt/homebrew/opt/go@1.24/bin:$PATH` |
| `make test` panics with `fork/exec /usr/local/kubebuilder/bin/etcd` | envtest assets missing | `make setup-envtest && export KUBEBUILDER_ASSETS=$(./hack/tools/bin/setup-envtest use --use-env -p path 1.32.0)` |
| `go generate` complains about missing `mockgen` | tools not built | `make hack/tools/bin/mockgen` |
| Pod on amd64 node: `exec format error` | manifest platform mismatch (Step 6 skipped) | Run `crane mutate` and re-push the fat manifest |
| Operator log: `release not found for version v2.11.1-nrN, ... 404 Not Found` | GitHub release exists but missing `metadata.yaml` / `infrastructure-components.yaml` assets, OR release is in draft state | First check `gh release view $TAG --json isDraft,assets`. If `isDraft: true`, run `gh release edit $TAG --draft=false`. If assets are missing, run Step 8's `make release-manifests` + `gh release upload` |
| Operator log: `failed to download files from GitHub release v2.11.1-nrN: failed to get file "metadata.yaml" from "v2.11.1-nrN" release` | Release is draft (URL resolves to `releases/tag/untagged-<hash>` instead of `releases/tag/<TAG>`) | `gh release edit $TAG --draft=false` |
| `IAMAuthenticatorConfigured=False` with `... GetRole, ... NoSuchEntity` (nr1 / pre-nr2) | Pre-nr2 image, type-confusion bug active | Roll forward to nr2 |
| `IAMAuthenticatorConfigured=False` with `... GetInstanceProfile, ... AccessDenied` (nr2+) | Patch D landed, but cell's bootstrap IAM policy lacks `iam:GetInstanceProfile` | Land `iam:GetInstanceProfile` on `aws_iam_policy.bootstrap_policy` in cell-foundation-module |
| Fat manifest only has one arch after `docker manifest push` | Transient `connection reset by peer` mid-`amend`; Docker silently produced a partial list | Re-run the manual `docker manifest rm/create/annotate/push` block from Step 5 |
| `crane config <amd64-image>` shows `architecture: arm64` | Step 6 not run (Apple Silicon host arch leaked into config blob) | Run `crane mutate --set-platform` on each arch image and re-push the fat manifest |
