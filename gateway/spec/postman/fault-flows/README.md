# Fault flows — Postman kit with a mock LLM backend

Exercises the policy-engine fault flow with `[policy_engine.llm_errors] disable_openai_format = false` (the default):
gateway-produced errors on `LlmProvider` / `LlmProxy` routes come back in the OpenAI envelope

```json
{"error":{"message":"…","type":"…","param":null,"code":"…"}}
```

while the provider's own errors, successful responses and REST APIs are left alone.

| File | What it is |
|---|---|
| `docker-compose.yaml` | Isolated stack: gateway-controller, gateway-runtime and the mock backend |
| `config.toml` | Gateway config with the option (and `handle_upstream_faults`) on |
| `fault-flows.env` | Admin user `admin` / `admin` (bcrypt hash) and runtime settings |
| `mock-backend.py` | Mock LLM backend, Python standard library only |
| `fault-flows.postman_collection.json` | The collection |
| `run.sh` | Builds the images, starts the stack, runs the collection |
| `policies/sample-sdk-fault` | Test policy: rejects and describes the failure in SDK `FaultDetails` |
| `policies/sample-fault-handler` | Test **fault** policy: stamps what it saw, can re-describe, write a body or change the status |

## Run it

```bash
gateway/spec/postman/fault-flows/run.sh          # build + up + test
gateway/spec/postman/fault-flows/run.sh build    # images only
gateway/spec/postman/fault-flows/run.sh up       # start the stack
gateway/spec/postman/fault-flows/run.sh test     # run the collection (newman, or npx newman)
gateway/spec/postman/fault-flows/run.sh down     # stop it and drop its volumes
gateway/spec/postman/fault-flows/run.sh clean    # remove the build worktree
```

Needs Docker, git, Go (the image builds run the unit tests first) and newman or npx.

**What `build` does.** LLM fault policies (`globalFaultPolicies` / `operationFaultPolicies`) need the
controller half of the fault flow (wso2/api-platform#3714), which is on upstream `main` but not on
this branch's base. So `build` creates a separate git worktree on `upstream/main` (`BASE_REF` to
change it), applies this branch's diff to it — committed and uncommitted — adds the two sample
policies to that worktree's `build.yaml`, and builds `local/fault-flows-gateway-runtime:fault-flows`
and `local/fault-flows-gateway-controller:fault-flows`. This checkout, its `build.yaml` and the stock
images are not touched. The Envoy `code_details` controller change this branch carries is skipped:
upstream `main` has it.

| Variable | Default |
|---|---|
| `BASE_REF` | `upstream/main`, else `origin/main` |
| `BUILD_DIR` | `$TMPDIR/apip-fault-flows-build` |
| `TAG` | `fault-flows` |

Host ports are shifted so the stack runs beside a stock gateway:

| Port | Service |
|---|---|
| `28080` | Router (Envoy) |
| `29090` | Management API (`admin` / `admin`) |
| `29002` / `29004` | Policy-engine admin / health |
| `28009` | Mock backend (direct access, for poking at it) |

In Postman: import the collection and *Run collection* (folders in order). `0. Setup › Wait for
deployment` retries until the routes are live; `Z. Teardown` removes everything it created (all
names start with `ff-`).

To see the contrast, set `disable_openai_format = true` in `config.toml`, run
`docker compose -f gateway/spec/postman/fault-flows/docker-compose.yaml restart gateway-runtime`, and
re-run: folders 1, 2 and the rendered cases of 7 fail (the bodies are no longer OpenAI-shaped).

## Mock backend

The gateway forwards request headers upstream, so the scenario is picked per request with
`x-mock-scenario` (or `?mock=` on a direct call). Every response it sends carries
`x-mock-backend: true`, which is how the tests tell its responses from the gateway's.

| Scenario | Response |
|---|---|
| `ok` (default) | 200 OpenAI chat completion |
| `long` | 200 completion with 60 words (trips the response guardrail) |
| `echo` | 200 echoing method, path, headers, body |
| `openai-429` | 429 already in the OpenAI envelope |
| `anthropic-529` | 529 in Anthropic's shape |
| `vendor-418` | 418 a vendor's own JSON |
| `plain-500` / `html-502` | text / HTML error |
| `empty-429` / `empty-500` | error with no body |
| `status-<code>` | that status with a small JSON body |
| `slow` | waits `x-mock-delay` seconds (default 5), then 200 |
| `reset` | TCP reset without answering |

```bash
curl -i localhost:28009/v1/chat/completions -H 'x-mock-scenario: anthropic-529'
```

## Resources the setup creates

| Resource | Upstream | Purpose |
|---|---|---|
| `ff-openai` provider | mock | `word-count-guardrail` on request (max 5 words of `$.prompt`) and response (max 20 words), plus `respond` edge paths |
| `ff-openai-bare` provider | mock, `resilience.timeout: 2s` | No body-reading policy: router errors must still be rewritten |
| `ff-openai-dead` provider | `mock-llm:1` | Connection refused |
| `ff-proxy` proxy | → `ff-openai-bare` | Proxy-level guardrail with `showAssessment: true` |
| `ff-rest` REST API | mock | Contrast — never reshaped |
| `ff-sdk` provider | mock | `sample-sdk-fault` / `respond` / `word-count-guardrail` per path, with `sample-fault-handler` as `operationFaultPolicies` |
| `ff-fp-upstream` provider | mock | `globalFaultPolicies`: re-describe |
| `ff-fp-dead` provider | `mock-llm:1` | `globalFaultPolicies`: re-describe a router failure |
| `ff-fp-body` provider | mock | `globalFaultPolicies`: write a body |

