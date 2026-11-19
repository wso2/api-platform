---
name: apip-local-dev
description: Run apim-saas's cloud consoles (apip-api-control-plane, apip-ai-workspace) locally against a local api-platform-1 checkout — build-and-redeploy loop, the OpenChoreo stub, common failure modes, and the checklist for when a feature works in api-platform-1 (OSS) but fails through the cloud wrapper. Use when asked to run either console locally, to reflect api-platform-1 changes in a running local instance, or when an OSS feature 404s/400s only when exercised through apim-saas.
---

# Running apim-saas's cloud consoles locally against api-platform-1

Needs both checkouts present on disk: this one (api-platform-1) and a sibling
`apim-saas` checkout — most file paths below are relative to whichever of the two is
named at each point, not to wherever this skill file itself lives.

**Before editing any source file in either repo, stop and confirm with the user
first** — show them the specific gap found (which checklist item, which file, what the
fix would be) and the exact change, and wait for a go-ahead before applying it. This
applies to every code change this skill leads to: a `deployableKinds` entry, a new
`pdk.Kind*` constant, a `config.toml` feature-flag section, a scope/role-mapping edit,
a new OpenChoreo-stub route — all of it. It does not apply to the routine, already-
authorized rebuild-and-redeploy loop itself (section 3) when the user has already asked
for their own in-progress edits to be reflected locally — that is the one thing this
skill does without re-asking each time, because re-running the same build/redeploy the
user just asked for is not a new decision. Diagnosing (reading code, curling endpoints,
checking `docker system df`, running through the checklist) is always free to do
without asking; only writing a fix is gated.

Two separate things are involved and most failures come from conflating them:

- **api-platform-1** (`github.com/wso2/api-platform`) — the OSS product: `platform-api`
  (Go backend) and the portals (`api-control-plane`, `ai-workspace`) plus
  `portals/cloud-plugins/` (cloud-only UI features, built from the same repo — see the
  `apip-cloud-extensions` skill for that wiring).
