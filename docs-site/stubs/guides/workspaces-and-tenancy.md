---
title: "Workspaces & Tenancy"
---

# Workspaces & Tenancy

A practical, task-oriented guide for operators managing **organisations**,
**workspaces**, **knowledge bases**, and **members** in Raven. Every example
below is an HTTP call against the public API (`/api/v1`) — the dashboard at
`/app` is a thin client over the same endpoints.

For the *why* behind the model — Postgres Row Level Security, the
`app.current_org_id` GUC, and the `tenant_isolation` policy — see
[Multi-Tenancy](/concepts/multi-tenancy). This page is the *how*.

## Quick reference

All endpoints are prefixed with `/api/v1` and require a bearer token unless
noted. `org_admin` always bypasses workspace role checks
(`internal/middleware/rbac.go`).

| Resource | Method & path | Required role | What it does |
|----------|---------------|---------------|--------------|
| Organisation | `POST /orgs` | _(none — onboarding)_ | Create a new org; the caller becomes its `org_admin`. |
| Organisation | `GET /orgs/:org_id` | any org member | Fetch an organisation. |
| Organisation | `PUT /orgs/:org_id` | `org_admin` | Update name / settings. |
| Organisation | `DELETE /orgs/:org_id` | `org_admin` | Hard-delete; cascades to all workspaces, KBs, documents. |
| Workspace | `POST /orgs/:org_id/workspaces` | _(none — onboarding)_ | Create a workspace inside an org. |
| Workspace | `GET /orgs/:org_id/workspaces` | any org member | List workspaces. |
| Workspace | `GET /orgs/:org_id/workspaces/:ws_id` | workspace `viewer`+ | Fetch one workspace. |
| Workspace | `PUT /orgs/:org_id/workspaces/:ws_id` | workspace `admin` | Update name / settings. |
| Workspace | `DELETE /orgs/:org_id/workspaces/:ws_id` | `org_admin` | Hard-delete; cascades to KBs and members. |
| Member | `POST /orgs/:org_id/workspaces/:ws_id/members` | workspace `admin` | Add a user to the workspace at a chosen role. |
| Member | `PUT /orgs/:org_id/workspaces/:ws_id/members/:user_id` | workspace `admin` | Change a member's role. |
| Member | `DELETE /orgs/:org_id/workspaces/:ws_id/members/:user_id` | workspace `admin` | Remove a member (callers cannot remove themselves). |
| Knowledge base | `POST .../workspaces/:ws_id/knowledge-bases` | _(none — onboarding)_ | Create a KB inside a workspace. |
| Knowledge base | `GET .../workspaces/:ws_id/knowledge-bases` | workspace `viewer`+ | List KBs in a workspace. |
| Knowledge base | `PUT .../knowledge-bases/:kb_id` | workspace `member` | Update KB name, description, settings, cache knobs. |
| Knowledge base | `DELETE .../knowledge-bases/:kb_id` | workspace `admin` | **Soft-delete** — sets `status = 'archived'`. |
| API key | `POST .../knowledge-bases/:kb_id/api-keys` | workspace `member` | Mint a scoped API key for the embedded widget. |
| API key | `DELETE .../knowledge-bases/:kb_id/api-keys/:key_id` | workspace `admin` | Revoke a key. |

Source: `cmd/api/main.go` (route table) and the `@Router` doc comments in
`internal/handler/`. The same paths appear in `contracts/openapi.yaml`.

## Create an organisation

The first user signing up creates the org; they receive `org_admin`
automatically (see the default-role assignment in
`internal/middleware/auth.go`). Subsequent users are created **inside** an
existing org and join through workspace membership — Raven does not allow a
user to "switch orgs".

**Dashboard.** Sign up via `/app/signup` → "Create organisation" — the
onboarding flow calls `POST /orgs`, then `POST /orgs/:org_id/workspaces`,
then `POST .../knowledge-bases` in sequence.

**API.**

```http
POST /api/v1/orgs
Authorization: Bearer <token>
Content-Type: application/json

{ "name": "Acme Inc." }
```

Response `201 Created` returns the `Organization` (`internal/model/org.go`):
`id`, `name`, `slug`, `status`, `settings`, `created_at`, `updated_at`.

The `slug` is derived server-side and is globally unique across the
deployment (the `organizations` table has `UNIQUE(slug)` —
`migrations/00003_organizations.sql`). `status` is one of `active`,
`suspended`, `deactivated` (the `org_status` ENUM in
`migrations/00001_extensions_and_types.sql`); set non-`active` values to
take an org offline without deleting it.

## Invite (add) members

Raven has **no email-invitation flow** today — membership is added directly
by user ID. The new user first signs in via Keycloak (which creates a row
in the `users` table — `migrations/00004_users.sql`); a workspace `admin`
then calls `POST /orgs/:org_id/workspaces/:ws_id/members` with the user's
UUID and the desired role.

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/members
Authorization: Bearer <admin-token>
Content-Type: application/json

