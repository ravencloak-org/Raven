package handler

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// sanitizeCSVCell neutralizes CSV injection by prefixing cells that start with
// formula-triggering characters (=, +, -, @, tab, carriage-return) with a single
// quote, preventing spreadsheet applications from interpreting cell content as formulas.
func sanitizeCSVCell(s string) string {
	if len(s) > 0 {
		switch s[0] {
		case '=', '+', '-', '@', '\t', '\r':
			return "'" + s
		}
	}
	return s
}

// LeadServicer is the interface the handler requires from the service layer.
type LeadServicer interface {
	Upsert(ctx context.Context, orgID string, req model.UpsertLeadRequest) (*model.LeadProfile, error)
	GetByID(ctx context.Context, orgID, id string) (*model.LeadProfile, error)
	List(ctx context.Context, orgID string, minScore *float32, limit, offset int) (*model.LeadListResponse, error)
	Update(ctx context.Context, orgID, id string, req model.UpdateLeadRequest) (*model.LeadProfile, error)
	Delete(ctx context.Context, orgID, id string) error
	ExportCSV(ctx context.Context, orgID string) ([]model.LeadProfile, error)
}

// LeadHandler handles HTTP requests for lead profile management.
type LeadHandler struct {
	svc LeadServicer
}

// NewLeadHandler creates a new LeadHandler.
func NewLeadHandler(svc LeadServicer) *LeadHandler {
	return &LeadHandler{svc: svc}
}

// UpsertLead handles POST /api/v1/orgs/:org_id/leads.
//
// @Summary     Upsert lead profile
// @Tags        leads
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.UpsertLeadRequest true "Lead profile payload"
// @Success     200 {object} model.LeadProfile
// @Failure     400 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/leads [post]
func (h *LeadHandler) UpsertLead(c *gin.Context) {
	orgID := c.Param("org_id")
	var req model.UpsertLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	lead, err := h.svc.Upsert(c.Request.Context(), orgID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, lead)
}

// ListLeads handles GET /api/v1/orgs/:org_id/leads.
//
// @Summary     List lead profiles (paginated, sorted by engagement score)
// @Tags        leads
// @Produce     json
// @Security    BearerAuth
// @Param       org_id     path  string  true  "Organisation ID"
// @Param       page       query int     false "Page number (default 1)"
// @Param       limit      query int     false "Page size (default 20, max 100)"
// @Param       min_score  query number  false "Minimum engagement score filter"
// @Success     200 {object} model.LeadListResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/leads [get]
func (h *LeadHandler) ListLeads(c *gin.Context) {
	orgID := c.Param("org_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var minScore *float32
	if msStr := c.Query("min_score"); msStr != "" {
		if ms, err := strconv.ParseFloat(msStr, 32); err == nil {
			ms32 := float32(ms)
			minScore = &ms32
		}
	}

	resp, err := h.svc.List(c.Request.Context(), orgID, minScore, limit, offset)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetLead handles GET /api/v1/orgs/:org_id/leads/:id.
//
// @Summary     Get lead profile by ID
// @Tags        leads
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Lead profile ID"
// @Success     200 {object} model.LeadProfile
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/leads/{id} [get]
func (h *LeadHandler) GetLead(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	lead, err := h.svc.GetByID(c.Request.Context(), orgID, id)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, lead)
}

// UpdateLead handles PUT /api/v1/orgs/:org_id/leads/:id.
//
// @Summary     Update lead profile
// @Tags        leads
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Lead profile ID"
// @Param       request body model.UpdateLeadRequest true "Lead profile update payload"
// @Success     200 {object} model.LeadProfile
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/leads/{id} [put]
func (h *LeadHandler) UpdateLead(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	var req model.UpdateLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	lead, err := h.svc.Update(c.Request.Context(), orgID, id, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, lead)
}

// DeleteLead handles DELETE /api/v1/orgs/:org_id/leads/:id.
//
// @Summary     Delete lead profile
// @Tags        leads
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       id      path string true "Lead profile ID"
// @Success     204
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/leads/{id} [delete]
func (h *LeadHandler) DeleteLead(c *gin.Context) {
	orgID := c.Param("org_id")
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), orgID, id); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// ExportLeadsCSV handles GET /api/v1/orgs/:org_id/leads/export.
//
// @Summary     Export leads as CSV for CRM integration
// @Tags        leads
// @Produce     text/csv
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Success     200
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/leads/export [get]
func (h *LeadHandler) ExportLeadsCSV(c *gin.Context) {
	orgID := c.Param("org_id")

	leads, err := h.svc.ExportCSV(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=leads.csv")

	w := csv.NewWriter(c.Writer)
	// Header row.
	if err := w.Write([]string{
		"id", "org_id", "knowledge_base_id",
		"email", "name", "phone", "company",
		"engagement_score", "total_messages", "total_sessions",
		"first_seen_at", "last_seen_at", "created_at", "updated_at",
	}); err != nil {
		return
	}
	for i := range leads {
		l := &leads[i]
		if err := w.Write([]string{
			l.ID, l.OrgID, l.KnowledgeBaseID,
			sanitizeCSVCell(l.Email),
			sanitizeCSVCell(l.Name),
			sanitizeCSVCell(l.Phone),
			sanitizeCSVCell(l.Company),
			fmt.Sprintf("%.2f", l.EngagementScore),
			strconv.Itoa(l.TotalMessages),
			strconv.Itoa(l.TotalSessions),
			l.FirstSeenAt.String(),
			l.LastSeenAt.String(),
			l.CreatedAt.String(),
			l.UpdatedAt.String(),
		}); err != nil {
			return
		}
	}
	w.Flush()
}
