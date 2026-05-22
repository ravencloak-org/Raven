import SuperTokens from "supertokens-web-js"
import Session from "supertokens-web-js/recipe/session"
import ThirdParty from "supertokens-web-js/recipe/thirdparty"

// Shared with components/TurnstileWidget.vue. The widget writes the
// solved token here; the preAPIHook reads it back when SuperTokens
// finishes the OAuth flow by POSTing /auth/signinup.
const TURNSTILE_STORAGE_KEY = "raven.turnstile.token.v1"

function attachTurnstileTokenOnSignup(input: {
  url: string
  requestInit: RequestInit
}): { url: string; requestInit: RequestInit } {
  if (!input.url.includes("/auth/signinup")) {
    return input
  }
  let token = ""
  try {
    token = sessionStorage.getItem(TURNSTILE_STORAGE_KEY) ?? ""
  } catch {
    // sessionStorage may be unavailable; backend middleware will 403
    // if the demo Turnstile gate is enabled.
  }
  if (!token) return input

  const headers = new Headers(input.requestInit.headers ?? {})
  headers.set("cf-turnstile-token", token)
  return {
    url: input.url,
    requestInit: { ...input.requestInit, headers },
  }
}

export function initSuperTokens() {
  SuperTokens.init({
    appInfo: {
      appName: "Raven",
      apiDomain: import.meta.env.VITE_API_DOMAIN || "http://localhost:8081",
      // Must match the apiBasePath the Go API's SuperTokens SDK is initialised
      // with (internal/auth/supertokens_init.go). The demo runs path-prefixed
      // (/raven/auth); SaaS/dev defaults to /auth.
      apiBasePath: import.meta.env.VITE_AUTH_BASE_PATH || "/auth",
    },
    recipeList: [
      Session.init(),
      ThirdParty.init({
        preAPIHook: async (context) => attachTurnstileTokenOnSignup(context),
      }),
    ],
  })
}
