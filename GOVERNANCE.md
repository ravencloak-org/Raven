# Governance

This document describes how the Raven project is governed: who decides what,
how decisions are made, how maintainers are added and removed, and how
releases and security incidents are handled. It complements
[`CONTRIBUTING.md`](./CONTRIBUTING.md), [`SECURITY.md`](./SECURITY.md), and
[`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md), and supersedes none of them.

## Mission

Raven is an open-source, self-hostable, multi-tenant knowledge base platform
with AI-powered chat, voice, and WhatsApp channels. The project's goal is
to give teams a production-grade retrieval-augmented generation platform
without vendor lock-in, runnable on a single Docker Compose host or on
edge hardware such as a Raspberry Pi, with data ownership end-to-end.

## Roles

Four roles operate within the project. Anyone may move between roles as
their participation grows.

| Role | Responsibilities | How to become one |
|---|---|---|
| **User** | Runs Raven, reports bugs, files feature requests, asks questions. | Use the project. |
| **Contributor** | Submits pull requests, helps in discussions, improves docs. | Open a PR; sign-off via DCO. |
| **Committer** | Reviews and approves pull requests within an area of expertise. Cannot merge to `main`. | Sustained, high-quality contribution recognised by an existing maintainer. |
| **Maintainer** | Final review and merge authority. Sets technical direction. Listed in [`MAINTAINERS.md`](./MAINTAINERS.md). | Nominated by an existing maintainer; approved per the rules below. |

The current list of maintainers and their areas of ownership is published in
[`MAINTAINERS.md`](./MAINTAINERS.md). The lead maintainer at the time of
writing is Jobin Lawrance (`@jobinlawrance`).

## Decision making

Decisions are made by lazy consensus where possible and explicit maintainer
approval where required.

| Change type | Required approvals |
|---|---|
| Typo fixes, dependency bumps, low-risk refactors | One maintainer review. Lazy consensus: 24 hours with no objection. |
| Feature additions, behaviour changes | One maintainer review and an explicit "approve". |
| Architectural change, public API change, breaking change | Two maintainer reviews. Discussion documented in an issue or ADR before merge. |
| Security-impacting change | Two maintainer reviews, one of whom is the security lead. Disclosure handled per [`SECURITY.md`](./SECURITY.md). |
| Governance change (this file, `MAINTAINERS.md`, license, code of conduct) | Two maintainer reviews and a 14-day public comment window on the PR. |

If maintainers disagree and consensus cannot be reached within seven days,
the lead maintainer breaks the tie. The decision and its rationale are
recorded in the originating issue or PR.

## Adding and removing maintainers

A new maintainer is nominated by an existing maintainer in a private channel
and confirmed by the lead maintainer. The nomination is then announced in a
public issue for transparency. New maintainers receive merge access and are
added to [`MAINTAINERS.md`](./MAINTAINERS.md) in the same PR.

A maintainer may step down at any time by opening a PR that removes their
entry from [`MAINTAINERS.md`](./MAINTAINERS.md). Involuntary removal
requires two maintainer approvals and a 30-day public notice on a tracking
issue, during which the affected maintainer may respond. A removal that is
the result of a code-of-conduct or security finding follows the process in
the relevant document and may be expedited.

A maintainer who has not contributed for 12 consecutive months is
automatically considered emeritus. Emeritus maintainers retain credit but
relinquish merge authority; they may return to active status by opening a
PR.

## Releases

Releases are cut from `main` and tagged via the
[`release.yml`](./.github/workflows/release.yml) workflow. The project ships
Go binaries, Docker images, and (where applicable) the frontend bundle. All
release artefacts are produced with SLSA Level 3 build provenance per
[`docs/security/slsa-verification.md`](./docs/security/slsa-verification.md)
and target the OpenSSF Baseline (`docs/compliance/`) self-assessment.

`main` is always release-shaped: every PR squash-merges, must pass the
required-status checks defined in
[`.github/workflows/ci-required.yml`](./.github/workflows/ci-required.yml),
and may not bypass hooks (`--no-verify` is not permitted). Release versions
follow semantic versioning. Pre-1.0, minor versions may contain breaking
changes; these are called out in the release notes.

## Security escalation

Suspected vulnerabilities, leaked credentials, and incidents follow the
private channels and SLAs documented in [`SECURITY.md`](./SECURITY.md).
Public disclosure happens through the GitHub Security Advisory and the
release notes of the fixed version. The security lead may merge a
security-only fix to `main` with a single maintainer review when speed is
necessary; the second review is performed post-merge and recorded in the
advisory.

## Code of conduct

All participants in Raven spaces are bound by the
[Code of Conduct](./CODE_OF_CONDUCT.md). Reports are handled by the
maintainer team via the channels documented there.

## Amendments

This document may be amended by a pull request approved per the "Governance
change" row in the [Decision making](#decision-making) table.

---

Last updated: 2026-05-08.
