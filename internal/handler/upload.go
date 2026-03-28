package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// UploadServicer is the interface the handler requires from the upload service layer.
type UploadServicer interface {
	Upload(ctx context.Context, params service.UploadParams) (*model.Document, error)
}

// UploadHandler handles HTTP requests for document uploads.
type UploadHandler struct {
	svc UploadServicer
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(svc UploadServicer) *UploadHandler {
	return &UploadHandler{svc: svc}
}

// Upload handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/upload.
//
// @Summary     Upload document to knowledge base
// @Description Upload a file to the specified knowledge base. Validates file type and size, checks for duplicates, and stores in SeaweedFS.
// @Tags        documents
// @Accept      multipart/form-data
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge base ID"
// @Param       file   formData file true "File to upload"
// @Success     201 {object} model.UploadDocumentResponse
// @Failure     400 {object} apierror.AppError
// @Failure     409 {object} apierror.AppError
// @Failure     413 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/upload [post]
func (h *UploadHandler) Upload(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	userID, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userID.(string)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  "missing or invalid file field: " + err.Error(),
		})
		c.Abort()
		return
	}
	defer file.Close()

	// Detect content type from file content.
	// Read first 512 bytes for MIME detection, then reconstruct full reader.
	buf := make([]byte, 512)
	n, readErr := file.Read(buf)
	if readErr != nil && readErr != io.EOF {
		_ = c.Error(apierror.NewInternal("failed to read file header: " + readErr.Error()))
		c.Abort()
		return
	}
	contentType := http.DetectContentType(buf[:n])

	// For text-based types, refine using the file extension since
	// DetectContentType returns "text/plain" for most text formats.
	contentType = refineContentType(contentType, header.Filename)

	// Reconstruct the reader: prepend the already-read bytes.
	combinedReader := io.MultiReader(
		byteReader(buf[:n]),
		file,
	)

	params := service.UploadParams{
		OrgID:           orgID,
		KnowledgeBaseID: kbID,
		FileName:        header.Filename,
		FileType:        contentType,
		FileSizeBytes:   header.Size,
		UploadedBy:      userIDStr,
		Reader:          combinedReader,
	}

	doc, err := h.svc.Upload(c.Request.Context(), params)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	resp := model.UploadDocumentResponse{
		ID:               doc.ID,
		OrgID:            doc.OrgID,
		KnowledgeBaseID:  doc.KnowledgeBaseID,
		FileName:         doc.FileName,
		FileType:         doc.FileType,
		FileSizeBytes:    doc.FileSizeBytes,
		FileHash:         doc.FileHash,
		StoragePath:      doc.StoragePath,
		ProcessingStatus: doc.ProcessingStatus,
		UploadedBy:       doc.UploadedBy,
		CreatedAt:        doc.CreatedAt,
	}
	c.JSON(http.StatusCreated, resp)
}

// byteReader wraps a byte slice in an io.Reader.
type byteReaderWrapper struct {
	data []byte
	pos  int
}

func byteReader(b []byte) io.Reader {
	return &byteReaderWrapper{data: b}
}

func (r *byteReaderWrapper) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// refineContentType adjusts the detected MIME type based on file extension
// for text-based formats that http.DetectContentType cannot distinguish.
func refineContentType(detected, filename string) string {
	// If DetectContentType already identified a non-text type, trust it.
	if detected != "text/plain; charset=utf-8" &&
		detected != "application/octet-stream" &&
		detected != "text/plain" {
		return detected
	}

	// Check file extension for known text-based types.
	lower := lowerFilename(filename)
	switch {
	case hasSuffix(lower, ".md"), hasSuffix(lower, ".markdown"):
		return "text/markdown"
	case hasSuffix(lower, ".html"), hasSuffix(lower, ".htm"):
		return "text/html"
	case hasSuffix(lower, ".csv"):
		return "text/csv"
	case hasSuffix(lower, ".txt"):
		return "text/plain"
	case hasSuffix(lower, ".pdf"):
		return "application/pdf"
	case hasSuffix(lower, ".docx"):
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case hasSuffix(lower, ".pptx"):
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	}
	return detected
}

func lowerFilename(name string) string {
	result := make([]byte, len(name))
	for i := range len(name) {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
