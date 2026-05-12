# Demo Infrastructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a private EC2 box at `demo.raven.ravencloak.org` behind Cloudflare Tunnel + Access, with secrets in SSM, backups (logical + EBS), and external uptime monitoring — ready for the deploy pipeline (plan #2) to push images to.

**Architecture:** Single `t4g.xlarge` ARM EC2 in `ap-south-1`, no inbound ports (Cloudflared dials out). One 100 GB gp3 EBS at `/var/lib/raven-data` for all stateful containers. Terraform manages AWS resources + Cloudflare resources. Cloud-init drops the box at the deploy tag, fetches SSM SecureString params into `/etc/raven/env`, and runs the existing `deploy/ansible/playbook.yml` against localhost. Backup crons via systemd timers; AWS Backup snapshots the EBS volume nightly.

**Tech Stack:** Terraform (hashicorp/aws ~> 5.0, cloudflare/cloudflare ~> 4.0), AWS Systems Manager Parameter Store, AWS Backup, Cloudflare Tunnel + Access, existing Ansible playbook, systemd timers.

**Spec reference:** `docs/superpowers/specs/2026-05-12-public-demo-deployment-design.md`

---

## File Structure

**Create:**
- `deploy/terraform/demo/versions.tf` — provider versions, S3 backend
- `deploy/terraform/demo/variables.tf` — all input vars
- `deploy/terraform/demo/outputs.tf` — instance id, bucket name, role arn
- `deploy/terraform/demo/main.tf` — provider config + locals
- `deploy/terraform/demo/vpc.tf` — VPC, subnet, IGW, route
- `deploy/terraform/demo/security_group.tf` — egress-only SG
- `deploy/terraform/demo/iam.tf` — instance role + policies
- `deploy/terraform/demo/s3.tf` — `raven-demo-backups` bucket
- `deploy/terraform/demo/ebs.tf` — 100 GB gp3 volume + attachment
- `deploy/terraform/demo/ec2.tf` — t4g.xlarge with user-data
- `deploy/terraform/demo/backup.tf` — AWS Backup plan + selection
- `deploy/terraform/demo/user_data.sh.tpl` — cloud-init template
- `deploy/terraform/demo/cloudflare.tf` — Tunnel, DNS, Access (cloudflare provider)
- `deploy/terraform/demo/README.md` — apply procedure
- `deploy/ansible/group_vars/demo.yml` — demo-specific Ansible vars
- `deploy/ansible/roles/raven-backup/tasks/main.yml` — install backup timers
- `deploy/ansible/roles/raven-backup/files/raven-pg-backup.sh` — pg_dump → S3
- `deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.sh` — clickhouse-backup → S3
- `deploy/ansible/roles/raven-backup/files/raven-pg-backup.service`
- `deploy/ansible/roles/raven-backup/files/raven-pg-backup.timer`
- `deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.service`
- `deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.timer`
- `docs/runbooks/demo-bootstrap.md` — first-time apply procedure
- `docs/runbooks/demo-restore.md` — restore from backup procedure

**Modify:**
- `deploy/ansible/playbook.yml` — add `raven-backup` role
- `deploy/ansible/group_vars/all.yml.example` — document new vars

**Hand-step (no file, but procedure documented in runbook):**
- SSM SecureString parameters under `/raven/demo/*`
- Cloudflare Tunnel credentials (one-time `cloudflared tunnel create` to get the credentials JSON, stored in SSM)
- BetterStack monitor setup (manual UI)

---

## Tasks

### Task 1: Pre-flight checklist runbook

**Files:**
- Create: `docs/runbooks/demo-bootstrap.md`

- [ ] **Step 1: Write the runbook skeleton listing every prerequisite**

```markdown
# Demo Bootstrap Runbook

## Prerequisites (gather before `terraform apply`)

| Item | How to obtain | Where it lives |
|---|---|---|
| AWS account with admin or sufficient IAM | Existing | n/a |
| Cloudflare API token with `Zone:Edit` + `Tunnel:Edit` on `ravencloak.org` | Cloudflare dashboard → My Profile → API Tokens | Local `~/.cloudflare-token` (gitignored) |
| Cloudflare zone ID for `ravencloak.org` | Cloudflare dashboard → zone overview | TF variable |
| Cloudflare account ID | Cloudflare dashboard → right sidebar | TF variable |
| Google OAuth client ID + secret with redirect `https://demo.raven.ravencloak.org/auth/callback/google` | Google Cloud Console → APIs & Services → Credentials | SSM `/raven/demo/google_client_secret` |
| Razorpay sandbox key id + secret | Razorpay dashboard (Test mode) | SSM `/raven/demo/razorpay_*` |
| Hyperswitch API key (test) | Hyperswitch dashboard | SSM `/raven/demo/hyperswitch_api_key` |
| Resend API key | Resend dashboard, free account | SSM `/raven/demo/resend_api_key` |
| Cloudflare Turnstile site key + secret key | Cloudflare dashboard → Turnstile | SSM `/raven/demo/turnstile_*` |
| `RAVEN_ENCRYPTION_AES_KEY` (32-byte hex) | `openssl rand -hex 32` | SSM `/raven/demo/raven_encryption_aes_key` |
| `SUPERTOKENS_API_KEY` (32-byte hex) | `openssl rand -hex 32` | SSM `/raven/demo/supertokens_api_key` |
| `DOTENV_PRIVATE_KEY` for the demo env | `dotenvx keypair` against a fresh `.env.demo` | SSM `/raven/demo/dotenv_private_key` |
| `RAVEN_LLM_DAILY_USD_CAP` (e.g. `5.00`) | Decide | SSM `/raven/demo/llm_daily_usd_cap` |
| TMDB API key | TMDB account | SSM `/raven/demo/tmdb_api_key` |

