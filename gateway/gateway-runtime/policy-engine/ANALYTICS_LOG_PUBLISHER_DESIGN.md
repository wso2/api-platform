# Design: Collector & Stdout Traffic Logging

> **Maintenance:** This document describes the shared **collector** (data-capture pipeline)
> and its stdout **traffic-logging** consumer, both built on the analytics pipeline. **Keep
> it in sync with the code — every change to this feature (new config, new capture point,
> behavior change) must update this document in the same PR**, including the Change Log at
> the bottom.

_Last updated: 2026-07-07_

---

## 1. Problem we are trying to solve

Operators running a **self-hosted** API Platform gateway want to observe API traffic —
who called what, status/latency, the request/response **headers**, and optionally the
**bodies** — as structured logs on **stdout**

Constraints / sub-problems:

- **Bodies and arbitrary headers cannot come from the Envoy access log.** Envoy's file
  access logger has no body operator, and headers must each be named explicitly — there is
  no "all headers" or "mask these" capability. So enriching `[router.access_logs]` is not
  enough.
- **Stdout logging must not require "enabling analytics."** Data capture and external
  analytics (Moesif) used to be a single `analytics.enabled` switch, so an operator who only
  wanted stdout logs had to turn on the whole analytics feature. Capture is a *shared*
  concern; the consumers (analytics, traffic logging) are independent.
- **Payload size control.** Bodies can be large; operators need a cap.
- **Sensitive data.** Header values such as `Authorization` must be redactable, both
  globally and per-API.
- **Per-API opt-in, not all-or-nothing.** Emitting a stdout line for *every* API is too
  noisy and rarely wanted. Operators need to log only specific APIs, chosen the same way
  other behavior is chosen — by **attaching a policy** to those APIs — while still getting
  the access-log-derived fields (latencies, status) that inline policies cannot see.

## 2. Proposed Solution

Split the existing analytics pipeline into a shared **collector** (data capture + transport)
and independent **consumers** that read what the collector gathers:

- **`[collector]`** — the capture pipeline: the auto-injected analytics system policy
  (captures headers/bodies) plus the Envoy→policy-engine ALS transport. It has **no on/off
  flag of its own**: it is **implicit**, active whenever a consumer is enabled and off
  otherwise (see `IsCollectorEnabled`).
- **`[analytics]`** — consumer 1: external analytics publishers (e.g. Moesif). The
  collector captures data for **all** publishers uniformly; masking/redaction is each
  publisher's own concern — the collector does not filter on a publisher's behalf (see D4
  in prior designs; Moesif receives raw headers regardless of `traffic_logging.masked_headers`).
- **`[traffic_logging]`** — consumer 2 (this feature): serializes a collected event to
  stdout as a single JSON line. No external SaaS needed. It is **per-API opt-in**: a line
  is emitted only for APIs that attach the **`log-message` policy with
  `enableTrafficLogging: true`**.

Capture settings (`send_*_body`, `send_*_headers`) live under `[collector]` and are shared
by all consumers — capture of each is **off by default**; operators enable what they need.
Output/presentation settings — `masked_headers` (redaction) and `max_payload_size` (payload
byte cap) — live under `[traffic_logging]` and apply only to the stdout publisher; other
consumers (e.g. Moesif) are unaffected.

### Per-API gating: the `log-message` policy (`enableTrafficLogging`)

The stdout publisher does **not** log every API. An API's traffic is logged only when a
two-way AND holds: `[traffic_logging].enabled` **and** the **`log-message` policy with
`enableTrafficLogging: true`** is attached to that API/route. (The collector is implicit —
enabling `[traffic_logging]` activates it automatically.)

`log-message` (in `gateway/dev-policies/log-message`, v1.1+) has two modes. The default
(`enableTrafficLogging: false`, or absent) logs in real time during mediation via `slog`
(including per-chunk streaming) with no dependency on the collector. Setting
`enableTrafficLogging: true` turns the policy into a lightweight **signal**: it emits no log
line itself — latencies only exist at access-log time, beyond an inline policy's reach.
Instead, in `OnRequestHeaders` it stamps a marker (`AnalyticsMetadata["traffic_log"] =
<json directive>`) carrying its per-API presentation config, and `Mode()` requests only the
request-header phase (no body buffering). The kernel merges that into `analytics_data`,
Envoy carries it onto the `HTTPAccessLogEntry`, and `prepareAnalyticEvent` reads it back
into `event.TrafficLog`. `Log.Publish` emits **only** when `event.TrafficLog != nil`, and
uses the directive to shape the line. Because capture happens globally in the collector
*before* any user policy runs, the directive can **filter/mask** what was captured but
cannot enable capture the collector skipped.

