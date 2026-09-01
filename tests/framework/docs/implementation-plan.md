# integration-v2 framework for api-platform — implementation plan

**Status:** Active implementation. Modelled on product-apim's `integration-v2`
(TestNG + Cucumber + Testcontainers), re-expressed for Go (godog + testcontainers-go) with a
YAML topology layer replacing TestNG's suite XML.

---

## 1. Goal & non-goals

**Goal.** One integration/E2E framework for every api-platform suite, providing:
declarative multi-component topologies, safe parallelism, framework-provisioned infrastructure
(containers + databases + schema), dynamic port mapping, scoped state, resource cleanup, and
coverage collection.

**Non-goals (this plan).**
- The test-authoring Claude skill — parked, but in the delivery plan (Phase 8).
- Replacing unit tests or the Gateway API conformance suite.
- Framework self-test gates equivalent to product-apim's 31 `verify-*.sh` phases — see §7 open items.

---

## 2. Locked decisions

| # | Decision | Choice |
|---|----------|--------|
| A | Container abstraction | **testcontainers-go** — needed for its docker/compose abstractions, dynamic ports, lifecycle, reaping |
| B | Topology declaration | **Custom YAML engine**, not an adopted runner. Go owns component definitions; YAML composes them |
| C | Module layout | **One Go module** at `tests/framework/`, suites beneath it. Avoids product-apim's stale-jar trap (their CLAUDE.md §8) |
| D | Feature→runner binding | **In YAML**, not Go runner files. Enables orphan/duplicate detection; product-apim's 131 runner classes are a TestNG constraint, not a design |
| E | Per-runner Go escape hatch | Optional `hook:` naming a registered Go func — added only when something needs it |
| F | Component entry | `name`, `overlay` (optional), `db` (optional), `replicas` (optional) |
| G | Component resolution | `component.db`/`component.version` → matrix and block overrides → `defaults.components`; versions fall back to each product's `VERSION` file |
| H | Store sharing | **Not in YAML.** Declared in the Go component definition (`SharesStoreWith`) because it is a fixed component property |
| I | Multi-instance | `replicas: N`; framework assigns ordinals. Features address by domain language ("the second gateway"), never by infra name |
| J | Wiring | **Typed** per component — load-time validation, consistent with the DB contract |
| K | State | Shared (per-block) + local (per-runner) scope carried on `context.Context`, which godog already threads |
| L | Parallelism | 2 levels: blocks (`t.Parallel` + semaphore) → runners (`t.Parallel`). Scenarios inside a runner are ALWAYS sequential in declared order — that guarantee is what lets a setup feature build fixtures for the features after it, so it is structural, not tunable. |
| M | Feature order | **List order in YAML is execution order** — explicit, unlike cucumber-testng's lexicographic sort |
| N | Taxonomy | product-apim's tag grammar (`@cap`/`@feat`/`@rule`/`@type`/`@dep`, exclusion markers) with an api-platform capability map |
| O | CI | **One workflow, one suite**; DB variants are blocks via `matrix`. Replaces the 3 separate gateway workflows |
| P | Migration | **Per-block cutover** — old suite's feature list shrinks as each block lands; exactly one owner per feature throughout |
| Q | Storage isolation | **One database server per block**, provisioned and torn down by the block via testcontainers. Sharing a server across blocks is a possible optimization, deliberately NOT taken now — it is premature, and it would require block-scoped database names (today's names derive from component + ordinal, which is unique only within a block) |
| R | CI runner | **`ubuntu-latest`** (standard GitHub-hosted, 4 vCPU / 16 GB). Same hardware product-apim's integration-v2 already runs 28 blocks on |
| S | Kubernetes scope | **Operator and Gateway API conformance are OUT of scope** for the framework. Both run solely on Kubernetes and would need a Kind-based topology provider rather than docker-network container composition; there are zero `.feature` files under `kubernetes/`. Backlogged, not cancelled |
| T | Unit tests & coverage wiring | **Deferred until the integration framework is stable.** Several suites never run in CI at all; that is a separate problem from this framework |

### Empirical basis for the concurrency decision

Measured from CI history rather than estimated, on identical standard runners:

| Suite | Shape | Wall-clock |
|---|---|---|
| product-apim `integration-v2` | 28 blocks, 594 scenarios, 2 concurrent containers, ONE runner | **81–93 min** |
| product-apim legacy | 4 shards | 183 / 159 / 155 / 113 min → **192 min critical path** |
| api-platform gateway IT | ~800 scenarios, sequential, one topology, ×3 engines as 3 jobs | **42 / 42 / 43 min** |

The first row is the important one: a single standard runner already hosts a 28-block suite, with containers heavier than ours (full JVM servers versus Go binaries). **Sharding is therefore a wall-clock optimization, not a requirement**, and the block engine does not need to be sharding-aware from the start. An earlier version of this plan assumed the opposite from memory arithmetic alone.

---

## 3. Target layout

```
tests/framework/
├── go.mod                       ONE module for framework + suites
├── docs/
│   ├── capability-map.yml       closed @cap/@feat vocabulary (drafted)
│   ├── coverage-tree.md         GENERATED
│   └── implementation-plan.md   this file
├── core/
│   ├── actor/                   administrative credentials and actor contracts
│   ├── components/              component definitions, instances, wiring and overlays
│   ├── topology/                YAML schema, loader, validation and selection
│   ├── runtime/                 networks, databases, containers and block lifecycle
│   ├── cleanup/                 owner-tagged resource registry
│   ├── util/                    shared HTTP, retry, context and unique-name helpers
│   └── taxonomy/                capability-map lint + coverage-tree renderer
└── suites/
    ├── gateway/                 it-suite.yaml + features/ + steps/
    ├── event-gateway/
    ├── e2e/
    ├── cli/
    └── portals/
```

---

## 4. Phases

Each phase has an exit criterion that must be demonstrable, not asserted.

### Phase 0 — Foundations
Component contract + the gateway block's containers, driven from Go (no YAML yet).

- `Component` definition type: image, exposed ports, network alias, wait strategy, config
  injection, **typed wiring**, **DB contract** (`Supported`, `Schema` per type, `SelfMigrates`,
  `SharesStoreWith`, `Env(DSN)`), accessor set.
- Registry + wrappers for: `gateway-controller`, `gateway-runtime`, `mock-backend`, `mock-jwks`,
  `mock-platform-api`, `mock-analytics-collector`, `mock-interceptor-service`,
  `mock-embedding-provider`, `mock-aws-bedrock-guardrail`, `mock-azure-content-safety`,
  `mcp-everything-streamable`, `redis`.
- One shared docker network per test process; alias+canonical port inside, **ephemeral mapped
  port outside**; no port offsetting. Debug port is the sole fixed-port exception (`IT_DEBUG_PORT`).
- Two-phase readiness: container wait strategy, then app-level health poll requiring **200**.
- Config layering: component's shipped config + shared base overlay + optional block overlay,
  deep-merged.
- DB engine: one container per non-embedded type per block; sqlite embedded (no container);
  generated database names + per-block credentials; schema applied via the right mechanism per
  type (Postgres initdb hook, SQL Server one-shot, none for SQLite); **schema-applied as its own
  readiness phase**; arm64 image substitution for SQL Server.

**Exit:** boot the gateway-core topology programmatically on all three DB types, reach health
200 via ephemeral ports, tear down clean. Two topologies boot concurrently without collision.

### Phase 1 — YAML engine
- Schema + loader + validation: unknown component, unsupported `db` for a component, duplicate
  block name, orphan/duplicate feature binding, malformed wiring key.
- Block engine: block→subtest with `t.Parallel()` under a semaphore; runner→`godog.TestSuite`
  with `t.Parallel()` among siblings; godog `Concurrency` pinned to 1.
- Lifecycle: boot → readiness → publish accessors → run → teardown. **Boot failure fails the
  block, never skips it.**
- Fixed-alias serialization via per-alias semaphores (flip-then-release), so only contending
  blocks serialize.
- Runner CLI: `-blocks=`, `-feature-tags=` (NOT `-tags`), `-block-parallel=` (NOT `-parallel`) — both collide with go test builtins — `--list`.

**Exit:** a 2-block YAML runs under `go test`; blocks run concurrently; a deliberately broken
block reports a **failure** with the boot cause, siblings still pass, and no container leaks.

### Phase 2 — Test primitives
- Scoped context: shared (per-block, read-only after boot) + local (per-runner) with
  `resolve`/`get`/`contains` intents.
- `Requests` funnel: clear-response-before-call so a throw leaves it absent, not stale.
- Raw HTTP chokepoint for intermediate reads that must not publish.
- Three retry contracts on one loop: result-is-assertion / self-healing prerequisite /
  settled-counter, with a single propagation ceiling as a **floor** on every deadline.
- `Names.unique` for collision-free resource naming.
- `ResourceCleanup`: owner-tagged registration, FK-safe deletion order, idempotent, runs on
  failure, WARN on non-2xx.
- Accessor helpers replacing every hardcoded URL.

**Exit:** one migrated feature runs green in two parallel blocks, twice in a row, with zero
residue and no `Thread.sleep`-equivalent.

### Phase 3 — gateway/it migration
Split deliberately, because the two halves have different risk.

- **3a — de-literalize.** Replace **1689 hardcoded `localhost:<port>` URLs across 60 of 70
  feature files** with path-based steps resolving through accessors. Mechanical, sweepable,
  unblocks parallelism. Nothing is parallel-safe until this lands.
- **3b — isolate & tag.** Per feature: unique naming, owned resources, cleanup wiring,
  `@cap`/`@feat` tags, `_setup_*` extraction where a fixture is shared. This is the genuinely
  per-scenario work and the real cost of the project.

**Exit:** all 70 features run under v2 across the three DB blocks; legacy `gateway/it` suite
retired; the two orphaned features (`basic-ratelimit`, `llm-policy-path-specificity`) either
wired in or deliberately deleted.

### Phase 4 — Taxonomy tooling
- `capability-map.yml` lint + `coverage-tree.md` renderer, **in Go** (so it is `go test`-able and
  CI-wired, not a Python side-car).
- Validations: exactly one `@cap`, exactly one `@feat`, pair exists in the map, valid `@type`,
  valid `@dep`, bidirectional `_setup_*` ⟺ `@setup`.
- Plus what product-apim cannot do: **every feature file bound to exactly one runner** —
  orphan and duplicate detection.
- Non-zero exit; CI gate.

**Exit:** tree renders with `invalid: 0` and `orphans: 0`; gate fails on an injected bad tag.

### Phase 5 — remaining Go suites
`event-gateway/it` (6 features, +Kafka), `tests/integration-e2e` (8, multi-component + mixed DB),
`cli/it` (5, +the two orphaned CLI suites). Retire the duplicated setup/state/retry code in each.

**Exit:** four suites, one framework, one CI workflow.

### Phase 6 — UI suites
Framework provisions the topology and hands accessors to an external Cypress process
(api-portal 22 specs, ai-workspace 11 specs, api-portal REST 37 Jest specs).

### Phase 7 — Coverage
The framework owns build-time instrumentation, graceful-shutdown collection, and separate Go
and Node/V8 report generation. Product HTTP coverage endpoints are not part of this design.

### Phase 8 — Test-authoring skill
Parked; in the delivery plan.

---

## 5. Ordering rationale

Phase 0 before 1 because the engine needs something real to orchestrate. Phase 2 before 3
because migrating features against missing primitives means migrating twice. **3a before 3b**
because de-literalization is a prerequisite for any parallel verification of 3b. Phase 4 after 3b
because the lint is only meaningful once features carry tags — but the capability map is drafted
*now* so features are tagged during migration rather than retro-tagged.

---

## 6. Risks

| Risk | Mitigation |
|---|---|
| **1689 hardcoded URLs / 60 files** — the single largest work item | Scripted sweep in 3a, reviewed per file; path-based steps prevent reintroduction |
| **~800 scenarios written against global mutable state** | Per-block cutover; `Names.unique` + cleanup enforced before 3b starts |
| **Wall-clock: 28+ blocks in one CI job** | `defaults.parallel` is the tuning knob; measure early, shard by `--blocks` if needed |
| Package-level vars are Go's version of scope leakage | Lint/review rule from day one; the current global `testState` is the anti-pattern |
| SQL Server on arm64 | Framework substitutes `azure-sql-edge` automatically instead of a manual `-f` overlay |
| Host resource exhaustion with parallel topologies | Per-container CPU/memory limits in component definitions; semaphore bound |
| Two suites live at once during migration | Per-block cutover keeps exactly one owner per feature; orphan check enforces it |

---

## 7. Open items

1. **Framework self-tests.** Explicitly declined. My recommendation stands that ~5 unit tests on
   scope isolation, boot-failure-is-red, and loader validation are worth it, because those failure
   modes are silent (they show green). Not in the plan unless you say so.
2. **`gateway-control` vs `platform-api` boundary** in the capability map — `api-management`
   exists on both surfaces.
3. **`operator/conformance`** may not belong in the coverage tree at all (the upstream suite is
   not Gherkin).
4. **Parity tagging during cutover** — product-apim needed `@legacy:`; here old and new share
   feature files, so probably unnecessary.
5. **Timeout configurability** — product-apim keeps timeouts as constants with recorded
   measurements rather than config. Recommend the same.

## 8. Source builds and coverage ownership

The framework owns test-image selection, source builds, and coverage collection. Product
Dockerfiles remain the canonical image definitions; the framework does not maintain copies.
Catalog packages provide product-specific build metadata, while the shared builder executes
that metadata.

### Executable task list

1. **Inventory and baseline**
   - Record the four product source roots, canonical Dockerfiles, image names, and version
     files.
   - [x] Identify every product-side change introduced solely for framework coverage.
   - Capture baseline unit, integration, and race-test results.

2. **Build contract**
   - [x] Add a shared build specification and executor under `core/builder`.
   - [x] Add catalog build specifications for `platformgateway`, `platformapi`, `aiworkspace`,
     and `apiportal`.
   - [x] Validate source roots, Dockerfiles, image outputs, versions, and coverage capability.
   - [x] Make execution concurrency-safe and test it with a fake runner under `-race`.

3. **Image selection**
   - [x] Keep explicit YAML/CLI versions in pull-only mode.
   - [x] Build from checked-out source only when no explicit version is supplied.
   - [x] Use the product `VERSION` value as the image tag.
   - [x] Reject `-gateway-version` combined with coverage mode.

4. **Framework-owned instrumentation**
   - [x] Remove the gateway `/debug/coverage` endpoint, coverage configuration, and shared
     `common/covdump` implementation from the product.
   - [x] Use build-time Go instrumentation for Go services where the product build supports it.
   - [x] Treat API Portal’s Node/V8 coverage as a separate capability; do not report it as
     Go coverage.
   - [x] Keep product Dockerfiles canonical and avoid duplicate framework Dockerfiles.

5. **Coverage collection**
   - [x] Collect Go and Node/V8 data from containers after graceful shutdown.
   - [x] Remove endpoint-specific dump code and configuration.
   - [x] Preserve per-block and per-service isolation during concurrent runs.
   - [x] Make collection failures diagnosable and non-destructive to sibling block cleanup.

6. **Test coverage**
   - Maintain one unit-test file per new module.
   - [x] Add unit tests for all four build specifications, invalid paths, unsupported coverage,
     version precedence, duplicate invocations, and command failures.
   - [x] Add integration tests that build and validate each product image when Docker is available.
   - [x] Add concurrent build tests and run them with `go test -race`.
   - [x] Add concurrent collection tests covering isolated stopped containers.

7. **Product and framework validation**
   - Run product unit tests for every changed product package.
   - Run framework unit tests, static analysis, and race tests.
   - [x] Run the gateway catalog integration path and gateway coverage smoke suite.
   - [x] Run coverage-enabled Platform API, AI Workspace, and API Portal suites.
   - [x] Verify no stale coverage endpoint or coverage image suffix remains in active framework paths.

8. **Documentation and handoff**
   - [x] Update framework and coverage documentation to describe the two image modes.
   - [x] Document graceful-shutdown collection and the Node/V8 capability boundary.
   - [x] Review the final diff and commit only after all required checks pass.