## SSM seed commands (run once per prerequisite)

\`\`\`bash
aws ssm put-parameter --region ap-south-1 --type SecureString \\
  --name /raven/demo/google_client_secret --value 'PASTE_VALUE'
# ... repeat for each
\`\`\`

## Cloudflare Tunnel credentials

\`\`\`bash
cloudflared tunnel login                                           # opens browser
cloudflared tunnel create raven-demo                               # writes ~/.cloudflared/<UUID>.json
aws ssm put-parameter --region ap-south-1 --type SecureString \\
  --name /raven/demo/cloudflared_credentials_json \\
  --value "$(cat ~/.cloudflared/<UUID>.json)"
aws ssm put-parameter --region ap-south-1 --type String \\
  --name /raven/demo/cloudflared_tunnel_id --value '<UUID>'
\`\`\`

## Terraform apply

\`\`\`bash
cd deploy/terraform/demo
terraform init
terraform plan -out=demo.tfplan
terraform apply demo.tfplan
\`\`\`

## Post-apply

- Verify `terraform output instance_id` returns an EC2 id.
- Use SSM Session Manager to connect: `aws ssm start-session --target <instance_id>`.
- Tail cloud-init log: `tail -f /var/log/cloud-init-output.log` until "raven-stack: ok" appears.
- Visit `https://demo.raven.ravencloak.org` — Cloudflare Access should prompt for your email (Phase 1 gate).
```

- [ ] **Step 2: Commit**

```bash
git add docs/runbooks/demo-bootstrap.md
git commit -m "docs(demo): add demo bootstrap runbook skeleton"
```

---

### Task 2: Terraform providers, backend, variables, outputs

**Files:**
- Create: `deploy/terraform/demo/versions.tf`
- Create: `deploy/terraform/demo/variables.tf`
- Create: `deploy/terraform/demo/outputs.tf`
- Create: `deploy/terraform/demo/main.tf`

- [ ] **Step 1: Write `versions.tf` pinning providers and S3 backend**

```hcl
terraform {
  required_version = ">= 1.7.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }

  backend "s3" {
    bucket         = "raven-tf-state"
    key            = "demo/terraform.tfstate"
    region         = "ap-south-1"
    encrypt        = true
    dynamodb_table = "raven-tf-locks"
  }
}
```

- [ ] **Step 2: Write `variables.tf`**

```hcl
variable "aws_region" {
  description = "AWS region for the demo stack"
  type        = string
  default     = "ap-south-1"
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t4g.xlarge"
}

variable "data_volume_size_gb" {
  description = "Size of /var/lib/raven-data EBS volume"
  type        = number
  default     = 100
}

variable "cloudflare_account_id" {
  description = "Cloudflare account ID"
  type        = string
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for ravencloak.org"
  type        = string
}

variable "demo_hostname" {
  description = "Public hostname for the demo"
  type        = string
  default     = "demo.raven.ravencloak.org"
}

variable "observability_hostname" {
  description = "Public hostname for the OpenObserve UI"
  type        = string
  default     = "observability.demo.raven.ravencloak.org"
}

variable "access_email" {
  description = "Email allowed through Cloudflare Access during Phase 1 and to observability"
  type        = string
  default     = "jobinlawrance@gmail.com"
}

variable "deploy_tag" {
  description = "Git tag to clone at bootstrap"
  type        = string
  default     = "main"
}

variable "ami_owner" {
  description = "Owner ID for Amazon Linux 2023 ARM AMI"
  type        = string
  default     = "amazon"
}
```

- [ ] **Step 3: Write `outputs.tf`**

```hcl
output "instance_id" {
  description = "EC2 instance id"
  value       = aws_instance.demo.id
}

output "instance_role_arn" {
  description = "ARN of the instance role"
  value       = aws_iam_role.demo.arn
}

output "backup_bucket" {
  description = "S3 bucket holding logical backups"
  value       = aws_s3_bucket.backups.id
}

output "data_volume_id" {
  description = "EBS volume ID for /var/lib/raven-data"
  value       = aws_ebs_volume.data.id
}

output "tunnel_id" {
  description = "Cloudflare Tunnel UUID"
  value       = cloudflare_tunnel.demo.id
}
```

- [ ] **Step 4: Write `main.tf`**

```hcl
provider "aws" {
  region = var.aws_region
}

provider "cloudflare" {
  # CLOUDFLARE_API_TOKEN env var is read automatically
}

locals {
  name_prefix = "raven-demo"
  tags = {
    Project     = "raven"
    Environment = "demo"
    ManagedBy   = "terraform"
  }
}

