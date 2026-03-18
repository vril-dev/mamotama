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

## Retention

Use `WAF_DB_RETENTION_DAYS` to control automatic pruning.

- `30` (default): keep last 30 days
- `0`: disable pruning

Pruning runs during DB sync (on API calls touching WAF DB pipeline).

## Backup

Recommended: snapshot before major rule/config changes.

```bash
cp data/logs/coraza/mamotama.db data/logs/coraza/mamotama.db.bak.$(date +%Y%m%d%H%M%S)
```

If WAL files exist, back up them together:

```bash
cp data/logs/coraza/mamotama.db-wal data/logs/coraza/mamotama.db-wal.bak.$(date +%Y%m%d%H%M%S) 2>/dev/null || true
cp data/logs/coraza/mamotama.db-shm data/logs/coraza/mamotama.db-shm.bak.$(date +%Y%m%d%H%M%S) 2>/dev/null || true
```

## Vacuum / Size Maintenance

When DB file grows after heavy tests, run `VACUUM` with SQLite CLI:

```bash
sqlite3 data/logs/coraza/mamotama.db "PRAGMA wal_checkpoint(TRUNCATE); VACUUM;"
```

## Recovery

If DB is corrupted or missing:

1. Stop stack (`docker compose down`).
2. Move broken DB file aside.
3. Start stack again (`docker compose up -d coraza nginx`).
4. Call `/mamotama-api/logs/stats` once to trigger re-ingest from `waf-events.ndjson`.

Rebuild depends on remaining `waf-events.ndjson` history.
