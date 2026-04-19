# Maintainers

This document lists the people with sensitive access to the Raven project,
their roles, and how additional maintainers are added. It satisfies the
OpenSSF Baseline L2 controls **OSPS-GV-01.01** (list of members with sensitive
access) and **OSPS-GV-01.02** (roles and responsibilities).

## Current Maintainers

| Name            | GitHub           | Role            | Areas of Ownership | Access Level |
| --------------- | ---------------- | --------------- | ------------------ | ------------ |
| Jobin Lawrance  | [@jobinlawrance](https://github.com/jobinlawrance) | Lead maintainer | All areas          | Admin        |

"Sensitive access" here means any of the following on the
[ravencloak-org/Raven](https://github.com/ravencloak-org/Raven) repository or
the surrounding `ravencloak-org` GitHub organisation:

- Admin on the repository (write, merge, and settings changes, including
  branch protection rules)
- Permission to push directly to protected branches (disabled in policy, but
  access level still implies the capability)
- Permission to publish releases or manage release artifacts
- Permission to manage GitHub Actions secrets and environments
- Permission to manage organisation-level settings, teams, and membership

## Roles and Responsibilities

### Lead maintainer

The lead maintainer is accountable for the overall direction and health of the
project. Concretely, that covers:

- **Review authority:** may approve and merge pull requests. The lead
  maintainer is listed as the default `CODEOWNERS` for the repository until
  further owners are added.
- **Release management:** cuts SemVer-tagged releases from `main`, signs
  release artifacts via the project's release pipeline, and publishes release
  notes and security advisories.
- **Security triage:** is the primary recipient of reports filed under
  [`SECURITY.md`](./SECURITY.md), owns the 72-hour acknowledgement SLA, drafts
  GitHub Security Advisories, and coordinates CVE assignment.
- **Project direction:** sets the roadmap, owns milestone planning, and makes
  the final call on scope, technology choices, and breaking changes.
- **Governance:** maintains this document, `CONTRIBUTING.md`,
  `SECURITY.md`, and other governance files; enforces branch protection and
  access policies on the repository and the `ravencloak-org` organisation.

### Contributors

Contributors submit issues, pull requests, and reviews but do not hold write
access to the repository. Contribution requirements are documented in
[`CONTRIBUTING.md`](./CONTRIBUTING.md).

## Becoming a Maintainer

Raven currently has a single maintainer. Additional maintainers will be added
as the project grows and a track record of contribution is established. There
is no fixed quota and no guaranteed path; the criteria below are the floor
rather than an exhaustive checklist.

### Criteria

A candidate for maintainer is expected to demonstrate, over time:

- **Sustained contribution:** a meaningful number of merged pull requests
  across more than one area of the codebase (for example, API, AI worker,
  frontend, docs, CI). A rough reference point is 10+ non-trivial contributions
  over 3+ months, but quality, review participation, and subject-matter depth
  weigh more than raw count.
- **Good judgement in review:** thoughtful and technically rigorous code
  review on others' pull requests.
- **Alignment with project conventions:** familiarity with and adherence to
  the conventions in [`CONTRIBUTING.md`](./CONTRIBUTING.md), the commit and
  branch style documented in the repository, and the security posture in
  [`SECURITY.md`](./SECURITY.md).
- **Trustworthiness around sensitive access:** demonstrated care with
  secrets, credentials, dependencies, and the release pipeline.

### Process

1. **Nomination.** An existing maintainer nominates the candidate in an issue
   on the repository, summarising their contributions and proposed area of
   ownership.
2. **Consensus.** All existing maintainers must agree. While there is only one
   maintainer, the decision rests with that maintainer; once additional
   maintainers are added, nominations require consensus from all of them.
3. **Access grant.** The candidate enables two-factor authentication on their
   GitHub account, their access level is raised on the repository and the
   organisation, and this document is updated in the same pull request that
   records the addition.

### Stepping down

Maintainers may step down at any time by opening a pull request that updates
this document. Inactive maintainers (no reviews, PRs, or security-triage
activity for 6 consecutive months) may be moved to emeritus status by the
remaining maintainers, with their access reduced accordingly.

---

Last updated: 2026-04-19.
