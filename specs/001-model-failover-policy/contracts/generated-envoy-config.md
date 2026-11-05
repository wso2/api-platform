# Contract: Generated Proxy Configuration

**Producer**: `gateway-controller` xDS translator
**Consumer**: Envoy data plane (**`envoyproxy/envoy:v1.40.0`**)
**Mechanism**: `priority_group` cluster specifier plugin — [research.md](../research.md) §10

Ordering is **request data**, never generated configuration: the downstream policy pass writes a
ranked list of priority groups once, and Envoy selects the group by attempt number.

---

## 1. Version requirements

| Pin | Required | Why |
|---|---|---|
| Envoy image | **≥ v1.40.0** | `envoy.router.cluster_specifier_plugin.priority_group` ships in 1.40.0 |
| `go-control-plane/envoy` | **≥ v1.40.0** | plugin proto + `RetryPolicy.refresh_cluster_on_retry`; absent in v1.37.0 |
| Go toolchain | **≥ 1.26.0** | required by `go-control-plane/envoy@v1.40.0` |

Validated on CI: the 1.40.0 trial and a 1.39.0 baseline both passed the full gateway integration
suite.

## 2. Per-provider clusters — unchanged

The existing one-cluster-per-provider model is used **as is**. Each keeps its own endpoints, TLS
context, credentials and timeouts. No failover cluster, no internal listeners, no
`internal_upstream` transport socket. This is the main structural win over the rejected Override
Host design.

## 3. Route configuration

```yaml
route_config:
  cluster_specifier_plugins:
  - extension:
      name: failover-groups
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.router.cluster_specifiers.priority_group.v3.PriorityGroupClusterSpecifier
        override_metadata_namespace: wso2.failover     # where the policy writes the per-request order
        priority_groups:                               # default order when the policy writes nothing
        - name: rank-1
          clusters: [ { cluster_name: <provider-cluster-1>, weight: 1 } ]
        - name: rank-2
          clusters: [ { cluster_name: <provider-cluster-2>, weight: 1 } ]

  virtual_hosts:
  - routes:
    - route:
        cluster_specifier_plugin: failover-groups
        retry_policy:
          retry_on: "5xx,connect-failure,reset,retriable-status-codes"
          retriable_status_codes: [ <failureConditions.statusCodes> ]
          num_retries: <chain length - 1>
          per_try_timeout: <perAttemptTimeoutMs>
          per_try_idle_timeout: <bounds a stalled stream>
          refresh_cluster_on_retry: true          # MANDATORY — see §5
```

**`retriable-status-codes` must appear in `retry_on`** or `retriable_status_codes` is never
evaluated.

**`reset` is redundant but retained.** Envoy retries an upstream reset under `5xx` independently of
`reset` (`retry_state_impl.cc`: *"Currently we count an upstream reset as a `5xx` (since it will
result in one)"*), and `gateway-error` behaves the same. So a post-request reset — a provider that
accepted the prompt, began work, then died — is retried, deliberately (T030l). Replacing `reset`
with `reset-before-request` would change nothing while reading as though it had; genuinely
narrowing it would mean dropping `5xx`/`gateway-error`, which would end failover for the most
common real provider failure.

**`rate_limited_retry_back_off` MUST NOT be set.** It honours the failing provider's `Retry-After`
(default ceiling 300s) before attempting a *different* provider — backwards for failover. Keep
`retry_back_off.base_interval` small and assert the absence in a test.

## 4. Runtime data contract

Written **once** by the downstream pass, before the first attempt:

| Item | Value |
|---|---|
| Dynamic metadata namespace | `wso2.failover` |
| Shape | `priority_groups`: ordered list of `{ name, clusters: [ { cluster_name, weight } ] }` |
| Contents | The resolved chain, suspended targets already removed |

Attempt N takes group N. Attempts cannot exceed the list under the generated configuration:
`num_retries` is the chain length minus one, so the sequence ends at the last published group and
no surplus attempt is made — verified at runtime by `failover-core.feature` (2026-11-05), which
supersedes POC probe B4's prediction that the last group would repeat. A group may hold several
clusters with weights, which expresses load spreading *within* a rank — not possible under the
rejected design.

