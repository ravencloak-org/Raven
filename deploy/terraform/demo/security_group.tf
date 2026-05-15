resource "aws_security_group" "demo" {
  name        = "${local.name_prefix}-sg"
  description = "Egress-only SG for the Raven demo box. Cloudflared dials out - no inbound."
  vpc_id      = aws_vpc.demo.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "All outbound - needed for image pulls, Cloudflared, SSM, OAuth, LLM APIs"
  }

  tags = merge(local.tags, { Name = "${local.name_prefix}-sg" })
}
