# SLSA Build Provenance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add SLSA Build Level 3 provenance and SPDX SBOM attestations to the three GHCR container images built by `.github/workflows/docker.yml`, signed keylessly via Sigstore, and document how downstream consumers verify them.

**Architecture:** Each `build-<image>` job in the existing workflow gets four additions: an `id: build` on the push step (to read `outputs.digest`), disabling build-push-action's inline provenance/SBOM, then three new steps (Syft SBOM generation → `attest-sbom@v3` → `attest-build-provenance@v3`) guarded on non-PR events. Top-level permissions gain `id-token: write` and `attestations: write`. A new `docs/security/slsa-verification.md` documents `gh attestation verify` + `cosign verify-attestation` commands; README gets a SLSA Level 3 badge.

**Tech Stack:** GitHub Actions, `actions/attest-build-provenance@v3`, `actions/attest-sbom@v3`, `anchore/sbom-action@v0`, `docker/build-push-action@v7`, Sigstore (Fulcio + Rekor), GHCR.

**Worktree:** `/Users/jobinlawrance/Project/raven/.worktrees/slsa-provenance`
**Branch:** `ci/slsa-provenance` (tracking `origin/main`)
**Spec:** `docs/superpowers/specs/2026-04-19-slsa-provenance-design.md`
**Issues:** parent #333; children #331 (workflow), #332 (docs)

---

## File Structure

**Modify:**
- `.github/workflows/docker.yml` — add top-level permissions, `workflow_dispatch` trigger, and append attestation steps to all three jobs.
- `README.md` — one-line SLSA 3 badge.

**Create:**
- `docs/security/slsa-verification.md` — consumer-facing verify instructions.

No source code under test. "Testing" here means: lint the workflow, trigger a dry-run, and verify attestations exist on real pushed images.

---

## Task 1: Add Permissions and `workflow_dispatch` Trigger

**Files:**
- Modify: `.github/workflows/docker.yml`

**Why:** `attest-build-provenance@v3` and `attest-sbom@v3` require `id-token: write` (Sigstore OIDC) and `attestations: write` (GitHub attestation store). The `workflow_dispatch` trigger lets us manually dry-run the workflow from the `ci/slsa-provenance` branch without needing a tag or path-triggering file change.

- [ ] **Step 1: Edit the `on:` block**

Current:
```yaml
on:
  push:
    branches: [main]
    tags: ['v*.*.*']
    paths:
      - 'Dockerfile'
      - 'ai-worker/Dockerfile'
      - 'frontend/Dockerfile'
      - 'frontend/nginx.conf'
      - 'docker-compose.yml'
      - '.github/workflows/docker.yml'
  pull_request:
    ...
```

Add `workflow_dispatch:` at the top of the `on:` block:

```yaml
on:
  workflow_dispatch:
  push:
    branches: [main]
    ...
```

- [ ] **Step 2: Edit the `permissions:` block**

Current:
```yaml
permissions:
  contents: read
  packages: write
```

Replace with:
```yaml
permissions:
  contents: read
  packages: write
  id-token: write       # Sigstore OIDC for keyless signing
  attestations: write   # GitHub attestation store
```

- [ ] **Step 3: Validate the YAML**

Run from the worktree root:
```
actionlint .github/workflows/docker.yml
```
Expected: no output (success). If `actionlint` is not installed: `brew install actionlint` (macOS) or skip and rely on CI to catch it.

- [ ] **Step 4: Commit**

```
git add .github/workflows/docker.yml
git commit -m "ci(docker): add workflow_dispatch and SLSA attestation permissions"
```

---

## Task 2: Attest the `go-api` Image

**Files:**
- Modify: `.github/workflows/docker.yml` (the `build-go-api` job)

**Why:** Each image needs its attestation steps. Start with `go-api` as the reference; Tasks 3 and 4 apply the same pattern to the other two.

- [ ] **Step 1: Add `id: build` to the push step and disable inline attestations**

Find the `build-push-action` step inside `build-go-api`:

```yaml
      - uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile
          push: ${{ github.event_name != 'pull_request' }}
          platforms: linux/amd64,linux/arm64
          cache-from: type=gha
          cache-to: type=gha,mode=max
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
```

Replace with:

```yaml
      - name: Build and push
        id: build
        uses: docker/build-push-action@v7
        with:
          context: .
          file: Dockerfile
          push: ${{ github.event_name != 'pull_request' }}
          platforms: linux/amd64,linux/arm64
          cache-from: type=gha
          cache-to: type=gha,mode=max
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          provenance: false
          sbom: false
```

