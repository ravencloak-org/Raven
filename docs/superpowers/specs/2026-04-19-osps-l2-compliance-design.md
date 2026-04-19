# OpenSSF Baseline L2 Compliance — Design

**Date:** 2026-04-19
**Baseline version:** [OSPS Baseline 2026-02-19](https://baseline.openssf.org/versions/2026-02-19)
**Target maturity:** Level 2 (includes all Level 1 controls)
**Milestone:** `M-OSPS-L2`
**Owner:** @jobinlawrance

## 1. Scope & Boundary

### In scope (OSS portion)

The code, artifacts, and documentation released under `LICENSE`:

- Backend API (`cmd/`, `internal/`, `pkg/`)
- Python AI worker (`ai-worker/`)
- Frontend (`frontend/`)
- Database migrations (`migrations/`)
- Deployment manifests (`deploy/`, `docker-compose*.yml`, `Dockerfile*`)
- Public documentation (`docs/`, `README.md`, `DEVELOPMENT.md`, `CONTRIBUTING.md`)

Releases of these artifacts (Go binaries, Docker images, frontend bundle) are the "released software assets" against which OSPS-LE-02.02 and OSPS-BR-* controls apply.

### Out of scope (EE portion)

Files under `ee-LICENSE` (currently `ee-LICENSE`, `ee-README.md`, any future `ee-*` content). These are non-OSS and are not published through the L2-compliant release pipeline. The boundary is documented in the root `README.md`. If/when EE artifacts ship, that is a separate compliance exercise.

### Prerequisites (fix-first blockers)

- `LICENSE` MUST be verified as OSI-approved before anything else in this milestone proceeds. If it isn't, that is the first fix.
- `ee-LICENSE` text MUST clearly mark its content as non-OSS (non-blocking for L2 scope, but recommended for clarity).

## 2. Gap Matrix

Legend: ✅ satisfied · ⚠ partial · ❌ missing

### Access Control

| Control | State | Action |
|---|---|---|
| AC-01.01 MFA on sensitive actions | ⚠ | Enforce MFA at GitHub org/repo level |
| AC-02.01 New collaborators min-priv | ✅ | GitHub default — verify, no change |
| AC-03.01 Block direct commits to `main` | ⚠ | Branch protection: require PR + 1 review + status checks |
| AC-03.02 Prevent `main` deletion | ⚠ | Branch protection "restrict deletions" |
| AC-04.01 CI/CD least-privilege default | ⚠ | Top-level `permissions: {}` + explicit per-job grants in every workflow |

### Build & Release

| Control | State | Action |
|---|---|---|
| BR-01.01 Sanitize untrusted CI metadata | ⚠ | Audit workflows for unescaped `${{ github.event.* }}` |
| BR-01.03 Priv creds blocked on untrusted code | ⚠ | Eliminate / tightly scope any `pull_request_target` |
| BR-02.01 Unique version identifier | ❌ | Adopt SemVer tags `v0.x.x`; enforce in release workflow |
| BR-03.01 Encrypted channels | ✅ | GitHub HTTPS — no change |
| BR-04.01 Release changelog | ❌ | `git-cliff` auto-generates from conventional commits |
| BR-05.01 Standardized dep tooling | ✅ | `go mod` / `npm` / `pip` already in use |
| BR-06.01 Signed release + hashes | ❌ | `goreleaser` + `cosign` keyless (OIDC) → signed binaries, Docker images, `checksums.txt`, `.sig`, `.pem` |
| BR-07.01 No secrets in VCS | ⚠ | Add `gitleaks` CI job (blocking) + audit `.gitignore` |

### Documentation

| Control | State | Action |
|---|---|---|
| DO-01.01 User guides | ✅ | `README.md` + `docs/` — verify completeness at release time |
| DO-06.01 Dep selection/tracking docs | ❌ | Add "Dependency Policy" section to `DEVELOPMENT.md` or `docs/dependency-policy.md` |
| DO-07.01 Build instructions | ✅ | `DEVELOPMENT.md` exists |

### Governance

| Control | State | Action |
|---|---|---|
| GV-01.01 List of members with sensitive access | ❌ | Create `MAINTAINERS.md` |
| GV-01.02 Roles/responsibilities | ❌ | Part of `MAINTAINERS.md` |
| GV-03.01 Contribution process | ✅ | `CONTRIBUTING.md` exists |
| GV-03.02 Contribution requirements | ⚠ | Extend `CONTRIBUTING.md` with coding standards, test requirements, submission rules |

### Legal

| Control | State | Action |
|---|---|---|
| LE-01.01 Contributor legal assertion | ❌ | DCO (`Signed-off-by:`) via `dco` action; or CLA Assistant |
| LE-02.01 OSI source license | ⚠ | Verify `LICENSE` is OSI-approved (prerequisite) |
| LE-02.02 OSI released-asset license | ⚠ | Same verification; document in `README.md` licensing section |

### Quality

| Control | State | Action |
|---|---|---|
| QA-01.02 Public commit record | ✅ | GitHub native |
| QA-02.01 Dependency list | ✅ | `go.mod`, `package.json`, `requirements.txt` |
| QA-04.01 Codebase list (multi-repo) | N/A | Monorepo |
| QA-05.01/02 No executables/binaries in VCS | ⚠ | `gitleaks` + file-size gate in pre-commit |

### Security Assessment

| Control | State | Action |
|---|---|---|
| SA-01.01 Design/architecture docs | ❌ | Add `docs/architecture.md` with system diagram, actors, trust boundaries |
| SA-02 SAST | ❌ | CodeQL workflow (Go, Python, JS/TS), PR + weekly schedule |
| SA-03 Dependency scanning | ✅ | `govulncheck` + Trivy + Dependabot already in `security.yml` |

### Vulnerability Management

| Control | State | Action |
|---|---|---|
| VM-01.01 CVD policy | ❌ | `SECURITY.md` — 90-day disclosure window |
| VM-02.01 Security contacts | ❌ | In `SECURITY.md` |
| VM-04.01 Publish discovered vulns | ❌ | GitHub Security Advisories — process documented in `SECURITY.md` |

**Totals:** 18 gaps, 9 partials. Most are docs + config; the release pipeline is the only substantial new build.

## 3. Release Pipeline Architecture

### Trigger

Tag push matching `v[0-9]+.[0-9]+.[0-9]+(-rc[0-9]+)?`. Tags are cut from `main` only; workflow preflight job rejects tags on other branches.

### Flow

```
dev commits → main (squash-merged, conventional commits)
                  │
                  ▼
        git tag v0.x.x && push
                  │
                  ▼
       .github/workflows/release.yml  (tag trigger)
                  │
     ┌────────────┼─────────────┬──────────────┬─────────────┐
     ▼            ▼             ▼              ▼             ▼
 preflight    changelog     go-binaries    docker-images   frontend-bundle
 (verify tag (git-cliff      (goreleaser:   (buildx multi-  (npm run build
  on main,    since last     linux amd64/   arch amd64 +    → dist/
  SemVer,     tag → release  arm64 + darwin arm64 → ghcr.   → frontend-
  fail        notes.md)      arm64;         io; cosign      vX.Y.Z.tgz;
  otherwise)                 SHA256SUMS;    sign each;      cosign sign)
                             cosign sign    provenance
                             checksums)     attestation)
                                │             │                │
                                └──────┬──────┴────────────────┘
                                       ▼
                              publish-release
                           (GitHub Release: notes +
                            artifacts + .sig + .pem
                            + SHA256SUMS)
```

### Key choices

- **Tag convention:** SemVer, `vMAJOR.MINOR.PATCH` (pre-1.0 → `v0.x.x`). Satisfies BR-02.01.
- **Signing — cosign keyless (OIDC):** no key management; each signature produces `.sig` + `.pem` (short-lived Fulcio cert) and is recorded in the Rekor transparency log. Verification commands published in `SECURITY.md`. Satisfies BR-06.01.
- **Changelog — git-cliff:** reads conventional commits since the last tag, emits release body + optional `CHANGELOG.md` append. Satisfies BR-04.01.
- **goreleaser:** Go binaries (multi-arch), `SHA256SUMS` generation, cosign invocation via its `signs:` block.
- **Docker images:** `ghcr.io/<owner>/raven-api:v0.x.x` + `:latest`, `ghcr.io/<owner>/raven-ai-worker:v0.x.x` + `:latest`. Each cosign-signed. Multi-arch (amd64 + arm64) via buildx — required for the Raspberry Pi/edge deployment target.
- **Frontend bundle:** `frontend-vX.Y.Z.tgz` containing `dist/`; cosign-signed blob attached to the Release.

### Workflow permissions (OSPS-AC-04.01)

Top-level: `permissions: {}` (empty).

Per-job:

| Job | Permissions |
|---|---|
| `preflight` | `contents: read` |
| `changelog` | `contents: read` |
| `go-binaries` | `contents: write`, `id-token: write` |
| `docker-images` | `contents: read`, `packages: write`, `id-token: write`, `attestations: write` |
| `frontend-bundle` | `contents: write`, `id-token: write` |
| `publish-release` | `contents: write` |

### Non-release CI additions

- **`.github/workflows/codeql.yml`** — CodeQL analysis for Go, Python, JS/TS. PR + weekly schedule. Satisfies SA-02.
- **`.github/workflows/gitleaks.yml`** — gitleaks on PR + push; fails on any detected secret. Satisfies BR-07.01 + QA-05.
- **`.github/workflows/dco.yml`** — DCO sign-off check. Satisfies LE-01.01.
- **Existing `security.yml` (Trivy + govulncheck)** — retained as-is. Satisfies SA-03.

### Files to create / modify

**New:**

- `.github/workflows/release.yml`
- `.github/workflows/codeql.yml`
- `.github/workflows/gitleaks.yml`
- `.github/workflows/dco.yml`
- `.goreleaser.yaml`
- `cliff.toml`
- `SECURITY.md`
- `MAINTAINERS.md`
- `docs/architecture.md`
- `docs/dependency-policy.md` (or section appended to `DEVELOPMENT.md`)
- `docs/compliance/osps-l2-2026-02-19.md` (self-assessment)

**Modified:**

- `README.md` — add "Licensing" section (OSS vs EE boundary) + links to `SECURITY.md`, `MAINTAINERS.md`, verification instructions
- `CONTRIBUTING.md` — append coding standards, test requirements, `Signed-off-by:` sign-off section
- All existing `.github/workflows/*.yml` — top-level `permissions: {}` + per-job grants

### Branch protection (GitHub UI, no code)

- Require PR before merge; 1 approving review
- Require CODEOWNERS review
- Required status checks: `go`, `frontend`, `python`, `security`, `codeql`, `gitleaks`, `dco`, `ci-required`
- Disallow force pushes; restrict deletion
- Linear history (already enforced by squash-only policy)

## 4. Milestone Breakdown

Each row below = one issue = one PR. Issues within a phase may run in parallel (separate worktrees per the milestone protocol).

### Phase 0 — Blocker verification

| # | Title | Controls |
|---|---|---|
| OSPS-1 | License audit & fix | LE-02.01, LE-02.02 |

Blocks every other issue. If `LICENSE` is not OSI-approved, stop and resolve before proceeding.

### Phase 1 — Documentation (parallel, after Phase 0)

| # | Title | Controls |
|---|---|---|
| OSPS-2 | `SECURITY.md` + enable GH private vuln reporting | VM-01.01, VM-02.01, VM-04.01 |
| OSPS-3 | `MAINTAINERS.md` | GV-01.01, GV-01.02 |
| OSPS-4 | Extend `CONTRIBUTING.md` with standards + DCO sign-off | GV-03.02, LE-01.01 |
| OSPS-5 | `docs/architecture.md` | SA-01.01 |
| OSPS-6 | Dependency policy doc | DO-06.01 |
| OSPS-7 | `README.md` licensing section + OSS/EE boundary | LE-02 scope |

### Phase 2 — Repo settings (owner-only, parallel to Phase 1)

| # | Title | Controls |
|---|---|---|
| OSPS-8 | Branch protection + MFA + org settings | AC-01.01, AC-02.01, AC-03.01, AC-03.02 |

No code. Checklist applied via GitHub UI / `gh api`. Evidence captured as a screenshot or `gh api` output in the PR description of OSPS-18.

### Phase 3 — CI hardening (parallel)

| # | Title | Controls |
|---|---|---|
| OSPS-9 | Audit & lock `permissions:` in all workflows | AC-04.01, BR-01.01, BR-01.03 |
| OSPS-10 | `gitleaks` workflow + pre-commit hook | BR-07.01, QA-05.01, QA-05.02 |
| OSPS-11 | CodeQL workflow (Go, Python, JS/TS) | SA-02 |
| OSPS-12 | DCO check workflow | LE-01.01 |

### Phase 4 — Release pipeline (depends on Phase 1 + 3)

| # | Title | Controls |
|---|---|---|
| OSPS-13 | `.goreleaser.yaml` + Go binary release path | BR-02.01, BR-06.01 |
| OSPS-14 | `cliff.toml` + changelog generation | BR-04.01 |
| OSPS-15 | Release workflow — Docker images (API + ai-worker, multi-arch) | BR-06.01 |
| OSPS-16 | Release workflow — frontend bundle | BR-06.01 |
| OSPS-17 | Dry-run `v0.1.0-rc1` tag + verification | All BR-* |

OSPS-17 is the integration gate: cuts a release candidate, runs all `cosign verify` commands end-to-end, confirms the Release page contains every expected artifact. Only after this passes can a production tag follow.

### Phase 5 — Wrap-up

| # | Title | Controls |
|---|---|---|
| OSPS-18 | Self-assessment doc + README badge | — |

`docs/compliance/osps-l2-2026-02-19.md` maps every L1 + L2 control to its evidence (PR number, file path, screenshot). Badge in README links to the assessment. Baseline version is pinned in the filename so future upgrades are explicit.

### Execution strategy

Per the project's milestone protocol (parallel worktree agents, quality gates, CodeRabbit review, release on milestone completion):

- Phases 1, 2, 3 run concurrently in separate worktrees after Phase 0 clears.
- Phase 4 is internally sequential (each PR builds on the previous) but begins as soon as Phase 1 + 3 land.
- Every PR: CodeRabbit review, CI gates (including the new `codeql`, `gitleaks`, `dco` checks once merged), squash-merge, no AI-attribution trailers.
- Milestone closes with OSPS-17 dry-run passing and OSPS-18 assessment merged.

**Estimated duration:** 2–4 weeks at steady cadence, parallelism-dependent.

## 5. Verification

Each control is verified one of three ways:

- **Artifact in repo** (e.g., `SECURITY.md` exists and satisfies the required fields).
- **CI job passes** (e.g., `codeql` green on every PR).
- **External check** — documented command in OSPS-18 self-assessment. Example for BR-06.01:

  ```bash
  cosign verify-blob \
    --certificate checksums.txt.pem \
    --signature checksums.txt.sig \
    --certificate-identity-regexp '^https://github.com/<owner>/<repo>' \
    --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
    checksums.txt
  ```

Each of these lands in `docs/compliance/osps-l2-2026-02-19.md` as the final deliverable.

## 6. Non-goals

- Level 3 controls (SBOM per release, formal threat model + attack-surface analysis, full secret-management policy) — tracked as a future milestone. Tooling choices here (cosign, goreleaser, CodeQL) are compatible with a future L3 push; `syft` can plug into goreleaser for SBOM and a STRIDE-based threat model can be appended to `docs/architecture.md`.
- EE compliance — out of scope; tracked separately if/when EE artifacts are publicly released.
- SLSA L3 provenance generator (`slsa-framework/slsa-github-generator`) — compatible with this design but not required for L2; deferred.
