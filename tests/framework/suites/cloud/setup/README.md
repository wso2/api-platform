# Cloud suite setup: the starter and the finalizer

The cloud suite runs against a shared, long-lived organisation. Two run-level components keep a
run honest about it:

- the **starter** (`starter/`, with the checks in `../steps/cloudconsole/starter.go` and
  `preconditions.go`) checks that the organisation holds what a scenario needs, so a scenario that
  cannot run is **skipped** instead of failing for a reason that is not the product's;
- the **finalizer** (`finalizer/`, with the sweeps in `../steps/cloudconsole/leftovers.go`) removes
  the data a run left behind.

`cmd/cloudsetup` runs both by hand, outside a suite run.

## What the starter checks

The starter does not read a list of runners. A scenario's steps say what it needs: each step that
looks something up in the organisation (`I find the default APIP project`, `I find an active APIP
gateway`, `I find an active managed API Portal`, and so on) maps to a **precondition** in
`stepNeeds`. Before a scenario runs, the starter makes sure every precondition its steps name is
met. Each precondition is prepared **once** per run and the answer is shared by every scenario and
runner after it, so:

- a precondition no scenario of the run needs is never looked at: a run of `apip-basic` alone does
  not touch the organisation's portals, environments or pipelines, and a runner that is commented
  out or filtered by a tag (`~@known-issue`) asks for nothing;
- the checks happen just before the first scenario that needs them, not for the whole suite up
  front.

The steps keep their own checks, which stay as the guard when the organisation changes during the
run.

| Precondition | Met when | Missing | Scenarios that need it |
|---|---|---|---|
| `default-project` | `GET /projects/default` answers 200 | not built: skipped | the scenarios that start with `I find the default APIP project` (basic, deploy, secrets, portal and some gateway scenarios) |
| `default-gateway` | the default project's deployment pipeline is readable and lists an environment, and the organisation holds an active regular gateway in it that no suite run created | **built** in the pipeline's first stage (`development` when the pipeline reaches it) and waited for until it is `ACTIVE` | the scenarios that start with `I find an active APIP gateway` (basic, deploy, gateway, gateway configuration, secrets) |
| `environment` | the platform lists an environment with a name | not built: skipped | the scenarios that select an existing environment for a managed or self-hosted gateway |
| `second-environment` | the platform lists a non-production environment besides the default gateway's, that no suite run created | not built: skipped | `ID_APIP_GATEWAY_006` and `_007` |
| `pipeline-project` | `CLOUD_IT_PIPELINE_PROJECT` names a project of the organisation that is bound to a deployment pipeline the platform lists, with at least one promotion path | not built: skipped | the scenarios that start with `I find the pipeline-bound APIP project` |
| `pipeline-chain-2`, `pipeline-chain-3` | that pipeline holds a promotion chain of 2 (3) environments, each with an active regular gateway | not built: skipped | the promotion scenarios (the chain of 3 is `ID_APIP_PROMOTION_006`), `ID_APIP_DEPLOY_011`, `ID_APIP_PROJECT_009` |
| `api-portal` | the organisation holds an active managed API Portal that no scenario created | **built** and waited for until it is `active` | the scenarios that start with `I find an active managed API Portal` (the portal feature) |

The starter never creates an environment, a deployment pipeline or a project, and never changes a
project's pipeline: the WSO2 Cloud console manages them. A scenario never creates them either.

## What happens when a precondition is not met

| Situation | The scenarios that need it | The run |
|---|---|---|
| Met, or built successfully | run | passes |
| Absent, and the starter cannot build it (the table above) | **skipped** | does not fail for it |
| Absent, and the platform refuses the starter's token the request that would build it (`401`/`403`) | **skipped** | does not fail for it |
| The starter tried to build it, or to read it, and could not (the build failed, timed out or the platform answered with an error) | **skipped** | **fails**, and the log names the precondition and the reason |

A gateway or portal the starter built and could not make active is **removed at once**. What the
starter did not build is never removed. Scenarios that need other preconditions are unaffected: a
gateway failure does not skip a portal scenario.

Every skip is logged with the scenario, the precondition and the reason, and the end of the run
logs one line per precondition with the number of scenarios it skipped (`cloud run starter: ...`).
Read those lines before trusting a green run: a skip is not a pass.

### Configuration

| Setting | Meaning |
|---|---|
| `CLOUD_CONSOLE_ACCESS_TOKEN` | the console token that decides the organisation (required, never printed) |
| `CLOUD_IT_PIPELINE_PROJECT` | the id of the project the promotion scenarios deploy through; unset skips them |
| `CLOUD_GATEWAY_ID` | pins the gateway the scenarios deploy to; the starter never builds a pinned one |

## What the finalizer removes

Every name the suite generates carries a suffix of its runner. After the run, the finalizer lists
the organisation's collections and deletes each entry named with a suffix of **this run**, in
dependency order: portal publications, REST APIs (in the default project, in the pipeline-bound
project and in each project the run created), secrets, self-hosted gateways, managed gateways,
projects. Whatever it removes is a leak the per-scenario cleanup missed and is logged as one.

The finalizer **never** deletes:

- anything the organisation already had;
- the gateway and the portal the starter built. They stay so that the next run finds them and uses
  them like any resource the organisation already has. The starter's suffix is not one the run's
  finalizer looks for;
- a managed API Portal of any kind, except one a scenario built for itself (`syn-portal-`). It
  empties the portals of what the run published, and nothing else.

A person who wants the starter's gateway gone names its suffix to the setup command (below).

## Running them by hand

```bash
# check every precondition (or the ones named), build a gateway and a portal when missing
go run ./suites/cloud/setup/cmd/cloudsetup start -endpoint "$CLOUD_PLATFORM_API_ENDPOINT" \
  [-precondition default-gateway]...

# delete what carries a suffix: a killed run's leftovers, or the gateway `start` built
go run ./suites/cloud/setup/cmd/cloudsetup finalize -endpoint "$CLOUD_PLATFORM_API_ENDPOINT" \
  [-suffix SUFFIX]...
```

`start` prints what it built or found and one line for each precondition that is not met, and
exits non-zero only for what it failed to prepare. It records the suffix of the gateway it builds in
a state file under the user cache directory so that `finalize` can remove it; a suite run never
does.

## Adding a precondition

1. Add the constant and its place in `allPreconditions` in `preconditions.go`.
2. Map the step text that needs it in `stepNeeds`, and add its check to `prepare`. A precondition
   the starter builds gets a `removeCreated` call for a build it could not finish; one it only
   checks returns an error that wraps `errUnavailable` when the resource is absent.
3. Add the cases to `lifecycle_test.go`. `TestEveryPreconditionIsAskedForByAScenarioOfTheFeatureFiles`
   fails when no scenario of the feature files asks for a precondition, which is how a reworded step
   shows up.
