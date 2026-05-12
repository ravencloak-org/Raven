resource "aws_backup_vault" "demo" {
  name = "${local.name_prefix}-vault"
  tags = local.tags
}

resource "aws_iam_role" "backup" {
  name = "${local.name_prefix}-backup"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "backup.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "backup" {
  role       = aws_iam_role.backup.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSBackupServiceRolePolicyForBackup"
}

resource "aws_backup_plan" "demo" {
  name = "${local.name_prefix}-daily"

  rule {
    rule_name         = "daily-ebs"
    target_vault_name = aws_backup_vault.demo.name
    schedule          = "cron(0 18 * * ? *)" # 18:00 UTC = 23:30 IST
    lifecycle {
      delete_after = 14
    }
  }

  tags = local.tags
}

resource "aws_backup_selection" "demo" {
  iam_role_arn = aws_iam_role.backup.arn
  name         = "${local.name_prefix}-selection"
  plan_id      = aws_backup_plan.demo.id

  selection_tag {
    type  = "STRINGEQUALS"
    key   = "BackupPlan"
    value = "raven-demo-daily"
  }
}
