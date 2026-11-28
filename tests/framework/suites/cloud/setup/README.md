# Cloud suite setup: the starter and the finalizer

The cloud suite runs against a shared, long-lived organisation. Two run-level components keep a
run honest about it:

- the **starter** (`starter/`, with the checks in `../steps/cloudconsole/starter.go` and
  `preconditions.go`) checks that the organisation holds what a scenario needs, so a scenario that
  cannot run is **skipped**, and the run is failed when the organisation lacks something the
  suite needs, so a green run never hides a missing environment, pipeline or portal;
- the **finalizer** (`finalizer/`, with the sweeps in `../steps/cloudconsole/leftovers.go`) removes
  the data a run left behind.

`cmd/cloudsetup` runs both by hand, outside a suite run.

## What the starter checks

The starter does not read a list of runners. A scenario's steps say what it needs: each step that
looks something up in the organisation (`I find the APIP test project`, `I find an active APIP
gateway`, `I find an active managed API Portal`, and so on) maps to a **precondition** in
`stepNeeds`. Before a scenario runs, the starter makes sure every precondition its steps name is
met. Each precondition is prepared **once** per run and the answer is shared by every scenario and
runner after it, so:

- a precondition no scenario of the run needs is never looked at: a run of `apip-basic` alone
  checks the default gateway (which reads the pipeline binding of `apip-test-project-1` and lists
  the gateways and pipelines) but not the organisation's portals or other environments, and a
  runner that is commented out or filtered by a tag (`~@known-issue`) asks for nothing;
- the checks happen just before the first scenario that needs them, not for the whole suite up
  front, with one exception: the two projects below are the first thing the starter makes sure of
  with the token.

### The projects the runners work in

The suite never uses the organisation's default project or default pipeline. Each runner works in
one of two projects the organisation already holds, each bound to a deployment pipeline of its own
named with the prefix `apip-test-`:

| Project | Runners |
|---|---|
| `apip-test-project-1` | `apip-basic` (with the GraphQL and subscription plan features), `apip-deploy` (with the managed gateway runtime and route features), `apip-gateway`, `apip-gateway-config`, `apip-secrets`, `apip-self-hosted-gateway-runtime` |
| `apip-test-project-2` | every other runner: `apip-auth`, `apip-self-hosted-gateway`, `apip-promotion`, `apip-portal` (and `apip-project`) |

A scenario works in the project of its feature file (one feature file is one runner). The default
gateway of a project is in a first stage of that project's pipeline, an environment no promotion
path targets, because the platform accepts a deployment to a later stage only by promotion. The
scenarios that deploy along a pipeline (`ID_APIP_DEPLOY_011`; the promotion scenarios) use the
pipeline of the project of their runner, so `apip-test-project-1` needs a promotion chain of 2
environments and `apip-test-project-2` one of 3.

Before any runner starts, the starter makes sure the organisation holds both projects. A project
it lacks is **created** by the starter (`POST /projects` with the handle `apip-test-project-N`) and
stays for later runs. Only the project is created: its deployment pipeline is managed in the WSO2
Cloud console, so a project the starter created is bound to the default pipeline, which the suite
does not use, and the preconditions that need its pipeline (the default gateway for project 1, the
promotion chains) stay unmet, and their scenarios are skipped, until the console binds it to a
pipeline named with the prefix `apip-test-`.

When the starter cannot create a project, every scenario that works in it is **skipped** before
anything else of the organisation is looked at, and the run **fails** with the reason. A platform
that refuses the starter's token the request (`401`/`403`) is the organisation's limit: skipped,
no failure. A scenario that uses no project, such as a sign-in one (`apip-auth`) or a self-hosted
gateway one, asks for none and is never skipped for a missing project.

The steps keep their own checks, which stay as the guard when the organisation changes during the
run.

