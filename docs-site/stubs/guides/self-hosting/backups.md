---
title: "Backups"
---

# Backups

Raven persists state across four data layers, but only some carry durable,
irreplaceable data. Backups in the reference deployment are anchored on
**pgBackRest**, run as a sidecar container against PostgreSQL. Everything
else is recomputable, externally backed (object volumes), or out of scope.

## What's backed up

| Layer / artefact          | What it stores                                                       | Backup mechanism in repo                                              | RPO target                |
|---------------------------|----------------------------------------------------------------------|-----------------------------------------------------------------------|---------------------------|
| PostgreSQL data           | Relational + pgvector + RLS state                                    | `pgbackrest` sidecar in `docker-compose.yml` — full + diff + incr     | < 1 hour (hourly `incr`)  |
| PostgreSQL WAL            | Continuous write-ahead log                                           | `archive_command=pgbackrest --stanza=raven archive-push %p` (line 142 of `docker-compose.yml`) | Streaming (PITR-capable) |
| ClickHouse                | Audit logs, analytics                                                | **Not wired.** No `clickhouse-backup` service or `BACKUP TABLE` automation in the repo. | n/a — see below           |
| SeaweedFS                 | Attachments, voice audio, anything written via `internal/storage`    | **Volume-level only.** No application-level backup; relies on `seaweedfs-data` volume snapshots. | Operator-defined          |
| Valkey                    | Cache + Asynq queues                                                 | **Not backed up by design.** Pending Asynq tasks live here too — accept loss on restart. | n/a                       |
| `migrations/*.sql`        | Schema definitions (goose-managed, 30 files at last count)           | Versioned in Git; not data, but the schema state every restore depends on. | n/a                       |

Note on Valkey: `internal/queue/client.go` and `internal/queue/server.go`
both wire Asynq with `asynq.RedisClientOpt{Addr: redisAddr}`. Asynq stores
task state in Valkey, not Postgres — so if Valkey is wiped, in-flight and
scheduled tasks are lost. That is the chosen trade-off; producers must be
idempotent. Do not add Valkey to your backup plan.

## pgBackRest setup

The `pgbackrest` service in `docker-compose.yml` (lines 289–311) runs the
official `pgbackrest/pgbackrest:latest` image as a long-lived sidecar
(`command: ["sleep", "infinity"]`) you `exec` into. Mounts:

| Host path / volume         | Container path                          | Purpose                                |
|----------------------------|-----------------------------------------|----------------------------------------|
| `./backup/pgbackrest.conf` | `/etc/pgbackrest/pgbackrest.conf` (ro)  | Config (also mounted into `postgres`). |
| `./backup/backup.sh`       | `/backup/backup.sh` (ro)                | Wrapper around `pgbackrest backup`.    |
| `./backup/restore.sh`      | `/backup/restore.sh` (ro)               | Wrapper around `pgbackrest restore`.   |
| `pg-data` volume           | `/var/lib/postgresql/data` (ro)         | Read-only access to the live cluster.  |
| `pgbackrest-repo` volume   | `/var/lib/pgbackrest`                   | The backup repository itself.          |
| `pgbackrest-log` volume    | `/var/log/pgbackrest`                   | pgBackRest log files.                  |

PostgreSQL is configured for archive-mode WAL shipping in the `postgres`
service `command:`:

```yaml
command:
  - "postgres"
  - "-c"
  - "archive_mode=on"
  - "-c"
  - "archive_command=pgbackrest --stanza=raven archive-push %p || true"
  - "-c"
  - "wal_level=replica"
  - "-c"
  - "max_wal_senders=3"
```

The `|| true` is deliberate: if pgBackRest is briefly unreachable,
Postgres keeps accepting writes rather than blocking. WAL replay on
restore is then limited to whatever was successfully archived.

### Repository, retention, compression

Defaults from `backup/pgbackrest.conf`:

| Setting                   | Value                            |
|---------------------------|----------------------------------|
| `repo1-type`              | `posix` (local volume `pgbackrest-repo`) |
| `repo1-path`              | `/var/lib/pgbackrest`            |
| `repo1-retention-full`    | `4` (keep last 4 full backups)   |
| `repo1-retention-diff`    | `30` (expire diffs older than 30)|
| `compress-type` / `level` | `zst` / `3`                      |
| `process-max`             | `2`                              |
| Stanza                    | `raven`                          |
| `pg1-host` / `pg1-port`   | `postgres` / `5432`              |
| `pg1-path`                | `/var/lib/postgresql/data`       |

