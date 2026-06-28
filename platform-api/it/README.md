# Platform-API cross-database integration tests

These tests run the **real** platform-api schema and data-access layer against a
real database engine — **SQLite, PostgreSQL and SQL Server** — so backend-specific
behavior (pagination, multi-table writes, delete cascades) is exercised on every
supported store instead of only on the SQLite path used by the unit tests.

## Layout

The suite is written in Gherkin and run by [godog](https://github.com/cucumber/godog)
(Cucumber for Go), consistent with `gateway/it` and `tests/integration-e2e`.

| Path | Purpose |
|------|---------|
| `src/internal/integration/` | The suite (Go, build tag `integration`). |
| `src/internal/integration/features/*.feature` | The scenarios (Gherkin). |
| `src/internal/integration/suite_test.go` | godog runner (`TestFeatures`) + per-scenario setup. |
| `src/internal/integration/harness.go` | DB selection + schema bootstrap, driven by `IT_DB`. |
| `src/internal/integration/world.go` | Per-scenario state + object-graph seeding. |
| `src/internal/integration/steps_crud.go` | Steps for the repository CRUD lifecycle scenarios. |
| `src/internal/integration/steps_cascade.go` | Steps for the delete-cascade scenarios. |
| `src/internal/integration/steps_pagination.go` | Steps for the pagination / filtered-listing scenarios. |
| `src/internal/integration/steps_llm.go` | Steps for the LLM control-plane (template / provider / proxy) scenarios. |
| `src/internal/integration/steps_mcp.go` | Steps for the MCP control-plane (proxy) scenario. |
| `src/internal/integration/steps_webbroker.go` | Steps for the WebBroker API repository scenario. |
| `src/internal/integration/steps_secret.go` | Steps for the secret repository scenario. |
| `it/docker-compose.postgres.yaml` | Throwaway PostgreSQL for the tests. |
| `it/docker-compose.sqlserver.yaml` | Throwaway SQL Server for the tests. |

The tests run on the host and connect to the database over a published port, so
the same test binary covers all three engines. A fresh database and a fresh
scenario state are created per scenario, so scenarios are independent. Each
scenario also surfaces as an individual `go test` subtest via godog's `TestingT`
integration, so `-run TestFeatures/<scenario>` works.

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
cd src
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
- **LLM control plane** — the provider-template, provider and proxy repositories
  round-trip through create / read / update / list / delete, version resolution
  (`is_latest` flip across a family), and the `provider → template` /
  `proxy → provider` foreign keys. These run entirely at the store layer: **no
  real LLM is contacted and the upstream API key is a dummy string**, so no
  provider credentials are needed on any engine.
- **MCP control plane** — the MCP proxy repository round-trips through
  create / read / update / list (by org and project) / delete, again with a
  **dummy upstream API key** and no real MCP server.
- **WebBroker API store** — the WebBroker (event-broker) API repository
  round-trips through create / read / update / paginated list / project-scoped
  list / delete on every engine.
- **Secret store** — the secret repository round-trips encrypted ciphertext and
  hash, reports type/provider/status defaults, checks existence and count, lists
  with pagination, rotates the ciphertext, and soft-deletes (deprecates) a secret
  that has no active references. This scenario surfaced — and the fix is verified
  by — two latent SQL Server bugs in `SecretRepo` where `List` and the
  soft-delete row-lock used the `LIMIT` keyword (rejected by SQL Server); both now
  use the dialect-aware `PaginationClause` / `FetchFirstClause` helpers.

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
