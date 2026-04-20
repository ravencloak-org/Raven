# Security Policy

Raven takes the security of its users, operators, and contributors seriously.
This document describes how to report vulnerabilities, what response you can
expect, and the scope of this policy.

## Supported Versions

Security fixes are provided for the following versions:

| Version                | Supported          |
| ---------------------- | ------------------ |
| `main` branch (HEAD)   | Yes                |
| Latest tagged release  | Yes                |
| All other versions     | No                 |

Once multiple release lines exist, this table will be expanded with explicit
version ranges and end-of-life dates.

## Reporting a Vulnerability

Please **do not** open public GitHub issues, pull requests, or discussions for
suspected vulnerabilities. Use one of the private channels below.

### Primary: GitHub Security Advisories (private reporting)

Report through GitHub's private vulnerability reporting:

<https://github.com/ravencloak-org/Raven/security/advisories/new>

This is the preferred channel because it keeps the report, discussion, patch,
and CVE assignment in a single place visible only to maintainers and invited
collaborators.

### Fallback: Email

If you cannot use GitHub Security Advisories, email:

- `security@ravencloak.org` (preferred)
- `jobinlawrance@gmail.com` (temporary until `security@ravencloak.org` is
  provisioned)

Encrypt sensitive details where possible. A PGP key will be published here once
the `security@` alias is live.

### What to include

To help us triage quickly, please include as much of the following as you can:

- Affected component (API, AI worker, frontend, deployment manifests, etc.)
- Affected versions or commit SHAs
- A clear description of the issue and its impact
- Reproduction steps, proof-of-concept, or exploit code
- Any suggested mitigation or fix
- Whether you would like public credit, and under what name

## Response SLA

| Stage                          | Target                  |
| ------------------------------ | ----------------------- |
| Initial acknowledgement        | Within **72 hours**     |
| Triage and severity assessment | Within **7 days**       |
| Fix, disclosure, and release   | Within **90 days**      |

The 90-day window is the coordinated disclosure target. If a fix requires more
time (for example, because the root cause spans an upstream dependency), we
will agree an extended timeline with the reporter in writing.

If we do not respond within the acknowledgement window, please escalate by
emailing `jobinlawrance@gmail.com` directly and referencing your original
report.

## Disclosure Process

1. Maintainers confirm the report and assess severity (CVSS where applicable).
2. A fix is developed in a private fork or a temporary private branch.
3. A GitHub Security Advisory (GHSA) is drafted in the repository.
4. A CVE is requested through GitHub's CNA where the issue qualifies.
5. A patched release is prepared and the advisory is published at release time.
6. The reporter is credited in the advisory unless they opt out.

Public disclosure happens via the published GitHub Security Advisory, the CVE
record, and the release notes of the fixed version.

## Scope

### In scope

All code, configuration, and documentation released under the top-level
`LICENSE` file, including:

- Go API (`cmd/`, `internal/`, `pkg/`)
- Python AI worker (`ai-worker/`)
- Frontend (`frontend/`)
- Database migrations (`migrations/`)
- Deployment manifests (`deploy/`, `docker-compose*.yml`, `Dockerfile*`)
- Public documentation (`docs/`, `README.md`, `DEVELOPMENT.md`,
  `CONTRIBUTING.md`, `SECURITY.md`, `MAINTAINERS.md`)
- Build and release artifacts produced for tagged releases

### Out of scope

- Enterprise (EE) content released under `ee-LICENSE` (files such as
  `ee-LICENSE`, `ee-README.md`, and any future `ee-*` content). EE content has
  its own, separate handling process.
- Third-party services, infrastructure, or dependencies not authored in this
  repository. Report those to the corresponding upstream project.
- Social engineering, physical attacks, and denial-of-service attacks against
  infrastructure you do not own.
- Findings that require a non-default, unsupported, or end-of-life
  configuration.

## Safe Harbor

We support good-faith security research. If you make a good-faith effort to
comply with this policy during your research, we will:

- Consider your research authorised with respect to this project.
- Not pursue or support any legal action against you related to your research.
- Work with you to understand and resolve the issue promptly.

Good faith means, at minimum:

- You avoid privacy violations, data destruction, and degradation of service
  for other users.
- You only interact with test accounts you own or accounts for which you have
  explicit permission from the account holder.
- You give us reasonable time to fix the issue before any public disclosure.
- You do not exploit the issue beyond what is necessary to demonstrate it.

This safe harbor applies only to the OSS components listed under **In scope**.
It does not grant permission to attack third-party services, and it cannot
waive obligations you may have to third parties.

## Maintainers and Escalation

The list of maintainers authorised to receive and triage security reports is
published in [`MAINTAINERS.md`](./MAINTAINERS.md). Reports sent through the
channels above reach those maintainers directly.

---

Last updated: 2026-04-19.
