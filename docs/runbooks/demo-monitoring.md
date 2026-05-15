# Demo Monitoring Runbook

## Uptime: BetterStack

- Monitor name: `raven-demo-healthz`
- URL: `https://demo.raven.ravencloak.org/healthz`
- Check frequency: 60 seconds
- Request timeout: 10 seconds
- Expected status code: 200
- Owner: jobinlawrance@gmail.com

### Manual setup (one-time)

1. Sign up at https://betterstack.com (free tier is sufficient).
2. Create an Uptime monitor with the parameters above.
3. Enable email + push notifications on the iOS/Android app.
4. Cloudflare Access bypass: during Phase 1 the whole demo hostname is gated.
   Add a Cloudflare Access bypass policy so that `Path == /healthz` is exempt,
   otherwise BetterStack probes will hit the Access login page.
   - Cloudflare dashboard → Zero Trust → Access → Applications → `raven-demo-phase1` → Add policy
   - Action: `Bypass`, Include: `Common Name` is not required (rule with no
     include criteria matches everything; restrict by `Path: /healthz` instead)
   - Alternative: configure BetterStack to follow Access redirects with a
     service token — overkill for a single endpoint.

### When the monitor fires

1. Open the BetterStack incident, note the timestamp.
2. SSM into the box:
   ```bash
   aws ssm start-session --target $(cd deploy/terraform/demo && terraform output -raw instance_id) --region ap-south-1
   ```
3. Inspect recent service logs:
   ```bash
   sudo journalctl -u docker --since '5 minutes ago' | grep -E 'go-api|python-worker'
   ```
4. Check container state:
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.demo.yml ps
   ```
   Restart any unhealthy service.
5. If unresolvable in 15 minutes, post status on `ravencloak.org/status` (Phase 3 only).

## Host metrics: Beszel

Already installed on the AWS Beszel instance. Confirm the demo box is registered
under the same Beszel hub.

## App telemetry: OpenObserve

`https://observability.demo.raven.ravencloak.org` — Cloudflare Access required.
Login email: `jobinlawrance@gmail.com`.
