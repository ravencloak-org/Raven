package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// DocumentServicer is the interface the handler requires from the service layer.
type DocumentServicer interface {
	GetByID(ctx context.Context, orgID, docID string) (*model.Document, error)
	List(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.DocumentListResponse, error)
	Update(ctx context.Context, orgID, docID string, req model.UpdateDocumentRequest) (*model.Document, error)
	Delete(ctx context.Context, orgID, docID string) error
	UpdateStatus(ctx context.Context, orgID, docID string, newStatus model.ProcessingStatus, errorMsg string) error
}

// DocumentHandler handles HTTP requests for document management.
type DocumentHandler struct {
	svc DocumentServicer
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(svc DocumentServicer) *DocumentHandler {
	return &DocumentHandler{svc: svc}
}

// List handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents.
//
// @Summary     List documents in a knowledge base
// @Tags        documents
// @Produce     json
// @Security    BearerAuth
// @Param       org_id    path  string true "Organisation ID"
// @Param       ws_id     path  string true "Workspace ID"
// @Param       kb_id     path  string true "Knowledge Base ID"
// @Param       page      query int    false "Page number (default 1)"
// @Param       page_size query int    false "Page size (default 20, max 100)"
// @Success     200 {object} model.DocumentListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents [get]
func (h *DocumentHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	kbID := c.Param("kb_id")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.svc.List(c.Request.Context(), orgID, kbID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/:doc_id.
//
// @Summary     Get document by ID
// @Tags        documents
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge Base ID"
// @Param       doc_id path string true "Document ID"
// @Success     200 {object} model.Document
// @Failure     404 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/{doc_id} [get]
func (h *DocumentHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	docID := c.Param("doc_id")

	doc, err := h.svc.GetByID(c.Request.Context(), orgID, docID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, doc)
}

// Update handles PUT /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/:doc_id.
//
// @Summary     Update document metadata
// @Tags        documents
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string                      true "Organisation ID"
// @Param       ws_id   path string                      true "Workspace ID"
// @Param       kb_id   path string                      true "Knowledge Base ID"
// @Param       doc_id  path string                      true "Document ID"
// @Param       request body model.UpdateDocumentRequest  true "Document update payload"
// @Success     200 {object} model.Document
// @Failure     422 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/{doc_id} [put]
func (h *DocumentHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	docID := c.Param("doc_id")

	var req model.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	doc, err := h.svc.Update(c.Request.Context(), orgID, docID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, doc)
}

// Delete handles DELETE /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/:doc_id.
//
// @Summary     Delete document
// @Tags        documents
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge Base ID"
// @Param       doc_id path string true "Document ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/{doc_id} [delete]
func (h *DocumentHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	docID := c.Param("doc_id")

	if err := h.svc.Delete(c.Request.Context(), orgID, docID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
