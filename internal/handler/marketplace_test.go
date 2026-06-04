package handler_test

import (
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
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockMarketplaceLister implements handler.MarketplaceLister for unit
// tests. Each function is settable so a test can target one path
// (List vs. Preview) without dragging in stubs for the other.
type mockMarketplaceLister struct {
	listFn    func(ctx context.Context, f marketplace.ListFilters) ([]marketplace.ListItem, error)
	previewFn func(ctx context.Context, kbID uuid.UUID) ([]marketplace.PreviewChunk, error)
}

func (m *mockMarketplaceLister) ListPublicKBs(ctx context.Context, f marketplace.ListFilters) ([]marketplace.ListItem, error) {
	return m.listFn(ctx, f)
}

func (m *mockMarketplaceLister) PreviewKB(ctx context.Context, kbID uuid.UUID) ([]marketplace.PreviewChunk, error) {
	return m.previewFn(ctx, kbID)
}

func newMarketplaceRouter(q handler.MarketplaceLister, resolver handler.SlugResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewMarketplaceHandler(q, resolver)
	r.GET("/api/v1/marketplace", h.List)
	r.GET("/api/v1/marketplace/:org_slug/:kb_slug", h.Detail)
	r.GET("/api/v1/marketplace/:org_slug/:kb_slug/preview", h.Preview)
	return r
}

// fakeListItem returns a ListItem with a deterministic UUID derived from
// `name` so tests can compare across calls without flakiness.
func fakeListItem(t *testing.T, name string) marketplace.ListItem {
	t.Helper()
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(name))
	return marketplace.ListItem{
		KBID:           id,
		OrgSlug:        "acme",
		OrgDisplayName: "Acme",
		KBSlug:         name,
		KBName:         name,
		LastModifiedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		ImportCount:    0,
	}
}

// ----------------------------------------------------------------------
// Listing endpoint
// ----------------------------------------------------------------------

func TestMarketplaceList_HappyPath_DefaultsToNewest(t *testing.T) {
	var seen marketplace.ListFilters
	svc := &mockMarketplaceLister{
		listFn: func(_ context.Context, f marketplace.ListFilters) ([]marketplace.ListItem, error) {
			seen = f
			return []marketplace.ListItem{fakeListItem(t, "alpha")}, nil
		},
	}
	r := newMarketplaceRouter(svc, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if seen.Sort != marketplace.SortNewest {
		t.Errorf("expected default sort=newest, got %q", seen.Sort)
	}
	if seen.Limit != 25 {
		t.Errorf("expected default limit=25, got %d", seen.Limit)
	}
	var resp handler.MarketplaceListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].KBSlug != "alpha" {
		t.Errorf("unexpected items: %+v", resp.Items)
	}
	if resp.NextOffset != nil {
		t.Errorf("expected next_offset nil for partial page, got %v", *resp.NextOffset)
	}
}

func TestMarketplaceList_InvalidSort_Returns400WithCode(t *testing.T) {
	svc := &mockMarketplaceLister{
		listFn: func(_ context.Context, _ marketplace.ListFilters) ([]marketplace.ListItem, error) {
			t.Fatal("Queries should not be called for invalid sort")
			return nil, nil
		},
	}
	r := newMarketplaceRouter(svc, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace?sort=bogus", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var body apierror.AppError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ErrorCode != "invalid_sort" {
		t.Errorf("expected error_code=invalid_sort, got %q", body.ErrorCode)
	}
}

func TestMarketplaceList_InvalidLicense_Returns400WithCode(t *testing.T) {
	svc := &mockMarketplaceLister{
		listFn: func(_ context.Context, _ marketplace.ListFilters) ([]marketplace.ListItem, error) {
			t.Fatal("Queries should not be called for invalid license")
			return nil, nil
		},
	}
	r := newMarketplaceRouter(svc, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace?license=BAD-LICENSE-9.0", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var body apierror.AppError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ErrorCode != "invalid_license" {
		t.Errorf("expected error_code=invalid_license, got %q", body.ErrorCode)
	}
}

func TestMarketplaceList_LimitClampedAt50(t *testing.T) {
	var seen marketplace.ListFilters
	svc := &mockMarketplaceLister{
		listFn: func(_ context.Context, f marketplace.ListFilters) ([]marketplace.ListItem, error) {
			seen = f
			return nil, nil
		},
	}
	r := newMarketplaceRouter(svc, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace?limit=999", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if seen.Limit != 50 {
		t.Errorf("expected limit clamped to 50, got %d", seen.Limit)
	}
}

