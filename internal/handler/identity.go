package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// IdentityServicer is the interface the handler requires from the service layer.
type IdentityServicer interface {
	Identify(ctx context.Context, orgID string, req model.IdentifyRequest) (*model.UserIdentity, error)
	Track(ctx context.Context, orgID string, req model.TrackEventRequest) error
	List(ctx context.Context, orgID string, limit, offset int) (*model.IdentityListResponse, error)
	Delete(ctx context.Context, orgID, id string) error
}

// IdentityHandler handles HTTP requests for cross-channel identity management.
type IdentityHandler struct {
	svc IdentityServicer
}

// NewIdentityHandler creates a new IdentityHandler.
func NewIdentityHandler(svc IdentityServicer) *IdentityHandler {
	return &IdentityHandler{svc: svc}
}

// Identify handles POST /api/v1/orgs/:org_id/identity.
// It upserts an identity record and links an anonymous session to a user when user_id is provided.
//
// @Summary     Identify / link anonymous session to user
// @Tags        identity
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.IdentifyRequest true "Identity payload"
// @Success     200 {object} model.UserIdentity
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/identity [post]
func (h *IdentityHandler) Identify(c *gin.Context) {
	orgID := c.Param("org_id")

	var req model.IdentifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	identity, err := h.svc.Identify(c.Request.Context(), orgID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, identity)
}

// Track handles POST /api/v1/orgs/:org_id/identity/track.
// It forwards a custom event to PostHog via Capture.
//
// @Summary     Track a PostHog event
// @Tags        identity
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.TrackEventRequest true "Event payload"
// @Success     204
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/identity/track [post]
func (h *IdentityHandler) Track(c *gin.Context) {
	orgID := c.Param("org_id")

	var req model.TrackEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	if err := h.svc.Track(c.Request.Context(), orgID, req); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// ListIdentities handles GET /api/v1/orgs/:org_id/identity.
//
// @Summary     List user identities (paginated)
// @Tags        identity
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       limit  query int false "Results per page (default 50, max 200)"
// @Param       offset query int false "Pagination offset"
// @Success     200 {object} model.IdentityListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/identity [get]
func (h *IdentityHandler) ListIdentities(c *gin.Context) {
	orgID := c.Param("org_id")

	limit := 50
	if lStr := c.Query("limit"); lStr != "" {
		v, err := strconv.Atoi(lStr)
		if err != nil || v < 1 || v > 200 {
			c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
				Code:    http.StatusBadRequest,
				Message: "Bad Request",
				Detail:  "limit must be an integer between 1 and 200",
			})
			return
		}
		limit = v
	}
	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		v, err := strconv.Atoi(oStr)
		if err != nil || v < 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
				Code:    http.StatusBadRequest,
				Message: "Bad Request",
				Detail:  "offset must be a non-negative integer",
			})
			return
		}
		offset = v
	}

	resp, err := h.svc.List(c.Request.Context(), orgID, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteIdentity handles DELETE /api/v1/orgs/:org_id/identity/:id.
//
// @Summary     Delete an identity record
// @Tags        identity
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Identity ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/identity/{id} [delete]
func (h *IdentityHandler) DeleteIdentity(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
