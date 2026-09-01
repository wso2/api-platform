# Migration policy

The legacy suite was written by many hands over a long period, so the same idea is expressed
several ways. The migration is a single-author rewrite and is expected to normalise that.

**The mandate:** deduplicate, consolidate, and rewrite as a senior QA engineer would — and
strengthen assertions where they are weak. **The constraint:** coverage and assertion strength
must never decrease. Those pull in opposite directions, so the rules below exist to make the
constraint checkable rather than asserted.

## What the legacy vocabulary actually looks like

Measured across all 70 feature files: **284 distinct step phrases**.

| Concept | Competing phrasings | Uses |
|---|---|---|
| Deploy an API | `deploy this API configuration:` / `deploy an API with the following configuration:` / `deploy a test API with the following configuration:` | 565 |
| Assert status | `response status code should be N` / `response status should be N` | ~1400 |
| Assert success | `response should be successful` (2xx) and `status code should be 200`, frequently both on one response | 1097 + 864 |

All three deploy phrasings already bind to ONE implementation. The synonyms are not expressing
different intent; they are drift.

## Rules

**1. One phrase per concept, enforced in the FEATURE text.**
Aliasing synonyms in the step glue hides the drift instead of removing it, and each alias is a
phrase a future author may copy. Pick the clearest phrasing, rewrite the features to it, and
register exactly one pattern.

**2. Never trade a specific assertion for a general one.**
This is the rule that protects strength, and it is directionally asymmetric:

- `status code should be 200` → `response should be successful` is a **LOSS** (200 becomes any
  2xx). Forbidden.
- `response should be successful` → `status code should be 200` is a **GAIN**, and is the
  preferred direction when the product has exactly one correct status.

Where a feature asserts both on the same response, keep the specific one and drop the general
one — that is deduplication, not weakening.

**3. Strengthen deliberately, not incidentally.**
Adding an assertion is welcome — a response body that was never checked, a status that was
only checked as "successful". Do it as a named change, not silently folded into a rename, so a
reviewer can see what became stricter and disagree.

**4. Removing a scenario requires proving it is redundant.**
Two scenarios are duplicates only if they exercise the same configuration AND assert the same
things. Different data through the same path is NOT redundant — it is the table-driven case the
original author wrote on purpose. When in doubt, keep it.

**5. Fixed sleeps become bounded polling.**
`I wait for N seconds` is not coverage; it is a guess about timing. Replace with a wait on the
condition actually required. This strictly increases reliability and never changes what is
asserted.

**6. Every deletion is recorded.**
A scenario or assertion that is dropped goes in the ledger below with its reason. "Coverage did
not decrease" must be auditable after the fact by someone who was not here — an unrecorded
deletion is indistinguishable from an accident.

## Rule: mock third-party services, NEVER our own products

A mock may stand in for something we do not build — an upstream backend, a third-party SaaS,
an external identity provider. It must NEVER stand in for a product in this repository.

When a feature needs another WSO2 product, pull that product's real image and model it as a
framework-provisioned component with its own config overlay, exactly as `platform-gateway` is
modelled. A hand-written mock of our own product tests the mock's behaviour, not the product's:
it drifts silently, cannot regress when the real thing changes, and its passing tells you
nothing about whether the two components actually interoperate.

The same rule in the framework this is modelled on: product-apim provisions a real WSO2 IS
container for identity, and mocks only the node backend.

### Audit of every mock in the catalog

| Mock | Stands in for | Ours? | Verdict |
|---|---|---|---|
| `backend`, `echo`, `mcp` (testbench services) | upstream backends | no | legitimate |
| `mock-jwks` | an external IdP's JWKS/token endpoint | no | legitimate |
| `mock-aws-bedrock-guardrail`, `mock-azure-content-safety`, `mock-embedding-provider`, `mock-analytics-collector` | third-party SaaS | no | legitimate |
| `mock-interceptor-service` | a user-supplied interceptor backend | no | legitimate |
| `redis` | real Redis image | no | legitimate |
| **`mock-platform-api`** | **platform-api, which is in this repo** | **YES** | **VIOLATION — replace with the real product** |

One violation, and it is the one that matters: `tests/mock-servers/mock-platform-api/main.go`
fakes token verification (accepts ANY non-empty api-key), hardcodes the connection ack, and
deliberately returns HTTP 500 for `/subscription-plans` and `/apis/{id}/subscriptions`. A
gateway passing against it has demonstrated nothing about real control-plane interop.

`tests/integration-e2e/docker-compose.yaml` already runs the real platform-api alongside a
gateway and is the reference for doing this properly.

## Canonical vocabulary

The phrasing every migrated feature uses. Chosen by which form is clearest, with the most-used
form winning ties — normalising toward the majority keeps the diff small.

