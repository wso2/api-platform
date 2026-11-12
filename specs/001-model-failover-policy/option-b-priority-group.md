# Option B — Model failover using the Priority Group cluster specifier

**Requires upgrading the Envoy data plane to 1.40.0.**

Feature 001-model-failover-policy · 2026-10-23 · Companion document: [Option A](./option-a-override-host.md)

---

## 1. What we are trying to do

Identical to Option A, and worth restating because the goal is what both options are judged against.

An application calls our gateway instead of calling OpenAI, Azure or Bedrock directly. When the model
it asked for is rate-limited or broken, the gateway quietly tries the next model on a list and
returns *that* answer. The application never sees the failure and never writes retry code.

The same three difficulties apply: the next attempt goes to a different vendor with different
credentials and format; the order is decided per request by whichever routing policy is attached; and
our policy chain must not re-run for every attempt.

## 2. The same Envoy fact still shapes everything

Envoy's **downstream chain** (where our policies live) runs **once per client request**. Its
**upstream chain** runs **once per attempt**. On a retry, our policies do not run again.

So the division of labour is unchanged from Option A:

| Work | Where | How often |
|---|---|---|
| Decide the order of targets | Downstream chain (our policy) | Once |
| Pick which target this attempt uses | Envoy | Per attempt |
| Apply that provider's key and format | Upstream chain | Per attempt |

What changes is **what Envoy picks between**. Option A makes Envoy pick between *endpoints inside one
cluster*. Option B makes Envoy pick between *clusters*. That single difference removes most of
Option A's awkwardness.

## 3. The Envoy pieces we use, in plain terms

### 3.1 Clusters stay exactly as they are

We keep one cluster per provider — OpenAI, Azure, Bedrock — each with its own TLS settings, its own
credentials, its own timeouts. **Nothing is restructured.** There is no failover cluster, no internal
listener, no transport socket trickery. This is the main prize of Option B.

### 3.2 Cluster specifier plugin — choosing the cluster per attempt

Normally a route names one cluster, decided once. A **cluster specifier plugin** replaces that fixed
name with a small piece of logic that decides which cluster a request goes to.

**Priority Group** is such a plugin. It organises the candidate clusters into an ordered list of
named **groups**, and picks the group by **attempt number**. From the Envoy 1.40.0 release notes:

> *"The group is selected based on the attempt count of the request (the initial attempt uses the
> first group, the first retry uses the second group, and so on, staying on the last group once the
> attempts go past the end of the list) and the target cluster is then selected from the group based
> on the cluster weights."*

So group 1 serves the first attempt, group 2 the first retry, and so on. For us, each group is simply
"the provider to use at this rank".

### 3.3 Per-request override — how our order gets in

The configured group order is only a default. The release notes continue:

> *"The groups can be overridden per request by an optional dynamic metadata namespace."*

**Dynamic metadata** is a scratch pad attached to a request that filters write and Envoy reads. Our
policy writes the whole ordered group list there, and Envoy uses it for that request alone.

The shape is a list of groups, each naming clusters and weights:

```
priority_groups:
  - name: rank-1
    clusters: [ { cluster_name: azure-cluster,   weight: 1 } ]
  - name: rank-2
    clusters: [ { cluster_name: openai-cluster,  weight: 1 } ]
  - name: rank-3
    clusters: [ { cluster_name: bedrock-cluster, weight: 1 } ]
```

That is the ranking `azure → openai → bedrock`, expressed as data, for this request only. The next
request can carry a completely different order.

**A bonus Option A cannot express:** a group may hold *several* clusters with weights, meaning "at
this rank, spread load across these two providers, and only drop to the next rank if both fail."

### 3.4 `refresh_cluster_on_retry` — mandatory, easy to forget

By default Envoy picks the cluster once and reuses it for retries. The plugin needs it re-picked each
attempt. The release notes are explicit:

> *"This must be used together with `refresh_cluster_on_retry` to ensure the target cluster is
> re-selected on every attempt and the request is retried in a different priority group."*

If we forget this flag, **the mechanism silently does nothing** — every retry returns to the same
provider and failover appears broken with no error anywhere. It must be set, and a test must assert
it is set.

### 3.5 Retry policy — unchanged from Option A

Same settings, same meanings:

- `retry_on` including `retriable-status-codes`
- `retriable_status_codes` — e.g. `429, 500, 502, 503, 504`
- `num_retries` — matched to the number of groups minus one
- `per_try_timeout` and `per_try_idle_timeout`

> **The same trap applies.** `rate_limited_retry_back_off` honours the provider's `Retry-After` and
> would delay our move to a *different* provider by up to 300 seconds. We leave it unset.

### 3.6 Health works normally

Each group names real clusters, and once a cluster is chosen its own load balancer picks an endpoint
with normal health handling — outlier detection, health checks, all of it behave as they do
everywhere else in the gateway.

This is a quiet but significant difference. In Option A, Envoy's chooser bypasses health entirely, so
*all* health enforcement falls to our policy. Here our policy still tracks suspension — that is a
product behaviour with its own thresholds and escalating backoff — but Envoy's own health machinery works
underneath it as a genuine second layer.

### 3.7 The upstream filter — per-attempt work

Unchanged from Option A: it runs on every attempt and attaches the right provider's credentials and
request format. Identifying the provider is simpler here, because the attempt went to that provider's
own cluster.

