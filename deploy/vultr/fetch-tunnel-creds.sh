#!/usr/bin/env bash
# fetch-tunnel-creds.sh — pull the Cloudflare tunnel credentials JSON from
# AWS SSM (where terraform stores it) and write it to a local file.
#
# Run this on your laptop (where AWS creds are available), then scp the
# resulting JSON to /etc/cloudflared/<tunnel_id>.json on the Vultr box.
#
# The tunnel itself is terraform-managed (deploy/terraform/demo/), so the
# tunnel ID + credentials live in AWS SSM as
#   /raven/demo/tunnel_id          (SecureString)
#   /raven/demo/tunnel_credentials (SecureString, base64-encoded JSON)
#
# Usage:
#   AWS_REGION=ap-south-1 bash deploy/vultr/fetch-tunnel-creds.sh ./tunnel.json
#   scp ./tunnel.json root@64.176.97.248:/etc/cloudflared/<id>.json
#   ssh root@64.176.97.248 'chmod 600 /etc/cloudflared/<id>.json'
#
set -euo pipefail

OUT="${1:-./tunnel.json}"
REGION="${AWS_REGION:-ap-south-1}"
TUNNEL_ID_PARAM="${TUNNEL_ID_PARAM:-/raven/demo/tunnel_id}"
TUNNEL_CREDS_PARAM="${TUNNEL_CREDS_PARAM:-/raven/demo/tunnel_credentials}"

command -v aws >/dev/null || { echo "aws CLI required" >&2; exit 1; }
command -v jq  >/dev/null || { echo "jq required" >&2; exit 1; }

echo "[fetch-creds] reading $TUNNEL_ID_PARAM ($REGION)..."
TUNNEL_ID=$(aws ssm get-parameter --region "$REGION" \
  --name "$TUNNEL_ID_PARAM" --with-decryption \
  --query 'Parameter.Value' --output text)

echo "[fetch-creds] reading $TUNNEL_CREDS_PARAM ($REGION)..."
aws ssm get-parameter --region "$REGION" \
  --name "$TUNNEL_CREDS_PARAM" --with-decryption \
  --query 'Parameter.Value' --output text | base64 -d > "$OUT"

# Sanity-check the JSON is well-formed and references the same tunnel.
JSON_TUNNEL_ID=$(jq -r '.TunnelID // .tunnel_id // empty' < "$OUT")
if [ -z "$JSON_TUNNEL_ID" ]; then
  echo "[fetch-creds] WARN: couldn't parse TunnelID from credentials JSON" >&2
elif [ "$JSON_TUNNEL_ID" != "$TUNNEL_ID" ]; then
  echo "[fetch-creds] WARN: tunnel_id ($TUNNEL_ID) != credentials TunnelID ($JSON_TUNNEL_ID)" >&2
fi

chmod 600 "$OUT"
echo "[fetch-creds] wrote $OUT (chmod 600)"
echo "[fetch-creds] tunnel_id=$TUNNEL_ID"
echo
echo "Next:"
echo "  scp $OUT root@<vultr-host>:/etc/cloudflared/${TUNNEL_ID}.json"
echo "  ssh root@<vultr-host> 'chmod 600 /etc/cloudflared/${TUNNEL_ID}.json'"
echo "  ssh root@<vultr-host> 'CLOUDFLARE_TUNNEL_ID=${TUNNEL_ID} bash /opt/raven/deploy/vultr/bootstrap.sh'"
