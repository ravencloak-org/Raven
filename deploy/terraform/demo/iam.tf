resource "aws_iam_role" "demo" {
  name = "${local.name_prefix}-instance"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.demo.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "ssm_read" {
  name = "ssm-read-demo-params"
  role = aws_iam_role.demo.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:GetParametersByPath"
      ]
      Resource = "arn:aws:ssm:${var.aws_region}:*:parameter/raven/demo/*"
      }, {
      Effect   = "Allow"
      Action   = "kms:Decrypt"
      Resource = "*"
      Condition = {
        StringEquals = {
          "kms:ViaService" = "ssm.${var.aws_region}.amazonaws.com"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "s3_backups" {
  name = "s3-write-backups"
  role = aws_iam_role.demo.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket",
        "s3:DeleteObject"
      ]
      Resource = [
        aws_s3_bucket.backups.arn,
        "${aws_s3_bucket.backups.arn}/*"
      ]
    }]
  })
}

resource "aws_iam_instance_profile" "demo" {
  name = "${local.name_prefix}-instance"
  role = aws_iam_role.demo.name
  tags = local.tags
}