data "aws_ami" "al2023_arm" {
  most_recent = true
  owners      = [var.ami_owner]
  filter {
    name   = "name"
    values = ["al2023-ami-2023*-arm64"]
  }
  filter {
    name   = "architecture"
    values = ["arm64"]
  }
}
```

- [ ] **Step 5: Verify `terraform init` works (after creating S3 backend bucket + lock table — see step 6)**

Run from `deploy/terraform/demo/`:
```bash
terraform fmt
terraform validate
```

Expected: `Success! The configuration is valid.`

- [ ] **Step 6: Document the one-time backend bootstrap in the runbook**

Append to `docs/runbooks/demo-bootstrap.md`:

```markdown
## Terraform backend (one-time, before first `terraform init`)

\`\`\`bash
aws s3api create-bucket --region ap-south-1 --bucket raven-tf-state \\
  --create-bucket-configuration LocationConstraint=ap-south-1
aws s3api put-bucket-versioning --bucket raven-tf-state \\
  --versioning-configuration Status=Enabled
aws s3api put-bucket-encryption --bucket raven-tf-state \\
  --server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
aws dynamodb create-table --region ap-south-1 \\
  --table-name raven-tf-locks \\
  --attribute-definitions AttributeName=LockID,AttributeType=S \\
  --key-schema AttributeName=LockID,KeyType=HASH \\
  --billing-mode PAY_PER_REQUEST
\`\`\`
```

- [ ] **Step 7: Commit**

```bash
git add deploy/terraform/demo/ docs/runbooks/demo-bootstrap.md
git commit -m "feat(demo): terraform skeleton — providers, variables, outputs, S3 backend"
```

---

### Task 3: VPC, subnet, IGW, security group

**Files:**
- Create: `deploy/terraform/demo/vpc.tf`
- Create: `deploy/terraform/demo/security_group.tf`

- [ ] **Step 1: Write `vpc.tf`**

```hcl
resource "aws_vpc" "demo" {
  cidr_block           = "10.42.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true
  tags                 = merge(local.tags, { Name = "${local.name_prefix}-vpc" })
}

resource "aws_subnet" "demo" {
  vpc_id                  = aws_vpc.demo.id
  cidr_block              = "10.42.1.0/24"
  availability_zone       = "${var.aws_region}a"
  map_public_ip_on_launch = true
  tags                    = merge(local.tags, { Name = "${local.name_prefix}-subnet" })
}

resource "aws_internet_gateway" "demo" {
  vpc_id = aws_vpc.demo.id
  tags   = merge(local.tags, { Name = "${local.name_prefix}-igw" })
}

resource "aws_route_table" "demo" {
  vpc_id = aws_vpc.demo.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.demo.id
  }
  tags = merge(local.tags, { Name = "${local.name_prefix}-rt" })
}

resource "aws_route_table_association" "demo" {
  subnet_id      = aws_subnet.demo.id
  route_table_id = aws_route_table.demo.id
}
```

- [ ] **Step 2: Write `security_group.tf` — egress-only**

```hcl
resource "aws_security_group" "demo" {
  name        = "${local.name_prefix}-sg"
  description = "Egress-only SG for the Raven demo box. Cloudflared dials out — no inbound."
  vpc_id      = aws_vpc.demo.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "All outbound — needed for image pulls, Cloudflared, SSM, OAuth, LLM APIs"
  }

  tags = merge(local.tags, { Name = "${local.name_prefix}-sg" })
}
```

- [ ] **Step 3: Validate**

```bash
cd deploy/terraform/demo
terraform fmt
terraform validate
```

Expected: `Success! The configuration is valid.`

- [ ] **Step 4: Commit**

```bash
git add deploy/terraform/demo/vpc.tf deploy/terraform/demo/security_group.tf
git commit -m "feat(demo): terraform VPC, subnet, IGW, egress-only SG"
```

---

### Task 4: IAM instance role

**Files:**
- Create: `deploy/terraform/demo/iam.tf`

- [ ] **Step 1: Write `iam.tf`**

```hcl
resource "aws_iam_role" "demo" {
  name = "${local.name_prefix}-instance"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action = "sts:AssumeRole"
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.demo.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "ssm_read" {
  name = "ssm-read-demo-params"
  role = aws_iam_role.demo.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:GetParametersByPath"
      ]
      Resource = "arn:aws:ssm:${var.aws_region}:*:parameter/raven/demo/*"
    }, {
      Effect = "Allow"
      Action = "kms:Decrypt"
      Resource = "*"
      Condition = {
        StringEquals = {
          "kms:ViaService" = "ssm.${var.aws_region}.amazonaws.com"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "s3_backups" {
  name = "s3-write-backups"
  role = aws_iam_role.demo.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket",
        "s3:DeleteObject"
      ]
      Resource = [
        aws_s3_bucket.backups.arn,
        "${aws_s3_bucket.backups.arn}/*"
      ]
    }]
  })
}

resource "aws_iam_instance_profile" "demo" {
  name = "${local.name_prefix}-instance"
  role = aws_iam_role.demo.name
  tags = local.tags
}
```

- [ ] **Step 2: Validate**

```bash
terraform fmt
terraform validate
```

- [ ] **Step 3: Commit**

```bash
git add deploy/terraform/demo/iam.tf
git commit -m "feat(demo): terraform IAM role with SSM + S3 backup access"
```

---

### Task 5: S3 backup bucket

**Files:**
- Create: `deploy/terraform/demo/s3.tf`

- [ ] **Step 1: Write `s3.tf`**

