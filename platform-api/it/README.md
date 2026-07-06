# Platform-API cross-database integration tests

These tests run the **real** platform-api schema and data-access layer against a
real database engine — **SQLite, PostgreSQL and SQL Server** — so backend-specific
behavior (pagination, multi-table writes, delete cascades) is exercised on every
supported store instead of only on the SQLite path used by the unit tests.

## Layout

| Path | Purpose |
|------|---------|
| `internal/integration/` | The tests (Go, build tag `integration`). |
| `internal/integration/harness_test.go` | DB selection + schema bootstrap, driven by `IT_DB`. |
| `internal/integration/cascade_test.go` | Delete-cascade behavior across the real foreign keys. |
| `internal/integration/lifecycle_test.go` | Create + paginated list through the real repository layer. |
| `it/docker-compose.postgres.yaml` | Throwaway PostgreSQL for the tests. |
| `it/docker-compose.sqlserver.yaml` | Throwaway SQL Server for the tests. |

The tests run on the host and connect to the database over a published port, so
the same test binary covers all three engines.

## Running

```bash
# from platform-api/
make it             # SQLite (no container needed)
make it-postgres    # spins up PostgreSQL, runs, tears down
make it-sqlserver   # spins up SQL Server, runs, tears down
make it-all-dbs     # all three in sequence
```

On Apple Silicon the default SQL Server image fails under emulation; use Azure
SQL Edge instead:

```bash
MSSQL_IMAGE=mcr.microsoft.com/azure-sql-edge:latest make it-sqlserver
```

### Selecting the engine manually

The suite is parameterized purely by environment variables, so it can also point
at an already-running database:

```bash
cd ..
IT_DB=sqlserver IT_DB_HOST=localhost IT_DB_PORT=1433 \
  IT_DB_USER=sa IT_DB_PASSWORD='Strong!Passw0rd' IT_DB_NAME=platform_api_it \
  go test -tags integration -v ./internal/integration/...
```

`IT_DB` ∈ `sqlite | postgres | sqlserver`. For postgres/sqlserver, set
`IT_DB_HOST`, `IT_DB_PORT`, `IT_DB_USER`, `IT_DB_PASSWORD`, `IT_DB_NAME`
(the SQL Server database is auto-created if missing).

## What is covered

- **Pagination** through the real repositories — the path that previously failed
  on SQL Server with `Incorrect syntax near 'LIMIT'` (SQL Server uses
  `OFFSET/FETCH`, not `LIMIT`).
- **Delete cascades** on the real foreign keys — confirms the SQL Server schema's
  `NO ACTION` edges (added to avoid SQL Server's multiple-cascade-paths
  restriction) preserve the same cleanup behavior as PostgreSQL/SQLite:
  deleting a REST API removes its subscriptions, deleting a gateway removes its
  deployments and deployment status, deleting a devportal removes its
  publications, deleting an application removes its mappings, and deleting a
  project removes its applications.

## Relationship to the gateway integration tests

This covers the **platform-api** (control plane) store. The **gateway** has its
own storage and its own cross-database integration suite under `gateway/it`,
already runnable per engine:

```bash
cd gateway/it
make test               # SQLite
make test-postgres      # PostgreSQL
make test-sqlserver     # SQL Server
```

Together these give a per-engine matrix for both components. A combined
end-to-end suite — real platform-api driving the real gateway data plane on a
shared engine — is the next layer; it would reuse the `gateway/it`
docker-compose services with the real platform-api image substituted for the
mock platform-api, and assert an API created in platform-api is served by the
gateway. The DB-selection and bootstrap conventions here are designed to extend
to that.
