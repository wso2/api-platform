# Test Framework Engineering Rules

## Architecture

Integration-test steps should use this structure:

```text
suites/it/steps/
├── base.go
├── request.go
├── assertions.go
├── health.go
│
├── common/
│   ├── request.go
│   ├── json.go
│   └── polling.go
│
└── platformgateway/
    ├── gateway.go
    ├── endpoint.go
    ├── health.go
    ├── resource.go
    ├── policy.go
    └── template.go
```

`base.go` is the entry point for suite construction and step registration.

The root `steps` package contains suite-wide behavior. The `common` package
contains reusable step mechanics that are independent of a product. The
`platformgateway` package contains platform-gateway-specific endpoints,
resources, health checks, policies, and template behavior.

When steps for another product are introduced, place them in a product-specific
package using the same structure. Do not reference product step packages that do
not yet exist, and do not add product-specific behavior to `common`.

Before adding a helper, step, or dependency:

- Search for an existing framework utility or Gherkin step.
- Reuse it when it already supports the behavior.
- Extend it when the new behavior is a natural generalization.
- Add a new implementation only when the existing abstraction cannot express
  the behavior cleanly.
- Avoid separate Gherkin steps for behavior that can be represented by
  parameters such as HTTP method, endpoint, service, or resource type.

## Product and Framework Test Boundaries

- `suites/it` and `suites/ui` contain product user journeys and product
  integration behavior defined by suite YAML.
- Do not use product feature files to test framework scheduling, runner or
  block isolation, cleanup ownership, concurrency orchestration, or topology
  mechanics.
- Test framework capabilities in the framework's Go unit or integration tests,
  primarily under `core`, using real containers only when the capability
  requires them.
- A product scenario may test concurrent product requests only when that
  concurrency is part of user-visible product behavior. Use deterministic
  synchronization; runner parallelism alone is not a concurrency guarantee.
- Keep framework-only tests out of `it-suite.yaml` and `ui-suite.yaml`.

## Step Definitions

- `base.go` is the suite entry point for common step registration.
- Common steps belong in the root `steps` package.
- Product-specific steps belong in a product subdirectory, such as
  `steps/platformgateway`.
- Generalize request, assertion, URL, and polling mechanics instead of
  duplicating them per operation.
- Keep Gherkin step definitions readable and behavior-oriented.
- Do not add multiple step variations when one parameterized step can express
  the behavior.
- Remove unused steps, methods, registrations, and stale references completely.
- Never embed YAML configuration documents inline in feature files. Always use
  the single canonical template for the resource kind and inject all
  scenario-specific data through the Gherkin values table. Do not respond to
  this rule by creating a new YAML file for the scenario.
- Resource templates are named for their resource kind, not for feature files,
  scenarios, runners, migration numbers, ordinals, or individual variations.
  The canonical-template rules, including the narrow malformed-document
  exception, are defined in `## Resource Templates` below.

## Resource Templates

- Maintain exactly one canonical template under
  `suites/it/resources/templates/` for each supported resource kind, such as
  `rest-api.yaml`, `llm-provider.yaml`, or `mcp.yaml`. Do not create a
  feature-, scenario-, runner-, ordinal-, or migration-specific copy of a
  supported resource.
- The canonical template owns the resource envelope: `apiVersion`, `kind`,
  `metadata.name`, and `spec`. Keep the envelope minimal and configurable;
  scenario-specific structure belongs in the Gherkin values table.
- Create and update resources through the template steps:
  `I create ... from "resources/templates/<kind>.yaml" with values:` and
  `I update ... from ... with values:`.
- Supply variation through table values. Use `name` for the generated resource
  name, `apiVersion` for the resource API version, `spec` for a complete
  structured override, or dotted paths such as `spec.displayName` for a
  focused override. Values may contain JSON objects or arrays when a nested
  policy, operation, upstream, or endpoint must be configured.
- For Platform Gateway resource templates, supply `apiVersion` as
  `${CTX:gatewaySpecVersion}`. The Platform Gateway binding publishes that
  scenario-scoped value before steps run, selecting the resource contract for
  the Gateway release under test. Do not hard-code a release-specific Gateway
  resource API version in a feature table unless the scenario is intentionally
  testing an invalid version.
- For a negative case that must omit fields from the final resource document,
  continue to use the canonical template step and provide a complete `spec`
  override containing only the fields the case requires. Supply harmless values
  for the template's other placeholders so rendering succeeds; the complete
  `spec` override replaces the template spec before the request is sent. Do not
  use inline YAML or JSON documents to express this omission.
