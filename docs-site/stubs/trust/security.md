---
title: "Security"
---

# Security

Raven's security posture is defence in depth, not a single control. The build
pipeline produces signed artifacts with [SLSA Level 3 provenance](/trust/slsa-level-3);
every pull request and weekly cron run scan source for vulnerabilities
([CodeQL](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/codeql.yml)
+ [Semgrep](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/semgrep.yml))
and for committed secrets
([Gitleaks](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/gitleaks.yml));
dependencies are tracked by [Dependabot](https://github.com/ravencloak-org/Raven/blob/main/.github/dependabot.yml)
and [govulncheck](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/security.yml);
the project's process compliance is published as a public
[OSPS-L2 self-assessment](/trust/openssf-baseline); and the disclosure path
is documented under [RFC 9116](https://www.rfc-editor.org/rfc/rfc9116) at
`.well-known/security.txt`. Each pillar links to its canonical source below.

## Pillars

| Pillar | What it covers | Canonical link |
| --- | --- | --- |
| Build provenance | Container images and release binaries carry SLSA L3 attestations, signed via Sigstore keyless OIDC. | [/trust/slsa-level-3](/trust/slsa-level-3) |
| Process compliance | OSPS Baseline 2026-02-19, Level 2 — every control mapped to evidence (PR, workflow, file). | [/trust/openssf-baseline](/trust/openssf-baseline) |
| Self-assessment / questionnaire | OpenSSF Best Practices Badge (project [12590](https://bestpractices.dev/projects/12590)), full answer key reproduced inline. | [/trust/openssf-best-practices](/trust/openssf-best-practices) |
| Pre-merge SAST | CodeQL with `security-extended` + `security-and-quality` query suites for Go, Python, and JS/TS, plus Semgrep OSS with `p/ci`, `p/security-audit`, `p/owasp-top-ten`, and `p/secrets` rulepacks. | [codeql.yml](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/codeql.yml) · [semgrep.yml](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/semgrep.yml) |
| Pre-merge secrets | Gitleaks blocks merges that introduce credentials; runs on PR, push, and a weekly cron. | [gitleaks.yml](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/gitleaks.yml) |
| Dependency security | Dependabot for `gomod`, `pip`, `npm`, `docker`, and `github-actions`; Trivy + govulncheck in CI; OpenSSF Scorecard publishes the `Vulnerabilities` check weekly. | [dependabot.yml](https://github.com/ravencloak-org/Raven/blob/main/.github/dependabot.yml) · [security.yml](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/security.yml) · [scorecard.yml](https://github.com/ravencloak-org/Raven/blob/main/.github/workflows/scorecard.yml) |
| Vulnerability disclosure | Coordinated disclosure under a 72h-ack / 7d-triage / 90d-fix SLA. | [/reference/security-policy](/reference/security-policy) |
| Machine-readable disclosure path | [RFC 9116](https://www.rfc-editor.org/rfc/rfc9116) `security.txt` — also served from the production deploy. | [.well-known/security.txt](https://github.com/ravencloak-org/Raven/blob/main/.well-known/security.txt) |

## Reporting a vulnerability

If you believe you have found a security issue in Raven, please report it
privately. Two channels are accepted:

- **GitHub Security Advisories** (preferred): open a private advisory at
  <https://github.com/ravencloak-org/Raven/security/advisories/new>.
- **Email**: `security@ravencloak.org`. If that alias is unreachable, fall
  back to `jobinlawrance@gmail.com` and reference the original report.

You will receive an acknowledgement within **72 hours**, a triage and severity
assessment within **7 days**, and a fix coordinated within **90 days** of the
initial report. Do not file a public GitHub issue, post on social media, or
otherwise disclose the issue until the advisory ships.

[Read the full policy →](/reference/security-policy)

## Bug bounty

Raven does not run a paid bug-bounty programme today. The security policy's
**Safe Harbor** clause applies: good-faith research that complies with the
policy is treated as authorised, will not be pursued legally, and will be
acknowledged in the resulting GitHub Security Advisory unless the reporter
opts out. See the [Safe Harbor section](/reference/security-policy#safe-harbor)
of the policy for the full terms.

## Cryptographic verification

Release binaries (`checksums.txt`, frontend bundle), Docker images, and their
attestations are signed with [Sigstore Cosign](https://docs.sigstore.dev/cosign/verifying/verify/)
using GitHub's OIDC identity — no long-lived signing keys. Step-by-step
recipes for verifying provenance with `gh attestation verify`, `cosign
verify-attestation`, and `slsa-verifier`, plus the production guidance on
**always pinning by digest**, live at:

- [/trust/slsa-level-3](/trust/slsa-level-3) — verification recipes
- [/trust/openssf-baseline#build--release](/trust/openssf-baseline#build--release) — what we sign and where the certificate identity is anchored

## Audit findings and pentests

Raven has not commissioned a formal third-party security audit or penetration
test to date. The project is single-maintainer open source pre-1.0 — a paid
audit is on the post-1.0 milestone list rather than something we can claim
retroactively. The compensating controls today are: automated SAST and
dependency scanning on every change, a public OSPS-L2 self-assessment, SLSA L3
build provenance, and a working coordinated-disclosure channel. When an audit
or pentest is commissioned, the findings (or a redacted summary) will be
linked from this page.

## Compliance

The OSS portion of the repository (everything under
[`LICENSE`](https://github.com/ravencloak-org/Raven/blob/main/LICENSE); the
Enterprise portion under `ee-LICENSE` is out of scope) targets **OSPS Baseline
2026-02-19 at Level 2**. The full control-by-control mapping is at
[/trust/openssf-baseline](/trust/openssf-baseline).

Privacy, GDPR, and India DPDP material lives under `/trust/privacy/`. That
page is held back behind a DRAFT counsel-review banner and is scheduled for
publication once counsel review completes; the supporting evidence (DPA
template, sub-processor list, retention schedule) is being prepared in
parallel.

## See also

- [Security Policy](/reference/security-policy) — the full disclosure policy mirrored from `SECURITY.md`
- [OpenSSF Baseline](/trust/openssf-baseline) — OSPS-L2 self-assessment evidence log
- [OpenSSF Best Practices](/trust/openssf-best-practices) — bestpractices.dev answer key
- [SLSA Level 3](/trust/slsa-level-3) — provenance verification recipes
- [Maintainers](/community/maintainers) — security triage owner and escalation path