- **apim-saas** (a separate sibling checkout) — the cloud layer: `services/apip-platform-api` (a Go wrapper
  that imports OSS `platform-api` as a *library* and layers cloud-only resources —
  environments, pipelines, managed gateways, managed portals, project provisioning — on
  top via `platform-api/pdk`'s plugin system) and `services/apip-api-control-plane` /
  `services/apip-ai-workspace` (Dockerfiles that clone/copy the OSS portal + its cloud
  plugins and bake in cloud-specific `config.toml`).

A working local stack has three independent pieces running at once: the **platform-api
wrapper** (host process), the **OpenChoreo stub** (host process, stands in for real
cloud infra), and **one or more console containers** (Docker, built from local
api-platform-1 source). All three must be up; most "nothing works" reports trace back to
one of them not running or not sharing the same backend.

## apim-saas at a glance

Enough of apim-saas's shape to work in it without a separate orientation pass — this
section exists so the skill is self-contained even in a session that only has
api-platform-1 open (apim-saas's own `apip-cloud-extensions` skill, which covers the
frontend half of this in more depth, is likely not reachable from here).

**Backend — `services/apip-platform-api`.** A Go wrapper that imports OSS
`platform-api` as a library and registers exactly one `pdk.Plugin` with it:
`internal/plugin/plugin.go`'s `CloudPlugin`, the wrapper's whole composition root. Its
`Init(deps *pdk.Deps)` builds every cloud-only resource (environments, pipelines,
gateways, managed_portals, projects, deployments, auth) from the shared `pdk.Deps` the
OSS server hands it, and its own file header spells out the pattern for adding a new
one: *"Adding a resource is: add its own package under `internal/`, then a wiring block
in `Init` and a `RegisterRoutes` call here."* Each existing resource under `internal/`
follows the same three-file shape — `handler.go` (HTTP routes + request/response DTOs),
`service.go` (business logic), `occlient.go` (the client for whatever real backend it
proxies to, usually OpenChoreo's Cloud Platform API) — worth matching if a brand-new
cloud-only resource is ever needed (distinct from extending an *existing* one, which is
what items 1-7 below mostly are).

**Frontend — `services/apip-api-control-plane` / `services/apip-ai-workspace`.** Neither
directory carries any app source of its own anymore — both are Dockerfiles only. Each
one clones the pinned api-platform-1 release tag (or, for local dev, builds straight
from an on-disk checkout via `--build-context api-platform-local=<path>`) and copies
*only* that console's own
`portals/cloud-plugins/apip-cloud-ui/src/hosts/<console>.tsx` file (plus `plugin.ts`,
`runtimeFlags.ts`, and the branding assets it imports) into the console's own
checked-in-but-empty `src/cloud` stub — never the whole `apip-cloud-ui/src` directory
(a blanket copy would drag the *other* console's host file along too, whose cross-portal
import would resolve inside the monorepo checkout but sit outside this console's own
`tsconfig` `rootDir` and fail `tsc --noEmit`). The console's own `main.tsx` imports
`cloudExtensions` from that stub unconditionally, so whatever the host file registers
compiles straight into the image — adding a new extension to an *existing* feature
package needs no Dockerfile or apim-saas-side change at all, only the api-platform-1
side (items 3-4 below cover when it still doesn't show up).

## 1. Platform-api wrapper (`services/apip-platform-api`)

One-time: symlink the wrapper's `.local/{platform-api,common,httpkit}` to your
api-platform-1 checkout (`./scripts/link-platform-api.sh <path-to-api-platform-1>`) — the
wrapper's `go.mod` `replace`s point there, so this is what makes "edit api-platform-1,
rerun the wrapper" actually pick up the change.

Write `config.local-dev.toml` next to the wrapper's `main.go` (gitignored — the
wrapper's own `-config` flag is **not** repeatable/mergeable, unlike OSS platform-api's,
so this must be one complete file, never an overlay on `config.toml`):

```toml
[platform_api]
db_schema_path                = "./.local/platform-api/internal/database/schema.sql"
openapi_spec_path             = "./.local/platform-api/resources/openapi.yaml"
llm_template_definitions_path = "./.local/platform-api/resources/default-llm-provider-templates"

[platform_api.logging]
level = "info"
format = "text"

[platform_api.security]
encryption_key = '{{ env "APIP_PAPI_ENCRYPTION_KEY" }}'

[platform_api.database]
driver = "sqlite3"
path   = "./data/api_platform.db"

[platform_api.server.https]
enabled   = true
port      = 9243
cert_file = "<api-platform-1>/platform-api/resources/certificates/cert.pem"
key_file  = "<api-platform-1>/platform-api/resources/certificates/key.pem"

# Only needed if a console's backend hop has no TLS-skip-verify option (ai-workspace's
# control_plane client doesn't — see "Known failure modes" below). Leave disabled
# otherwise; both listeners can run together.
[platform_api.server.http]
enabled = false
port    = 9080

[platform_api.auth]
mode = "file"

[platform_api.auth.authorization]
enabled = false
mode    = "scope"
role_to_scope_mapping = "<api-platform-1>/platform-api/resources/role-to-scope-mapping.yaml"

[platform_api.auth.jwt]
issuer           = "platform-api"
public_key_file  = "<api-platform-1>/platform-api/resources/keys/jwt_public.pem"
private_key_file = "<api-platform-1>/platform-api/resources/keys/jwt_private.pem"

[platform_api.auth.file.organization]
id = "default"
display_name = "Default"
region = "us"

[[platform_api.auth.file.users]]
username      = "admin"
password_hash = '{{ env "APIP_CP_ADMIN_PASSWORD_HASH" }}'
roles         = ["ap_admin"]

[platform_api.auth.internal_token]
skip_validation = false
```

cert/key material and the two env values come from api-platform-1's own one-time setup:
`ADMIN_USERNAME=admin ADMIN_PASSWORD=admin make -C platform-api setup-local-dev` (from
the api-platform-1 checkout) generates `resources/{certificates,keys}/` and
`platform-api/local-dev.env` (holds `APIP_CP_ADMIN_PASSWORD_HASH`).

Run it with the full env block (every one of these matters — the cloud-only resources
404 or 500 without them):

```bash
export APIP_PAPI_ENCRYPTION_KEY=$(cat <api-platform-1>/platform-api/resources/keys/encryption.key)
export APIP_CP_ADMIN_PASSWORD_HASH=$(grep APIP_CP_ADMIN_PASSWORD_HASH <api-platform-1>/platform-api/local-dev.env | cut -d= -f2-)
export CLOUD_PLATFORM_API_URL="http://localhost:8090"
export PLATFORM_IDP_TOKEN_URL="http://localhost:8090/token"
export API_PLATFORM_SYSTEM_CLIENT_ID="stub-client"
export API_PLATFORM_SYSTEM_CLIENT_SECRET="stub-secret"
export GATEWAY_DP_DOMAIN="portal.test:8091"
export CLOUD_CP_TLS_SKIP_VERIFY="true"
export MANAGED_PORTALS_DB_HOST="localhost"
export MANAGED_PORTALS_DB_NAME="portals"
export MANAGED_PORTALS_DB_USER="portals"
export MANAGED_PORTALS_DB_SECRET_REF="stub-db-secret"
export MANAGED_PORTALS_DB_SECRET_PROPERTY="password"
GOWORK=off go run . -config config.local-dev.toml
```

Serves HTTPS on `:9243` (and `:9080` plain HTTP if enabled above). `go run` recompiles
on every start, so api-platform-1 source edits (including to `platform-api/pdk`) are
picked up on the next restart with no extra step.

## 2. OpenChoreo stub (same repo, `openchoreo-stub/`)

Real WSO2 Cloud provisions environments/pipelines/managed-gateways/managed-portals
through a real OpenChoreo control plane; nothing in apim-saas or api-platform-1
reproduces that. `openchoreo-stub/stub_server.py` is a small, dependency-free Python
server that answers the exact HTTP contract the wrapper expects, backed by a JSON file —
not real infra, just enough CRUD to stop every cloud-only resource from 500ing locally.

```bash
python3 openchoreo-stub/stub_server.py 8090 8091 8092
```

- `:8090` — the OpenChoreo Cloud Platform API + fake Platform IDP token endpoint
  (`CLOUD_PLATFORM_API_URL`/`PLATFORM_IDP_TOKEN_URL` point here).
- `:8091` — HTTPS portal/gateway responder (self-signed cert) for managed-portal
  readiness probes and managed-gateway provisioning.
- `:8092` — a **plain-HTTP twin** of the same portal responder. Needed because
  `platform-api`'s own Publish-to-portal HTTP client (`internal/service/http_portal_publisher.go`)
  has no TLS-skip-verify option and rejects the `:8091` self-signed cert — any API
  Portal you need `Publish` to actually work against must have its `url` row repointed
  at `http://localhost:8092` (`UPDATE api_portals SET url = 'http://localhost:8092' WHERE
  handle = '<handle>';` against the wrapper's sqlite db). This is per-portal: every new
  portal still mints with the `:8091` URL by default.

A gateway's `isActive` field is **unrelated to the stub** — it's native platform-api
state, true only while a real WebSocket connection to
`/api/internal/v1/ws/gateways/connect` is live. To light one up: create the native
record (`POST /proxy/api/v0.9/gateways {"id":...,"functionalityType":"regular"|"ai",...}`),
rotate a token (`POST /proxy/api/v0.9/gateways/{id}/tokens`), then hold
`openchoreo-stub/fake_gateway_client.py <platform-api-base> <token>` running in the
background — it also acks `<kind>.deployed`/`<kind>.undeployed` events so a real deploy
actually settles into `DEPLOYED` instead of sitting in `DEPLOYING` forever. Creating a
managed gateway through the console only writes the binding row (the native record is
normally created by a real "integrator" callback this stub doesn't reproduce) — the
stub's `POST /cloud/gateways` writes that binding row directly via sqlite as the
documented out-of-band stand-in.

**A brand-new organization is not auto-seeded with anything locally** — in real cloud
this comes from external bootstrap infra outside both repos. For a working "new user"
experience, create by hand through the normal APIs: a `default` project, two
environments (a single-environment pipeline is rejected — every promotion path needs a
real target), a `default` pipeline promoting between them, and a gateway in the first
environment, then activate it as above. `"default"` as a gateway handle is reserved by
the real product for the gateway auto-created with an environment, which doesn't happen
here — write its binding row directly via sqlite rather than fighting the reservation.

## 3. Console containers, built from local source

```bash
docker buildx build -f Dockerfile \
  --build-context api-platform-local=<api-platform-1> \
  --no-cache \
  -t ghcr.io/wso2/api-platform/<image>:<tag> -t ghcr.io/wso2/api-platform/<image>:latest \
  --load .
```

Run from `services/apip-api-control-plane` or `services/apip-ai-workspace`. **Always
pass `--no-cache`.** Without it, BuildKit has silently reused a cached layer for the
"copy local checkout into the build" step on observed occasions — a rebuild reports
success but the running container still serves the old UI. `--no-cache` costs the full
~2-3 min rebuild every time but is the only way to be sure.

`ai-workspace` additionally ships two cloud-only `patches/*.patch` files
(`services/apip-ai-workspace/patches/`) meant for the pinned release tag; against a
fast-moving local api-platform-1 checkout they can stop applying (`git apply` fails,
build errors out before it even reaches the source). If so, temporarily rename them
(`mv foo.patch foo.patch.skip-local-build`), build, then rename them back — never leave
them renamed, that silently drops real cloud behavior from a real build.

Minimal `docker-compose.yaml` service shape (both consoles follow this; adjust image/
port/env per console):

```yaml
services:
  api-control-plane:
    image: ghcr.io/wso2/api-platform/apip-api-control-plane:<tag>
    environment:
      APIP_ACP_BILLING_URL: "https://billing.invalid/api"   # dummy is fine for local
      APIP_ACP_CONTROL_PLANE_URL: "https://host.docker.internal:9243"
      APIP_ACP_CONTROL_PLANE_TLS_SKIP_VERIFY: "true"
    ports: ["8082:8082"]

  apip-ai-workspace:
    image: ghcr.io/wso2/api-platform/apip-ai-workspace:<tag>
    environment:
      # ai-workspace's control-plane client has NO tls_skip_verify knob at all, so it
      # cannot use the :9243 HTTPS listener's self-signed cert the way api-control-plane
      # does — point it at the wrapper's plain-HTTP listener instead (enable
      # [platform_api.server.http] in config.local-dev.toml above).
      APIP_AIW_CONTROL_PLANE_URL: "http://host.docker.internal:9080"
      # File-auth platform-api issues plain JWT claim names; config.toml's own default
      # maps Thunder/cloud-IDP names instead, which silently leaves the SPA with an
      # empty org/identity after an apparently-successful login.
      APIP_AIW_AUTH_CLAIM_MAPPINGS_ORGANIZATION: "organization"
      APIP_AIW_AUTH_CLAIM_MAPPINGS_ORG_NAME: "org_name"
      APIP_AIW_AUTH_CLAIM_MAPPINGS_ORG_HANDLE: "org_handle"
      APIP_AIW_AUTH_CLAIM_MAPPINGS_USERNAME: "username"
    ports: ["5380:5380"]
```

`docker rm -f <container>` then `docker compose up -d <service>` after every rebuild —
redeploying restarts the BFF too, since both consoles bake the BFF binary and the SPA
build into one image/container.

Login on both: `admin` / `admin`.

## Known failure modes, in the order to check them

- **`ENOSPC: no space left on device` mid-build.** Not the host disk — Docker Desktop/
  Rancher Desktop's VM disk, filled by `--no-cache` build cache (seen growing past 40GB
  across a day of rebuilds). `docker system df` to confirm, `docker builder prune -af`
  to fix (may take several minutes; rerun if it reports `context canceled` partway).
- **Wrapper fails to start with `missing go.sum entry for module providing package
  ...`.** api-platform-1 pulled in a new dependency upstream (e.g. GraphQL support
  added `gqlparser`) that the wrapper's own `go.sum` hasn't caught up with, since the
  wrapper imports api-platform-1 as a library via a local `replace`. Fix: `GOWORK=off go
  mod tidy` in `services/apip-platform-api`.
- **A cloud-plugin page has doubled padding, or a small button renders full-width.**
  The host shell (`AppLayout.tsx`'s `<PageContent>`, or ai-workspace's equivalent) already
  wraps every routed page — a feature package returning its own `<PageContent>` as its
  root stacks a second padding on top. Fix is NOT to return a bare `<>...</>` fragment
  instead, though: the host wraps routed content in a flex `<Stack>`, so multiple
  top-level fragment children each get `align-items: stretch` individually (this is
  exactly what makes a small button render edge-to-edge). Use a plain `<Box>` as the
  root instead — block, non-flex, absorbs the stretch without adding padding. Exception:
  a feature package shared with a host whose shell does *not* auto-wrap (AI Workspace's
  own `DefaultLayout` doesn't; `apip-cloud-ui-insights`'s `InsightsFeature.tsx` shows the
  right pattern — wrap conditionally per host, don't strip unconditionally).
- **The scratchpad compose file / background host processes are gone after a multi-day
  gap in the same session.** Scratchpad directories get cleaned between long gaps; the
  compose file above is cheap to recreate from scratch. Check `ps aux | grep -E
  "apip-platform-api|stub_server.py|fake_gateway_client"` before assuming anything's
  still running.

## Cloud-side integration surface: what a new api-platform-1 feature might still need

This is the question to run through **whenever a developer has built something in
api-platform-1 locally and asks "does this need anything on the cloud side too?"** —
not only when something has already failed. The cloud wrapper is a separate,
independently-maintained integration layer that has to be told about each new thing
api-platform-1 adds; it does not infer it, and a feature can be 100% correct and
complete in api-platform-1 while still being invisible, half-wired, or rejected the
moment it's exercised through apim-saas's wrapper or either console's local build. Go
through every item below for a new feature, not just the first one that seems relevant
— these are independent gaps and a feature commonly needs two or three of them at once
(the Agent Proxy feature needed items 1 and 2 together, for example).

1. **A new artifact/resource kind.** `services/apip-platform-api/internal/deployments/handler.go`'s
   `deployableKinds` map is a hand-maintained whitelist of what this wrapper will deploy
   — a kind fully wired into OSS `platform-api`'s own `DeploymentsByKind`
   (`platform-api/internal/service/deployments_by_kind.go`) still 400s here
   (`"<kind>" is not an artifact kind that can be deployed.`) until added to this map
   too. Before adding it, check whether `platform-api/pdk/deps.go` even has a
   `pdk.Kind<Thing>` constant for it yet — the pdk package's public list sometimes lags
   behind what's internally wired (confirmed by `platform-api/pdk/kinds_test.go`'s
   `TestKindConstantsMatchThePlatform`, which pins each `pdk.Kind*` to its internal
   `constants.*` counterpart). If the constant is missing, add it there first
   (matching the existing entries' exact pattern, plus the test case), then reference
   the real constant from this wrapper's whitelist — never hardcode the string locally
   as a workaround; that papers over a gap this test exists to catch everywhere else.
2. **A new UI feature flag never reaching the browser.** Both consoles' BFFs only
   expose a `config.toml` key to the SPA's `window.__RUNTIME_CONFIG__` if the key
   *exists in the parsed config* (see ai-workspace's `bff/internal/config/runtime_config.go`'s
   `browserSafeKeys` allowlist and its "absent means off" comment) — setting only the
   matching env var does nothing if the cloud-baked `config.toml`
   (`services/apip-ai-workspace/config/config.toml` /
   `services/apip-api-control-plane/config.toml`) never declared the section at all.
   Add the section with the same `'{{ env "APIP_..._..." "false" }}'` token convention
   every other boolean in that file already uses, then set the env var in compose.
3. **A new cloud UI extension not registered for this host.** Extensions are registered
   per host in `portals/cloud-plugins/apip-cloud-ui/src/hosts/{api-control-plane,
   ai-workspace}.tsx` — see the `apip-cloud-extensions` skill for the full Slot/Hideable/
   Port pattern. A feature package existing under `portals/cloud-plugins/
   apip-cloud-ui-<feature>/` is not itself enough; it has to be imported and registered
   in the specific host file for each console it should appear in — registered for
   `api-control-plane` only is a silent no-op for `ai-workspace`, and vice versa.
4. **A new cloud-plugin package not reaching the Docker build at all.** Confirm it's a
   real sibling directory under `portals/cloud-plugins/` in the api-platform-1 checkout
   you're building from (`--build-context api-platform-local=...`) — the Dockerfile
   wires in every directory there automatically, so this is rarely the actual gap, but
   worth a `ls portals/cloud-plugins/` gut-check before going deeper into 1-3 above.
5. **A new scope nobody's role grants yet.** A new route or action usually comes with
   a new `ap:*`/`dp:*` scope. Two places both need it for it to actually be enforced:
   the OpenAPI scope registry api-platform-1's `platform-api/resources/openapi.yaml`
   feeds (`middleware.LoadScopeRegistry`, loaded at `server.go`'s startup) — the route's
   operation needs the scope named there — and
   `platform-api/resources/role-to-scope-mapping.yaml`, which is the *only* place a
   role's grants are defined (no per-user scope list anywhere to fall back to). Easy to
   miss locally because apim-saas's local-dev config usually sets
   `[platform_api.auth.authorization] enabled = false`, which silently allows every
   authenticated request regardless of scope — a feature can look fully working in this
   local setup while still being unreachable for every role except the one granted every
   scope in real cloud. Check both files whenever a feature adds a new permission, not
   just when something refuses a request locally.
6. **A new artifact kind with no declared gateway restriction.** `services/apip-platform-api/internal/deployments/gateway_kinds.go`'s
   `gatewayKindsForArtifact` map says which gateway kind(s) (`"regular"` vs `"ai"`) an
   artifact kind may deploy to. An artifact kind *absent* from this map deploys to any
   gateway unrestricted — deliberately permissive so a brand-new kind is never
   accidentally blocked, but worth a conscious decision for a new feature rather than
   leaving it as an oversight: should this new kind actually be restricted to `"ai"`
   gateways the way MCP/LLM-proxy/LLM-provider are?
7. **A new kind of cloud resource the local OpenChoreo stub doesn't know about.** This
   one only matters for *testing the feature locally*, not for real cloud — but it's
   easy to lose an hour to. If the feature introduces a new kind of OpenChoreo-shaped
   resource beyond what `openchoreo-stub/stub_server.py` already answers for
   (environments, deploymentpipelines, projects, resources/resourcereleases, gateways,
   portal publish/apis), the wrapper will 404/500 against the real
   `CLOUD_PLATFORM_API_URL` endpoint shape with nothing to fix in api-platform-1 or
   apim-saas at all — the stub itself needs a new route added, following the existing
   generic `_handle_oc_resource` pattern for a plain CRUD resource, or a bespoke handler
   if the new resource needs special-cased behavior (the way `resourcereleasebindings`
   and managed-gateway binding writes are special-cased today).

Resist the urge to patch around a missing OSS constant (item 1) with a wrapper-local
string literal, or to set an env var without first checking the config.toml declares
the key (item 2) — both look like they work but leave the actual gap for the next
person. When reporting back to whoever asked "does this need cloud changes," name which
of the seven items actually apply and which you checked and ruled out — a silent "looks
fine" is not the same as having gone through the list.

## Worked example, start to finish

api-platform-1 had just gained Agent Proxy support end to end: deploy/build services on
the OSS backend (`platform-api/internal/service/deployments_by_kind.go` already wired an
`AgentProxy` entry alongside `RestApi`/`Mcp`/`LlmProxy`), and a full console feature —
`AgentProxyOverview.tsx`, `apip-cloud-ui-deploy`'s deploy UI, the `ai-workspace.tsx` host
registration. Asked "is this working end to end locally," the answer came from going
through the checklist, not from re-reading the new UI code:

1. **Artifact kind whitelist** — `curl .../deployments?kind=AgentProxy` against the
   locally-running `apip-api-control-plane`/`apip-ai-workspace` returned `400 "AgentProxy"
   is not an artifact kind that can be deployed.` Traced to
   `services/apip-platform-api/internal/deployments/handler.go`'s `deployableKinds` map
   — hand-maintained, and nobody had added `AgentProxy` to it yet even though the OSS
   `DeploymentsByKind` fully supported it. One level deeper: `platform-api/pdk/deps.go`
   (this repo) didn't have a `pdk.KindAgentProxy` constant either, unlike
   `pdk.KindGraphQLAPI`, which *had* been added when GraphQL support landed — a genuine
   gap in this repo's own public plugin surface, not something either side "forgot" in
   isolation. Fixed properly rather than worked around: added `KindAgentProxy =
   "AgentProxy"` to `platform-api/pdk/deps.go` (matching the existing entries' exact
   shape) plus the matching case in `platform-api/pdk/kinds_test.go`'s
   `TestKindConstantsMatchThePlatform`, *then* referenced the real `pdk.KindAgentProxy`
   constant from apim-saas's whitelist — not a local string literal standing in for a
   constant that should exist.
2. **UI feature flag** — the console's Agent Proxy pages were gated behind
   `agent_proxy_enabled`, read from `window.__RUNTIME_CONFIG__`. Setting
   `APIP_AIW_FEATURE_FLAGS_AGENT_PROXY_ENABLED=true` in the container's environment did
   nothing on its own: `services/apip-ai-workspace/config/config.toml` (the cloud-baked
   config, separate from api-platform-1's own dev-only `configs/config.toml`) had no
   `[ai_workspace.feature_flags]` section at all, and the BFF's
   `buildRuntimeConfig`/`browserSafeKeys` only ever surfaces a key that *exists in the
   parsed config* — an unset section beats a set env var every time. Added the section
   with the same `'{{ env "..." "false" }}'` token convention every other boolean in
   that file already uses, *then* the env var took effect.
3. **UI extension registration** — already done by the feature itself
   (`ai-workspace.tsx` registered the Agent Proxy deploy slot); nothing to add. Named and
   ruled out, not skipped.
4. **Docker build wiring** — `apip-cloud-ui-deploy` was a pre-existing sibling package
   under `portals/cloud-plugins/`, already wired automatically. Ruled out.
5. **Scopes** — the deploy/build routes Agent Proxy reuses already carry the same
   scopes REST/MCP/LLM-proxy deploy use; no new scope was introduced. Ruled out.
6. **Gateway-kind restriction** — out of scope for this particular pass; left as a
   follow-up decision rather than silently assumed either way.
7. **OpenChoreo stub coverage** — Agent Proxy deploys through the same
   environments/pipelines/gateways shapes every other artifact kind does; no new
   OpenChoreo-shaped resource. Ruled out.

Net result reported back: *"Needed items 1 and 2 together — a real upstream pdk gap plus
a missing feature-flag declaration, both now fixed and rebuilt. 3, 4, 5, 7 already fine.
6 is a product decision, not a bug, still open."* That two-line answer is the point of
running the full list every time: it's short precisely because six of the seven were
checked and ruled out, not because they were assumed fine.