### Policy parameters (`log-message` v1.1, `enableTrafficLogging`)

Attach the policy to an API's definition YAML with `enableTrafficLogging: true` to opt that
API into stdout traffic logging. Both `request` and `response` are optional but at least
one, or `fields`, should be set for the line to carry anything beyond the base event fields.

```yaml
policies:
  request:
    - policyName: log-message
      policyVersion: v1.1.0
      parameters:
        enableTrafficLogging: true      # required to activate stdout traffic logging
        request:
          payload: true                # include request body in the log line
          headers: true                # include request headers
        response:
          payload: false
          headers: true
        # Optional: additional header names (case-insensitive) redacted for this API,
        # merged with the global traffic_logging.masked_headers.
        maskedHeaders:
          - x-internal-token
        # Optional: extra key/value pairs added under the top-level "properties" object.
        # "$ctx:" values are resolved from the request context at request time; other
        # values are literal.
        properties:
          subject: "$ctx:auth.subject"
          authType: "$ctx:auth.type"
          tenant: "$ctx:auth.property.tenant"
          env: prod
        # Optional: fine-grained field projection over the emitted JSON line.
        # When set, fields is authoritative — request/response payload & header booleans
        # are ignored (maskedHeaders still applies).
        fields:
          exclude:
            - requestBody
```

| Parameter | Type | Default | Description |
|---|---|---|---|
| `enableTrafficLogging` | bool | `false` | **Must be `true`** to opt in. When `false` (default), the policy logs inline in real time via `slog` during mediation — independent of the collector. |
| `request.payload` | bool | `false` | Include the request body. No-op if `[collector].send_request_body = false`. Ignored when `fields` is set. |
| `request.headers` | bool | `false` | Include request headers. No-op if `[collector].send_request_headers = false`. Ignored when `fields` is set. |
| `response.payload` | bool | `false` | Include the response body. No-op if `[collector].send_response_body = false`. Ignored when `fields` is set. |
| `response.headers` | bool | `false` | Include response headers. No-op if `[collector].send_response_headers = false`. Ignored when `fields` is set. |
| `maskedHeaders` | `[]string` | `[]` | Additional header names (case-insensitive) redacted to `****` in `requestHeaders`/`responseHeaders`, merged with the global `traffic_logging.masked_headers`. Always applied, even when `fields` is set. To drop a header entirely instead of redacting it, use a dotted `fields.exclude` path (e.g. `requestHeaders.X-Secret`). |
| `properties` | `object` | `{}` | Extra key→value pairs emitted under a top-level `properties` object. String values prefixed `$ctx:` are resolved from the request context at request time (see below); non-`$ctx` strings are literals; non-string values pass through as-is. Unresolvable `$ctx:` refs are skipped. |
| `fields.only` | `[]string` | `[]` | Keep only the listed keys (top-level, e.g. `latencies`, `requestHeaders`, `properties`, or dotted sub-paths, e.g. `requestHeaders.authorization`, `properties.env`). Authoritative over `request`/`response` payload & header booleans. If both `only` and `exclude` are set, `only` wins. |
| `fields.exclude` | `[]string` | `[]` | Drop the listed keys/dotted sub-paths and keep everything else. |

