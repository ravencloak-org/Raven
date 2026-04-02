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

// StrangerServicer is the interface the handler requires from the stranger service layer.
type StrangerServicer interface {
	List(ctx context.Context, orgID string, status *model.StrangerStatus, limit, offset int) ([]model.StrangerUser, int, error)
	GetByID(ctx context.Context, orgID, id string) (*model.StrangerUser, error)
	Block(ctx context.Context, orgID, id, blockedBy string, req model.BlockStrangerRequest) error
	Unblock(ctx context.Context, orgID, id string) error
	SetRateLimit(ctx context.Context, orgID, id string, rpm *int) error
	Delete(ctx context.Context, orgID, id string) error
}

// StrangerListResponse wraps a paginated list of stranger records.
type StrangerListResponse struct {
	Strangers []model.StrangerUser `json:"strangers"`
	Total     int                  `json:"total"`
}

// StrangerHandler handles HTTP requests for stranger user management.
type StrangerHandler struct {
	svc StrangerServicer
}

// NewStrangerHandler creates a new StrangerHandler.
func NewStrangerHandler(svc StrangerServicer) *StrangerHandler {
	return &StrangerHandler{svc: svc}
}

// List handles GET /api/v1/orgs/:org_id/strangers.
//
// @Summary     List stranger users
// @Tags        strangers
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path  string false "Organisation ID"
// @Param       status  query string false "Filter by status (active|throttled|blocked|banned)"
// @Param       limit   query int    false "Results per page (default 50, max 200)"
// @Param       offset  query int    false "Offset for pagination"
// @Success     200 {object} StrangerListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/strangers [get]
func (h *StrangerHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var status *model.StrangerStatus
	if s := c.Query("status"); s != "" {
		ss := model.StrangerStatus(s)
		status = &ss
	}

	strangers, total, err := h.svc.List(c.Request.Context(), orgID, status, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, StrangerListResponse{Strangers: strangers, Total: total})
}

// Get handles GET /api/v1/orgs/:org_id/strangers/:id.
//
// @Summary     Get stranger user
// @Tags        strangers
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Stranger user ID"
// @Success     200 {object} model.StrangerUser
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/strangers/{id} [get]
func (h *StrangerHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	s, err := h.svc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, s)
}

// Block handles POST /api/v1/orgs/:org_id/strangers/:id/block.
//
// @Summary     Block or ban a stranger user
// @Tags        strangers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Stranger user ID"
// @Param       request body model.BlockStrangerRequest true "Block payload"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/strangers/{id}/block [post]
func (h *StrangerHandler) Block(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	userIDVal, exists := c.Get(string(middleware.ContextKeyUserID))
	if !exists {
		_ = c.Error(apierror.NewUnauthorized("user authentication required"))
		c.Abort()
		return
	}
	userID, _ := userIDVal.(string)

	var req model.BlockStrangerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	if err := h.svc.Block(c.Request.Context(), orgID, id, userID, req); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// Unblock handles POST /api/v1/orgs/:org_id/strangers/:id/unblock.
//
// @Summary     Unblock a stranger user
// @Tags        strangers
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Stranger user ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/strangers/{id}/unblock [post]
func (h *StrangerHandler) Unblock(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	if err := h.svc.Unblock(c.Request.Context(), orgID, id); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// SetRateLimit handles PUT /api/v1/orgs/:org_id/strangers/:id/rate-limit.
//
// @Summary     Override per-session rate limit (RPM)
// @Tags        strangers
// @Accept      json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Stranger user ID"
// @Param       request body model.SetRateLimitRequest true "Rate limit payload"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/strangers/{id}/rate-limit [put]
func (h *StrangerHandler) SetRateLimit(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	var req model.SetRateLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		return
	}

	if err := h.svc.SetRateLimit(c.Request.Context(), orgID, id, req.RPM); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// Delete handles DELETE /api/v1/orgs/:org_id/strangers/:id.
//
// @Summary     Delete a stranger user record
// @Tags        strangers
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Param       id     path string true "Stranger user ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/strangers/{id} [delete]
func (h *StrangerHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