It must still read the request body **as our policies left it** — masked, guardrailed — never an
earlier copy.

## 4. What our policy does (once, on the way in)

1. Ask the routing policy for its preference order, if one is attached.
2. Append our own configured targets that are not already in that order.
3. Remove any target currently suspended by our health tracking.
4. Write the result as an ordered list of priority groups into dynamic metadata.

Note what is missing compared with Option A: no translating targets into IP addresses. We name
clusters, which is what we already know.

## 5. A request, step by step

**Everything healthy:**

1. Request arrives. Policy chain runs once, writes groups `azure → openai → bedrock`.
2. Attempt 1: the plugin takes group 1 and routes to `azure-cluster`.
3. Upstream chain attaches the Azure key and converts the request to Azure's format.
4. Azure answers 200. Client gets it.

**Azure is rate-limited:**

1. Same through step 3.
2. Azure answers **429** — retriable, so Envoy retries.
3. **Our policy chain does not run again.** Because `refresh_cluster_on_retry` is set, the cluster is
   chosen afresh.
4. Attempt 2: the plugin takes group 2 and routes to `openai-cluster`.
5. Upstream chain attaches the OpenAI key and converts to OpenAI's format.
6. OpenAI answers 200. One response reaches the client.

## 6. Configuration shape

```yaml
# On the route — instead of naming one cluster
cluster_specifier_plugin: failover-groups

# Plugin definition
name: failover-groups
typed_config:
  "@type": type.googleapis.com/envoy.extensions.router.cluster_specifiers.priority_group.v3.PriorityGroupClusterSpecifier
  override_metadata_namespace: wso2.failover      # where our policy writes the per-request order
  priority_groups:                                # default order, used if the policy writes nothing
  - name: rank-1
    clusters: [ { cluster_name: openai-cluster,  weight: 1 } ]
  - name: rank-2
    clusters: [ { cluster_name: azure-cluster,   weight: 1 } ]

retry_policy:
  retry_on: "5xx,connect-failure,reset,retriable-status-codes"
  retriable_status_codes: [429, 500, 502, 503, 504]
  num_retries: 2
  per_try_timeout: 30s
  per_try_idle_timeout: 10s
  refresh_cluster_on_retry: true        # MANDATORY — without it nothing fails over
  # rate_limited_retry_back_off deliberately NOT set
```

Provider clusters are untouched and are not shown — that is the point.

## 7. What must be proven before we build

| # | Question | If the answer is no |
|---|---|---|
| 1 | Does the per-request metadata override actually drive the group order? | The option fails |
| 2 | Does `refresh_cluster_on_retry` cause a genuinely different cluster per attempt? | Failover silently does nothing |
| 3 | Can the upstream chain tell which provider it landed on? | We cannot apply the right credentials |
| 4 | Does the upstream chain see the body our policies already modified? | Masking and guardrails would silently lapse — blocking |
| 5 | Does Envoy 1.40.0 pass our full integration suite? | The upgrade is blocked |

Questions 3 and 4 are shared with Option A. Question 5 is the new one, and it is an upgrade question
rather than a design question.

## 8. Known limitations

**Running past the end of the list repeats the last group.** Where Option A falls back to round robin
over everything, this plugin stays on the final group — so a surplus attempt re-tries the last
provider rather than a random one. Better, but still a reason to match `num_retries` to the group
count.

**A silent failure mode exists.** Forgetting `refresh_cluster_on_retry` produces no error, no warning
and no failover. It must be covered by a test, not by care.

**The release is new.** Envoy 1.40.0 was published on 2026-10-13. The plugin has had little time in
the field, and we would be early adopters.

## 9. Cost and risk summary

| | |
|---|---|
| Envoy upgrade needed | **Yes — 1.39.0 → 1.40.0** across every gateway deployment |
| Library upgrade needed | **Yes** — our pinned `go-control-plane` has neither the plugin proto nor `refresh_cluster_on_retry` |
| New moving parts | Cluster specifier plugin, upstream filter, policy. **No failover cluster, no internal listeners** |
| Biggest risk | The data-plane upgrade itself, on a release ten days old |
| Secondary risk | Silent no-op if `refresh_cluster_on_retry` is missed |
| What disappears | Internal listener addressing, the undocumented address-matching reliance, round-robin overrun, Envoy ignoring health |

**In one line:** the mechanism fits our existing cluster model exactly and removes Option A's riskiest
dependency, but it costs a data-plane upgrade to a very recent release.

## 10. How the two options differ, briefly

| | Option A — Override Host | Option B — Priority Group |
|---|---|---|
| Envoy picks between | Endpoints of one new cluster | Our existing clusters |
| Provider clusters | Reached via internal listeners | Used directly, untouched |
| Riskiest unknown | Internal listener addressing is undocumented | The 1.40.0 upgrade |
| Health handling | Envoy ignores it; policy is the only layer | Normal cluster health plus our policy |
| Overrun behaviour | Round robin over all endpoints | Repeats the last group |
| Load spreading within one rank | Not possible | Weighted clusters in a group |
| Silent failure mode | Writing too few addresses | Forgetting `refresh_cluster_on_retry` |
| Upgrade required | None | Envoy and control-plane library |

Both deliver the same product behaviour. The choice is between **shipping on today's data plane with
an undocumented dependency**, and **upgrading the data plane to get a mechanism built for this shape
of problem**.
