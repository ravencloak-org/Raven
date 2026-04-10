package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// Provisioner is the service interface required by ProvisionerHandler.
type Provisioner interface {
	ProvisionRealm(ctx context.Context, realm string) error
}

// ProvisionerHandler handles internal realm-provisioning requests.
type ProvisionerHandler struct {
	svc         Provisioner
	internalKey string
}

// NewProvisionerHandler creates a new ProvisionerHandler.
func NewProvisionerHandler(svc Provisioner, internalKey string) *ProvisionerHandler {
	return &ProvisionerHandler{svc: svc, internalKey: internalKey}
}

// provisionRealmRequest is the JSON body for POST /internal/provision-realm.
type provisionRealmRequest struct {
	Realm string `json:"realm" binding:"required,min=1,max=255"`
}

// ProvisionRealm handles POST /internal/provision-realm.
// It requires the X-Internal-Key header to match the configured internal API key.
func (h *ProvisionerHandler) ProvisionRealm(c *gin.Context) {
	if h.internalKey != "" {
		key := c.GetHeader("X-Internal-Key")
		if key != h.internalKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, apierror.AppError{
				Code:    http.StatusUnauthorized,
				Message: "Unauthorized",
				Detail:  "missing or invalid X-Internal-Key",
			})
			return
		}
	}

	var req provisionRealmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, apierror.AppError{
			Code:    http.StatusBadRequest,
			Message: "Bad Request",
			Detail:  err.Error(),
		})
		return
	}

	if err := h.svc.ProvisionRealm(c.Request.Context(), req.Realm); err != nil {
		_ = c.Error(err)
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "realm": req.Realm})
}
