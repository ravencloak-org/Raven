---
title: "Multi-Tenancy"
---

# Multi-Tenancy

Raven is a multi-tenant platform: a single deployment serves many independent
**organisations**, each of which owns one or more **workspaces** containing
one or more **knowledge bases**. Tenant isolation is enforced at the
**database** layer using PostgreSQL Row Level Security (RLS), not in
application code. Every tenant-scoped table carries an `org_id` column, every
request runs inside a transaction with `app.current_org_id` set to the
caller's organisation, and a `tenant_isolation` policy on each table
restricts every `SELECT`, `INSERT`, `UPDATE`, and `DELETE` to rows whose
`org_id` matches. A forgotten `WHERE org_id = ?` clause in repository code
therefore cannot leak data — the database itself refuses to return it.

## Hierarchy

The tenant tree has four levels. Every level below `Organization` carries a
denormalised `org_id` foreign key so RLS can be enforced uniformly without
relying on joins.

| Level | Go type | Key fields | Migration |
|-------|---------|------------|-----------|
| Organisation | `model.Organization` | `ID`, `Name`, `Slug`, `Status` | `migrations/00003_organizations.sql` |
| Workspace | `model.Workspace` | `ID`, `OrgID`, `Name`, `Slug` | `migrations/00005_workspaces.sql` |
| Knowledge base | `model.KnowledgeBase` | `ID`, `OrgID`, `WorkspaceID`, `Name`, `Slug`, `Status` | `migrations/00007_knowledge_bases.sql` |
| Document | `model.Document` | `ID`, `OrgID`, `KnowledgeBaseID`, `FileName`, `ProcessingStatus` | `migrations/00008_documents.sql` |
| Conversation session | `model.ConversationSession` | `ID`, `OrgID`, `KBID`, `UserID`, `Channel` | `migrations/00037_conversation_sessions.sql` |

The `organizations` table is the tenant root and is intentionally **not**
RLS-protected — it *is* the tenant boundary. See the comment block at the
top of `migrations/00015_rls_policies.sql`.

## Row Level Security