func TestMarketplaceList_Pagination_DistinctPages(t *testing.T) {
	pages := [][]marketplace.ListItem{
		{fakeListItem(t, "a"), fakeListItem(t, "b")},
		{fakeListItem(t, "c"), fakeListItem(t, "d")},
	}
	svc := &mockMarketplaceLister{
		listFn: func(_ context.Context, f marketplace.ListFilters) ([]marketplace.ListItem, error) {
			pageIdx := f.Offset / 2
			if pageIdx >= len(pages) {
				return nil, nil
			}
			return pages[pageIdx], nil
		},
	}
	r := newMarketplaceRouter(svc, nil)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace?limit=2&offset=0", nil)
	r.ServeHTTP(w1, req1)
	var p1 handler.MarketplaceListResponse
	if err := json.Unmarshal(w1.Body.Bytes(), &p1); err != nil {
		t.Fatalf("decode p1: %v", err)
	}
	if len(p1.Items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(p1.Items))
	}
	if p1.NextOffset == nil || *p1.NextOffset != 2 {
		t.Errorf("expected next_offset=2 on full page, got %v", p1.NextOffset)
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace?limit=2&offset=2", nil)
	r.ServeHTTP(w2, req2)
	var p2 handler.MarketplaceListResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &p2); err != nil {
		t.Fatalf("decode p2: %v", err)
	}
	if len(p2.Items) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(p2.Items))
	}

	// Pages must not overlap — every slug from page 1 must be absent
	// from page 2.
	page1Slugs := map[string]bool{}
	for _, it := range p1.Items {
		page1Slugs[it.KBSlug] = true
	}
	for _, it := range p2.Items {
		if page1Slugs[it.KBSlug] {
			t.Errorf("page 2 leaked item %q from page 1", it.KBSlug)
		}
	}
}

// ----------------------------------------------------------------------
// Detail endpoint
// ----------------------------------------------------------------------

func TestMarketplaceDetail_HappyPath(t *testing.T) {
	item := fakeListItem(t, "alpha")
	svc := &mockMarketplaceLister{}
	resolver := func(_ context.Context, orgSlug, kbSlug string) (marketplace.SlugStatus, error) {
		if orgSlug != "acme" || kbSlug != "alpha" {
			t.Errorf("unexpected resolver args (%q,%q)", orgSlug, kbSlug)
		}
		return marketplace.SlugStatus{KBID: item.KBID, Detail: &item}, nil
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/acme/alpha", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out handler.MarketplaceListItem
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.KBID != item.KBID {
		t.Errorf("expected kb id %s, got %s", item.KBID, out.KBID)
	}
}

func TestMarketplaceDetail_NotFound_Returns404(t *testing.T) {
	svc := &mockMarketplaceLister{}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{}, nil // Detail nil + IsHeld false → 404
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/unknown/missing", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMarketplaceDetail_Held_Returns410WithCode(t *testing.T) {
	svc := &mockMarketplaceLister{}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{IsHeld: true}, nil
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/acme/old", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d: %s", w.Code, w.Body.String())
	}
	var body apierror.AppError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ErrorCode != "slug_held" {
		t.Errorf("expected error_code=slug_held, got %q", body.ErrorCode)
	}
}

func TestMarketplaceDetail_ResolverError_Returns500(t *testing.T) {
	svc := &mockMarketplaceLister{}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{}, errors.New("boom")
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/acme/alpha", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ----------------------------------------------------------------------
// Preview endpoint
// ----------------------------------------------------------------------

func TestMarketplacePreview_HappyPath_ReturnsChunks(t *testing.T) {
	item := fakeListItem(t, "alpha")
	chunks := []marketplace.PreviewChunk{
		{ChunkID: uuid.New(), Ordinal: 0, Text: "first"},
		{ChunkID: uuid.New(), Ordinal: 1, Text: "second"},
	}
	svc := &mockMarketplaceLister{
		previewFn: func(_ context.Context, kbID uuid.UUID) ([]marketplace.PreviewChunk, error) {
			if kbID != item.KBID {
				t.Errorf("expected kb id %s, got %s", item.KBID, kbID)
			}
			return chunks, nil
		},
	}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{KBID: item.KBID, Detail: &item}, nil
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/acme/alpha/preview", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var out handler.MarketplacePreviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(out.Chunks))
	}
}

func TestMarketplacePreview_NotPublic_Returns403(t *testing.T) {
	item := fakeListItem(t, "alpha")
	svc := &mockMarketplaceLister{
		previewFn: func(_ context.Context, _ uuid.UUID) ([]marketplace.PreviewChunk, error) {
			return nil, marketplace.ErrKBNotPublic
		},
	}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{KBID: item.KBID, Detail: &item}, nil
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/acme/alpha/preview", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMarketplacePreview_NotFound_Returns404(t *testing.T) {
	svc := &mockMarketplaceLister{
		previewFn: func(_ context.Context, _ uuid.UUID) ([]marketplace.PreviewChunk, error) {
			t.Fatal("PreviewKB should not be called when slug resolution misses")
			return nil, nil
		},
	}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{}, nil
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/unknown/missing/preview", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMarketplacePreview_Held_Returns410(t *testing.T) {
	svc := &mockMarketplaceLister{
		previewFn: func(_ context.Context, _ uuid.UUID) ([]marketplace.PreviewChunk, error) {
			t.Fatal("PreviewKB should not be called when slug is held")
			return nil, nil
		},
	}
	resolver := func(_ context.Context, _, _ string) (marketplace.SlugStatus, error) {
		return marketplace.SlugStatus{IsHeld: true}, nil
	}
	r := newMarketplaceRouter(svc, resolver)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/marketplace/acme/old/preview", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d: %s", w.Code, w.Body.String())
	}
	var body apierror.AppError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ErrorCode != "slug_held" {
		t.Errorf("expected error_code=slug_held, got %q", body.ErrorCode)
	}
}