### Backup cadence

`backup/backup.sh` accepts `full | diff | incr` (default `incr`) and is
invoked via `docker compose exec`. The script header documents the
recommended cron pattern (host crontab):

```bash
# Daily full backup at 02:00 UTC
0  2 * * * docker compose -f /path/to/docker-compose.yml exec -T pgbackrest /backup/backup.sh full

# Hourly incremental
0  * * * * docker compose -f /path/to/docker-compose.yml exec -T pgbackrest /backup/backup.sh incr
```

The script:

1. Validates the backup type.
2. Calls `pgbackrest --stanza=raven stanza-create` (idempotent).
3. Runs `pgbackrest --stanza=raven --type=<type> backup`.
4. Prints `pgbackrest info` so the cron log records the new inventory.
5. Runs `pgbackrest --stanza=raven expire` to apply the retention policy
   from `pgbackrest.conf`.

One-shot manual run:

```bash
docker compose exec -T pgbackrest /backup/backup.sh           # incr
docker compose exec -T pgbackrest /backup/backup.sh full      # full
docker compose exec -T pgbackrest pgbackrest --stanza=raven info  # inventory
```

## Restore procedure

`backup/restore.sh` is the wrapper. PostgreSQL **must be stopped** — the
script enforces this with a `pg_isready` probe and refuses to run otherwise.

```bash
# 1. Stop Postgres (workers depending on it will fail their healthchecks
#    and pause; restart them after the restore).
docker compose stop postgres

# 2. Restore latest backup
docker compose exec -T pgbackrest /backup/restore.sh

# 3. Bring Postgres back up — WAL replay happens on startup
docker compose start postgres
```

The script runs `pgbackrest --stanza=raven --delta --link-all "$@" restore`
under the hood, so any extra flags are forwarded.

### Point-in-time recovery (PITR)

```bash
docker compose stop postgres

docker compose exec -T pgbackrest /backup/restore.sh \
  --type=time \
  --target="2026-03-28 12:00:00+00"

docker compose start postgres
```