- The renderer must preserve omission semantics. If an optional field is not
  supplied by the table, it must remain absent from the rendered document;
  do not add placeholder defaults merely to simplify a scenario.
- Reuse the same canonical template across feature files and runners whenever
  the resource kind is the same. Differences in display name, context, version,
  upstream, operations, policies, headers, and invalid values are data, not
  reasons for another template.
- Deliberately malformed documents that must fail before normal resource
  parsing are the only exception. Keep such fixtures rare, document the
  parser-level contract they protect, and do not use them for ordinary valid,
  negative, or policy-specific variations.
- Before adding or changing a template, search all feature references and
  update reusable table values instead of copying YAML. After migration,
  verify that no obsolete resource references remain and that the canonical
  template and renderer tests pass.
- `resources/apis/` is not a second template catalogue. Do not add runtime
  fixtures there when the canonical template can express the resource.

## Version-Aware Product Contracts

- Gherkin steps must describe product behavior, not a version-specific response
  representation. For example, assert that an API was successfully deployed
  instead of asserting that a particular response field is a string.
- Resolve the effective product version for each block before scenarios run and
  make those versions available through the block context used by step bindings.
- A product-specific binding must retrieve the relevant version from that
  context and support every product version declared as supported by the
  framework.
- Platform Gateway bindings publish `${CTX:gatewaySpecVersion}` for resource
  templates. It resolves to `gateway.api-platform.wso2.com/v1alpha1` for
  Gateway `1.1.0` and `gateway.api-platform.wso2.com/v1` for Gateway `1.2.0`
  and current source builds. Keep that mapping in the Platform Gateway step
  package; templates and feature files must consume the context value instead
  of duplicating release checks.
- Platform Gateway bindings publish `${CTX:gatewayMCPUpstreamPath}` for
  testbench MCP fixtures. It is empty for Gateway `1.1.0` and older, which
  append the request's `/mcp` path to the upstream URL, and `/mcp` for Gateway
  `1.2.0` and current source builds. Use it where a scenario configures the
  testbench MCP upstream instead of embedding a release check in a feature.
- Gateway `1.1.0` and older represent LLM operation policies as
  `spec.policies`; Gateway `1.2.0` and newer use `spec.operationPolicies`.
  The Platform Gateway template binding translates the compatible policy shape
  and the LLM-provider assertion binding selects the returned field. Feature
  files must describe the attached-policy behavior, not either representation.
- Gateway `1.1.0` semantic-cache configuration uses the legacy root Redis keys;
  the semantic-AI overlay and Redis provisioner must keep those keys and the
  per-topology password available alongside the current vector-store keys.
  Gateway-version-specific semantic-cache behavior must be isolated in a
  runner selector: the cross-authenticated-caller isolation scenario is gated
  to Gateway `>=1.2.0`, where that contract exists. Do not weaken the
  isolation assertion to make an older Gateway pass.
- Keep version-specific response or request handling in the relevant product
  step package. Do not make generic JSON assertions silently accept multiple
  data types or response shapes.
- Add unit tests for every supported contract and for unsupported or malformed
  version information. A new supported version must include its contract
  behavior before it is added to suite configuration.

## Components and Runtime

- Keep component definitions declarative and independent of runtime orchestration.
- Keep runtime lifecycle behavior in `core/runtime`.
- Keep cleanup registration and retry behavior explicit and bounded.
- Log failed cleanup with enough context to identify the resource, block, runner,
  and attempt.
- Keep configuration overlays separate from component definitions.
- Validate nil, zero-value, duplicate, invalid, and unsupported inputs at
  package boundaries.

## Tests

- Each module must have one unit-test file containing all unit tests for that
  module.
- Cover success, failure, malformed input, missing values, duplicates, zero
  values, nil values, and boundary conditions.
- Test concurrent use wherever the implementation supports concurrent execution.
- Add integration tests only for behavior requiring real containers, networks,
  services, or Docker.
- Do not weaken production validation to make an existing test pass.
- When behavior changes, update or remove tests that assert obsolete behavior.

## Comments and Documentation

- Use concise comments that describe current behavior, purpose, or constraints.
- Exported identifiers must have professional Go documentation comments.
- Do not describe implementation history, migrations, temporary fixes, agent
  work, host-specific observations, or removed designs.
- Avoid comments that merely restate the code.
- Keep package documentation in `doc.go`.

## Dependencies and Implementation

- Prefer the Go standard library and existing framework packages.
- Do not add a third-party dependency when an existing package provides the
  required capability.
