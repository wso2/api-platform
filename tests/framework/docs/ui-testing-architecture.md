# UI testing under the v2 framework — feasibility & design

Status: **proposed** — research complete, nothing implemented. Verdict up front: **feasible,
and a clean fit**. The engine, catalog and launcher are already suite-agnostic; a `suites/ui`
mirroring `suites/it` (features/, steps/, ui-suite.yaml) needs no engine change. The two
decisions that make or break it are where the browser runs (answer: in a container on the
block's network) and how scenarios authenticate (answer: the portals' basic/local mode plus
storage-state reuse — no IdP required). Everything below was verified against the repo and
current upstream state (2026-08); citations are file:line or links.

---

## 1. What exists today

**Three portals, two architectures** (`portals/`):

| Portal | Shape | Port | Auth default | Backend needs | In catalog? |
| --- | --- | --- | --- | --- | --- |
| api-portal (developer portal) | Server-rendered: Node 24 / Express 5 + Handlebars | 9543 | `local` — the portal's backend form-POSTs to platform-api and sets a session cookie; the browser only ever sees the portal origin | platform-api; Postgres (only engine its schema supports) | **Yes** — `framework/core/catalog/apiportal/definition.go`, boot-proven by its integration test |
| api-control-plane (management console) | React 19 SPA + Go BFF in one image; BFF serves the SPA and owns auth | 8082 (plain HTTP) | `basic` — login is a fetch to the BFF's own `/api/auth`; the BFF talks to platform-api server-side | platform-api; stateless (no DB) | No — new Definition + compose needed |
| ai-workspace | Same SPA+Go-BFF shape | 9643 (HTTPS self-signed, Secure cookies) | `basic` | platform-api (+ gateway for AI flows); stateless | No — and its Dockerfile needs `common`/`httpkit` build contexts the others don't |

**platform-api is catalog-complete** (`framework/core/catalog/platformapi/definition.go`: HTTPS 9243, health,
config injection, sqlite self-migrating / postgres+sqlserver schema-provisioned, generated
TLS + keys, admin auth). So an api-portal UI block is provisionable **today** from existing
catalog parts.

**Prior art in-repo is Cypress, not Playwright**: `portals/api-portal/it/` runs a
`cypress/included` container on the same compose network as api-portal + platform-api, and
`portals/ai-workspace/cypress/` exists too. That suite is the working precedent for the
browser-in-container pattern; Playwright is net-new tooling here. (wso2/apim-apps also uses
Cypress — scenario ideas, not architecture.)

**The engine is suite-agnostic — verified, not assumed.** A suite supplies exactly: a
`suite_test.go` (TestMain parallel-budget + one `engine.Run` call), `engine.Deps{RepoRoot,
FeatureRoot, Steps, CleanupDeleters?, Coverage?}`, a steps package, and its suite YAML.
Nothing in `framework/core/runtime`, `topology`, or the catalog registry references the
gateway. `steps/base.go`'s HTTP/JSON layer is generic and reusable for the API-side setup
steps a UI scenario needs.

---

## 2. Decisions

### U1 — `suites/ui` mirrors `suites/it`; no engine changes.

Same layout (`features/`, `steps/`, `ui-suite.yaml`, `suite_test.go`), same registry +
`topology.LoadFile`, same two-level concurrency (blocks × runners, scenarios sequential per
runner), same tcontext scopes and cleanup registry. The UI steps package registers its own
`sc.Before`/`sc.After` hooks beside the engine's — godog supports multiple hooks, and
`deps.Steps(sc, topo)` runs inside the ScenarioInitializer, so the browser lifecycle needs no
engine surgery.

### U2 — The browser runs IN the block's docker network, as the official Playwright image in server mode. This is the load-bearing decision.

`mcr.microsoft.com/playwright:v<pinned>-noble` joined to the block network via the existing
launcher, running `npx playwright run-server --port 3000 --host 0.0.0.0`; the Go side
connects with `BrowserType.Connect("ws://...")` over the mapped port.

Why in-network and not a host-side browser launching local browsers:

- **The redirect/alias problem.** Portals and platform-api address each other by network
  alias. A host browser cannot resolve `api-portal:9543`, and api-portal's configured
  `base_url` (`https://localhost:9543`) scopes cookies/redirects to a host:port a mapped
  ephemeral port never matches. In-network, the browser resolves the SAME aliases and
  canonical ports as every sibling container — nothing needs per-block base-url surgery, and
  a future OIDC IdP redirect works unchanged.
- **Colima port fragility.** Host browsers depend on many host-published ports; this
  machine's history includes the grpc port-forwarder killing all of them at once. The
  in-network browser needs exactly one published port (the ws endpoint).
- **CI/local parity.** The browser environment is byte-identical on macOS dev and Linux CI;
  no `playwright install` on dev machines or runners.
- **It is the repo's established pattern** — the Cypress suite does exactly this.

The browser becomes a catalog component (image + `ws` endpoint 3000 + a TCP/HTTP health
gate), one instance per block like the testbench.

### U3 — Binding: playwright-go, with the version triad pinned and asserted.

`github.com/mxschmitt/playwright-go` (the module moved from `playwright-community` as of
v0.6100.0). State as of 2026-08: v0.6201.1 tracking Playwright 1.62.1, releases within days
of upstream, 4 open issues, maintained by a Playwright core-team member; remote `Connect` is
a real exercised path (recent releases fixed remote-connection deadlocks; issue #502 is a
user running exactly the testcontainers + run-server + Connect pattern). Alternatives
rejected: chromedp/go-rod are Chromium-only with no auto-waiting/locator parity — we would
rebuild Playwright's flake control by hand; a TS `@playwright/test` split would bifurcate the
suite into two languages and lose Gherkin-drives-the-browser.

**The versioning rule, asserted not documented:** the Go module minor encodes the Playwright
version (`v0.6201.x` == 1.62.1), and the browser image tag must match it
(`mcr.microsoft.com/playwright:v1.62.1-noble`). Skew is the binding's most-reported failure
mode. The suite derives the image tag from the module version (or pins both in one place)
and fails boot loudly on mismatch.

### U4 — Lifecycle: one browser server per block, one Browser connection per runner, one BrowserContext + Page per scenario.

- **Per block**: the Playwright server container (a topology component).
- **Per runner**: a `playwright.Browser` from `Connect`, held in the runner's Local scope,
  created lazily on first scenario, closed by the runner-end cleanup sweep. Scenarios within
  a runner are sequential (engine invariant), so one live context per runner at a time.
- **Per scenario**: a fresh `BrowserContext` (+ Page) in `sc.Before`, closed in `sc.After` —
  Playwright's own isolation recommendation (~20–50MB per context vs 150–400MB per browser).

Concurrency budget: contexts concurrent = runners in flight. On the 6-CPU/12GB Colima VM,
cap UI runner parallelism at **~3** and give the browser container a memory limit around
`400MB + 50MB × max contexts`. This is a block-level `parallel:` value in ui-suite.yaml, not
framework code.

### U5 — Auth: basic/local mode + storage-state reuse; no IdP.

There is no IdP in the catalog (only MockJWKS), and none is needed: every portal defaults to
basic/local auth against platform-api, in which the browser only ever talks to the portal
origin. Two rules:

1. Dedicated login scenarios exercise the real login UI.
2. Every other scenario bypasses it: log in once per runner (through the BFF/portal API),
   save `BrowserContext.StorageState()`, and boot each scenario's fresh context from that
   state (`StorageState` option). This is the single biggest flake-and-speed win the
   reference implementations agree on.

OIDC mode is explicitly deferred; when it comes, U2 already solves its redirect problem (the
IdP joins the network and the browser resolves it), and the catalog gains an IdP component
then.

### U6 — Step vocabulary: declarative features, POM-style page structs, selectors never in feature files.

- Features say intent: `When I publish the "orders" API`, never `When I click "#publish"`.
- A `pages/` package in `suites/ui/steps`: one struct per page/section owning locators
  (`GetByRole`/`GetByLabel` first, `data-testid` fallback) and actions. Steps call page
  methods; a DOM change touches one file.
