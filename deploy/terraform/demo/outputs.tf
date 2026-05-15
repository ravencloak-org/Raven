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
