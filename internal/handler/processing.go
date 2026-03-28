package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ProcessingEventServicer is the interface the handler requires from the service layer.
type ProcessingEventServicer interface {
	Transition(ctx context.Context, orgID, docID string, toStatus model.ProcessingStatus, errorMsg string) (*model.ProcessingEvent, error)
	ListByDocumentID(ctx context.Context, orgID, docID string) (*model.ProcessingEventListResponse, error)
}

// ProcessingEventHandler handles HTTP requests for processing event management.
type ProcessingEventHandler struct {
	svc ProcessingEventServicer
}

// NewProcessingEventHandler creates a new ProcessingEventHandler.
func NewProcessingEventHandler(svc ProcessingEventServicer) *ProcessingEventHandler {
	return &ProcessingEventHandler{svc: svc}
}

// transitionRequest is the payload for POST .../documents/:doc_id/transitions.
type transitionRequest struct {
	ToStatus     model.ProcessingStatus `json:"to_status" binding:"required"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

// ListEvents handles GET /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/:doc_id/events.
//
// @Summary     List processing events for a document
// @Tags        processing
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       ws_id  path string true "Workspace ID"
// @Param       kb_id  path string true "Knowledge Base ID"
// @Param       doc_id path string true "Document ID"
// @Success     200 {object} model.ProcessingEventListResponse
// @Failure     404 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/{doc_id}/events [get]
func (h *ProcessingEventHandler) ListEvents(c *gin.Context) {
	orgID := c.Param("org_id")
	docID := c.Param("doc_id")

	resp, err := h.svc.ListByDocumentID(c.Request.Context(), orgID, docID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Transition handles POST /api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/:doc_id/transitions.
//
// @Summary     Trigger a processing status transition for a document
// @Tags        processing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string            true "Organisation ID"
// @Param       ws_id   path string            true "Workspace ID"
// @Param       kb_id   path string            true "Knowledge Base ID"
// @Param       doc_id  path string            true "Document ID"
// @Param       request body transitionRequest  true "Transition payload"
// @Success     201 {object} model.ProcessingEvent
// @Failure     400 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/workspaces/{ws_id}/knowledge-bases/{kb_id}/documents/{doc_id}/transitions [post]
func (h *ProcessingEventHandler) Transition(c *gin.Context) {
	orgID := c.Param("org_id")
	docID := c.Param("doc_id")

	var req transitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	evt, err := h.svc.Transition(c.Request.Context(), orgID, docID, req.ToStatus, req.ErrorMessage)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, evt)
}