| Concept | Canonical | Absorbs | Lines affected |
|---|---|---|---|
| Deploy an API | `I deploy this API configuration:` | `I deploy an API with the following configuration:`, `I deploy a test API with the following configuration:` | 123 |
| Assert status | `the response status code should be N` | `the response status should be N` | 132 |
| Wait for a fixed time | *(removed — see rule 5)* | `I wait for N seconds`, `I wait for policy snapshot sync` | 131 + 38 |

### Deliberately NOT merged

Pairs that look like synonyms and are not. Recorded because merging them is the plausible
mistake, and the reason is not obvious from the phrasing:

- **`I set header "X" to "X"` vs `I send a GET request to "X" with header "X" value "X"`.**
  The first is *sticky* — it applies to every later request in the scenario. The second is
  *one-shot*. Collapsing them would leak a header into subsequent assertions in scenarios that
  deliberately send one request with it and one without.
- **`I send a GET request to "X"` vs `I send a GET request to the "X" service at "X"`.**
  The first addresses the gateway data plane; the second addresses a named component in the
  topology. Different targets, not different wording.
- **`deploy this API configuration:` vs `deploy this MCP configuration:` / `deploy this LLM
  proxy configuration:`.** Different product resources with different endpoints and lifecycles.
- **`response should be successful` vs `response status code should be 200`.** Not synonyms —
  the first is any 2xx. See rule 2; the merge direction is one-way.

## Ledger

Per migrated feature: scenarios and assertions before and after, and the reason for any drop.

| Feature | Legacy scenarios | Migrated | New steps | Notes |
|---|---|---|---|---|
| api_deploy | 7 | 7 | — | Normalised: 2 × `I wait for 2 seconds` removed (rule 5), replaced by an explicit readiness wait. Retains the k8s-style status-object special case in `jsonFieldEquals`. |
| dynamic_endpoint | 8 | 8 | 4 | Readiness bound raised from a hard 9s to the block propagation ceiling. |
| content_length_guardrail | 18 | 18 | 1 | Addressing only. |
| word_count_guardrail | 13 | 13 | 0 | Addressing only. |
| respond | 30 | 30 | 5 | Addressing only. Forced the retry-on-404 removal below. |
| json_schema_guardrail | 28 | 28 | 0 | Addressing only. |
| regex_guardrail | 23 | 23 | 0 | Addressing only. |
| prompt_template | 14 | 14 | 0 | Addressing only. |
| cel_conditions | 6 | 6 | 3 | Addressing only. Exposed the keyword-restriction bug below. |
| model_round_robin | 16 | 16 | 1 | Addressing only. |
| **Total** | **163** | **163** | **14** | |

Counts are per feature file; the suite executes each in every block of the matrix.

**New step definitions per feature fall to zero as the vocabulary converges** — three consecutive
features (65 scenarios) needed none. That is the argument for ordering the migration by step
COST rather than scenario count: the order originally recorded in `PILOT-STATUS.md` would have
started with the 6-scenario feature that needed 3 new steps, ahead of the 65 that needed none.

**No scenario and no assertion has been dropped.** The only removals so far are the two fixed
sleeps, which asserted nothing.

### Two defects the migration found in its own step glue

Both were introduced by the port, not present in the original, and both are recorded because
they are the failure mode this ledger exists to catch — a change that looks like a faithful
port but silently alters behaviour.

1. **Header precedence.** Sticky headers were applied AFTER the explicit content type, so a
   scenario setting `Content-Type: application/json` overrode the `application/yaml` of a
   later management deploy. The gateway reported `failed to parse JSON: invalid character
   'a'` — the first byte of `apiVersion` — naming neither the header nor the step.

2. **Header lifetime.** Request-shaping state was put in `Local` scope, which is per RUNNER;
   a header set in one scenario therefore applied to every scenario after it. The original
   got per-scenario lifetime from its step struct. Now reset in a `Before` hook rather than
   depending on features calling `I clear all headers`, which only works where an author
   noticed.

3. **Retry folded into invocation.** The data-plane invocation steps retried until the status
   was not 404, to ride out the gap between a deploy being accepted and the route being
   programmed. That is indistinguishable from a 404 the product returns DELIBERATELY, and
   `respond` asserts 404, 410 and 503 as correct answers — those scenarios would have burned
   the full propagation window and then failed with a timeout, reporting nothing about the
   correct response the gateway gave instantly. Waiting for routability is now a separate,
   explicit step, which is what the original always did: all 29 `respond` scenarios already
   called `I wait for the endpoint ... to be ready` before invoking.

4. **Keyword-restricted registration.** Steps were registered with `sc.When`/`sc.Then`, which
   match only that keyword — while Gherkin's `And`/`But` inherit the keyword of the step
   above. So `And I clear all headers` written after a `Then` is a Then-step, and the
   When-registered pattern silently did not match: the step reported UNDEFINED despite being
   registered and correct. Six steps in `cel_conditions` failed this way. All registrations
   are now keyword-agnostic `sc.Step`, as the original suite used.

All four share a shape worth naming: **each worked for every feature migrated so far and
failed on the first feature containing the triggering input.** None would have been found by
migrating one feature at a time and declaring victory. Batch migration is what surfaces them.


