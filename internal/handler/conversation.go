package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ConversationServicer is the slice of ConversationService the handler needs.
type ConversationServicer interface {
	ListForUser(ctx context.Context, orgID, kbID, userID string, limit, offset int) (*model.ConversationListResponse, error)
	GetTranscript(ctx context.Context, orgID, kbID, sessionID, userID string) (*model.ConversationSession, error)
}

// ConversationHandler exposes cross-channel conversation history.
type ConversationHandler struct {
	svc ConversationServicer
}

// NewConversationHandler creates a new ConversationHandler.
func NewConversationHandler(svc ConversationServicer) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

// List handles GET /api/v1/orgs/:org_id/kbs/:kb_id/conversations.
// Returns the authenticated user's paginated session history for the KB.
//
// @Summary     List user's conversation sessions for a KB
// @Tags        conversations
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path  string true  "Organisation ID"
// @Param       kb_id  path  string true  "Knowledge base ID"
// @Param       limit  query int    false "Page size (default 20, max 100)"
// @Param       offset query int    false "Offset (default 0)"
// @Success     200 {object} model.ConversationListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/kbs/{kb_id}/conversations [get]
func (h *ConversationHandler) List(c *gin.Context) {
	orgID, userID, ok := extractOrgAndUser(c)
	if !ok {
		return
	}
	kbID := c.Param("kb_id")
	if kbID == "" {
		_ = c.Error(apierror.NewBadRequest("kb_id is required"))
		c.Abort()
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	// Mirror the clamp in internal/handler/chat.go GetHistory/ListSessions so
	// callers get defensive pagination at the HTTP edge as well as the repo.
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	resp, err := h.svc.ListForUser(c.Request.Context(), orgID, kbID, userID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/v1/orgs/:org_id/kbs/:kb_id/conversations/:session_id.
// Returns the full transcript for a session owned by the authenticated user.
//
// @Summary     Get conversation transcript
// @Tags        conversations
// @Produce     json
// @Security    BearerAuth
// @Param       org_id     path string true "Organisation ID"
// @Param       kb_id      path string true "Knowledge base ID"
// @Param       session_id path string true "Session ID"
// @Success     200 {object} model.ConversationSession
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/kbs/{kb_id}/conversations/{session_id} [get]
func (h *ConversationHandler) Get(c *gin.Context) {
	orgID, userID, ok := extractOrgAndUser(c)
	if !ok {
		return
	}
	kbID := c.Param("kb_id")
	sessionID := c.Param("session_id")
	if kbID == "" || sessionID == "" {
		_ = c.Error(apierror.NewBadRequest("kb_id and session_id are required"))
		c.Abort()
		return
	}

	sess, err := h.svc.GetTranscript(c.Request.Context(), orgID, kbID, sessionID, userID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, sess)
}

// extractOrgAndUser reads orgID and the authenticated user's stable
// identifier (JWT `sub` claim, stored in ContextKeyExternalID by SuperTokens)
// from the Gin context. Responds with 401 when either is missing.
func extractOrgAndUser(c *gin.Context) (orgID, userID string, ok bool) {
	orgID, ok = extractOrgID(c)
	if !ok {
		return "", "", false
	}
	userID = c.GetString(string(middleware.ContextKeyExternalID))
	if userID == "" {
		_ = c.Error(apierror.NewUnauthorized("authenticated user required"))
		c.Abort()
		return "", "", false
	}
	return orgID, userID, true
}
