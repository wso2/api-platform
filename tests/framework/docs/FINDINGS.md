# Product findings from the pilot migration

Ten findings. The first two are RED SCENARIOS inside otherwise-green features, left exactly as
written rather than tuned until they pass — an assertion relaxed until it goes green is an
assertion that no longer checks the thing it was written for. Findings 3 and 4 BLOCK scenarios
that are consequently not wired into the suite.

Findings 6 and 7 are a different kind: both were found by STRENGTHENING an assertion rather than
migrating one. Certificate seeding from the filesystem imports nothing, and every migrated
scenario passed regardless because the feature is database-backed — invisible in a green run on
both suites, since the legacy one mounted the directory and exercised the code without ever
checking what it did. Finding 7 came out of replacing `the response should be a client error`
with an exact status: the loose assertion was green for 400 and 404 alike, which concealed both a
mis-scoped scenario and the product defect underneath it. Finding 9 came out of the same pass.
Both are error-classification defects on the same verb — a malformed or unresolvable `PUT`
returns 500 and 404 respectively where 400 is declared, and only `llm-provider-templates` gets it
right.

Findings 1 and 2 are observations and are not confirmed on CI yet, so **confirm each on Linux
before filing**. There is no known environment artifact to blame them on: this setup runs 232
scenarios against one gateway with zero transport errors (PILOT-STATUS.md).

Findings 3 and 4 are not observations. Both are read directly from the two products' source,
and in each case the code comments state the behaviour is deliberate — so neither needs
reproducing so much as a decision about whether the behaviour is intended.

| # | Where | Symptom | Severity if confirmed |
|---|---|---|---|
| 1 | `policy_engine_admin` — Config dump reflects API deletion | the policy-engine dump never drops a deleted route (routing is FINE) | medium |
| 2 | `sandbox_routing` + `backend_timeout` — connect AND route timeouts | the gateway gives up ~1-2% early, proportionally | low/medium |
| 3 | `subscription_validation` — whole feature, not wired in | a gateway-originated API can never receive a subscription | high |
| 4 | `template_functions` — 2 scenarios, not wired in | a template is rejected at deploy in any non-string field | medium |
| 5 | `llm_cost_based_ratelimit` — feature not wired in | additive special cost classes appear not to be charged (INFERRED) | high |
| 6 | `certificates` — 1 parked scenario | filesystem certificate seeding imports nothing on a fresh database | medium |
| 7 | `llm_proxies` — 1 parked scenario | `PUT /llm-proxies/{id}` returns 500 for a malformed body; 400 unreachable | medium |
| 9 | `mcp_deploy` — 1 parked scenario | `PUT /mcp-proxies/{id}` answers 404 for every error; 400 and 500 unreachable | medium |
| 10 | `api_management` — no red scenario | DOC GAP: contract never says whether a read returns `spec.context` resolved or as authored | low |

Plus two watch items below: an LLM template that once failed to sync, and a harness gap with no
zero-cost settle after a policy update (which blocks 3 rate-limit scenarios).

Everything else is green: the suite runs 25 features and 342 scenarios, and the only failures
are scenarios 1 and 2 above.

---

## Finding 1 — a deleted route lingers in the policy-engine config dump

**DOWNGRADED, and the original claim was wrong.** This was filed as "a deleted API is still
SERVED", severity high. It is not. Measured properly, the data plane stops serving a deleted API
promptly; only the config dump keeps reporting the route. That is a reporting defect, not a
routing one.

**How the error happened, because it is an easy one to repeat.** The original scenario fired a
SINGLE data-plane request immediately after the DELETE and saw a 200 at roughly 0.3s. Deletion
propagates to Envoy asynchronously, so that request sampled the propagation gap rather than the
outcome. A single shot taken 300ms after a mutation cannot distinguish "never took effect" from
"had not taken effect yet", and it was written up as the former.

Re-run with the data-plane check as a POLL — `I wait for the endpoint ... to return 404` — the
scenario passes that step every time. The route stops being served. Only then does the dump
assertion fail, and by that point routing has demonstrably caught up, so what remains in the
dump is stale reporting about a route that is already gone.

The scenario now asserts in that order deliberately: data plane first and polled, dump second.
That ordering is what separates the two claims.

**Also checked:** this reproduces on a gateway rebuilt from current source, so it is not an
artifact of a stale image. The suite had been running a 2-day-old `:test` image at the time —
a hand-retagged copy that nothing rebuilt. That whole class of staleness is now gone: the suite
runs `make build`'s plain snapshot images, pinned to `gateway/VERSION` and read from that file
rather than duplicated in Go, so there is no hand-maintained tag left to go stale.

**THE DUMP NEVER CONVERGES — and a wrong measurement briefly suggested otherwise.** A live
capture appeared to show the entries clearing at ~32s, which would have made this a budget set
slightly too short rather than a defect. That reading was an artifact: the assertion times out at
30s, the block then tears down, and once the container is gone `curl | grep -c` returns 0 —
indistinguishable from "the route left the dump". The 32s matched teardown, not convergence.

Raising the poll to a bounded 90s settles it. The entries are STILL present at 90s, three times
the interval the bad measurement suggested. The policy engine's dump does not clear a deleted
route on any timescale a test can reasonably wait for, while the data plane stops serving it
promptly. That is a real reporting defect, not a slow one.

### What still fails



**Scenario:** `suites/it/features/policy_engine_admin.feature`, "Config dump reflects API
deletion". The other 10 scenarios in that feature pass.

**Reproduce:**

```
cd tests/framework/suites/it
go test -timeout 25m -blocks gateway-configdump -feature-tags "@policy-engine-admin"
```

**Sequence.** Deploy `admin-delete-test-api-v1` (context `/admin-delete-test/$version`, two GET
operations), wait for it to serve, `DELETE` it through the management API — which returns
success — then look at the policy engine.

**Measured, from two runs:**

```
~0.3s after the DELETE, the data plane STILL SERVES the route:
    GET /admin-delete-test/v1/health -> 200 {"status":"healthy"}      (expected 404)

30s of polling after the DELETE, the route is still in BOTH sections of
127.0.0.1:9002/config_dump:
    policy_chains  -> GET|/admin-delete-test/v1/health|*
    policy_chains  -> GET|/admin-delete-test/v1/info|*
    route_metadata -> GET|/admin-delete-test/v1/health|*
    route_metadata -> GET|/admin-delete-test/v1/info|*
```

So this is not a stale report. The route is still there and still routable.

**The controller side is fine.** `config_dump.feature`'s own deletion scenario passes, so the
CONTROL PLANE forgets the API while the POLICY ENGINE keeps serving it. That split is the
interesting part.

**Not a concurrency artifact.** It fails identically with the sibling runner filtered out.

### CORRECTED: the dump half is very likely a REGRESSION, not an untested gap

An earlier version of this entry said the legacy scenario "asserted only that the route left the
dump", and suggested checking whether it passed because the dump had never been POPULATED. Both
were wrong, and the legacy file settles it —
gateway/it/features/policy-engine-admin.feature:105-132:

    Then the config dump should contain route with basePath "/admin-delete-test/v1"   <- before
    When I delete the API "admin-delete-test-api-v1"
    Then the response should be successful
    And I wait for 3 seconds
    Then the config dump should not contain route with basePath "/admin-delete-test/v1"

It asserts the route IS present before deleting, so "never populated" is ruled out. And it
asserts the route LEAVES the dump — the same claim v2 makes, which now fails after 30 seconds of
polling rather than 3.

