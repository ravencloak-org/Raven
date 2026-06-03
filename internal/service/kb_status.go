package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// KBStatusReader is the narrow slice of *repository.KBRepository the
// service layer needs to enforce KBStatusGate semantics at write / chat
// surfaces. Kept as an interface so tests can pass a stub without
// standing up a real Postgres pool.
type KBStatusReader interface {
	LoadStatus(ctx context.Context, tx pgx.Tx, orgID, kbID string) (model.KBStatus, error)
}

// KBStatusGuard runs the gate check for a KB in its own transaction.
// Services hold one of these (optional; nil disables the check for unit
// tests that don't care about lifecycle gating) and dispatch every
// status-sensitive operation through it. The capability argument decides
// which gate method is consulted — the per-action error rendering lives
// in capabilityAction so call sites stay as a single line.
type KBStatusGuard struct {
	reader KBStatusReader
	pool   *pgxpool.Pool
}

// NewKBStatusGuard builds a guard. Production wiring supplies a real
// repository + pool; tests that only care about the non-gate behaviour
// of a service can leave both nil and call sites short-circuit.
func NewKBStatusGuard(reader KBStatusReader, pool *pgxpool.Pool) *KBStatusGuard {
	return &KBStatusGuard{reader: reader, pool: pool}
}

// KBAction names the capability being asserted against the gate. The set
// is closed (no string sprawl at call sites) and each value maps to a
// canonical AppError shape so handlers don't have to translate.
type KBAction int

const (
	// KBActionIngest covers "mutate the corpus" — uploads, source CRUD,
	// settings updates that affect what the KB returns.
	KBActionIngest KBAction = iota
	// KBActionChat covers chat / widget completion paths.
	KBActionChat
	// KBActionPublish covers the Marketplace publish action (ADR-0002).
	KBActionPublish
)

// Require asserts that the KB identified by kbID currently permits the
// given action. It returns nil for unknown KBs (the caller's existing
// "not found" path stays in charge — keeping the guard cleanly additive
// rather than coupling it to every service's missing-row policy).
//
// When the guard's reader is nil (test wiring) Require is a no-op so
// existing unit tests that focus on validation logic don't need to
// stand up a full Postgres fake.
func (g *KBStatusGuard) Require(ctx context.Context, orgID, kbID string, action KBAction) error {
	if g == nil || g.reader == nil || g.pool == nil {
		return nil
	}
	var status model.KBStatus
	err := db.WithOrgID(ctx, g.pool, orgID, func(tx pgx.Tx) error {
		var loadErr error
		status, loadErr = g.reader.LoadStatus(ctx, tx, orgID, kbID)
		return loadErr
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			// Unknown KB — leave the existing "not found" path to handle
			// it. The guard is about freeze policy, not existence.
			return nil
		}
		return apierror.NewInternal("failed to resolve knowledge base status: " + err.Error())
	}
	return checkKBStatus(status, action)
}

// RequireInTx is the in-transaction variant for callers that have
// already opened a db.WithOrgID block. It avoids the second round-trip
// the standalone Require would otherwise pay.
func (g *KBStatusGuard) RequireInTx(ctx context.Context, tx pgx.Tx, orgID, kbID string, action KBAction) error {
	if g == nil || g.reader == nil {
		return nil
	}
	status, err := g.reader.LoadStatus(ctx, tx, orgID, kbID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
			return nil
		}
		return apierror.NewInternal("failed to resolve knowledge base status: " + err.Error())
	}
	return checkKBStatus(status, action)
}

// isAppErr reports whether err is an already-rendered *apierror.AppError.
// Services that wrap the guard inside a db.WithOrgID closure use this to
// short-circuit their generic "no rows → 404" / "wrapping" branches when
// the guard has already produced a 409 / 423 — preserving the canonical
// machine-readable code instead of laundering it into a 500.
func isAppErr(err error) bool {
	var appErr *apierror.AppError
	return errors.As(err, &appErr)
}

// checkKBStatus is the shared policy: it asks the gate and renders the
// canonical AppError for the rejected action. Keeping it package-private
// means every service that gates uses identical wording and error codes.
func checkKBStatus(status model.KBStatus, action KBAction) error {
	gate := model.NewKBStatusGate(status)
	switch action {
	case KBActionIngest:
		if gate.CanIngest() {
			return nil
		}
	case KBActionChat:
		if gate.CanChat() {
			return nil
		}
	case KBActionPublish:
		if gate.CanPublish() {
			return nil
		}
	default:
		return apierror.NewInternal(fmt.Sprintf("unknown KBAction: %d", action))
	}

	// Rejected. Pick the error code based on the underlying status, not
	// the action — the SPA reacts to the lifecycle state, not the verb.
	reason := gate.PublicReason()
	switch status {
	case model.KBStatusDMCAPending:
		return apierror.NewKBDMCALocked(reason)
	case model.KBStatusArchived:
		return apierror.NewNotFound("knowledge base not found")
	default:
		// read_only_private and any future "soft freeze" land here.
		return apierror.NewKBFrozen(reason)
	}
}
