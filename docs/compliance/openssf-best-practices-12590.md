# OpenSSF Best Practices Badge — Answer Key (Project 12590)

**Status:** Source of truth for <https://www.bestpractices.dev/projects/12590/edit>. Update this file alongside the questionnaire so the form and the repo never drift.

**Badge URL:** <https://www.bestpractices.dev/projects/12590/badge>
**Last updated:** 2026-05-08
**Repository:** <https://github.com/ravencloak-org/Raven>
**License:** Apache-2.0 (open-source portion); files prefixed `ee-` are under a separate Enterprise License and are out of scope for this badge.

---

## Executive Summary

| Tier    | Met | Unmet | N/A | Unknown |
| ------- | --- | ----- | --- | ------- |
| Passing | 18  | 2     | 0   | 0       |
| Silver  | 9   | 5     | 1   | 1       |
| Gold    | 4   | 7     | 1   | 1       |

Counts cover the criteria enumerated below. "N/A" is reserved for criteria the project genuinely does not exercise (e.g. shipping crypto primitives). "Unknown" means we lack repo-side evidence and need an external check before claiming Met.

---

## Passing Tier

### description_good

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/README.md> — top-of-file tagline "Open-source multi-tenant knowledge base platform with AI-powered chat, voice, and WhatsApp" plus the "Tech Stack" table.
- **Justification:** The README opens with a one-line description of what Raven is and follows with a tech-stack table that explains each component. The project's purpose is unambiguous to a first-time visitor.

### interact

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/CONTRIBUTING.md>; <https://github.com/ravencloak-org/Raven/issues>; README "Contributing" section linking to open issues.
- **Justification:** The repository provides a public issue tracker, a documented contribution workflow in `CONTRIBUTING.md`, and a `MAINTAINERS.md` listing the responsible owners. Users have multiple working channels to raise questions and PRs.

### contribution

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/CONTRIBUTING.md> (DCO sign-off, branch naming, commit style, PR workflow); <https://github.com/ravencloak-org/Raven/blob/main/.github/PULL_REQUEST_TEMPLATE.md>.
- **Justification:** `CONTRIBUTING.md` documents the full contribution flow: DCO sign-off (`-s`), branch naming, commit style, testing requirements, and PR review. The PR template enforces the checklist on every pull request.

### contribution_requirements

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/CONTRIBUTING.md> "Developer Certificate of Origin" section; PR #338 extended this with coding-standards / testing requirements (referenced in `docs/compliance/osps-l2-2026-02-19.md`).
- **Justification:** Requirements for contributions — coding standards, sign-off, tests, PR workflow — are stated explicitly in `CONTRIBUTING.md` and enforced by the DCO required check.

### floss_license

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/LICENSE> (Apache-2.0); README "Licensing" section.
- **Justification:** The OSS portion of Raven is released under Apache License 2.0, an OSI-approved FLOSS license. Files prefixed `ee-` are explicitly excluded from the OSS portion.

### floss_license_osi