- The portals adopt a `data-testid` convention (area-component-element) on section containers
  and key interactive elements — a small product-side ask that decides how well selectors
  age. Screenplay is noted and rejected for now: no idiomatic Go implementation, POM is the
  lower-friction fit.

### U7 — Flake policy: kill it at the source; no scenario retry.

Web-first assertions (auto-retrying `expect`) and auto-waiting actions only; fixed sleeps are
banned the way `awaitRouteProgrammed` was in the gateway suite — the same readiness
philosophy, ported: *wait for what a user would observe*. godog has **no retry** and its
maintainers refuse one (issues #199/#497/#658), which this design treats as a feature: the
gateway suite reached deterministic-green without retries and the UI suite is held to the
same bar. If CI ever genuinely needs a net, the wrapper re-runs failed feature files via a
second `go test` invocation — outside the framework, never inside it.

### U8 — Artifacts: trace + screenshot on failure, captured per scenario.

`sc.Before` starts `context.Tracing()` (screenshots+snapshots+sources); `sc.After` on failure
stops the trace to `<artifacts>/ui/<block>/<runner>/<scenario>-<ts>.zip` and takes a
screenshot; on success the trace is discarded. Videos only if traces prove insufficient
(cost). One mechanical consequence of U2: the browser container writes trace/video files in
ITS filesystem — mount a shared volume (or copy out via the docker API, the coverage
copy-out helper already exists) so artifacts land host-side. Traces open in
`npx playwright show-trace` / trace.playwright.dev; CI uploads the directory as a job
artifact.

### U9 — ui-suite.yaml: reuse what the catalog has, add what it lacks.

First block (everything already exists):

```yaml
blocks:
  - name: portal-core
    components:
      - name: platform-api      # catalog-ready; sqlite (self-migrates, no DB container)
      - name: api-portal        # catalog-ready; REQUIRES postgres (its only engine)
      - name: browser           # NEW catalog component: playwright server image
    runners:
      - name: onboarding
        features: [features/portal_login.feature, ...]
```

`defaults.components` must name engines for platform-api (sqlite) and api-portal (postgres) — the
gateway suite dropped them from its defaults, and the load-time DB planner fails fast on a
missing engine. Later blocks add `api-control-plane` and `ai-workspace`, each needing a new
catalog Definition + compose (modelled on platformapi.go); ai-workspace's Dockerfile
additionally needs the `common`/`httpkit` build contexts wired into whatever builds its
image. Gateway + testbench join blocks whose scenarios exercise end-to-end API/AI flows.

### U10 — Coverage is a build and collection concern.

Source-built Go services may expose `GOCOVERDIR` data without product HTTP endpoints. The
framework collects that data after graceful shutdown. Frontend coverage remains a separate
JavaScript reporting concern and is not mixed with Go coverage.

---

## 3. Risks

| Risk | Mitigation |
| --- | --- |
| Version triad skew (Go binding ↔ driver ↔ browser image) | One pinned constant; boot-time assertion that the connected server version matches the binding. The binding's most-reported failure class. |
| Browser memory under parallel runners | Runner parallelism ≤3 on 12GB; container memory limit sized to contexts; contexts-not-browsers per scenario. |
| godog has no retry | U7: flake-to-zero discipline (the gateway suite proved it's achievable); CI rerun wrapper as last resort, outside the framework. |
| No visual-regression assertion in playwright-go (`ToHaveScreenshot`, issue #337) | Out of scope for functional UI tests; hand-roll pixel-diff later only if genuinely needed. |
| Artifacts land inside the browser container | Shared volume or docker-API copy-out (helper exists from coverage work). |
| ai-workspace image build wiring (`common`/`httpkit` contexts) | Sequence it last (U9); api-portal and control-plane carry the suite until then. |
| Selector rot in React portals | U6's data-testid convention is a product-side prerequisite for the control-plane/SPA blocks; the server-rendered api-portal is more tolerant. |
| playwright-go is community-maintained (one maintainer) | He is Playwright core team; releases track upstream within days; 4 open issues. Accepted, monitored at each version bump. |

---

## 4. Phased plan

**Phase U0 — spike (the feasibility proof).** One block: platform-api (sqlite) + api-portal
(postgres) + browser component; one feature with two scenarios (portal renders; real login
via the UI). Proves: launcher runs the playwright image, `Connect` over the mapped ws port,
alias-resolution from browser to portal, artifacts out. Exit: green twice consecutively,
trace artifact lands host-side.

**Phase U1 — the api-portal suite proper.** Storage-state auth (U5), page structs + step
vocabulary (U6), artifact hooks (U8), 5–10 real journeys (browse APIs, subscribe, try-out).
Exit: green at runner parallelism 3; zero sleeps in the diff.

**Phase U2 — api-control-plane.** New catalog Definition + compose; data-testid pass over
the console's key screens; console journeys (create API, deploy, view analytics).

**Phase U3 — ai-workspace + CI.** Its Definition (+ build-context wiring), CI job (browsers
come with the pinned image — no runner install), artifact upload, and the version-pin
assertion in CI.

---

## 5. Open questions (decided at implementation, flagged now)

1. **Where the ws port is published** — the browser container needs one host-reachable port
   for the Go client; under Colima that's the VM address (the framework's `Instance.Host()`
   already handles this). Alternative if it ever breaks: run the godog process itself in a
   container on the block network (heavier; not the default).
2. **api-portal's `base_url`** — in-network browsing may still want it set to the alias form
   (`http://api-portal:9543`) per block; its config injection already supports overlays.
3. **Trace volume vs copy-out** — start with copy-out (helper exists, no mount-driver
   variance on Colima); switch to a volume if trace sizes make per-scenario copies slow.

## 6. Execution findings (implementation, 2026-08)

What the build proved out, and where reality diverged from the plan above.

### F1 — The block's real gateway must register as an AI gateway.

The workspace lists only gateways whose `functionalityType` is `"ai"`; the framework's
gateway registration provision minted `"regular"`. `IT_GATEWAY_FUNCTIONALITY_TYPE`
(catalog) now parameterizes the registration, and the UI suite's `TestMain` sets it to
`ai` — per suite, not per block, because `Provisions` has no wiring access and no suite
mixes both types yet.

### F2 — Workspace-defined provider templates never reach a gateway (product gap).

There is no `llmprovidertemplate.*` control-plane event; the controller only knows the
seven built-in templates it loads from disk at boot. A provider created from a custom
template deploys with `failed to retrieve template '<handle>': configuration not found`.
Consequence for the journey: the provider comes from the built-in **Azure AI Foundry**
template — one of the three built-ins (with Azure OpenAI and AWS Bedrock) that carry no
pinned `endpointUrl`, so the creation form exposes the upstream URL field and the mock
LLM address goes in the UI-real way.

### F3 — The proxy's "API key" field wants a platform-minted provider key.

An app LLM proxy loops back through its provider's own context on the same gateway, and
injects the credential from the proxy form on that hop. The provider's downstream
api-key-auth accepts only keys the platform minted for that provider — an arbitrary
string yields 401 `Valid API key required` on every proxy invocation. The journey
therefore generates a provider key through the provider overview's Generate API Key
dialog and pastes it into the proxy form, exactly as a user must.

### F4 — Invocation is asserted end to end, from the browser's network position.

The proxy overview renders the gateway invoke URL and names the `X-API-Key` header in
its key dialog; the invoke step reads both from the page and POSTs `/chat/completions`
through the browser context's API request facility (in-network, CORS-free). Key and
deployment propagate through the control plane asynchronously, so the step retries
until the gateway serves the request (bounded by `invokeAttempts`); the assertion is the
mock's canned completion text coming back — the user-visible proof of the whole chain.

### F5 — Failure artifacts: screenshot + DOM + trace, retain-on-failure.

Every scenario records a Playwright trace (started on its browser context); the After
hook keeps `<scenario>-<time>-trace.zip` plus a full-page screenshot and the DOM under
`suites/ui/artifacts/` only when the scenario failed. Traces stream back from the remote
browser through the driver, so nothing needs copying out of the container (U8's open
question, resolved in favour of the driver path).
