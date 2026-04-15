import SuperTokens from "supertokens-web-js"
import Session from "supertokens-web-js/recipe/session"
import ThirdParty from "supertokens-web-js/recipe/thirdparty"

export function initSuperTokens() {
  SuperTokens.init({
    appInfo: {
      appName: "Raven",
      apiDomain: import.meta.env.VITE_API_DOMAIN || "http://localhost:8081",
      apiBasePath: "/auth",
    },
    recipeList: [
      Session.init(),
      ThirdParty.init(),
    ],
  })
}
