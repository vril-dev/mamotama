# SQLite DB Operations

This document covers practical operations for `WAF_DB_ENABLED=true` deployments.

Default DB path:

- `logs/coraza/mamotama.db`

Related env vars:

- `WAF_DB_ENABLED`
- `WAF_DB_PATH`
- `WAF_DB_RETENTION_DAYS`

## What Is Stored

`waf_events` stores ingested WAF log records (`waf-events.ndjson`) for:

- `/mamotama-api/logs/stats`
- `/mamotama-api/logs/read?src=waf`
- `/mamotama-api/logs/download?src=waf`
- FP tuner latest-event lookup

`waf_config_blobs` stores editable runtime config snapshots for:

- `rules` (base rule files from `WAF_RULES_FILE`)
- `bypass`, `country-block`, `rate-limit`
- `bot-defense`, `semantic`
- `cache`, `crs-disabled`

## Retention

Use `WAF_DB_RETENTION_DAYS` to control automatic pruning.

- `30` (default): keep last 30 days
- `0`: disable pruning

Pruning runs during DB sync (on API calls touching WAF DB pipeline).

## Backup / Restore Script

Use `scripts/db_ops.sh`:

```bash
# Show resolved DB path and file status
./scripts/db_ops.sh info

# Create backup (default: data/logs/coraza/backups/mamotama-<timestamp>.db)
./scripts/db_ops.sh backup

# Or specify backup output path
./scripts/db_ops.sh backup data/logs/coraza/backups/manual.db

# Restore from backup (run while coraza is stopped)
./scripts/db_ops.sh restore data/logs/coraza/backups/manual.db
```

Behavior:

- If `sqlite3` is available, `backup` uses `.backup` for online-safe snapshots.
- If `sqlite3` is not available, it copies `db` and optional `-wal`/`-shm`.
- Relative `WAF_DB_PATH` like `logs/coraza/mamotama.db` is mapped to `data/logs/coraza/mamotama.db`.

## Vacuum / Size Maintenance

When DB file grows after heavy tests:

```bash
./scripts/db_ops.sh vacuum
```

## Recovery

If DB is corrupted or missing:

1. Stop `coraza` (`docker compose stop coraza`).
2. Restore from a known-good backup (`./scripts/db_ops.sh restore <backup_file>`), or move broken DB aside.
3. Start stack again (`docker compose up -d coraza nginx`).
4. Call `/mamotama-api/logs/stats` once to trigger re-ingest from `waf-events.ndjson` if needed.

Rebuild depends on remaining `waf-events.ndjson` history.
