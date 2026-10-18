# Gateway Postman collections

## `llm-openai-compatible-errors.postman_collection.json`

Exercises the default `[policy_engine.llm_errors] disable_openai_format = false`: every gateway-produced
error on an `LlmProvider` / `LlmProxy` route is returned in the OpenAI error envelope

```json
{"error":{"message":"…","type":"…","param":null,"code":"…"}}
```

while the provider's own error documents, successful responses and REST APIs are left alone.
Design and rules: [`docs/gateway/fault-policies.md`](../../../docs/gateway/fault-policies.md#openai-format-errors-on-llm-apis).

The collection needs nothing but a stock gateway. Backend errors are simulated by a REST API
(`oai-pm-backend-sim`) built from the `respond` policy, which the LLM providers use as their
upstream — the gateway calls its own router. Every resource it creates is prefixed `oai-pm-` and
removed again by the Teardown folder.

### Prerequisites

1. A gateway (controller + runtime) with the option on — the default; check that the config the **policy engine** reads does not turn it off:

   ```toml
   [policy_engine.llm_errors]
   disable_openai_format = false
   ```

   For the docker-compose setup in `gateway/` that is `gateway/configs/config.toml`; restart
   `gateway-runtime` after changing it.

2. The runtime must be reachable from inside its own container as `gateway-runtime:8080` (true
   for `gateway/docker-compose.yaml` and the IT compose). Otherwise set `selfUpstream`.

### Variables

| Variable | Default | Meaning |
|---|---|---|
| `controllerUrl` | `http://localhost:9090/api/management/v1` | Management API base URL |
| `routerUrl` | `http://localhost:8080` | Router (Envoy) HTTP listener |
| `adminUser` / `adminPassword` | `admin` / *(empty)* | Management API basic-auth credentials — set the password |
| `selfUpstream` | `http://gateway-runtime:8080` | How the runtime reaches its own router (the providers' upstream) |
| `deadUpstream` | `http://gateway-runtime:1` | A host:port that refuses connections (router-failure cases) |
| `restSampleUpstream` | `http://sample-backend:5000` | Any URL; the simulator's operations respond locally and never call it |

### Running

Run the folders **in order** (the collection order): `0. Setup`, the seven `Template:` folders,
`Edge cases`, `Known limitations`, `Z. Teardown`. In the Postman app use *Run collection*; from a
terminal:

```bash
newman run gateway/spec/postman/llm-openai-compatible-errors.postman_collection.json \
  --env-var adminPassword=<password> --delay-request 50
```

If the first `Template:` requests return 404, the routes were not pushed yet — re-run
`0. Setup › Wait for deployment` and continue.

To see the contrast, run it once with the option **off**: the reshaping assertions fail and the
pass-through assertions still hold.

### What is covered

Per built-in template — `openai`, `azure-openai`, `azureai-foundry`, `anthropic`, `gemini`,
`mistralai`, `awsbedrock`:

| Case | Expected |
|---|---|
| `word-count-guardrail` rejection (REST-style body, no fault description) | 422 kept, OpenAI envelope, `invalid_request_error` |
| Request within the guardrail limit | 200, upstream body untouched |
| Upstream refuses connections (Envoy's own 503), route with no body-reading policy | 503 kept, `server_error`, router `code`, Envoy text gone |
| Backend 418 with its own body | passed through byte-for-byte |
| Backend 429 already in OpenAI shape | passed through byte-for-byte |
| Backend 529 in another vendor's (Anthropic) shape | passed through byte-for-byte |
| Backend 429 / 500 with **no** body | passed through as it is: status kept, still no body |

Edge cases (on the `openai` provider unless noted), each a `respond` body with no fault
description:

| Policy body | Expected |
|---|---|
| `{"error":"Forbidden","message":"…"}` (403) | `message` lifted, `permission_error` |
| `{"error":"invalid_token","error_description":"…"}` (401) | `error_description` lifted, `authentication_error` |
| `{"detail":"…"}` (404) | `detail` lifted, `not_found_error` |
| `{"error":"…"}` (400) | the string lifted, `invalid_request_error` |
| `{"message":{…object…}}` (422) | non-string skipped → status text |
| plain text (429) | the text becomes the message |
| HTML page (502) | markup is never a message → status text |
| empty (503) | status text |
| unusual status 451 | status kept, `invalid_request_error` |
| already the OpenAI envelope, with `param`/`code` set (400) | left byte-for-byte |
| `{"error":{"message":"…"}}` without `type` (409) | readable as-is → left byte-for-byte |
| successful 200 | untouched |
| `Accept: application/xml` | still the OpenAI JSON |
| `stream: true` / `Accept: text/event-stream` rejection | plain JSON error, as the reference API does |
| `HEAD` | status only, no body |
| LLM proxy guardrail rejection / allowed request | reshaped / 200 |
| Same body on a REST API | NOT reshaped |

Known limitations (asserted as current behaviour): a request that matches no API has no API kind,
so its 404 is not reshaped.

Not reachable from Postman with a stock gateway, covered by unit tests in
`gateway-runtime/policy-engine` instead: a body written by an operator's **fault policy** (kept as
written — no shipped policy is a fault policy yet), a policy that describes its failure in
`FaultDetails`, and the option being off for LLM kinds.
