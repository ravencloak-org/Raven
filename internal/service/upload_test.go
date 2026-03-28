package service_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockStorageClient implements storage.Client for unit tests.
type mockStorageClient struct {
	uploadFn   func(ctx context.Context, filename string, reader io.Reader) (string, error)
	downloadFn func(ctx context.Context, fid string) (io.ReadCloser, error)
	deleteFn   func(ctx context.Context, fid string) error
}

func (m *mockStorageClient) Upload(ctx context.Context, filename string, reader io.Reader) (string, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, filename, reader)
	}
	return "3,test-fid", nil
}

func (m *mockStorageClient) Download(ctx context.Context, fid string) (io.ReadCloser, error) {
	if m.downloadFn != nil {
		return m.downloadFn(ctx, fid)
	}
	return io.NopCloser(strings.NewReader("content")), nil
}

func (m *mockStorageClient) Delete(ctx context.Context, fid string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, fid)
	}
	return nil
}

func TestUploadService_ValidateFileType_Rejected(t *testing.T) {
	store := &mockStorageClient{}
	svc := service.NewUploadService(nil, nil, store, 50*1024*1024, []string{
		"application/pdf",
		"text/plain",
	})

	_, err := svc.Upload(context.Background(), service.UploadParams{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		FileName:        "image.png",
		FileType:        "image/png",
		FileSizeBytes:   1024,
		Reader:          strings.NewReader("fake png"),
	})
	if err == nil {
		t.Fatal("expected error for disallowed type")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected 400, got %d", appErr.Code)
	}
	if !strings.Contains(appErr.Detail, "image/png") {
		t.Errorf("expected detail to contain 'image/png', got: %s", appErr.Detail)
	}
}

func TestUploadService_ValidateFileSize_Rejected(t *testing.T) {
	store := &mockStorageClient{}
	svc := service.NewUploadService(nil, nil, store, 1024, []string{"text/plain"})

	_, err := svc.Upload(context.Background(), service.UploadParams{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		FileName:        "big.txt",
		FileType:        "text/plain",
		FileSizeBytes:   2048,
		Reader:          strings.NewReader("big content"),
	})
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected 400, got %d", appErr.Code)
	}
	if !strings.Contains(appErr.Detail, "too large") {
		t.Errorf("expected detail to contain 'too large', got: %s", appErr.Detail)
	}
}

func TestUploadService_AllowedType_CaseInsensitive(t *testing.T) {
	store := &mockStorageClient{}

	// Configure with mixed-case type.
	svc := service.NewUploadService(nil, nil, store, 50*1024*1024, []string{
		"Application/PDF",
	})

	// Uppercase type should be rejected (not in the normalised map), confirming
	// that the map stores lowercase.
	_, err := svc.Upload(context.Background(), service.UploadParams{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		FileName:        "image.png",
		FileType:        "IMAGE/PNG",
		FileSizeBytes:   100,
		Reader:          strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error for disallowed type")
	}
	appErr, ok := err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	if appErr.Code != 400 {
		t.Errorf("expected 400, got %d", appErr.Code)
	}

	// Lowercase of the configured type should NOT be rejected at the type check.
	// It will fail later (nil pool), but verifying we don't get 400 for the type.
	_, err = svc.Upload(context.Background(), service.UploadParams{
		OrgID:           "org-1",
		KnowledgeBaseID: "kb-1",
		FileName:        "test.pdf",
		FileType:        "application/pdf",
		FileSizeBytes:   50*1024*1024 + 1, // intentionally over limit to fail early
		Reader:          strings.NewReader("pdf data"),
	})
	if err == nil {
		t.Fatal("expected error (size limit)")
	}
	appErr, ok = err.(*apierror.AppError)
	if !ok {
		t.Fatalf("expected *apierror.AppError, got %T", err)
	}
	// Should fail on size, not type -- confirming case-insensitive type acceptance.
	if !strings.Contains(appErr.Detail, "too large") {
		t.Errorf("expected size error, got: %s", appErr.Detail)
	}
}
