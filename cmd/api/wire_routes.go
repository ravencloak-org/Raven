package main

// Route-wiring helpers. Each `wireXxxRoutes` function registers a single
// logical group of routes under a caller-supplied *gin.RouterGroup. This
// keeps main.go readable: a 1127-line file used to inline every route, which
// led to duplicate registrations being merged without anyone seeing the
// existing entry (see PR #689 vs PR #749). New feature groups should be
// added here rather than inlined in main.go.

import (
	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
)

// wirePasskeyRoutes registers the M14 passkey management routes
// (issue #771). All three sit under the standard session middleware that the
// caller has already mounted on `api`, so the caller's internal user ID +
// SuperTokens external ID are already on the gin context. Ownership of
// :credential_id is verified inside the handler by listing the core's
// credentials for the session user.
func wirePasskeyRoutes(api *gin.RouterGroup, h *handler.PasskeyHandler) {
	api.GET("/me/passkeys", h.List)
	api.PATCH("/me/passkeys/:credential_id", h.Patch)
	api.DELETE("/me/passkeys/:credential_id", h.Delete)
}

// wireLLMProviderRoutes registers /orgs/:org_id/llm-providers routes. The
// `/test` probe is admin-only because it accepts the API key in the request
// body even though no row is written. The `/default/health` probe is open
// to any authenticated org member because the connectivity toast lives on
// every authenticated page.
func wireLLMProviderRoutes(api *gin.RouterGroup, h *handler.LLMProviderHandler) {
	g := api.Group("/orgs/:org_id/llm-providers")
	g.POST("", middleware.RequireOrgRole("org_admin"), h.Create)
	g.POST("/test", middleware.RequireOrgRole("org_admin"), h.TestConnection)
	g.GET("", h.List)
	g.GET("/default/health", h.DefaultHealth)
	g.GET("/:provider_id", h.Get)
	g.PUT("/:provider_id", middleware.RequireOrgRole("org_admin"), h.Update)
	g.DELETE("/:provider_id", middleware.RequireOrgRole("org_admin"), h.Delete)
	g.PUT("/:provider_id/default", middleware.RequireOrgRole("org_admin"), h.SetDefault)
}

// wireAirbyteConnectorRoutes registers /orgs/:org_id/connectors routes for
// Airbyte source-connector management. Mutation routes are admin-only; read
// routes are open to any authenticated org member.
func wireAirbyteConnectorRoutes(api *gin.RouterGroup, h *handler.AirbyteHandler) {
	g := api.Group("/orgs/:org_id/connectors")
	g.POST("", middleware.RequireOrgRole("org_admin"), h.Create)
	g.GET("", h.List)
	g.GET("/:connector_id", h.Get)
	g.PUT("/:connector_id", middleware.RequireOrgRole("org_admin"), h.Update)
	g.DELETE("/:connector_id", middleware.RequireOrgRole("org_admin"), h.Delete)
	g.POST("/:connector_id/sync", middleware.RequireOrgRole("org_admin"), h.TriggerSync)
	g.GET("/:connector_id/history", h.GetSyncHistory)
}

// wireRoutingRuleRoutes registers /orgs/:org_id/routing-rules routes that
// control which LLM provider answers which kind of request. The entire
// surface is admin-only since routing changes affect every chat completion.
func wireRoutingRuleRoutes(api *gin.RouterGroup, h *handler.RoutingHandler) {
	g := api.Group("/orgs/:org_id/routing-rules", middleware.RequireOrgRole("org_admin"))
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:rule_id", h.Get)
	g.PUT("/:rule_id", h.Update)
	g.DELETE("/:rule_id", h.Delete)
	g.POST("/resolve", h.Resolve)
}