**Security**: gateway-internal. MUST NOT be derivable from client input, and any inbound header or
metadata of the same name MUST be stripped at ingress.

## 5. Two settings that silently break the feature

Both were proven necessary by POC probes, not inferred:

- **`refresh_cluster_on_retry: true`** — without it, probe B3 showed Envoy retrying the **same
  failing provider three times** and returning 500 while the healthy next target received nothing.
  No error, no warning: a silent no-op *and* a 3× load amplifier on a failing backend.
- **Route cache cleared after the metadata write** — without it, probe B2 showed the **first**
  attempt using the *configured* group order, honouring the per-request override only from the retry
  onward. The policy-engine kernel already sets `ClearRouteCache: true`, so this holds today, but it
  must be asserted by a test rather than assumed.

## 6. Per-attempt processing

Runs in the **upstream** filter chain (`envoy.filters.http.upstream`), the only chain Envoy
re-invokes on a retry:

1. Identify which provider cluster this attempt selected.
2. Apply that provider's credentials and request translation.
3. Read the **current buffered body** — never a router-phase snapshot (FR-012a).

The downstream chain does not re-run, so side-effecting policies are once-per-client-request by
construction (FR-012d).

## 6b. Access-log sink for grading

A route attaching failover makes the listener emit the gRPC access-log cluster and sink even when
analytics and traffic logging are both off (their default). This is the only channel carrying the
outcome of an attempt that committed `200` and then failed mid-body; ext_proc has no completion
or stream-reset notification, so the policy's response phase cannot distinguish that from a
response served whole (FR-030n/o).

| Property | Value |
|---|---|
| Gate | collector enabled **OR** a failover route is present — one predicate (`needsALSStream`) for both the cluster and the sink |
| Filter, failover-only case | `response_flag_filter` over `common/collector.FailoverGradingResponseFlags` |
| Filter, collector enabled | unchanged (health-path + ignore-prefix); the unfiltered sink already carries flags, so no second sink is emitted |
| Flags admitted | `UPE`, `UC`, `UR`, `UT`, `UF`, `URX`, `UMSDR` — upstream only |
| Flags deliberately excluded | `DC` (the *client* disconnected — not the target's fault), `UH`/`NR` (no attempt reached an upstream) |

Healthy traffic matches no flag, so it is never serialized, buffered or sent: the stream costs
nothing until a target actually fails. Envoy validates each flag string against its own closed
list at load, so a typo fails the config push rather than silently matching nothing.

## 7. Invariants

| Invariant | Source |
|---|---|
| Ordering obeyed exactly as published | FR-021 |
| Downstream chain executes once per client request | FR-012d |
| Each target attempted at most once | FR-010 — `num_retries` = chain length − 1 |
| Suspended targets never attempted | FR-025 — omitted from the published groups |
| A budget larger than the published chain makes no extra attempt, and never reaches an excluded target | `failover-core.feature` (2026-11-05); supersedes POC probe B4, which predicted a repeat of the last group |
| No failover after response headers/body reach the client | FR-013 |
| Exhaustion returns the feature's sterile error | FR-014 |
| Config scoped to the attaching operation; shared provider clusters unmutated | FR-030c/d |
| `NormalizePath` / `MergeSlashes` / `PathWithEscapedSlashesAction` retained | `go-control-plane-xds-security.md` d.6 |
| Failover provisions its own access-log sink; grading never depends on analytics being enabled | FR-030o — §6b |
| The flag set the sink admits is the same one the engine decodes | FR-030o — one definition in `common/collector` |

## 8. Rejected generation shapes

Plain retries cannot leave the selected cluster. Aggregate clusters select by health, not attempt
index. Composite clusters are statically ordered. Override Host selects *endpoints*, requiring
internal listeners and an undocumented address form — and POC probe A3 showed it routing to a
deliberately excluded target on overrun. Full comparison in [research.md](../research.md) §2 and
[poc-results.md](../poc-results.md).
