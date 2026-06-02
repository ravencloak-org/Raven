# 0009 — Single org-wide LLM Provider default

**Status:** Accepted
**Date:** 2026-06-02

## Decision

An Org has **exactly one default LLM Provider** at a time, represented by the boolean `is_default` on the provider row. Every chat, embedding, and RAG call resolves through this single default unless the caller passes an explicit provider slug.

We do **not** split the default by purpose (no separate "chat default" and "embed default"). We do **not** allow per-User overrides of the Org default.

## Why

- The schema and call path already assume a single default. `llm_provider_configs.is_default boolean` is the canonical source; `useLlmProvidersStore.setDefaultLlmProvider` and the `PUT /llm-providers/:id/default` route both treat it as singular. Recent fixes (`fix(chat): resolve org's default LLM provider instead of hardcoding 'anthropic'`, commit `9c4e0d58`) closed the last gap where call paths bypassed it.
- The recurring chat-vs-embed-leak bug — where a chat model accidentally got passed to the embedding endpoint — was a **model-picker** problem, not a **provider-scope** problem. A single provider config holds both a default chat model and a default embedding model in its `config` JSON; the call site picks which one to use. Splitting the default by purpose would not have prevented that bug.
- Per-User override breaks the "Org owns its config" mental model already established in CONTEXT.md and reinforced by ADR-0005 (Org is the Marketplace publisher). Two members of the same Org should see the same chat behavior; otherwise debugging "why did this answer differ?" becomes an opaque per-User config diff.
- The cost of adding (b) — per-purpose split — later is small and additive: introduce a `default_for ENUM('all','chat','embed')` column, keep `default_for='all'` as the singleton invariant, allow rows with `default_for='chat'` or `'embed'` to coexist. No data loss, no breaking change for current call sites.

## Trade-offs accepted

- **Mixed-vendor stacks need creative model wiring.** A common real-world stack is "chat via Anthropic, embed via local Ollama." Under this decision, the Org's single default provider is one of them, and the *other* call path must pass an explicit provider slug (or look up the right provider by model name). We accept this constraint because the alternative — two defaults — multiplies invariants (what if both are missing? what if both point at the same row? how does the UI surface both?) for a stack shape that is currently exercised by one of our own users (the demo box on Ollama).
- **Switching the default is a tenant-wide observable.** Org member A flips the default; member B's next chat hits a different vendor. We accept this because (1) all members already share the same `knowledge_bases` and embeddings, so the vendor change does not affect retrieval semantics, only the LLM that synthesizes the answer; (2) the alternative is per-User defaults, which we rejected above.
- **Auto-defaulting on first create stays.** A fresh Org with zero providers cannot chat (the chat path 500s with "No active 'X' provider config found"). The list page already sets `is_default=true` on the first row at create time (`feat(llm): guide users to vendor console + auto-default first provider`, commit `9f8ad6b4`). This decision keeps that behavior.

## Consequences

- The list page UI gains a single "Default" pill per Org (not multiple pills for chat/embed).
- The "Make default" action on a non-default card flips exactly one boolean — atomic, no per-purpose menu.
- The `setDefault` endpoint signature stays `PUT /llm-providers/:id/default` with no body. No `?for=chat` query param.
- If we ever ship per-purpose defaults, the migration is purely additive: add `default_for ENUM` with default `'all'`, keep this row as the only `'all'` row, and add new rows for the new purposes. This ADR will need a sibling.

## Alternatives considered

- **(b) Split by purpose: chat-default + embed-default.** Two flags on the provider row (or one `default_for` enum). Rejected: invariants explode (what if both `is_chat_default` rows? what if neither?), and the bug it was supposed to fix is a model-picker issue.
- **(c) Per-User override.** Each User picks their preferred provider for their session; Org default is the fallback. Rejected: breaks the "Org owns its config" mental model and creates per-User opaque divergence.