Postgres [Row Level Security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
attaches a per-row predicate to every query against a table. Once a table
has `ENABLE ROW LEVEL SECURITY` set and at least one `CREATE POLICY` defined,
non-superuser connections only see rows that satisfy the policy's `USING`
expression and may only insert / update rows that satisfy `WITH CHECK`.

Raven enables RLS on every tenant-scoped table at the moment that table is
created. `migrations/00015_rls_policies.sql` is a no-op marker that records
which tables are covered. As of writing the list is:

| Table | Created in |
|-------|------------|
| `users` | `00004_users.sql` |
| `workspaces` | `00005_workspaces.sql` |
| `workspace_members` | `00006_workspace_members.sql` |
| `knowledge_bases` | `00007_knowledge_bases.sql` |
| `documents` | `00008_documents.sql` |
| `sources` | `00009_sources.sql` |
| `chunks` | `00010_chunks_and_embeddings.sql` |
| `embeddings` | `00010_chunks_and_embeddings.sql` |
| `llm_provider_configs` | `00011_llm_provider_configs.sql` |
| `api_keys` | `00012_api_keys.sql` |
| `chat_sessions` | `00013_chat.sql` |
| `chat_messages` | `00013_chat.sql` |
| `processing_events` | `00014_processing_events.sql` |
| `routing_rules` | `00020_routing_rules.sql` |
| `catalog_metadata` | `00020_routing_rules.sql` |
| `airbyte_connectors` | `00021_airbyte_connectors.sql` |
| `airbyte_sync_history` | `00021_airbyte_connectors.sql` |
| `security_rules` | `00022_security_rules.sql` |
| `security_events` | `00022_security_rules.sql` |
| `lead_profiles` | `00023_lead_profiles.sql` |
| `webhook_configs` | `00024_webhook_configs.sql` |
| `webhook_deliveries` | `00024_webhook_configs.sql` |
| `stranger_users` | `00025_stranger_users.sql` |
| `notification_configs` | `00026_email_notifications.sql` |
| `notification_log` | `00026_email_notifications.sql` |
| `response_cache` | `00027_response_cache.sql` |
| `user_identities` | `00028_user_identity.sql` |
| `voice_sessions` | `00029_voice_sessions.sql` |
| `voice_turns` | `00029_voice_sessions.sql` |
| `voice_usage_summaries` | `00030_voice_usage.sql` |
| `whatsapp_phone_numbers` | `00031_whatsapp_calling.sql` |
| `whatsapp_calls` | `00031_whatsapp_calling.sql` |
| `subscriptions` | `00032_billing_rls_and_payment_events.sql` |
| `payment_events` | `00032_billing_rls_and_payment_events.sql` |
| `payment_intents` | `00033_payment_intents.sql` |
| `conversation_sessions` | `00037_conversation_sessions.sql` |

The shape of every policy is identical. Here is the canonical example from
`migrations/00005_workspaces.sql` verbatim:

```sql
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON workspaces
    FOR ALL
    USING (org_id = current_setting('app.current_org_id')::uuid)
    WITH CHECK (org_id = current_setting('app.current_org_id')::uuid);

CREATE POLICY admin_bypass ON workspaces
    FOR ALL TO raven_admin
    USING (true);
```

Two policies are stacked on every protected table:

- `tenant_isolation` — applies to all roles. Reads the per-transaction GUC
  `app.current_org_id`, casts it to `uuid`, and matches against the row's
  `org_id`. `WITH CHECK` mirrors `USING` so writes can never plant a row
  under another tenant.
- `admin_bypass` — applies only to the `raven_admin` role and returns
  `true` unconditionally. This is how migrations and operator scripts read
  across tenants without disabling RLS.

## Per-request GUC

Application code never executes SQL against the pool directly. It calls
`db.WithOrgID`, which opens a transaction, sets the GUC, runs the caller's
function, and commits.

`internal/db/db.go`:

```go
// WithOrgID executes fn inside a transaction with app.current_org_id set for RLS.
// The transaction is automatically rolled back on error and committed on success.
// orgID is passed as a parameter to set_config to prevent SQL injection.
func WithOrgID(ctx context.Context, pool *pgxpool.Pool, orgID string, fn func(tx pgx.Tx) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx) //nolint:errcheck

    if _, err := tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID); err != nil {
        return fmt.Errorf("set org_id: %w", err)
    }
    if err := fn(tx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}
```

Three details are load-bearing:

1. **`set_config(..., true)` — local scope.** The third argument is
   `is_local`; the binding is dropped when the transaction commits or rolls
   back. There is no risk of one request's `org_id` leaking into another
   request that re-uses the same pooled connection.
2. **Parameterised, not interpolated.** `orgID` is bound via `$1` rather
   than concatenated into the SQL string. Even if a caller passed an
   attacker-controlled string, no SQL injection is possible at this layer.
3. **`fn` receives a `pgx.Tx`, never the pool.** Every repository method
   takes the `pgx.Tx` as a parameter, so it is structurally impossible to
   reach the database with the GUC unset.

The canonical caller pattern comes from `internal/service/seed.go`, where a
document is created during demo seeding:

```go
var created *model.Document
err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
    var createErr error
    created, createErr = s.docRepo.Create(ctx, tx, doc)
    return createErr
})
if err != nil {
    return fmt.Errorf("create document record: %w", err)
}
```

Repositories follow the matching shape. From `internal/repository/`
(Airbyte repository shown — the same shape applies to every repo):

```go
func (r *AirbyteRepository) Create(ctx context.Context, tx pgx.Tx, orgID string, req model.CreateConnectorRequest, createdBy string) (*model.AirbyteConnector, error) {
    row := tx.QueryRow(ctx, /* … INSERT … */, orgID, /* … */)
    /* … */
}
```

Note the signature: `(ctx, tx, orgID, …)`. The transaction is always passed
in from above; the repo never opens its own. Today's call sites for
`db.WithOrgID` include `internal/repository/notification.go`,
`internal/repository/conversation.go`, `internal/service/seed.go`, and
others — `grep -rn db.WithOrgID` in the repo enumerates every one.

## Auth middleware → context → DB flow

A single request travels through this sequence:

1. **SuperTokens session verification.** `middleware.SessionMiddleware` in
   `internal/middleware/auth.go` calls `provider.VerifySession(c.Request)`
   on the registered `auth.Provider`. On success it stores the user's
   `ExternalID`, `Email`, and display name in the Gin context.
2. **User → org resolution.** A subsequent `UserLookup` middleware
   resolves the external ID against `users` and writes
   `ContextKeyUserID`, `ContextKeyOrgID`, `ContextKeyOrgRole`, and
   `ContextKeyWorkspaceRole` into the Gin context. Context keys are declared
   in the same file:
   ```go
   const (
       ContextKeyUserID        contextKey = "user_id"
       ContextKeyOrgID         contextKey = "org_id"
       ContextKeyOrgRole       contextKey = "org_role"
       ContextKeyWorkspaceRole contextKey = "workspace_role"
       // …
   )
   ```
3. **Handler reads `org_id` from context.** The HTTP handler pulls
   `ContextKeyOrgID` and passes it down to the service layer.
4. **Service calls `db.WithOrgID`.** The service opens a transaction
   scoped to that org and invokes the repository inside the callback.
5. **Postgres evaluates the policy.** Every statement the repository runs
   is rewritten to append `org_id = current_setting('app.current_org_id')::uuid`.
   Rows belonging to other tenants are invisible.

`RAVEN_SINGLE_USER=true` swaps `SessionMiddleware` for
`SingleUserMiddleware`, which injects fixed UUIDs
(`00000000-0000-0000-0000-000000000001` as the org, `…002` as the user) so
single-user deployments take the same code path without contacting
SuperTokens.

## Why RLS instead of application-side filtering

Application-side filtering — every query carrying its own
`WHERE org_id = $1` clause — is a single point of failure. One forgotten
clause in one repository method is a tenant data leak. Code review catches
most of them; an audit log catches the rest *after* the leak has occurred.

RLS makes the leak impossible at the layer below the application. If a
query against a tenant-scoped table is dispatched without
`app.current_org_id` set, the GUC defaults to the empty string, the cast
`''::uuid` raises `invalid input syntax for type uuid`, and Postgres
returns an error rather than rows. If the GUC is set to a different
tenant's UUID, the policy evaluates to `false` for every row and the query
returns the empty set without leaking a count. This is the defence in
depth the platform's security posture requires.

## Admin bypass

Postgres has a built-in `BYPASSRLS` role attribute, but Raven does **not**
use it. The `raven_app` and `raven_admin` roles created in
`migrations/00002_roles.sql` are plain roles with no special attributes:

```sql
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'raven_app') THEN
    CREATE ROLE raven_app;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'raven_admin') THEN
    CREATE ROLE raven_admin;
  END IF;
END $$;
```

Bypass is implemented as a **second policy** on every protected table,
restricted to the `raven_admin` role:

```sql
CREATE POLICY admin_bypass ON workspaces
    FOR ALL TO raven_admin
    USING (true);
```

When goose runs migrations as `raven_admin` (or the postgres superuser,
which short-circuits RLS in any case) the policy short-circuits to `true`
and the migration sees every row. Production application traffic connects
as the unprivileged `raven_app` role, for which `admin_bypass` does not
match, leaving only `tenant_isolation` in effect.

Newer migrations (`migrations/00032_billing_rls_and_payment_events.sql`)
have started accepting an alternative bypass signal — a GUC named
`app.bypass_rls` set to the string `'true'`. This is used by trusted
internal jobs (e.g. payment webhook reconciliation) that must aggregate
across tenants without holding the `raven_admin` role.

## Testing

RLS is exercised by live-database integration tests under
`internal/integration/`. Mocked unit tests cannot exercise RLS by
construction — there is no Postgres to enforce the policy — which is one
of the reasons the project's testing convention does not use SQL mocks.

The dedicated coverage lives in `internal/integration/rls_test.go`. The
top-level `TestRLS(t)` function fans out into the following sub-tests:

| Sub-test | Asserts |
|----------|---------|
| `document_isolation` | Org-A cannot read Org-B's documents |
| `chunk_isolation_bm25` | BM25 search filtered by RLS, no cross-org chunks returned |
| `embedding_isolation_vector` | Vector similarity does not surface foreign-tenant embeddings |
| `cache_isolation` | Org-A's `Stats(orgA, orgB.KBID)` returns zero entries |
| `cache_invalidation_scoping` | Cache invalidation is bounded to the calling org |
| `source_isolation` | `sources` rows are isolated |
| `cross_org_kb_access` | `GetKnowledgeBase` with another org's ID returns nothing |
| `admin_bypass` | The `raven_admin` role can read across tenants |

Supporting fixtures live in `internal/integration/seed_test.go` (two-org
fixture builder), `internal/integration/helpers_test.go`, and
`internal/integration/setup_test.go` (live Postgres harness via the
project's `setup_test` bootstrap). The migration runner itself is exercised
by `internal/integration/migration_test.go`, which confirms RLS is enabled
on every table named in `00015_rls_policies.sql`.

## Limitations

RLS only protects rows in Postgres. Each non-relational store has its own
isolation strategy.

| Surface | Isolation strategy |
|---------|--------------------|
| SeaweedFS object storage | Object keys are prefixed with `<org_id>/<kb_id>/`. Buckets are not per-tenant; the application layer never composes a SeaweedFS path without an `org_id` prefix. |
| Valkey (cache, queues) | Asynq queue names and cache keys include the `org_id` segment. Keyspace separation is convention, not enforced — corrupting it requires either bypassing the queue/cache wrappers or holding direct Valkey credentials. |
| LiveKit voice rooms | Room names are `<org_id>:<conversation_session_id>`. Tokens minted by the API embed the room name as a claim, and LiveKit rejects a join attempt against a different room. |
| ClickHouse (enterprise QBit) | A separate connection initialised in `cmd/api/main.go` via `db.NewClickHouse` (`internal/db/clickhouse.go`). ClickHouse does not have RLS; every query is parameterised with the caller's `org_id` and reviewed by hand. |
| OpenObserve / Beszel telemetry | Run as separate deployments; access control is at the OpenObserve organisation / Beszel user level, not row-level. |

These surfaces are why "Raven uses RLS for tenant isolation" is true but
incomplete. The full answer is: **RLS for the relational data, key-prefix
discipline for object and cache stores, and signed room/queue names for
real-time channels.**

## See also

- [Self-Hosting: Hardening](/guides/self-hosting/hardening)
- [Data model](/concepts/data-model)
- [API: Authentication](/api/authentication)
