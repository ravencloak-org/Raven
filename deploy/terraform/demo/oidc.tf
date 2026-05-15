# GitHub Actions OIDC provider (one per AWS account; create if absent)
data "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"
}

# If the provider does not yet exist, comment the data source above and
# uncomment this resource:
# resource "aws_iam_openid_connect_provider" "github" {
#   url             = "https://token.actions.githubusercontent.com"
#   client_id_list  = ["sts.amazonaws.com"]
#   thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
# }

resource "aws_iam_role" "github_demo_deploy" {
  name = "${local.name_prefix}-github-deploy"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = data.aws_iam_openid_connect_provider.github.arn
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = "repo:ravencloak-org/Raven:ref:refs/tags/v*"
        }
      }
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy" "github_demo_deploy" {
  name = "ssm-send-command-demo"
  role = aws_iam_role.github_demo_deploy.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssm:SendCommand",
          "ssm:GetCommandInvocation",
          "ssm:ListCommandInvocations"
        ]
        Resource = "*"
        Condition = {
          StringEquals = {
            "aws:ResourceTag/Environment" = "demo"
          }
        }
      },
      {
        Effect   = "Allow"
        Action   = ["ssm:SendCommand"]
        Resource = "arn:aws:ssm:*::document/AWS-RunShellScript"
      },
      {
        Effect = "Allow"
        Action = [
          "ec2:DescribeInstances",
          "ssm:GetCommandInvocation",
          "ssm:ListCommandInvocations"
        ]
        Resource = "*"
      }
    ]
  })
}

output "github_deploy_role_arn" {
  description = "ARN of the role GitHub Actions assumes for demo deploys"
  value       = aws_iam_role.github_demo_deploy.arn
}
