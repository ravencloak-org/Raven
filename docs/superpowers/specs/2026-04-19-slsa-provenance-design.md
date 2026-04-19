# SLSA Build Provenance for GHCR Images

**Status:** Approved — ready for implementation plan
**Date:** 2026-04-19
**Scope:** Producer-side only (SLSA "Get Started" — Producer path)
**Target:** SLSA Build Level 3 on GitHub-hosted runners

## Goal

Generate SLSA Build Level 3 provenance and SPDX SBOM attestations for the three
container images published by `.github/workflows/docker.yml`
(`ghcr.io/ravencloak-org/{go-api,python-worker,frontend}`), signed keylessly via
Sigstore (GitHub OIDC), and stored both in the GitHub attestation store and as
OCI referrers alongside the image in GHCR.

Verification is documented but not enforced at deploy time in this iteration.

## In Scope

- Edits to `.github/workflows/docker.yml` to add attestation steps for all
  three images.
- Syft-based SBOM generation (`anchore/sbom-action@v0`, SPDX-JSON format).
- A new `docs/security/slsa-verification.md` documenting both `gh attestation
  verify` and `cosign verify-attestation` commands for consumers.
- A SLSA Level 3 badge in `README.md` linking to the verification doc.

## Out of Scope

- Runtime policy enforcement at deploy time (edge/Raspberry Pi nodes) —
  tracked as a future follow-up.
- Binary releases (no `goreleaser` workflow exists today; if added, it will
  need its own attestation pattern).
- Images outside `docker.yml` (e.g. anything under `landing/`, `backup/`,
  `deploy/` that is not currently pushed by this workflow).
- Signing of source git tags (e.g. `gitsign`).
- Changes to vulnerability scanning (`.github/workflows/security.yml` is
  untouched).

## Architecture

For each of the three `build-<image>` jobs in `docker.yml`, after the existing
`docker/build-push-action@v7` step, three additional steps are appended:

1. **Generate SBOM** — `anchore/sbom-action@v0` runs Syft against the pushed
   image (addressed by digest, not tag) and writes an SPDX-JSON file.
2. **Attest SBOM** — `actions/attest-sbom@v3` takes the SPDX file and the
   image digest, signs keylessly via Sigstore, and pushes the attestation
   both to the GitHub attestation store and to GHCR as an OCI referrer.
3. **Attest build provenance** — `actions/attest-build-provenance@v3`
   produces a SLSA v1 provenance statement for the same image digest, signed
   via the same keyless flow, stored the same two ways.

The digest used as the attestation subject comes from
`steps.build.outputs.digest` (where `build` is the `id` assigned to the
existing `docker/build-push-action` step). Never bind attestations to a tag —
tags are mutable; `sha256:...` content addresses are not.

`docker/build-push-action@v7` has its own optional provenance and SBOM flags.
Both are explicitly disabled (`provenance: false`, `sbom: false`) to avoid two
competing attestations per image; `actions/attest-*@v3` is the single source.

The three image jobs attest independently. No shared job, no reusable
workflow. A failure in one image's attestation does not affect the others.

## Triggers and Guards

The existing workflow runs on `push` (main + `v*.*.*` tags) and `pull_request`
to main. The build-push-action already uses
`push: ${{ github.event_name != 'pull_request' }}` — PR builds do not push,
so they have no digest to attest against.

All three new steps therefore guard on
`if: github.event_name != 'pull_request'`. On PRs, they simply don't run.

## Permissions

The workflow's top-level `permissions` block gains two scopes:

```yaml
permissions:
  contents: read
  packages: write
  id-token: write       # NEW — Sigstore OIDC for keyless signing
  attestations: write   # NEW — write to GitHub attestation store
```

`id-token: write` is the Sigstore OIDC scope. `attestations: write` is the
GitHub-native attestation-store scope. Both are required by
`actions/attest-build-provenance@v3` and `actions/attest-sbom@v3`.

## Workflow Diff Shape

