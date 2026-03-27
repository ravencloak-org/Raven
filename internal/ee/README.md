# Raven Enterprise Edition — Go Backend

Enterprise-only Go packages for the API server.

Licensed under the [Raven Enterprise License](../../ee-LICENSE).
Production use requires a valid license key.

## Packages

| Package | Feature | Tier |
|---------|---------|------|
| `licensing/` | License key validation (signed JWT) | All ee/ |
| `lead/` | Lead intelligence — profiles, scoring, CRM export | Business+ |
| `webhooks/` | Event webhooks — lead.generated, escalation | Business+ |
| `connectors/` | Connector UI backend + data catalog integration | Enterprise |
| `security/` | Advanced WAF rules, DDoS, per-user blocking | Enterprise |
| `audit/` | Audit logs + compliance reports (GDPR/SOC2) | Enterprise |
| `sso/` | Enterprise SSO (SAML/OIDC beyond Keycloak) | Enterprise |
| `analytics/` | Advanced analytics — lead profiles, dashboards | Business+ |
