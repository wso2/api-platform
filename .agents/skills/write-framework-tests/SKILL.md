---
name: write-framework-tests
description: Write a new WSO2 API Platform test in the integration framework (tests/framework), from its scenario-list entry to a passing scenario. Use when the user asks to add, write or automate an integration (IT), UI (browser) or cloud test, automate a scenario ID such as TENANT-004, add a feature file or step, or decide which suite, feature file or block a test belongs in.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# Write a framework test

Takes one behaviour to test from its scenario-list entry to a scenario that passes and cleans
up after itself. `tests/framework/CLAUDE.md` holds the rules; each step below names the section
to follow. Run commands from `tests/framework`.

## Step 1: Start from the scenario list

Every test starts from an entry in its feature's scenario list ("Scenario Lists" in
`CLAUDE.md`):

- On-premises tests (`suites/it`, `suites/ui`): `scenarios/on-prem/<CODE>.yaml`.
- Cloud tests (`suites/cloud`): `scenarios/cloud/<CODE>.yaml`.

Find the entry that describes the behaviour, with its ID, title (action and expected result)
and priority. The lists in the repository are where IDs live; you do not need any other
document. If there is no entry, add one in the same change:

- Take the next free number for the list's ID prefix (`APIDEF-014` → `APIDEF-015`; `AIW-LLM`
  uses `AIW-PRV` and `AIW-PRX`, one prefix per sub-feature).
- Write the title as the action and its expected result, and set a priority (P0–P2).
- If the feature has no list, start `<CODE>.yaml` with a short code and the header the other
  lists carry.
- Never reuse or renumber a merged ID. The pull request review approves the new entry.

## Step 2: Pick the suite

"Suites: Where a Test Goes" in `CLAUDE.md`:

| The behaviour | Suite | List layer |
|---|---|---|
| An on-premises product at the API level (REST, `ap` CLI, gateway traffic) | `suites/it` | `it` |
| An on-premises portal page or journey in the browser | `suites/ui` | `ui` |
| Something only the hosted cloud has (console, managed gateways, environments, promotion) | `suites/cloud` | `cloud` |

An on-premises entry records both layers. When a layer cannot perform the action (no console
control for it, say), mark that layer `exempt` with the reason instead of testing around it.

## Step 3: Pick the feature file

Search before creating ("Reuse Principle", "Feature level"):

```bash
grep -rln "@<CODE>" suites/*/features
grep -rn "<a phrase from the behaviour>" suites/<suite>/features
```

- Add the scenario to the feature file that already owns the behaviour; extend a
  `Scenario Outline` when only the inputs differ.
- Otherwise create a file in the suite's layout ("Feature files" in `CLAUDE.md`): IT under
  `features/<product>/<area>/`, UI under `features/<portal>/`, cloud as
  `features/apip_<area>.feature`.
- IT covers one ID per scenario (or one outline). UI covers a journey: one browser scenario that
  walks a user task through several IDs.

## Step 4: Write and tag the scenario

- Tag the scenario with every ID it covers (`@TENANT-004`) and the feature with the list's
  code (`@TENANT`). Add `@type:negative` and similar where they apply ("Scenario
  Annotations").
- Assert the exact expected result from the entry; never widen it (`401 || 403`).
- Pair every negative assertion (404, missing entry, hidden control) with a positive control
  that succeeds for the owner.
- If the product does not meet the expected result yet, keep the expected result, tag the
  scenario `@known-issue`, record the issue in the entry, and make sure the runner that hosts
  it excludes `~@known-issue`.
- Never embed YAML documents in a feature; use the canonical resource template and pass the
  values through the table ("Resource Templates").

## Step 5: Reuse or add steps

Search the existing steps before adding one ("Step Definitions", "Step level"):

```bash
grep -rn "sc.Step(\`\^.*<a word from your step>" suites/<suite>/steps
```

- Extend a near-fit step with a parameter rather than adding a variation.
- A new step goes in the product's package ("Step packages" in `CLAUDE.md`), never in
  `common` if it is product-specific.
- Send every request the scenario asserts on through the funnel for its plane ("Funnel
  Principle"), so a stale response cannot satisfy the assertion.
- Mock a backend with a testbench service, never a custom container.

## Step 6: Clean up everything the test creates

"Resource Cleanup" in `CLAUDE.md`. Register each resource with the cleanup registry the moment
it is created, as the actor that created it, and deregister what the scenario deletes
itself.

- A resource created by clicking in a browser is invisible to the framework. Register it in
  the same step that clicks, as soon as the console confirms the creation (for example from
  the ID in the console's network response), so a later failure in that step or the next one
  cannot skip the registration. If registration fails, delete the resource on a best-effort
  basis and fail the step.
- A cleanup failure in the log (`WARN resource cleanup failed`) is a leak, not noise.
- Cloud scenarios run against a shared organisation: create only what the scenario owns,
  never change a shared resource (the organisation's gateway, for example) without restoring
  it, and never delete what the run did not create.

## Step 7: Put the feature in a runner

Reuse before adding, in this order ("Runner and block levels" in `CLAUDE.md`):

1. Add the feature to an existing runner whose setup, dependencies and tags fit it.
2. Otherwise add a runner to an existing block that fits. A new runner is only for a feature
   that needs a different grouping or lifecycle.
3. A new block, or a new block file, is the last resort ("Integration Suite Blocks").

Where to look:

- IT: the block file under `suites/it/blocks/` for the products the scenarios check; most
  features fit the file's generic block.
- UI: `suites/ui/ui-suite.yaml`, a block that starts the portal the journey uses.
- Cloud: the `apip-cloud` block in `suites/cloud/cloud-suite.yaml`.
- A runner that hosts `@known-issue` scenarios excludes them: `tags: "~@known-issue"`. A
  Gateway release boundary goes in the runner's tags (`gateway-version>=1.2.0;~@known-issue`),
  never in feature tags.

## Step 8: Run it and prove it

```bash
# IT: the block (or block variant) that hosts the runner, narrowed to the feature's tag
go test ./suites/it -count=1 -v -timeout=60m -blocks <block-or-variant> -feature-tags "@<tag>"
# UI
go test ./suites/ui -count=1 -v -timeout=25m -blocks <block> -feature-tags "@<tag>"
# Cloud: compile and unit-test locally; a real run needs a cloud environment and token
GOWORK=off CGO_ENABLED=0 go test -c -o /dev/null ./suites/cloud && go test -count=1 ./suites/cloud/steps/...
```

- A block without a database matrix is selected by its bare name (`api-portal-core`). A block
  with a matrix runs as one variant per engine (`gateway-core/sqlite`), and a matrix on two
  components names each variant after both (`platform-api-core/api-sqlite_gateway-sqlite`).
  An unknown name fails with the list of valid ones.
- Check the output says the scenario ran (`N scenarios (N passed)`), not that it was skipped.
- A scenario is done when it passes three local runs in a row and fails when you temporarily
  break the condition it asserts.
- Run the relevant database variants, and the standards check ("Validation" in `CLAUDE.md`).
- Check that no cleanup failure was logged.

## Step 9: Update the scenario list

Set the entry's layer status (`automated`, `known-issue`, `exempt` or `planned`) and its `refs`
(feature path relative to `tests/framework` and the exact scenario name) in the same change as
the scenario.

## Before opening the pull request

- The entry, tags, runner and refs all name the same IDs.
- New steps have unit tests where they hold logic, and `gofmt`, `go vet` and the standards
  check pass.
- The pull request says which IDs it automates, which are `@known-issue` (with their issues),
  and which layers are exempt and why.