- Do not import indirect dependencies directly without making them explicit in
  `go.mod`.
- Use `core/util/httpx` for framework HTTP operations.
- Use `core/util/retry` for polling and eventual-consistency waits.
- Use `gopkg.in/yaml.v3` for YAML parsing.
- Use `encoding/json` for JSON parsing unless a stronger requirement exists.
- Do not implement ad hoc polling loops when the retry package can express the
  behavior.

## Validation

For framework changes, run the narrowest relevant checks first, then broaden
validation:

```bash
go test ./path/to/changed/package -count=1
go test -race ./path/to/changed/package -count=1
go vet ./path/to/changed/package
```

For a migrated feature, run the framework standards checker against the feature
and its step definitions:

```bash
go run ./cmd/standards \
  -root . \
  -unit-root core \
  -steps suites/it/steps \
  -features suites/it/features/<feature>.feature \
  -scripts tools \
  -suites suites/it/it-suite.yaml,suites/ui/ui-suite.yaml \
  -docs core
```

This checker is the Go-framework equivalent of a PMD/Checkstyle gate. It
enforces the repository's feature, step, dependency, architecture, cleanup,
suite-boundary, documentation, and unit-test-layout rules. A migration is not
complete while it reports an issue.

For Docker-backed tests, use the configured Docker/Testcontainers environment
and verify cleanup after the run.

Before completing work:

- Run `gofmt -d` and format changed Go files with `gofmt -w` when needed.
- Run `go test ./path/to/changed/package -count=1`.
- Run `go test -race ./path/to/changed/package -count=1`.
- Run `go vet ./path/to/changed/package`.
- Run the focused SQLite feature runner.
- Run the complete migrated feature file.
- Run the relevant database matrix after the focused run passes.
- Run `git diff --check`.
- Review the full diff.
- Search for stale names, comments, registrations, and references.
- Report tests that could not be run and the reason.

## Concurrency and Isolation Principle

Tests run concurrently against shared component instances: multiple blocks may
run at the same time and multiple runners may execute scenarios within each
block.

The isolation principle is:

> Every test owns its resources and shares nothing mutable.

- **Unique naming is mandatory.** Different literal names chosen by different
  runners do not guarantee isolation. Every test-owned resource must use the
  framework's unique-name mechanism.
- In feature values and payloads, use `${UNIQUE:base}` or a generated context
  value. Never hand-name APIs, users, tenants, databases, networks, files, or
  other test-owned resources.
- Reuse the generated value throughout the scenario instead of generating a
  second name for the same resource.
- A runner is the isolation boundary. A runner must not depend on another
  runner's data, execution order, or side effects.
- Scenarios within one runner execute sequentially in the declared feature-file
  order. Feature files may be reused as setup features when their resources use
  unique names and are intentionally shared by later features in that runner.
- Register ordinary scenario resources for scenario cleanup. Register setup
  resources intended to survive across scenarios for runner cleanup.
- **Every successfully created test-owned resource must be registered for
  cleanup immediately.** If registration fails, the creating operation must
  fail and must make a best effort to remove the resource.
- Never rely on a later step, deferred cleanup, process exit, or manual cleanup
  to remove an unregistered resource.
- If a scenario needs a pre-existing resource, create it explicitly through a
  setup feature or setup step and give it the appropriate runner ownership.
- Do not use mutable package-level state or shared mutable fields for scenario
  or runner data.
- Keep temporary files, coverage output, ports, and cleanup registrations
  isolated by block, runner, or scenario as appropriate.
- Poll for readiness, propagation, and eventual consistency with the framework
  retry utilities. Do not use `time.Sleep` or fixed thread sleeps.
- Ensure cleanup is safe when scenarios run concurrently and when a resource
  has already been deleted by the scenario that created it.

### Increasing suite concurrency safely

The integration suite uses nested `t.Parallel` subtests: blocks run in parallel
and runners inside each block also run in parallel. Go's global `-parallel`
budget applies to both levels. If it is lower than the number of parent and
nested subtests that must make progress, the run can deadlock after component
boot instead of reporting a test failure.

- Do not force `go test -parallel` below the framework-computed budget. The suite
  test main raises the budget when the selected topology requires it; preserve
  that behavior when adding or increasing `parallel` values in suite YAML.
- Increase one block's runner concurrency at a time and run one focused
  iteration first. Inspect failures and elapsed time before running repeated
  iterations or increasing it again.
