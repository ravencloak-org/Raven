#!/bin/bash
set -euxo pipefail

# ── 1. System deps ─────────────────────────────────────────────────────────
dnf -y update
dnf -y install git docker ansible-core jq awscli amazon-ssm-agent
systemctl enable --now docker
systemctl enable --now amazon-ssm-agent
usermod -aG docker ec2-user

# ── 2. Mount data volume ──────────────────────────────────────────────────
DEVICE=/dev/nvme1n1
MOUNT=/var/lib/raven-data
if ! blkid "$DEVICE"; then
  mkfs.ext4 -L raven-data "$DEVICE"
fi
mkdir -p "$MOUNT"
echo "LABEL=raven-data $MOUNT ext4 defaults,nofail 0 2" >> /etc/fstab
mount "$MOUNT"

# ── 3. Pull SSM params into /etc/raven/env ────────────────────────────────
mkdir -p /etc/raven
PARAMS=$(aws ssm get-parameters-by-path \
  --region ${aws_region} \
  --path /raven/demo \
  --with-decryption \
  --recursive \
  --query 'Parameters[].[Name,Value]' --output text)

: > /etc/raven/env.tmp
while IFS=$'\t' read -r name value; do
  key=$(basename "$name" | tr '[:lower:]' '[:upper:]')
  # don't expose cloudflared_credentials_json this way — handled separately
  if [ "$key" = "CLOUDFLARED_CREDENTIALS_JSON" ]; then continue; fi
  printf '%s=%s\n' "$key" "$value" >> /etc/raven/env.tmp
done <<< "$PARAMS"

mv /etc/raven/env.tmp /etc/raven/env
chmod 0600 /etc/raven/env
chown root:root /etc/raven/env

# ── 4. Cloudflared credentials ────────────────────────────────────────────
mkdir -p /etc/cloudflared
TUNNEL_ID=$(aws ssm get-parameter --region ${aws_region} \
  --name /raven/demo/cloudflared_tunnel_id --query Parameter.Value --output text)
aws ssm get-parameter --region ${aws_region} \
  --name /raven/demo/cloudflared_credentials_json \
  --with-decryption --query Parameter.Value --output text \
  > /etc/cloudflared/$TUNNEL_ID.json
chmod 0600 /etc/cloudflared/$TUNNEL_ID.json
echo "TUNNEL_ID=$TUNNEL_ID" >> /etc/raven/env

# ── 5. Clone repo and run Ansible ─────────────────────────────────────────
cd /opt
git clone https://github.com/ravencloak-org/Raven.git raven
cd raven
git checkout ${deploy_tag}

ansible-playbook deploy/ansible/playbook.yml \
  -i 'localhost,' -c local \
  -e ansible_group_vars_file=group_vars/demo.yml

echo "raven-stack: ok"