**`properties` `$ctx:` references** (resolved in the policy's `OnRequestHeaders`, so only request-phase context is reachable; fixed names are case-insensitive, `auth.property.<key>` is case-sensitive):

| Reference | Resolves to |
|---|---|
| `$ctx:request.path` / `.method` / `.authority` / `.scheme` / `.vhost` | request line fields |
| `$ctx:request.id` | correlation request ID (`RequestHeaderContext.RequestID`) |
| `$ctx:request.header.<name>` | first value of that request header (skipped if absent) |
| `$ctx:api.id` / `.name` / `.version` / `.context` / `.kind` / `.operation_path` | API metadata |
| `$ctx:project.id` | project the API belongs to |
| `$ctx:auth.subject` / `.type` / `.issuer` / `.credential_id` / `.token_id` | `AuthContext` string fields (`token_id` = jwt `jti`, skipped when empty) |
| `$ctx:auth.authenticated` / `.authorized` | `"true"` / `"false"` |
| `$ctx:auth.audience` | `AuthContext.Audience` joined by `,` |
| `$ctx:auth.scopes` | granted scopes, space-joined, sorted |
| `$ctx:auth.property.<claim>` | `AuthContext.Properties["<claim>"]` (custom claims) |

> `auth.*` references require an authentication policy to have run **before** `log-message` in the request-header phase; when `AuthContext` is nil they resolve to nothing and the property is skipped. `properties` is only meaningful with `enableTrafficLogging: true` (ignored in inline mode).

> **Precedence:** `fields` (when present) overrides `request.payload`, `request.headers`,
> `response.payload`, and `response.headers`. `maskedHeaders` (per-API, merged with the
> global `traffic_logging.masked_headers`) always applies regardless.
>
> **Capture limitation:** the directive can only filter/mask what the collector already
> captured. `request.payload: true` has no effect if `[collector].send_request_body = false`.

### Data flow (gating in **bold**)

```
client → Envoy   (ALS cluster + gRPC access log attached when COLLECTOR enabled)
          │  ext_proc → policy-engine system policies
          │     • collector system policy (injected when COLLECTOR enabled) captures
          │       headers/body into dynamic metadata "analytics_data"
          ▼
        Envoy gRPC ALS  ── HTTPAccessLogEntry ──▶ policy-engine ALS server
                                                  (started when COLLECTOR enabled)
                                                      │
                                                      ▼
                                            Analytics.Process(entry)
                                              • prepareAnalyticEvent → dto.Event
                                              • for each registered publisher: Publish(event)
                                                      │
                              ┌───────────────────────┴───────────────────────┐
                              ▼                                                 ▼
                  Moesif publisher (ANALYTICS enabled)        Log publisher (TRAFFIC_LOGGING enabled)
                    (logs every event, raw headers)              **if event.TrafficLog == nil → skip**
                                                                (per-API: set only when log-message
                                                                 with enableTrafficLogging is attached)
                                                                  → shape per directive (filter/mask,
                                                                    truncate to max_payload_size)
                                                                  → JSON line (incl. latencies) → stdout
```

The `log-message` policy (with `enableTrafficLogging: true`) runs on the ext_proc side
(above, "collector system policy" step): it stamps `analytics_data["traffic_log"]`
alongside the captured headers/bodies, so by the time the entry reaches
`prepareAnalyticEvent` the opt-in marker is present.

## 3. Test scenarios

| Area | Scenario | Where |
|---|---|---|
| Log publisher | Per-API gate: an event **without** `TrafficLog` is not logged | `publishers/log_test.go: TestLog_Publish_SkipsWhenNoDirective` |
| Log publisher | Emits one JSON line incl. ALS-derived `latencies`; event fields present | `TestLog_Publish_WritesJSONLineWithLatencies` |
| Log publisher | `headers:false`/nil flow omits the property; `payload:false` omits the payload | `TestLog_Publish_DisabledFieldsOmitted` |
| Directive extraction | `prepareAnalyticEvent` reads `traffic_log` marker into `event.TrafficLog` (nil when absent; opts in with defaults when malformed); never leaks into `Properties` | `analytics_test.go: TestPrepareAnalyticEvent_TrafficLogMarker`, `_NoTrafficLogMarker`, `_MalformedTrafficLogMarker` |
| log-message (traffic logging) | `enableTrafficLogging` parsing, `Mode()` (request-header only in that mode), marker JSON from params (incl. `fields`, `properties`, `maskedHeaders`), no inline log, safe degradation | `gateway/dev-policies/log-message/logmessage_test.go` |
| `properties` `$ctx:` | `resolveContextValue` resolves request/api/auth refs (incl. `auth.property.<key>`, joined audience/scopes); literals pass through; unresolvable/nil-auth refs skipped; non-string passthrough; empty result omits the marker field | policy `logmessage_test.go: TestResolveContextValue`, `_NilAuthSkipped`, marker-build tests |
| Properties emission | resolved properties appear under a top-level `properties` object; absent when the directive has none; projectable via `fields` (`properties`/`properties.<key>`) | `publishers/log_test.go: TestLog_Publish_PropertiesTopLevel`, `_NoPropertiesWhenAbsent`, `_PropertiesProjectableViaFields` |
| Field projection | `fields.only` keeps only named keys/dotted sub-paths; `fields.exclude` drops them; authoritative over presence (flow booleans ignored) while `maskedHeaders` still applies | `publishers/log_test.go: TestLog_Publish_FieldsInclude`, `_FieldsExclude`, `_FieldsIncludeRequestBodyAndProperties` |
| Log publisher | `masked_headers` (global) redacts values to `****`, case-insensitive, for request & response headers | `TestLog_Publish_MasksHeaders` |
| Log publisher | Per-API `maskedHeaders` merges with the global list (with and without a global list configured) | `TestLog_Publish_PerAPIMaskedHeadersMergedWithGlobal`, `_PerAPIMaskedHeadersNoGlobal` |
| Log publisher | A dotted `fields.exclude` path (e.g. `requestHeaders.X-Secret`) drops a header entirely, vs. masking which redacts it | `TestLog_Publish_ExcludeHeadersDrops` |
| Log publisher | Masking does **not** mutate the shared event seen by other publishers | `TestLog_Publish_DoesNotMutateSharedEvent` |
| Log publisher | `nil` event is a no-op (no panic, no output) | `TestLog_Publish_NilEvent` |
| Log publisher | A header value that is not valid JSON is passed through unchanged / dropped | `TestLog_Publish_UnparseableHeadersDropped` |
| Log publisher | `NewLog(nil)` returns a usable publisher with no masked headers | `TestNewLog_NilConfig` |
| Publisher wiring | `traffic_logging.enabled` registers the stdout publisher | `analytics_test.go: TestNewAnalytics_TrafficLoggingEnabled` |
| Collector (implicit) | `IsCollectorEnabled()` is false with no consumer, true when analytics or traffic logging is on; config validates in each case | engine `config_test.go: TestIsCollectorEnabled`; controller `config_test.go: TestConfig_IsCollectorEnabled` |
| Deprecated aliases | `analytics.allow_payloads`/`send_*_body` migrate onto `collector.send_*_body`; directional flags win; skipped when analytics is disabled | `TestValidate_AnalyticsPayloadMigration(_SkippedWhenAnalyticsDisabled)` (engine), `TestConfig_ValidateAnalyticsPayloadMigration(_SkippedWhenAnalyticsDisabled)` (controller) |
| Deprecated transport aliases | `analytics.access_logs_service`/`grpc_event_server` migrate onto `collector.als` only while analytics is enabled, so a traffic-logging-only deployment never re-applies a stale analytics override | `TestValidate_AnalyticsTransportMigration_SkippedWhenAnalyticsDisabled` (engine), `TestConfig_ValidateAnalyticsTransportMigration_SkippedWhenAnalyticsDisabled` (controller) |
| Max payload (publisher) | Log publisher truncates `request_payload`/`response_payload` to `traffic_logging.max_payload_size` (0 = no limit) | `publishers/log_test.go: TestLog_Publish_TruncatesPayload`, `_NoTruncationWhenZero`; validation: `config_test.go: TestValidate_TrafficLoggingMaxPayloadSize` |
| Collector gating | system policy injected only when a consumer is enabled (`IsCollectorEnabled`); capture flags (body and header, both off by default) propagate from `[collector]` | `gateway-controller .../system_policies_test.go: TestInjectSystemPolicies_Collector*`, `_BodyFlagsPropagated`, `_BodyFlagsDefaultFalse`, `_HeaderFlagsPropagated`, `_HeaderFlagsDefaultFalse`, `_TrafficLoggingOnlyEnablesCollector` |
| Header capture | `getHeaderFlags` parses bool/string forms; `serializeHeaders` emits a JSON object (lower-cased keys, multi-value joined) and "" for empty | `system-policies/analytics/analytics_headers_test.go` |

**Manual / end-to-end (not yet automated):** with `traffic_logging.enabled = true`,
`analytics.enabled = false` (the collector activates implicitly), and
`collector.send_request_body`/`send_response_body`/`send_request_headers`/`send_response_headers`
turned on, deploy two APIs and attach the **`log-message`** policy with
**`enableTrafficLogging: true`** to only one. A request to the API with the policy produces
one JSON line on the **policy-engine** stdout containing the latencies, headers (auth
masked), and body (≤ cap), with **no** Moesif activity; a request to the API **without** the
policy produces **no** line; disabling `traffic_logging.enabled` (with analytics also off)
produces no lines even with the policy attached — the collector goes inactive.

## 4. Implementation

| Component | File | Responsibility |
|---|---|---|
| Log publisher | `internal/analytics/publishers/log.go` | `Log` implements `Publisher.Publish`; **per-API gate** (`event.TrafficLog == nil → skip`); `toTrafficLogEvent` (in `traffic_log_event.go`) shapes the per-flow headers/payload per the directive, merges per-API `MaskedHeaders` with the global config list, and truncates to `traffic_logging.max_payload_size` (0 = no limit) on a locally-built `TrafficLogEvent`, never mutating the shared `dto.Event`/`Properties`. When `Fields` is set, `applyFieldsProjection` shallow-decodes the marshaled line into `map[string]json.RawMessage` and projects to (`Only`) or without (`Exclude`) the named top-level keys / dotted sub-paths — decoding and re-encoding only the touched nested objects, leaving untouched top-level fields as raw bytes. `sync.Mutex` serializes writes; `NewLog(*config.TrafficLoggingConfig)` |
| Publisher selection | `internal/analytics/analytics.go` | `NewAnalytics` registers Moesif when `analytics.enabled` and the stdout `Log` publisher when `traffic_logging.enabled` (independently) |
| Event enrichment | `internal/analytics/analytics.go` | `prepareAnalyticEvent` attaches `requestHeaders`/`responseHeaders` and `request_payload`/`response_payload`; reads the `traffic_log` marker and parses it per-request via `parseTrafficLogDirective` (no caching — the marker's `Properties` are already request-specific, so caching by raw JSON would leak one API's properties into another request that happens to share the same static portion) into `event.TrafficLog` (gating state, kept off `Properties`) |
| log-message policy (`enableTrafficLogging`) | `gateway/dev-policies/log-message/logmessage.go` (v1.1+, in `build-manifest.yaml`/`build.yaml`) | `enableTrafficLogging: true` turns the policy into a per-API opt-in signal: `Mode()` = request-header phase only; `OnRequestHeaders` → `stampTrafficLogMarker` stamps `AnalyticsMetadata["traffic_log"] = <json directive>` (request/response `payload`/`headers`, `fields`, `properties`, `maskedHeaders`). `buildProperties`/`resolveContextValue` expand `$ctx:` refs in `properties` at request time and bake the resolved values into the marker. `false` (default) keeps the original real-time slog logging. Registered via build-manifest |
| Traffic-log directive | `internal/analytics/dto/event.go` | `Event.TrafficLog *TrafficLogDirective` (`json:"-"`); `TrafficLogDirective{Request,Response *TrafficLogFlow; Fields *TrafficLogFields; Properties map[string]interface{}; MaskedHeaders []string}`, `TrafficLogFlow{Payload,Headers bool}`, `TrafficLogFields{Only,Exclude []string}` |
| Engine ALS startup | `cmd/policy-engine/main.go` | ALS gRPC server started when `IsCollectorEnabled()` (a consumer is on) |
| Policy-engine config | `internal/config/config.go` | `CollectorConfig{SendRequestBody, SendResponseBody, AccessLogsServiceCfg}` (no `Enabled` — implicit; no header flags — header capture is a controller-side/capture-time concern); `IsCollectorEnabled() = Analytics.Enabled \|\| TrafficLogging.Enabled`; `TrafficLoggingConfig{Enabled, MaskedHeaders, MaxPayloadSize}`; `validateCollectorConfig` (deprecated-alias migration for body flags **and** `access_logs_service`, the latter only while `Analytics.Enabled`; validates the ALS receiver when the collector is active); `traffic_logging.max_payload_size >= 0` validated in `Validate` |
| Controller config | `gateway-controller/pkg/config/config.go` | `CollectorConfig{SendRequest/ResponseBody, SendRequest/ResponseHeaders, GRPCEventServerCfg}` (no `Enabled`; body and header capture both default **false** — off until an operator opts in); `TrafficLoggingConfig{Enabled}` (so the controller knows traffic logging activates the collector); `IsCollectorEnabled() = Analytics.Enabled \|\| TrafficLogging.Enabled`; `validateCollectorConfig` (alias migration for body flags **and** `grpc_event_server`, the latter only while `Analytics.Enabled`; validates the ALS sink via `validateGRPCEventServerConfig` when the collector is active) |
| Controller gating | `gateway-controller/pkg/xds/translator.go` | ALS cluster + gRPC access-log config attached when `IsCollectorEnabled()`, built from `Collector.GRPCEventServerCfg` |
| Engine ALS receiver | `internal/utils/access_logger_server.go` | Reads `Collector.AccessLogsServiceCfg` (mode, port, TLS, message/header limits) to start the ALS gRPC server |
| System-policy wiring | `gateway-controller/pkg/utils/system_policies.go` | The analytics (collector) system policy is injected when `IsCollectorEnabled()`; propagates `send_*_body`, `send_*_headers` from `[collector]` into policy params |
| Analytics system policy | `gateway/system-policies/analytics/analytics.go` | Captures headers (`serializeHeaders`, gated by `getHeaderFlags`, off by default) into `request_headers`/`response_headers` and request/response bodies (gated by the body flags, off by default) at all capture points (request, buffered response, streaming chunks). No size cap at capture — payload truncation is output-side in the Log publisher |
| Config docs | `gateway/configs/config-template.toml`, `config.toml` | Document/example `[collector]`, `[analytics]`, `[traffic_logging]` |

### Configuration surface

```toml
[collector]                       # implicit — shared capture pipeline + transport (no enabled flag)
send_request_body = false         # capture bodies in full (controller→capture AND engine→attach); off by default
send_response_body = false
send_request_headers = false      # capture ALL headers (capture side, controller only); off by default
send_response_headers = false

# ALS transport tuning (advanced; sensible defaults). One shared section read by
# BOTH ends: the controller (Envoy's gRPC ALS sink — buffers, ports, TLS) and the
# policy-engine (the receiving ALS server — mode, port, limits). Envoy-sender-only
# keys (buffer_*, grpc_request_timeout) are ignored by the engine.
[collector.als]

[analytics]                       # consumer 1 — enabling it activates the collector
enabled = false
enabled_publishers = ["moesif"]

[traffic_logging]                 # consumer 2 — enabling it activates the collector
enabled = true
masked_headers = ["authorization"]              # output-side redaction (log publisher only)
max_payload_size = 2048                         # 0 = no limit; truncates the stdout line only
```

**Where each key is read (two-sided config note):**

- `collector.send_*_body` — **both** sides: controller config drives the system policy to
  *capture* the (full) payload; policy-engine config drives `prepareAnalyticEvent` to *attach* it.
- `collector.send_*_headers` — **capture side only** (controller config → system policy param).
- `traffic_logging.max_payload_size` — **policy-engine** config, applied output-side by the
  Log publisher (per-consumer; other consumers unaffected).
- `collector.als` — **both** sides (one shared section): the controller reads it to tune
  Envoy's gRPC ALS sink (validated in the controller's `validateCollectorConfig`), and the
  policy-engine reads it to configure/validate its receiving ALS server. Shared keys
  (`server_port`, TLS, `max_message_size`/`max_header_limit`) must agree on both ends;
  Envoy-sender-only keys (`buffer_*`, `grpc_request_timeout`) are ignored by the engine.
