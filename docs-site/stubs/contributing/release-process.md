---
title: "Release Process"
---

# Release Process

Raven follows [Semantic Versioning](https://semver.org/). While the project
is pre-1.0, **minor version bumps may include breaking changes**; patch
releases (`vX.Y.Z` → `vX.Y.Z+1`) are reserved for bug-fix and security
backports. Once `v1.0.0` ships, the standard SemVer contract applies.

A release publishes the following artefacts together under a single Git tag:

- **Go binaries** — `raven-api` and `raven-worker`, built per OS/arch by
  GoReleaser (`linux/amd64`, `linux/arm64`, `darwin/arm64`).
- **Docker images** — `ghcr.io/ravencloak-org/{go-api,python-worker,frontend}`,
  multi-arch (`linux/amd64` + `linux/arm64`), cosign-signed with SLSA
  build provenance and SPDX SBOM attestations.
- **Frontend bundle** — `frontend-X.Y.Z.tgz` (Vite `dist/`) for self-hosted
  static deployments, with a SHA256 sum and a cosign blob signature.
- **Release notes** — generated from conventional commits via `git-cliff`.
- **GitHub Release** — assembled by `softprops/action-gh-release`.

Cloudflare Pages production deploys for the frontend, landing, and docs
sites are **not** part of this workflow; they ship continuously on every
push to `main` via dedicated workflows (`pages.yml`, `landing.yml`,
`docs.yml`).

## Versioning policy

| Phase | Rule |
|-------|------|
| `0.x.y` (current) | Minor bumps (`0.3 → 0.4`) may break APIs, CLI flags, env vars, or config. Patch bumps (`0.3.0 → 0.3.1`) MUST be backwards-compatible. |
| `1.x.y` (future)  | Standard SemVer: breaking changes only on major bumps; minors add features, patches fix bugs. |
| Release candidates | `vX.Y.Z-rcN` (matched by the workflow regex). The release is automatically marked **prerelease** on GitHub. |

Tags MUST match `^v[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)?$` — the preflight job
in `release.yml` rejects anything else.

## Release cadence

There is no fixed cadence. Releases are cut when a milestone closes or
when a critical fix needs to ship. The most recent published release
sequence on the signed-pipeline line:

| Tag       | Date         | Trigger |
|-----------|--------------|---------|
| `v0.3.0`  | 2026-04-23   | M9 milestone + security hardening (#362, etc.) |
| `v0.1.0`  | 2026-04-20   | First public release |

`v0.2.0` exists as an older pre-signed-pipeline tag and was deliberately
skipped in the signed-release sequence to avoid collision (see the
`Notes` section in `CHANGELOG.md`).

## How to cut a release

### 1. Pre-flight (local)

- Lint clean: `golangci-lint run ./...` and `ruff check ai-worker/`.
- Unit tests green: `go test ./...`, `pytest ai-worker/`,
  `cd frontend && npm run test`.
- Integration tests green: `make integration-test`.
- All milestone issues closed; all dependent PRs merged.
- `main` is green on CI.

### 2. Bump versions on `main`

Open a PR titled `chore(release): vX.Y.Z` that bumps:

- `frontend/package.json` → `"version": "X.Y.Z"`
- `ai-worker/pyproject.toml` → `version = "X.Y.Z"`

Get the PR reviewed and squash-merged. (The Go binaries embed the version
via `-X main.version={{.Version}}` ldflags injected by GoReleaser at build
time; no source bump is required there.)

### 3. Regenerate `CHANGELOG.md`

`cliff.toml` reads conventional commits (`feat:`, `fix:`, `perf:`,
`refactor:`, `docs:`, `test:`, `ci:`, `chore:`, `deps:`, `security:`,
`revert:`) and groups them into Markdown sections.

```bash
git cliff --tag vX.Y.Z -o CHANGELOG.md
```

Commit on `main` as `docs(changelog): vX.Y.Z` and merge.

### 4. Tag the release commit

Tags MUST be **signed** (the repo enforces signed commits + DCO; the same
key SHOULD sign tags) and MUST be cut from `main` — `release.yml`'s
preflight job verifies the tag is reachable from `origin/main` with
`git merge-base --is-ancestor` and aborts otherwise.

```bash
git checkout main
git pull --ff-only
git tag -s vX.Y.Z -m "vX.Y.Z — <one-line summary>"
git push origin vX.Y.Z
```

For a release candidate use `vX.Y.Z-rcN` (e.g. `v0.4.0-rc1`).

That is the entire manual step. The rest is the workflow.

## What the release workflow does

`.github/workflows/release.yml` triggers on tag push and runs six jobs in
DAG order. Each job's permissions are scoped to the minimum it needs.

### `preflight`

- Validates the tag against the SemVer regex (`v[0-9]+.[0-9]+.[0-9]+(-rcN)?`).
- Verifies the tag is reachable from `origin/main`.
- Exposes `tag` and `version` outputs to downstream jobs.

### `changelog`

- Runs `orhun/git-cliff-action` with `--latest --strip header` against
  `cliff.toml`.
- Writes `RELEASE_NOTES.md` and uploads it as an artefact.

### `go-binaries`

- Sets up Go (`1.26.x`) with module cache.
- Installs `cosign` via `sigstore/cosign-installer`.
- Runs `goreleaser release --clean --skip=publish` (GoReleaser v2). The
  `release.disable: true` setting in `.goreleaser.yaml` lets the workflow,
  not GoReleaser, create the GitHub Release.
- GoReleaser produces tar.gz archives per OS/arch + `checksums.txt`, then
  cosign-keyless-signs `checksums.txt` (Rekor transparency log →
  `checksums.txt.sig` + `checksums.txt.pem`).

### `docker-build` (matrix) → `docker-merge`

- `docker-build` runs **per (component, arch)** on native runners
  (`ubuntu-latest` for amd64, `ubuntu-24.04-arm` for arm64). Each leg
  pushes by digest only — no tags. This avoids QEMU emulation for the
  arm64 leg (CGO + eBPF build) and keeps the critical path under
  five minutes per arch.
- `docker-merge` downloads the per-arch digests, assembles a multi-arch
  manifest, tags it (`{{version}}`, `{{major}}.{{minor}}`, `latest`),
  signs it with `cosign sign --yes`, and attaches:
  - **SLSA build provenance** via `actions/attest-build-provenance@v4`
    (in-toto statement, OIDC-signed by the GitHub-hosted Sigstore Fulcio CA),
  - **SPDX SBOM** generated by `anchore/sbom-action`, attested via
    `actions/attest-sbom@v4` (SPDX JSON only — CycloneDX is not produced).
  Both attestations are pushed to the registry as referrer manifests.

### `frontend-bundle`

- Builds `frontend/` with `npm ci && npm run build`.
- Tarballs `dist/` as `frontend-X.Y.Z.tgz`, computes a SHA256 sum, and
  cosign-blob-signs the tarball.

### `publish-release`

- Downloads all artefacts from the previous jobs.
- Calls `softprops/action-gh-release` to create the GitHub Release with:
  - `tag_name` and `name` set to the tag,
  - `body_path` pointing at `RELEASE_NOTES.md`,
  - `prerelease: true` when the tag contains `-rc`,
  - the Go archives, checksums + signatures, and the frontend bundle +
    its signature attached as release assets.

Docker images are *not* attached as release assets — they live in
`ghcr.io/ravencloak-org/*` and are referenced by tag and digest.

## Verifying a release

Every signed artefact in the pipeline can be verified offline by anyone.
Full instructions live at [/trust/slsa-level-3](/trust/slsa-level-3).

Quick reference for the container images:

```bash
# Provenance
cosign verify-attestation \
  --type slsaprovenance1 \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/(docker|release)\.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/ravencloak-org/go-api:vX.Y.Z

# SBOM
cosign verify-attestation \
  --type spdxjson \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/(docker|release)\.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/ravencloak-org/go-api:vX.Y.Z
```

For the Go binary checksums:

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature   checksums.txt.sig \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/release\.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

`gh attestation verify` is the simpler path when the GitHub CLI is
available — see [/trust/slsa-level-3](/trust/slsa-level-3) for the
exact commands.

## Hotfix releases

For a critical security or correctness fix that lands on `main` between
scheduled releases:

1. Cherry-pick or merge the fix into `main` via PR (squash, signed).
2. Bump the patch version (`vX.Y.Z` → `vX.Y.(Z+1)`) per the steps above.
3. Regenerate `CHANGELOG.md` and tag.
4. The release workflow republishes images, binaries, and the frontend
   bundle exactly as for any other release.

There is no separate hotfix branch — Raven releases are always cut from
`main`, and the preflight job enforces this.

## Rollback

### Container images

Pinned tags are immutable in practice (signatures and attestations
reference a specific manifest digest). To roll back a deployment:

```bash
# In your docker-compose.yml or k8s manifest, replace the tag:
#   image: ghcr.io/ravencloak-org/go-api:v0.3.0
# with the previous good version:
#   image: ghcr.io/ravencloak-org/go-api:v0.1.0
docker compose pull && docker compose up -d
```

For paranoia, pin by digest (`@sha256:...`) — the merged manifest digest
is captured in the `docker-merge` job log.

### Go binaries / frontend bundle

Re-download the previous tag's assets from
`https://github.com/ravencloak-org/Raven/releases/tag/vX.Y.Z` and
re-deploy. Verify signatures before swapping in (see above).

### Cloudflare Pages sites (frontend / landing / docs)

These deploy continuously from `main`, not from tags. Rollback is via the
**Cloudflare Pages dashboard** → project → Deployments → previous
deployment → "Rollback to this deployment". A revert PR on `main` will
also redeploy automatically; pick whichever is faster.

## Cross-references

- [Dev setup](/contributing/dev-setup) — how to get a local build green
  before a release.
- [SLSA Level 3 verification](/trust/slsa-level-3) — full verification
  procedure, what the attestations prove, and what they don't.
- [Security policy](/reference/security-policy) — how to report a
  vulnerability that may warrant a hotfix.