- [ ] **Step 2: Append SBOM generation step**

Directly after the push step, add:

```yaml
      - name: Generate SBOM (go-api)
        if: github.event_name != 'pull_request'
        uses: anchore/sbom-action@v0
        with:
          image: ghcr.io/${{ github.repository_owner }}/go-api@${{ steps.build.outputs.digest }}
          format: spdx-json
          output-file: sbom-go-api.spdx.json
```

- [ ] **Step 3: Append SBOM attestation step**

```yaml
      - name: Attest SBOM (go-api)
        if: github.event_name != 'pull_request'
        uses: actions/attest-sbom@v3
        with:
          subject-name: ghcr.io/${{ github.repository_owner }}/go-api
          subject-digest: ${{ steps.build.outputs.digest }}
          sbom-path: sbom-go-api.spdx.json
          push-to-registry: true
```

- [ ] **Step 4: Append provenance attestation step**

```yaml
      - name: Attest build provenance (go-api)
        if: github.event_name != 'pull_request'
        uses: actions/attest-build-provenance@v3
        with:
          subject-name: ghcr.io/${{ github.repository_owner }}/go-api
          subject-digest: ${{ steps.build.outputs.digest }}
          push-to-registry: true
```

- [ ] **Step 5: Validate the YAML**

```
actionlint .github/workflows/docker.yml
```
Expected: no output.

- [ ] **Step 6: Commit**

```
git add .github/workflows/docker.yml
git commit -m "ci(slsa): attest go-api image with provenance and SBOM"
```

---

## Task 3: Attest the `python-worker` Image

**Files:**
- Modify: `.github/workflows/docker.yml` (the `build-python-worker` job)

Identical pattern to Task 2, but with `python-worker` in place of `go-api` and SBOM filename `sbom-python-worker.spdx.json`. The existing `build-push-action` step uses `context: ai-worker` and `file: ai-worker/Dockerfile` — those do not change.

- [ ] **Step 1: Add `id: build` + `provenance: false` + `sbom: false` to the push step.**

- [ ] **Step 2: Append the three new steps**, parameterized with `python-worker`:

```yaml
      - name: Generate SBOM (python-worker)
        if: github.event_name != 'pull_request'
        uses: anchore/sbom-action@v0
        with:
          image: ghcr.io/${{ github.repository_owner }}/python-worker@${{ steps.build.outputs.digest }}
          format: spdx-json
          output-file: sbom-python-worker.spdx.json

      - name: Attest SBOM (python-worker)
        if: github.event_name != 'pull_request'
        uses: actions/attest-sbom@v3
        with:
          subject-name: ghcr.io/${{ github.repository_owner }}/python-worker
          subject-digest: ${{ steps.build.outputs.digest }}
          sbom-path: sbom-python-worker.spdx.json
          push-to-registry: true

      - name: Attest build provenance (python-worker)
        if: github.event_name != 'pull_request'
        uses: actions/attest-build-provenance@v3
        with:
          subject-name: ghcr.io/${{ github.repository_owner }}/python-worker
          subject-digest: ${{ steps.build.outputs.digest }}
          push-to-registry: true
```

- [ ] **Step 3: Lint**

```
actionlint .github/workflows/docker.yml
```

- [ ] **Step 4: Commit**

```
git add .github/workflows/docker.yml
git commit -m "ci(slsa): attest python-worker image with provenance and SBOM"
```

---

## Task 4: Attest the `frontend` Image

**Files:**
- Modify: `.github/workflows/docker.yml` (the `build-frontend` job)

Identical pattern. Note: the frontend `build-push-action` step has extra `build-args` (VITE_API_URL, etc.) — those stay untouched. Only add `id`, `provenance: false`, `sbom: false` to it; append three new steps after.

- [ ] **Step 1: Add `id: build` + `provenance: false` + `sbom: false` to the push step.**

- [ ] **Step 2: Append the three new steps** with `frontend` substituted everywhere and SBOM filename `sbom-frontend.spdx.json`.

- [ ] **Step 3: Lint**

```
actionlint .github/workflows/docker.yml
```

- [ ] **Step 4: Commit**

```
git add .github/workflows/docker.yml
git commit -m "ci(slsa): attest frontend image with provenance and SBOM"
```

---

## Task 5: Dry-Run the Workflow and Verify Attestations Land