- collector activation — **derived on both sides** from the consumer flags
  (`IsCollectorEnabled() = analytics.enabled || traffic_logging.enabled`): the controller
  injects the system policy + ALS sink and the engine starts the ALS server when it is true.
  There is no `collector.enabled` key.
- `traffic_logging.enabled` / `masked_headers` — **policy-engine** config, consumed by the
  log publisher. Note `enabled` only *arms* the publisher; whether a given API is logged is
  decided per-API by the marker (`event.TrafficLog`) from the attached `log-message` policy
  with `enableTrafficLogging: true`.
- `log-message` **policy with `enableTrafficLogging: true`** (per-API attachment) —
  data-plane opt-in: its `OnRequestHeaders` stamps the `analytics_data["traffic_log"]`
  marker read back in `prepareAnalyticEvent`.

## 4a. Configuration changes & backward compatibility

Relative to the previous release, this PR **adds** the `[collector]`, `[collector.als]`, and
`[traffic_logging]` sections and turns `[analytics]` into a consumer of an **implicit**
collector (the collector has no `enabled` flag — it activates whenever a consumer is on).
The only pre-existing (released) `[analytics]` keys affected are the payload-capture flags
and the ALS transport tuning, which are **deprecated and migrated** onto `[collector]`.

**Which side reads each section:** `[collector]` capture flags split by side — the
controller owns header capture (`send_*_headers`), the engine owns body-attach
(`send_*_body`); `[collector.als]` is read by **both**; collector *activation* is derived on
both sides from the consumer flags; `[traffic_logging]` (including its output caps) is
**engine-only**, except `[traffic_logging].enabled`, which the controller also reads so it
can activate the collector when only traffic logging is on.