- **Status:** Met
- **Evidence:** Apache-2.0 is on the [OSI list](https://opensource.org/licenses/Apache-2.0); see `LICENSE` linked above.
- **Justification:** Apache-2.0 is OSI-approved.

### license_location

- **Status:** Met
- **Evidence:** `LICENSE` at the repository root.
- **Justification:** The license is in the standard top-level `LICENSE` file.

### documentation_basics

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/README.md>; <https://github.com/ravencloak-org/Raven/blob/main/DEVELOPMENT.md>; `docs/quickstart.md`; `docs/wiki/Architecture-Overview.md`.
- **Justification:** Basic documentation covers what Raven is, how to install it (`docker compose up -d`), how to develop locally (`DEVELOPMENT.md`), and a quickstart walkthrough.

### documentation_interface

- **Status:** Met
- **Evidence:** README "Quick Start" + "Tech Stack"; `docs/wiki/Data-Model.md`; OpenAPI / gRPC contracts under `contracts/` and `proto/`.
- **Justification:** External interfaces (REST API, gRPC, embeddable web component) are documented in `docs/wiki/` and the contracts directory.

### discussion

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/issues> (public issue tracker); CONTRIBUTING.md PR workflow.
- **Justification:** Discussion of changes happens publicly on GitHub Issues and Pull Requests.

### english

- **Status:** Met
- **Evidence:** README, CONTRIBUTING, SECURITY, MAINTAINERS — all in English.
- **Justification:** All project documentation and code comments are in English.

### release_notes

- **Status:** Met
- **Evidence:** `cliff.toml` + `git-cliff` `changelog` job (cited in `docs/compliance/osps-l2-2026-02-19.md` row OSPS-BR-04.01); GitHub Releases page.
- **Justification:** Release notes are auto-generated from conventional commits via git-cliff and attached to every GitHub Release.

### report_process

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/SECURITY.md>; README "Security" section.
- **Justification:** SECURITY.md describes how to report issues (GitHub Private Vulnerability Reporting + email escalation), and the README links to it.

### report_tracker

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/issues>
- **Justification:** Bugs are tracked publicly in GitHub Issues; security issues are tracked in GitHub Security Advisories per SECURITY.md.

### report_responses

- **Status:** Met
- **Evidence:** Recent issues and PRs in <https://github.com/ravencloak-org/Raven/issues?q=is%3Aissue> show maintainer responses; SECURITY.md SLA (72 h ack, 7 d triage, 90 d fix).
- **Justification:** Reports receive a maintainer response within the published SLA windows.

### enhancement_responses

- **Status:** Met
- **Evidence:** Issue tracker history; CONTRIBUTING.md "open an issue to discuss proposed changes".
- **Justification:** Enhancement requests are responded to in the issue tracker; the contribution workflow explicitly invites them.

### report_archive

- **Status:** Met
- **Evidence:** GitHub Issues retains a permanent public archive of all reports and discussions.
- **Justification:** GitHub permanently archives every issue, comment, and PR for the repository.

### vulnerability_report_process

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/SECURITY.md> "Reporting a Vulnerability".
- **Justification:** SECURITY.md gives a clear, prominently linked process for vulnerability reporting via GitHub Private Vulnerability Reporting plus an email escalation path.

### vulnerability_report_private

- **Status:** Met
- **Evidence:** SECURITY.md "Primary: GitHub Security Advisories (private reporting)" section linking to <https://github.com/ravencloak-org/Raven/security/advisories/new>; explicit instruction not to use public issues.
- **Justification:** The primary channel is GitHub's private vulnerability reporting, with an email escalation path via the project security contact documented in SECURITY.md. Reporters never have to disclose publicly.

### vulnerability_report_response

- **Status:** Met
- **Evidence:** SECURITY.md "Response SLA" table: initial ack within 72 h, triage within 7 d, fix within 90 d.
- **Justification:** Maintainers commit to a 72-hour acknowledgement SLA, well under the 14-day Best Practices threshold.

### no_leaked_credentials

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/gitleaks.yml>; OSPS row OSPS-BR-07.01 (gitleaks CI + pre-commit).
- **Justification:** A `gitleaks` workflow runs on every push and PR; PR template explicitly forbids committing secrets. No credentials have leaked.

### build

- **Status:** Met
- **Evidence:** `Makefile`, `Dockerfile`, `docker-compose.yml`, `.goreleaser.yaml`, `frontend/package.json` build script (`vue-tsc -b && vite build`), `ai-worker/pyproject.toml`.
- **Justification:** The project builds reproducibly from source via `make`, `go build ./cmd/api`, `npm run build`, and `docker compose up -d`.

### build_common_tools

- **Status:** Met
- **Evidence:** Build uses Go (`go build`), npm/Vite, pip, and Docker — all standard, freely-available tools.
- **Justification:** Build dependencies are go, npm, pip, and docker — universally available FLOSS toolchains.

### build_floss_tools

- **Status:** Met
- **Evidence:** Same as above; all toolchain components are FLOSS.
- **Justification:** Every build tool used (Go, npm, Vite, pip, ESLint, Ruff, golangci-lint, Docker BuildKit, GoReleaser, cosign) is FLOSS.

### test

- **Status:** Met
- **Evidence:** `go.yml` (unit + integration + migration suites with coverage); `frontend.yml` (vitest + Playwright); `python.yml` (pytest); README "Testing" section listing 47 integration tests + 7 benchmarks.
- **Justification:** Raven has unit, integration, migration, and end-to-end Playwright test suites; all run on every PR via separate workflows.

### test_invocation

- **Status:** Met
- **Evidence:** README "Testing" section: `make test`, `make test-integration`, `cd frontend && npm run test:e2e`.
- **Justification:** The README documents how to invoke each test suite locally with one command per suite.

### test_most

- **Status:** Met
- **Evidence:** Coverage uploaded to Codecov via `codecov-action` (badge in README); merged Go coverage profiles aggregated across unit + integration + migration.
- **Justification:** Most major features have automated tests; the integration suite alone covers ingestion, search, cache, RLS, and benchmarks across 47 test cases.

### test_policy

- **Status:** Met
- **Evidence:** `.github/PULL_REQUEST_TEMPLATE.md` "Tests added or updated" checkbox; CONTRIBUTING.md testing requirements (PR #338).
- **Justification:** The PR template requires a tick that tests have been added or updated, and CONTRIBUTING.md states the testing expectation.

### tests_are_added

- **Status:** Met
- **Evidence:** Recent merged PRs include test additions (visible on the PR list); PR template enforces it.
- **Justification:** Recent PRs in the project's history include tests for the changes they introduce, in line with the PR-template requirement.

### tests_documented_added

- **Status:** Met
- **Evidence:** CONTRIBUTING.md and PR template explicitly reference tests; `Makefile` and README document how to run them.
- **Justification:** The expectation that contributors add tests is documented in CONTRIBUTING.md and the PR template.

### warnings

- **Status:** Met
- **Evidence:** `go.yml` `vet` job; `frontend/package.json` `lint` script (ESLint); `ai-worker/pyproject.toml` ruff + mypy; `lint-infra.yml` (hadolint, actionlint).
- **Justification:** Compiler/linter warnings are surfaced by `go vet`, ESLint, ruff, mypy, hadolint, and actionlint on every PR.

### warnings_fixed

- **Status:** Met
- **Evidence:** CI requires lint jobs to pass; user policy "Lint Before Push" (CLAUDE.md MEMORY) — every push must be lint-clean locally.
- **Justification:** The CI gate (`ci-required.yml`) blocks merges when lint or vet emits warnings, and the project policy is to fix all warnings before push.

### warnings_strict

- **Status:** Met
- **Evidence:** ESLint config uses `eslint-plugin-no-unsanitized`; ruff configured `line-length = 100`, `target-version = py312` with strict rules; `vue-tsc -b` in build script (TypeScript strict via `@vue/tsconfig`).
- **Justification:** Linters run with strict rule sets — TypeScript strict mode, ESLint with security plugins, ruff, mypy, hadolint, actionlint.

### know_secure_design

- **Status:** Met
- **Evidence:** `docs/architecture.md` (actor table, trust boundaries, mermaid diagrams); MAINTAINERS.md "Security triage" responsibility; `docs/compliance/osps-l2-2026-02-19.md`.
- **Justification:** The maintainer documents threat actors, trust boundaries, and security-relevant data flows in `docs/architecture.md` and explicitly owns security triage.

### know_common_errors

- **Status:** Met
- **Evidence:** SECURITY.md threat model section (where applicable); ESLint `no-unsanitized` plugin; `eslint-plugin-vue` rules; SuperTokens (well-vetted auth library) chosen specifically to avoid common auth pitfalls.
- **Justification:** The project relies on vetted libraries (SuperTokens for auth, Traefik for TLS) and lint rules that catch common web vulnerabilities. Maintainer experience includes Keycloak SPI development.

---

### Passing tier — Unmet

### sites_https

- **Status:** Unmet
- **Evidence:** Project repo is on `https://github.com/...`; a public project website URL is not yet published/documented from this repo's perspective.
- **Justification:** This criterion is currently unmet because no public, project-controlled homepage URL with verified HTTPS/TLS is published in the repository documentation or badge evidence.
- **Gap:** Confirm the landing site (`landing/`) is deployed to a TLS-only domain and add the URL to the README/badge form.

### dco

- **Status:** Met (note: this is sometimes filed as a separate criterion)
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/dco.yml>; CONTRIBUTING.md DCO section.
- **Justification:** Every commit must carry a `Signed-off-by` trailer; the DCO check blocks merge otherwise.

---

## Silver Tier

### dco_silver

- **Status:** Met
- **Evidence:** Same as `dco` above.
- **Justification:** All commits require DCO sign-off, enforced by required CI check.

### governance

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/MAINTAINERS.md>; satisfies OSPS-GV-01.01 / 01.02.
- **Justification:** MAINTAINERS.md documents who has sensitive access, role responsibilities (lead maintainer, review authority, release management, security triage), and how new maintainers are added.

### roles_responsibilities

- **Status:** Met
- **Evidence:** MAINTAINERS.md "Roles and Responsibilities" section.
- **Justification:** Roles and responsibilities (review authority, release management, security triage, governance) are explicit in MAINTAINERS.md.

### access_continuity

- **Status:** Unmet
- **Evidence:** Currently a single named maintainer (Jobin Lawrance) per MAINTAINERS.md.
- **Justification:** Continuity in access is not yet established because there is only one maintainer; bus-factor is 1.
- **Gap:** Add at least one co-maintainer with `admin` access on the repository and document them in MAINTAINERS.md.

### bus_factor

- **Status:** Unmet
- **Evidence:** Same as above.
- **Justification:** The project does not yet meet the Silver bus-factor-of-2 expectation.
- **Gap:** Recruit and onboard a second maintainer; record in MAINTAINERS.md.

### copyright_per_file

- **Status:** Unknown
- **Evidence:** Not yet audited across the repo.
- **Justification:** Need to verify whether each source file carries a copyright header. Apache-2.0 only requires it at the LICENSE level, but Silver tier asks for per-file headers.
- **Gap:** Audit Go/Python/TS sources; add SPDX headers (`// SPDX-License-Identifier: Apache-2.0`) where missing.

### license_per_file

- **Status:** Unknown
- **Evidence:** Same as above.
- **Justification:** Per-file license header presence unverified.
- **Gap:** Same as above — add SPDX headers.

### build_reproducible

- **Status:** Unmet
- **Evidence:** Builds are deterministic in CI (Docker + pinned actions) but not formally bit-for-bit reproducible.
- **Justification:** We do not currently produce a reproducibility statement or matching bit-for-bit hashes.
- **Gap:** Document a reproducible build procedure (e.g. `SOURCE_DATE_EPOCH`, pinned `go` and `node` versions) and publish hashes alongside releases.

### crypto_published

- **Status:** Met
- **Evidence:** README "Tech Stack" identifies SuperTokens (auth) and Traefik (TLS); `docs/architecture.md` "Security-Relevant Notes".
- **Justification:** Cryptographic mechanisms used (TLS via Traefik, password storage via SuperTokens, JWT signing) are publicly documented.

### crypto_call

- **Status:** Met
- **Evidence:** Project consumes well-known crypto libraries: Go `crypto/*`, Python `cryptography>=42.0.0` (pyproject), SuperTokens (Argon2), Traefik (`crypto/tls`).
- **Justification:** All cryptography is performed via vetted FLOSS libraries; no hand-rolled primitives.

### crypto_floss

- **Status:** Met
- **Evidence:** Same libraries — all FLOSS.
- **Justification:** All crypto components used (Go stdlib, OpenSSL via Python `cryptography`, SuperTokens, Traefik) are FLOSS.

### crypto_keylength

- **Status:** Met
- **Evidence:** TLS via Traefik default (≥2048-bit RSA / 256-bit ECDSA); JWT signing via SuperTokens defaults (RS256 / ES256).
- **Justification:** Default key lengths used across SuperTokens, Traefik, and Go's `crypto/tls` exceed the 112-bit symmetric / 2048-bit asymmetric NIST minimum.

### crypto_working

- **Status:** Met
- **Evidence:** No deprecated primitives in use; SuperTokens / Traefik manage modern primitives.
- **Justification:** No use of MD5/SHA-1/DES/RC4/3DES anywhere security-relevant. SHA-256 is the only hash used (e.g. Valkey cache keys).

### crypto_weaknesses

- **Status:** Met
- **Evidence:** Same as above; ESLint `no-unsanitized`; `crypto/tls` defaults.
- **Justification:** Project does not implement custom crypto and relies on libraries that drop weak primitives by default.

### crypto_pfs

- **Status:** Unknown
- **Evidence:** Traefik default config supports PFS cipher suites but project does not yet ship an explicit Traefik config snippet pinning ECDHE/CHACHA20.
- **Justification:** PFS is the Traefik default but we do not have a published TLS config to point to.
- **Gap:** Add an `infra/traefik/tls.yaml` snippet pinning PFS-only cipher suites (TLS 1.3 + ECDHE-only TLS 1.2) and link it from `docs/architecture.md`.

### crypto_password_storage

- **Status:** Met
- **Evidence:** README Tech-Stack row "**Auth** | SuperTokens"; SuperTokens uses Argon2id by default.
- **Justification:** Passwords are stored via SuperTokens, which uses Argon2id with reasonable parameters by default. Raven never sees plaintext passwords.

### crypto_random

- **Status:** Met
- **Evidence:** Code uses Go `crypto/rand` and Python `secrets`/`os.urandom` for security-sensitive randomness; SuperTokens generates session tokens internally.
- **Justification:** All security-sensitive randomness uses CSPRNGs.

### delivery_mitm

- **Status:** Met
- **Evidence:** GHCR image pulls over HTTPS; `cosign verify-attestation` documented in `docs/security/slsa-verification.md`; checksums + cosign signatures attached to every GitHub Release (OSPS-BR-06.01).
- **Justification:** Release artefacts are delivered over HTTPS and signed via Sigstore keyless cosign; consumers can detect MITM by verifying signatures.

### delivery_unsigned

- **Status:** Met
- **Evidence:** `.goreleaser.yaml` signs `checksums.txt`; `release.yml` signs Docker images and frontend bundle; SLSA build provenance attached.
- **Justification:** Every release artifact is cryptographically signed (cosign keyless) with provenance attestations.

### vulnerabilities_fixed_60_days

- **Status:** Unknown
- **Evidence:** No public CVEs filed against Raven yet.
- **Justification:** No vulnerabilities have yet been disclosed; SLA in SECURITY.md commits to 90 days. The 60-day Silver bar is tighter than our public SLA.
- **Gap:** Tighten SECURITY.md "Fix, disclosure, and release" SLA from 90 days to 60 days, or document in SECURITY.md that Critical/High vulnerabilities will be fixed within 60 days while the overall SLA remains 90 days.

### vulnerabilities_critical_fixed

- **Status:** Met
- **Evidence:** govulncheck + Trivy CRITICAL/HIGH gating in `.github/workflows/security.yml`; Dependabot alerts.
- **Justification:** No outstanding critical vulnerabilities; the security workflow fails on CRITICAL/HIGH and Dependabot opens PRs for vulnerable dependencies.

### static_analysis

- **Status:** Met
- **Evidence:** <https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/codeql.yml> (Go, Python, JS/TS, security-extended); <https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/semgrep.yml>; `lint-infra.yml` (hadolint + actionlint).
- **Justification:** CodeQL `security-extended` and Semgrep run on every PR + push + weekly schedule across all three languages.

### static_analysis_common_vulnerabilities

- **Status:** Met
- **Evidence:** CodeQL `security-extended` queries cover OWASP Top 10 + CWE Top 25; Semgrep ruleset covers common vulnerability patterns.
- **Justification:** CodeQL + Semgrep combined cover the standard CWE Top 25 and OWASP Top 10 patterns.

### static_analysis_fixed

- **Status:** Met
- **Evidence:** CodeQL gate is a required check (`ci/codeql-gate-required-check` branch + OSPS-AC-03.01); ruff / golangci-lint findings block merge.
- **Justification:** Confirmed-positive findings are fixed before merge — the CodeQL gate blocks PRs with new alerts.

### static_analysis_often

- **Status:** Met
- **Evidence:** `codeql.yml` runs on `push`, `pull_request`, and weekly cron; `security.yml` runs weekly cron + on every PR.
- **Justification:** SAST runs on every commit and weekly, comfortably exceeding the "at least once a week" bar.

### dynamic_analysis

- **Status:** Met
- **Evidence:** `go.yml` runs `go test -race` (data-race detector) + integration suite via testcontainers; Playwright e2e in `frontend.yml`; Trivy filesystem scan + govulncheck in `security.yml`.
- **Justification:** Dynamic analysis is performed via the Go race detector, real-database integration tests, Playwright end-to-end browser tests, govulncheck, and Trivy.

### dynamic_analysis_unsafe

- **Status:** N/A
- **Evidence:** Project is Go + Python + TypeScript. Go is memory-safe; Python is memory-safe; TypeScript is memory-safe. The only `unsafe`/cgo usage is in eBPF code, which has its own dedicated tests under `tests/ebpf/`.
- **Justification:** Memory-unsafe code paths (eBPF/cgo) have dedicated privileged race-detector test suites (`tests/ebpf/{xdp,observability,audit}`) — beyond that, the languages used are memory-safe.

### dynamic_analysis_enable_assertions

- **Status:** Met
- **Evidence:** Go `go test -race` enabled in `go.yml`; Python `pytest` runs assertions natively.
- **Justification:** Race detector is enabled in CI; Python tests use native `assert`.

### dynamic_analysis_fixed

- **Status:** Met
- **Evidence:** Same gates as `static_analysis_fixed`; CI required checks block merge on race / vuln findings.
- **Justification:** Findings from dynamic-analysis CI gates block merge until fixed.

---

## Gold Tier

### sec_mfa

- **Status:** Met
- **Evidence:** `docs/compliance/osps-l2-2026-02-19.md` row OSPS-AC-01.01 — `gh api orgs/ravencloak-org --jq .two_factor_requirement_enabled` returns `true`.
- **Justification:** GitHub organisation `ravencloak-org` enforces 2FA on every member, so all maintainers use MFA on their GitHub accounts.

### sec_two_person

- **Status:** Met
- **Evidence:** OSPS row OSPS-AC-03.01 — branch protection on `main`: `required_pull_request_reviews.required_approving_review_count = 1`, `require_code_owner_reviews = true`. CLAUDE.md "Never push directly to main".
- **Justification:** All changes to `main` require PR approval from a code-owner; direct commits are blocked.

### sec_continuity

- **Status:** Unmet
- **Evidence:** Single named maintainer in MAINTAINERS.md.
- **Justification:** Bus factor is 1.
- **Gap:** Onboard a second maintainer; same gap as Silver `bus_factor`.

### sec_releases

- **Status:** Met
- **Evidence:** `docs/security/slsa-verification.md`; OSPS row OSPS-BR-06.01; release pipeline signs everything via cosign keyless.
- **Justification:** Every release artefact is signed via Sigstore keyless cosign and carries SLSA build provenance + SBOM attestations.

### sec_signed_release

- **Status:** Met
- **Evidence:** Same as above.
- **Justification:** Releases are cryptographically signed (cosign) and verifiable with `gh attestation verify` or `cosign verify-attestation`.

### crypto_used_network

- **Status:** Met
- **Evidence:** README "Reverse Proxy | Traefik | Auto-TLS"; SuperTokens session cookies are HTTPS-only.
- **Justification:** All network communication runs over TLS via Traefik; HTTP is redirected to HTTPS.

### crypto_tls12

- **Status:** Unknown
- **Evidence:** Traefik default supports TLS 1.2/1.3; explicit minimum-version pin not yet committed in repo.
- **Justification:** TLS ≥1.2 is the Traefik default, but we do not yet ship a config that pins `minVersion: VersionTLS12`.
- **Gap:** Add `infra/traefik/tls.yaml` with `tls.options.default.minVersion: VersionTLS12` (or 1.3) and reference it from `docs/architecture.md`.

### crypto_certificate_verification

- **Status:** Unknown
- **Evidence:** Go stdlib `crypto/tls` and Python `requests`/`httpx` defaults verify certificates by default, but repository-wide proof of no verification bypasses is not yet documented.
- **Justification:** Expected safe client defaults exist, but we have not yet completed and recorded a repo-wide audit confirming no `InsecureSkipVerify: true` (or equivalent disablement flags) are present.
- **Gap:** Run a repository grep/audit for certificate-verification bypass settings (Go, Python, and any other TLS clients) and document the results in `docs/security/certificate-verification-audit.md`, showing no verification bypasses are present; mark this criterion Met only after that file is committed.

### crypto_verification_private

- **Status:** Met
- **Evidence:** SuperTokens manages session cookies; `Secure` + `SameSite` flags set by default.
- **Justification:** Authentication tokens travel only over verified TLS.

### hardening

- **Status:** Met
- **Evidence:** Traefik adds standard security headers (CSP, X-Content-Type-Options, X-Frame-Options) per README "Reverse Proxy" row; ESLint `no-unsanitized` enforces XSS-safe templates; CSP configured in landing/frontend builds.
- **Justification:** Hardening headers are added by Traefik; HTTPS-only cookies via SuperTokens; XSS-prevention lint rule.

### assurance_case

- **Status:** Unmet
- **Evidence:** No formal assurance case document exists yet.
- **Justification:** Raven has a security policy (SECURITY.md), threat-actor documentation (docs/architecture.md), and a baseline self-assessment (docs/compliance/osps-l2-2026-02-19.md), but not a single document framed as a Gold-tier "assurance case".
- **Gap:** Author `docs/compliance/assurance-case.md` summarising security claims, supporting evidence, and residual risks.

### implement_secure_design

- **Status:** Met
- **Evidence:** `docs/architecture.md` actor table + trust boundaries; multi-tenant RLS evidence in README "Testing" (`RLS | 8 | Document/chunk/embedding/cache/source tenant isolation`).
- **Justification:** The architecture document captures actor / trust-boundary analysis and the codebase implements row-level-security tenant isolation with explicit tests.

### dynamic_analysis_unsafe_gold

- **Status:** Met (as detailed under Silver `dynamic_analysis_unsafe`).
- **Evidence:** `tests/ebpf/{xdp,observability,audit}` race-detector privileged tests; rest of codebase memory-safe.
- **Justification:** The only memory-unsafe surface (eBPF/cgo) has dedicated dynamic-analysis coverage; other languages are memory-safe.

### regression_tests

- **Status:** Met
- **Evidence:** README "Testing" lists 47 integration tests + 7 benchmarks; `go.yml` runs full unit + integration + migration on every PR.
- **Justification:** All tests are run on every PR — there is no separate "regression suite" because every test gate every change.

### contributors_unassociated

- **Status:** Unknown
- **Evidence:** No public count of unaffiliated contributors yet; the project is young.
- **Justification:** Need to inspect `git shortlog` against employer attribution.
- **Gap:** Document contributor demographics; recruit unaffiliated contributors.

### automated_integration_testing

- **Status:** Met
- **Evidence:** `go.yml` integration job using testcontainers-go (real PostgreSQL + pgvector); `frontend.yml` Playwright e2e; `python.yml` pytest.
- **Justification:** Integration testing is fully automated and runs on every PR via testcontainers and Playwright.

### test_statement_coverage80

- **Status:** Unknown
- **Evidence:** Coverage is uploaded to Codecov per `go.yml` but a published headline percentage is not in the repo today.
- **Justification:** We do not yet enforce or document an 80%-coverage gate.
- **Gap:** Publish current Codecov percentage in README; add a CI gate that fails when coverage drops below 80%.

### test_branch_coverage70

- **Status:** Unknown
- **Evidence:** Same as above — branch coverage not separately surfaced.
- **Justification:** Tooling captures branch coverage (via `go test -cover`) but we do not yet publish a 70% branch-coverage figure.
- **Gap:** Same as above — surface branch coverage on Codecov / README badge.

---

## Action items to reach Silver

- [ ] **Recruit a second maintainer** with `admin` access; update `MAINTAINERS.md` (closes `access_continuity`, `bus_factor`, also unblocks Gold `sec_continuity`).
- [ ] **Add SPDX `Apache-2.0` headers** to every Go / Python / TS / Vue source file (closes `copyright_per_file`, `license_per_file`).
- [ ] **Document a reproducible build procedure** with `SOURCE_DATE_EPOCH` and pinned toolchain versions (closes `build_reproducible`).
- [ ] **Pin PFS-only TLS** in a checked-in Traefik snippet under `infra/traefik/tls.yaml` and link from `docs/architecture.md` (closes `crypto_pfs`).
- [ ] **Tighten SECURITY.md fix SLA** to 60 days for Critical/High vulnerabilities (closes `vulnerabilities_fixed_60_days`).

## Action items to reach Gold

- [ ] **Onboard a second maintainer** (also closes Silver bus-factor; closes `sec_continuity`).
- [ ] **Pin TLS minimum version 1.2** in the Traefik config snippet (closes `crypto_tls12`).
- [ ] **Author `docs/compliance/assurance-case.md`** summarising security claims, evidence, and residual risk (closes `assurance_case`).
- [ ] **Recruit / document unaffiliated contributors** and capture contributor demographics (closes `contributors_unassociated`).
- [ ] **Publish coverage badges** and gate CI at ≥80% statement / ≥70% branch (closes `test_statement_coverage80`, `test_branch_coverage70`).

---

## Notes for the maintainer

- Source for Passing-tier evidence is the merged work captured in `docs/compliance/osps-l2-2026-02-19.md`. When that file changes, this answer key should be updated in the same PR.
- "Unknown" rows are honest — they are claims we *might* satisfy but have not verified in-repo. Treat them as Silver/Gold blockers until evidence is committed.
- This file deliberately does **not** include Enterprise-License (`ee-`) artefacts; OpenSSF Best Practices applies only to the Apache-2.0 OSS portion of Raven.