The `--type=time` and `--target` flags are passed straight through to
`pgbackrest restore`. See the [upstream pgBackRest command
reference](https://pgbackrest.org/command.html#command-restore) for the
full flag set (`--type=immediate`, `--target-action`, etc.) — Raven does
not wrap them.

### Restoring on a fresh host

To rebuild on a new host:

1. Provision the host, check out the repo, populate `.env`.
2. Copy the source `pgbackrest-repo` volume contents onto the new host's
   `pgbackrest-repo` volume.
3. `docker compose up -d postgres pgbackrest`, then stop Postgres and run
   `restore.sh`.
4. Start Postgres, then bring up the rest of the stack.

## Off-host backup

**Status: not wired in the repo.** `repo1-type` is `posix` — backups live
on the `pgbackrest-repo` Docker volume on the same host as the database.
A single host failure (disk, theft, region outage) loses both the live
data and the backup repository.

Recommended (configure separately):

- **S3 / R2 / B2.** pgBackRest natively supports S3-compatible storage.
  Add a second repo to `pgbackrest.conf`:

  ```ini
  [global]
  repo2-type=s3
  repo2-path=/raven
  repo2-s3-bucket=raven-backups
  repo2-s3-endpoint=s3.amazonaws.com   # or r2 / b2 endpoint
  repo2-s3-region=us-east-1
  repo2-s3-key=<access-key>
  repo2-s3-key-secret=<secret-key>
  repo2-retention-full=12
  ```

  pgBackRest will write to both repos on each backup. See [pgBackRest S3
  configuration](https://pgbackrest.org/user-guide.html#quickstart/configure-s3).
- **rsync / restic the `pgbackrest-repo` volume.** Lower-rent alternative:
  on a cron, `rsync` the contents of the volume to remote storage. You
  lose pgBackRest's verification and atomicity but it is one line of bash.

Pick one. Without an off-host copy, the in-repo setup is local-disaster
protection only.

## SeaweedFS volume backup

pgBackRest covers Postgres only. SeaweedFS object data lives in the
`seaweedfs-data` volume (line 316 of `docker-compose.yml`), mounted into
the `seaweedfs-volume` service at `/data`. There is no application-level
backup wired in the repo. Two practical options:

### Option A — offline volume snapshot (simplest)

```bash
# Quiesce writes
docker compose stop seaweedfs-volume seaweedfs-filer seaweedfs-master

# Snapshot the volume to a tarball
docker run --rm \
  -v seaweedfs-data:/data:ro \
  -v "$(pwd)/snapshots":/out \
  alpine tar czf /out/seaweedfs-$(date -u +%Y%m%dT%H%M%SZ).tgz -C /data .

# Bring services back
docker compose start seaweedfs-master seaweedfs-volume seaweedfs-filer
```

Acceptable for small deployments and edge nodes where a few minutes of
write downtime per night is fine.

### Option B — `weed volume.copy` (online, between hosts)

SeaweedFS ships `weed shell` with a `volume.copy` command that can clone
volumes to a second cluster on another host without stopping the source.
See [SeaweedFS replication and
backup](https://github.com/seaweedfs/seaweedfs/wiki/Replication) and
[`volume.copy`](https://github.com/seaweedfs/seaweedfs/wiki/SeaweedFS-Shell-commands).
This is the path for production but is operator-configured — Raven does
not run a second SeaweedFS cluster out of the box.

## ClickHouse backup

**Not wired in the repo.** A `grep` for `clickhouse.*backup` returns
nothing across `docker-compose.yml`, `backup/`, and `deploy/`. The
ClickHouse instance (line 157, image `clickhouse/clickhouse-server`)
mounts only `clickhouse-data:/var/lib/clickhouse` and the init SQL.

If you care about ClickHouse history (audit logs, analytics), the
canonical options are:

- `BACKUP TABLE … TO Disk(...)` / `RESTORE TABLE` — built into
  ClickHouse 22.8+. See [ClickHouse BACKUP
  docs](https://clickhouse.com/docs/en/operations/backup).
- The
  [clickhouse-backup](https://github.com/Altinity/clickhouse-backup)
  sidecar — same model as `pgbackrest` here.
- Volume snapshots of `clickhouse-data` — same idea as the SeaweedFS
  Option A above; works because ClickHouse on-disk format is
  copy-friendly when the server is stopped.

If audit logs are not critical to your deployment, treat ClickHouse as
rebuildable and don't bother.

## Backup verification

A backup you have never restored is a guess. Verify quarterly at minimum:

1. Spin up a sandbox stack with a different project name:
   `docker compose -p raven-restore-test up -d postgres pgbackrest`.
2. Copy a recent `pgbackrest-repo` volume contents into the sandbox.
3. Stop the sandbox Postgres and run `restore.sh`.
4. Start Postgres and sanity-check known tables (`SELECT count(*) FROM
   organizations;`, `SELECT max(created_at) FROM documents;`).
5. Run the integrity probe:

   ```bash
   docker compose -p raven-restore-test exec -T pgbackrest \
     pgbackrest --stanza=raven check
   ```

Run `pgbackrest check` on the production stanza on a cron too. See
[`pgbackrest check`](https://pgbackrest.org/command.html#command-check)
upstream.

## What's NOT backed up

Explicit list, so nobody is surprised:

- **Valkey state** — cache and Asynq queues. Pending and scheduled jobs
  are lost on a Valkey restart. Producers must be idempotent.
- **ClickHouse data** — audit logs and analytics. No backup mechanism is
  configured in this repo (see above).
- **OpenObserve data** (`openobserve-data` volume) — telemetry. Treat as
  ephemeral; rebuild from re-ingestion.
- **LiveKit / SuperTokens internal state** — SuperTokens persists into
  the same Postgres database (see `POSTGRESQL_CONNECTION_URI` on line
  199), so it rides along inside the pgBackRest backups. LiveKit is
  stateless beyond the live session.
- **Traefik ACME data** (`acme-data` volume) — certificates. Lose them
  and Let's Encrypt will reissue. Not worth backing up unless you are
  near rate limits; back up only if you operate at scale.
- **Container filesystems**, `node_modules/`, build caches, `.tmp/`, and
  anything else not in a named volume — by definition ephemeral.
- **`.env`** — not a backup target; treat as a secret and store it in
  your password manager / SOPS / Vault, separately from data backups.
