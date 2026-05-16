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

  # Local state — solo-operator demo, no team concurrency to protect
  # against. The state file lives at deploy/terraform/demo/terraform.tfstate
  # on the operator's machine (gitignored). If the machine is lost, the
  # demo can be re-created from scratch by re-running `terraform apply`;
  # the EBS data volume has prevent_destroy so re-import after re-create
  # is straightforward. Switch to a remote backend if/when more than one
  # operator ever needs to drive this stack.
}
