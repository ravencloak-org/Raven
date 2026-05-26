package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// LLMProviderServicer is the interface the handler requires from the service layer.
type LLMProviderServicer interface {
	Create(ctx context.Context, orgID, userID string, req model.CreateLLMProviderRequest) (*model.LLMProviderResponse, error)
	GetByID(ctx context.Context, orgID, configID string) (*model.LLMProviderResponse, error)
	List(ctx context.Context, orgID string) ([]model.LLMProviderResponse, error)
	Update(ctx context.Context, orgID, configID string, req model.UpdateLLMProviderRequest) (*model.LLMProviderResponse, error)
	Delete(ctx context.Context, orgID, configID string) error
	SetDefault(ctx context.Context, orgID, configID string) error
	TestConnection(ctx context.Context, orgID string, req model.TestProviderRequest) (*model.TestConnectionResult, error)
}

// LLMProviderHandler handles HTTP requests for LLM provider config management.
type LLMProviderHandler struct {
	svc LLMProviderServicer
}

// NewLLMProviderHandler creates a new LLMProviderHandler.
func NewLLMProviderHandler(svc LLMProviderServicer) *LLMProviderHandler {
	return &LLMProviderHandler{svc: svc}
}

// Create handles POST /api/v1/orgs/:org_id/llm-providers.
//
// @Summary     Create LLM provider config
// @Description Store a new LLM provider configuration with encrypted API key
// @Tags        llm-providers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateLLMProviderRequest true "LLM provider config payload"
// @Success     201 {object} model.LLMProviderResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers [post]
// TestConnection handles POST /api/v1/orgs/:org_id/llm-providers/test.
//
// Probes the supplied provider credentials WITHOUT persisting anything.
// The dialog uses this as a green-light gate before letting the user
// click Create, so a bad key / unreachable Base URL fails loudly here
// instead of inside the chat pipeline minutes later.
//
// @Summary     Test LLM provider connectivity
// @Tags        llm-providers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.CreateLLMProviderRequest true "Payload to probe (same shape as Create)"
// @Success     200 {object} service.TestConnectionResult
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers/test [post]
func (h *LLMProviderHandler) TestConnection(c *gin.Context) {
	var req model.CreateLLMProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}
	result, err := service.TestConnection(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(apierror.NewInternal("provider probe failed: " + err.Error()))
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *LLMProviderHandler) Create(c *gin.Context) {
	orgID := c.Param("org_id")
	userID, _ := c.Get(string(middleware.ContextKeyUserID))
	userIDStr, _ := userID.(string)

	var req model.CreateLLMProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	resp, err := h.svc.Create(c.Request.Context(), orgID, userIDStr, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// List handles GET /api/v1/orgs/:org_id/llm-providers.
//
// @Summary     List LLM provider configs
// @Description List all LLM provider configurations for an organisation
// @Tags        llm-providers
// @Produce     json
// @Security    BearerAuth
// @Param       org_id path string true "Organisation ID"
// @Success     200 {array} model.LLMProviderResponse
// @Failure     401 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers [get]
func (h *LLMProviderHandler) List(c *gin.Context) {
	orgID := c.Param("org_id")
	providers, err := h.svc.List(c.Request.Context(), orgID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	providers = lo.Ternary(providers == nil, []model.LLMProviderResponse{}, providers)
	c.JSON(http.StatusOK, providers)
}

// Get handles GET /api/v1/orgs/:org_id/llm-providers/:provider_id.
//
// @Summary     Get LLM provider config
// @Description Get a single LLM provider configuration by ID
// @Tags        llm-providers
// @Produce     json
// @Security    BearerAuth
// @Param       org_id      path string true "Organisation ID"
// @Param       provider_id path string true "Provider config ID"
// @Success     200 {object} model.LLMProviderResponse
// @Failure     401 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers/{provider_id} [get]
func (h *LLMProviderHandler) Get(c *gin.Context) {
	orgID := c.Param("org_id")
	providerID := c.Param("provider_id")
	resp, err := h.svc.GetByID(c.Request.Context(), orgID, providerID)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Update handles PUT /api/v1/orgs/:org_id/llm-providers/:provider_id.
//
// @Summary     Update LLM provider config
// @Description Update an existing LLM provider configuration. If a new API key is provided it will be re-encrypted.
// @Tags        llm-providers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id      path string true "Organisation ID"
// @Param       provider_id path string true "Provider config ID"
// @Param       request     body model.UpdateLLMProviderRequest true "Update payload"
// @Success     200 {object} model.LLMProviderResponse
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers/{provider_id} [put]
func (h *LLMProviderHandler) Update(c *gin.Context) {
	orgID := c.Param("org_id")
	providerID := c.Param("provider_id")

	var req model.UpdateLLMProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	resp, err := h.svc.Update(c.Request.Context(), orgID, providerID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/orgs/:org_id/llm-providers/:provider_id.
//
// @Summary     Delete LLM provider config
// @Description Remove an LLM provider configuration
// @Tags        llm-providers
// @Security    BearerAuth
// @Param       org_id      path string true "Organisation ID"
// @Param       provider_id path string true "Provider config ID"
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers/{provider_id} [delete]
func (h *LLMProviderHandler) Delete(c *gin.Context) {
	orgID := c.Param("org_id")
	providerID := c.Param("provider_id")
	if err := h.svc.Delete(c.Request.Context(), orgID, providerID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}

// Test handles POST /api/v1/orgs/:org_id/llm-providers/test.
//
// Two body shapes are accepted:
//
//  1. Inline credentials — {provider, base_url?, api_key} probes the vendor with
//     the supplied key without persisting anything.
//  2. Stored credentials — {provider_id} loads the row in the caller's org,
//     decrypts the stored key, and probes the vendor with it. Lets the
//     Edit-credentials dialog re-test without holding the secret in memory.
//
// @Summary     Test LLM provider credentials
// @Description Probe the vendor with either inline credentials or a stored provider_id
// @Tags        llm-providers
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       org_id  path string true "Organisation ID"
// @Param       request body model.TestProviderRequest true "Test connection payload"
// @Success     200 {object} model.TestConnectionResult
// @Failure     400 {object} apierror.AppError
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Failure     422 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers/test [post]
func (h *LLMProviderHandler) Test(c *gin.Context) {
	orgID := c.Param("org_id")

	var req model.TestProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(&apierror.AppError{
			Code:    http.StatusUnprocessableEntity,
			Message: "Unprocessable Entity",
			Detail:  err.Error(),
		})
		c.Abort()
		return
	}

	resp, err := h.svc.TestConnection(c.Request.Context(), orgID, req)
	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, resp)
}

// SetDefault handles PUT /api/v1/orgs/:org_id/llm-providers/:provider_id/default.
//
// @Summary     Set default LLM provider
// @Description Mark a provider config as the default for the organisation
// @Tags        llm-providers
// @Security    BearerAuth
// @Param       org_id      path string true "Organisation ID"
// @Param       provider_id path string true "Provider config ID"
// @Success     204
// @Failure     401 {object} apierror.AppError
// @Failure     403 {object} apierror.AppError
// @Failure     404 {object} apierror.AppError
// @Router      /orgs/{org_id}/llm-providers/{provider_id}/default [put]
func (h *LLMProviderHandler) SetDefault(c *gin.Context) {
	orgID := c.Param("org_id")
	providerID := c.Param("provider_id")
	if err := h.svc.SetDefault(c.Request.Context(), orgID, providerID); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}
	c.Status(http.StatusNoContent)
}