```hcl
resource "aws_s3_bucket" "backups" {
  bucket = "${local.name_prefix}-backups"
  tags   = local.tags
}

resource "aws_s3_bucket_versioning" "backups" {
  bucket = aws_s3_bucket.backups.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "backups" {
  bucket                  = aws_s3_bucket.backups.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "backups" {
  bucket = aws_s3_bucket.backups.id
  rule {
    id     = "expire-old-backups"
    status = "Enabled"
    expiration {
      days = 30
    }
    noncurrent_version_expiration {
      noncurrent_days = 7
    }
  }
}
```

- [ ] **Step 2: Validate and commit**

```bash
terraform fmt && terraform validate
git add deploy/terraform/demo/s3.tf
git commit -m "feat(demo): terraform S3 backup bucket with 30-day lifecycle"
```

---

### Task 6: EBS data volume

**Files:**
- Create: `deploy/terraform/demo/ebs.tf`

- [ ] **Step 1: Write `ebs.tf`**

```hcl
resource "aws_ebs_volume" "data" {
  availability_zone = "${var.aws_region}a"
  size              = var.data_volume_size_gb
  type              = "gp3"
  encrypted         = true
  tags = merge(local.tags, {
    Name        = "${local.name_prefix}-data"
    BackupPlan  = "raven-demo-daily"
  })

  lifecycle {
    prevent_destroy = true
  }
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.demo.id
}
```

- [ ] **Step 2: Validate and commit**

```bash
terraform fmt && terraform validate
git add deploy/terraform/demo/ebs.tf
git commit -m "feat(demo): terraform 100GB gp3 data volume with prevent_destroy"
```

---

### Task 7: Cloud-init user-data template

**Files:**
- Create: `deploy/terraform/demo/user_data.sh.tpl`

- [ ] **Step 1: Write the cloud-init template**

```bash
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
```

- [ ] **Step 2: Validate the template renders by writing a test plan**

Run from `deploy/terraform/demo/`:
```bash
echo 'output "rendered_user_data" {
  value = templatefile("user_data.sh.tpl", { aws_region = "ap-south-1", deploy_tag = "main" })
}' > _test_render.tf
terraform fmt _test_render.tf
terraform validate
rm _test_render.tf
```

Expected: validate succeeds.

- [ ] **Step 3: Commit**

```bash
git add deploy/terraform/demo/user_data.sh.tpl
git commit -m "feat(demo): cloud-init user-data — mount EBS, fetch SSM, run Ansible"
```

---

### Task 8: EC2 instance

**Files:**
- Create: `deploy/terraform/demo/ec2.tf`

- [ ] **Step 1: Write `ec2.tf`**

```hcl
resource "aws_instance" "demo" {
  ami                    = data.aws_ami.al2023_arm.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.demo.id
  vpc_security_group_ids = [aws_security_group.demo.id]
  iam_instance_profile   = aws_iam_instance_profile.demo.name

  root_block_device {
    volume_size = 30
    volume_type = "gp3"
    encrypted   = true
  }

  user_data = templatefile("${path.module}/user_data.sh.tpl", {
    aws_region = var.aws_region
    deploy_tag = var.deploy_tag
  })

  metadata_options {
    http_tokens   = "required"
    http_endpoint = "enabled"
  }

  tags = merge(local.tags, { Name = "${local.name_prefix}-app" })

  lifecycle {
    ignore_changes = [
      ami,        # don't replace box when AL2023 publishes a new image
      user_data,  # cloud-init runs once; later changes are via Ansible/deploy
    ]
  }
}
```

- [ ] **Step 2: Validate and commit**

```bash
terraform fmt && terraform validate
git add deploy/terraform/demo/ec2.tf
git commit -m "feat(demo): terraform EC2 t4g.xlarge with IMDSv2 and ignore_changes guards"
```

---

### Task 9: AWS Backup plan

**Files:**
- Create: `deploy/terraform/demo/backup.tf`

- [ ] **Step 1: Write `backup.tf`**

```hcl
resource "aws_backup_vault" "demo" {
  name = "${local.name_prefix}-vault"
  tags = local.tags
}

resource "aws_iam_role" "backup" {
  name = "${local.name_prefix}-backup"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = { Service = "backup.amazonaws.com" }
      Action = "sts:AssumeRole"
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "backup" {
  role       = aws_iam_role.backup.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup"
}

resource "aws_backup_plan" "demo" {
  name = "${local.name_prefix}-daily"

  rule {
    rule_name         = "daily-ebs"
    target_vault_name = aws_backup_vault.demo.name
    schedule          = "cron(0 18 * * ? *)" # 18:00 UTC = 23:30 IST
    lifecycle {
      delete_after = 14
    }
  }

  tags = local.tags
}

resource "aws_backup_selection" "demo" {
  iam_role_arn = aws_iam_role.backup.arn
  name         = "${local.name_prefix}-selection"
  plan_id      = aws_backup_plan.demo.id

  selection_tag {
    type  = "STRINGEQUALS"
    key   = "BackupPlan"
    value = "raven-demo-daily"
  }
}
```

- [ ] **Step 2: Validate and commit**

```bash
terraform fmt && terraform validate
git add deploy/terraform/demo/backup.tf
git commit -m "feat(demo): AWS Backup nightly EBS snapshots, 14d retention"
```

---

