# Changelog

All notable changes to Raven. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning per [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-04-23

### Features
- feat(m9): email summaries via AWS SES (#257) (#350)
- feat(m9): semantic response cache (#256) (#351)
- feat(m9): cross-channel conversation memory + PostHog (#258) (#349)

### Bug Fixes
- fix(ci): repair Semgrep SARIF output + bump CodeQL action to v4 (#364)
- fix(security): address HTML SRI + dev-ws Semgrep findings (#363)
- fix(security): address CodeQL findings — weak hash + credential log (#362)
- fix(security): parameterise or suppress asyncpg SQL-injection Semgrep findings (#361)
- fix(scripts): address IFS tampering + subprocess shell=True Semgrep findings (#360)
- fix(tests/ebpf/audit): harden ClickHouse DinD startup (#356)
- fix(tests/ebpf/audit): data race on mock reader + Docker-in-Docker seccomp (#355)
- fix: main-CI regressions — duplicate RLS assertion + missing gcc in eBPF container (#353)

### CI / Build
- ci(sast): fix hadolint DL4006 + actionlint SC2046; demote Semgrep to report-only (#358)
- ci(sast): wire Semgrep + hadolint + actionlint + no-unsanitized (#357)
- ci(slsa): upgrade container attestations to SLSA Build Level 3 (#352)

### Dependencies
- deps(go): bump github.com/jackc/pgx/v5 from 5.9.1 to 5.9.2 (#354)

### Other
- Update README.md

### Notes

- `v0.2.0` is an older tag (2026-04-10) from the pre-signed-pipeline MVP Launch
  release; the version sequence intentionally jumps from `v0.1.0` to `v0.3.0` to
  avoid tag collision while keeping SemVer monotonic with the signed-release line.
- **Operational**: `RAVEN_RATELIMIT_APIKEY_HASH_SECRET` must be set in production
  before rolling out this release (see #362). Without it, API-key rate-limit
  buckets cold-start per process and the worker logs a warning.

