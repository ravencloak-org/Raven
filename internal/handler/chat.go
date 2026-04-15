package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ChatServicer is the interface the handler requires from the service layer.
type ChatServicer interface {
	StreamCompletion(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error)
	GetHistory(ctx context.Context, orgID, sessionID string, limit, offset int) (*model.HistoryResponse, error)
	ListSessions(ctx context.Context, orgID, kbID string, limit, offset int) (*model.SessionListResponse, error)
	DeleteSession(ctx context.Context, orgID, sessionID string) error
}

// ChatHandler handles HTTP requests for chat completions with SSE streaming.
type ChatHandler struct {
	svc ChatServicer
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(svc ChatServicer) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// StreamCompletion handles POST /v1/chat/:kb_id/completions.
//
// @Summary     Chat completion with SSE streaming
// @Description Accepts a chat query, calls the AI worker's RAG pipeline, and streams tokens back as Server-Sent Events.
// @Tags        chat
// @Accept      json
// @Produce     text/event-stream
// @Param       kb_id   path   string                    true "Knowledge base ID"
// @Param       request body   model.ChatCompletionRequest true "Chat completion request"
// @Success     200 {string} string "SSE event stream"
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /chat/{kb_id}/completions [post]
func (h *ChatHandler) StreamCompletion(c *gin.Context) {
	kbID := c.Param("kb_id")

	// Get org_id from context (set by API key middleware).
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	var req model.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	// Set SSE headers before starting the stream.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	eventCh, err := h.svc.StreamCompletion(c.Request.Context(), orgID, kbID, &req)
	if err != nil {
		// If we haven't started streaming yet, we can still return a JSON error.
		_ = c.Error(err)
		c.Abort()
		return
	}

	// Write status 200 explicitly before streaming.
	c.Status(http.StatusOK)

	// Stream SSE events to the client using a manual loop.
	// This avoids c.Stream() which requires http.CloseNotifier (not available
	// in httptest.ResponseRecorder).
	ctx := c.Request.Context()
	for {
		select {
		case event, open := <-eventCh:
			if !open {
				return // Channel closed, stream complete.
			}
			writeSSEEvent(c, event)
		case <-ctx.Done():
			return // Client disconnected.
		}
	}
}

// writeSSEEvent formats and writes a single SSE event to the response.
func writeSSEEvent(c *gin.Context, event service.SSEEvent) {
	data, err := json.Marshal(event.Data)
	if err != nil {
		data = []byte(fmt.Sprintf(`{"error":"marshal error: %s"}`, err.Error()))
	}

	// Write SSE format: "event: <type>\ndata: <json>\n\n"
	_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Event, data)
	c.Writer.Flush()
}

// GetHistory handles GET /v1/chat/:kb_id/sessions/:session_id/history.
// Returns paginated message history for a session.
//
// @Summary     Get conversation history
// @Description Returns paginated message history for a chat session.
// @Tags        chat
// @Produce     json
// @Param       kb_id      path  string true  "Knowledge base ID"
// @Param       session_id path  string true  "Session ID"
// @Param       limit      query int    false "Number of messages (default 50)"
// @Param       offset     query int    false "Offset (default 0)"
// @Success     200 {object} model.HistoryResponse
// @Failure     404 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /chat/{kb_id}/sessions/{session_id}/history [get]
func (h *ChatHandler) GetHistory(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	sessionID := c.Param("session_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	resp, err := h.svc.GetHistory(c.Request.Context(), orgID, sessionID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListSessions handles GET /v1/chat/:kb_id/sessions.
// Returns active sessions for the authenticated org.
//
// @Summary     List chat sessions
// @Description Returns active chat sessions for a knowledge base.
// @Tags        chat
// @Produce     json
// @Param       kb_id  path  string true  "Knowledge base ID"
// @Param       limit  query int    false "Number of sessions (default 20)"
// @Param       offset query int    false "Offset (default 0)"
// @Success     200 {object} model.SessionListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /chat/{kb_id}/sessions [get]
func (h *ChatHandler) ListSessions(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	kbID := c.Param("kb_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	resp, err := h.svc.ListSessions(c.Request.Context(), orgID, kbID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteSession handles DELETE /v1/chat/:kb_id/sessions/:session_id.
// Deletes a session and all its messages.
//
// @Summary     Delete chat session
// @Description Deletes a chat session and all associated messages.
// @Tags        chat
// @Param       kb_id      path string true "Knowledge base ID"
// @Param       session_id path string true "Session ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Router      /chat/{kb_id}/sessions/{session_id} [delete]
func (h *ChatHandler) DeleteSession(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	sessionID := c.Param("session_id")

	if err := h.svc.DeleteSession(c.Request.Context(), orgID, sessionID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// extractOrgID retrieves the org_id from the Gin context.
// Returns the orgID and true, or aborts with 401 and returns false.
func extractOrgID(c *gin.Context) (string, bool) {
	orgIDVal, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		_ = c.Error(apierror.NewUnauthorized("organisation context required"))
		c.Abort()
		return "", false
	}
	orgID, ok := orgIDVal.(string)
	if !ok || orgID == "" {
		_ = c.Error(apierror.NewUnauthorized("invalid organisation context"))
		c.Abort()
		return "", false
	}
	return orgID, true
}