Each `build-<image>` job changes as follows (sketch — final YAML will match
each job's existing context and image name):

```yaml
- name: Build and push
  id: build                  # NEW — needed to read outputs.digest
  uses: docker/build-push-action@v7
  with:
    context: .               # (or ai-worker / frontend)
    file: Dockerfile
    push: ${{ github.event_name != 'pull_request' }}
    platforms: linux/amd64,linux/arm64
    cache-from: type=gha
    cache-to: type=gha,mode=max
    tags: ${{ steps.meta.outputs.tags }}
    labels: ${{ steps.meta.outputs.labels }}
    provenance: false        # NEW — use attest-build-provenance instead
    sbom: false              # NEW — use attest-sbom instead

- name: Generate SBOM
  if: github.event_name != 'pull_request'
  uses: anchore/sbom-action@v0
  with:
    image: ghcr.io/${{ github.repository_owner }}/go-api@${{ steps.build.outputs.digest }}
    format: spdx-json
    output-file: sbom-go-api.spdx.json

- name: Attest SBOM
  if: github.event_name != 'pull_request'
  uses: actions/attest-sbom@v3
  with:
    subject-name: ghcr.io/${{ github.repository_owner }}/go-api
    subject-digest: ${{ steps.build.outputs.digest }}
    sbom-path: sbom-go-api.spdx.json
    push-to-registry: true

- name: Attest build provenance
  if: github.event_name != 'pull_request'
  uses: actions/attest-build-provenance@v3
  with:
    subject-name: ghcr.io/${{ github.repository_owner }}/go-api
    subject-digest: ${{ steps.build.outputs.digest }}
    push-to-registry: true
```

The `python-worker` and `frontend` jobs follow the same pattern with their
own image names, contexts (`ai-worker`, `frontend`), and SBOM filenames
(`sbom-python-worker.spdx.json`, `sbom-frontend.spdx.json`).

## Verification Documentation

A new file, `docs/security/slsa-verification.md`, is added. It contains:

1. **What is attested.** Every `ghcr.io/<owner>/{go-api,python-worker,
   frontend}` image pushed from `main` or a `v*.*.*` tag carries a SLSA v1
   build provenance attestation and an SPDX SBOM attestation, both signed
   keylessly via Sigstore and anchored in the public Rekor transparency log.

2. **Verify with `gh`.** Primary path — no extra install beyond GitHub CLI:

   ```
   gh attestation verify oci://ghcr.io/<owner>/go-api:<tag> \
     --owner <owner> \
     --predicate-type https://slsa.dev/provenance/v1

   gh attestation verify oci://ghcr.io/<owner>/go-api:<tag> \
     --owner <owner> \
     --predicate-type https://spdx.dev/Document
   ```

3. **Verify with `cosign`.** For non-GitHub tooling:

   ```
   cosign verify-attestation \
     --type slsaprovenance1 \
     --certificate-identity-regexp 'https://github.com/<owner>/Raven/.github/workflows/docker.yml@.*' \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     ghcr.io/<owner>/go-api:<tag>
   ```

4. **What verification proves (and does not prove).** Proves: source repo,
   source commit, workflow path, runner type, build timestamp, content
   integrity (digest binding). Does not prove: vulnerability-freeness (that
   is Trivy/govulncheck), reproducibility, or runtime integrity.

5. **Troubleshooting.** Two common failure modes: (a) verifying an image
   pushed before this change lands (no attestation exists), (b) `gh` not
   authenticated or lacking read access to the org.

The project `README.md` gains a single line near the top:

```
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](docs/security/slsa-verification.md)
```

## Testing and Rollout

**Workflow validation:**
1. Run `actionlint` locally against the modified `docker.yml`.
2. Push to the `ci/slsa-provenance` feature branch with a temporary
   `workflow_dispatch` trigger. Confirm:
   - All three image jobs succeed.
   - Each image has attestations visible via
     `gh attestation list --repo ravencloak-org/Raven`.
   - OCI referrers are present via
     `crane manifest ghcr.io/.../go-api:sha-<sha>` (or
     `oras discover`).
3. Run both verify commands from `docs/security/slsa-verification.md`
   against the test image. Only merge once both succeed.

**Rollout:** single PR, squash-merged. Additive change; existing unsigned
images keep working. Rollback = revert the PR. Images built before the
revert remain verifiable — Rekor is append-only.

## Risks and Mitigations

- **Fulcio / Rekor outage during build.** `attest-build-provenance` fails
  the step, which fails the job, which fails the workflow. We'd rather not
  push an image than push one without provenance. Mitigation: workflow
  re-run once Sigstore recovers.
- **Digest/subject drift.** Guarded by consistently using
  `steps.build.outputs.digest` as the subject. Tags are never used.
- **Extra OCI traffic.** Each image gains two OCI referrers per build
  (provenance + SBOM). Negligible relative to existing layer pushes.
- **Syft on Go-with-eBPF binaries.** SBOMs for `go-api` will include
  cilium/ebpf artifacts. Expected, not a bug; consumers can filter.
- **Confusion with `build-push-action` inline provenance.** Explicitly
  disabled via `provenance: false` and `sbom: false` to avoid dueling
  attestations.

## Future Follow-ups (not in this change)

- Deploy-time verification gate in `deploy/` scripts / edge node bootstrap.
- Source-provenance for git tags (`gitsign`).
- Binary attestations once a `goreleaser` or equivalent binary-release
  workflow is added.
- Policy-as-code (Rego / Kyverno) for the verification gate.