| Precondition | Met when | Missing | Scenarios that need it |
|---|---|---|---|
| `project@<project>` | `GET /projects/<project>` answers 200, for `apip-test-project-1` and `apip-test-project-2` | **built** by the starter; skipped and the run fails when it cannot be built | the scenarios that read, find or create resources in the project of their runner (basic, deploy, gateway, secrets, portal, project, promotion), as their first check |
| `default-gateway@<project>` | the deployment pipeline of the project is readable, is named with the prefix `apip-test-` and lists an environment, and the organisation holds an active regular gateway in a first stage of it that no suite run created | **built** in the pipeline's first stage (`development` when it is one) and waited for until it is `ACTIVE` | the scenarios that start with `I find an active APIP gateway`, for the project of their runner (basic, deploy, gateway, gateway configuration, secrets, portal) |
| `environment` | the platform lists an environment the suite may select (see "Which resources the suite selects") | not built: skipped, and the run **fails** | the scenarios that select an existing environment for a managed or self-hosted gateway |
| `second-environment` | the platform lists a non-production environment the suite may select, besides the default gateway's, that no suite run created | not built: skipped, and the run **fails** | `ID_APIP_GATEWAY_006` and `_007` |
| `pipeline-project@<project>` | the project is bound to a deployment pipeline named with the prefix `apip-test-` that the platform lists, with at least one promotion path | not built: skipped, and the run **fails** | the scenarios that start with `I find the pipeline-bound APIP project` |
| `pipeline-chain-2@<project>`, `pipeline-chain-3@<project>` | that pipeline holds a promotion chain of 2 (3) environments the suite may select, each with an active regular gateway | not built: skipped, and the run **fails** | the promotion scenarios of `apip-test-project-2` (the chain of 3 is `ID_APIP_PROMOTION_006`), `ID_APIP_DEPLOY_011` of `apip-test-project-1`, `ID_APIP_PROJECT_009` |
| `api-portal` | the organisation holds an active managed API Portal that no scenario created | not built while `starterCreatesPortal` is `false` (the cloud intermittently fails to build one): skipped and the run **fails**, so provide an active portal for these scenarios. Built and waited for until it is `active` once the constant is `true` | the scenarios that start with `I find an active managed API Portal` (the portal feature) |

The starter never creates an environment or a deployment pipeline, and never changes a project's
pipeline: the WSO2 Cloud console manages them. A scenario never creates them either. The starter
creates a missing suite project (see "The projects the runners work in").

### Which resources the suite selects

The organisation is shared with other consoles (agent manager and others), so environments,
deployment pipelines and projects belong to everyone. The suite selects an environment only if it
is the organisation's default one or its name **starts with `apip-test-`**, and a deployment
pipeline or a project only if its name starts with `apip-test-`:

| Resource | The suite selects | Examples |
|---|---|---|
| Environment | `development`, or a name starting with `apip-test-` | `apip-test-e1` |
| Deployment pipeline | a name starting with `apip-test-`; never `default` | `apip-test-pipeline-1`, `apip-test-pipeline-2` |
| Project | `apip-test-project-1` and `apip-test-project-2` only; never `default` | |

An environment that hosts the gateways the scenarios build must also leave room for their names:
the platform limits a gateway handle to 37 minus twice the environment name's length, and the suite's
names need 13 characters, so such an environment name is at most 12 characters (`apip-test-e1`).
The starter asks the platform for the limit (the request is refused before anything is built) and
passes over a longer name: `apip-test-environment` allows a 1-character handle and would fail the
scenarios that build a gateway in it. Environments that only hold an existing gateway (promotion
chains) are not subject to this.

Any other environment, pipeline or project is passed over, even when it has everything a scenario
needs (a gateway in the environment, a promotion path in the pipeline): the precondition is not met
and the scenarios that need it are skipped and the run fails. A promotion chain only runs through
environments the suite may select. Gateways and portals are not selected by name: the starter finds or builds them
under any name.

## What happens when a precondition is not met

| Situation | The scenarios that need it | The run |
|---|---|---|
| Met, or built successfully | run | passes |
| Absent, and the starter cannot build it: an environment, a deployment pipeline or a promotion chain (the table above) | **skipped** | **fails**, and the log names the precondition and the reason |
| The starter tried to build it, or to read it, and could not (the build failed, timed out or the platform answered with an error) | **skipped** | **fails**, and the log names the precondition and the reason |
| The platform refuses the starter's token the request that would build it (`401`/`403`) | **skipped** | does not fail for it |
| It depends on a precondition that failed (a chain needs its pipeline, a second environment needs the default gateway) | **skipped** | does not fail for it; the precondition it depends on is the failure |

A gateway or portal the starter built and could not make active is **removed at once**: the starter
attempts the removal, and when the platform refuses it the log says the resource "could not be
removed" and the run fails with that error. Check the platform for the resource (it may still be
there, possibly failed) and delete it by hand or with `cloudsetup finalize -suffix <suffix>`. What
the starter did not build is never removed. Scenarios that need other preconditions are
unaffected: a gateway failure does not skip a portal scenario.

Every skip is logged with the scenario, the precondition and the reason, and the end of the run
logs one line per precondition with the number of scenarios it skipped (`cloud run starter: ...`).
Read those lines before trusting a green run: a skip is not a pass.

### Configuration