So if the legacy suite was green, this is a REGRESSION of a previously-passing assertion, not a
defect nobody tested for. Stronger claim, and it comes with a bisect target: whatever changed
between the legacy suite last passing and now.

TWO THINGS TO CONFIRM before stating it that way, neither established:
  - that the legacy scenario was actually green, rather than known-red or skipped;
  - that the legacy `should not contain` step inspects the same dump sections v2's does. v2
    checks BOTH policy_chains and route_metadata; if legacy checked only one, the two are not
    the same claim and the regression framing weakens.

### What IS newly covered: the data-plane half

**No feature — migrated or legacy — asserted that a deleted API stops being SERVED.** Every
other "404 after delete" is a management-API lookup (`GET /rest-apis/{name}`), never a data-plane
request. That assertion was added during this migration and failed on its first run.

### The "corroboration" was WITHDRAWN — it was the same measurement error

`token_based_ratelimit` / "Template change triggers cache invalidation" was written up as
independent evidence: it deletes a provider, recreates it with a larger limit, and got a 429
where a fresh quota was expected. That looked like deleted policy state surviving.

It was not. The scenario sampled once immediately after the recreate, and the recreated provider
is indistinguishable to any route-presence probe — same path, same method as the one deleted. The
only thing that separates old from new is BEHAVIOUR: the old quota was exhausted and answers 429,
the new one answers 200.

Polled on that condition, the scenario passes. It is now live and untagged, and no longer
supports this finding. Two apparent corroborations of "deleted state persists" have now both
dissolved into the same mistake — inferring a state change from a probe that could not observe it.

### Before filing

- Re-measure the data-plane probe a full 30s after deletion. The scenario currently aborts on
  the dump assertion first, so the `200` above was captured at ~0.3s in a separate run where
  the data-plane check ran first.
- Establish whether the legacy scenario was green. If it was, BISECT — this is a regression of an
  assertion that used to hold, and the change that broke it is findable.
- Confirm on Linux/CI.

---

## Watch item — an LLM provider template once failed to sync within 30s

Not promoted to a finding: seen ONCE in three runs, and it may be the same underlying defect
as Finding 1.

`lazy_resources_xds.feature` / "LLM provider template is synchronized to policy engine via
xDS" creates `xds-test-template`, gets a successful response from the management API, then
polls the policy engine's config dump for it. On one run — with the two sibling runners
(`configdump`, `policy-admin`) active against the same gateway — it never appeared within 30s:

```
lazy resource "xds-test-template" of type "LlmProviderTemplate" never appeared:
the policy engine's config dump did not reach the expected state within 30s
```

Re-run evidence:

| configuration | runs | result |
|---|---|---|
| whole `gateway-configdump` block (3 runners) | 3 | 1 failure, 2 clean |
| `lazy-resources` runner alone | 3 | 3 clean |

So it is concurrency-associated but NOT deterministic — unlike Finding 1, which reproduces
every time and reproduces alone.

Why it may be the same defect: both are "the control plane accepted the change and the POLICY
ENGINE never saw it". Finding 1 is a deletion that never propagates; this is a creation that
did not propagate within 30s under concurrent load. If Finding 1 is diagnosed, check whether
the same mechanism explains this.

Worth re-testing at higher repetition once Finding 1 is understood, rather than chasing
separately now.

---

## Finding 2 — 503 arrives ~130ms before the configured connect timeout

**Scenario:** `suites/it/features/sandbox_routing.feature`, "Sandbox ref should honor
upstreamDefinitions connect timeout". The other 22 scenarios in that feature pass.

**Reproduce:**

```
cd tests/framework/suites/it
go test -timeout 25m -blocks gateway-core -feature-tags "@sandbox-routing"
```

**Sequence.** Deploy an `upstreamDefinition` with `timeout: connect: 6000ms` pointing at
`192.0.2.1` (RFC 5737 TEST-NET-1, reserved non-routable so it can never answer), invoke it
through the sandbox vhost, and assert 503 after AT LEAST 6 seconds. The 503 is correct; the
timing is not.

**Measured elapsed, 12 runs (6 distinct values across the sqlite and postgres legs):**

```
5.841s   5.854s   5.872s   5.888s   5.901s   5.917s
```

Always short, never over, total spread 76ms. That consistency is what makes it look
systematic rather than noise: the gateway gives up ~100-160ms (~2%) before the configured
connect timeout.

### Why the assertion is not being relaxed

The lower bound is the entire point of the scenario. An upstream that refuses instantly
returns the same 503 as one that timed out correctly, so without "at least the configured
timeout" the test proves nothing about the timeout. Changing it to `5.8` would make it green
while deleting the only thing it checks.

### Ruled out

- **Not the readiness probe priming Envoy.** The first version of the wait sent a GET to the
  same dead route, which could plausibly have made the second attempt fail faster. Replacing
  it with a probe of the API's MAIN vhost — which never touches the dead upstream — changed
  nothing: 5.917 -> 5.872.
- **Not the measurement.** The time mark is recorded before the request and read after it; the
  steps in between are in-memory.

### Extended: it is not only CONNECT timeouts

`backend_timeout.feature` reaches the same wall on a ROUTE (response) timeout, which is a
different code path — a slow upstream answering late rather than an address that never answers,
504 rather than 503. Against a configured 2s route timeout, using the reflector's `/delay/{n}`:

```
1.969s   1.983s        (configured 2000ms, asserted ">= 2s")
```

Short again, never over. That matters for diagnosis in two ways.

It rules out anything specific to connect handling — whatever returns early does so for both
timeout families, so the cause is likely shared plumbing rather than the connect path.

And the margin looks PROPORTIONAL rather than fixed: ~130ms short of 6000ms is ~2%, ~20-30ms
short of 2000ms is ~1-1.5%. A fixed offset would have shown ~130ms here too. That points at
something scaling with the configured duration — a unit conversion, or a timer armed against a
slightly-scaled value — rather than a constant scheduling lag.

The two affected scenarios are tagged `@finding-2-early-timeout` and excluded from the runner.
The third scenario in that feature, which asserts only the 504 and not the elapsed time, passes.

### Before filing

- Inspect the cluster config the runtime is actually programmed with, via the policy-engine
  admin on 9002 (now published as the `policy-admin` endpoint), to see whether the connect
  timeout arrives slightly under 6000ms — and now also whether the ROUTE timeout arrives under
  2000ms. If both are short in the programmed config, this is a translation bug and not a
  runtime one, which would be much easier to fix.
- Test a third duration. Two points cannot distinguish proportional from merely "smaller for
  smaller timeouts"; a 20s timeout should be ~400ms short if the ratio holds.
- Establish whether the 503 is emitted on connect-timeout expiry at all, or on some other
  trigger that merely correlates with it.
- Confirm on Linux/CI.

---

## Finding 3 — a gateway-originated API can never receive a subscription

**Status:** confirmed against the real products, from source. Not yet filed.

**Claim.** A subscription created in the developer portal is delivered, accepted and broadcast
correctly, and the gateway then discards it — for any API that ORIGINATED on that gateway and
reached the control plane by the data-plane push (dp -> cp). The API goes on answering
`403 {"error":"forbidden","message":"Subscription required for this API"}` with the subscription
sitting in the control plane's database, marked ACTIVE.

**Why.** The two planes identify the same API by different UUIDs, and the event carries the
wrong one for this direction of travel.

- `platform-api/internal/service/artifact_import.go` is explicit that this is by design:
  *"The control plane owns the artifact UUID; it does NOT reuse the data-plane UUID"*, and the
  pushed UUID is kept only as `DPID`, *"informational (logging/traceability) and never used as
  the CP artifact UUID"*. Matching on re-push is by HANDLE, not id.
