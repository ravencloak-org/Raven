package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// stubKBStatusReader returns a pre-canned status / error for the in-tx
// guard path. It's intentionally minimal — the guard never inspects the
// pgx.Tx argument so a nil sentinel is sufficient.
type stubKBStatusReader struct {
	status model.KBStatus
	err    error
	calls  int
}

func (s *stubKBStatusReader) LoadStatus(_ context.Context, _ pgx.Tx, _, _ string) (model.KBStatus, error) {
	s.calls++
	return s.status, s.err
}

// TestKBStatusGuard_RequireInTx_MatrixByStatus is the integration-shape
// test promised in the issue body: for every (status × action) pair we
// confirm the HTTP shape returned to the handler. This is the gate's
// contract; it pins both the status code AND the machine-readable error
// code so the SPA / widget JS can branch deterministically.
func TestKBStatusGuard_RequireInTx_MatrixByStatus(t *testing.T) {
	type want struct {
		// nilErr is true when the action is allowed.
		nilErr bool
		// code is the expected HTTP status when nilErr is false.
		code int
		// errorCode is the machine-readable identifier on the AppError.
		errorCode string
	}
	type row struct {
		status model.KBStatus
		action KBAction
		want   want
	}

	cases := []row{
		// Active — everything green.
		{model.KBStatusActive, KBActionIngest, want{nilErr: true}},
		{model.KBStatusActive, KBActionChat, want{nilErr: true}},
		{model.KBStatusActive, KBActionPublish, want{nilErr: true}},

		// Read-only-private: 409 kb_frozen for write paths; chat allowed.
		{model.KBStatusReadOnlyPrivate, KBActionIngest, want{code: http.StatusConflict, errorCode: "kb_frozen"}},
		{model.KBStatusReadOnlyPrivate, KBActionChat, want{nilErr: true}},
		{model.KBStatusReadOnlyPrivate, KBActionPublish, want{code: http.StatusConflict, errorCode: "kb_frozen"}},

		// DMCA pending: 423 kb_dmca_locked across the board.
		{model.KBStatusDMCAPending, KBActionIngest, want{code: http.StatusLocked, errorCode: "kb_dmca_locked"}},
		{model.KBStatusDMCAPending, KBActionChat, want{code: http.StatusLocked, errorCode: "kb_dmca_locked"}},
		{model.KBStatusDMCAPending, KBActionPublish, want{code: http.StatusLocked, errorCode: "kb_dmca_locked"}},

		// Archived: 404 (the row should never have reached the gate, but
		// if it does the user-facing answer is "doesn't exist", not
		// "frozen" — we don't leak the soft-delete state).
		{model.KBStatusArchived, KBActionIngest, want{code: http.StatusNotFound}},
		{model.KBStatusArchived, KBActionChat, want{code: http.StatusNotFound}},
		{model.KBStatusArchived, KBActionPublish, want{code: http.StatusNotFound}},
	}

	for _, c := range cases {
		c := c
		t.Run(string(c.status)+"/"+actionName(c.action), func(t *testing.T) {
			reader := &stubKBStatusReader{status: c.status}
			guard := NewKBStatusGuard(reader, nil)
			err := guard.RequireInTx(context.Background(), nil, "org-1", "kb-1", c.action)
			if c.want.nilErr {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var appErr *apierror.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("expected *apierror.AppError, got %T", err)
			}
			if appErr.Code != c.want.code {
				t.Errorf("HTTP code: want %d, got %d", c.want.code, appErr.Code)
			}
			if appErr.ErrorCode != c.want.errorCode {
				t.Errorf("ErrorCode: want %q, got %q", c.want.errorCode, appErr.ErrorCode)
			}
		})
	}
}

// TestKBStatusGuard_NoReader_IsNoop pins the test-wiring contract: a guard
// constructed without a reader silently approves every action so existing
// service unit tests that ignore lifecycle gating continue to pass.
func TestKBStatusGuard_NoReader_IsNoop(t *testing.T) {
	var nilGuard *KBStatusGuard
	if err := nilGuard.Require(context.Background(), "org", "kb", KBActionIngest); err != nil {
		t.Errorf("nil guard: expected no-op, got %v", err)
	}
	if err := nilGuard.RequireInTx(context.Background(), nil, "org", "kb", KBActionChat); err != nil {
		t.Errorf("nil guard: expected no-op, got %v", err)
	}

	emptyGuard := &KBStatusGuard{}
	if err := emptyGuard.RequireInTx(context.Background(), nil, "org", "kb", KBActionIngest); err != nil {
		t.Errorf("empty guard: expected no-op, got %v", err)
	}
}

// TestKBStatusGuard_RequireInTx_NoRowsIsNoop verifies that a missing KB
// surfaces nil from the guard — the caller's existing "not found" path
// stays in charge of that branch. Keeps the guard cleanly additive to
// every service's existence-handling, rather than a second source of
// truth.
func TestKBStatusGuard_RequireInTx_NoRowsIsNoop(t *testing.T) {
	reader := &stubKBStatusReader{err: pgx.ErrNoRows}
	guard := NewKBStatusGuard(reader, nil)
	if err := guard.RequireInTx(context.Background(), nil, "org", "kb", KBActionIngest); err != nil {
		t.Errorf("no-rows from reader: expected nil, got %v", err)
	}
}

// TestKBStatusGuard_RequireInTx_LoadError_500 verifies an unexpected DB
// error is wrapped as 500, not silently approved (which would be a
// permissive failure mode).
func TestKBStatusGuard_RequireInTx_LoadError_500(t *testing.T) {
	reader := &stubKBStatusReader{err: errors.New("connection refused")}
	guard := NewKBStatusGuard(reader, nil)
	err := guard.RequireInTx(context.Background(), nil, "org", "kb", KBActionIngest)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var appErr *apierror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", appErr.Code)
	}
}

func actionName(a KBAction) string {
	switch a {
	case KBActionIngest:
		return "ingest"
	case KBActionChat:
		return "chat"
	case KBActionPublish:
		return "publish"
	}
	return "unknown"
}