| Setting | Meaning |
|---|---|
| `CLOUD_CONSOLE_ACCESS_TOKEN` | the console token that decides the organisation (required unless the file below is set, never printed) |
| `CLOUD_CONSOLE_ACCESS_TOKEN_FILE` | a file holding the console token, read on every request and so free to be replaced during a run; wins over `CLOUD_CONSOLE_ACCESS_TOKEN` |
| `CLOUD_GATEWAY_ID` | pins the gateway the scenarios deploy to, which must be in a first stage of the project's pipeline; the starter never builds a pinned one |
| `CLOUD_IAM_ACCESS_TOKEN` | a token of the organisation's administrator for the Identity Platform API, for the portal administrator (optional, never printed) |
| `CLOUD_IAM_ACCESS_TOKEN_FILE` | a file holding that token, replaced during a run like the console token's; wins over `CLOUD_IAM_ACCESS_TOKEN` |
| `CLOUD_IAM_API_URL` | the Identity Platform API's base URL, `https://api-<env>.gateway.<domain>/iam/v1` |

### The portal administrator and the portal sweep

The platform never deletes a portal's copy of a subscription plan, so every run's synthetic plans
stay on the shared portal until something with the portal's administrator role removes them. Once
the `api-portal` precondition is met, the starter makes that administrator, `apip-test-portal-admin`,
and signs it in:

1. it deletes the previous `apip-test-portal-admin` on the Identity Platform's control plane and
   deploys its removal, together with the `dp_admin` role that still names it, to the environment
   the portal's consumers sign in on (the portal's `loginEnvironment`);
2. it creates the user again (OU `default`, user type `apip-portal-subscriber`) with a new random
   password, gives it `dp_admin`, and deploys the two.

A deploy carries only those changes, never anything else pending in the organisation, and its
import results are checked, since the Identity Platform answers 200 even when an item failed. The
user is made again every run because a password changed on its own is never deployed. The password
stays in memory and is never logged.

The administrator then sweeps the portal, at the start of the run and again after the finalizer.
The sweep goes by the synthetic prefix, not the run's suffix, so it removes what **every** run left:

| Portal collection | What the sweep removes |
|---|---|
| API listings | the listings of synthetic APIs (`syn-`) the platform no longer holds |
| subscription plans | every synthetic plan (`syn-plan-`) of this run, and another run's once it is older than 30 minutes |

The age rule keeps a plan another run may still publish with, since the portal protects only a
plan an API offers or a subscription uses. The portal refuses to remove such a plan, which is kept
and reported. Without `CLOUD_IAM_ACCESS_TOKEN`, or when the administrator cannot be made, the scenarios
still run and the run does not fail: the leftovers stay, and the starter's report says why.

## What the finalizer removes

After the run, the finalizer deletes what the run created, in dependency order. Every name the
suite generates carries a suffix of its runner, and the finalizer deletes only the entries named
with a suffix of **this run**:

| Collection | What is removed |
|---|---|
| portal publications | the publications of the run's APIs, from every active portal |
| managed API Portals | only a portal a scenario built for itself (`syn-portal-`) |
| REST APIs and GraphQL APIs | the run's APIs, in `apip-test-project-1`, `apip-test-project-2` and the projects the run created |
| subscription plans | the plans the run created (`syn-plan-`) |
| secrets, service accounts, self-hosted gateways, managed gateways, projects | the run's entries |

Whatever the finalizer removes is a leak the per-scenario cleanup missed and is logged as one.

The finalizer **never** deletes:

- anything the organisation already had, or another run created, including environments,
  deployment pipelines, the other consoles' resources and the publications of their APIs;
- the projects `apip-test-project-1` and `apip-test-project-2`, the gateway the starter built and
  the portal. They stay so that the next run finds them and uses them like any resource the
  organisation already has. The starter's suffix is not one the run's finalizer looks for;
- the default gateway of an environment;
- a managed API Portal of any kind, except one a scenario built for itself (`syn-portal-`). It
  empties the portals of what the run published and nothing else.

A person who wants the starter's gateway gone names its suffix to the setup command (below).

## Running them by hand

```bash
# check every precondition, building the projects and the gateway the organisation lacks (a missing
# portal is a failure while `starterCreatesPortal` is false)
go run ./suites/cloud/setup/cmd/cloudsetup start -endpoint "$CLOUD_PLATFORM_API_ENDPOINT"

# check only the default gateway (or `project`, `pipeline-project`, ... for both projects)
go run ./suites/cloud/setup/cmd/cloudsetup start -endpoint "$CLOUD_PLATFORM_API_ENDPOINT" \
  -precondition default-gateway

# delete what carries the suffix abcde234: a killed run's leftovers, or the gateway `start` built
go run ./suites/cloud/setup/cmd/cloudsetup finalize -endpoint "$CLOUD_PLATFORM_API_ENDPOINT" \
  -suffix abcde234
```

`-precondition` limits `start` to one precondition and may be repeated to name several; without it
`start` checks every one. `-suffix` names a suffix for `finalize` to delete and may be repeated;
without it `finalize` deletes only what the setup state file records.

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