- `platform-api/internal/service/subscription_service.go` broadcasts
  `SubscriptionCreatedEvent{ApiId: sub.ArtifactUUID, ...}` — the CONTROL PLANE's UUID.
- `gateway-controller/pkg/subscriptionxds/subscription_snapshot.go` builds the set of APIs it
  will accept subscriptions for from its OWN configs — `apiIDs[cfg.UUID] = true` — and then
  skips every subscription whose `APIID` is not in that set.

For a CP-originated API the two agree, because the gateway's config was created from the control
plane's deployment. For a gateway-originated one they cannot: the control plane minted a fresh
UUID at import, deliberately.

**Observed.** Every step of the chain reports success and the last one silently drops the result:

```
portal    Delivered {"eventType":"subscription.created","status":200}
gateway   Received WebSocket event type=subscription.created
gateway   Processing replica sync event event_type=SUBSCRIPTION action=CREATE
gateway   Subscription snapshot updated successfully subscription_count=0   <-- dropped here
```

There is no warning on the drop. The neighbouring branch in the same function DOES log and abort
when a subscription references a missing plan; the missing-API case just `continue`s.

**Impact.** Subscription validation and per-subscription rate limiting are unavailable for any
API created directly on a gateway, which is the whole gateway-first workflow. The failure is
silent on both planes: the portal shows an active subscription, the control plane stores it, and
only the 403 at the data plane says otherwise.

**What this cost the test.** `subscription_validation.feature` was written the way the legacy
suite deployed — straight to the gateway — and cannot pass on that path. It needs the API created
on the control plane and deployed down. See the note in `it-suite.yaml`.

### Before filing

- Confirm the CP-originated path does pass, which pins this to the dp-origin case rather than to
  subscriptions generally.
- Decide the intended fix: match on handle in the snapshot, carry `DPID` in the event, or have
  the gateway record the CP artifact id against its own config (it already stores
  `cp_artifact_id`, which suggests the mapping exists and is simply not consulted here).
- At minimum, log the drop. A subscription discarded for an unknown API should not be quieter
  than one discarded for an unknown plan.

---

## Finding 4 — a templated value is rejected at deploy in any non-string field

**Status:** determined from source in both products. Not yet filed. Reproduced as two deploy-time
400s while migrating `template-functions`.

**Claim.** A `{{ env "..." }}` or `{{ secret "..." }}` template placed in a policy parameter that
the JSON schema types as anything other than a string is REJECTED at deploy time:

```
spec.operations[0].policies[0].params.quotas.0.limits.0.limit
  -> Invalid type. Expected: integer, given: string
spec.policies[0].params.allowCredentials
  -> Invalid type. Expected: boolean, given: string
```

So a rate limit cannot be driven by `{{ env "IT_RATE_LIMIT" }}`, and a CORS `allowCredentials`
cannot be driven by `{{ env "IT_ALLOW_CREDENTIALS" }}`. Templating is effectively string-only.

**Why.** The coercion that would fix the type runs, but too late.

- `gateway-controller/pkg/config/policy_validator.go` still has `CoerceRestAPIPolicies`, and its
  own doc comment says it "Must be called after template rendering (e.g. in the event listener)".
- `api_processor.go` shows the order: RenderSpec, then Coerce — both AFTER deploy, feeding only
  the in-memory store and the xDS snapshot.
- Deploy-time JSON-schema validation therefore sees the RAW template string sitting in an
  integer or boolean field, and rejects the configuration before anything renders.

**Not a migration artifact.** The templated values are byte-identical to the legacy source
(`- limit: '{{ env "IT_RATE_LIMIT" }}'`, `allowCredentials: '{{ env "IT_ALLOW_CREDENTIALS" }}'`),
and there is no other way to write them: a bare `{{` opens a YAML flow mapping, so a templated
value MUST be a quoted string. The legacy scenarios expected 201 at deploy plus the runtime
effects — a limit of 5 enforced as 4x200 then 429, and `Access-Control-Allow-Credentials: true`
on the response — and their inline comments state that coercion "turns it into the numeric value
5 so schema validation passes". That pipeline no longer feeds deploy-time validation.

**Impact.** Any operator config that parameterises a numeric or boolean policy field per
environment is unavailable, and the failure is a schema type error naming the field rather than
anything mentioning templates — so it reads as a malformed config rather than an unsupported
combination.

### Before filing

- Decide the intended behaviour: should deploy-time validation be template-aware, deferring type
  checks on any field whose value contains `{{ }}`? That is the product question underneath.
- Whatever the answer, the error message should say that a template cannot be used in a typed
  field. The current one sends the reader looking for a malformed value.
- Do NOT resolve this by retyping the schema field to string, which would remove type checking
  for every non-templated user of the same field.

---

## Watch item — no zero-cost settle after a policy update

Not a product defect claim. A HARNESS gap, recorded because it currently blocks three scenarios
and the workaround people will reach for is the one that destroys the test.

**The problem.** Rate-limit scenarios count: they assert that a specific request number crosses
the limit. Any readiness wait that sends a real request SPENDS a token from the bucket under
test, so the arithmetic shifts by one. The legacy suite dodged this with `wait 2 seconds`, which
costs nothing — but a fixed sleep is exactly what this framework refuses to reintroduce.

Two waits cost nothing, and neither is sufficient after an UPDATE:

- `I wait for policy snapshot sync` compares plane versions. After a policy-only update it can
  return immediately, because both planes still report the pre-update version and therefore
  compare equal.
- The config-dump check gates the policy engine, not Envoy — and it needs the dump overlay, so
  it is unavailable in the baseline block where these features run.

Every wait that DOES prove the data plane is serving the new configuration sends a request, and
that request is counted.

**Blocked, tagged `@needs-settle-primitive`:**

- `basic_ratelimit` — "Rate limit scope based on policy attachment level"
- `basic_ratelimit` — "Updating API adds then removes route-level policy for the same route"
- `ratelimit` — "Multiple limits per quota - enforces most restrictive limit"

The other 52 scenarios in those two features pass.

**THE MECHANISM, pinned to source.** `policy_chain_version` comes from the POLICY ENGINE's xDS
client — `gateway-runtime/policy-engine/internal/admin/handlers.go` returns
`h.xds.GetPolicyChainVersion()`, the version of the policy CHAINS the ext_proc engine received.
`xds_sync_status` compares that between controller and engine. It never observes ENVOY's route
table. Routes reach Envoy over a separate xDS path (RDS) from the chains that reach the engine.

So the primitive is correct for what it claims and wrong for what it is being used as:

- After a policy-only UPDATE the route already exists in Envoy, so snapshot sync IS a valid
  settle — nothing about the route table needs to change.
- After a fresh CREATE the RDS push can still be in flight once policy_chain_version already
  matches, so the first data-plane request 404s.

Any feature that treats snapshot sync as route readiness after creating a route is exposed to
this race, whether or not it has failed yet.

**It is not only UPDATES.** `llm_policy_path_specificity` fails all 8 scenarios with a 404 on
the first counted request, using policy snapshot sync as its only readiness. So sync can return
before Envoy serves a FRESHLY CREATED route too — the version genuinely advances on a create,
and it still is not a guarantee that the data plane is ready. That widens the gap: snapshot sync
is not a settle primitive for either create or update, it merely reports that the control plane
has moved on.

Those scenarios are exact-count and every path sits under a catch-all quota, so the one wait
that WOULD prove readiness — the route-programmed probe — spends a token and shifts every
expectation. That is the vice precisely: the only reliable signal costs the thing being measured.