{
  "user_id": "0193ffee-...",
  "role": "member"
}
```

`role` must be one of `owner`, `admin`, `member`, `viewer` — validated by
the `binding:"oneof=..."` tag on `AddWorkspaceMemberRequest`
(`internal/model/workspace.go`).

> **Email invitations are on the roadmap.** AWS SES is already wired up for
> transactional mail (notifications, password reset). A future
> `POST /invitations` endpoint will email a one-time accept link — track the
> milestone in the issue tracker rather than relying on this page.

## Manage roles

Two role systems coexist:

### Organisation roles

Stored in the JWT claim and resolved by `JWTMiddleware`. Today only
`org_admin` exists in code (the org creator gets it by default —
`internal/middleware/auth.go`). An `org_admin` can update/delete the org,
create/delete workspaces, and manage org-scoped resources (LLM providers,
routing rules, connectors, security rules). They **bypass every workspace
role check** (`RequireWorkspaceRole` short-circuits when
`org_role == "org_admin"`). Additional org-level roles will be added when
multi-admin orgs are needed.

### Workspace roles

Stored per-user-per-workspace in the `workspace_members` table
(`migrations/00006_workspace_members.sql`). The `role` column is the
`workspace_role` ENUM defined in
`migrations/00001_extensions_and_types.sql`:

```sql
CREATE TYPE workspace_role AS ENUM ('owner', 'admin', 'member', 'viewer');
```

Permissions are ranked — `RequireWorkspaceRole` admits any role at or above
the minimum (`internal/middleware/rbac.go`):

| Role | Rank | Can do (concrete) |
|------|------|-------------------|
| `viewer` | 0 | Read the workspace, list KBs, read KBs, run search and chat completions against KBs. |
| `member` | 1 | Everything `viewer` can, plus update KB settings (rename, description, settings, semantic-cache knobs) and mint API keys. |
| `admin` | 2 | Everything `member` can, plus update workspace settings, add/remove/change members, revoke API keys, archive (soft-delete) KBs. |
| `owner` | 3 | Reserved — currently identical to `admin` in middleware checks. Designed for future "single responsible owner" workflows (billing handover, ownership transfer). |

`org_admin` is **not** in the workspace ENUM — it lives at the org level
and silently outranks every workspace role.

### Change a role

```http
PUT /api/v1/orgs/{org_id}/workspaces/{ws_id}/members/{user_id}
Authorization: Bearer <admin-token>
Content-Type: application/json

{ "role": "admin" }
```

## Workspaces

Every workspace lives inside exactly one organisation (`workspaces.org_id`
is `NOT NULL REFERENCES organizations(id) ON DELETE CASCADE` —
`migrations/00005_workspaces.sql`). Slugs are unique **per org**, not
globally: `UNIQUE(org_id, slug)`.

```http
POST   /api/v1/orgs/{org_id}/workspaces           # { "name": "Marketing" }
GET    /api/v1/orgs/{org_id}/workspaces?offset=0&limit=20
PUT    /api/v1/orgs/{org_id}/workspaces/{ws_id}   # partial: name, settings
DELETE /api/v1/orgs/{org_id}/workspaces/{ws_id}   # hard delete — see below
```

The first workspace is created during onboarding without a role check; all
subsequent operations are gated by the table above. `settings` is a
free-form JSONB blob; reserve a stable dotted-key namespace per feature
(e.g. `notification.daily_digest`).

### Archive a workspace

Workspaces have **no soft-delete**: `DELETE` is a hard delete that cascades
through every nested resource. To "freeze" a workspace without losing data,
revoke its API keys, remove non-admin members, and leave it untouched. A
status column may be added later; track the schema rather than building
around today's behaviour.

## Knowledge bases

A KB is a scoped document store inside a workspace. It owns documents,
sources, chunks, embeddings, conversations, and any API keys that
authenticate the embedded widget.

### Create

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases
{ "name": "Public docs", "description": "Customer-facing help centre" }
```

### List

```http
GET /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases
```

Each entry includes a `doc_count` and a `status` of `active` or `archived`
(the `kb_status` ENUM, `migrations/00001_extensions_and_types.sql`).

### Update KB-level settings

`PUT .../knowledge-bases/:kb_id` accepts a partial payload (only non-nil
fields apply — `internal/model/kb.go::UpdateKBRequest`):

```json
{
  "name": "Public docs (EN)",
  "description": "...",
  "settings": {
    "default_llm_provider_id": "0193...",
    "embedding_model": "text-embedding-3-small",
    "retention_days": 365
  },
  "cache_enabled": true,
  "cache_similarity_threshold": 0.92
}
```