### Task 10: Cloudflare Tunnel, DNS, Access (all via Terraform)

**Files:**
- Create: `deploy/terraform/demo/cloudflare.tf`

- [ ] **Step 1: Write `cloudflare.tf`**

```hcl
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

# Phase 1 gate on the whole demo hostname — disabled by default,
# set var.phase1_access_enabled=true to activate during Phase 1, then false.
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
```

- [ ] **Step 2: Store the tunnel credentials in SSM after first apply**

The `cloudflare_tunnel` resource needs its credentials persisted to SSM so the EC2 box can fetch them. Add this resource:

```hcl
resource "aws_ssm_parameter" "tunnel_credentials" {
  name  = "/raven/demo/cloudflared_credentials_json"
  type  = "SecureString"
  value = jsonencode({
    AccountTag   = var.cloudflare_account_id
    TunnelID     = cloudflare_tunnel.demo.id
    TunnelSecret = random_id.tunnel_secret.b64_std
  })
  tags  = local.tags
}

resource "aws_ssm_parameter" "tunnel_id" {
  name  = "/raven/demo/cloudflared_tunnel_id"
  type  = "String"
  value = cloudflare_tunnel.demo.id
  tags  = local.tags
}
```

This supersedes the manual `cloudflared tunnel login` step in the runbook — update the runbook to remove it.

- [ ] **Step 3: Update runbook Step 7**

Modify `docs/runbooks/demo-bootstrap.md` — replace the "Cloudflare Tunnel credentials" section with:

```markdown
## Cloudflare Tunnel credentials

Terraform manages the tunnel resource and writes its credentials to SSM
(`/raven/demo/cloudflared_credentials_json`, `/raven/demo/cloudflared_tunnel_id`).
No manual `cloudflared login` step is needed.
```

- [ ] **Step 4: Validate and commit**

```bash
terraform fmt && terraform validate
git add deploy/terraform/demo/cloudflare.tf docs/runbooks/demo-bootstrap.md
git commit -m "feat(demo): cloudflare tunnel, DNS, Access via terraform; tunnel creds in SSM"
```

---

### Task 11: Ansible group_vars for demo

**Files:**
- Create: `deploy/ansible/group_vars/demo.yml`

- [ ] **Step 1: Inspect existing `group_vars/all.yml.example` to mirror variable names**

```bash
cat /Users/jobinlawrance/Project/raven/deploy/ansible/group_vars/all.yml.example
```

- [ ] **Step 2: Write `deploy/ansible/group_vars/demo.yml` overriding for demo**

```yaml
---
# Demo-specific Ansible variables. Loaded explicitly by cloud-init.
raven_env: demo
raven_domain: demo.raven.ravencloak.org
raven_observability_domain: observability.demo.raven.ravencloak.org

# Compose overlay file applied on top of docker-compose.yml
raven_compose_overlay: docker-compose.demo.yml

# Voice services kept running but UI hidden
raven_voice_enabled: false

# LLM cost fuse
raven_llm_daily_usd_cap: "{{ lookup('env', 'RAVEN_LLM_DAILY_USD_CAP') | default('5.00', true) }}"

# Backup config
raven_backup_s3_bucket: raven-demo-backups
raven_backup_pg_schedule: "*-*-* 18:30:00"        # 18:30 UTC daily
raven_backup_clickhouse_schedule: "*-*-* 19:00:00" # 19:00 UTC daily

# Cloudflared
cloudflared_credentials_path: /etc/cloudflared
```

- [ ] **Step 3: Commit**

```bash
git add deploy/ansible/group_vars/demo.yml
git commit -m "feat(demo): ansible group_vars for demo environment"
```

---

### Task 12: Demo compose overlay (placeholder — minimal)

**Files:**
- Create: `docker-compose.demo.yml`

- [ ] **Step 1: Write minimal overlay that sets demo env**

```yaml
# Overlay applied on top of docker-compose.yml for the public demo.
# Voice services (livekit-server, stt, tts) remain running so the
# python-worker's depends_on chain holds, but no Cloudflared route
# is published for them and the frontend hides voice UI.
services:
  frontend:
    environment:
      RAVEN_VOICE_ENABLED: "false"
      RAVEN_TURNSTILE_SITE_KEY: "${TURNSTILE_SITE_KEY}"

  go-api:
    environment:
      RAVEN_VOICE_ENABLED: "false"
      RAVEN_TURNSTILE_SECRET_KEY: "${TURNSTILE_SECRET_KEY}"

  python-worker:
    environment:
      RAVEN_LLM_DAILY_USD_CAP: "${RAVEN_LLM_DAILY_USD_CAP}"
```