**What would fix it.** Route readiness read from ENVOY rather than the policy engine — its RDS
state, or a route dump — without a data-plane request. The policy engine already exposes
`total_routes` at /config_dump, which may be the cheapest starting point. The requirement is
only that it observes the route table rather than the chain version, and costs no traffic
through the counted route — either by reading the version the data plane
reports, or by probing an unlimited operation on the same API and confirming the version stamp
advanced. An unlimited `/probe` operation alone is not enough: it answers 200 both before and
after a policy-only change, so it adds a round-trip without proving anything.

**What must NOT be done.** Adjusting the expected counts to whatever a run produces. The count
IS the assertion — tuned to observed behaviour, the scenario passes whatever the limiter does.

---

## Watch item — intermittent 500 on nested LLM rate-limit paths

Not promoted to a finding: measured 1 failure in 4 runs, and the cause is not yet isolated.

`llm_policy_path_specificity` / "Overlapping nested paths each apply only their most specific
advanced-ratelimit" intermittently returns

```
POST /nestedrl/chat/completions -> 500 {"error":"Internal Server Error"}
```

on a request the scenario expects to be 200. On one occurrence a SIBLING scenario (`proxyspec`),
unmodified and passing in every other run, returned 500 in the same run — so it may not be
confined to one scenario.

**It is not the route-readiness race.** This scenario originally failed with a 404 every time,
against `I wait for policy snapshot sync`. Switching it to the Envoy route-readiness wait
eliminated the 404 completely and deterministically — the route is programmed before the first
request. What remains is a different failure with a different status, exposed only once the
readiness problem stopped masking it.

Measured, same configuration, no code changes between runs:

| runs | result |
|---|---|
| 4 | 3 clean, 1 with a 500 |

**Why it is excluded rather than left in.** A scenario that fails a quarter of the time is worse
than one that is honestly absent: it trains readers to re-run rather than to look, and it would
be indistinguishable from the genuine product findings in the same file.

### Before promoting

- Read the policy-engine and gateway-controller logs during a failing run. A 500 on an LLM route
  usually means the policy chain errored rather than that routing failed, which would place this
  in the product rather than the harness.
- Establish whether it is confined to nested paths, or whether any LLM rate-limit scenario can
  hit it — the one sibling occurrence suggests the latter.
- Note the file also contains a KNOWN-RED product scenario (`advrl-path`, most-specific-path-
  wins), so do not conflate the two when reading failures from this feature.

---

## Finding 5 — the llm-cost policy does not appear to charge additive special cost classes

**Status: INFERRED, not measured.** The reasoning below is arithmetic over observed failures plus
the shipped pricing data. Nobody has read the calculator — it is a Python-bridge policy baked
into the runtime image, and the pricing keys appear in no readable source. One 60-second check
settles it; see below. Treat this as a strong hypothesis until that runs.

**Claim.** The pricing DATA supports what `llm_cost_based_ratelimit` asserts, and the policy does
not apply it. Specifically the classes that ADD cost — service-tier surcharge, geo/speed
multiplier, 1-hour cache creation, web-search per-query — appear not to be charged, so cost
collapses to base tokens and a budget sized for the real cost never trips.

**What is ruled out.** Base rates are NOT drifted: every base-token scenario passes (enforce,
anthropic, gemini, per-provider, zero-cost), and the shipped rates match the feature's cost
comments exactly. The mock is not at fault either — its responses arrive intact and carry the
right token counts.

**The arithmetic.** Cost if the special class were dropped, against what the scenario expects:

| scenario | expected | if class dropped | ratio |
|---|---|---|---|
| priority (tier surcharge) | 0.00105 | 0.00060 | 0.57 |
| geo-speed (x6.6 multiplier) | 0.00231 | 0.00035 | 0.15 |
| cache-1hr (+cache creation) | 0.00580 | 0.000175 | 0.03 |
| web-search (+2 x $0.01) | 0.02014 | 0.00014 | 0.01 |

All four are in the failing set. The undercharge is 2x to 100x, so the conclusion does not depend
on the exact block semantics.

**Why this is more than a fitted story.** The same model predicts which scenarios should NOT
fail: for `batch` and `gemini-cached`, dropping the class would OVERCHARGE and trip the budget
early — and neither is in the failing set. A hypothesis that only explained the failures would be
worth much less.

**Three that need a different explanation**, and they are separate mechanisms rather than
loose ends:

- `flex` (gpt-5.4) and `cached`: dropping the tier or discount would overcharge, yet both pass a
  request that should have been blocked. So they undercharge by another route — plausibly the
  calculator cannot price those entries at all (gpt-5.4 has an unusual nested
  `_above_272k`/`_flex`/`_priority` structure) and returns ~0.
- `reasoning` (o4-mini): reasoning tokens bill at the output rate with no surcharge, so base
  equals expected and it should pass. It does not, implying the calculator charges LESS than
  base — likely excluding `reasoning_tokens` from billed completion.
- `multiwin`: same model, endpoint and per-request cost as `enforce`, which passes. It differs
  only in having a second budget window, so this is multi-window enforcement, not pricing.

**Separately:** `mistral-small-latest` exists in the pricing file only as
`mistral/mistral-small-latest`. If the bare name does not resolve, the policy's documented
fail-soft (unknown model -> cost 0, do not block) makes that scenario's 429 unsatisfiable with
the shipped data.

### The check that settles this

The policy emits `x-ratelimit-cost-remaining-dollars`. Its per-request DELTA is the policy's own
computed cost, so no calculator source is needed:

- geo-speed: a delta near 0.00035 confirms the 6.6x multiplier is dropped.
- flex: near 0.001 means the tier is dropped; near 0 means gpt-5.4 is not priced at all.

Two requests resolve every row in the table above.

**Do not tune the budgets.** They are arithmetically correct against the shipped rates, and these
scenarios exist precisely to prove the special classes are charged. Tuned to fit current
behaviour, they would pass while asserting nothing.

---

## Watch item — a full-suite run stops progressing; "the Docker daemon dies" is RETRACTED

This entry exists mainly to stop the next person repeating the investigation. It is a HARNESS
observation, not a product finding, and its main content is a correction.

### What was claimed, and why it was wrong

For several sessions the working diagnosis was that the Docker daemon died during full-suite
runs. Effort went into memory sampling (`tools/vmwatch.sh`), dockerd RSS, container accumulation,
teardown ordering, scheduling, network multi-homing, and block count. Every one of those came
back clean, and each clean result was read as "so the cause must be elsewhere".

`journalctl -u docker` inside the VM says the premise was false:

    dockerd[1459] ran continuously 15:23:08 -> 17:14:59, the entire suite window.
    Zero panics. Zero fatals. Zero kernel OOM kills. Zero unexpected restarts.
    The only stop was systemd, gracefully, when the VM was manually restarted.

The daemon never died. Every measurement came back clean because there was no resource failure
to find. Two further hypotheses died the same way and are recorded so they are not re-proposed:

- **"We multi-home shared containers across per-block networks, product-apim does not."** False.
  `tests-common/testcontainers/NodeAppServer.java` does exactly this — a shared backend on
  `BACKEND_HOME_NETWORK`, `connectToNetworkCmd` per block, `disconnectFromNetworkCmd` at
  teardown. It is where our design came from.
- **"We run more blocks than product-apim."** False. `testng-v2.xml` declares 24 `<test>` blocks
  at `parallel="tests" thread-count="2"`. That is more blocks than our 16, same concurrency, on
  the same machine, working.

