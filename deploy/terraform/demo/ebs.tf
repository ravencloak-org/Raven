resource "aws_ebs_volume" "data" {
  availability_zone = "${var.aws_region}a"
  size              = var.data_volume_size_gb
  type              = "gp3"
  encrypted         = true
  tags = merge(local.tags, {
    Name       = "${local.name_prefix}-data"
    BackupPlan = "raven-demo-daily"
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
