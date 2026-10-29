# Feature Specification: Model Failover Policy

**Feature Branch**: `001-model-failover-policy`

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "I want to design plan and implement the feature https://github.com/wso2/api-platform/issues/3356"

**Source**: [wso2/api-platform#3356 — \[Feature\]: Model Failover Policy](https://github.com/wso2/api-platform/issues/3356)


> **Reconciliation with the original design document** (composite clusters, probe recovery, per-provider upstream policies): see [`design-doc-reconciliation.md`](design-doc-reconciliation.md). It records what was carried in, what this spec supersedes, and three open conflicts (C1–C3) that still need a decision.


> **Agent Router comparison** (source-verified): see [`agent-router-comparison.md`](agent-router-comparison.md) — three adopted changes to filter placement, response phase and baseline state, and why their core failover mechanism is not adopted.

## Clarifications

### Session 2026-10-22

- Q: Should model failover be a capability built into each existing routing policy, or one separate policy that reads the routing policy's decision and owns the retry and health logic? → A: A separate `model-failover` policy. It works standalone with its own ordered target list, and when a routing policy is present it consumes that policy's target selection through the existing shared-metadata contract. Existing routing policies are not modified and need no re-release.
- Q: When a routing policy picked the first target and it fails, where does the next target come from? → A: Routing policies publish an **ordered preference list** of targets rather than a single winner, and failover traverses that list in order. The routing algorithm is consulted once per request, never re-run per attempt. A routing policy that still publishes only a single selection degrades gracefully to a one-element list, after which failover continues with its own configured chain.
- Q: Which routing policies should publish the ordered preference list as part of this feature? → A: `model-round-robin` and `model-weighted-round-robin` adopt the contract at launch. `llm-header-router` and `typesafe-jev-model-routing` (vendor-owned) keep publishing a single selection and compose through the graceful-degradation path until they adopt it later.
- Q: When failover is attached to the same operation as a routing policy that has its own `suspendDuration`, which health view governs? → A: The routing policy defers — it ranks over the healthy target set failover publishes, and its own `suspendDuration` becomes inert for that operation. The override is surfaced at deployment time and in telemetry rather than applied silently. `suspendDuration` keeps working unchanged when the routing policy is used without failover. **[SUPERSEDED 2026-10-29 — implemented, then reverted. Neither half survived: failover publishes no health state (FR-026 withdrawn), and the routing policy's `suspendDuration` is not overridden (FR-026a/FR-026b withdrawn). Both policies keep their own verdict and both are correct, because a routing policy only ever observes the final status of a request. See US6 acceptance scenarios 3-5 and 7.]**
- Q: Does the failover policy still need its own target list once routing policies publish one, and how do the two combine? → A: Keep one `targets` list, but make it **optional when an adopting routing policy is attached** and required otherwise. When both exist, the routing policy's published list is traversed first and failover's own targets that are not already in it follow as a last-resort tail, deduplicated. Omitting `targets` with an adopting routing policy present means the published rotation is the whole chain.
  **Superseded 2026-10-29 by FR-021/FR-021a**: configured fallbacks now take precedence over the remainder of the published list, and the chain is selected by the target actually used rather than by the requested model.
- Q: If same-request retry requires changes to the gateway's core routing configuration rather than living entirely inside the new policy, is that in scope? → A: Yes. The feature may add the missing re-dispatch capability at the gateway routing layer, recorded in the plan's Complexity Tracking table as a justified deviation from the policy-first principle. Dropping FR-008 and shipping next-request failover only is explicitly rejected — it would duplicate what the existing routing policies already do.
- Q: When failover re-attempts a request against the next target, must that attempt carry request-body changes made by other policies (PII masking, guardrails, prompt decoration) that ran before the first attempt? → A: Yes. Every attempt MUST carry all request-body mutations applied by preceding policies; equivalence of the mutated body across attempts is a required and explicitly tested property, and an attempt that cannot preserve them MUST fail rather than send the unmutated body.
- Q: Should failover re-run the policy chain for each attempt (via proxy-level re-entry) so the routing policy's full ranking is honored, with side-effecting policies executing once per client request? → A: Yes. Each attempt re-enters the chain, which honors an arbitrary per-request ranking and causes body mutations to be re-applied rather than replayed. Policies with external side effects (quota consumption, token/cost accounting, analytics, guardrail screening) execute once per client request; per-target policies (credential injection, request translation, target steering) execute per attempt. Each policy's class is declared, never inferred.
- Q: Should a target also be suspended when its failures cross a rate threshold inside a time window, in addition to the existing consecutive-failure rule? (ECI requirement 3, wso2-apim-internal#18469) → A: Yes — both criteria, either can suspend. Keep consecutive-failure counting and add a windowed rate rule with a minimum sample size, so a target failing intermittently (never producing a run of failures) is still detected.
- Q: Should latency-based suspension use an aggregate measure rather than one slow request triggering failover? (ECI requirement 4, wso2-apim-internal#18469) → A: Yes. `perAttemptTimeoutMs` continues to bound an individual request, but suspension on latency requires a percentile over the rolling window to cross a threshold, reusing the same window and minimum-sample-size machinery as the error-rate rule. A single slow request fails over; it does not condemn the target.
- Q: How should a suspended target return to service, and what brings an escalated suspension back to its base duration? → A: No probe traffic and no probing state. When the suspension expires the target returns to **full normal rotation**. If it then re-trips any suspension criterion it is suspended again for **double** the previous duration, capped at a configured maximum. The duration resets to base only after the target has stayed healthy for a configured period (`backoffResetSeconds`); a target that keeps flapping never earns the reset and keeps doubling toward the cap.
- Q: Which API kinds should be allowed to attach the model-failover policy, and must other kinds stay unaffected? → A: Any kind whose operations can resolve model targets — `RestApi` and `LlmProxy` today, and any future AI-capable kind — matching how `model-round-robin` already behaves. Attaching it where targets cannot resolve is rejected at deployment. Separately, generated configuration MUST be scoped to the operation that attaches the policy: an API of any kind that does not attach it, including one routing to the same shared provider upstream, MUST be byte-for-byte unaffected.
- Q: When the policy is attached where it cannot work, should that be rejected at deployment, or silently do nothing? → A: Fully silent no-op. Deployment succeeds, the policy is inert at runtime, and nothing is logged, counted, or surfaced. This matches every other policy in the gateway — the controller rejects no deployment today for an unresolvable policy reference — and keeps a GitOps deployment from being blocked by an environment mismatch. Schema-level errors (numeric values out of range) are still rejected, since those are malformed input rather than an environment mismatch.
- Q: Which flow should the failover policy attach to — request or response? → A: Both, for different jobs. The **request** flow is where failover happens: it resolves the chain once and publishes it, and without it there is no failover at all. The **response** flow does nothing for the current request — by then the attempt sequence is over — and exists only to grade target health for *future* requests. Because the response flow runs once and sees only the final response, it must use the upstream attempt count to attribute that outcome to the target that actually produced it.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Automatic fallback to a healthy model (Priority: P1)

An API publisher exposes an AI proxy API to internal applications. They configure an ordered list of
targets — a primary model and one or more fallbacks — on the operation. When the primary model
answers with a failure (an overload/rate-limit response, a server-side error, or a connection
problem), the gateway transparently retries the same client request against the next target in the
list and returns that target's successful answer to the calling application. The application sees a
normal successful response and never writes fallback logic of its own.

**Why this priority**: This is the whole point of the feature. Without it, every consuming
application must implement its own catch-and-retry logic. Shipping only this story already removes
that burden and keeps applications up when a single model degrades.

**Independent Test**: Configure an operation with a primary target that is forced to fail and one
healthy fallback target, send a single request, and confirm the client receives a successful
response served by the fallback target.

**Acceptance Scenarios**:

1. **Given** an operation configured with primary target A and fallback target B, **When** a request
   is sent and target A answers with a configured failure response, **Then** the gateway attempts
   target B and returns B's successful response to the client.
2. **Given** the same configuration, **When** target A answers successfully, **Then** the gateway
   returns A's response and never contacts target B.
3. **Given** an operation configured with targets A, B and C in that order, **When** A and B both
   fail, **Then** the gateway attempts C and returns its successful response.
4. **Given** an operation configured with targets A and B, **When** the gateway cannot establish a
   connection to A, A resets the connection, or A does not answer within the configured per-attempt
   time limit, **Then** the gateway treats that as a failure and attempts B.
5. **Given** an operation with failover configured, **When** a target returns a failure that is NOT
   in the configured failure conditions (for example a client-side validation error), **Then** the
   gateway returns that response to the client unchanged and does not fail over.

---

### User Story 2 - Same-model failover across provider configurations (Priority: P1)

An API publisher runs the same model in two regions through two separate provider configurations
(for example a primary region and a secondary region). They list both as targets so that a regional
outage or regional rate limit moves traffic to the other region without any change to the calling
application, and without the two regions sharing a health verdict.

**Why this priority**: Regional capacity problems and regional rate limits are the most common
real-world trigger for this feature, and the same-model case is the simplest correct behavior to
demonstrate. It is P1 alongside Story 1 because the ordered-fallback mechanism is of limited use if
targets can only be distinguished by model name.

**Independent Test**: Configure two targets that name the same model but different provider
configurations, force the first to fail, and confirm the request is served by the second with the
provider-appropriate credentials applied.

**Acceptance Scenarios**:

1. **Given** two targets naming the same model under different provider configurations, **When** the
   first fails, **Then** the gateway attempts the second using that provider configuration's own
   credentials and endpoint.
2. **Given** that configuration, **When** the first provider configuration is marked unhealthy,
   **Then** the second provider configuration's health is unaffected and it continues serving.

---

### User Story 3 - Cross-provider failover with automatic request adaptation (Priority: P2)

An API publisher wants a model from a different vendor as a last-resort fallback. They add it as a
target; the gateway applies that vendor's authentication and translates the request and response to
and from that vendor's format, so the calling application keeps using one request/response shape
regardless of which vendor ultimately served the call.

**Why this priority**: Highest-value resilience (surviving a whole-vendor outage) but it depends on
the ordered-fallback mechanism from Story 1 and on cross-vendor request adaptation, so it follows
the same-vendor cases.

**Independent Test**: Configure a primary target from one vendor and a fallback target from a
different vendor, force the primary to fail, and confirm the client receives a successful response
in the expected shape without changing its request.

**Acceptance Scenarios**:

1. **Given** a primary target on vendor X and a fallback target on vendor Y, **When** the primary
   fails, **Then** the gateway sends the request to vendor Y with vendor Y's authentication and
   request format and returns a response in the shape the client expects.
2. **Given** that configuration, **When** vendor Y serves the request, **Then** usage and cost
   attribution recorded for the call reflect vendor Y and its model, not the primary.
3. **Given** a routing policy spreading load across in-house models and a vendor Y model configured
   only on the failover policy, **When** the in-house models are healthy, **Then** vendor Y receives
   no traffic; **When** every in-house model has failed or is suspended, **Then** vendor Y serves the
   request.

---

### User Story 4 - Suspending and recovering unhealthy targets (Priority: P2)

An API publisher does not want every request paying the latency cost of trying a target that is
known to be down. After a configurable number of consecutive failures, the gateway suspends that
target for a configurable period and skips it when selecting targets. When the suspension period
ends, the target returns to normal rotation. If it fails again it is suspended for double the
previous period, up to a configured ceiling, so a persistently broken target is retried ever less
often while a recovered one is back in service immediately.

**Why this priority**: Without suspension, a sustained outage adds a failed attempt (up to the
per-attempt time limit) to every single request. Valuable, but the feature is already useful without
it, so it sits below the core fallback behavior.

**Independent Test**: Force a target to fail the configured number of consecutive times, confirm
subsequent requests skip it immediately, then make it healthy again and confirm it returns to
rotation immediately, then confirm a second failure doubles the next suspension.

**Acceptance Scenarios**:

1. **Given** a suspension threshold of N consecutive failures, **When** a target fails N times in a
   row, **Then** it is marked suspended and subsequent requests skip it without attempting it.
2. **Given** a suspended target, **When** the suspension period has not yet elapsed, **Then** normal
   traffic is not sent to it.
3. **Given** a suspended target whose suspension period has elapsed, **When** the period ends,
   **Then** the target returns to full normal rotation with no probe phase, and is restored to
   normal rotation.
4. **Given** a restored target that re-trips any suspension criterion, **Then** it is suspended again
   for **double** the previous duration, never exceeding the configured maximum.
4a. **Given** a target already suspended at the maximum duration, **When** it fails again, **Then**
   the duration stays at the maximum and does not grow further.
4b. **Given** a restored target that stays healthy for the configured reset period, **Then** its next
   suspension starts from the base duration again.
5. **Given** a target that fails fewer than N consecutive times with a success in between, **Then**
   the consecutive-failure count resets and the target is not suspended.
6. **Given** all targets in the chain are suspended, **When** a request arrives, **Then** the
   gateway returns the failover-exhausted error rather than hanging or silently returning success.

---

### User Story 5 - Observability of failover decisions (Priority: P3)

An API publisher or platform operator investigating an incident needs to see which target actually
served each request, why a failover happened, when a target was suspended or recovered, and how
often the fallback chain was exhausted — without reading application-side logs or guessing from
latency.

**Why this priority**: Essential for operating the feature in production, but the resilience benefit
lands before the telemetry does.

**Independent Test**: Drive a request that fails over, then confirm the recorded telemetry names the
attempted target, the failure reason, and the target that ultimately served the request.

**Acceptance Scenarios**:

1. **Given** a request that fails over from A to B, **Then** telemetry records that A was attempted,
   the reason A was treated as failed, and that B served the request.
2. **Given** a target that crosses the suspension threshold, **Then** a suspension record is emitted
   naming the target, and a corresponding record is emitted when it recovers.
3. **Given** a request where every target failed, **Then** an exhaustion record is emitted naming
   each target attempted and its failure reason.
4. **Given** any of the above, **Then** no credential, secret, or end-user prompt content appears in
   the emitted telemetry.

---

### User Story 6 - Load spreading and failover together (Priority: P3)

An API publisher already spreads traffic across several models with a round-robin or weighted
round-robin policy to avoid overloading any one of them. They add failover to the same operation so
that when the model picked for a given request fails, that request still completes on another target
instead of erroring — and so that a target known to be down stops receiving its share of the
distributed load entirely.

**Why this priority**: Both halves are useful on their own and ship independently; combining them is
the mature end state rather than the first slice. It is listed last because it depends on the
failover chain (Story 1) and the health tracking (Story 4) already working.

**Independent Test**: Configure an operation with both a routing policy and failover over the same
targets, force one target to fail persistently, and confirm requests still succeed by walking the
routing policy's published order, that the failing target stops being attempted once suspended even
though the routing policy keeps nominating it, and that load spreads across the remaining targets.

**Acceptance Scenarios**:

1. **Given** an operation with a routing policy over targets A, B, C and failover configured,
   **When** the routing policy publishes the ordered list [B, C, A] for a request and B fails,
   **Then** the gateway attempts C next, then A, and returns the first successful response.
2. **Given** that same request, **When** failover traverses the list, **Then** the routing policy is
   invoked only once — no re-classification, re-scoring, or re-drawn weighting occurs per attempt.
3. **Given** that configuration, **When** target B crosses the consecutive-failure threshold, **Then**
   B is suspended and is no longer attempted — including when the routing policy nominates it first,
   in which case the request leads with the next unsuspended target in the published order.
   *(Revised 2026-10-29: originally "subsequent lists published by the routing policy exclude it".
   A routing policy cannot exclude it — failover's health is private to failover, FR-026 withdrawn.
   It keeps nominating B; B simply stops being attempted. Enforced where the chain is resolved.)*
4. **Given** a weighted routing policy, **When** one target is suspended, **Then** its published
   ordering is unchanged — weights are its own concern — and the suspended target is skipped at chain
   resolution rather than removed from the ranking, so a skip costs no attempt.
   *(Revised 2026-10-29 for the same reason as scenario 3: re-normalizing weights over "the healthy
   set" required a healthy set the routing policy can no longer see.)*
5. **Given** a suspended target whose suspension expires, **When** a later request is routed to it,
   **Then** it is admitted as a half-open probe and returns to the chain once the probe succeeds. The
   routing policy was nominating it throughout and needs no signal that it is back.
   *(Revised 2026-10-29: originally "the routing policy resumes including it in published lists".)*
6. **Given** a routing policy that publishes only a single selection, **When** that target fails,
   **Then** failover continues through its own configured chain and the request still succeeds.
7. **Given** a routing policy configured with its own suspension setting and failover attached to the
   same operation, **When** a target fails, **Then** both policies keep their own health verdict and
   neither is overridden.
   *(Revised 2026-10-29: originally required failover to supersede the routing policy's setting and
   report the override, FR-026a/FR-026b — both withdrawn. The double count they guarded against does
   not occur: the downstream chain is entered once per client request, so a routing policy observes
   only the final status. A rescued request reports success and suspends nothing; a 5xx arrives only
   on a fully exhausted chain, where the target it selected did fail. Both verdicts are correct.)*
8. **Given** that same routing policy used on an operation with no failover, **Then** its suspension
   setting behaves exactly as before.
9. **Given** that configuration, **When** every target is suspended, **Then** the client receives the
   same failover exhaustion error as a failover-only configuration.

---

### Edge Cases

- **Response already started**: If the gateway has already sent response headers or any streamed
  content to the client, a later upstream failure MUST NOT trigger failover — the client is told the
  response failed rather than receiving two concatenated or interleaved answers.
- **Failure on the last target**: When the final target in the chain fails, the client receives a
  single, consistent failover-exhausted error, not the raw upstream error body.
- **Every target suspended**: Requests return the exhaustion error immediately rather than waiting
  through per-attempt timeouts.
- **Duplicate targets**: A target that appears more than once in the chain is attempted at most once
  per request.
- **Single-target configuration**: A chain with one target behaves exactly like no failover — one
  attempt, response returned as-is — and is accepted, not rejected.
- **Slow primary**: A target that is reachable but far slower than the per-attempt limit is
  abandoned at the limit so the remaining targets still fit inside the client's overall timeout.
- **Request not replayable**: If the original request content is no longer available to re-send to
  the next target, the gateway returns a failure rather than sending a truncated or empty request.
- **Mutated body not preservable on a later attempt**: The attempt fails and the reason is recorded,
  rather than the fallback target receiving pre-masking or pre-guardrail content. This condition is
  never silent.
- **Response arrives after failover**: The outcome is attributed to the serving target, and every
  earlier target in the chain is recorded as failed — a client seeing `200` does not mean every
  target was healthy.
- **Attempt count missing from the response**: Treated as one attempt; only the first target is
  graded, and no failure is invented for targets that may never have been reached.
- **Policy attached to a kind with no model concept**: Deployment succeeds and the policy is inert;
  the operation behaves exactly as if no failover were attached, with no telemetry about it.
- **Some targets resolve, others do not**: The unresolvable ones are skipped and the chain is built
  from the rest; the operation still fails over across whatever resolved.
- **Two APIs sharing one provider upstream, one with failover**: The API without failover sees no
  change in routing, retries, or health handling — the shared upstream is not mutated on its behalf.
- **Quiet period with few requests**: The windowed rate rule stays inert below `minimumRequests`, so
  a target that fails one of its only two requests overnight is not suspended on a 100% error rate.
- **Both criteria trip together**: The target suspends once, not twice; the suspension record names
  the criterion that crossed first.
- **One very slow request**: Fails over so the client is served, counts toward the failure and
  latency samples, but does not on its own suspend the target (FR-018f).
- **Latency degrading below the timeout**: A target whose p95 triples while every request still
  completes under `perAttemptTimeoutMs` is suspended by FR-018e — the case a timeout-only rule never
  sees.
- **Window with zero requests**: The rate is undefined, not zero; the rule neither fires nor resets
  any state.
- **Request too large to retain for a second attempt**: Failover is unavailable for that request; the
  first target's response is returned as-is and the condition is recorded, rather than the request
  being rejected outright or silently truncated. The size ceiling is operator-configurable.
- **Attempt budget exhausted by the re-entry mechanism**: The bound on internal attempts MUST equal
  the configured chain length, so exhaustion produces the FR-014 error rather than a proxy-level
  failure the client cannot interpret.
- **Misconfigured target**: A target naming a model or provider configuration that does not exist is
  rejected at configuration time, not silently skipped at request time.
- **Client cancels mid-chain**: If the caller disconnects while a fallback attempt is in flight, the
  gateway stops the chain and does not keep attempting further targets.
- **Concurrent requests during suspension transitions**: Simultaneous requests observe a consistent
  view of a target's health; a single failure cannot be counted so many times that it skips the
  configured threshold, and a target cannot be suspended twice concurrently for the same failure.
- **Failure condition list is empty**: Transport-level failures still trigger failover; only
  response-status-based failover is disabled.
- **Layered chains that do not overlap**: When the routing policy's published list and the failover
  targets name different targets, the chain for the routing policy's FIRST target applies (FR-021a).
  If the publisher configured none for it, the request has no configured fallbacks and failover
  follows the remainder of the published list. This is valid configuration, not an error.
- **Failover targets omitted with an adopting routing policy present**: The published rotation is the
  whole chain; traversal and exhaustion behave exactly as if those targets had been restated.
- **Failover targets omitted with only a non-adopting routing policy present**: Rejected at
  deployment — a single published selection plus no configured targets is a one-element chain, which
  is failover in name only.
- **Routing policy later removed from an operation that omitted its targets**: The deployment is
  rejected by the same validation rather than silently degrading to a one-element chain.
- **Routing policy publishes an empty list**: Failover falls back to its own configured chain rather
  than failing the request, and the condition is recorded.
- **Routing policy publishes a suspended target**: Failover skips it without counting an attempt,
  rather than honoring the ranking into a known-dead target.
- **Target in both the published list and the configured chain**: It is attempted once, at its
  position in the published list, and not re-attempted when the chain is traversed.
- **Layered health settings conflict** *(revised 2026-10-29)*: When a routing policy declares its own
  suspension setting and failover is also configured, both keep their own verdict and neither is
  overridden. The two do not race: the downstream chain is entered once per client request, so a
  routing policy observes only the final status — a rescued request reports success and suspends
  nothing, and a 5xx arrives only on a fully exhausted chain, where the target it selected did fail.
- **Suspension during rotation** *(revised 2026-10-29)*: A target suspended by failover stays in the
  routing policy's rotation — failover's health is private to it — and is skipped at chain
  resolution instead. The skip costs no attempt, so the request simply leads with the next
  unsuspended target. Shares are not re-normalized: doing so would require the routing policy to see
  a healthy set it has no access to.
- **Detection lag across instances**: A target failing while several gateway instances serve the API
  is suspended by each instance separately, so a few additional failed attempts may occur per
  instance before all of them skip it. Each such request still fails over successfully.
- **Instance restart**: A restarted gateway instance begins with no health history and re-learns a
  target's state from live traffic; this costs at most the configured failure threshold in extra
  attempts and never returns an error to a client that has a healthy fallback available.

## Requirements *(mandatory)*

### Functional Requirements

**Configuration**

- **FR-001**: An API publisher MUST be able to attach failover behavior to an API operation that can
  resolve model targets
  and configure an ordered list of targets, where order expresses priority.
- **FR-001a**: That target list MUST be optional when the operation also has a routing policy that
  publishes an ordered preference list, and MUST be required otherwise. A publisher whose fallback
  chain is exactly the routing policy's rotation MUST NOT have to restate those targets.
- **FR-001b**: Fallbacks configured for the target a request is actually sent to MUST be attempted
  **before** the remainder of a published list (FR-021), and never nominated during normal routing.
  This is how a premium or cross-vendor model is kept out of rotation while remaining the first
  thing reached when its target fails.

  **Revised 2026-10-29.** These were previously a last-resort tail, reachable only after the whole
  published list was spent — which let a routing policy's ordering silently outrank a fallback the
  publisher had written deliberately.
- **FR-002**: Each target MUST identify a model, and MUST optionally identify the provider
  configuration that model is reached through, so the same model name can appear more than once
  under different provider configurations.
- **FR-003**: The publisher MUST be able to configure which response statuses are treated as
  failover-worthy, with rate-limit (429) and server-error (5xx) statuses available as selectable
  conditions.
- **FR-004**: The publisher MUST be able to configure the number of consecutive failures that
  suspends a target, the base suspension duration, the maximum suspension duration, and the healthy
  period that resets escalation. Each MUST have a documented default, and suspension MUST be
  disableable.
- **FR-005**: The publisher MUST be able to configure a per-attempt time limit that bounds how long a
  single target may be waited on.
- **FR-006**: Configuration MUST be rejected at deployment time, with an actionable message, only for
  **schema-level** errors — a numeric setting outside its permitted range, or a structurally invalid
  parameter. These are malformed input, not an environment mismatch.
- **FR-006a**: A target naming a model or provider configuration that does not resolve MUST NOT be
  rejected. The target is simply unusable and is skipped; if no target resolves, the policy is inert
  (FR-030b). Deployment succeeds either way.
- **FR-006b**: A failover policy with no targets and no attached routing policy publishing an ordered
  preference list MUST likewise be accepted and inert, not rejected. The gateway rejects no
  deployment today for an unresolvable policy reference, and failover MUST NOT become the first
  policy that blocks a deployment for an environment mismatch.
- **FR-007**: The failover configuration MUST be expressible in the same declarative,
  version-controllable form as the platform's other API configuration, so it can be reviewed and
  deployed through the existing GitOps flow.

**Request-time behavior**

- **FR-008**: On a failover-worthy failure from a target, the system MUST re-attempt the client's
  original request against the next non-suspended target in configured order.
- **FR-009**: The system MUST treat connection failures, connection resets, unavailable upstreams,
  and per-attempt timeouts as failover-worthy regardless of the configured response-status list.
- **FR-009a**: **Accepted limit.** Those transport failures MUST fail over, but the system is not
  required to count them toward suspension directly. The proxy answers them itself without
  invoking the policy chain, so no policy observes them. A member that fails at transport level is
  still recorded as failed whenever a LATER member returns a real response, because the attempt
  count attributes attempts 1..N-1 as failures (FR-030f–FR-030j) — so a dead member followed by a
  healthy one does reach suspension. Only a chain whose members all fail at transport level goes
  unrecorded, where suspension would have nowhere healthier to route. This limit MUST be stated in
  the policy's own user-facing description rather than left implicit. Probe F ruled out closing it
  with proxy-level outlier detection; see `poc-results.md`.
- **FR-010**: The system MUST attempt each target at most once per request, and MUST stop when the
  configured chain is exhausted — never restarting the chain or looping.
- **FR-011**: The system MUST skip suspended targets when selecting the next target and continue to
  the following one, without treating the skip as an attempt.
- **FR-012**: When a target is reached through a different provider configuration than the previous
  attempt, the system MUST apply that provider configuration's own credentials, endpoint, and
  request/response format so the client's request and the response it receives keep a single
  consistent shape.
- **FR-012a**: Every failover attempt MUST carry the request-body modifications applied by policies
  that ran before the first attempt — masking, redaction, guardrail rewrites, prompt decoration,
  templating, compression. A fallback target MUST NOT receive content that the first target was not
  allowed to receive.
- **FR-012b**: If an attempt cannot be made with those modifications preserved, the system MUST fail
  that attempt rather than send the unmodified body, and MUST record the reason. Silently falling
  back to the client's original body is prohibited — a security control that lapses without a signal
  is worse than one that fails loudly.
- **FR-012c**: Body-mutation preservation MUST be verified by explicit tests, not assumed: at minimum
  a masking policy combined with failover, asserting that the fallback target received the masked
  content.
- **FR-012d**: Policies with external side effects — quota consumption, rate-limit counting,
  token/cost accounting, analytics emission, guardrail screening — MUST execute **once per client
  request**, regardless of how many failover attempts that request makes. A client that fails over
  three times MUST be charged, rate-limited, screened, and reported once.
- **FR-012e**: Policies whose work is target-specific — credential injection, request/response
  translation, target steering — MUST execute **per attempt**, so each attempt is correctly formed
  for the target it reaches.
- **FR-012f**: Each policy's execution class under failover MUST be an explicit, declared property of
  that policy, never inferred at runtime. A policy with no declared class MUST default to
  once-per-client-request, so a policy that is later discovered to have side effects fails safe.
- **FR-012g**: Internal failover attempts MUST NOT be observable to the client as separate requests,
  separate responses, or redirects. The client sees exactly one request and one response.
- **FR-013**: The system MUST NOT attempt failover once response headers or any streamed response
  content have been sent to the client.
- **FR-014**: When all targets have been attempted or skipped without success, the system MUST
  return a single, consistent error response that identifies the condition as failover exhaustion
  and MUST NOT leak upstream error bodies, credentials, or internal addresses to the client.
- **FR-015**: The response returned to the client on a successful failover MUST be a normal
  successful response for the operation, requiring no client-side change to consume.
- **FR-016**: Usage, cost, and analytics attribution for a request MUST reflect the target that
  actually served it.

**Health tracking**

- **FR-017**: The system MUST track consecutive failures and health independently per target, where
  a target is the combination of model and provider configuration, so one region's or one model's
  problems do not suspend an unrelated target.
- **FR-018**: The system MUST suspend a target once its consecutive-failure count reaches the
  configured threshold, and MUST reset that count on a successful response from the target.
- **FR-018a**: The system MUST additionally suspend a target when its failure **rate** crosses a
  configured threshold within a configured rolling time window — `errorRatePercent` over
  `windowSeconds` — so a target that fails intermittently is detected even though it never produces
  a run of consecutive failures. Failures counted are the same failover-worthy outcomes as FR-003 and
  FR-009 (configured status codes plus transport failures and per-attempt timeouts).
- **FR-018b**: The windowed rate rule MUST NOT fire below a configured `minimumRequests` sample size
  within the window. Without it, two failures out of two requests reads as a 100% error rate and
  would suspend a healthy target during quiet periods.
- **FR-018c**: The two suspension criteria are independent and either MUST be able to suspend a
  target: consecutive failures (FR-018) catches a hard-down target quickly, the windowed rate
  (FR-018a) catches sustained partial degradation. Suspending via either path produces the same
  suspended state, the same escalating suspension duration, and the same return to full rotation
  when it expires.
- **FR-018d**: Each suspension record MUST state **which** criterion fired and its observed value —
  the consecutive count, the error rate with its sample size, or the observed latency percentile —
  so an operator can tell a hard outage from intermittent degradation from creeping slowness without
  reconstructing it from raw logs.
- **FR-018e**: The system MUST additionally suspend a target when its **latency percentile** over the
  rolling window crosses a configured threshold (`p95LatencyMs` over `windowSeconds`), subject to the
  same `minimumRequests` guard as FR-018b. Latency is judged in aggregate, never from one sample.
- **FR-018f**: `perAttemptTimeoutMs` (FR-005) MUST remain a per-request bound and MUST NOT by itself
  suspend a target. One unusually slow request fails over — that request cannot wait indefinitely —
  but condemning the target requires the aggregate in FR-018e. A per-attempt timeout still counts as
  a failure toward FR-018 and FR-018a.
- **FR-018h**: Suspension counts only outcomes the policy observes: a response the provider
  actually sent, or a failure inferred from the attempt count. Transport failures are covered by
  FR-009a and are not an additional, separately configurable suspension input.
- **FR-019**: When a target's suspension period elapses, the target MUST enter a **half-open probe
  phase** rather than returning directly to full rotation. It is admitted **one request at a time**
  until `recoverAfterSuccesses` consecutive probes succeed, at which point it returns to normal
  rotation. Any failed probe re-suspends it immediately — without waiting for `suspendAfterFailures`,
  because the probe itself was the evidence.

  Revised 2026-10-28, restoring the approved design document's behaviour. An earlier revision of this
  spec returned the target to **full** rotation on expiry, on the reasoning that escalating backoff
  made a probe phase unnecessary. That reasoning was wrong, and the two mechanisms are complementary
  rather than alternatives: backoff decides *how long* a target stays out, the probe phase decides
  *how it gets back in*. Restoring at full share exposes every request dispatched between expiry and
  the next recorded failure to a target that may still be broken — a window bounded only by
  failure-detection latency, which with `perTryTimeout` unset is the whole route timeout.
- **FR-019e**: A probe slot MUST be released by a bounded lease as well as by an outcome. An outcome
  can legitimately never arrive — the request was abandoned, or the chain never reached that member —
  and without an expiry a lost outcome would hold the one-at-a-time gate shut permanently and the
  target could never recover.
- **FR-019f**: Reading a target's health MUST NOT consume a probe slot, and a half-open target MUST
  NOT be reported as healthy. Asking is not an attempt — a half-open target's single slot belongs to
  a real request, and spending it on a query would stop the target ever recovering. (This originally
  governed publication to routing policies; FR-026 withdrew that, but the rule still binds every
  read-only health query inside the policy.)
- **FR-019a**: If a restored target re-trips **any** suspension criterion (FR-018, FR-018a, FR-018e),
  the system MUST suspend it again for **double** the previous duration.
- **FR-019b**: The doubled duration MUST be capped at a configured maximum. Once at the cap, further
  failures MUST keep the duration at the cap rather than growing it.
- **FR-019c**: The suspension duration MUST reset to the base value only after the target has stayed
  healthy for a configured reset period. A target that keeps flapping never earns the reset and keeps
  doubling toward the cap; a genuinely recovered target starts from base next time.
- **FR-019d**: Each suspension record MUST state the duration applied and the escalation step that
  produced it, so an operator can see a target climbing toward the cap rather than inferring it from
  repeated records.
- **FR-020**: In this release, target health state MUST be tracked independently within each gateway
  instance. No cross-instance coordination is required, and no shared store is introduced.
- **FR-020a**: Health tracking MUST be isolated behind a single, replaceable health-state boundary so
  that a future shared/cross-instance implementation can be substituted without changing the
  publisher-facing configuration, the per-request selection semantics (FR-008 through FR-013), or the
  emitted telemetry. No configuration setting may assume instance-local storage.
- **FR-020b**: Because detection is instance-local, the documented behavior MUST state that a failing
  target is detected independently by each instance, so across N instances a target may absorb up to
  N times the configured consecutive-failure threshold before every instance suspends it. Suspension
  durations and their escalation (FR-019a through FR-019c) are likewise tracked per instance.

**Interaction with routing policies (round-robin, weighted round-robin, header-based, semantic)**

- **FR-021**: Failover MUST be configurable on an operation that also has a routing policy, and the
  two divide the decision as follows (revised 2026-10-29):
  - The routing policy chooses the **first** target. That is the decision it exists to make, and
    discarding it would silently disable load spreading.
  - The publisher's **configured fallbacks take precedence over the rest of the routing policy's
    order**. An explicitly written fallback chain is a deliberate statement about where traffic may
    go next, and a load balancer must not overrule it.
  - Whatever else the routing policy ranked follows as **further depth**, once the configured
    fallbacks are spent. Ranked targets are not discarded — the policy already judged them usable.
  - The configured target remains reachable at the tail, so the model the client asked for is never
    dropped entirely.

  Worked through: a routing policy publishes `[A, D, C]`, and the chain for the requested model is
  `A → [B]`. The result is **A, B, D, C** — when A fails the retry goes to B, the fallback the
  publisher wrote for it, not to D.

  **Superseded**: this requirement previously had the published ranking taken whole and the
  configured chain appended after it as a last-resort tail, which meant a routing policy's order
  silently overrode an explicitly configured fallback. (Which routing policies publish a list at
  launch is fixed by FR-028; those that do not are covered by FR-023.)
- **FR-021a**: The chain that applies MUST be selected by the target the request is **actually
  sent to**, never by the model the client named. A routing policy's first choice is that target;
  with no routing policy it is the requested model, because that is where the request goes by
  default.

  If the publisher configured no chain for that target and no `"*"` catch-all matches, the request
  has **no fallbacks**. A chain written for one target MUST NOT be borrowed for another: the
  publisher never said what should follow a target they did not name, and inferring it from an
  unrelated chain sends traffic somewhere nobody nominated for it.

  Where a routing policy published a ranking, the policy still publishes that ranking even when no
  configured chain matches — otherwise attaching failover would discard the routing policy's
  decision entirely. Inertness (FR-050) applies only when there is neither a matching chain nor a
  published ranking.
- **FR-022**: The routing algorithm MUST be consulted at most once per request. Failover MUST NOT
  re-run it for subsequent attempts, so a request never pays for repeated classification, repeated
  scoring, or a re-rolled weighted draw.
- **FR-023**: A routing policy that publishes only a single selection (no ordered list) MUST still
  work: failover treats it as a one-element list and continues with its own configured chain after
  it. Adoption of the ordered-list contract MUST be incremental — no routing policy is required to
  change for failover to function.
- **FR-024**: Once the configured fallbacks for the first target are spent, failover MUST continue
  through the remainder of the published list, in the order the routing policy ranked it. Ranked
  targets are not discarded — the routing policy already judged them usable. A target reachable
  through both sources MUST be attempted at most once per request (FR-010), at its earliest
  position in the resulting chain.
- **FR-025**: Failover MUST be the single health authority for the operation. Targets suspended
  under the failover configuration MUST be removed from the traversal regardless of where they sit
  in the routing policy's list, and MUST be skipped rather than counted as an attempt.
- **FR-026** *(withdrawn 2026-10-29)*: Failover MUST NOT publish its health state to other
  policies. Suspension is **per-instance state private to failover**, and exposing it would make a
  routing policy's behaviour depend on an internal view it cannot reason about or verify.
  The requirement originally read "failover MUST publish the current healthy target set so a
  routing policy can rank over healthy targets only". It is withdrawn for two reasons. First, it
  buys nothing: chain resolution already filters suspended targets out wherever they appear,
  including the leading one, so a suspended target is never attempted regardless of what a routing
  policy nominated. Second, it cannot be delivered as written — both policies choose in the
  request-header phase, which runs in author-written `operationPolicies` order, so a routing policy
  placed first decides before failover could have published anything.
  The information flow is **one-way by design**: a routing policy publishes its ranking, failover
  consumes it. Nothing flows back. Where a routing policy must change behaviour because failover is
  attached (FR-026a), it is told so at **deployment time** by the controller, not at request time
  through shared metadata.
- **FR-026a** *(withdrawn 2026-10-29)*: A routing policy's own suspension setting is NOT superseded
  when failover is attached. The two health views are independent and both correct, so neither
  needs to defer to the other.
  The requirement originally made the routing policy's suspension inert, on the grounds that one
  upstream failure must not count against two thresholds. It does not, in practice. The downstream
  chain is entered **once per client request** — retries re-invoke only upstream filters — so the
  status a routing policy observes is the FINAL one. A request failover rescued reports success and
  suspends nothing. A 5xx reaches the routing policy only once the whole chain is exhausted, which
  means every member failed, including the one it selected. Suspending that member is accurate, not
  a double count and not a misattribution.
- **FR-026b** *(withdrawn 2026-10-29)*: With no override to apply, there is nothing to report.
- **FR-026c**: A routing policy's own suspension setting MUST continue to behave exactly as it does
  today when that policy is used without failover. Adding failover MUST NOT invalidate, reject, or
  require editing an existing deployed configuration. *(Now trivially satisfied: with FR-026a
  withdrawn, a routing policy's suspension behaves identically either way.)*
- **FR-027** *(revised 2026-10-29)*: When every target is suspended, the operation MUST return the
  same failover exhaustion error as a failover-only configuration (FR-014). A suspended target MUST
  NOT be attempted — enforced where the chain is resolved, which filters suspended targets wherever
  they appear including the leading one, not by constraining what a routing policy may nominate.
  The original wording ("a routing policy MUST NOT nominate a suspended target") placed the duty on
  the producer, which would require it to see failover's health state. FR-026 withdrew that:
  failover's health is private, so a routing policy cannot know, and does not need to. It may
  nominate a suspended target; the target still will not be attempted.
- **FR-028**: `model-round-robin` and `model-weighted-round-robin` MUST publish an ordered
  preference list as part of this feature — round-robin ordering the full rotation from its current
  cursor, weighted ordering the targets by a weighted draw — so the layered behavior in User Story 6
  is verifiable at launch.
- **FR-029**: Routing policies that have not adopted the contract (`llm-header-router`,
  `typesafe-jev-model-routing`) MUST keep working unchanged through the single-selection degradation
  path in FR-023, and MUST NOT be required to change for this feature to ship.
- **FR-030**: Failover MUST NOT change the behavior of operations that do not configure it,
  including operations that configure a routing policy alone.

**API kind applicability and isolation**

- **FR-030a**: The policy MUST be attachable to any API kind whose operations can resolve model
  targets — `RestApi` and `LlmProxy` today, and any future AI-capable kind. There MUST NOT be a
  hard-coded kind allowlist; applicability is decided by whether targets resolve, exactly as
  `model-round-robin` behaves today.
- **FR-030b**: Attaching the policy to an operation whose targets cannot resolve MUST be accepted and
  the policy MUST be **inert** — the request path behaves exactly as if the policy were absent.
  Nothing is logged, counted, or surfaced about the inert state. A kind with no model concept
  therefore deploys successfully and does nothing, consistent with every other policy in the gateway.
- **FR-030e**: Being inert MUST be indistinguishable from the policy not being attached at all: the
  same routing, the same retry behavior, the same responses, and no additional telemetry. "Inert"
  MUST NOT mean "attached but emitting errors per request".
- **FR-030c**: Generated configuration MUST be scoped to the operation that attaches the policy. An
  API that does not attach it MUST be unaffected **even when it routes to the same shared provider
  upstream** — failover MUST NOT mutate the load-balancing behavior, retry behavior, or health
  handling of any shared upstream that other APIs depend on.
- **FR-030d**: The isolation in FR-030c MUST be verified by an explicit test: two APIs sharing one
  provider upstream, one with failover attached and one without, where the second API's routing,
  retry count, and response are unchanged from a deployment with no failover anywhere.

**Flow attachment and outcome attribution**

- **FR-030f**: The policy MUST attach to the **request** flow to resolve and publish the chain. This
  is the only point at which failover is established; the publication must happen before the proxy
  selects an upstream.
- **FR-030g**: The policy MUST also attach to the **response** flow, solely to record target health.
  The response flow MUST NOT attempt to alter routing for the current request — by the time a
  response exists, the attempt sequence has already completed.
- **FR-030h**: The response flow MUST attribute the outcome to the target that actually produced it,
  using the number of upstream attempts made. Attempts 1 through N−1 were retried and therefore
  failed; attempt N produced the response in hand. Crediting the first target for a response a later
  target produced would record a consistently failing target as healthy on every request that
  failover rescued, so it would never be suspended and every request would keep paying its failed
  attempt.
- **FR-030i**: The attempt count MUST be exposed only for operations that fail over. An operation
  without failover MUST see no additional response header or behaviour change (FR-030c).
- **FR-030j**: When the attempt count is absent or unparseable, the system MUST assume a single
  attempt. That grades only the first target and never invents failures for targets the request may
  not have reached.

**Observability**

- **FR-031**: The system MUST record, per request, each target attempted, the reason each attempt was
  treated as a failure, and the target that ultimately served the request. Where a distribution
  policy selected the first target, the record MUST make clear which target it selected.
- **FR-032**: The system MUST emit a record when a target is suspended and when it is restored,
  naming the target and the triggering condition.
- **FR-033**: The system MUST expose counts of failover occurrences, suspensions, recoveries, and
  chain exhaustions, broken down per target, for operational dashboards and alerting.
- **FR-034**: Emitted telemetry MUST NOT contain credentials, authorization headers, or end-user
  prompt or completion content.

#### Published groups carry their clusters

- **FR-057**: A published priority group MUST carry its cluster list, not only its name. Name-only
  publication limits the reachable set to what the control plane pre-enumerated, which is the
  deploy-time constraint `priority_group` was chosen to escape — composite clusters were rejected
  precisely because configuration had to anticipate every case.
- **FR-058**: The control plane MUST therefore supply the policy with a provider-to-cluster mapping
  at deploy time. The cluster is a property of the provider, not the model, so this mapping is
  small, static, and sufficient to construct a group for any model at request time.
- **FR-057a**: Each attempt MUST send **its own member's model**, not the model the client asked
  for. Routing the attempt to the fallback provider is only half of a *model* failover: with the
  payload still naming the primary's model, the fallback is asked for a model it may not serve at
  all, and the rescue fails for a reason that looks like the fallback's fault.

  The substitution is performed by the failover policy itself, running a second time at the
  per-attempt position (FR-037) — not by the engine, which must stay ignorant of failover
  semantics, and not by the vendor transformers, which would each have to re-implement the same
  lookup. The per-attempt instance takes **no configuration**: the per-operation chain was already
  consumed by the downstream pass, so it reads the ranking that request actually installed out of
  dynamic metadata and its own target out of `wso2.attempt` (FR-037f).

  It MUST run **first** in the per-attempt chain. Everything after it — the publisher's own
  policies, the provider's transformer — reads the payload it leaves behind, and a transformer
  translating before the substitution would faithfully translate the wrong request.
- **FR-057b**: The published ranking MUST carry each member's model keyed by **cluster**, as a
  sibling of the priority groups rather than a field inside them (the cluster specifier parses the
  groups, and nothing is gained by handing it a shape it did not ask for). Keying by cluster rather
  than by position is required for the same reason as FR-037f: an index into the ranking goes wrong
  the moment a member is suspended or skipped.

  The `wso2.failover` namespace MUST therefore be forwarded to the upstream filter position. Only
  the analytics namespace was forwarded previously, which would have tied per-attempt behaviour to
  an unrelated serialisation format.
- **FR-057c**: The per-attempt instance MUST be attached only to APIs that actually configure
  failover. A chain containing it declares `RequiresRequestBody`, so attaching it unconditionally
  would make every provider on every LlmProxy buffer request bodies — and pay a gRPC round trip per
  attempt — for a policy with nothing to do.
- **FR-059a**: The attempt budget MUST accommodate the longest ranking the operation can produce.
  Generating `num_retries` from the configured chain length alone caps a routing policy's ranking
  at the publisher's own chain size, silently dropping the tail. Order and length are separate
  freedoms and both are needed for FR-021.
- **FR-059**: A `"*"` member MUST resolve to a working group. Under name-only publication it cannot:
  the controller configures `t:<provider>:*` while the policy publishes
  `t:<provider>:<requested-model>`, and the names never match. This is the concrete failure FR-057
  exists to prevent.

#### Groups must cover every reachable target

- **FR-054**: The control plane MUST generate a priority group for every `(model, provider)` pair
  reachable on an operation — including every target of an attached routing policy, not only those
  listed in `model-failover`'s own parameters. Both lists are known at deploy time.
- **FR-055**: A published ranking naming a group the route does not configure MUST NOT be able to
  misroute the request. Verified by Probe G: Envoy does not skip an unresolvable name, it falls
  back to the **configured group at that position**, so a ranking of
  `[unknown, second-choice]` made three attempts against the operator's *first* configured target —
  a failing one — and never reached the healthy target the ranking preferred. The failure is
  silent: a later attempt may still return 200, so a discarded ranking leaves no signal.
- **FR-056**: Where FR-054 cannot be satisfied for a target — a routing policy nominating something
  the control plane cannot resolve to a cluster — the system MUST drop that entry from the
  published ranking rather than publish a name with no group, so position-based fallback is never
  triggered.

#### Interaction with dynamic cluster selection

- **FR-051**: A route has exactly one cluster specifier, so failover and `cluster_header`
  (`x-target-upstream`) selection are mutually exclusive on the same route. Where an operation
  attaches failover, the `priority_group` specifier MUST own cluster selection.
- **FR-052**: A routing policy's decision MUST therefore reach the data plane as a published
  ordered ranking in `wso2.failover` metadata, not as a header naming one cluster. A header names a
  single cluster and cannot survive a retry, which is precisely what failover needs it to do.
- **FR-053**: **Known regression surface.** `UseClusterHeader` is generated for any API with
  upstream definitions or a sandbox — which includes every multi-provider LlmProxy — so attaching
  failover silently stops `x-target-upstream` taking effect for cluster selection on that
  operation. For model/provider steering this is intended (FR-052 supplies the replacement path).
  For a policy steering to an arbitrary upstream definition, no replacement path exists today. The
  system MUST NOT leave this silent: it MUST at minimum warn at generation time, and SHOULD reject
  a policy combination known to depend on header steering.

#### Model-keyed fallback chains

Adopted from the original design document's parameter schema (pp. 14–16), approved at design
review. Supersedes the flat single-chain shape this spec previously described.

- **FR-045**: A policy attachment MUST be able to declare more than one chain, each keyed by the
  model the **client requested**. A publisher gives `gpt-4o` one chain and `gpt-4.1` another on the
  same operation.
- **FR-046**: A chain keyed `"*"` MUST act as the catch-all for any requested model no other entry
  names. An exact entry MUST always win over `"*"`.
- **FR-047**: A chain member whose model is `"*"` MUST pass the client's own requested model through
  unchanged — the mechanism for failing the same model over to a different provider. One health
  circuit covers every model such a member serves.
- **FR-048**: A fallback with no `provider` MUST inherit its chain's target provider, not the
  proxy's primary. Failing over to the primary from a target on another provider therefore requires
  naming the primary explicitly.
- **FR-049**: Selecting the chain requires the requested model, which is carried in the request
  body, so the policy MUST read the buffered request body. The cost of that buffering MUST be
  confined to operations that attach the policy — an operation without it MUST NOT be forced to
  buffer (FR-030b).
- **FR-050**: When the requested model matches no chain and no `"*"` catch-all exists, the policy
  MUST publish no ordering and MUST NOT treat it as an error.

  **Verified limit (2026-10-26):** publishing nothing does not make the request bypass failover.
  The generated route carries its `cluster_specifier_plugin` unconditionally, so Envoy still walks
  the route's CONFIGURED group order — in practice the first authored chain's members. Measured: a
  request for `a-model-nobody-configured` published no `wso2.failover` metadata (confirmed in the
  policy-engine log) yet still made two attempts along the configured chain.

  Making an unmatched model genuinely bypass failover would require the route to select between a
  failover and a non-failover action per request, which a route-level cluster specifier cannot do.
  Publishers who want a defined destination for unnamed models should configure a `"*"` catch-all;
  without one, the configured order applies. This MUST be stated in the policy's user-facing
  description rather than left as a surprise.

#### Upstream and downstream policy attachment

Carried over from the original design document (§ "Per-provider upstream policies", § "Upstream
ext_proc and existing policies") and the 2026-10-09 design review, which minuted: *"We decided to
go with the upstream ext proc. Under `spec.upstream` or `spec.providers` we will add a section
called `policies` and attach the per upstream policies there."* These requirements define a general
platform capability; model failover is its first consumer, not its only one.

- **FR-035**: A policy MUST be attachable at one of two positions. **Downstream** policies attach to
  an operation and run once per client request. **Upstream** policies attach to an *upstream* and
  run once per attempt, only on attempts that reach that upstream — including a failover retry that
  lands there. This is the attachment-point expression of the execution classes in FR-012d–FR-012f.
- **FR-036**: Upstream policies MUST be attachable on every kind that has an upstream, not only on
  LLM kinds:
  - `LlmProxy`: `providers[].policies` (the primary provider and each additional provider)
  - `LlmProvider`: `upstream.policies`
  - `RestApi`: `upstream.main.policies` and `upstream.sandbox.policies`
- **FR-037**: An upstream policy MUST observe the same policy interface as a downstream one — the
  engine calls the same `On*` methods — so a policy author writes one kind of policy and chooses
  where it runs.
- **FR-037a**: The per-attempt chain is **request-only**. Response processing — including a
  provider's response translation — MUST happen once, downstream, which already knows which target
  answered via the attempt count (FR-030f–FR-030j).

  Revised 2026-10-27 after reading Agent Router, which skips response processing at the upstream
  position deliberately *"to avoid the shenanigans around the retry+the upstream filter"*. We hit
  that interaction directly: a response at the upstream position carries no cluster attribute, so
  resolving its chain required carrying per-stream state from the request phase. Moving response
  work downstream removes that state and the interaction with it. See
  `agent-router-comparison.md` § A2.
- **FR-037b**: Anything an attempt derives from Envoy's `request_attributes` (its target cluster,
  the metadata context) MUST be resolved during that attempt's **headers** phase and carried
  forward to its body phase. Envoy attaches attributes to the **first message of an ext_proc
  stream only**, so a `RequestBody` message carries no `xds.cluster_name` and cannot identify its
  own target.

  This is not a precaution: deriving the cluster from the body message resolved it to `""`, found
  no chain, and silently skipped the per-attempt transformer, sending the fallback provider a
  request in the primary's format while the credential (applied in the headers phase) was correct.
  The failure mode is a silent no-op, not an error. One value per stream suffices because an
  upstream stream serves exactly one attempt. See `poc-results.md` § Probe I.
- **FR-037c**: The body handed to a per-attempt chain MUST be marked **present and complete**.
  Every body-rewriting policy in this repo opens with a variant of `if Body == nil ||
  !Body.Present || len(Content) == 0 { nothing to do }`, so a body passed with `Present` left at
  its zero value makes the chain run and the transformer return no modification — the fallback
  provider is sent the primary's payload format, with no error raised anywhere. Since the body
  phase is requested in BUFFERED mode (FR-037), the whole payload arrives in one message, so
  "present and complete" is the accurate description, not an optimistic one.
- **FR-037d**: The per-attempt body phase MUST run against the attempt's **headers as its own
  headers phase left them**, and any header change the body phase makes MUST be emitted back to
  Envoy alongside the body rewrite. Two reasons, both load-bearing:
  - The executor writes a policy's `HeadersToSet` directly into the header map; a body context
    built without one panics the engine (`assignment to entry in nil map`), killing the ext_proc
    stream and failing the request with a 500.
  - A body-rewriting policy routinely adjusts headers alongside the payload — a corrected
    `content-length`, a vendor API version such as `anthropic-version`. Dropping those sends the
    provider a payload its own headers contradict.

  The map is copied across the phase boundary so a body-phase mutation cannot reach back into the
  headers phase's context.
- **FR-037e**: A path rewrite produced by the per-attempt **body** phase MUST be re-pointed into
  the attempt's provider context, exactly as FR-038's re-point treats the request path. Vendor
  transformers rewrite the path alongside the payload (`openai-to-anthropic-transformer` emits
  `/v1/messages`); emitted verbatim that path lands at the gateway root instead of inside the
  provider's own upstream context, and the attempt reaches the wrong resource. The provider and
  route base paths are resolved during the headers phase and carried forward under FR-037b.
- **FR-037g**: Whenever a per-attempt chain rewrites the body, the engine MUST correct
  `content-length` on that attempt. Envoy rejects a mutated body whose declared length no longer
  matches — `mismatch_between_content_length_and_the_length_of_the_mutated_body` — and surfaces it
  to the client as a 500, so a working rewrite otherwise breaks the request it was meant to rescue.
  A policy that set `content-length` explicitly wins; the correction only fills the gap.
- **FR-037f**: The engine MUST publish a reserved `wso2.attempt` shared-context namespace to every
  per-attempt chain, carrying `number` (1-based; 1 is the original try) and `cluster` (the target
  this attempt is aimed at). This is the platform half of per-attempt policy support: a policy
  whose decision varies per attempt can derive neither value on its own, because the downstream
  chain ran once — before any target was chosen — and Envoy reports the selection only as an
  ext_proc attribute on the attempt's first message.

  A policy identifying *which target* it is talking to MUST prefer `cluster` over `number`: an
  index into a published ranking goes wrong the moment a member is suspended or skipped, whereas
  the cluster names the target directly. The namespace is written on every per-attempt chain and
  is read-only to policies.
- **FR-038**: Each attempt MUST begin from the **post-downstream baseline** — the request as it
  stood when the downstream chain finished, before any upstream chain ran. Concretely, each
  attempt:
  - **keeps** every downstream mutation (masking, redaction, guardrail rewrites, prompt
    decoration, templating, compression), because those ran once and must apply to every attempt;
    this is FR-012a and a fallback target must never receive content the first target was not
    allowed to receive;
  - **discards** the previous attempt's upstream mutations — the prior provider's context path,
    its transformation, its credential — which belong to that target alone.

  "The original request" is NOT the raw client request. Re-deriving from that would discard the
  downstream work and contradict FR-012a; re-running the downstream chain to recover it would
  violate FR-012d by charging quota, rate limits and analytics once per attempt.
- **FR-038a**: Satisfying FR-038 MUST NOT require re-running the downstream chain. The engine
  snapshots the post-downstream state once and resets to it before each upstream invocation.
  That snapshot MUST be held as **server-side state keyed by request**, not round-tripped through
  Envoy.

  Revised 2026-10-27: the earlier design carried the baseline in dynamic metadata written on
  attempt 1, which rests on an assumption never measured — that an *upstream* ext_proc's metadata
  write survives into the next attempt. Agent Router holds the post-downstream request in the
  ext_proc server's own memory and never re-receives it
  (`RequestBodyMode: NONE`, "it can access the original body in its memory"). Holding it
  server-side removes the assumption rather than requiring a probe to confirm it, and saves a
  metadata round trip per attempt. See `agent-router-comparison.md` § A3.
  Without the reset, an upstream policy that appends a header appends it twice on a retry, and one
  that rewrites a path rewrites an already-rewritten one — Probes D and E both observed exactly
  that accumulation in raw Envoy (`/ctx-attempt-2/ctx-attempt-1/...`), so it MUST be handled by
  the engine rather than left to each policy author to remember.
- **FR-039**: For a given upstream, the controller MUST compose the per-attempt chain in this order:
  the author's policies for that upstream, then that upstream's transformer, then that upstream's
  authentication. Authentication is last so that no author-written policy ever observes the
  credential.
- **FR-040**: `model-failover` MUST be attached once, in the operation's policies, and MUST NOT be
  attachable in an upstream's policy list. Whatever per-attempt work it requires is added by the
  controller, never authored into a provider list.
- **FR-041**: An attempt MUST run only the selected upstream's policy list. Running another
  upstream's list would apply the wrong transformation and the wrong credential to that attempt.
- **FR-042**: The upstream position MUST fail closed: if the per-attempt chain cannot run, the
  attempt MUST NOT proceed to the upstream unauthenticated or untransformed. No upstream credential
  may reach the downstream response path, and any internal provider-selection header MUST be removed
  before the response reaches the client.
- **FR-043**: Splitting policies across two chains costs a second ext_proc round trip per attempt.
  When an API has no per-attempt work to do, the system MUST NOT pay that cost: the upstream chain
  is generated only where it is needed.
- **FR-043b**: Per-attempt state MUST be keyed on a cluster that is unique to the API. The main
  upstream slot's cluster key is derived from host and port alone
  (`upstream_main_127.0.0.1_8080`), so every LlmProxy with a loopback primary shares it; the
  primary also has its own API-scoped loopback definition, and that is the one to key on.

  Found 2026-10-27 while checking whether per-cluster filter placement fits this topology.
  Keying on the shared cluster collides across APIs: two proxies publish different chains under
  the same `upstream:<cluster>` key, the last write wins, and one API's per-attempt policies —
  including its credential — are applied to another API's request. The same sharing would make a
  per-cluster filter apply across APIs.

- **FR-043a**: The upstream filter chain MUST be attached **per cluster**, via
  `HttpProtocolOptions.http_filters` on the cluster's `typed_extension_protocol_options` — not to
  the Router filter, whose `upstream_http_filters` apply to every route on the listener.

  Revised 2026-10-27. This spec previously asserted that no per-cluster placement existed and that
  listener-wide attachment plus `ExtProcPerRoute` gating was the only option; Agent Router's
  extension server does exactly this per cluster, so the assertion was wrong. Per-cluster
  placement satisfies FR-043 by construction: a cluster with no per-attempt work simply carries no
  filter. No gating mechanism is required, and no opt-in flag is needed to protect unrelated
  traffic. The cluster is the provider here, which makes this a closer fit than per-route would
  have been. See `agent-router-comparison.md` § A1.
- **FR-044**: Envoy does not invoke ext_proc for its own locally generated replies, so a connection
  failure, reset, or per-attempt timeout produces no upstream or downstream policy invocation. The
  system MUST therefore record transport-failure outcomes from a source other than the policy
  response path, or explicitly document that transport failures fail over (Envoy retries them) but
  never contribute to suspension (FR-009, FR-018).

### Key Entities

- **Failover configuration**: The ordered, publisher-authored definition attached to an AI proxy API
  operation. Holds the target list — optional when an adopting routing policy is attached, and
  supplying the fallbacks attempted immediately after that policy's chosen target — plus failure
  conditions, suspension threshold and duration,
  suspension escalation settings (multiplier cap and reset period), and per-attempt time limit.
- **Target**: One addressable destination in the chain — a model together with the provider
  configuration it is reached through. Identity is the pair, not the model name alone; the same model
  under two provider configurations is two distinct targets.
- **Target health**: The runtime state of one target — consecutive failure count, healthy / suspended
  status, the current suspension duration and escalation step, and when the suspension expires. Held per gateway instance in this
  release, behind a replaceable boundary so the same state can later be held in a store shared across
  instances without changing publisher-facing behavior. When a routing policy is layered on the
  same operation, this is the single health view both policies consult.
- **Target preference list**: The ordered ranking a routing policy publishes for one request — its
  own answer to "where should this go, best first". Failover traverses it, filters it against target
  health, and continues into its own configured chain once the list is exhausted. A routing policy
  that publishes a single selection produces a one-element list.
- **Failover attempt record**: What happened for one request — the ordered list of targets attempted,
  each attempt's outcome and failure reason, the serving target, and the total targets skipped.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: When a configured failure condition is returned by the primary target, 100% of affected
  requests that have a healthy fallback target available are served successfully, with no error
  surfaced to the calling application.
- **SC-002**: Applications consuming an AI proxy API require zero code changes to benefit from
  failover — the request and the successful response shape are unchanged from the non-failover case.
- **SC-003**: During a complete outage of the primary target, the added end-to-end latency for a
  served request stays within the configured per-attempt time limit plus the fallback target's own
  response time, and once the primary is suspended that added latency drops to effectively zero.
- **SC-004**: An outage confined to one provider configuration or one region leaves requests to other
  targets completely unaffected — zero suspensions and zero failovers attributable to it on unrelated
  targets.
- **SC-005**: A publisher can configure an ordered fallback chain for an operation and verify it
  working in under 10 minutes, using only product documentation.
- **SC-006**: When every target is unavailable, 100% of requests receive the same consistent
  exhaustion error, and none receive an upstream error body, partial response, or hung connection.
- **SC-007**: An operator can determine, for any failed-over request, which target served it and why
  the previous target was skipped, entirely from platform telemetry.
- **SC-008**: Under sustained load with one target failing, a target is never attempted more than
  once per request and the chain never loops — verified across at least 10,000 requests.
- **SC-009**: With a routing policy and failover configured on the same operation and one target
  failing persistently, requests continue to succeed, the failing target receives no distributed
  traffic once suspended, and the remaining healthy targets receive traffic in their configured
  proportions (within normal distribution tolerance).
- **SC-010**: In 100% of fallback attempts on an operation that also applies masking, redaction, or
  guardrail policies, the fallback target receives the same protected content the primary target
  would have received — zero attempts carry pre-mutation content, and any attempt that cannot is
  failed and recorded rather than sent.
- **SC-014a**: A target that fails on every request but is always rescued by failover is suspended
  within its configured threshold — the suspension counters advance even though the client never
  sees an error.
- **SC-012a**: An API that does not attach failover behaves identically whether or not another API
  on the same gateway attaches it to a shared provider upstream — same route selection, same retry
  count, same responses, verified by comparing against a deployment with no failover present.
- **SC-011b**: A target whose p95 latency crosses the configured threshold while no individual
  request exceeds `perAttemptTimeoutMs` is suspended within one window once the minimum sample size
  is met; a single outlier-slow request against an otherwise fast target suspends nothing.
- **SC-011a**: A target failing roughly one request in three — never producing a run of consecutive
  failures — is suspended within one configured window once the minimum sample size is met, and the
  suspension record names the rate criterion and the observed rate.
- **SC-011**: A client request that fails over any number of times consumes exactly one unit of
  quota, is counted once against rate limits, is billed for the tokens of the attempt that served it,
  and appears once in analytics — verified across chains of at least three attempts.
- **SC-012**: A publisher whose fallback chain is exactly the routing policy's rotation configures
  those targets in one place only — zero restatement — and a publisher who wants a last-resort model
  held out of rotation can express it without disabling load balancing.
- **SC-013**: Every existing deployed configuration that uses a routing policy without failover keeps
  its current behavior unchanged — zero configurations require editing, and zero are rejected at
  deployment, as a result of this feature.
- **SC-014**: Introducing shared, cross-instance health state in a later release requires no change
  to any publisher's existing failover configuration and no change to the per-request behavior those
  publishers observe.

## Assumptions

- The feature is delivered as a gateway policy attachable to AI/LLM proxy API operations, consistent
  with the platform's existing model-routing policies; it is not a change to core proxy routing.
- Targets are drawn from provider configurations already associated with the API, using the existing
  mechanism that lets an AI proxy API reference additional providers. Failover does not introduce a
  new way to declare providers or credentials.
- Default failure conditions, when the publisher configures none, are rate-limit (429) and
  server-error (5xx) responses plus all transport-level failures. Authentication, authorization, and
  client-input errors are not failover-worthy by default, since retrying them on another target would
  fail the same way or mask a genuine client bug.
- Defaults for suspension threshold, base suspension duration, and escalation settings follow the
  conventions already set by the existing model-routing policies, so publishers meet one
  consistent set of knobs.
- Health state is instance-local for this release by explicit decision (FR-020). Shared,
  cross-instance health state is an anticipated future enhancement, so the design is expected to keep
  health tracking behind one replaceable boundary (FR-020a) rather than spreading instance-local
  assumptions through selection, configuration, and telemetry.
- Failover ships as its own policy, separate from the routing policies, and owns the retry, health,
  suspension, and recovery logic (clarification 2026-10-22). Routing policies are not required to
  change for failover to work.
- The integration contract between routing policies and failover is an ordered target preference
  list (FR-021 through FR-024). Routing policies adopting it is an incremental enhancement to each
  one, not a precondition: an unmodified routing policy that publishes a single selection still
  composes correctly.
- Adoption at launch covers the two WSO2-owned load-balancing routers only (FR-028). Extending the
  contract to `llm-header-router` and to the vendor-owned `typesafe-jev-model-routing` is deferred to
  a later release and does not block this one.
- Same-request re-attempt (FR-008) is the feature's core value and is **not** negotiable down to
  next-request failover; the existing routing policies already provide the latter. Where the gateway
  cannot today re-send a request after an upstream failure, adding that capability at the routing
  layer is in scope for this feature (clarification 2026-10-22), justified in the plan's Complexity
  Tracking table rather than assumed silently.
- Proxy-level research (2026-10-22, revised) settled how an attempt sequence is realized. Eight
  mechanisms were evaluated; the adopted one lets the gateway **publish a ranked list of targets as
  request data**, which the proxy then walks across attempts in that order, skipping entries that are
  unavailable. This satisfies FR-021 as written — any per-request ranking is expressible — while the
  policy chain executes exactly once per client request and no per-ordering configuration is
  generated. Mechanisms that ordered targets statically, that re-ran the policy chain per attempt, or
  that enumerated configuration per possible ordering were rejected.
- Because the policy chain executes once, the once-per-client-request behavior required by FR-012d is
  structural rather than enforced by bookkeeping. FR-012f's execution-class declaration determines
  which processing position a policy occupies: the single downstream pass, or the per-attempt path.
- Per-attempt work — provider credentials and request translation — runs at the processing position
  that re-executes per attempt. That position must read the request body **as mutated by the policy
  chain**, never an earlier snapshot; this is exactly the hazard FR-012a through FR-012c exist to
  prevent, and it is a blocking verification item, not a known issue.
- The adopted mechanism selects among targets reachable within a single upstream group, while this
  gateway models one upstream per provider. Making each provider addressable within that group —
  without losing its own transport security and credentials — is an open design item recorded as R1
  in the plan, to be resolved before implementation begins.
- Steering an individual attempt to a different provider's upstream is an existing gateway
  capability (a policy-set upstream directive resolved through header-based cluster selection, with
  the route cache cleared so routing is re-evaluated). Failover depends on that existing mechanism
  rather than introducing a second way to choose an upstream.
- The failover policy keeps its own target list because it serves two purposes a published list
  cannot: it is the only source of targets when no routing policy is attached (the issue's headline
  case), and it expresses last-resort targets deliberately held out of normal rotation. Where a
  published rotation is already the whole intended chain, the list is simply omitted (FR-001a).
- The platform's routing policies today perform target selection and consecutive-failure suspension
  but do NOT re-attempt a failing request — the gateway returns that request's error to the client
  and only subsequent requests move elsewhere. Same-request chain traversal is the new capability
  this feature introduces, not a re-implementation of existing behavior.
- The client-visible successful response is the serving target's response; the platform does not add
  a new client-facing header naming the serving model. Serving-target identity is exposed through
  telemetry and analytics instead.
- Non-streaming requests are fully replayable to a second target. For streaming requests, failover is
  available only before the first byte of the response reaches the client.
- Cost and token accounting already attribute usage per model and provider; failover reuses that
  attribution rather than defining its own.
- Management portal / UI affordances for editing the failover configuration follow whatever the
  existing model-routing policies offer; no bespoke UI work is assumed in this specification.
- Out of scope for this feature: ordered-list adoption by `llm-header-router` and
  `typesafe-jev-model-routing`, shared cross-instance health state, latency-based or cost-based
  automatic target selection, retrying the same target more than once, partial-response resumption of
  an interrupted stream, and failover for operations that cannot resolve model targets.