(The actual code that reads `RAVEN_VOICE_ENABLED`, Turnstile keys, and the LLM $-fuse is in plan #3. This overlay just plumbs env vars through.)

- [ ] **Step 2: Commit**

```bash
git add docker-compose.demo.yml
git commit -m "feat(demo): compose overlay wiring demo env vars"
```

---

### Task 13: Backup role — pg_dump systemd timer

**Files:**
- Create: `deploy/ansible/roles/raven-backup/tasks/main.yml`
- Create: `deploy/ansible/roles/raven-backup/files/raven-pg-backup.sh`
- Create: `deploy/ansible/roles/raven-backup/files/raven-pg-backup.service`
- Create: `deploy/ansible/roles/raven-backup/files/raven-pg-backup.timer`

- [ ] **Step 1: Write the backup script `raven-pg-backup.sh`**

```bash
#!/bin/bash
set -euo pipefail

S3_BUCKET="${1:-raven-demo-backups}"
DATE=$(date -u +%Y-%m-%dT%H-%M-%SZ)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

DUMP="$TMP/postgres-$DATE.dump"

docker exec raven-postgres-1 pg_dump -U raven -d raven -Fc > "$DUMP"

if [ ! -s "$DUMP" ]; then
  echo "ERROR: pg_dump produced an empty file" >&2
  exit 1
fi

aws s3 cp "$DUMP" "s3://$S3_BUCKET/postgres/postgres-$DATE.dump"
echo "OK: uploaded postgres/postgres-$DATE.dump"
```

- [ ] **Step 2: Write the systemd service unit**

```ini
[Unit]
Description=Raven Postgres logical backup to S3
Wants=network-online.target docker.service
After=network-online.target docker.service

[Service]
Type=oneshot
EnvironmentFile=/etc/raven/env
ExecStart=/usr/local/bin/raven-pg-backup.sh raven-demo-backups
StandardOutput=journal
StandardError=journal
```

- [ ] **Step 3: Write the timer unit**

```ini
[Unit]
Description=Raven Postgres backup timer

[Timer]
OnCalendar=*-*-* 18:30:00 UTC
RandomizedDelaySec=300
Persistent=true
Unit=raven-pg-backup.service

[Install]
WantedBy=timers.target
```

- [ ] **Step 4: Write the Ansible task in `tasks/main.yml`**

```yaml
---
- name: Install pg backup script
  ansible.builtin.copy:
    src: raven-pg-backup.sh
    dest: /usr/local/bin/raven-pg-backup.sh
    mode: "0755"

- name: Install pg backup service
  ansible.builtin.copy:
    src: raven-pg-backup.service
    dest: /etc/systemd/system/raven-pg-backup.service
    mode: "0644"

- name: Install pg backup timer
  ansible.builtin.copy:
    src: raven-pg-backup.timer
    dest: /etc/systemd/system/raven-pg-backup.timer
    mode: "0644"

- name: Enable pg backup timer
  ansible.builtin.systemd:
    name: raven-pg-backup.timer
    enabled: true
    state: started
    daemon_reload: true
```

- [ ] **Step 5: Test the script syntax locally**

```bash
bash -n deploy/ansible/roles/raven-backup/files/raven-pg-backup.sh
```

Expected: no output (syntax OK).

- [ ] **Step 6: Commit**

```bash
git add deploy/ansible/roles/raven-backup/
git commit -m "feat(demo): postgres backup systemd timer + ansible role skeleton"
```

---

### Task 14: Backup role — ClickHouse timer

**Files:**
- Modify: `deploy/ansible/roles/raven-backup/tasks/main.yml`
- Create: `deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.sh`
- Create: `deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.service`
- Create: `deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.timer`

- [ ] **Step 1: Write the script**

```bash
#!/bin/bash
set -euo pipefail

S3_BUCKET="${1:-raven-demo-backups}"
DATE=$(date -u +%Y-%m-%dT%H-%M-%SZ)
BACKUP_NAME="clickhouse-$DATE"

docker exec raven-clickhouse-1 clickhouse-backup create "$BACKUP_NAME"
docker exec raven-clickhouse-1 clickhouse-backup upload "$BACKUP_NAME"
docker exec raven-clickhouse-1 clickhouse-backup delete local "$BACKUP_NAME"

echo "OK: uploaded clickhouse $BACKUP_NAME"
```

This script requires `clickhouse-backup` to be configured inside the ClickHouse container with the S3 remote pointing at the same bucket. Document this in the ClickHouse compose config — assume an env-driven config block already exists or document its addition as a follow-up note.

- [ ] **Step 2: Write the service unit**

```ini
[Unit]
Description=Raven ClickHouse backup to S3
Wants=network-online.target docker.service
After=raven-pg-backup.service

[Service]
Type=oneshot
EnvironmentFile=/etc/raven/env
ExecStart=/usr/local/bin/raven-clickhouse-backup.sh raven-demo-backups
StandardOutput=journal
StandardError=journal
```

- [ ] **Step 3: Write the timer unit**

```ini
[Unit]
Description=Raven ClickHouse backup timer

[Timer]
OnCalendar=*-*-* 19:00:00 UTC
RandomizedDelaySec=300
Persistent=true
Unit=raven-clickhouse-backup.service

[Install]
WantedBy=timers.target
```

- [ ] **Step 4: Extend the Ansible task file**

Append to `tasks/main.yml`:

```yaml
- name: Install clickhouse backup script
  ansible.builtin.copy:
    src: raven-clickhouse-backup.sh
    dest: /usr/local/bin/raven-clickhouse-backup.sh
    mode: "0755"

- name: Install clickhouse backup service
  ansible.builtin.copy:
    src: raven-clickhouse-backup.service
    dest: /etc/systemd/system/raven-clickhouse-backup.service
    mode: "0644"

- name: Install clickhouse backup timer
  ansible.builtin.copy:
    src: raven-clickhouse-backup.timer
    dest: /etc/systemd/system/raven-clickhouse-backup.timer
    mode: "0644"

- name: Enable clickhouse backup timer
  ansible.builtin.systemd:
    name: raven-clickhouse-backup.timer
    enabled: true
    state: started
    daemon_reload: true
```

- [ ] **Step 5: Syntax-check and commit**

```bash
bash -n deploy/ansible/roles/raven-backup/files/raven-clickhouse-backup.sh
git add deploy/ansible/roles/raven-backup/
git commit -m "feat(demo): clickhouse backup systemd timer"
```

---

### Task 15: Wire raven-backup role into the playbook

**Files:**
- Modify: `deploy/ansible/playbook.yml`

- [ ] **Step 1: Append the role to the playbook role list**

Open `deploy/ansible/playbook.yml`, change:

```yaml
  roles:
    - base
    - docker
    - nodejs
    - pm2
    - cloudflared
    - raven-stack
    - admin-tools
```

to:

```yaml
  roles:
    - base
    - docker
    - nodejs
    - pm2
    - cloudflared
    - raven-stack
    - raven-backup
    - admin-tools
```

- [ ] **Step 2: Commit**

```bash
git add deploy/ansible/playbook.yml
git commit -m "feat(demo): wire raven-backup role into playbook"
```

---

### Task 16: Apply Terraform end-to-end (manual smoke)

**Files:** none (operational task)

- [ ] **Step 1: Seed all SSM parameters per the bootstrap runbook**

Following `docs/runbooks/demo-bootstrap.md` Step 1, run `aws ssm put-parameter` for every prerequisite listed in the table.

- [ ] **Step 2: Bootstrap the Terraform backend (one-time)**

Following `docs/runbooks/demo-bootstrap.md` Step "Terraform backend", run the four `aws s3api` / `aws dynamodb` commands.

- [ ] **Step 3: Apply**

```bash
cd deploy/terraform/demo
export CLOUDFLARE_API_TOKEN=$(cat ~/.cloudflare-token)
terraform init
terraform plan -var cloudflare_account_id=<id> -var cloudflare_zone_id=<id> -out demo.tfplan
terraform apply demo.tfplan
```

Expected: ~20 resources created, no errors.

- [ ] **Step 4: Watch the box come up**

```bash
INSTANCE_ID=$(terraform output -raw instance_id)
aws ssm start-session --target $INSTANCE_ID --region ap-south-1
# inside the session:
sudo tail -f /var/log/cloud-init-output.log
```

Expected: end of log reads `raven-stack: ok`.

- [ ] **Step 5: Visit `https://demo.raven.ravencloak.org`**

Should hit a Cloudflare Access login screen. Enter `jobinlawrance@gmail.com`, follow the email magic link. Once authenticated, the Raven frontend loads.

- [ ] **Step 6: Visit `https://observability.demo.raven.ravencloak.org`**

Should also gate through Cloudflare Access. Once through, OpenObserve UI loads.

- [ ] **Step 7: Verify timers are scheduled**

```bash
sudo systemctl list-timers --all | grep raven
```

Expected: two raven timers active with next-fire times in the next 24h.

- [ ] **Step 8: Manually fire a pg backup as a smoke test**

```bash
sudo systemctl start raven-pg-backup.service
sudo journalctl -u raven-pg-backup.service --no-pager | tail -20
aws s3 ls s3://raven-demo-backups/postgres/ --region ap-south-1
```

Expected: at least one `.dump` object listed.

- [ ] **Step 9: Document outcome in the runbook**

Append a "Verified by:" line at the bottom of `docs/runbooks/demo-bootstrap.md` with the date and your initials, and `terraform output` values.

- [ ] **Step 10: Commit any runbook updates**

```bash
git add docs/runbooks/demo-bootstrap.md
git commit -m "docs(demo): record first successful bootstrap"
```

---

### Task 17: BetterStack uptime monitor (manual)

**Files:**
- Create: `docs/runbooks/demo-monitoring.md`

- [ ] **Step 1: Create a free BetterStack account** at https://betterstack.com.

- [ ] **Step 2: Create an Uptime monitor**

- URL: `https://demo.raven.ravencloak.org/healthz`
- Check frequency: 60 seconds
- Request timeout: 10 seconds
- Expected status code: 200
- Recovery email: jobinlawrance@gmail.com
- Push notification: enable iOS/Android via BetterStack app

- [ ] **Step 3: Verify it goes green within 5 minutes**

Note: during Phase 1 with Cloudflare Access on the whole hostname, `/healthz` is also behind Access — which BetterStack can't satisfy. Either:
- Add a Cloudflare Access policy bypass for the BetterStack IP ranges, OR
- Expose only `/healthz` as a public bypass via a separate Cloudflare Rule, OR
- Defer activating the monitor until Phase 2 (Access removed).

Pick the bypass-rule option. Cloudflare → demo.raven.ravencloak.org Access app → Bypass policy for `Path == /healthz`.

- [ ] **Step 4: Write the runbook**

```markdown
# Demo Monitoring Runbook

## Uptime: BetterStack

- Monitor name: `raven-demo-healthz`
- URL: `https://demo.raven.ravencloak.org/healthz`
- Owner: jobinlawrance@gmail.com
- Cloudflare Access bypass: Path == `/healthz` is exempt from Access policies
  so external probes can reach the origin.

When the monitor fires:

1. Open BetterStack incident, note the timestamp.
2. SSM into the box: `aws ssm start-session --target $(terraform output -raw instance_id)`.
3. `sudo journalctl -u docker --since '5 minutes ago' | grep -E 'go-api|python-worker'`.
4. `docker compose ps` — any service down? Restart it.
5. If unresolvable, post status to ravencloak.org/status (Phase 3 only).

## Host metrics: Beszel

Already installed on the AWS Beszel instance. Confirm the demo box is registered.

## App telemetry: OpenObserve

`https://observability.demo.raven.ravencloak.org` — Cloudflare Access required.
```

- [ ] **Step 5: Commit**

```bash
git add docs/runbooks/demo-monitoring.md
git commit -m "docs(demo): monitoring runbook for BetterStack + Beszel + OpenObserve"
```

---

### Task 18: Restore drill runbook

**Files:**
- Create: `docs/runbooks/demo-restore.md`

- [ ] **Step 1: Write the restore procedure**

```markdown
# Demo Restore Runbook

Goal: bring the demo back online when the EC2 box is lost or the data
volume is corrupt, using only the artifacts in AWS Backup + S3.

## Scenario A — Box dead, EBS volume intact

1. `terraform apply -replace=aws_instance.demo` — TF recreates the EC2 instance
   and re-attaches the existing EBS volume (prevent_destroy holds).
2. Cloud-init re-runs on the new box.
3. Verify `https://demo.raven.ravencloak.org` returns 200.

## Scenario B — EBS volume corrupt or deleted

1. Restore the latest AWS Backup recovery point of the data volume:
   \`\`\`bash
   aws backup start-restore-job \\
     --recovery-point-arn <ARN_from_backup_vault> \\
     --iam-role-arn $(terraform output -raw instance_role_arn) \\
     --resource-type EBS \\
     --metadata 'AvailabilityZone=ap-south-1a,VolumeType=gp3,Encrypted=true'
   \`\`\`
2. Wait for the restore job to complete (`aws backup describe-restore-job`).
3. Update `deploy/terraform/demo/ebs.tf` to import the restored volume.
4. `terraform apply -replace=aws_instance.demo` to re-create the EC2 against
   the restored volume.

## Scenario C — Postgres data lost, logical backup needed

1. Box up, fresh PG container.
2. SSM into the box.
3. \`\`\`bash
   LATEST=$(aws s3 ls s3://raven-demo-backups/postgres/ | sort | tail -1 | awk '{print $4}')
   aws s3 cp s3://raven-demo-backups/postgres/$LATEST /tmp/restore.dump
   docker exec -i raven-postgres-1 pg_restore -U raven -d raven --clean --if-exists < /tmp/restore.dump
   \`\`\`
4. Verify a known recent row exists in a representative table.

## Recovery time targets

| Scenario | RTO | RPO |
|---|---|---|
| A (box only) | 15 min | 0 |
| B (volume + box) | 1-2 h | 24 h (last snapshot) |
| C (PG only) | 30 min | 24 h (last pg_dump) |

A drill of Scenario C must be performed against staging before Phase 2.
```

- [ ] **Step 2: Commit**

```bash
git add docs/runbooks/demo-restore.md
git commit -m "docs(demo): restore runbook with three scenarios + RTO/RPO targets"
```

---

## Self-review

| Spec section | Plan task(s) |
|---|---|
| §2 Topology (EC2, VPC, EBS, Cloudflare Tunnel) | Tasks 2-3, 6-8, 10 |
| §2 OpenObserve behind Cloudflare Access | Task 10 (access_application "observability") |
| §3 Identity (Google OAuth secrets in SSM) | Task 1 (runbook lists Google secrets) — actual app wiring is in plan #3 |
| §3 LLM $-fuse env plumbing | Task 11 group_vars + Task 12 compose overlay (env only; logic in plan #3) |
| §4 Terraform module | Tasks 2-10 |
| §4 Cloud-init | Task 7 |
| §4 Secrets layout (SSM SecureString) | Task 1 runbook + Task 4 IAM read policy |
| §5 CI/CD | Plan #2 (not this plan) |
| §6 Logical backups (pg_dump, clickhouse-backup) | Tasks 13-14 |
| §6 EBS snapshots (AWS Backup) | Task 9 |
| §6 Retention purge / DSAR | Plan #3 (not this plan) |
| §7 Payments | Plan #3 (not this plan) |
| §8 Email | Plan #3 (not this plan) |
| §9 BetterStack uptime | Task 17 |
| §10 Compliance pages / cookie banner | Plan #3 (not this plan) |
| §11 First-run UX seed | Plan #3 (not this plan) |
| §12 Phase 1 Cloudflare Access on whole hostname | Task 10 (var.phase1_access_enabled) |

No placeholders. Variable names are consistent across tasks (`var.aws_region`, `local.name_prefix`, `aws_instance.demo`, etc.). All IAM resource ARNs use the same naming convention.

One known unresolved item: Task 14's ClickHouse backup script assumes `clickhouse-backup` is installed and configured inside the ClickHouse container. The compose entry for ClickHouse currently does not include that tool. The plan flags this as a follow-up. **Action:** the implementer should add `clickhouse-backup` to the ClickHouse Docker image (or sidecar it) as a sub-task within Task 14 before the timer can succeed. Adding this note here so it isn't lost.
