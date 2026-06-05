// Derivative-owner notifier: when a Public KB is taken down, every Org
// whose private KB was imported from that Public KB (one hop only) gets
// an email so the importer's admins know their source has been removed.
// ADR-0004 §"Notify, don't cascade" pins this rule: derivatives keep
// working — they were forked at import time per ADR-0001 — but their
// owners deserve a heads-up so they can decide whether to retain the
// fork, swap to a different source, or delete it.
//
// One hop only is structural — the SQL below has no recursion. Derivatives
// of derivatives are not in scope: the lineage UI shows one hop back, the
// notification mirrors that scope, and chasing the full graph would
// quickly fan out to thousands of unrelated Orgs.
//
// The notifier is fire-and-forget at the call site: failures are logged
// with enough detail to recover (the takedown row is the durable record;
// the email is best-effort delivery on top). Promotion to an Asynq task
// is the natural M3 follow-up — the issue acceptance criteria mentions
// `internal/jobs/marketplace_takedown_notifier.go` for that — but the
// in-process path is deliberate for this PR so the takedown audit log
// and the notification semantics ship together and #734's approve handler
// has one obvious wire-up point.

package marketplace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/mail"
)

// Notifier is the seam between the marketplace package and whatever
// transport actually delivers a derivative-takedown notice. Production
// wiring satisfies this with internal/mail.Sender; tests substitute a
// recording mock to assert "called once per derivative" without spinning
// up SMTP. Decoupling from internal/mail.Sender directly means we can
// route to Asynq later (issue #735's stretch goal) by registering an
// adapter that enqueues a job instead of sending inline.
type Notifier interface {
	// NotifyDerivativeTakedown is invoked once per workspace-admin email
	// that should receive a notice. Implementations MUST be safe to call
	// concurrently; the walker may parallelise per derivative in the
	// future.
	NotifyDerivativeTakedown(ctx context.Context, n DerivativeNotice) error
}

// DerivativeNotice carries everything an email template needs: who to
// reach (RecipientEmail), what their KB is called (DerivativeKBName),
// which upstream KB went away (SourceKBName, SourceOrgDisplayName), and
// what the moderation note said (Reason). Notes may be empty when the
// takedown row had no notes.
//
// We pass display-ready strings rather than IDs so the notifier
// implementation doesn't have to re-query the DB just to format an
// email — the walker already paid for the JOIN.
type DerivativeNotice struct {
	RecipientEmail       string
	DerivativeKBID       uuid.UUID
	DerivativeKBName     string
	SourceKBID           uuid.UUID
	SourceKBName         string
	SourceOrgDisplayName string
	Reason               string
}

// MailNotifier adapts a generic mail.Sender to the Notifier seam. It is
// the default production wiring; the email body is plain text rather
// than HTML so the M3 transactional path works without templates — when
// templates land per #735's acceptance criteria, swap this for a
// templated implementation without touching the walker.
type MailNotifier struct {
	Sender mail.Sender
	From   string
}

// NewMailNotifier returns a MailNotifier backed by the supplied sender.
// A nil sender falls back to a NoopSender so dev / single-user runs
// without RESEND_API_KEY still progress through the takedown path
// without panicking.
func NewMailNotifier(sender mail.Sender) *MailNotifier {
	if sender == nil {
		sender = &mail.NoopSender{}
	}
	return &MailNotifier{Sender: sender}
}