### Added

| Key | Read by | Purpose |
|---|---|---|
| `[collector].send_request_body` / `send_response_body` | both | Capture (controller) + attach (engine) request/response bodies (in full). Default `false`. |
| `[collector].send_request_headers` / `send_response_headers` | controller | Capture all request/response headers. Default `false`. |
| `[collector.als].*` | both | ALS transport tuning (one shared section — see §4) |
| `[traffic_logging].enabled` | both | Arm the stdout traffic-logging publisher (engine); activate the collector (controller) |
| `[traffic_logging].masked_headers` | engine | Output-side header value redaction |
| `[traffic_logging].max_payload_size` | engine | Truncate the logged payload (`0` = no limit); output-side, publisher-only |

### Deprecated & migrated (from `[analytics]`)

These released `[analytics]` keys still work: they are migrated onto `[collector]` at load
time with a `slog.Warn`, so existing configs keep functioning unchanged.

| Deprecated key | Migrated onto | Notes |
|---|---|---|
| `analytics.grpc_event_server.*` (controller) | `collector.als.*` | Whole-struct migration; only applied when `[collector.als]` is still at its default **and** `analytics.enabled = true`, so an explicit `[collector.als]` is never clobbered, and a traffic-logging-only deployment never re-applies a stale analytics override |
| `analytics.access_logs_service.*` (engine) | `collector.als.*` | Same whole-struct migration, same `analytics.enabled` guard |
| `analytics.allow_payloads` | `collector.send_request_body` + `collector.send_response_body` | Directional flags win over `allow_payloads` |
| `analytics.send_request_body` / `send_response_body` | `collector.send_*_body` | — |

