# platform-api v1 → v2 database migration client

A one-shot Go client that reads the **platform-api v1** Postgres database and
writes the transformed rows into the **platform-api v2** database (including the
EventGateway plugin schema). The column-level transform is specified in
`bijira-migration/db-migration-client/DB_MAPPING.md` and its companion Claude
artifact; this client implements it faithfully.

It lives **inside the v2 module** on purpose: the transform reuses v2's own
`internal/` helpers by import — deterministic uuidv7 (`utils.GenerateDeterministicUUIDv7`),
handle slugging (`utils.GenerateHandle`), vault AES-GCM + HMAC (`vault.InHouseVault`),
and key derivation (`utils.DeriveEncryptionKey`) — so the migrated bytes match
what v2's own create path would have produced. Living anywhere else would mean
re-implementing that crypto/uuid/handle code and risking silent divergence.

## Layout

```
cmd/migrate/main.go            entrypoint (flag parsing + Run)
internal/migration/
  config.go / flags.go         config + CLI/env parsing (secrets are env-only)
  db.go                        database/sql + pgx stdlib connections, tx batching
  logging.go                   slog logger + key redaction (never logs plaintext)
  checkpoint.go                resumability store (minted handles + progress)
  kernels.go                   shared transforms: actor resolve, handle mint/carry,
                               ts→tstz, secret hash, deterministic ids
  config_transform.go          secret externalization (§B.10), LLM policy split
                               (§B.11), transport fold — operate on the config JSON
  idempotency.go               ON CONFLICT / delete-by-parent helpers (§B.9)
  preflight.go                 §B.0 go/no-go gates
  migrator.go / runner.go      interface, FK-ordered registry, orchestration
  resource_helpers.go          batched-tx + dry-run runner, count verification
  res_*.go                     the 25 per-resource migrators
  res_reverse.go               v2→v1 reverse for the cleanly-invertible tables (§13)
  kernels_test.go              unit tests for the deterministic kernels
```

## Build

```sh
make migrate-build     # host platform → ./migrate, prints `./migrate --version`
make migrate-dist      # operator bundles → dist/migrate/*.tar.gz + SHA256SUMS
go build ./cmd/migrate # plain dev build (version falls back to Go's embedded VCS info)
```

`--version` prints the build identity, which is also the first thing a run logs
(`migration starting … build="…"`), so an operator's log proves which build ran:

```
migrate v20260929-r1 (commit 0171c336c05f, built 2026-09-30T13:31:37+05:30, go1.27.1 linux/amd64)
```

