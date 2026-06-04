package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// stubImporter is a hand-rolled stub that records the call and replays a
// canned response. Hand-rolled (no testify mock) because the surface is one
// method and one response shape — a mock library would add more noise than
// it removes.
type stubImporter struct {
	called bool
	args   struct {
		sourceKBID, dstOrgID, dstWS, dstUser uuid.UUID
	}
	result *marketplace.ImportResult
	err    error
}

func (s *stubImporter) Import(_ context.Context,
	sourceKBID, dstOrgID, dstWS, dstUser uuid.UUID,
) (*marketplace.ImportResult, error) {
	s.called = true
	s.args.sourceKBID = sourceKBID
	s.args.dstOrgID = dstOrgID
	s.args.dstWS = dstWS
	s.args.dstUser = dstUser
	return s.result, s.err
}

func newTestRouter(h *handler.MarketplaceHandler, userID, orgID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(string(middleware.ContextKeyUserID), userID)
		}
		if orgID != "" {
			c.Set(string(middleware.ContextKeyOrgID), orgID)
		}
		c.Next()
	})
	r.POST("/marketplace/import/:public_kb_id", h.Import)
	return r
}

func TestMarketplaceHandler_Import_HappyPath(t *testing.T) {
	srcID := uuid.New()
	orgID := uuid.New()
	wsID := uuid.New()
	userID := uuid.New()

	st := &stubImporter{
		result: &marketplace.ImportResult{
			KBID:                   uuid.New(),
			WorkspaceID:            wsID,
			Visibility:             "private",
			ImportedFromRevisionAt: time.Now().UTC(),
			SourcePublicKBID:       srcID,
		},
	}
	h := handler.NewMarketplaceHandler(st)
	r := newTestRouter(h, userID.String(), orgID.String())

	body, _ := json.Marshal(handler.ImportKBRequest{WorkspaceID: wsID.String()})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/marketplace/import/"+srcID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d want 201; body=%s", rec.Code, rec.Body.String())
	}
	if !st.called {
		t.Fatal("Importer.Import not called")
	}
	if st.args.sourceKBID != srcID || st.args.dstOrgID != orgID ||
		st.args.dstWS != wsID || st.args.dstUser != userID {
		t.Errorf("args mismatch: %+v", st.args)
	}
}

func TestMarketplaceHandler_Import_InvalidPathParam_404(t *testing.T) {
	h := handler.NewMarketplaceHandler(&stubImporter{})
	r := newTestRouter(h, uuid.New().String(), uuid.New().String())

	body, _ := json.Marshal(handler.ImportKBRequest{WorkspaceID: uuid.New().String()})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/marketplace/import/not-a-uuid", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid uuid; got %d", rec.Code)
	}
}

func TestMarketplaceHandler_Import_MissingUser_401(t *testing.T) {
	h := handler.NewMarketplaceHandler(&stubImporter{})
	r := newTestRouter(h, "", uuid.New().String()) // no user

	body, _ := json.Marshal(handler.ImportKBRequest{WorkspaceID: uuid.New().String()})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/marketplace/import/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with no user context; got %d", rec.Code)
	}
}

func TestMarketplaceHandler_Import_InvalidWorkspaceID_422(t *testing.T) {
	h := handler.NewMarketplaceHandler(&stubImporter{})
	r := newTestRouter(h, uuid.New().String(), uuid.New().String())

	// workspace_id is not a UUID — binding tag rejects.
	body := []byte(`{"workspace_id":"banana"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/marketplace/import/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid workspace_id; got %d", rec.Code)
	}
}

// TestMarketplaceHandler_Import_ImporterErrorPropagates verifies the
// handler does NOT translate service-layer AppErrors itself — it just
// forwards them through Gin's error middleware. Keeps the
// "service owns the contract" invariant from publish.go.
func TestMarketplaceHandler_Import_ImporterErrorPropagates(t *testing.T) {
	st := &stubImporter{err: apierror.NewConflict("already imported")}
	h := handler.NewMarketplaceHandler(st)
	r := newTestRouter(h, uuid.New().String(), uuid.New().String())

	body, _ := json.Marshal(handler.ImportKBRequest{WorkspaceID: uuid.New().String()})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
		"/marketplace/import/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409; got %d", rec.Code)
	}
}

// TestMarketplaceHandler_NewMarketplaceHandler_NilImporter_Panics pins the
// "fail loud on misconfiguration" contract from NewMarketplaceHandler.
func TestMarketplaceHandler_NewMarketplaceHandler_NilImporter_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic with nil importer")
		}
	}()
	_ = handler.NewMarketplaceHandler(nil)
}

// Compile-time check that errors package is reachable from this test (used
// only by assertion shapes if any). Silences linter false positives when
// further tests are added.
var _ = errors.New
