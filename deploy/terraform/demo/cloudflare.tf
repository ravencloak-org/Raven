resource "random_id" "tunnel_secret" {
  byte_length = 35
}

resource "cloudflare_tunnel" "demo" {
  account_id = var.cloudflare_account_id
  name       = "raven-demo"
  secret     = random_id.tunnel_secret.b64_std
}

resource "cloudflare_tunnel_config" "demo" {
  account_id = var.cloudflare_account_id
  tunnel_id  = cloudflare_tunnel.demo.id

  config {
    ingress_rule {
      hostname = var.demo_hostname
      service  = "http://localhost:8180"
      path     = "/api/.*"
    }
    ingress_rule {
      hostname = var.demo_hostname
      service  = "http://localhost:8080"
    }
    ingress_rule {
      hostname = var.observability_hostname
      service  = "http://localhost:5080"
    }
    ingress_rule {
      service = "http_status:404"
    }
  }
}

resource "cloudflare_record" "demo" {
  zone_id = var.cloudflare_zone_id
  name    = var.demo_hostname
  content = "${cloudflare_tunnel.demo.id}.cfargotunnel.com"
  type    = "CNAME"
  proxied = true
}

resource "cloudflare_record" "observability" {
  zone_id = var.cloudflare_zone_id
  name    = var.observability_hostname
  content = "${cloudflare_tunnel.demo.id}.cfargotunnel.com"
  type    = "CNAME"
  proxied = true
}

# Cloudflare Access: gate observability subdomain to your email only
resource "cloudflare_access_application" "observability" {
  account_id       = var.cloudflare_account_id
  name             = "raven-demo-observability"
  domain           = var.observability_hostname
  session_duration = "24h"
}

resource "cloudflare_access_policy" "observability_allow" {
  account_id     = var.cloudflare_account_id
  application_id = cloudflare_access_application.observability.id
  name           = "allow-operator"
  precedence     = 1
  decision       = "allow"

  include {
    email = [var.access_email]
  }
}

# Phase 1 gate on the whole demo hostname — set var.phase1_access_enabled=false to lift.
variable "phase1_access_enabled" {
  description = "Set true during Phase 1 to gate demo.* behind Cloudflare Access"
  type        = bool
  default     = true
}

resource "cloudflare_access_application" "demo_phase1" {
  count            = var.phase1_access_enabled ? 1 : 0
  account_id       = var.cloudflare_account_id
  name             = "raven-demo-phase1"
  domain           = var.demo_hostname
  session_duration = "24h"
}

resource "cloudflare_access_policy" "demo_phase1_allow" {
  count          = var.phase1_access_enabled ? 1 : 0
  account_id     = var.cloudflare_account_id
  application_id = cloudflare_access_application.demo_phase1[0].id
  name           = "allow-operator"
  precedence     = 1
  decision       = "allow"

  include {
    email = [var.access_email]
  }
}

# Persist tunnel credentials to SSM so the EC2 box can fetch them at cloud-init.
resource "aws_ssm_parameter" "tunnel_credentials" {
  name = "/raven/demo/cloudflared_credentials_json"
  type = "SecureString"
  value = jsonencode({
    AccountTag   = var.cloudflare_account_id
    TunnelID     = cloudflare_tunnel.demo.id
    TunnelSecret = random_id.tunnel_secret.b64_std
  })
  tags = local.tags
}

resource "aws_ssm_parameter" "tunnel_id" {
  name  = "/raven/demo/cloudflared_tunnel_id"
  type  = "String"
  value = cloudflare_tunnel.demo.id
  tags  = local.tags
}
