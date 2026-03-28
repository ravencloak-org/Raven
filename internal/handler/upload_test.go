package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockUploadService implements handler.UploadServicer for unit tests.
type mockUploadService struct {
	uploadFn func(ctx context.Context, params service.UploadParams) (*model.Document, error)
}

func (m *mockUploadService) Upload(ctx context.Context, params service.UploadParams) (*model.Document, error) {
	return m.uploadFn(ctx, params)
}

func newUploadRouter(svc handler.UploadServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "member")
		c.Next()
	})
	h := handler.NewUploadHandler(svc)
	r.POST("/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/upload", h.Upload)
	return r
}

func createMultipartRequest(t *testing.T, filename, content string) (*http.Request, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	_ = writer.Close()

	req, err := http.NewRequest(http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/upload",
		body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

func TestUpload_Success(t *testing.T) {
	svc := &mockUploadService{
		uploadFn: func(_ context.Context, params service.UploadParams) (*model.Document, error) {
			return &model.Document{
				ID:               "doc-1",
				OrgID:            params.OrgID,
				KnowledgeBaseID:  params.KnowledgeBaseID,
				FileName:         params.FileName,
				FileType:         params.FileType,
				FileSizeBytes:    params.FileSizeBytes,
				FileHash:         "abc123",
				StoragePath:      "3,01637037d6",
				ProcessingStatus: model.ProcessingStatusQueued,
				UploadedBy:       params.UploadedBy,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}, nil
		},
	}
	r := newUploadRouter(svc)
	req, _ := createMultipartRequest(t, "test.pdf", "fake pdf content")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.UploadDocumentResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", resp.ID)
	}
	if resp.StoragePath != "3,01637037d6" {
		t.Errorf("expected storage_path '3,01637037d6', got '%s'", resp.StoragePath)
	}
}

func TestUpload_MissingFile_Returns422(t *testing.T) {
	svc := &mockUploadService{}
	r := newUploadRouter(svc)

	req, _ := http.NewRequest(http.MethodPost,
		"/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/upload",
		nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_ServiceBadRequest_Returns400(t *testing.T) {
	svc := &mockUploadService{
		uploadFn: func(_ context.Context, _ service.UploadParams) (*model.Document, error) {
			return nil, apierror.NewBadRequest("file type not allowed: image/png")
		},
	}
	r := newUploadRouter(svc)
	req, _ := createMultipartRequest(t, "image.png", "fake png content")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_Duplicate_Returns409(t *testing.T) {
	svc := &mockUploadService{
		uploadFn: func(_ context.Context, _ service.UploadParams) (*model.Document, error) {
			return nil, &apierror.AppError{
				Code:    409,
				Message: "Conflict",
				Detail:  "duplicate file",
			}
		},
	}
	r := newUploadRouter(svc)
	req, _ := createMultipartRequest(t, "test.pdf", "content")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 409 {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_ServiceError_Returns500(t *testing.T) {
	svc := &mockUploadService{
		uploadFn: func(_ context.Context, _ service.UploadParams) (*model.Document, error) {
			return nil, apierror.NewInternal("storage failure")
		},
	}
	r := newUploadRouter(svc)
	req, _ := createMultipartRequest(t, "test.pdf", "content")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_ContextParams(t *testing.T) {
	// Verify that org_id, kb_id, and user_id are passed correctly to the service.
	var capturedParams service.UploadParams
	svc := &mockUploadService{
		uploadFn: func(_ context.Context, params service.UploadParams) (*model.Document, error) {
			capturedParams = params
			return &model.Document{
				ID:               "doc-2",
				OrgID:            params.OrgID,
				KnowledgeBaseID:  params.KnowledgeBaseID,
				FileName:         params.FileName,
				FileType:         params.FileType,
				FileSizeBytes:    params.FileSizeBytes,
				ProcessingStatus: model.ProcessingStatusQueued,
				UploadedBy:       params.UploadedBy,
				CreatedAt:        time.Now(),
			}, nil
		},
	}
	r := newUploadRouter(svc)
	req, _ := createMultipartRequest(t, "notes.md", "# Hello")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	if capturedParams.OrgID != "org-abc" {
		t.Errorf("expected OrgID 'org-abc', got '%s'", capturedParams.OrgID)
	}
	if capturedParams.KnowledgeBaseID != "kb-1" {
		t.Errorf("expected KnowledgeBaseID 'kb-1', got '%s'", capturedParams.KnowledgeBaseID)
	}
	if capturedParams.UploadedBy != "user-1" {
		t.Errorf("expected UploadedBy 'user-1', got '%s'", capturedParams.UploadedBy)
	}
	if capturedParams.FileName != "notes.md" {
		t.Errorf("expected FileName 'notes.md', got '%s'", capturedParams.FileName)
	}
	fmt.Println("Detected content type:", capturedParams.FileType)
}
