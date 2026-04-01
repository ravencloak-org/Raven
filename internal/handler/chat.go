package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ChatServicer is the interface the handler requires from the service layer.
type ChatServicer interface {
	StreamCompletion(ctx context.Context, orgID, kbID string, req *model.ChatCompletionRequest) (<-chan service.SSEEvent, error)
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
	orgIDVal, exists := c.Get(string(middleware.ContextKeyOrgID))
	if !exists {
		_ = c.Error(apierror.NewUnauthorized("organisation context required"))
		c.Abort()
		return
	}
	orgID, ok := orgIDVal.(string)
	if !ok || orgID == "" {
		_ = c.Error(apierror.NewUnauthorized("invalid organisation context"))
		c.Abort()
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