**Why:** Before writing consumer docs, confirm the workflow actually produces attestations on real pushes. If this fails, the docs would describe commands that won't work.

- [ ] **Step 1: Push the branch**

```
git push -u origin ci/slsa-provenance
```

- [ ] **Step 2: Trigger the workflow manually**

```
gh workflow run docker.yml --ref ci/slsa-provenance --repo ravencloak-org/Raven
```

- [ ] **Step 3: Wait for completion and record the digests**

Find the run:
```
gh run list --workflow docker.yml --branch ci/slsa-provenance --repo ravencloak-org/Raven --limit 1
```
Watch:
```
gh run watch <run-id> --repo ravencloak-org/Raven
```
Expected: all three `Build <image>` jobs succeed, including the three new steps per job. If any attestation step fails, read the log, fix, re-push, re-run.

- [ ] **Step 4: Confirm attestations are listed on GitHub**

```
gh attestation list --repo ravencloak-org/Raven --limit 10
```
Expected: six new attestations (provenance + SBOM for each of three images), all from commit SHA = tip of `ci/slsa-provenance`.

- [ ] **Step 5: Confirm OCI referrers in GHCR**

Install `crane` if needed: `brew install crane` (macOS).

```
# Resolve the sha-tagged image digest for go-api (use the branch-ref tag produced by docker/metadata-action@v6, i.e. "ci-slsa-provenance")
crane digest ghcr.io/ravencloak-org/go-api:ci-slsa-provenance

# Look up referrers for that digest
crane manifest ghcr.io/ravencloak-org/go-api@sha256:<digest> | jq .
# Then:
crane referrers ghcr.io/ravencloak-org/go-api@sha256:<digest>
```
Expected: two referrers, one with `artifactType` for SLSA provenance and one for SPDX SBOM.

Repeat for `python-worker` and `frontend`.

- [ ] **Step 6: (No commit — this is a verification task.)**

Record the digests + run URL in the PR description later.

---

## Task 6: Write Verification Documentation

**Files:**
- Create: `docs/security/slsa-verification.md`

**Why:** Closes child issue #332. Consumer-facing. Hardcodes `ravencloak-org` so copy-paste commands work as-is (per spec reviewer's advisory).

- [ ] **Step 1: Create the file with this exact content**

```markdown
# Verifying SLSA Build Provenance for Raven Container Images

Every container image published by this repository to GHCR carries two
cryptographic attestations:

- **SLSA v1 build provenance** — who built the image, from which commit,
  on which workflow, at what time.
- **SPDX SBOM** — the full software bill of materials for the image.

Both are signed keylessly via [Sigstore](https://www.sigstore.dev/) using
GitHub OIDC and anchored in the public [Rekor](https://docs.sigstore.dev/logging/overview/)
transparency log. Attestations are stored both in the GitHub attestation
store and as OCI referrers alongside the image in GHCR.

Images covered: `ghcr.io/ravencloak-org/{go-api,python-worker,frontend}`.
Attestations exist for every image pushed from `main` or a `v*.*.*` tag on
or after the commit that landed this feature.

## Verify with `gh` (GitHub CLI)

Primary path — no extra install beyond `gh`.

**Provenance:**

```
gh attestation verify \
  oci://ghcr.io/ravencloak-org/go-api:latest \
  --owner ravencloak-org \
  --predicate-type https://slsa.dev/provenance/v1
```

**SBOM:**

```
gh attestation verify \
  oci://ghcr.io/ravencloak-org/go-api:latest \
  --owner ravencloak-org \
  --predicate-type https://spdx.dev/Document
```

Substitute `go-api` with `python-worker` or `frontend` for the other two
images, and replace `latest` with any tag or `sha256:` digest you want to
verify.

## Verify with `cosign`

For environments without `gh`:

```
cosign verify-attestation \
  --type slsaprovenance1 \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/docker.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/ravencloak-org/go-api:latest
