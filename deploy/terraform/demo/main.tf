provider "aws" {
  # Retained solely so the legacy aws_ssm_parameter.tunnel_credentials /
  # aws_ssm_parameter.tunnel_id resources still living in cloudflare.tf can
  # be torn down by the post-cutover terraform destroy. Will be removed in
  # the follow-up tunnel-import PR once those SSM params are deleted from
  # state.
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
