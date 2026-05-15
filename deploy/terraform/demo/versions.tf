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
