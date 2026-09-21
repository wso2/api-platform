# migrationcore

The **one implementation** of the Platform API v1→v2 per-row transform + idempotent
v2 write, shared by two callers:

- **Batch backfill** — `cmd/dbmigrate` (the one-time migrator).
- **Live dual-write intermediate** — the v1 build that mirrors each v1 mutation into
  the v2 DB after the v1 repo write (a separate design; its spec lives outside this repo).

Both call the SAME `UpsertX`/`DeleteX`/`ResolveIdentity` so config-blob reshapes,
type conversions, handle/identity rules and the v2 column layout can never drift
between the backfill and the live path.

## What's here

| file | contents |
|---|---|
| `transform.go` | pure config-blob reshapes (`ReshapeRestAPIConfig`, `ReshapeWebSubConfig`+`PreprocessWebSubRaw`, `ReshapeWebBrokerConfig`, `RemarshalConfig`) and type conversions (`ReinterpretTZ`, `BoolToSmallint`, `TruncateStr`, `Slug`, `ParseTransport`) |
| `core.go` | `Options`, `Reporter`, `Execer`, the idempotent `upsert`/`deleteWhere` engine, `ResolveIdentity`, `DeterministicUUID` |
| `upsert.go` | per-entity `V1Row` structs + `UpsertX` (each writes an entity's FULL v2 footprint — `artifacts` + type row + child rows) |
| `delete.go` | per-entity `DeleteX` (live-only; reproduce v2's cascade shape) |
| `upsert_pg_test.go` | Postgres affordance tests (upsert/update, InsertOnly, delete, incremental identity, dry-run) |

## Design contract

- **Caller owns the transaction.** `UpsertX(ex Execer, row V1Row, opts Options, rep Reporter)`
  takes a `*sql.Tx | *sql.DB`, so one entity's artifact + type + child rows upsert atomically.
- **Parameterized, no globals.** Everything comes via `Options{ EncryptionKey, SourceTZ,
  Epoch, InsertOnly, SkipIdentityUpsert, DryRun }`. The core reads no env/flags.
  **The live path MUST use the same `EncryptionKey` + `Epoch` as the batch backfill**, or
  synthesized UUIDs / token passthrough diverge.
- **Idempotent.** `InsertOnly` → `ON CONFLICT DO NOTHING` (batch); otherwise `DO UPDATE`
  (live UPDATE mirroring). `DeleteX` mirrors deletes.
- **Pluggable reporting.** The core emits flags/quarantine/drops through `Reporter`; the
  batch writes the JSONL files, the live path writes logs + a `v2_dual_write_failures` table.
- **Batch-only concerns stay in `cmd/dbmigrate`:** table iteration + FK order, FK-parent
  gating, handle persist-and-replay, checkpoint/resume, dropped-field discovery, DDL init.

Caller wiring:

```text
batch (cmd/dbmigrate):  Options{InsertOnly:true, SkipIdentityUpsert:true, DryRun:cfg.DryRun}
                        Reporter = the JSONL Run
live (v1 dual-write):   Options{InsertOnly:false, SkipIdentityUpsert:false}
                        Reporter = logs + v2_dual_write_failures
```

## Validated (2026-08-23)

Extracted from the already-validated `cmd/dbmigrate` with a regression gate:

1. **Byte-identical batch behavior** — `dbmigrate migrate -dry-run` on the real choreo dump:
   report aggregates, drops, and quarantine identical to pre-extraction; flags identical
   except the `artifact_gateway_mappings` audit `source_key` was reformatted from the
   batch-only `assoc:<serial-id>` to the natural `artifact|gateway` key (same 108 rows —
   the live path produces that key too).
2. **Live + verify** — refactored batch → fresh v2 → `dbmigrate verify` **PASS (121/1/0)**.
3. **Affordance tests** (`go test ./migrationcore` with `MIGRATIONCORE_TEST_DSN`) — upsert
   INSERT, `ON CONFLICT DO UPDATE`, `InsertOnly` DO-NOTHING, `DeleteX`, incremental
   `ResolveIdentity` (deterministic + idempotent), and `DryRun` (no write) all pass.

## Pin

Built against v2 `internal/model` at commit `a2911a091…`. Deploy the batch, the live build,
and this package from the same revision — struct/DDL skew silently corrupts blobs.
