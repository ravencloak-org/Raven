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
  default     = "demo.ravencloak.org"
}

variable "demo_path_prefix" {
  description = "URL path prefix the demo app is served under (e.g. /raven). No trailing slash."
  type        = string
  default     = "/raven"
}

variable "observability_hostname" {
  description = "Public hostname for the OpenObserve UI"
  type        = string
  default     = "observability.ravencloak.org"
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