| Field | Where it lives | Notes |
|-------|----------------|-------|
| `name`, `description` | columns | Trimmed to 255 / TEXT. |
| `settings.default_llm_provider_id` | JSONB | UUID of an LLM provider created under `/orgs/:org_id/llm-providers`. See [LLM Providers](/guides/llm-providers). |
| `settings.embedding_model` | JSONB | Free-form string; the AI worker validates it against the configured provider catalogue. |
| `settings.retention_days` | JSONB | Soft retention hint for conversation logs. |
| `cache_enabled` | column | Toggles the semantic response cache (issue #256). |
| `cache_similarity_threshold` | column | Cosine floor for a cache HIT (0.80–0.99). |

### Soft-delete (archive) a KB

```http
DELETE /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}
```

This calls `KBHandler.Archive` (`internal/handler/kb.go`) which flips
`status` from `active` to `archived`. The row stays in the table, documents
remain on SeaweedFS, and chunk embeddings stay in `pgvector` — search and
chat against an archived KB return 404 but the data can be revived by an
operator with direct DB access. There is no API endpoint today to
un-archive a KB; do this via a SQL `UPDATE knowledge_bases SET status =
'active' WHERE id = ?` against a connection that holds the `raven_admin`
role (so the `admin_bypass` RLS policy applies).

## Cross-tenant operations

**There are none.** RLS guarantees that a request authenticated as a member
of org A cannot read or write rows belonging to org B, even if the
application code forgets a `WHERE` clause. The only legitimate cross-tenant
queries are:

- Migrations and operator scripts, which connect as the `raven_admin`
  Postgres role and short-circuit `tenant_isolation` via the `admin_bypass`
  policy.
- Trusted internal jobs (e.g. payment-webhook reconciliation) which set the
  `app.bypass_rls` GUC to `'true'` —
  `migrations/00032_billing_rls_and_payment_events.sql`.

Application traffic (the API binary) connects exclusively as `raven_app`
and has no path to the bypass. See [Multi-Tenancy
&rarr; Admin bypass](/concepts/multi-tenancy#admin-bypass).

## Soft delete vs hard delete

| Resource | `DELETE` semantics | Where the row goes |
|----------|-------------------|--------------------|
| Organisation | **Hard delete**. `ON DELETE CASCADE` removes every workspace, member, KB, document, conversation, API key, security rule, etc. Irreversible from the API. |
| Workspace | **Hard delete**. Cascades to KBs, members, KB-scoped API keys, KB-scoped conversations. |
| Workspace member | **Hard delete**. Row removed from `workspace_members`; the user account itself is untouched. |
| Knowledge base | **Soft delete (archive)**. Row stays, `status` → `archived`. Reads return 404. No tombstone TTL — archived KBs persist until an operator runs a cleanup migration. |
| API key | **Hard delete (revoke)**. Row stays but `status` → `revoked` (`api_key_status` ENUM); the secret hash is no longer accepted. |

There is no two-phase "tombstone then purge" pipeline today. If you need
recoverable deletion of an org or workspace, do it at the database level
before invoking the API.

## API keys per workspace

The embedded chat widget authenticates with a **knowledge-base-scoped** API
key — it cannot read or write across KB boundaries. Operators mint keys
through the dashboard or the API:

```http
POST /api/v1/orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/api-keys
{ "name": "marketing-site-prod" }
```

Response includes the plaintext secret **once** — store it somewhere safe;
Raven only keeps a hash (`api_keys.status` ENUM is `active` or `revoked` —
`migrations/00012_api_keys.sql`). For the full embed flow, see
[Embed the chat widget](/get-started/embed-the-chat-widget).

Revoke a key with `DELETE .../api-keys/:key_id`; the revoked row stays in
the table so audit queries can still attribute past traffic.

## Audit

Raven records two kinds of audit data:

1. **HTTP access logs** — every request, with `org_id`, `user_id`, route,
   status, and latency. These ship to OpenObserve (see [Observability
   stack](/guides/self-hosting/observability)).
2. **Security events** — `migrations/00022_security_rules.sql` creates
   `security_events`, an `org_id`-scoped table populated by the security
   middleware whenever a `security_rules` row fires (`event_type` is
   `blocked`, `throttled`, `alert`, or `suspicious`). This is **not** a
   generic CRUD audit log — it captures rule hits (IP/geo blocks, pattern
   matches, rate overrides), not every API call.

Query security events directly (RLS scopes results to your org):

```sql
SELECT created_at, event_type, ip_address, request_path, details
FROM security_events
WHERE created_at > NOW() - INTERVAL '24 hours'
ORDER BY created_at DESC;
```

A dedicated CRUD-audit table (write trail for org / workspace / member /
KB mutations) is not yet in the schema. Until it lands, reconstruct
mutation history from OpenObserve access logs filtered to write methods
(`POST`, `PUT`, `DELETE`).

## See also

- [Multi-Tenancy](/concepts/multi-tenancy) — the RLS model, the
  `app.current_org_id` GUC, and how the auth middleware drives them.
- [API Overview](/api/overview) — base URL, auth, and the OpenAPI spec.
- [Billing](/guides/billing) — per-org plans, usage limits, payment
  intents.
- [Embed the chat widget](/get-started/embed-the-chat-widget) — using a
  workspace-scoped API key from a browser.
