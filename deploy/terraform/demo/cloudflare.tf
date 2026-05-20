####
# `cloudflare_tunnel.demo` was originally created by `terraform apply` from
# this file (tunnel id `9bae2af2-f8e8-45ed-aa61-e1eb29182c3c`, connected
# from an AWS EC2 box). On 2026-05-18 the demo was cut over to Vultr per
# `docs/runbooks/demo-cutover.md` — a SECOND tunnel
# (`ac5bd284-c908-4c7d-8666-f6e8e824c693`) was created out-of-band in the
# Cloudflare dashboard with the same two `/raven/...` ingress rules, the
# Vultr cloudflared registered against it, and `demo.ravencloak.org`'s
# CNAME was flipped to the new tunnel. The old `9bae2af2` tunnel is now
# dormant and will be deleted alongside the AWS decommission.
#
# The `import` blocks below adopt the new (Vultr) tunnel + its config
# into the same `cloudflare_tunnel.demo` / `cloudflare_tunnel_config.demo`
# addresses so Terraform converges on reality. The dashboard-configured
# ingress rules are identical to those declared in `cloudflare_tunnel_config.demo`
# below, so the post-import plan should be a clean no-op refresh.
#
# `secret` is kept (provider schema makes it required) but wrapped in
# `lifecycle { ignore_changes }` so Terraform does NOT rotate the
# dashboard-issued secret on first apply — rotating it would invalidate
# the token the Vultr cloudflared connector is bootstrapped from. The
# `random_id.tunnel_secret` + `aws_ssm_parameter.*` resources below are
# AWS-only and will be removed by the parallel AWS-decommission PR.
#
# REVIEWER NOTE: state currently has `cloudflare_tunnel.demo` +
# `cloudflare_tunnel_config.demo` bound to the OLD tunnel
# (`9bae2af2-…`). Before `terraform plan`, run:
#   terraform state rm cloudflare_tunnel.demo cloudflare_tunnel_config.demo
# so the `import` blocks have a clean slot to re-populate.
####
import {
  to = cloudflare_tunnel.demo
  id = "${var.cloudflare_account_id}/${var.demo_tunnel_id}"
}

import {
  to = cloudflare_tunnel_config.demo
  id = "${var.cloudflare_account_id}/${var.demo_tunnel_id}"
}

resource "random_id" "tunnel_secret" {
  byte_length = 35
}

resource "cloudflare_tunnel" "demo" {
  account_id = var.cloudflare_account_id
  name       = "raven-demo"
  # `secret` is required by the provider schema, but the imported (Vultr)
  # tunnel was created in the dashboard with its own server-generated
  # secret; the live Vultr cloudflared connector was bootstrapped from
  # that secret's token and would disconnect if Terraform rotated it.
  # We seed the argument from `random_id.tunnel_secret` to satisfy the
  # schema and `ignore_changes` to prevent any rotation post-import.
  secret = random_id.tunnel_secret.b64_std
  lifecycle {
    ignore_changes = [secret]
  }
}

resource "cloudflare_tunnel_config" "demo" {
  account_id = var.cloudflare_account_id
  tunnel_id  = cloudflare_tunnel.demo.id

  config {
    # API requests at <demo_hostname><prefix>/api/* → Go API on 8081.
    # cloudflared forwards the full path (including the prefix); the Go API
    # router mounts everything under cfg.Server.PathPrefix so paths line up.
    ingress_rule {
      hostname = var.demo_hostname
      service  = "http://localhost:8081"
      path     = "${var.demo_path_prefix}/api/.*"
    }
    # Everything else under the prefix → Vue SPA on 3080 (Vite built with
    # base=<prefix>/ so asset URLs resolve correctly).
    ingress_rule {
      hostname = var.demo_hostname
      service  = "http://localhost:3080"
      path     = "${var.demo_path_prefix}/.*"
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