- A green run is not evidence of isolation by itself. Verify every stateful
  testbench service used by the block—such as event stores, counters, and
  response records—is addressed by the correct runner- or scenario-scoped
  partition. Runner-local Go context does not isolate mutable state inside a
  shared product or testbench process.
- Partition state at the same granularity as the assertion. If a scenario
  expects a fresh counter, event stream, webhook history, OAuth token sequence,
  or response record, use a scenario-scoped partition; a runner-scoped context
  alone is insufficient when multiple scenarios share the service. Pass the
  partition explicitly through every fixture and lookup involved in the
  assertion.
- Do not assert global absence or an unqualified list while another runner can
  create the same resource class. Scope the query to the owning organization,
  project, or generated resource identifier, and select the exact resource by
  its stored ID where the API or UI exposes one. If the behavior specifically
  requires an empty global state, place that scenario in a dedicated topology
  whose components and runner cannot create that state.
- Resource readiness is separate from resource creation. Before opening a
  dependent UI page or asserting a dependent control, poll the product's
  supported list or status endpoint until the exact generated ID is observable.
  Use a bounded retry deadline; never replace propagation polling with a fixed
  delay or assume that a successful create response means all read paths are
  ready.
- Each UI runner gets its own browser context and page state. Do not share a
  mutable page, cookies, local storage, or route state between runners. For
  state-changing actions that trigger SPA navigation, synchronize the action
  with the expected navigation and wait for the required document or control,
  rather than relying on a generic load-state wait. Use exact selectors tied to
  the scenario's generated resource when multiple cards or rows may exist.
- When a browser or service operation is eventually consistent, poll the
  observable product condition with `core/util/retry`, bounded by the test
  context. Do not interpret `net::ERR_ABORTED`, a missing control, or a transient
  404 as an isolation failure until the relevant navigation, readiness, and
  partition assumptions have been checked.
- If a run hangs, stop the test process and its run-owned containers, then
  inspect the final goroutine stack and service logs. Do not classify a timeout
  as a passing stress result.

### Measuring and changing concurrency

- Treat concurrency as a measured resource decision, not only a throughput
  setting. After changing a block's concurrency, run one focused iteration and
  inspect failures before repeating it or increasing the value. Repeat a green
  configuration when isolation or timing behavior is important.
- For a sequential timing pass, record each runner's elapsed time after it has
  acquired its runner slot. A simple first estimate is
  `ceil(sum(runner durations) / longest runner duration)`. Include boot,
  teardown, browser, and service resource limits when choosing the final value;
  the equation is an upper-bound efficiency estimate, not permission to exceed
  host capacity.
- If a sequential run fails, fix or explain that failure before using its
  timings to justify concurrency. A failure in a sequential run is evidence
  against attributing the problem solely to load; a failure only at higher
  concurrency requires comparison of resource state, partitions, browser
  diagnostics, and product logs before declaring a framework limitation.

## Capability Map

`docs/capability-map.yml` is the source of truth for the integration suite's
capability and feature vocabulary. Capability and feature identifiers used by
scenario annotations must be defined in that file.

Before adding or modifying a product scenario:

1. Identify the capability and feature being tested in the map.
2. Review the existing feature files and coverage documentation.
3. Extend an existing capability or feature when it already represents the
   behavior. Add a new map entry only when the behavior is genuinely new.

Unknown capability or feature identifiers must fail validation. Do not invent
similar identifiers or use free-form capability names.

## Scenario Annotations

Every product scenario must identify what it tests and how it is classified:

| Annotation | Cardinality | Meaning |
|---|---:|---|
| `@cap:<id>` | 1 | The capability under test and the subject of the assertions. |
| `@feat:<id>` | 1 | The feature within that capability. |
| `@rule:<slug>` | 0..1 | An optional subgroup within the feature. |
| `@type:smoke`, `@type:negative`, `@type:regression` | 0..N | The nature or selection category of the scenario. |
| `@dep:<cap>` | 0..N | A cross-capability prerequisite, not coverage of that capability. |
| `@legacy:<id>` | 0..N | An optional legacy test identifier used for parity tracking. |

Use exactly one `@cap` and one `@feat` for each product scenario. If a scenario
tests multiple capabilities, split it unless one capability is clearly the
subject and the others are only prerequisites. Do not use `@dep` for universal
baseline requirements.

Non-product or framework-support features must use one exclusion annotation
instead of product capability annotations:

```text
@setup    @infra    @framework    @migration
```

Annotations must describe the scenario's purpose, not merely the services or
fixtures it happens to use.

## Reuse Principle

