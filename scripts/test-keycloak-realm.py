#!/usr/bin/env python3
"""
Validation tests for the Raven Keycloak realm export (raven-realm.json).

Uses only Python stdlib (json, sys, os). Exit 0 on success, 1 on failure.
"""

import json
import os
import sys

REALM_PATH = os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    "..", "deploy", "keycloak", "raven-realm.json",
)

PASS = 0
FAIL = 0


def check(description, condition, detail=""):
    global PASS, FAIL
    if condition:
        PASS += 1
        print(f"  PASS  {description}")
    else:
        FAIL += 1
        msg = f"  FAIL  {description}"
        if detail:
            msg += f" -- {detail}"
        print(msg)


def find_client(clients, client_id):
    for c in clients:
        if c.get("clientId") == client_id:
            return c
    return None


def find_scope(scopes, name):
    for s in scopes:
        if s.get("name") == name:
            return s
    return None


def find_mapper(mappers, name):
    for m in mappers:
        if m.get("name") == name:
            return m
    return None


def main():
    print(f"Loading realm JSON from {REALM_PATH}\n")
    with open(REALM_PATH, "r") as f:
        realm = json.load(f)

    # ── Realm-level settings ──────────────────────────────────────────
    print("== Realm settings ==")
    check("realm id is 'raven'", realm.get("realm") == "raven")
    check("displayName is 'Raven Platform'", realm.get("displayName") == "Raven Platform")
    check("realm is enabled", realm.get("enabled") is True)
    check("registration is disabled", realm.get("registrationAllowed") is False)
    check("login with email allowed", realm.get("loginWithEmailAllowed") is True)
    check("sslRequired is 'external'", realm.get("sslRequired") == "external")

    # Token lifespans
    print("\n== Token lifespans ==")
    atl = realm.get("accessTokenLifespan", 0)
    check("access token lifespan is 300", atl == 300, f"got {atl}")
    sso_idle = realm.get("ssoSessionIdleTimeout", 0)
    check("SSO idle timeout is 1800", sso_idle == 1800, f"got {sso_idle}")
    sso_max = realm.get("ssoSessionMaxLifespan", 0)
    check("SSO max lifespan is 36000", sso_max == 36000, f"got {sso_max}")

    # Token lifespans within expected ranges (sanity)
    check("access token lifespan 60-600", 60 <= atl <= 600, f"got {atl}")
    check("SSO idle 600-7200", 600 <= sso_idle <= 7200, f"got {sso_idle}")
    check("SSO max 3600-86400", 3600 <= sso_max <= 86400, f"got {sso_max}")

    # ── Realm roles ───────────────────────────────────────────────────
    print("\n== Realm roles ==")
    realm_roles = realm.get("roles", {}).get("realm", [])
    role_names = {r["name"] for r in realm_roles}
    for expected in ("platform_admin", "org_admin", "user"):
        check(f"role '{expected}' exists", expected in role_names)

    # ── Clients ───────────────────────────────────────────────────────
    print("\n== Clients ==")
    clients = realm.get("clients", [])
    client_ids = [c["clientId"] for c in clients]
    for cid in ("raven-web", "raven-api", "raven-chatbot"):
        check(f"client '{cid}' exists", cid in client_ids)

    # raven-web
    print("\n  -- raven-web --")
    web = find_client(clients, "raven-web")
    if web:
        check("raven-web is public", web.get("publicClient") is True)
        check("raven-web standard flow enabled", web.get("standardFlowEnabled") is True)
        check(
            "raven-web PKCE S256",
            web.get("attributes", {}).get("pkce.code.challenge.method") == "S256",
        )
        redirects = web.get("redirectUris", [])
        check("redirect includes localhost:3000/*", "http://localhost:3000/*" in redirects)
        check("redirect includes localhost:8080/*", "http://localhost:8080/*" in redirects)
        origins = web.get("webOrigins", [])
        check("webOrigin localhost:3000", "http://localhost:3000" in origins)
        check("webOrigin localhost:8080", "http://localhost:8080" in origins)
        check(
            "raven-org in default scopes",
            "raven-org" in web.get("defaultClientScopes", []),
        )

    # raven-api
    print("\n  -- raven-api --")
    api = find_client(clients, "raven-api")
    if api:
        check("raven-api is confidential", api.get("publicClient") is False)
        check("raven-api service accounts enabled", api.get("serviceAccountsEnabled") is True)
        check("raven-api standard flow disabled", api.get("standardFlowEnabled") is False)
        check(
            "raven-org in default scopes",
            "raven-org" in api.get("defaultClientScopes", []),
        )

    # raven-chatbot
    print("\n  -- raven-chatbot --")
    bot = find_client(clients, "raven-chatbot")
    if bot:
        check("raven-chatbot is public", bot.get("publicClient") is True)
        check("raven-chatbot bearer-only", bot.get("bearerOnly") is True)
        check("raven-chatbot standard flow disabled", bot.get("standardFlowEnabled") is False)

    # ── Client scope: raven-org ───────────────────────────────────────
    print("\n== Client scope: raven-org ==")
    scopes = realm.get("clientScopes", [])
    org_scope = find_scope(scopes, "raven-org")
    check("raven-org scope exists", org_scope is not None)
    if org_scope:
        mappers = org_scope.get("protocolMappers", [])
        mapper_names = {m["name"] for m in mappers}
        for expected in ("org_id", "org_role", "workspace_ids"):
            check(f"mapper '{expected}' exists", expected in mapper_names)

        # org_id mapper
        print("\n  -- org_id mapper --")
        m = find_mapper(mappers, "org_id")
        if m:
            check(
                "protocolMapper type",
                m.get("protocolMapper") == "oidc-usermodel-attribute-mapper",
            )
            cfg = m.get("config", {})
            check("claim.name is org_id", cfg.get("claim.name") == "org_id")
            check("access.token.claim true", cfg.get("access.token.claim") == "true")
            check("id.token.claim true", cfg.get("id.token.claim") == "true")
            check("jsonType is String", cfg.get("jsonType.label") == "String")

        # org_role mapper
        print("\n  -- org_role mapper --")
        m = find_mapper(mappers, "org_role")
        if m:
            check(
                "protocolMapper type",
                m.get("protocolMapper") == "oidc-usermodel-attribute-mapper",
            )
            cfg = m.get("config", {})
            check("claim.name is org_role", cfg.get("claim.name") == "org_role")
            check("access.token.claim true", cfg.get("access.token.claim") == "true")
            check("jsonType is String", cfg.get("jsonType.label") == "String")

        # workspace_ids mapper
        print("\n  -- workspace_ids mapper --")
        m = find_mapper(mappers, "workspace_ids")
        if m:
            check(
                "protocolMapper type",
                m.get("protocolMapper") == "oidc-usermodel-attribute-mapper",
            )
            cfg = m.get("config", {})
            check("claim.name is workspace_ids", cfg.get("claim.name") == "workspace_ids")
            check("access.token.claim true", cfg.get("access.token.claim") == "true")
            check("multivalued true", cfg.get("multivalued") == "true")
            check("jsonType is String", cfg.get("jsonType.label") == "String")

    # ── No hardcoded secrets ──────────────────────────────────────────
    print("\n== Security: no hardcoded secrets ==")
    raw = json.dumps(realm)
    # The api client secret should use a placeholder, not a real value
    if api:
        secret = api.get("secret", "")
        check(
            "raven-api secret is a placeholder (not a real value)",
            secret.startswith("${") or secret == "",
            f"got '{secret}'",
        )
    # Broad scan for common secret patterns
    suspicious = []
    for keyword in ("password", "secret_key", "api_key", "private_key"):
        # Skip known safe keys like "secret" (the client property)
        lower = raw.lower()
        # Count occurrences that look like actual values (not config keys)
        if f'"{keyword}": "' in lower:
            # Check the value is not a placeholder
            idx = lower.find(f'"{keyword}": "')
            if idx != -1:
                val_start = lower.index('"', idx + len(keyword) + 4) + 1
                val_end = lower.index('"', val_start)
                val = raw[val_start:val_end]
                if val and not val.startswith("${"):
                    suspicious.append(f"{keyword}={val}")
    check("no hardcoded password/secret_key/api_key/private_key values", len(suspicious) == 0,
          f"found: {suspicious}")

    # ── Summary ───────────────────────────────────────────────────────
    total = PASS + FAIL
    print(f"\n{'='*50}")
    print(f"Results: {PASS}/{total} passed, {FAIL} failed")
    print(f"{'='*50}")
    return 0 if FAIL == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
