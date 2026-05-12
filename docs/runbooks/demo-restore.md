# Demo Restore Runbook

Goal: bring the demo back online when the EC2 box is lost or the data volume is corrupt, using only the artifacts in AWS Backup + S3.

## Scenario A — Box dead, EBS volume intact

1. `terraform apply -replace=aws_instance.demo` — TF recreates the EC2 instance and re-attaches the existing EBS volume (`prevent_destroy` holds).
2. Cloud-init re-runs on the new box.
3. Verify `https://demo.raven.ravencloak.org/healthz` returns 200.

## Scenario B — EBS volume corrupt or deleted

1. Restore the latest AWS Backup recovery point of the data volume:
   ```bash
   aws backup start-restore-job \
     --recovery-point-arn <ARN_from_backup_vault> \
     --iam-role-arn $(cd deploy/terraform/demo && terraform output -raw instance_role_arn) \
     --resource-type EBS \
     --metadata 'AvailabilityZone=ap-south-1a,VolumeType=gp3,Encrypted=true'
   ```
2. Wait for the restore job to complete (`aws backup describe-restore-job`).
3. Update `deploy/terraform/demo/ebs.tf` to import the restored volume or use `terraform import aws_ebs_volume.data <new-volume-id>`.
4. `terraform apply -replace=aws_instance.demo` to re-create the EC2 against the restored volume.

## Scenario C — Postgres data lost, logical backup needed

1. Box up, fresh PG container.
2. SSM into the box.
3. Pull the latest dump from S3 and restore:
   ```bash
   LATEST=$(aws s3 ls s3://raven-demo-backups/postgres/ | sort | tail -1 | awk '{print $4}')
   aws s3 cp s3://raven-demo-backups/postgres/$LATEST /tmp/restore.dump
   docker exec -i raven-postgres-1 pg_restore -U raven -d raven --clean --if-exists < /tmp/restore.dump
   ```
4. Verify a known recent row exists in a representative table.

## Recovery time targets

| Scenario | RTO | RPO |
|---|---|---|
| A (box only) | 15 min | 0 |
| B (volume + box) | 1–2 h | 24 h (last snapshot) |
| C (PG only) | 30 min | 24 h (last pg_dump) |

A drill of Scenario C must be performed against staging before Phase 2.