Reuse cascades through every framework layer. Before creating a feature, step,
runner, block, service, helper, or dependency, search for an existing solution.
Add new behavior only when nothing fits, and extend a near-fit instead of
duplicating it.

### Feature level

- Search `docs/capability-map.yml` and existing `.feature` files for the
  capability and feature under test.
- Add scenarios to the existing feature file when it already owns the behavior.
- Extend a `Scenario Outline` with another example when only the inputs differ.
- Create a new feature file only when no existing feature represents the
  capability.

### Step level

- Search existing step definitions before adding a Gherkin step.
- Extend a near-fit with a parameter or small input-specific branch.
- Move shared behavior into a private helper and keep Gherkin steps thin.
- Add a new step only when no existing step expresses the operation, and route
  its HTTP request through the appropriate funnel.

### Runner and block levels

- Add a feature to an existing runner when its setup, dependencies, and
  assertions fit that runner.
- Reuse setup features within a runner; each runner receives its own isolated
  execution context.
- Create a new runner only when the feature requires a different grouping or
  lifecycle.
- Add a runner to an existing block when its component and overlay requirements
  already fit.
- Create a new block or overlay only when an existing configuration cannot
  express the required behavior. A new block provisions another component
  topology and must be justified.
- A runner may declare a Platform Gateway release boundary in `tags` with
  `gateway-version<operator>major.minor.patch`, using one of `>`, `>=`, `<`,
  `<=`, `=`, or `==`. An optional Godog expression follows exactly one `;`,
  for example `gateway-version>1.2.0;~@known-issue`. The framework validates
  and evaluates the release selector before Godog, removes it from the Godog
  expression, and reports incompatible runners as skipped.
- Use a Gateway-version selector only for a real release capability boundary.
  It belongs at runner scope, not in feature tags or scenario steps. Keep
  release-specific configuration in a dedicated block rather than changing a
  shared block overlay. Malformed selectors, a missing Godog expression after
  `;`, or more than one `;` are configuration errors and must not be worked
  around with looser tags.
- Use `defaults.components.platform-gateway.dbCompatibility` for Gateway database
  support boundaries shared by matrix blocks, for example
  `sqlserver: "gateway-version>=1.2.0"`. Its values use the same strict selector
  syntax as runner tags. The framework excludes incompatible database variants
  before boot and reports the skip; an explicitly selected incompatible variant is
  a configuration error. Do not duplicate a shared database compatibility rule in
  every block or encode it in scenario tags.

### Infrastructure level

- Reuse an existing testbench service when it can produce the required response
  or failure condition.
- Do not create custom Docker containers to mock backends for a suite scenario.
- When the testbench cannot produce the required behavior, add the mock backend
  as a testbench service and wire its lifecycle, endpoint, image, and readiness
  behavior completely.

## Funnel Principle

Every assertion-visible HTTP request made by a step must pass through one
framework funnel for its plane:

- Management-plane operations use the management request funnel.
- Gateway data-plane invocations use the data-plane request funnel.
- Component-service requests use the service request funnel.

The funnel is responsible for resolving the request, applying scenario state,
executing it, clearing stale response state before the call, and publishing the
new response for subsequent assertions.

Polling requests that are only intermediate observations may use the underlying
client, but they must not replace the response published for the step being
asserted. Any exception to the funnel rule must be explicit, narrowly scoped,
and covered by a test.

Do not call an HTTP client directly from a step when the request should be
visible to generic response assertions. Centralizing request execution prevents
stale responses from allowing an assertion to pass against an earlier call.

## Resource Cleanup

Every resource created by a test must be removed, including when the scenario
fails. Cleanup belongs in lifecycle hooks, never in inline teardown steps that
can be skipped after a failure.

- Register every successfully created resource immediately with the cleanup
  registry.
- If registration fails, fail the creating operation and make a best-effort
  deletion of the resource.
- Choose ownership by lifetime: scenario-owned resources are cleaned after the
  scenario; runner-owned setup resources are cleaned after the runner's final
  scenario.
- Setup resources must remain available to later features in the same runner
  and must not be removed by scenario cleanup.
- Explicitly deleted resources must be deregistered so cleanup does not attempt
  to delete them again.
- Cleanup must remove all test-owned resources, including APIs, applications,
  policies, scopes, tenants, keys, databases, containers, networks, files, and
  other temporary artifacts.
- Cleanup failures must be logged with the resource, owner, block, runner,
  scenario, and attempt information. A cleanup failure is a leak signal and
  must not be ignored.
- Cleanup must be idempotent and continue processing other resources when one
  deletion fails.
