package auth

import (
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
}

// InitSuperTokens initialises the SuperTokens Go SDK with the ThirdParty and
// Session recipes. The SDK must be initialised once, before any route
// registration, so that supertokens.Middleware can intercept /auth/* paths.
func InitSuperTokens(cfg SuperTokensInitConfig) error {
	apiBasePath := "/auth"
	websiteBasePath := "/auth"

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
			CookieDomain: &cookieDomain,
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
