# Raven PostgreSQL Backup — pgBackRest

This directory contains the pgBackRest configuration and scripts for backing up and restoring the Raven PostgreSQL database.

## Architecture

```
┌──────────────┐       WAL archive        ┌──────────────────┐
│  PostgreSQL  │ ──────────────────────▶  │   pgbackrest     │
│  (primary)   │                          │   (sidecar)      │
│              │ ◀── restore ───────────  │                  │
└──────────────┘                          └──────────────────┘
       │                                          │
       ▼                                          ▼
   pg-data volume                         pgbackrest-repo volume
```

- **Continuous WAL archiving**: PostgreSQL streams WAL segments to pgBackRest in real time.
- **Scheduled backups**: A cron job (or manual invocation) triggers full/differential/incremental backups.
- **30-day retention**: Full backups kept for 4 cycles; differential backups retained for 30 days.

## Files

| File | Purpose |
|---|---|
| `pgbackrest.conf` | pgBackRest configuration (mounted at `/etc/pgbackrest/pgbackrest.conf`) |
| `backup.sh` | Run a backup (full / diff / incr) |
| `restore.sh` | Restore from the latest backup or perform PITR |

## Quick Start

### Run a manual backup

```bash
# Incremental (default, fastest)
docker compose exec pgbackrest /backup/backup.sh

# Full backup
docker compose exec pgbackrest /backup/backup.sh full

# Differential
docker compose exec pgbackrest /backup/backup.sh diff
```

### Check backup status

```bash
docker compose exec pgbackrest pgbackrest --stanza=raven info
```

### Restore

> PostgreSQL **must be stopped** before restoring.

```bash
# 1. Stop PostgreSQL
docker compose stop postgres

# 2. Restore latest backup
docker compose exec pgbackrest /backup/restore.sh

# 3. Start PostgreSQL (it will replay WAL automatically)
docker compose start postgres
```

### Point-in-Time Recovery (PITR)

```bash
docker compose stop postgres

docker compose exec pgbackrest /backup/restore.sh \
  --type=time \
  --target="2026-03-28 12:00:00+00"

docker compose start postgres
```

## Scheduling Backups via Cron

Add to the host's crontab:

```cron
# Daily full backup at 02:00 UTC
0 2 * * * cd /path/to/raven && docker compose exec -T pgbackrest /backup/backup.sh full >> /var/log/raven-backup.log 2>&1

# Hourly incremental backup
0 * * * * cd /path/to/raven && docker compose exec -T pgbackrest /backup/backup.sh incr >> /var/log/raven-backup.log 2>&1
```

## Configuration

All settings are in `pgbackrest.conf`. Key parameters:

| Parameter | Value | Description |
|---|---|---|
| `repo1-retention-full` | 4 | Keep last 4 full backups |
| `repo1-retention-diff` | 30 | Keep 30 days of differential backups |
| `compress-type` | zst | Zstandard compression |
| `process-max` | 2 | Parallel backup processes |
| `pg1-path` | `/var/lib/postgresql/data` | PostgreSQL data directory |

## Environment Variables

The following variables in `.env.example` control pgBackRest behavior:

- `PGBACKREST_REPO1_PATH` — Override the backup repository path (default: `/var/lib/pgbackrest`)
- `POSTGRES_PASSWORD` — Required for stanza creation

## Volumes

- **`pgbackrest-repo`** — Stores all backup data and WAL archives. **Back this volume up to off-site storage for disaster recovery.**
- **`pgbackrest-log`** — Log files for audit and debugging.
