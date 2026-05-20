# Runbook: cut `demo.ravencloak.org` over from AWS to Vultr

**Status**: pending (operational steps below have not been executed yet)
**Estimated time**: ~15 min hands-on + ~5 min DNS propagation
**Reversibility**: full, ~5 min via DNS flip-back
**Risk**: low — both stacks remain runnable during cutover; the only
shared resource is Cloudflare DNS

## Background

`demo.ravencloak.org` currently routes to a cloudflared connector on an
**AWS EC2** instance (`raven-demo-app`, public IP `65.0.104.188`, in
`ap-south-1`). That stack runs pre-#625 code — broken behind the new
`/raven/*` path-prefixed ingress rules.

A parallel **Vultr** stack (`64.176.97.248`) has been provisioned and
proven via the public-IP URL `http://64.176.97.248:13080/raven/` (see
#628 for provisioning, #631 for the path-prefixed CI image). Its
cloudflared connector is already running, but bound to a **different**
tunnel (`ac5bd284-c908-4c7d-8666-f6e8e824c693`) than the one DNS
currently points at (`9bae2af2-f8e8-45ed-aa61-e1eb29182c3c`).

Cutover means: route `demo.ravencloak.org` through the Vultr tunnel
instead of the AWS one, then decommission AWS in a follow-up.

## Tunnel topology — before / after

| | Before | After |
|---|---|---|
| `demo.ravencloak.org` CNAME → | `9bae2af2-….cfargotunnel.com` (terraform-managed) | `ac5bd284-….cfargotunnel.com` (created out-of-band) |
| Connector for that tunnel | cloudflared on AWS EC2 | cloudflared on Vultr |
| Ingress rules on tunnel | terraform: `/raven/api/.*` → `localhost:8081`, `/raven/.*` → `localhost:3080` (AWS box) | dashboard: same two rules, on the Vultr tunnel |
| Old tunnel (`9bae2af2`) | active, terraform-managed | dormant; decommission with AWS |

## Pre-flight checks (do not skip)

Run all of these on your laptop. If any fails, fix before proceeding.

```bash
# 1. Vultr stack is healthy end-to-end via the public IP. Replace
#    asset hash with whatever index-*.js the current build emits.
curl -sf http://64.176.97.248:13080/raven/api/v1/config \
  | grep -q single_user && echo "✓ API responds with JSON"

curl -sf -o /dev/null -w "asset HTTP %{http_code}\n" \
  "http://64.176.97.248:13080$(curl -s http://64.176.97.248:13080/raven/ \
    | grep -oE '/raven/assets/index-[^"]+\.js' | head -1)"

# 2. Vultr cloudflared is connected to the tunnel.
ssh root@64.176.97.248 'systemctl is-active cloudflared && \
  journalctl -u cloudflared -n 10 --no-pager | grep -c "Registered tunnel connection"'
# expect: active, plus a non-zero "Registered tunnel connection" count

# 3. AWS demo is still up (so we have a rollback target).
curl -sI https://demo.ravencloak.org/raven/api/v1/config \
  | head -1   # any 200/404 from Go's gin (with `x-trace-id` header) is fine — proves AWS is reachable
```

## Cutover steps

### Step 1 — add Public Hostnames on the Vultr tunnel (Cloudflare dashboard)

Zero Trust → Networks → Tunnels → click into tunnel `ac5bd284-c908-4c7d-8666-f6e8e824c693` → **Public Hostnames** tab → **Add a public hostname**.

Add **two** entries in this order (more specific first):

| Subdomain | Domain | Path | Service |
|----|----|----|----|
| `demo` | `ravencloak.org` | `raven/api/.*` | `http://localhost:8081` |
| `demo` | `ravencloak.org` | `raven/.*`     | `http://localhost:3080` |

Click **Additional application settings** → leave defaults.
Save each entry.

When prompted to create/overwrite the DNS record, **decline** — we'll handle DNS explicitly in Step 2 so we can verify+roll back independently.

### Step 2 — verify ingress is published before touching DNS

Cloudflare propagates ingress rules to connectors within ~5s. Confirm:

```bash
ssh root@64.176.97.248 'journalctl -u cloudflared -n 20 --no-pager' \
  | grep -i 'ingress\|configuration update\|reload' | tail -5
```

You should see a recent log line about a config update. If nothing appears within a minute, retry the Cloudflare save.

### Step 3 — flip the DNS CNAME

Cloudflare dashboard → `ravencloak.org` zone → **DNS** → **Records** → click the `demo` row → edit:

| Field | Before | After |
|----|----|----|
| Target | `9bae2af2-….cfargotunnel.com` | `ac5bd284-c908-4c7d-8666-f6e8e824c693.cfargotunnel.com` |
| Proxy status | Proxied (orange) | unchanged |
| TTL | Auto | unchanged |

Save. Cloudflare's edge picks up the new CNAME within ~30s.

### Step 4 — smoke test

```bash
# Within ~30s after the DNS save:
for _ in {1..6}; do
  curl -sS -o /tmp/resp -D /tmp/headers -w "HTTP %{http_code}\n" \
    https://demo.ravencloak.org/raven/api/v1/config
  if grep -q 'single_user' /tmp/resp; then
    echo "✓ JSON received from new Vultr stack"
    break
  fi
  sleep 5
done

# Confirm response came from the right origin:
# - go-api's middleware adds x-trace-id; AWS path was getting Go's 404 page
# - bundle digest should match the frontend-raven image you just published
grep -i 'x-trace-id' /tmp/headers   # presence proves Vultr go-api responded
curl -s https://demo.ravencloak.org/raven/ \
  | grep -oE '/raven/assets/index-[^"]+\.js' | head -1
```

If any of those fail, go to **Rollback** below.

### Step 5 — stop the AWS cloudflared connector

We **don't terraform-destroy AWS yet** — keeping it as a warm rollback target for ~24h after cutover. Just stop the cloudflared service so it stops trying to serve the now-orphaned tunnel:

```bash
# Via AWS SSM (preferred — keeps SSH closed):
aws ssm send-command --region ap-south-1 \
  --instance-ids $(aws ec2 describe-instances --region ap-south-1 \
    --filters 'Name=tag:Name,Values=raven-demo-app' \
              'Name=instance-state-name,Values=running' \
    --query 'Reservations[0].Instances[0].InstanceId' --output text) \
  --document-name AWS-RunShellScript \
  --comment 'stop cloudflared post-cutover' \
  --parameters 'commands=["systemctl stop cloudflared && systemctl disable cloudflared"]'
```

After this, tunnel `9bae2af2` has zero connectors. Cloudflare keeps the
tunnel registered (it's a billing/free-tier resource that stays in your
account) but it serves no traffic.

## Rollback (executable in ~2 min)

If Step 4 fails:

1. Cloudflare DNS → `demo` row → flip Target back to `9bae2af2-….cfargotunnel.com`. Save.
2. The AWS cloudflared is still running (Step 5 hasn't happened yet), so traffic resumes via AWS within ~30s.
3. Investigate the Vultr-side failure (cloudflared logs on Vultr, Public Hostname config on `ac5bd284`).
4. Fix, retry from Step 4.

If Step 5 already ran (AWS cloudflared stopped) and you need to roll back: re-enable + start AWS cloudflared via the same SSM pattern (`systemctl enable --now cloudflared`), then flip DNS back.

## Post-cutover follow-ups (separate PRs)

- **Terraform import** of the now-canonical tunnel `ac5bd284` and replace `cloudflare_tunnel.demo` + `cloudflare_tunnel_config.demo` references. Drop the orphaned `9bae2af2` resource. Touches `deploy/terraform/demo/cloudflare.tf` + `terraform import` blocks.
- **AWS decommission**: `terraform destroy` of `aws_instance.demo` + IAM + S3 backup bucket + SSM parameters. Tracked as Task #11. The terraform code removal lands in a separate PR ahead of the destroy; see the data-export checklist below.
- **Vultr lockdown**: now that the demo serves through cloudflared, the public-IP ports `13080` + `18081` are no longer needed. Either add a Vultr cloud-firewall rule restricting them to SSH-only, OR rebind compose ports back to `127.0.0.1:` in `docker-compose.override.yml` on the box.
- **Frontend image cleanup**: retire the one-off `ghcr.io/ravencloak-org/frontend:raven-prefixed` tag I hand-built — every release tag now produces `frontend-raven:<version>` via CI (#631).

### AWS data-export + destroy checklist (Task #11)

Run this **only after** cutover has been stable for ~24h, and **only after** the
terraform-code-removal PR has merged (the one that deletes `ec2.tf`, `iam.tf`,
`s3.tf`, `backup.tf`, `oidc.tf`, `security_group.tf`, `vpc.tf`, `ebs.tf`,
`user_data.sh.tpl`). Order matters — back up first, destroy second.

1. **Export S3 backups locally** (postgres + clickhouse dumps written by the
   nightly job). The bucket name is `raven-demo-backups`:

   ```bash
   aws s3 sync s3://raven-demo-backups ./local-backups/ \
     --region ap-south-1
   # Optional: push the copy to Vultr / OneDrive / wherever long-term lives.
   du -sh ./local-backups/
   ```

   Verify a sample tarball untars and the latest postgres dump is restorable
   before proceeding.

2. **Preview the full destroy** so nothing unexpected is in the blast radius:

   ```bash
   cd deploy/terraform/demo
   terraform plan -destroy -out=destroy.tfplan
   ```

   The plan should show only `aws_*` resources plus the two
   `aws_ssm_parameter.*` entries living in `cloudflare.tf`. Anything
   `cloudflare_*` should be UNCHANGED — if it isn't, stop and reconcile.

3. **Targeted destroy in safe order** (each `-target` invocation is its own
   `terraform destroy`):

   ```bash
   # a. Compute first — frees the EBS attachment and IAM-profile association.
   terraform destroy -target=aws_volume_attachment.data
   terraform destroy -target=aws_instance.demo

   # b. EBS data volume. Note: lifecycle.prevent_destroy = true in ebs.tf —
   #    once that file is deleted, the protection is gone, BUT terraform may
   #    still complain it's referenced. Remove from state manually if needed:
   #      terraform state rm aws_ebs_volume.data
   #    then destroy via the AWS console / CLI.
   terraform destroy -target=aws_ebs_volume.data

   # c. S3 backup bucket (only after step 1 above succeeded).
   terraform destroy -target=aws_s3_bucket_lifecycle_configuration.backups
   terraform destroy -target=aws_s3_bucket_public_access_block.backups
   terraform destroy -target=aws_s3_bucket_server_side_encryption_configuration.backups
   terraform destroy -target=aws_s3_bucket_versioning.backups
   terraform destroy -target=aws_s3_bucket.backups   # bucket must be empty first

   # d. Backup vault + plan + selection + role.
   terraform destroy -target=aws_backup_selection.demo
   terraform destroy -target=aws_backup_plan.demo
   terraform destroy -target=aws_backup_vault.demo
   terraform destroy -target=aws_iam_role_policy_attachment.backup
   terraform destroy -target=aws_iam_role.backup

   # e. IAM instance profile + role + inline policies.
   terraform destroy -target=aws_iam_instance_profile.demo
   terraform destroy -target=aws_iam_role_policy.s3_backups
   terraform destroy -target=aws_iam_role_policy.ssm_read
   terraform destroy -target=aws_iam_role_policy_attachment.ssm_core
   terraform destroy -target=aws_iam_role.demo

   # f. GitHub Actions OIDC role (the OIDC provider itself is account-wide;
   #    leave it unless no other AWS account workflows use it).
   terraform destroy -target=aws_iam_role_policy.github_demo_deploy
   terraform destroy -target=aws_iam_role.github_demo_deploy

   # g. SSM parameters living in cloudflare.tf (cleaned up by the parallel
   #    tunnel-import PR; if that hasn't landed yet, do them by hand here).
   terraform destroy -target=aws_ssm_parameter.tunnel_credentials
   terraform destroy -target=aws_ssm_parameter.tunnel_id

   # h. Network: route table assoc → IGW → subnet → VPC.
   terraform destroy -target=aws_route_table_association.demo
   terraform destroy -target=aws_route_table.demo
   terraform destroy -target=aws_internet_gateway.demo
   terraform destroy -target=aws_security_group.demo
   terraform destroy -target=aws_subnet.demo
   terraform destroy -target=aws_vpc.demo
   ```

   Alternatively, once the terraform code is removed, a plain
   `terraform apply` (against the now-orphaned state) will destroy everything
   in dependency order. Use `terraform plan -destroy` first to inspect.

4. **Sanity-check the AWS console** that no `raven-demo-*` resources remain in
   `ap-south-1`: EC2 instances, EBS volumes, S3 buckets, IAM roles,
   VPCs/subnets, Backup vaults, SSM parameters.

5. **Update task #11 → completed** and write a Stash entry under
   `/projects/raven/infra` recording the final destroy date so future agents
   know AWS is officially out of the demo path.

## What success looks like

- `curl https://demo.ravencloak.org/raven/api/v1/config` → `{"single_user":false}` with `x-trace-id` header
- `curl https://demo.ravencloak.org/raven/` → SPA HTML referencing `/raven/assets/index-….js`
- Browser load of `https://demo.ravencloak.org/raven/` → boots the Vue app, can hit the login page (SuperTokens calls via same-origin nginx proxy)
- Vultr `journalctl -u cloudflared` shows connection events; AWS box shows the same when re-enabled