`make migrate-dist` cross-compiles static binaries (`CGO_ENABLED=0 -trimpath
-buildvcs=false`, version/commit/commit-date stamped via `-ldflags -X`) for
`linux/amd64 linux/arm64 darwin/amd64 darwin/arm64` and packs each with a `README.txt`
and a per-bundle `SHA256SUMS`; `dist/migrate/SHA256SUMS` covers the tarballs. (The v2
schema is NOT in the bundle: provisioning the target schema is the v2 deployment's job.) Builds are **reproducible**: rebuilding at the same commit with the same Go
toolchain yields a byte-identical binary (verified), so anyone can audit a shipped bundle.

## Configuration

**Passwords and the encryption key are read from the environment ONLY** — never
CLI flags (flags leak into `ps` and shell history) and are never logged.

| env var | meaning |
|---|---|
| `V1_DB_PASSWORD` | v1 (source) DB password |
| `V2_DB_PASSWORD` | v2 (target) DB password |
| `V2_ENCRYPTION_KEY` | v2's single `security.encryption_key` (64-hex or base64 → 32 bytes). Per the Gate-2 decision this **is v1's subscription-token key bytes**, so copied token ciphertext decrypts by design. It feeds ALL at-rest crypto: subscription-token decrypt, secret-vault AES-GCM, and the `secrets.hash` HMAC. |

Key flags (`migrate -h` for the full list):

| flag | meaning |
|---|---|
| `--v1-host/-port/-db/-user/-sslmode` | v1 (source) connection |
| `--v2-host/-port/-db/-user/-sslmode` | v2 (target) connection |
| `--v1-timezone` | **required** — the v1 **app server's** IANA zone (e.g. `Asia/Colombo`). pgx stored bare `time.Now()` as the app's local wall-clock digits with the zone discarded, so this is the zone `tsToTstz` interprets the ~130 app-bound TIMESTAMP columns in before converting to UTC (§B.3). Use `UTC` only if the v1 fleet provably ran `TZ=UTC`. Determine it from the v1 app **container/host `TZ`**, not the DB. |
| `--org-handles-csv` | **required (forward)** — Choreo **org** handle export (`org_uuid,org_handle[,org_status,org_name]`, from `cr/test/export-org-handles.sh`). v2 (REST path ids + the AI Workspace URL) addresses orgs **by handle** and v1's handles are random strings, not Choreo's — so the migrated handle is taken from this file (§B.14, §B.0 Gate 5). |
| `--project-handles-csv` | **required (forward)** — Choreo **project** handle export (`project_uuid,project_handler`, from `cr/test/export-project-handlers.js`). Projects Choreo does not know (v1's auto-created `default` rows) fall back to a handle minted from the v1 name, **suffixed** if it collides with a Choreo handle in the same org (§B.14). |
| `--handles-report` | CSV written at the end of **every forward run, dry-run included** (default `migration-handles.csv`): one row per organization/project — `kind,uuid,organization_uuid,v1_value,v2_handle,v2_display_name,source`, where `source` is `choreo-export`, `v1-handle-kept` (org missing from the export), `minted-from-v1-name` or `minted-from-v1-name-suffixed` (project missing from the export). Non-Choreo rows sort first within each block; the file is `0600` (handles are PII). Use it to resolve the non-Choreo rows against Choreo by uuid and fix the export before the real run (§B.14). |
| `--migration-actor-uuid` | synthetic actor for v2 NEW audit columns with no v1 source (default `019b76da-a800-7a6c-aad4-385ed4394247`, §B.4) |
| `--direction forward\|reverse` | `forward` = v1→v2 (default); `reverse` = v2→v1 reconciliation (§13) |
| `--dry-run` | full transform + verification with **zero writes** |
| `--verify-only` | run §9 verification against an already-migrated target |
| `--resume` + `--checkpoint-file` | resumable; reuses checkpointed minted handles/UUIDs |
| `--validate-keys` | (default on) preflight token-parity + vault round-trip (§B.0 Gates 2 & 4) |
| `--resources` / `--skip` | optional per-resource selectors (FK-dependency-checked) |
| `--batch-size` | optional rows per tx batch (default ~750; never 1) |
| `--continue-on-error` | do not fail-fast; log and continue past a failed batch |
| `--log-level` / `--log-format` | `debug\|info\|warn\|error` / `json\|text` |

## Running

**Always dry-run first**, then a real run:

```sh
export V1_DB_PASSWORD=… V2_DB_PASSWORD=… V2_ENCRYPTION_KEY=…

# 1. Dry-run: full transform + verification, zero writes to v2.
migrate --v1-host … --v1-db plat_v1 --v1-user … \
        --v2-host … --v2-db plat_v2 --v2-user … \
        --v1-timezone Asia/Colombo \
        --org-handles-csv ./org_handles.csv --project-handles-csv ./project_handlers.csv \
        --dry-run

# 2. Real run (resumable).
migrate … --v1-timezone Asia/Colombo \
        --org-handles-csv ./org_handles.csv --project-handles-csv ./project_handlers.csv \
        --resume --checkpoint-file ./migration.ckpt

# 3. Re-verify an already-migrated target (with the CSVs: also asserts handle parity).
migrate … --verify-only --org-handles-csv ./org_handles.csv --project-handles-csv ./project_handlers.csv
```

The run:
1. Enforces the **§B.0 preflight gates** against v1 (forward only): length limits
   (handles ≤40, issuer/allowed_targets ≤255), duplicate-subscription scan,
   **Choreo handle maps** (coverage / collisions / v2-format, §B.14 — Gate 5),
   subscription-token key smoke-test, vault-key round-trip. Any hard-gate failure
   aborts before a single v2 row is written.
2. Walks the 25 resources in **FK-safe order**, each: read v1 → transform (kernels)
   → idempotent write v2 → verify.
3. Prints a per-resource **PASS / WARN / FAIL** report (row-count parity + notes).

Every write is **idempotent** (UUID `ON CONFLICT DO NOTHING`, natural-key upsert,
partial-index-aware, or delete-by-parent for the SERIAL `gateway_endpoints`), so a
re-run is safe; minted handles are checkpointed so a resumed run reuses them.

**Checkpoint hygiene:** minted handles in the checkpoint are reused by *every* run
(resume-safe by design), so a leftover `migration-checkpoint.json` from another run or
an older export silently steers a new one. Every run therefore logs `checkpoint loaded
… mintedHandles=N completedResources=M` first, WARNs when completed state is found
without `--resume`, and the project planner **hard-fails** if a checkpointed handle
collides with a handle planned in the same org (a fresh mint never can). Start a new
migration with a fresh `--checkpoint-file`.

## What is NOT migrated

- Choreo APIM's own DB (handles are carried byte-for-byte; the handle *is* Choreo's
  UUID — §B.12).
- Dropped resources: `devportals`, `publication_mappings`, the `dev_portal`
  association rows, `gateway_states`, `events`.
- Intentionally-empty tables: `user_organization_mappings`, `audit`,
  `artifact_subscription_plans` (no v2 writer / not read by audit resolution).

`secrets` / `secret_scopes` / `artifact_secret_refs` are **not** a separate pass —
they are emitted inline while each artifact's inline credential is externalized
(§B.10); the `secrets` step is a verify-only integrity check.

## Reverse migration (§13)

Reverse (`--direction reverse`) is **best-effort and NOT the primary rollback** —
the primary rollback is keeping v1 untouched (the forward run never writes to v1)
plus a pre-cutover backup: to abandon a cut-over, point everything back at v1 and
re-provision v2 from scratch. Reverse exists for **reconciliation**: bringing rows
*created in v2 after cut-over* back into v1. Roles swap (read v2, write v1); the
walk order stays **parents-first** (it still inserts, so FK parents must exist —
a children-first "undo" order broke on `applications_organization_uuid_fkey`);
the same idempotent (`ON CONFLICT (uuid) DO NOTHING`) / dry-run / verify machinery
applies. **Implemented (4 tables):** organizations, projects, applications,
subscriptions — actor UUIDs are recovered from `user_idp_references.idp_id`
(migration actor → `''`), timestamps inverted to naive wall-clock, v2-NEW columns
(`updated_by`, minted handles, `data_version`) dropped. **Everything else is
`ErrReverseUnsupported`** (artifacts + all type rows, templates, plans, gateways,
tokens, api_keys, deployments, mappings; secret re-inline) — logged and SKIPped,
never fabricated. v2-only tables (`user_idp_references`, `secrets`) skip Verify in
reverse. Reverse is insert-only: it never overwrites a v1 row (a migrated org keeps
its v1 handle/name in v1) and never deletes from v2. A row v1's stricter FKs reject
(e.g. a subscription whose plan belongs to another org — v1 has a composite
`(plan, org)` FK, v2 does not) fails that resource's batch; use `--continue-on-error`
to carry on.

Tested recipe (never against the real v1 — clone it):
```sh
docker exec platform-api-v1-db psql -U postgres -c "create database dbv1_reverse template dbv1"
migrate --direction reverse --v1-db dbv1_reverse … --v2-db dbv2 … --v1-timezone UTC --dry-run   # no CSV flags
migrate --direction reverse --v1-db dbv1_reverse … --v2-db dbv2 … --v1-timezone UTC
```
Expect `PASS=4 SKIP=22`, the clone gaining exactly the v2-created rows, a second run
adding none, and every pre-existing row byte-identical to `dbv1`.

## Tests

```
go test ./internal/migration/...
```

Unit tests cover the deterministic kernels — timestamp zone conversion, the
`hmac-sha256:` secret hash, deterministic uuidv7, handle carry/mint + checkpoint,
the LLM policy split, transport parsing, credential-less skip, and end-to-end
config secret externalization (all DB-free). Full row-level correctness requires a
live v1/v2 pair — run `--dry-run` then `--verify-only` against real databases.
```

## Handles & display names — organizations and projects (§B.14)

v2 addresses organizations and projects **by handle** (REST path ids and the AI
Workspace v2 URL), but v1's values are not Choreo's: v1 org handles are random
7-letter strings (`ebfoyrc`) and v1 project names are `default` / random 5-letter
strings (`kyrbv`). The client therefore takes both handles from Choreo's own exports:

| | source | v2 `handle` | fallback when the export lacks the uuid |
|---|---|---|---|
| organizations | `--org-handles-csv` (Choreo App Service `dbo.organization`) | Choreo org handle, **verbatim** | v1 handle kept (WARN — will not match any Choreo URL) |
| projects | `--project-handles-csv` (Choreo Project Manager `am_projects`) | Choreo project handler, **verbatim** | minted from v1 `name` (checkpointed); **suffixed** if it collides with a Choreo handle in the same org |

- Choreo handles always **win** inside an org: project handles are planned up front
  (`planProjectHandles`), so v1's auto-created `default` project becomes
  `default-xxxx` wherever Choreo's own `default` project exists in that org
  (43 of the dev dump's 107 orgs).
- Handles are **never slugified, cased or truncated** — the URL must match Choreo
  exactly. A Choreo handle that v2's create-validation would reject (uppercase,
  `.`) is still written verbatim — v2 serves reads/path params as stored — and
  Gate 5 WARNs about it.
- **`display_name`:** organizations ← the export's `org_name` (Choreo's name; the AI
  Workspace header shows v2's org display_name via the forwarded `GET /organizations/{handle}`),
  falling back to the handle; projects ← the handle (v2's project display_name is shown
  nowhere in Bijira — project names come from Choreo — and the handle keeps the reverse
  path valid against v1's `UNIQUE(name, organization_uuid)`). v1's random names are dropped.
- **Why handles are load-bearing in Bijira:** the AI Workspace talks to Choreo APIM's
  platform-api facade, which lazily mirrors orgs/projects into v2 by handle
  (`ensureOrganization` / `ensureProject`: `GET /projects/{handle}` → 404 → `POST`). A migrated
  row under any other handle gets a **duplicate** created beside it, orphaning its proxies.
- Every forward run writes the **handles report** (`--handles-report`): the complete uuid → v2 handle map
  with a `source` column, so the rows that did NOT get a Choreo handle are a one-line filter
  (`grep -v ',choreo-export$'`). The per-project "not in the Choreo export" gate lines are DEBUG; the
  report is the actionable artifact.
- `--verify-only` with the CSVs asserts **handle parity** (every Choreo-known row
  carries the exported handle). Writes are insert-only, so an export that changed
  after a row's first migration is NOT re-applied — parity is what surfaces it.