### What the log actually shows

    17:12:09  Container failed to exit within 10s of signal 15 - using the force
    17:12:10  ignoring event ... TaskDelete
              <- 2m49s of complete silence ->
    17:14:59  Stopping docker.service        (manual restart)

Nearly three minutes with no Docker API activity at all, during a suite that should be churning
containers constantly. The daemon was up, idle, and waiting. Nothing was asking it to do work.

So the open question is not why the daemon dies. It is **why the suite stops calling it**, and
that answer is on our side of the socket.

### The two candidates, and the measurement that separates them

1. The suite is blocked in its own code — a poll, a channel, a `retry.Until`, the teardown path.
2. Host->VM socket forwarding broke — colima proxies `/var/run/docker.sock` over ssh, so calls
   can stop reaching a healthy daemon.

Next stall, before restarting anything:

    kill -QUIT <test-pid>       # Go dumps every goroutine — names the blocked line for (1)
    colima ssh -- docker ps     # runs inside the VM; if this works and host `docker ps` hangs, (2)

Restarting the VM destroys the evidence for both, which is why every previous occurrence taught
us nothing. Note also that the journal is volatile: `journalctl --list-boots` keeps only recent
boots, so a VM restart can erase the window being investigated.

### Two real defects the log surfaced along the way

- **Containers ignore SIGTERM.** `failed to exit within 10s of signal 15 - using the force`
  appears for essentially every container, so each teardown pays a forced 10-second wait. Not a
  crash — but it makes full-suite teardown far slower than it needs to be, and slow teardown is
  easily mistaken for a hang.
- **A wait strategy polls containers that are already being torn down.** Repeated
  `exec failed: ... stat /bin/sh: no such file or directory`, ~5 times in 200ms. The first guess
  — a shell-less/distroless image — is WRONG: all four suite images
  (gateway-controller-coverage, gateway-runtime-coverage, platform-api, api-portal) have
  `/bin/sh`, verified by running it. The real cause is in the two lines preceding each failure:
  `stream copy error: reading from a closed fifo`. Closed stdio fifos plus a vanished `/bin/sh`
  means the ROOTFS WAS ALREADY UNMOUNTED. testcontainers' `wait.ForListeningPort` runs an
  internal check by exec'ing `/bin/sh -c '</dev/tcp/localhost/PORT'` (`wait/host_port.go:294`),
  and it was still polling a container being removed underneath it.

  Benign in itself — testcontainers maps this to `errShellNotFound`, logs "Shell not found in
  container", and returns nil because the external port check already passed. But it is a real
  signal about the stall: at 17:11:00 something was being torn down while its wait strategies
  were still live, i.e. a startup and a teardown overlapped on the same containers.

Separately, and still unfixed regardless of the above: network teardown leaks. `removing network
... has active endpoints` leaves orphaned UUID bridges that survive a VM restart (11 observed).
Real, worth fixing, and NOT claimed to be connected to the stall.

---

## Finding 6 — filesystem certificate bootstrap silently imports nothing, on every fresh database

The controller can seed its custom trust store from a directory of PEM files at startup. It
cannot. The directory is read, each file is parsed and validated, and then every one is
discarded — on any database that does not already contain it, which is precisely the state
seeding exists for.

### The chain

`pkg/storage/sql_store.go` is correct — absence is a sentinel, not a failure:

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrNotFound          // storage/errors.go: "configuration not found"
}
```

`pkg/certstore/certstore.go:466` does not special-case that sentinel:

```go
func (cs *CertStore) certificateExistsByName(name string) (bool, error) {
    cert, err := cs.db.GetCertificateByName(name)
    if err != nil {
        return false, err            // ErrNotFound leaves here as a failure
    }
    return cert != nil, nil
}
```

and the caller treats any error as "skip this file" (`certstore.go:344`):

```go
exists, err := cs.certificateExistsByName(filename)
if err != nil {
    cs.logger.Warn("Failed to check if certificate exists", ...)
    return nil                       // the import never happens
}
```

The check exists to prevent duplicate imports on restart. Its not-found branch — the normal
first-run case — is the one that prevents the import entirely.

### Observed

Controller log from a real block, directory mounted and the file present:

```
WARN certstore.go:344 msg="Failed to check if certificate exists"
     filename=default-listener.crt error="configuration not found"