---

## Mock audit — what should NOT be a mock

Done before Phase 3, because it decides what the remaining 49 features need. The rule is
already stated above: mock a BACKEND; never mock a product we ship, and never mock a
third-party product that has a runnable official image.

### Must be the real product

| Mock | Verdict |
|---|---|
| `mock-platform-api` (596 lines) | **Removed from the framework catalog.** It served `/api/internal/v1/...` and the gateway websocket — our own control-plane contract. `PlatformAPI()` runs the real product instead. |

### Legitimately mocked — leave them

`mock-aws-bedrock-guardrail`, `mock-azure-content-safety`, `mock-embedding-provider` and
`mock-analytics-collector` all stand in for third-party SaaS with nothing runnable in CI.
Three of those are now testbench services rather than components — `content-safety` on 3005,
`embeddings` on 3004 and `analytics` on 3007 — which changes where the code lives, not whether
mocking is legitimate.
`mock-interceptor-service` mocks a CUSTOMER-supplied interceptor, which is a backend.
`sample-service`/`echo` are generic upstreams, now testbench services.

`mock-analytics-collector` carried a caveat that has now been designed out rather than lived
with: it BUFFERS events and exposes `/test/events`, so it is stateful and could not be a shared
testbench service AS-IS. It is one now because it partitions those buffers by block key
(`/<block>/v1/events`, `/<block>/test/events`), with the key injected by the framework and
never settable from a feature — see `testbench-design.md` and `testbench.Partitioned`. It serves
~13 scenarios across analytics-basic, subscription-analytics and analytics-header-filter, and is
reached through `[analytics.publishers.moesif] moesif_base_url` — no feature names the host.

### Arguably should be real, and currently covered by nothing

`mock-jwks` serves `/jwks` + `/token`: an identity provider's surface. There is **no real IdP
anywhere in this repo**, yet platform-api ships IdP-specific behaviour — `default_config.go`
defaults to the flat `roles` claim "what Asgardeo and Entra ID use" and notes "Keycloak
overrides it with realm_access.roles". That mapping has never met a real provider.

Recommendation: KEEP the mock for the negative cases (a deliberately wrong issuer, a bad
audience, an expired token are trivial to mint against a mock and awkward against a real IdP —
which is most of jwt_auth), and add ONE block running a real Keycloak to prove interop and the
claim mapping. That is new coverage, not a migration.

### Not a mock-rule issue, found while auditing

- `rakhitharr/mcp-everything:v3` backs **8 features** and appears in two `kubernetes/helm`
  demo resources. It is a PERSONAL Docker Hub account, unofficial and not digest-pinned. The
  MCP "everything" reference server is published officially; use that.
- Ten `:latest` tags in one legacy compose file, including unmaintained
  `kennethreitz/httpbin:latest`. Non-reproducible: a silent upstream change becomes a mystery
  failure.

### The lesson about how this audit was done

The first pass grepped feature files for each mock's HOSTNAME and reported
`mock-analytics-collector: 0 features`. That was wrong — it is addressed through
`test-config.toml`, like the bedrock guardrail before it. **Map mocks CONFIG-FIRST**, not by
searching feature text, or a component that no feature names looks unused.


---

## Fault injection: use a payload, not a mode switch

A scenario that tests how the product handles a FAILURE from another component must trigger
that failure with DATA, through the same steps as the success case — not with a step that puts
a collaborator into a failing mode.

The legacy suite did the latter: `Given I make the control plane reject artifact imports`
flipped a flag on the mock, after which every import failed. That is two problems at once. It
needs a stub with a test-only control surface, and it never proves the real component rejects
for a REASON — a global "fail everything" switch would pass even if the product's validation
did not exist.

The replacement uses a validation asymmetry between the two products: an artifact the GATEWAY
accepts and stores but the CONTROL PLANE cannot resolve.

```gherkin
Scenario: A push rejected by the control plane is recorded as failed
  When I create this LLM provider:            # the SAME step the success scenario uses
    """ ...spec references a template that was never pushed... """
  Then the gateway should record cp_sync_status "failed" for the "LlmProvider" artifact "..."
```

Zero new steps, no stub, no proxy, and the real rejection path runs.

How to find the asymmetry for a new case: look for per-item error handling in the receiving
product, then for a validation it applies that the sending product does not. For artifact
import that is `service/artifact_import.go` (per-artifact results) plus the unresolved
reference errors in `artifact_import_llm_provider.go` / `artifact_import_llm_proxy.go`.

The exception, and it is narrow: a component being UNAVAILABLE is not expressible as a
payload. Stop the container.

And before reaching for either — check whether the fault is already implicit in the topology.
`startup-db-bootstrap` tests that the gateway recovers from its own database when
control-plane startup sync FAILS; the legacy suite achieved that with a mock that
deliberately did not implement those endpoints. Running the feature in a block with no
control plane at all produces the same failure for real, with nothing to configure.
