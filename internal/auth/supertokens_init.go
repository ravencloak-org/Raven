package auth

import (
	"net/http"

	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty/tpmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
)

// SuperTokensInitConfig holds the parameters required to initialise the
// SuperTokens Go SDK. APIDomain and WebsiteDomain come from config so that
// the same binary can run locally (localhost) or in production (ravencloak.org).
type SuperTokensInitConfig struct {
	ConnectionURI string
	APIKey        string
	APIDomain     string // e.g. https://api.ravencloak.org
	WebsiteDomain string // e.g. https://app.ravencloak.org
	// PathPrefix prepends to apiBasePath + websiteBasePath so the SuperTokens
	// SDK matches incoming requests at the same path the Go API actually
	// mounts auth routes under. The demo runs under /raven; SaaS at root.
	// Empty string = no prefix.
	PathPrefix string
}

// InitSuperTokens initialises the SuperTokens Go SDK with the ThirdParty and
// Session recipes. The SDK must be initialised once, before any route
// registration, so that supertokens.Middleware can intercept the auth paths.
func InitSuperTokens(cfg SuperTokensInitConfig) error {
	apiBasePath := cfg.PathPrefix + "/auth"
	websiteBasePath := cfg.PathPrefix + "/auth"

	// Cookie domain for cross-subdomain session sharing.
	// api.ravencloak.org sets cookies, app.ravencloak.org reads them.
	cookieDomain := ".ravencloak.org"
	if cfg.APIDomain == "" || cfg.APIDomain == "http://localhost:8081" {
		cookieDomain = "localhost"
	}

	recipeList := []supertokens.Recipe{
		thirdparty.Init(&tpmodels.TypeInput{
			// Provider list is managed via the SuperTokens Core multitenancy API.
		}),
		session.Init(&sessmodels.TypeInput{
			CookieDomain:  &cookieDomain,
			CookieSameSite: strPtr("none"),
			// Pin tokens to the Authorization header / response-header path so
			// the supertokens-web-js SDK (configured with
			// tokenTransferMethod: 'header' in frontend/src/plugins/supertokens.ts)
			// receives tokens it can store in localStorage. Without this, the
			// backend's default ("any") chose cookies on the OAuth callback
			// response, leaving localStorage empty — Session.doesSessionExist()
			// then returned false on the next page-load and the SPA bounced the
			// user to /login (the long-running "logout on navigation" bug).
			GetTokenTransferMethod: func(_ *http.Request, _ bool, _ supertokens.UserContext) sessmodels.TokenTransferMethod {
				return sessmodels.HeaderTransferMethod
			},
		}),
	}

	return supertokens.Init(supertokens.TypeInput{
		Supertokens: &supertokens.ConnectionInfo{
			ConnectionURI: cfg.ConnectionURI,
			APIKey:        cfg.APIKey,
		},
		AppInfo: supertokens.AppInfo{
			AppName:         "Raven",
			APIDomain:       cfg.APIDomain,
			WebsiteDomain:   cfg.WebsiteDomain,
			APIBasePath:     &apiBasePath,
			WebsiteBasePath: &websiteBasePath,
		},
		RecipeList: recipeList,
	})
}

func strPtr(s string) *string { return &s }
