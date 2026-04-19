# Dependency Policy

How Raven selects, obtains, and tracks its dependencies. Satisfies **OSPS-DO-06.01** from the [OpenSSF Baseline](https://baseline.openssf.org/versions/2026-02-19).

## Selection Criteria

New dependencies are evaluated against:

1. **License** — must be compatible with Apache 2.0 (the OSS project license). Preferred: Apache 2.0, MIT, BSD-2/3-Clause, ISC, MPL 2.0. Avoided without maintainer review: GPL / AGPL / SSPL / other copyleft.
2. **Maintenance signal** — release within the last 12 months, active issue tracker, responsive to security reports.
3. **Maintainer breadth** — prefer more than one active maintainer on the upstream.
4. **Security track record** — no unresolved high-severity CVEs; a published security policy is a plus.
5. **Footprint** — the minimum that solves the problem. Transitive dep count matters, especially for the edge/Raspberry Pi target.
6. **Supply chain** — packages published under an identifiable org account; avoid long-tail single-author packages for security-sensitive paths.

## Obtaining

| Ecosystem | Source of truth | Lockfile | Registry |
|---|---|---|---|
| Go | `go.mod` | `go.sum` | proxy.golang.org (default) |
| Python (ai-worker) | `ai-worker/pyproject.toml` (and/or `requirements*.txt`) | pip-tools output | PyPI |
| Node / Bun (frontend) | `frontend/package.json` | `frontend/bun.lock` | npm registry |
| Docker base images | `Dockerfile` and `ai-worker/Dockerfile` | tag pins | Docker Hub / GHCR official repos |
| GitHub Actions | `.github/workflows/*.yml` | commit SHA pins | github.com |

GitHub Actions are **pinned to commit SHA**, not floating tags. This prevents silent supply-chain attacks where a tag is retargeted.

## Tracking

Automated monitoring covers each ecosystem weekly:

- **[Dependabot](../.github/dependabot.yml)** — weekly PRs for `gomod`, `pip`, `npm`, `docker`, `github-actions`. Up to 10 open per ecosystem.
- **[Trivy](../.github/workflows/security.yml)** — filesystem scan (Critical + High only) on every push/PR that touches Go/Python/Docker; weekly scheduled run on Mondays.
- **[govulncheck](../.github/workflows/security.yml)** — Go-specific advisory check on every push/PR.
- **[CodeQL](../.github/workflows/codeql.yml)** — SAST covering Go + Python + JS/TS on every PR and weekly.
- **[gitleaks](../.github/workflows/gitleaks.yml)** — secret scanning on every push/PR and weekly.
- **GitHub Advanced Security / Dependabot alerts** — in-repo advisories surfaced at `/security/dependabot`.

## Review & Update Cadence

- Dependabot PRs are reviewed within **7 days** of opening. Patch/minor bumps that pass CI auto-merge after review; major bumps require manual validation.
- Any CVE with CVSS ≥ 7.0 is triaged within **72 hours** of alert receipt. If no fix is available upstream, document the mitigation in `.trivyignore` with rationale and an expiry date.
- Unmaintained dependencies (no release in 18+ months + no response to a security issue) are candidates for replacement or vendoring.

## Vendoring & Pinning

Raven does not vendor Go modules. Lockfiles (`go.sum`, `bun.lock`, Python pinned requirements) are committed and reviewed.

Docker base images pin by tag (e.g., `postgres:18.2-alpine`) rather than SHA for readability; tag movement is caught by Dependabot's docker ecosystem.

## Quarantined / Exempted Dependencies

Entries in [`.trivyignore`](../.trivyignore) document every known unfixed vulnerability plus the reason it is exempted (upstream has no patch, not reachable in our code path, etc.) and — where possible — a date at which we should revisit.

A CVE entering `.trivyignore` requires:

1. Confirming no upstream fix is available.
2. A code-level check that the vulnerable path is not exercised in our usage, or a mitigation note.
3. Maintainer review.

## License Compliance at Release

Before any tagged release, `go-licenses` / `pip-licenses` / `license-checker` runs (or will run — see OpenSSF L2 Phase 4 work) produce a licenses manifest bundled with release artifacts. Apache-2.0-incompatible licenses in direct dependencies block the release.

## Out of Scope

- **Enterprise Edition** (`ee-*` files) may carry different dependency rules; covered separately.
- **LLM provider APIs** — tenant-supplied at runtime (BYOK) and not part of the build-time dependency graph.

## References

- [`SECURITY.md`](../SECURITY.md) — vulnerability disclosure policy.
- [`MAINTAINERS.md`](../MAINTAINERS.md) — who triages.
- [OpenSSF Baseline 2026-02-19](https://baseline.openssf.org/versions/2026-02-19) — control taxonomy.