## Guardrail rejections

The shipped guardrails write `{"type":"…_GUARDRAIL","message":{"interveningGuardrail":…,"action":…,"actionReason":…,"direction":…,"assessments":…}}`.
On an LLM route that becomes:

```json
{"error":{"message":"<actionReason>","type":"invalid_request_error","param":null,"code":null,
  "guardrail":{"interveningGuardrail":"word-count-guardrail","action":"GUARDRAIL_INTERVENED",
               "actionReason":"<actionReason>","direction":"REQUEST",
               "assessments":{"details":"<the guardrail's assessment>"}}}}
```

`assessments` appears only when the guardrail's `showAssessment` is on. An object assessment is
carried as it is; a string or array is carried under `details`.

## What is covered

| Folder | Cases | Expected |
|---|---|---|
| 1. Policy rejections | request guardrail, **response** guardrail, policy REST body (403), plain-text body (429), body already OpenAI-shaped, explicitly empty body, proxy guardrail, `stream: true`, `Accept: application/xml` | Envelope with status kept and the policy's message lifted; a guardrail's body also yields `error.guardrail` (`interveningGuardrail`, `action`, `actionReason`, `direction`, and `assessments` only with `showAssessment`); an OpenAI-shaped body is kept byte-for-byte; an empty body stays empty |
| 2. Router failures | refused connection, connection reset (route with and without policies), route timeout | 503 `101503` / 504 `101504`, `server_error`, no Envoy text |
| 3. Backend errors | OpenAI 429, Anthropic 529, vendor 418, text 500, HTML 502, bodyless 429/500 — each on a route with policies and one without | Byte-for-byte pass-through, bodyless stays bodyless |
| 4. Success | 200 via both providers and the proxy | Untouched |
| 5. Contrast | same cases on a REST API | Not reshaped |
| 6. Known limitations | request matching no API | 404 not reshaped (no API kind to decide by) |

| 7. SDK FaultDetails and fault-policy priority | see below | see below |

## SDK `FaultDetails` and fault-policy priority (folder 7)

`sample-sdk-fault` returns `ImmediateResponse{Fault: &FaultDetails{…}}` (or
`DownstreamResponseModifications{Fault: …}` with `phase: response`), optionally with a fallback body.
`sample-fault-handler` runs on the fault path; it always sets
`x-sample-fault-handler: source=…; policy=…; code=…` with what it saw, and can re-describe the failure
(`message`, `code`, `type`; `inherit: false` sends only those fields), write a `body`, or change the
`statusCode`.

What the client gets, highest priority first:

| # | Who decides | Result |
|---|---|---|
| 1 | A **fault policy writes a body** | That body, byte-for-byte — on a gateway error *and* on a backend error |
| 2 | A **fault policy re-describes** the failure | The envelope from its description. It replaces the producer's description as a whole (`inherit: false` shows a dropped message falling back to the status text) |
| 3 | The **producing policy's `FaultDetails`** | The envelope from it. `Description` is never rendered |
| 4 | The producing policy's **body** | Beside a `FaultDetails` it is only a fallback and is not sent. Alone it is reshaped (message lifted; a guardrail's body becomes the `guardrail` block) unless already OpenAI-shaped or empty |
| — | A **backend error** | Passed through even when a fault policy re-describes it; only a fault policy body (1) replaces it |

| Request | Expected |
|---|---|
| `/ff-sdk/sdk/plain` | 401 envelope from `FaultDetails`: `authentication_error`, code `900901`, no `Description` |
| `/ff-sdk/sdk/fallback` | Same; the fallback body is not sent |
| `/ff-sdk/sdk/guardrail` | 422 with the `guardrail` block, object `assessments` as is |
| `/ff-sdk/sdk/response` | 422, response-path description, direction `RESPONSE` |
| `/ff-sdk/fp/stamp` | Unchanged envelope + `x-sample-fault-handler: source=gateway; policy=sample-sdk-fault; code=900901` |
| `/ff-sdk/fp/redescribe` | Fault policy's message and code `999001` |
| `/ff-sdk/fp/replace` | Code `999002`, message falls back to `Unauthorized` |
| `/ff-sdk/fp/body` | `{"custom":"fault policy body"}`, status 401 |
| `/ff-sdk/fp/status` | 429 `rate_limit_error` from the fault policy |
| `/ff-sdk/fp/legacy` | `respond`'s legacy body re-described: code `999003` |
| `/ff-sdk/fp/guardrail` | Legacy guardrail body's block kept, code `999004` added |
| `/ff-fp-upstream/…` + backend 429 | Backend body passes through; header shows `source=backend` |
| `/ff-fp-dead/…` | 503 envelope with the fault policy's message, code `101503` |
| `/ff-fp-body/…` + backend 429 | The fault policy's body |
