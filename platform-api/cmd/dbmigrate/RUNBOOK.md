# dbmigrate — Runbook

One-time, offline migration of a Platform API **v1** PostgreSQL database into a fresh **v2**
database (core + EventGateway plugin). Build and run from the pinned v2 revision
(`a2911a091…`, see MIGRATION_MAPPING.md).

## 0. Build

```sh
cd /Users/renuka/git/api-platform-migration/platform-api
go build -o dbmigrate ./cmd/dbmigrate/
```

## 1. Back up (non-destructive; v1 is the rollback point)

The tool never writes to v1. Keep the v1 dump (`choreo-dev-v1-backup.sql`) as the rollback point.

## 2. Stand up the databases

**v1 (source)** — restore the dump into an isolated Postgres on host `:5432`:

```sh
docker run -d --name mig-v1 -e POSTGRES_PASSWORD=admin -e POSTGRES_DB=dbv1 \
  -v "$PWD/choreo-dev-v1-backup.sql:/docker-entrypoint-initdb.d/backup.sql:ro" \
  -p 5432:5432 postgres:15
```

**v2 (target)** — a fresh, empty Postgres on host `:5433`. The tool applies the core **and**
plugin DDL itself (`-init-schema`, idempotent), so no schema mounts are needed:

```sh
docker run -d --name mig-v2 -e POSTGRES_PASSWORD=admin -e POSTGRES_DB=dbv2 \
  -p 5433:5432 postgres:15
```

(Alternatively use the `platform-api-v1` / `platform-api-v2` compose stacks, which seed the v2
schema for you; then pass `-init-schema=false`.)

## 3. Set the subscription-token key

v2's `security.encryption_key` must be the value v1 **actually used** for
`subscription_token` (`database.subscription_token_encryption_key`, else `auth.jwt.secret_key`).
The tool reads it from the environment (never a flag):

```sh
export APIP_MIGRATION_ENCRYPTION_KEY="<the exact 32-byte key v1 used, hex(64) or base64>"
```

If v1 ran on the ephemeral fallback, those tokens are unrecoverable and must be re-issued.

## 4. Dry run (transform + validate, NO writes)

```sh
V1="postgres://postgres:admin@localhost:5432/dbv1?sslmode=disable"
V2="postgres://postgres:admin@localhost:5433/dbv2?sslmode=disable"
OUT=/Users/renuka/google-workspace/platform-api-migration/db-migration/migration-out

./dbmigrate migrate -v1-dsn "$V1" -v2-dsn "$V2" -out-dir "$OUT" -run-id prod -dry-run
```

Review, in `$OUT`:
- `migration-report-prod-dryrun.json` → `dropped_config_fields` (§E): **any field that carries
  data is a STOP** — remap before the live run.
- `quarantine-prod-dryrun.jsonl` → decide each row (fix source, or sign off as loss).
- `flags-prod-dryrun.jsonl` → every truncation / placeholder / synthesized value.

The dry run needs no key; add `-skip-decrypt-check` if `APIP_MIGRATION_ENCRYPTION_KEY` is unset.

## 5. Live run

```sh
./dbmigrate migrate -v1-dsn "$V1" -v2-dsn "$V2" -out-dir "$OUT" -run-id prod
```

Idempotent and resumable: re-run the same command after fixing source data or an interruption
(ON CONFLICT DO NOTHING + the file checkpoint keep handles stable and rows de-duplicated).

## 6. Verify (read-only gate; non-zero exit on FAIL)

```sh
./dbmigrate verify -v1-dsn "$V1" -v2-dsn "$V2" -out-dir "$OUT" -run-id prod
```

Writes `verify-report-prod.json`. The gate passes only when every table reconciles
(`v2 + quarantine == v1`), every transform round-trips, and every quarantined key is resolved in
v2 or listed in `quarantine-signoff.jsonl` ({source_table, source_key} per line).

No `APIP_CP_ENCRYPTION_KEY` is needed (the WebSub HMAC table stays empty, §K.3).

## Flags of note

| flag | default | purpose |
|---|---|---|
| `-dry-run` | off | transform+validate, no writes |
| `-init-schema` | on | apply v2 core + plugin DDL (idempotent) |
| `-source-tz` | `UTC` | timezone of naive v1 TIMESTAMP values |
| `-skip-decrypt-check` | off | skip the mandatory token decrypt guard (dry-runs only) |
| `-populate-artifact-subscription-plans` | off | derive the (redundant) artifact_subscription_plans rows |
| `-audit-marker` | off | emit one "migrated" audit row per org |
| `-migration-epoch` | `2026-01-01T00:00:00Z` | fixed epoch for deterministic synthesized UUIDs |
