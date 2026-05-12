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

```bash
aws ssm put-parameter --region ap-south-1 --type SecureString \
  --name /raven/demo/google_client_secret --value 'PASTE_VALUE'
# ... repeat for each
```

## Cloudflare Tunnel credentials

```bash
cloudflared tunnel login                                           # opens browser
cloudflared tunnel create raven-demo                               # writes ~/.cloudflared/<UUID>.json
aws ssm put-parameter --region ap-south-1 --type SecureString \
  --name /raven/demo/cloudflared_credentials_json \
  --value "$(cat ~/.cloudflared/<UUID>.json)"
aws ssm put-parameter --region ap-south-1 --type String \
  --name /raven/demo/cloudflared_tunnel_id --value '<UUID>'
```

## Terraform apply

```bash
cd deploy/terraform/demo
terraform init
terraform plan -out=demo.tfplan
terraform apply demo.tfplan
```

## Post-apply

- Verify `terraform output instance_id` returns an EC2 id.
- Use SSM Session Manager to connect: `aws ssm start-session --target <instance_id>`.
- Tail cloud-init log: `tail -f /var/log/cloud-init-output.log` until "raven-stack: ok" appears.
- Visit `https://demo.raven.ravencloak.org` — Cloudflare Access should prompt for your email (Phase 1 gate).

## Terraform backend (one-time, before first `terraform init`)

```bash
aws s3api create-bucket --region ap-south-1 --bucket raven-tf-state \
  --create-bucket-configuration LocationConstraint=ap-south-1
aws s3api put-bucket-versioning --bucket raven-tf-state \
  --versioning-configuration Status=Enabled
aws s3api put-bucket-encryption --bucket raven-tf-state \
  --server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
aws dynamodb create-table --region ap-south-1 \
  --table-name raven-tf-locks \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST
```
