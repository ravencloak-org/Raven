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
      ami,       # don't replace box when AL2023 publishes a new image
      user_data, # cloud-init runs once; later changes are via Ansible/deploy
    ]
  }
}
