package auth

import (
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty/tpmodels"
	"github.com/supertokens/supertokens-golang/supertokens"
)

// SuperTokensInitConfig holds the parameters required to initialise the
// SuperTokens Go SDK. APIDomain and WebsiteDomain come from config so that
// the same binary can run locally (localhost) or in production (ravencloak.org).
type SuperTokensInitConfig struct {
	ConnectionURI      string
	APIKey             string
	APIDomain          string // e.g. https://api.ravencloak.org
	WebsiteDomain      string // e.g. https://app.ravencloak.org
	GoogleClientID     string
	GoogleClientSecret string
}

// InitSuperTokens initialises the SuperTokens Go SDK with the ThirdParty and
// Session recipes. The SDK must be initialised once, before any route
// registration, so that supertokens.Middleware can intercept /auth/* paths.
//
// Google OAuth credentials are optional here: when multitenancy is used the
// provider configuration lives in the Core and does not need to be repeated
// in the SDK init. Pass them only for single-tenant / static setups.
func InitSuperTokens(cfg SuperTokensInitConfig) error {
	apiBasePath := "/auth"
	websiteBasePath := "/auth"

	recipeList := []supertokens.Recipe{
		thirdparty.Init(&tpmodels.TypeInput{
			// Provider list is managed via the SuperTokens Core multitenancy API.
			// Static providers can be added here for single-tenant deployments.
		}),
		session.Init(nil),
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