```

## What Verification Proves

- The image was built from `ravencloak-org/Raven` by the `docker.yml`
  workflow on a GitHub-hosted runner.
- The commit SHA in the provenance matches a real commit on this repo.
- The image digest you pull is byte-for-byte the one that was attested.
- The SBOM was produced by the same build.

## What Verification Does NOT Prove

- That the image is free of known vulnerabilities — that is what Trivy and
  `govulncheck` in `.github/workflows/security.yml` cover.
- That the build is bit-for-bit reproducible.
- Runtime integrity. Attestation is a build-time guarantee.

## Notes

- `actions/attest-sbom@v3` supports SPDX only; the SBOM attestation uses
  `spdx-json`. A CycloneDX variant is not produced.
- If you verify an image pushed before this feature landed, both commands
  will fail with "no attestation found". That is expected.
- If `gh attestation verify` fails with an auth error, run `gh auth login`
  and ensure the account has read access to `ravencloak-org/Raven`.

## References

- [SLSA v1.0 specification](https://slsa.dev/spec/v1.0/)
- [GitHub artifact attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
- [`gh attestation verify` manual](https://cli.github.com/manual/gh_attestation_verify)
- [`cosign verify-attestation` docs](https://docs.sigstore.dev/cosign/verifying/verify/)
```

- [ ] **Step 2: Run each `gh attestation verify` command from Step 1 against a real image from Task 5's test run**

Substitute `latest` with `ci-slsa-provenance` (the branch tag produced by `docker/metadata-action@v6`) for this dry-run. Expected: all three images verify successfully for both predicate types.

If a command fails, fix the doc (or the workflow) until it succeeds. Do not proceed with a doc that contains a broken command.

- [ ] **Step 3: Commit**

```
git add docs/security/slsa-verification.md
git commit -m "docs(slsa): add verification guide for container attestations"
```

---

## Task 7: Add README Badge

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add the badge line**

Inside the existing `<p align="center">...</p>` badge block in the README, after the Security badge line, insert:

```html
  <a href="docs/security/slsa-verification.md"><img src="https://slsa.dev/images/gh-badge-level3.svg" alt="SLSA 3" /></a>
```

Keep the surrounding badges in their current order.

- [ ] **Step 2: Commit**

```
git add README.md
git commit -m "docs(readme): add SLSA Level 3 badge"
```

---

## Task 8: Open PR and Queue Auto-Merge

**Files:** none — GitHub-side only.

**Why:** Project convention (CLAUDE.md): PRs are queued for squash auto-merge immediately after creation.

- [ ] **Step 1: Push latest commits**

```
git push
```

- [ ] **Step 2: Open the PR**

```
gh pr create \
  --repo ravencloak-org/Raven \
  --base main \
  --title "ci(slsa): add SLSA Build Level 3 provenance + SBOM attestations for GHCR images" \
  --body "Closes #331, closes #332, advances #333.

## Summary
- Adds \`actions/attest-build-provenance@v3\` + \`actions/attest-sbom@v3\` + \`anchore/sbom-action@v0\` steps to each of the three image jobs in \`.github/workflows/docker.yml\`.
- Grants \`id-token: write\` + \`attestations: write\` permissions at workflow level.
- Disables \`build-push-action\`'s inline provenance/SBOM to avoid dueling attestations.
- Adds \`workflow_dispatch\` trigger for manual re-runs.
- New \`docs/security/slsa-verification.md\` documents verify flow.
- README gains SLSA Level 3 badge.

Design: \`docs/superpowers/specs/2026-04-19-slsa-provenance-design.md\`
Plan: \`docs/superpowers/plans/2026-04-19-slsa-provenance.md\`

## Dry-run verification
Workflow run: <paste URL from Task 5>
All six attestations (2 per image × 3 images) verified with both \`gh attestation verify\` and \`cosign verify-attestation\`.

## Test plan
- [x] \`actionlint .github/workflows/docker.yml\` passes locally.
- [x] Manual \`workflow_dispatch\` run succeeds end-to-end on \`ci/slsa-provenance\`.
- [x] \`gh attestation list\` shows 6 new attestations.
- [x] \`crane referrers\` shows both referrers on each image digest.
- [x] Verify commands from the new doc succeed against the dry-run images."
```

- [ ] **Step 3: Queue squash auto-merge**

```
gh pr merge <PR_NUMBER> --auto --squash --repo ravencloak-org/Raven
```

- [ ] **Step 4: Post a comment linking the dry-run run URL if not already in the PR body.**

---

## Completion Criteria

- All 8 tasks checked off.
- PR squash-merged by auto-merge.
- Issues #331 and #332 auto-close on merge (via "Closes" in PR body).
- At least one post-merge build on `main` produces verifiable attestations end-to-end (confirm once merge lands).

## Out of Scope (Future Issues, Not Now)

- Deploy-time verification gate on edge / Raspberry Pi nodes.
- Source-tag signing via `gitsign`.
- Binary release attestations (no goreleaser workflow exists yet).
- Policy-as-code (Rego / Kyverno) for enforcement.