// NotifyDerivativeTakedown formats and sends a single notice. The
// subject and body are deliberately terse — the email is informational,
// not transactional, and we don't want to import a template engine just
// for this surface. Future templated bodies can extend this.
func (m *MailNotifier) NotifyDerivativeTakedown(ctx context.Context, n DerivativeNotice) error {
	subject := fmt.Sprintf("Heads-up: upstream Marketplace KB %q was taken down", n.SourceKBName)
	body := fmt.Sprintf(
		`Hi,

A Public Knowledge Base you imported into Raven has been taken down from the Marketplace.

  Upstream KB : %s (org: %s)
  Your KB     : %s
  Reason      : %s

Your private copy of the data still works — Raven forks Public KBs at import time,
so the upstream takedown does not cascade to your workspace. You may want to:

  - Review whether to keep your derivative KB.
  - Swap to a different upstream source if one exists.
  - Delete your derivative if you no longer have grounds to use it.

If you have questions, reply to this email and we will route you to the moderation
team.

— Raven`,
		n.SourceKBName, n.SourceOrgDisplayName, n.DerivativeKBName, fallback(n.Reason, "(no reason recorded)"),
	)
	return m.Sender.Send(ctx, mail.Message{
		To:      n.RecipientEmail,
		Subject: subject,
		Text:    body,
	})
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

// DerivativeNotifier walks the one-hop lineage graph for a taken-down
// Public KB and dispatches notices via the configured Notifier.
//
// Construction takes the pool because the walker issues a single
// JOINed SELECT under raven_admin RLS bypass — non-admin sessions would
// silently return zero rows due to tenant_isolation on knowledge_bases,
// users, and workspace_members. The repository switches roles inside a
// transaction so the caller's outer session isolation is preserved.
type DerivativeNotifier struct {
	pool     *pgxpool.Pool
	notifier Notifier
}

// NewDerivativeNotifier returns a walker bound to pool + notifier.
// A nil notifier panics — that would only happen on a wiring bug, and
// a panic at boot is preferable to silently dropping every notice.
func NewDerivativeNotifier(pool *pgxpool.Pool, notifier Notifier) *DerivativeNotifier {
	if notifier == nil {
		panic("marketplace: NewDerivativeNotifier requires a non-nil Notifier")
	}
	return &DerivativeNotifier{pool: pool, notifier: notifier}
}

// NotifyDerivativeOwners walks `knowledge_bases` rows whose
// `source_public_kb_id = takenDownKBID` and dispatches one notice per
// workspace admin of each derivative's Org.
//
// "One hop only" is structural: the SELECT has no recursive CTE. A
// derivative-of-a-derivative would have its source_public_kb_id pointing
// at the intermediate, not at takenDownKBID, so it never matches.
//
// Errors from individual notifier sends are logged but not fatal — the
// walker continues through the remaining recipients. The first
// dispatch error is returned so callers can surface a single "some
// notices failed" signal; the audit log row was already committed and is
// the durable record either way.
func (d *DerivativeNotifier) NotifyDerivativeOwners(ctx context.Context, takenDownKBID uuid.UUID, reason string) error {
	if d == nil {
		return nil
	}

	// Switch to raven_admin so the SELECT can cross tenant boundaries —
	// the walker reaches into every importer Org's workspace_members.
	// Running this in a transaction means SET LOCAL ROLE is scoped to
	// the JOIN and reverts on commit; the outer pool connection is
	// returned in its original role.
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("marketplace: notify derivatives: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SET LOCAL ROLE raven_admin"); err != nil {
		return fmt.Errorf("marketplace: notify derivatives: set role: %w", err)
	}

	// The SELECT joins three tables. derived_kb is the importer's
	// private KB; source_kb gives us the display name of the taken-down
	// upstream (this is the same KB ID we were called with, but joining
	// it here keeps the row shape self-contained); source_org gives the
	// upstream Org's display name; workspace_members gates on role='admin'
	// so we email decision-makers, not every viewer in the workspace.
	//
	// DISTINCT on (derivative_kb_id, user_email) collapses the case where
	// the same admin is a member of multiple workspaces inside the
	// importer Org — we send them one email per derivative KB, not one
	// per (KB, workspace) pair.
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT
		    derived_kb.id           AS derivative_kb_id,
		    derived_kb.name         AS derivative_kb_name,
		    source_kb.id            AS source_kb_id,
		    source_kb.name          AS source_kb_name,
		    source_org.name         AS source_org_display_name,
		    u.email                 AS recipient_email
		FROM knowledge_bases AS derived_kb
		JOIN knowledge_bases  AS source_kb  ON source_kb.id  = derived_kb.source_public_kb_id
		JOIN organizations    AS source_org ON source_org.id = source_kb.org_id
		JOIN workspace_members AS wm        ON wm.workspace_id = derived_kb.workspace_id
		                                   AND wm.role = 'admin'
		JOIN users             AS u         ON u.id = wm.user_id
		                                   AND u.status = 'active'
		WHERE derived_kb.source_public_kb_id = $1
	`, takenDownKBID)
	if err != nil {
		return fmt.Errorf("marketplace: notify derivatives: query: %w", err)
	}

	var notices []DerivativeNotice
	for rows.Next() {
		var n DerivativeNotice
		if err := rows.Scan(
			&n.DerivativeKBID,
			&n.DerivativeKBName,
			&n.SourceKBID,
			&n.SourceKBName,
			&n.SourceOrgDisplayName,
			&n.RecipientEmail,
		); err != nil {
			rows.Close()
			return fmt.Errorf("marketplace: notify derivatives: scan: %w", err)
		}
		n.Reason = reason
		notices = append(notices, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("marketplace: notify derivatives: rows: %w", err)
	}

	// Commit before any I/O. The DB work is done; downstream notifier
	// sends shouldn't hold a transaction open.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("marketplace: notify derivatives: commit: %w", err)
	}

	// Dispatch outside the transaction. Log every failure with enough
	// detail to manually re-send if a downstream provider is flaky.
	var firstErr error
	for _, n := range notices {
		if err := d.notifier.NotifyDerivativeTakedown(ctx, n); err != nil {
			slog.Error("derivative takedown notice failed",
				slog.String("recipient", n.RecipientEmail),
				slog.String("source_kb_id", n.SourceKBID.String()),
				slog.String("derivative_kb_id", n.DerivativeKBID.String()),
				slog.String("err", err.Error()),
			)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// ─── OnTakedownCreated registry ──────────────────────────────────────────────
//
// We chose a registry over a callback on Takedowns.Create deliberately:
//
//   - Takedowns.Create is a thin INSERT — adding a callback parameter
//     would couple the data-access layer to the notifier interface (and
//     mean every test that touches Takedowns has to thread one through).
//   - The notifier is a *side effect* of the takedown decision, not part
//     of the audit-log write. ADR-0004 is explicit that the takedown
//     row is the durable record; notification is best-effort on top.
//     Separating them at the seam mirrors that split.
//   - #734's admin approve path needs to invoke this from a service that
//     already has a takedown writer wired; a registry the API layer
//     populates once at boot keeps #734 from importing internal/mail.
//
// Mutability + thread-safety: handlers register at boot, so we lock
// only for paranoia — the production wire-up is single-writer at
// startup and many-reader at request time. The mutex makes that
// invariant explicit and gives tests a clean reset path.

type takedownCreatedHook func(ctx context.Context, t Takedown, reason string) error

var (
	takedownCreatedMu    sync.RWMutex
	takedownCreatedHooks []takedownCreatedHook
)

// RegisterOnTakedownCreated subscribes a hook to be invoked whenever a
// takedown audit-log row is created via DispatchOnTakedownCreated. The
// notifier wires itself here at API boot; tests subscribe a recording
// closure.
//
// Hooks are invoked in registration order; a hook returning an error
// does NOT short-circuit later hooks (the takedown row already exists
// and we don't want one flaky notifier to block another).
func RegisterOnTakedownCreated(h takedownCreatedHook) {
	if h == nil {
		return
	}
	takedownCreatedMu.Lock()
	defer takedownCreatedMu.Unlock()
	takedownCreatedHooks = append(takedownCreatedHooks, h)
}

// ResetOnTakedownCreatedForTest clears the registry. Test-only. Lives
// in non-test code because cross-package tests (cmd/api integration)
// also need it.
func ResetOnTakedownCreatedForTest() {
	takedownCreatedMu.Lock()
	defer takedownCreatedMu.Unlock()
	takedownCreatedHooks = nil
}

// DispatchOnTakedownCreated invokes every registered hook with the
// given takedown row + reason. The first error is returned (caller
// can decide to log + ignore); the rest are folded via errors.Join.
//
// Callers MUST invoke this after a successful Takedowns.Create —
// the moderation service in #734's admin approve path does so. This
// PR wires the registry but does not retrofit existing call sites
// (publisher self-takedown and #734) — both are TODO comments on the
// relevant handlers and #734 will add the explicit dispatch as part
// of landing its admin approve flow.
func DispatchOnTakedownCreated(ctx context.Context, t Takedown, reason string) error {
	takedownCreatedMu.RLock()
	hooks := append([]takedownCreatedHook(nil), takedownCreatedHooks...)
	takedownCreatedMu.RUnlock()

	var errs []error
	for _, h := range hooks {
		if err := h(ctx, t, reason); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