### Backward-compatibility verdict

- **Deprecated `[analytics]` keys — backward compatible.** The keys above are migrated onto
  `[collector]` at load with a deprecation warning; nothing an operator set previously stops
  working.
- **Implicit collector — upgrades are zero-touch, and there is no `collector.enabled` key.**
  A previously-valid config that sets `analytics.enabled = true` keeps working: the collector
  activates automatically because a consumer is on (`IsCollectorEnabled`). A stale
  `collector.enabled = true` left in an old config is harmless — the key is no longer bound,
  so koanf ignores it. Nothing an operator must edit on upgrade.

## 5. Open items / future considerations

- Automated end-to-end test (containerized run) is not yet in place; covered manually.
- No `max_headers` / allow-list cap on header capture (all captured headers are attached);
  add if header cardinality becomes a concern.
- Cross-publisher masking is out of scope: masking/redaction is a per-publisher presentation
  concern, not a collector guarantee. The collector hands every publisher the same raw
  captured data; if Moesif is enabled alongside traffic logging, it receives unmasked
  headers regardless of `traffic_logging.masked_headers` or a policy's `maskedHeaders`. If
  cross-publisher masking is ever needed, it would have to move to capture time.

## Change Log

| Date | Change |
|---|---|
| 2026-07-02 | Initial version — PR "Add Access Logging support". Introduced the shared **`[collector]`** capture pipeline (request/response header & body capture, plus ALS transport tuning under **`[collector.als]`**) as a prerequisite for its consumers. `[analytics]` (Moesif) became an explicit consumer; its released capture/transport keys (`allow_payloads`, `send_*_body`, `grpc_event_server`/`access_logs_service`) are deprecated and migrated onto `[collector]` with warnings (§4a). Added **`[traffic_logging]`**, a stdout JSON consumer that is **per-API**: the `log-message` policy stamps a `traffic_log` marker that gates and shapes the emitted line — including a `fields` include/exclude projection and header masking — enriched with ALS-derived latencies. |
| 2026-07-03 | Simplified the collector config. **Removed `collector.enabled`** — the collector is now **implicit**, derived from whether a consumer is enabled (`IsCollectorEnabled() = analytics.enabled \|\| traffic_logging.enabled`); the controller gained a minimal `[traffic_logging].enabled` binding so it activates the collector when only traffic logging is on. **Renamed the `log-message` activation param** from `destination: inline\|access-log` to the boolean **`enableTrafficLogging`** (default `false`; `true` ≡ the old `access-log`). Per-API stdout logging is now a two-way AND (publisher + policy). |
| 2026-07-03 | Added **`customProperties`** to `log-message` (traffic-logging mode): user-defined key→value pairs, later renamed to **`labels`** (see 2026-07-07), resolved from `$ctx:`-prefixed request-context references and emitted on the log line. Resolved values are baked into the `traffic_log` marker. |
| 2026-07-03 | Expanded the request-context `$ctx:` surface with five more request-phase scalars: `request.vhost`, `request.id`, `api.kind`, `api.operation_path`, and `project.id`. The inter-policy `Metadata` map and all response-phase fields remain intentionally unexposed (the latter is a hard phase limit — the marker is stamped at request-header time). |
| 2026-07-03 | Audited the `$ctx:` surface against the latest SDK (`sdk/core` v0.2.16). Bumped `log-message` from v0.2.4 → v0.2.16 and exposed `$ctx:auth.token_id` (jwt `jti`, added in v0.2.15). |
| 2026-07-07 | Header capture flags (`collector.send_request_headers` / `send_response_headers`) now default to **`false`**, matching body capture — nothing is captured until an operator opts in. Renamed `customProperties` → **`labels`** and dropped the separate per-flow `excludeHeaders` param in favor of a single per-API **`maskedHeaders`** list (merged with the global `traffic_logging.masked_headers`); dropping a header entirely (rather than redacting it) is done via a dotted `fields.exclude` path. `TrafficLogDirective` gained `MaskedHeaders`. |
| 2026-07-07 | Removed the per-request `directiveCache` from `analytics.go`: the marker's resolved `Labels` are request-specific, so caching the parsed `TrafficLogDirective` by raw JSON string risked leaking one request's labels onto another that happened to produce the same cache key. `parseTrafficLogDirective` now parses the marker fresh on every request. |
| 2026-07-07 | Log publisher field projection (`applyFieldsProjection`) now shallow-decodes the marshaled line into `map[string]json.RawMessage` instead of a full `map[string]interface{}`, and only decodes/re-encodes the specific nested objects a dotted `fields` path references — avoiding a deep unmarshal/remarshal of the whole event on every projected line. |
| 2026-07-07 | Renamed the `log-message` **`labels`** param to **`properties`** (and the emitted top-level log-line key, the marker key, `TrafficLogDirective.Properties`, and the policy's `buildProperties`) — a straight rename of the concept, no behavioral change. `fields` dotted paths are now `properties.<key>`. |
| 2026-07-07 | Gave the traffic log its own **microsecond** latencies block (`dto.TrafficLogLatencies`: `durationUs`, `requestMediationLatencyUs`, `responseMediationLatencyUs`, `backendLatencyUs`), computed from the ALS `CommonProperties` timepoints at full precision via a new `toUs` in `analytics.go`. It no longer embeds the Moesif-oriented `dto.Latencies` (milliseconds), so the Moesif-oriented `*Latency` fields no longer leak into the traffic-log line and Moesif's millisecond units are unchanged. Dropped the now-unused millisecond `Duration`/`BackendProcDuration`/`ResponseProcDuration` fields (and `Get/SetDuration`) from `dto.Latencies`. |
