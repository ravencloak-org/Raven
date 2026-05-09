---
title: "Architecture Decisions"
---

# Architecture Decisions

Raven does not maintain a parallel `docs/adr/` tree, and it does not use the
[MADR](https://adr.github.io/madr/) template. Instead, **specification documents
in [`docs/superpowers/specs/`](https://github.com/ravencloak-org/Raven/tree/main/docs/superpowers/specs)
are the ADR equivalent**. Each spec captures the rationale for a non-trivial
decision, lives alongside the code it describes, and is preserved as a
canonical archive once its implementation lands.

Specs are produced by the [`superpowers:brainstorming`](https://github.com/ravencloak-org/Raven/tree/main/docs/superpowers)
skill flow and are paired with an implementation plan in
[`docs/superpowers/plans/`](https://github.com/ravencloak-org/Raven/tree/main/docs/superpowers/plans).

## When to write a spec

| Change type                                          | Write a spec? | Why                                                     |
| ---------------------------------------------------- | ------------- | ------------------------------------------------------- |
| One-line bug fix                                     | No            | The PR description is the record.                       |
| Multi-file refactor with no behaviour change         | No            | The PR description is enough; link prior spec if any.   |
| New feature with non-obvious tradeoffs               | Yes           | Capture options considered and the chosen path.         |
| Cross-service contract change (API, gRPC, schema)    | Yes           | Multiple consumers need a shared, durable rationale.    |
| Replacing a vendor (e.g. Zitadel → SuperTokens)      | Yes           | Plus a paired failure post-mortem (see below).          |
| Process change (commits, PR flow, branch naming)     | No            | Edit `CLAUDE.md` / `CONTRIBUTING.md` instead.           |

If the spec cannot be finished in one sitting, ship it with `Status: DRAFT`
and iterate. A draft spec beats a decision with no record.

## Naming and location

```
docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md
docs/superpowers/plans/YYYY-MM-DD-<topic>-implementation.md
```

- `YYYY-MM-DD` is the date the spec was opened, not when it was approved.
- `<topic>` is kebab-case and matches the plan filename.
- Specs are immutable archives once their implementation merges. To revise
  a decision, write a **new** dated spec that links back via a
  `Supersedes:` header (see the SuperTokens example below).

## Structure

Specs follow a loose template drawn from the existing corpus. Not every
section is required — pick what fits — but the ordering should hold so
reviewers know where to look.

| Section                  | Purpose                                                        |
| ------------------------ | -------------------------------------------------------------- |
| **Status**               | `DRAFT`, `Approved`, `Superseded`, or `Final`.                 |
| **Date**                 | ISO date matching the filename.                                |
| **Author(s)**            | Humans accountable for the decision.                           |
| **Supersedes / Spec ID** | Link to any prior spec being replaced; optional stable ID.     |
| **Summary**              | 2–4 sentences. What is being decided and why now.              |
| **Goals**                | Bulleted, testable success criteria.                           |
| **Non-goals**            | Explicitly out of scope. Future work goes here.                |
| **Architecture**         | Diagrams, sequence flows, module layouts, decision tables.     |
| **IA / API / Data**      | Routes, gRPC methods, schemas, events, sidebar layout, etc.    |
| **Build & deploy**       | CI/CD, container topology, Cloudflare bindings, env vars.      |
| **Risks**                | Failure modes and the mitigation for each.                     |
| **Acceptance criteria**  | Concrete checks the implementation must pass.                  |
| **Tracking items**       | GitHub issues, milestones, follow-up tickets.                  |

The Status / Date / Author block is typically rendered as a small markdown
table at the top of the file (see `2026-05-09-docs-site-design.md`).

## Workflow

1. **Brainstorm** — invoke the `superpowers:brainstorming` skill to scope
   the problem, surface options, and produce the design spec.
2. **Plan** — invoke `superpowers:writing-plans` to convert the approved
   spec into a step-by-step implementation plan in
   `docs/superpowers/plans/`.
3. **Execute** — invoke `superpowers:subagent-driven-development` (or
   `superpowers:executing-plans` for solo work) to ship the plan.
4. **Archive** — once the implementation merges, the spec stays in place
   as the canonical record. Do not delete or rewrite it; supersede it.

See [Dev Setup](/contributing/dev-setup) for environment prerequisites and
[Release Process](/contributing/release-process) for how the implementation
lands in a release.

## Examples

Real specs currently in [`docs/superpowers/specs/`](https://github.com/ravencloak-org/Raven/tree/main/docs/superpowers/specs):

- **`2026-03-27-raven-platform-design-final.md`** — the v1.2 source of
  truth for the whole platform; every wiki page, milestone, and roadmap
  derives from it.
- **`2026-04-07-ebpf-edge-optimization-design.md`** — kernel-level
  observability and edge networking via shared eBPF programs.
- **`2026-04-13-integration-tests-design.md`** — Go integration test
  harness for the Raven core pipeline (test DB lifecycle, gRPC AI worker
  mock, fixtures).
- **`2026-04-13-zitadel-migration-design.md`** — original plan to replace
  Keycloak with Zitadel. Now superseded; see post-mortems below.
- **`2026-04-14-supertokens-migration-design.md`** — replaces Zitadel with
  SuperTokens; explicitly cites the production failures of the prior spec
  and switches to cookie-based sessions on PG18.
- **`2026-04-19-osps-l2-compliance-design.md`** — OpenSSF OSPS Baseline
  Level 2 compliance design.
- **`2026-05-06-raven-marketing-site-design.md`** — Tailwind Plus Salient
  rebuild of `raven.ravencloak.org`.
- **`2026-05-08-resilience-design.md`** — circuit breakers, retries, and
  deadlines across the Go API, gRPC AI worker, and Asynq handlers.
- **`2026-05-08-m11-phase1-design.md`** — Ollama-backed local AI worker,
  setup wizard, and installer for the M11 self-hosting path.
- **`2026-05-09-docs-site-design.md`** — the spec that produced **this
  documentation site** (VitePress at `docs.raven.ravencloak.org`,
  spec-fed OpenAPI reference, PageFind search).

Every implementation plan referenced from these specs lives at the matching
date in `docs/superpowers/plans/`.

## Failure post-mortems

When a decision is reversed, the new spec **must** explain why the previous
one failed. Do not silently delete or rewrite the original — keep it as
evidence and link to it from the replacement.

The canonical example: `2026-04-14-supertokens-migration-design.md` opens
with a `Supersedes:` header pointing at
`2026-04-13-zitadel-migration-design.md`, then names the four production
failures (PG18 incompatibility, opaque token behaviour, broken
`start-from-init` steps, forced MFA on passkey registration) before
proposing the replacement. That is the pattern to copy.

If a reversal is large enough to warrant a standalone narrative, add a
sibling document under `docs/superpowers/specs/` (e.g.
`<date>-<topic>-postmortem.md`) and cross-link both ways.

## Do not

- **Do not** introduce a parallel `docs/adr/` directory. The spec file
  *is* the ADR.
- **Do not** copy MADR, Nygard, or Y-statement templates from elsewhere.
  The structure above is what reviewers expect.
- **Do not** rewrite a merged spec in place. Supersede it with a new
  dated file and link both directions.
- **Do not** put process or workflow rules in a spec. Those belong in
  `CLAUDE.md` or `CONTRIBUTING.md`.
- **Do not** open a spec for a one-line bug fix or a pure refactor — the
  PR description is the audit trail.
