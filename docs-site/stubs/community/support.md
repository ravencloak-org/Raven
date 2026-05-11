---
title: "Support"
---

# Support

Raven is open source software, maintained as a public good by a single
maintainer ([Jobin Lawrance](/community/maintainers)). There is no support
desk and no paid tier — but there are clear, public channels for every kind
of question, bug, or vulnerability. This page is the router.

## Quick router

Pick the row that matches what you need.

| I need to&hellip; | Go to |
|---|---|
| Report a bug | [GitHub Issues](https://github.com/ravencloak-org/Raven/issues) |
| Ask a question or share an idea | [GitHub Issues](https://github.com/ravencloak-org/Raven/issues) with the `question` label (GitHub Discussions is not enabled on the repo) |
| Report a security vulnerability | [Private security advisory](https://github.com/ravencloak-org/Raven/security/advisories/new) — **do not** open a public issue. See the [security policy](/reference/security-policy). |
| Suggest a feature | [Open an issue](https://github.com/ravencloak-org/Raven/issues/new) describing the use case and proposed behaviour |
| Read the docs | You're here. Start at the [Quickstart](/get-started/installation). |
| Contribute code | [Contributing overview](/contributing/overview) |
| Find a maintainer | [Maintainers](/community/maintainers) |

## Issue templates

The repository does not currently use structured issue templates — issues
use GitHub's default form. When opening one, include:

- **What you expected to happen** and **what actually happened**.
- The **release tag or commit SHA** you're running.
- The **deployment shape** (Docker Compose, edge / Raspberry Pi, etc.) and
  any relevant environment notes.
- For bugs: a **minimal reproducer** and any logs scrubbed of secrets.
- For features: the **problem you're trying to solve**, not just the
  solution you have in mind.

Structured templates may be added later; the checklist above will keep
working in the meantime.

## Response expectations

Raven is currently maintained by one person, in the open, alongside other
work. Please calibrate expectations accordingly:

- **Non-security issues and questions:** best-effort, no guaranteed SLA.
  Triage and replies happen in batches. Well-formed reports with
  reproducers get answered faster — that's not a policy, it's just how
  attention works.
- **Pull requests:** see the [contributing guide](/contributing/overview)
  for the review and merge workflow. Small, focused PRs are merged quickly;
  large changes need a discussion in an issue first.
- **Security reports:** governed by the [security policy](/reference/security-policy).
  The committed SLA is:

  | Stage | Target |
  |---|---|
  | Initial acknowledgement | within **72 hours** |
  | Triage and severity assessment | within **7 days** |
  | Fix, disclosure, and release | within **90 days** (coordinated disclosure) |

  If you don't hear back inside the acknowledgement window, escalate by
  emailing `security@ravencloak.org` or `jobinlawrance@gmail.com` and
  referencing your original report.

## Commercial support

There is **no paid support tier** for Raven today. The project is run as
open source for the public good; everything that exists is in the open and
free to use under the [licence](https://github.com/ravencloak-org/Raven/blob/main/LICENSE).

If managed hosting, paid support, or commercial agreements become
available in the future, they will be announced here and on the project's
GitHub releases page. There is nothing to buy in the meantime, and no
private back-channel that gets you a faster response.

## Stay in touch

The honest answer is that Raven does not yet have a chat community, a
mailing list, or a social media presence. The public channels that exist
today are:

- [GitHub Releases](https://github.com/ravencloak-org/Raven/releases) —
  watch the repository (Releases only) to get notified when a new version
  ships.
- [GitHub Issues](https://github.com/ravencloak-org/Raven/issues) — the
  current home of all public conversation about the project.

Anything that looks like an official Raven Slack, Discord, or Telegram
right now is not us. If that changes, this page will be updated first.

## Related

- [Security policy](/reference/security-policy) — how to report
  vulnerabilities, supported versions, and disclosure timelines.
- [Code of conduct](/community/code-of-conduct) — the behavioural ground
  rules for every channel above.
- [Contributing overview](/contributing/overview) — how to land a change
  in the codebase.
- [Maintainers](/community/maintainers) — who is currently empowered to
  merge and release.
