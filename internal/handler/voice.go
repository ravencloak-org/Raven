package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// VoiceServicer is the interface the handler requires from the service layer.
type VoiceServicer interface {
	CreateSession(ctx context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error)
	GetSession(ctx context.Context, orgID, sessionID string) (*model.VoiceSession, error)
	UpdateSessionState(ctx context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error)
	ListSessions(ctx context.Context, orgID string, limit, offset int) (*model.VoiceSessionListResponse, error)
	AppendTurn(ctx context.Context, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error)
	ListTurns(ctx context.Context, orgID, sessionID string) (*model.VoiceTurnListResponse, error)
}

// VoiceHandler handles HTTP requests for voice session lifecycle and transcription.
type VoiceHandler struct {
	svc VoiceServicer
}

// NewVoiceHandler creates a new VoiceHandler.
func NewVoiceHandler(svc VoiceServicer) *VoiceHandler {
	return &VoiceHandler{svc: svc}
}

// CreateSession handles POST /v1/orgs/:org_id/voice-sessions.
//
// @Summary     Create voice session
// @Description Creates a new voice session in the 'created' state.
// @Tags        voice
// @Accept      json
// @Produce     json
// @Param       org_id  path   string                          true "Organisation ID"
// @Param       request body   model.CreateVoiceSessionRequest true "Create session request"
// @Success     201 {object} model.VoiceSession
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/voice-sessions [post]
func (h *VoiceHandler) CreateSession(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	var req model.CreateVoiceSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	session, err := h.svc.CreateSession(c.Request.Context(), orgID, &req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, session)
}

// GetSession handles GET /v1/orgs/:org_id/voice-sessions/:session_id.
//
// @Summary     Get voice session
// @Description Returns a voice session by ID.
// @Tags        voice
// @Produce     json
// @Param       org_id     path string true "Organisation ID"
// @Param       session_id path string true "Session ID"
// @Success     200 {object} model.VoiceSession
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/voice-sessions/{session_id} [get]
func (h *VoiceHandler) GetSession(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	sessionID := c.Param("session_id")
	session, err := h.svc.GetSession(c.Request.Context(), orgID, sessionID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, session)
}

// UpdateSessionState handles PATCH /v1/orgs/:org_id/voice-sessions/:session_id.
//
// @Summary     Update voice session state
// @Description Transitions a voice session to 'active' or 'ended'.
// @Tags        voice
// @Accept      json
// @Produce     json
// @Param       org_id     path   string                               true "Organisation ID"
// @Param       session_id path   string                               true "Session ID"
// @Param       request    body   model.UpdateVoiceSessionStateRequest true "State transition"
// @Success     200 {object} model.VoiceSession
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/voice-sessions/{session_id} [patch]
func (h *VoiceHandler) UpdateSessionState(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	sessionID := c.Param("session_id")

	var req model.UpdateVoiceSessionStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	session, err := h.svc.UpdateSessionState(c.Request.Context(), orgID, sessionID, req.State)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, session)
}

// ListSessions handles GET /v1/orgs/:org_id/voice-sessions.
//
// @Summary     List voice sessions
// @Description Returns org-scoped voice sessions with pagination.
// @Tags        voice
// @Produce     json
// @Param       org_id path  string true  "Organisation ID"
// @Param       limit  query int    false "Number of sessions (default 20)"
// @Param       offset query int    false "Offset (default 0)"
// @Success     200 {object} model.VoiceSessionListResponse
// @Failure     401 {object} apierror.AppError
// @Failure     500 {object} apierror.AppError
// @Router      /orgs/{org_id}/voice-sessions [get]
func (h *VoiceHandler) ListSessions(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.svc.ListSessions(c.Request.Context(), orgID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// AppendTurn handles POST /v1/orgs/:org_id/voice-sessions/:session_id/turns.
//
// @Summary     Append voice turn
// @Description Stores a transcribed turn for an existing voice session.
// @Tags        voice
// @Accept      json
// @Produce     json
// @Param       org_id     path   string                      true "Organisation ID"
// @Param       session_id path   string                      true "Session ID"
// @Param       request    body   model.AppendVoiceTurnRequest true "Turn data"
// @Success     201 {object} model.VoiceTurn
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/voice-sessions/{session_id}/turns [post]
func (h *VoiceHandler) AppendTurn(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	sessionID := c.Param("session_id")

	var req model.AppendVoiceTurnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	turn, err := h.svc.AppendTurn(c.Request.Context(), orgID, sessionID, &req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, turn)
}

// ListTurns handles GET /v1/orgs/:org_id/voice-sessions/:session_id/turns.
//
// @Summary     List voice turns
// @Description Returns all transcription turns for a session ordered by started_at.
// @Tags        voice
// @Produce     json
// @Param       org_id     path string true "Organisation ID"
// @Param       session_id path string true "Session ID"
// @Success     200 {object} model.VoiceTurnListResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/voice-sessions/{session_id}/turns [get]
func (h *VoiceHandler) ListTurns(c *gin.Context) {
	orgID, ok := extractOrgID(c)
	if !ok {
		return
	}

	sessionID := c.Param("session_id")

	resp, err := h.svc.ListTurns(c.Request.Context(), orgID, sessionID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}
