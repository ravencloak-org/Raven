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