INFO certstore.go:125 msg="Certificate trust store initialized" custom_certs=0 total_bytes=182140
```

`custom_certs=0` with a valid certificate sitting in the mounted directory. `GET /certificates`
returns `{"certificates":null,"totalCount":0,...}`.

Ruled out before concluding: the mount is real (`docker inspect` shows
`.../certificates -> /app/certificates rw=true`), the file is inside the container
(`/app/certificates/default-listener.crt`, 2122 bytes), WORKDIR is `/app` so the configured
`./certificates` resolves to it, and the PEM parses (`openssl x509` reads the subject).

### The fix

One branch, in `certificateExistsByName`:

```go
if errors.Is(err, storage.ErrNotFound) {
    return false, nil
}
```

### Why nothing caught this

Certificate management is database-backed, so all 13 migrated scenarios — upload, list, delete,
reload, and the error cases — pass whether or not the seed directory is even mounted. This
framework did not mount it, so `bootstrapCertificatesFromFilesystem` returned at its first
`os.Stat` and the path never ran. The legacy compose DID mount it
(`../gateway-controller/certificates:/app/certificates`) and so ran the code every time — and
still asserted nothing about it, which is why the defect survived there too.

The mount is now restored, matching the legacy and product compose files, and a fourteenth
scenario asserts the seeded certificate reaches the store. It is tagged `@parked-finding-6` and
excluded from the `controller` runner, so the suite stays green while the assertion remains on
record. It goes green on its own the day the branch above is added.

**Do not "fix" this by deleting the scenario or by seeding the certificate over the API
instead.** Both would make the suite pass while retiring the only check that the filesystem
seeding path does anything at all.

---

## Finding 7 — `PUT /llm-proxies/{id}` returns 500 for a malformed body, and 400 is unreachable

`management-openapi.yaml` declares `200, 400, 404, 500` for this operation, and the POST sibling
returns 400 for the same malformed body. The PUT returns **500** — observed, not inferred:

```
expected status 400, got PUT /api/management/v1/llm-proxies/invalid-update-proxy -> 500,
body={"message":"Failed to update LLM proxy configuration","status":"error"}
```

The provider and proxy creations ahead of it both returned 201, so the proxy existed and the
malformed body genuinely reached the parser.

The cause is a shadowed error wrap. `UpdateLLMProxy` calls `isLLMProxyUndeployRequest` to read
the deployment state before it reaches `DeployLLMProxyConfiguration`, and that helper returns the
parse error unwrapped:

```go
// pkg/utils/llm_deployment.go:1307
func (s *LLMDeploymentService) isLLMProxyUndeployRequest(params LLMDeploymentParams) (bool, error) {
	var proxyConfig api.LLMProxyConfiguration
	if err := s.parser.Parse(params.Data, params.ContentType, &proxyConfig); err != nil {
		return false, fmt.Errorf("failed to parse proxy configuration: %w", err)
	}
```

With no `ErrLLMProxyValidation` in the chain, none of the handler's branches match and it falls
through to the terminal 500 at `pkg/api/handlers/llm_proxy_handler.go:222`. The correct wrapping
exists at `llm_deployment.go:443` but is only reachable on the POST path — which is exactly why
`Create LLM proxy with invalid JSON body returns error` passes while the update does not.

### How it was found, and what the old scenario was actually testing

The scenario `Update LLM proxy with invalid JSON body returns error` sent its malformed body to
`some-proxy`, a handle that is never created. Existence is checked before the body is parsed, so
the request died at the not-found check and the malformed body never reached a parser: the
scenario was a duplicate of `Update non-existent LLM proxy returns 404`, wearing a title about
invalid JSON. It asserted `the response should be a client error`, which is satisfied by 400 and
404 alike, so nothing ever pointed at the mismatch.

Tightening that assertion to an exact status is what exposed it. The scenario now creates a
provider and a proxy first, so the body reaches the parser, and asserts the declared 400. It is
tagged `@parked-finding-7` and excluded from the `llm-proxies` runner.

Two sibling endpoints share the shape and are **not** yet covered — their scenarios still target
non-existent handles:

- `PUT /llm-provider-templates/{id}` wraps parse errors correctly, so it should return 400 once
  its scenario targets an existing template.
- `PUT /mcp-proxies/{id}` maps *every* non-render error to 404 with no `IsNotFoundError` test, no
  validation branch and no 500 path (`pkg/api/handlers/mcp_proxy_handler.go:261`), so its declared
  400 and 500 are unreachable and a database outage would report "not found". Worth filing on its
  own.

---

## Finding 9 — `PUT /mcp-proxies/{id}` answers 404 for everything, and update had no live coverage

`pkg/api/handlers/mcp_proxy_handler.go:261` collapses every non-render error into 404:

```go
if err != nil {
    if mapRenderError(w, "update", err) {
        return
    }
    log.Warn("MCP proxy configuration not found", slog.String("handle", handle))
    httputil.WriteJSON(w, http.StatusNotFound, api.ErrorResponse{
        Status:  "error",
        Message: fmt.Sprintf("MCP configuration with handle '%s' not found", handle),
    })
    return
}
```

No `IsNotFoundError` test, no validation branch, no 500 path. Observed: an unparseable body against
an **existing** proxy returns 404, not the declared 400. So the declared `400` and `500` are both
unreachable on this operation, and a genuine database outage would report "not found" to the
caller. `CreateMCPProxy` classifies correctly by contrast (`mcp_proxy_handler.go:95`). Tagged
`@parked-finding-9`.

### The coverage gap that hid it

Nothing live ever called `PUT /mcp-proxies/{id}`. All three existing update scenarios were
excluded — `mcp_deploy` "Deploy a sample MCP Server and do a tools/call" (200) and `mcp_policies`
"mcp-acl-list policy modes" both legitimately behind `@needs-mcp-client`, and `mcp_deploy` "Update
non-existent MCP proxy returns 404" behind the same tag **despite binding no MCP-client step at
all**. A CRUD verb with zero live assertions is why an unconditional 404 went unnoticed.

`Update an existing MCP proxy with a valid body` now covers the happy path and needs no MCP client:
it creates a proxy, PUTs a changed `displayName`, asserts 200, and reads the change back over the
management API so a no-op cannot pass. It is green — the success path is fine, and this finding is
strictly about error classification.

Do **not** "fix" the coverage by un-tagging "Update non-existent MCP proxy returns 404". While
every error maps to 404 that scenario cannot fail — it would pass whether or not the proxy
exists — so it adds no signal until this finding is resolved. Un-tag it then.

---

## Finding 10 — DOCUMENTATION GAP: the contract does not say which form each read returns

An API deployed with a `$version` placeholder is reported differently by the two endpoints that
return it:

| Read | `spec.context` |
|---|---|
| `GET /rest-apis` -> `apis[0].spec.context` | `/api/v2.0` — resolved |
| `GET /rest-apis/versioned-context-api` -> `spec.context` | `/api/$version` — raw placeholder |

Both documents captured in the same run, for the same record — `status.createdAt` is identical to the
nanosecond (`2026-08-19T14:42:18.450511089Z`), as are `metadata.name`, `spec.displayName`,
`spec.version`, `spec.operations`, `spec.upstream`, `spec.vhosts` and every `status` field.
`spec.context` is the ONLY field that differs.

The data plane serves the resolved path (`GET /api/v2.0/data` returns 200), so routing is correct.
Only the management representation is inconsistent.

`management-openapi.yaml` documents `$version` on the REQUEST schema only — *"Use $version to embed
the version in the path (e.g. /reading-list/$version resolves to /reading-list/v1.0)"* — and says
nothing about which form a read returns. So neither endpoint contradicts the contract on its own;
the defect is that they contradict each other. A client that fetches an API by id cannot learn the
path the gateway actually serves without re-implementing the substitution.

This is filed as a DOCUMENTATION gap, not a defect: the split is understood to be intended — the
collection view renders the effective path, the single-resource view returns the record as
authored. What is missing is any statement of that in the contract. A client reading by id has no
documented way to know the served path differs from `spec.context`, and nothing stops a future
change from "fixing" either endpoint to match the other.

**Ask of the product:** document, per operation, whether `spec.context` is returned resolved or as
authored.

### How it was found

`api_management`'s "Create API with context containing version placeholder" asserted against the
listing, migrated verbatim from the legacy suite. Rewriting it to fetch the API by id — a
deterministic single document instead of a shared listing whose element order is not stable — was
expected to be a pure improvement. It failed: `expected "/api/v2.0", got "/api/$version"`.

The paired `should not contain "/api/$version"` is what identifies the original intent: it claims
the raw template appears NOWHERE in the listing, which is a whole-body assertion no field path can
express, and it passes. The scenario was testing the collection view specifically.

The scenario now pins BOTH reads — whole-body assertions on the listing, and
`spec.context should be "/api/$version"` on the single-resource read. **Do not "tidy" either into
the other.** An exact whole-body assertion is not an option on either response: `status.createdAt`,
`updatedAt` and `deployedAt` are per-run wall-clock values, and the listing's contents additionally
depend on what else is deployed concurrently.

---

## Watch item — RESOLVED: the full suite dies because lima's SSH tunnel collapses, not Docker

This closes the question the "daemon death" watch item above could not answer. The daemon was
never the problem; the retraction recorded there was correct but incomplete. Here is the cause.

### Measured, with the VM still running

Caught live at 18:22 on a full-suite run, before restarting anything — which is what made the
diagnosis possible, and why the instruction not to restart on a stall is load-bearing.

| Check | Result |
|---|---|
| `systemctl is-active docker` (in VM) | **active** |
| `docker ps` (in VM) | works |
| `docker ps` (host, via forwarded socket) | `Cannot connect to the Docker daemon` |
| docker.service transitions since 18:00 | **0** |
| VM reboot | none (booted 17:14:20) |
| kernel OOM kills | **0** |

The daemon ran continuously and healthily. Only the host lost access.

### The chain

Lima forwards every published container port over SSH, spawning a process per operation against
one shared ControlMaster (`ControlPath=~/.colima/_lima/colima/ssh.sock`). During the run:

    distinct ports forwarded : 211
    ssh forward/cancel calls : 1077

Bursts of those failed, from 18:05 onward and escalating (9, 9, 27, 8, 10, 11, 16, 8 per minute):

    ssh ... -O forward -L 0.0.0.0:32922 ... : exit status 255
    level=warning msg="failed to set up forwarding tcp port 32922 (negligible if already forwarded)"

The guest's sshd is configured `maxstartups 10:30:100` — random drops above 10 concurrent
unauthenticated connections, hard refusal at 100 — and `maxsessions 10`. Lima's bursts exceed
that, connections are dropped, and eventually the shared ControlMaster goes with them.
`docker.sock` is forwarded over that same master, so the host loses Docker.

### Why every earlier hypothesis missed

The client error says "Is the docker daemon running?", which is a lie about someone else's
process, and it is the only signal the test log carries. Every measurement aimed at the VM came
back clean because the VM was fine: memory peaked at 7%, load 0.22, dockerd RSS ~116MB, no OOM.
Restarting colima always "fixed" it — because that rebuilds the SSH tunnel.

It also explains the shape of the failure. Ports scale with blocks, so only full-suite runs
reach the burst size; a single block never does. And product-apim does not hit it because its
blocks run roughly one container each, against our three-to-five.

Lima's own warning — "negligible if already forwarded" — is actively misleading. They were not
already forwarded, and it was not negligible.

### The gRPC forwarder was TRIED and is WORSE. Do not repeat it.

`colima start --port-forwarder grpc` (Colima >= 0.9.0) looked like the structural answer: Lima's
gRPC forwarder spawns no child process, so it cannot exhaust sshd connections. Measured, it does
remove the storm completely — one block went from ~24 ssh invocations to **0**, and passed 11/11.

Then it fails differently and far more deceptively. On a full run it carried traffic for the
first ~805 scenarios and then **silently stopped forwarding every port at once**:

    host   : curl localhost:32840/... -> timeout          (16 of 16 ports, both live stacks)
    in VM  : curl localhost:32840/... -> 200 OK
    docker : socket perfectly healthy throughout

And in Lima's host-agent log, for the port that failed:

    19:24:50 {"level":"info","msg":"Forwarding TCP from 0.0.0.0:32781 to 0.0.0.0:32781"}
    - zero "Stopping forwarding" events
    - zero error or warning lines
    - the agent went on forwarding OTHER ports minutes later

So the forward was established, never torn down, never errored, and carried nothing. There is no
timeout to raise: nothing timed out. The bookkeeping says healthy while the tunnel is dead.

This is a known Colima bug, and the maintainer's own remedy is to go back to SSH —
abiosoft/colima#1376: *"This is likely related to switching to the new grpc port forwarder ...
a revert to the SSH forwarder may need to be done"*, later *"the previous SSH port-forwarder
should be reinstated."* A reporter on Darwin 24.6.0 — the same OS build as this machine —
described exactly this symptom and confirmed SSH cured it. The `--port-forwarder` flag exists as
the escape hatch from this bug, so choosing `grpc` opts INTO it.

Worse than the SSH failure in one specific way: the SSH collapse announces itself
(`Cannot connect to the Docker daemon`), while this one presents as the product being unhealthy.
Without checking in-VM versus host, it reads as a gateway regression.

### Options, in order of how structural they are

1. **Take port forwarding off the path entirely.** `--network-address` **plus**
   `--port-forwarder=none`, then `TESTCONTAINERS_HOST_OVERRIDE=<VM IP>`. Both flags are needed:
   Colima always installs full-range `[1,65535]` TCP+UDP forwards unless the forwarder is
   `none`, so `--network-address` on its own leaves the storm and the flaky forwarder exactly
   where they were. Needs no framework code change — `framework/core/runtime/compose.go` already resolves
   addresses through testcontainers' `Host()`/`MappedPort()`, which honour the override.
   `docker.sock` survives `none`: Colima registers it as a Lima `guestSocket` forward before the
   port-forwarder branch, Lima's hostagent sets socket forwards up unconditionally, and Lima's
   dynamic matcher explicitly skips socket rules (`if rule.GuestSocket != "" { continue }`), so
   the ignore rule can never apply to it. COSTS: a VM recreate (every local image is lost) and
   it CANNOT be disabled once enabled. Read the VM address from `colima ls -j` at runtime and
   never hardcode it: with `vmType: vz` on the default shared network, Colima uses Apple's
   vzNAT (`yaml.go`: `if l.VMType == VZ && conf.Network.Mode != "bridged"` -> `VZNAT: true`) and
   macOS assigns the address. The `192.168.106.x` range belongs to the rootful `socket_vmnet`
   daemon, which is used only for bridged mode or non-vz VM types — so the sudoers requirement
   and the corporate-VPN subnet clash that come with it do NOT apply to this configuration.
2. **Stay on the default `ssh` forwarder and cap the run size.** Focused blocks are reliable —
   configdump 11/11, certificates 13/13, template-functions 6/6 all passed on it. Note this has
   NOT been retested since the shutdown-drain fix landed, and that fix changed teardown timing
   substantially, which is precisely when the forwarder was failing. Retesting SSH costs a
   restart and no images; it should be tried before anything expensive.
3. **Publish fewer ports.** Real but small: 8 per gateway stack, and all 8 are addressed by
   steps — the two metrics ports are scraped by `scrapeMetrics`, so none is dead weight.
4. **Lower block parallelism.** Mitigation at best, and probably not even that: one block's own
   teardown is ~12 ports x 2 address families = ~24 simultaneous ssh spawns, already over the
   limit without any help from a second block.

**Not an option: `grpc`.** See above.

Note for whoever picks this up: this is an environment defect, not a product one, and not a
framework one either — nothing in the suite is misbehaving. But it caps how much of the suite
can run in one go on this setup, so it belongs here rather than in a backlog.

---

## MEASURED: the concurrency ceiling is in Colima's tunnel, not the host

Two full-suite runs, identical in every respect except `-block-parallel`. This is the first hard
number on where the limit sits, and it settles what the earlier sections could only speculate about.

|                       | `parallel: 2`     | `parallel: 6`        |
|-----------------------|-------------------|----------------------|
| **Completed**         | **yes — 33 min**  | **no — died ~7 min** |
| Scenarios executed    | 1275              | 796, then noise      |
| Blocks                | 15 passed, 1 failed | —                  |
| `not answering health`| **0**             | **13**               |
| `Cannot connect to Docker` | 0            | 0                    |
| ssh forward invocations | 0               | 0                    |
| Peak containers       | 9                 | 19                   |
| Peak memory           | 1249 MB (10%)     | 1809 MB (15%)        |
| Peak load             | 0.58 (10% of 6)   | 0.66 (11% of 6) [1]  |

### Two things this proves

**1. Poll frequency was the dominant factor, and the 2s floor fixed it.** Before that floor the
suite died at ~7 minutes on the ssh forwarder and again at ~7 on grpc. With it, the SAME Colima,
the SAME grpc forwarder and the SAME VM ran 1275 scenarios to completion. Nothing else changed
between those runs. See the BaseInterval section in framework/core/util/retry — 250ms polls across ~8
concurrent runners issued roughly 32 connections/second through the tunnel; at 2s it is ~4.

**2. The ceiling is the tunnel, and the host is nowhere near it.** At the moment of total collapse
under `parallel: 6`:

    ports reachable from the host : 0
    ports UNREACHABLE            : 22   (every one)
    the same port, inside the VM : 200 in 0.005s
    container                    : Up 7 minutes, healthy
    VM load                      : 0.81 of 6 cpus
    VM memory                    : 1723 MB used, 10211 MB available

15% memory and 62% peak CPU, with a container answering in five milliseconds internally while the
host timed out for eight seconds. This is not resource exhaustion. Do not go looking for one again
— that mistake cost several sessions (see the retracted daemon-death section above).

### The failure has no graceful degradation

22 of 22 ports died together, and 33477 was still dead three minutes later. It does not shed load,
drop the newest ports, or recover. It stops carrying traffic entirely and stays stopped, while its
own bookkeeping continues to report every forward as established. That is what makes it dangerous:
it presents as the product being unhealthy.

### What follows

`parallel: 2` is the safe setting on this host and produces a complete run. `parallel: 6` does not.
The limit is a RATE, not a container count — see the replication below.

The throughput argument for removing the forwarder is now quantified rather than asserted: 62% CPU
at 19 containers suggests the machine would host 25-30, but the tunnel gives out at 19. Roughly a
third of the available capacity is unreachable — not because the host cannot take it, but because
every request squeezes through a tunnel that fails silently and totally.
`--network-address` + `--port-forwarder=none` + `TESTCONTAINERS_HOST_OVERRIDE` removes it from the
path (see the options list above; docker.sock survives `none`, source-verified).

### Scenario failures from the completing run, for separate triage

4 real failures, ALL in `gateway-core/postgres`; the sqlite variant of every one passed.

    ratelimits         X-Ratelimit-Remaining absent after a template change
    schema-guardrails  regex-unicode route never served
    guardrails         url-assessment and sentence-count-max routes never served

Three are "waiting for /health to return 200: the condition never held" — and they now poll the
full 180s ceiling, so this is NOT a short budget. On postgres, under concurrent load, those routes
genuinely never propagate. Engine-specific propagation is exactly what a db matrix exists to
surface, which is an argument for widening it (see the sqlserver gap below).

### Coverage gap: sqlserver is not exercised at all

CI runs the gateway IT on all three engines — `gateway-integration-test.yml` (sqlite, plus
vhosts-single/multi), `-postgres.yml`, and `-sqlserver.yml`, all `make test-integration`. This suite
runs everything on sqlite, only `gateway-core` on postgres, and sqlserver NOWHERE (zero mentions in
it-suite.yaml), despite `components.SQLServer`, the sqlserver DDL, `sqlServerDSN` and
`engine_sqlserver.go` all existing and the image being available.

Note sqlite is the engine LEAST like production: it self-migrates, while postgres and sqlserver
expect a pre-provisioned schema, which is the path operators actually use. So the engine exercised
15 ways is the atypical one, and one of the two production paths is untested. Adding sqlserver to
the `gateway-core` matrix is a one-line change; deciding whether the whole suite should sweep
engines (as CI does) is a bigger, separate call.


### REPLICATED, with two corrections to the numbers above

The `parallel: 6` run above was measured on a VM that ALSO had four product-apim containers
(`wso2am`, `node-app-server`, a Java ryuk) on the same tunnel. I did not check the VM was quiet
before starting it and only found them afterwards. It was re-run on a verified-empty VM.

    parallel 2                    : no collapse, completed in 33 min
    parallel 6, contaminated VM   : collapsed at ~7 min
    parallel 6, clean VM          : collapsed at ~16 min

**[1] The 3.73 peak load was not ours.** On the clean VM the identical workload — same 19
containers, same block concurrency — peaked at **0.66**, about 11% of six cores. The 3.73 was
product-apim's JVMs. The table above is corrected; treat any earlier statement that this suite
uses 62% CPU as wrong.

**The ceiling is real, but it is not a container count.** Contamination only changed WHEN it broke,
from 16 minutes to 7. Removing it did not prevent the collapse. Time-to-failure tracks the RATE of
traffic through the tunnel, which is what the documented lima mechanisms predict — a leaked FD per
forwarded connection (#5210) and a 2048-stream ceiling (#5042) both accumulate. A fixed
concurrent-capacity limit would have produced the same failure point regardless of what else shared
the VM; it did not.

Collapse signature on the clean VM, identical to before:

    host  -> 000 after 6s        in-VM -> 200 in 0.003s
    ports: 0 reachable, 27 UNREACHABLE
    VM   : load 0.15, 10621 MB free

**The uncomfortable implication: `parallel: 2` is probably not immune, only slower to arrive.** It
finished 1275 scenarios in 33 minutes without collapsing, but nothing observed here says it has a
different ceiling — only that it approaches it more slowly. A longer suite, a third database in the
matrix, or a slower machine could cross the same line. Tuning `parallel` buys time; it does not
remove the mechanism. That is the argument for taking the forwarder out of the path
(`--network-address` + `--port-forwarder=none`), which is NOT yet proven — see the note on staged
verification: test `--port-forwarder=none` alone first, since that is a reversible restart that
answers whether docker.sock survives, before paying for the irreversible `--network-address`
recreate.

### Scenario failures, corrected count

The completing `parallel: 2` run had **4** scenario failures out of 1275, not the 8 stated earlier.
That 8 came from a grep matching "N failed" on both the scenario summary AND the step summary of
each runner, double-counting every failure. Authoritative:

    ratelimits         68 scenarios (67 passed, 1 failed)
    schema-guardrails  51 scenarios (50 passed, 1 failed)
    guardrails         55 scenarios (53 passed, 2 failed)

---

## PROVEN: the forwarder can be removed entirely, and it costs nothing

Everything in the options list above was source-reasoning. It has now been RUN, and three of the
risks I attached to it were wrong. Configuration:

    colima start --port-forwarder=none --network-address
    export TESTCONTAINERS_HOST_OVERRIDE=$(colima ls -j | jq -r '.address')

| check | result |
|---|---|
| `docker.sock` survives `--port-forwarder=none` | YES — the blocking unknown, now settled |
| images lost | **NO — 22 before, 22 after** |
| VM recreated | **NO — a plain restart** |
| VM address | `192.168.64.2` (vzNAT) |
| forwarded ports | correctly dead (`localhost` -> 000) |
| container reachable at the VM IP | **YES — 200** |
| focused block through the VM IP | **11/11 scenarios, 85/85 steps** |

### Corrections to what this document previously claimed

- **"`--network-address` requires a VM recreate, every local image is lost."** Wrong. It is a
  restart; nothing was lost. That claim came from generalising the 0.8.1 -> 0.10.3 UPGRADE, which
  did destroy the images — but that was a Lima instance-format change, not a config change.
- **"`192.168.106.x` hardcoded, needs a socket_vmnet sudoers entry, may clash with a corporate
  VPN."** Wrong for this configuration. With `vmType: vz` on the default shared network Colima
  uses Apple's vzNAT and macOS assigns the address — 192.168.64.2 here. The socket_vmnet range
  applies only to bridged mode or non-vz VM types. No sudoers prompt appeared.
- The only genuine one-way door is that `--network-address` cannot be turned off again. That is a
  config property, not lost work.

### Why this matters more than "parallel 2 works"

BOTH forwarders were measured failing, and the second one is the reason tuning concurrency is not
a fix:

    grpc, parallel 6, contaminated VM : collapsed at ~7 min
    grpc, parallel 6, clean VM        : collapsed at ~16 min
    grpc, after a restart             : NEVER worked — one container, idle VM, ports refused in 0.2ms
    ssh, ONE block (gateway-core/postgres) : 588 forward invocations, 77 exit-255, docker.sock lost

That last line is the important one. The ssh forwarder lost the Docker socket during a SINGLE
block, with the 2s poll floor already in place. `parallel: 2` completing earlier was not a safe
operating point; it was a run that finished before the tunnel gave out.

With the forwarder gone there is nothing to exhaust: no per-port ssh processes, no gRPC tunnel, no
accumulating stream or FD leak. Container traffic goes host -> VM IP -> container directly, and
only `docker.sock` still rides SSH — one long-lived connection rather than one per published port.

### What still needs doing

The full suite has NOT been run in this configuration, at any concurrency. The obvious next
measurements are `parallel: 2` (to compare against the 33-minute baseline) and then `parallel: 6`,
which previously collapsed twice. The host has the headroom — 19 containers cost 11% CPU and 13%
memory — and the tunnel is no longer in the way.

Note for CI: none of this applies on Linux. Docker is native there, `localhost:<port>` is a kernel
DNAT rule, and no forwarder exists to fail.
