# Option A — Model failover using Override Host

**No Envoy upgrade. Works on what we ship today.**

Feature 001-model-failover-policy · 2026-10-23 · Companion document: [Option B](./option-b-priority-group.md)

---

## 1. What we are trying to do

An application calls our gateway instead of calling OpenAI, Azure or Bedrock directly. When the
model it asked for is rate-limited or broken, we want the gateway to quietly try the next model on a
list and return *that* answer. The application should never see the failure and should never have to
write retry code.

Three things make this harder than an ordinary retry:

1. **The next attempt goes somewhere else.** A different model, often a different vendor, with
   different credentials and a different request format.
2. **The order is decided per request.** If a routing policy (round-robin, weighted, semantic) is
   attached, *it* decides the preference order for this particular request. We must follow that
   order, not a fixed list.
3. **It must be cheap.** The policy chain must not run again for every attempt, or we would
   double-charge quota and re-screen the same prompt.

## 2. The one Envoy fact that shapes everything

Envoy has two filter chains. Think of them as two checkpoints a request passes through.

- The **downstream chain** is where our policy engine lives. It runs **once per client request**.
- The **upstream chain** sits closer to the backend. It runs **once per attempt**.

Envoy's own documentation says it plainly:

> *"as the upstream HTTP filter chain is upstream of the router filter, it is run per each retry
> attempt allowing header manipulation per retry"*

So when a retry happens, our policies **do not run again**. Only the upstream chain does.

That gives us the division of labour for the whole design:

| Work | Where it happens | How often |
|---|---|---|
| Decide the order of targets | Downstream chain (our policy) | Once |
| Pick which target this attempt uses | Envoy's load balancer | Per attempt |
| Apply that provider's key and request format | Upstream chain | Per attempt |

## 3. The Envoy pieces we use, in plain terms

### 3.1 Cluster and endpoints

A **cluster** is Envoy's name for "a backend". Its **endpoints** are the actual addresses behind it.
Today we have one cluster per provider — an OpenAI cluster, an Azure cluster — each with its own TLS
settings and credentials.

For failover we add **one more cluster per API operation**: the *failover cluster*. Its endpoints are
the providers we may fail over between.

### 3.2 Load balancing policy — the part that chooses

Normally a load balancer picks an endpoint for you (round robin, least requests). We replace it with
a special one.

**Override Host** is a load balancing policy that does not choose at all. It reads a list of
addresses that someone else wrote onto the request, and uses them **in that order**:

> *"The request is forwarded to the first address and subsequent addresses are used for request
> retries... if an overridden host address is not present in the current endpoint set, it is skipped
> and the next found address is used."*

This is the heart of the design. **The order is data on the request, not configuration.** Our policy
writes `azure, openai, bedrock` for one request and `openai, bedrock` for the next, and Envoy follows
each one.

### 3.3 Dynamic metadata — the envelope the list travels in

**Dynamic metadata** is a scratch pad attached to a request that filters can write and Envoy can
read. Our policy writes the ordered address list there; Override Host reads it from there.

We use the namespace `envoy.lb` and the key `wso2.failover.targets`, with a value like:

```
10.0.3.11:443,10.0.7.4:443,10.0.9.2:443
```

### 3.4 Retry policy — the part that decides *when* to try again

Sitting on the route, the **retry policy** tells Envoy which failures are worth another attempt and
how many attempts are allowed:

- `retry_on` — the failure kinds: server errors, connection failures, and *retriable status codes*
- `retriable_status_codes` — our configured list, e.g. `429, 500, 502, 503, 504`
- `num_retries` — how many extra attempts; **must equal the list length minus one**
- `per_try_timeout` — how long one attempt may take
- `per_try_idle_timeout` — how long one attempt may go silent (matters for streaming responses)

> **Trap we must avoid.** There is a setting called `rate_limited_retry_back_off` that reads the
> provider's `Retry-After` header and waits that long before the next attempt — up to 300 seconds by
> default. That is right for retrying the *same* provider and completely wrong for us: we are moving
> to a *different* provider and must not wait out the first one's cooldown. We leave it unset
> deliberately.

### 3.5 `previous_hosts` — a guard, not a guarantee

A **retry host predicate** can veto a candidate. `previous_hosts` vetoes any address already tried in
this request. We configure it.

But it is not absolute. Envoy only re-picks a limited number of times, and then:

> *"the host that was last selected will be routed to"* — the vetoed one.

So the real protection against trying a target twice is the `num_retries` number being exactly right.
The predicate is a seatbelt, not the brakes.

### 3.6 Internal listeners — how a provider becomes an endpoint

Here is the awkward part. Override Host picks between **endpoints of one cluster**, but our providers
are **separate clusters**, each with its own TLS and API key. We do not want to throw that away.

An **internal listener** is a listener that exists only inside Envoy — nothing outside can reach it.
We give each provider one. The failover cluster's endpoints point at these internal listeners, and
behind each internal listener sits that provider's real cluster, unchanged.

The **`internal_upstream` transport socket** wraps that hop and carries extra information across it:

> *"allows passing additional structured state across the user space socket"*

We use it to carry `{model, provider}` through, so the upstream chain knows whose API key to use.

```
failover cluster
  endpoint 1 ──► internal listener A ──► OpenAI cluster   (own TLS, own key)
  endpoint 2 ──► internal listener B ──► Azure cluster    (own TLS, own key)
  endpoint 3 ──► internal listener C ──► Bedrock cluster  (own TLS, own key)
```

> **This is the weakest link in Option A.** The override list is documented as `IP:Port` addresses.
> An internal listener's address is not an `IP:Port` — it is a listener name. The Envoy code does a
> plain text lookup, so it *may* match on the internal address's text form, but no documentation
> promises this. A spike must prove it before we build on it. See §7.

### 3.7 The upstream filter — per-attempt work

Because this chain runs on every attempt, this is where we:

1. Read which provider this attempt landed on (from the metadata carried across the internal hop).
2. Attach that provider's credentials.
3. Translate the request into that provider's format.

It must read the request body **as our policies left it** — already masked, already guardrailed —
never an earlier copy. A comparable product shipped a bug here where a PII mask was silently dropped
on the retry.

## 4. What our policy does (once, on the way in)

1. Ask the routing policy for its preference order, if one is attached.
2. Append our own configured targets that are not already in that order.
3. Remove any target currently suspended by our health tracking.
4. Translate the remaining targets into addresses.
5. Write them, comma-separated, into dynamic metadata.

Then it gets out of the way. Everything after that is Envoy.

## 5. A request, step by step

**Everything healthy:**

1. Request arrives. Policy chain runs once, writes `azure, openai, bedrock`.
2. Override Host picks `azure` — the first entry.
3. Upstream chain attaches the Azure key and converts the request to Azure's format.
4. Azure answers 200. Client gets it. `openai` and `bedrock` were never contacted.

**Azure is rate-limited:**

1. Same as above through step 3.
2. Azure answers **429**. That is in our retriable list, so Envoy retries.
3. **Our policy chain does not run again.**
4. Override Host takes the next entry: `openai`.
5. Upstream chain attaches the OpenAI key and converts to OpenAI's format.
6. OpenAI answers 200. The client receives one normal response and never learns two attempts happened.

## 6. Configuration shape

```yaml
# The failover cluster — one per API operation
name: chat-api-failover
load_balancing_policy:
  policies:
  - typed_extension_config:
      name: envoy.load_balancing_policies.override_host
      typed_config:
        override_host_sources:
        - metadata: { key: envoy.lb, path: [ { key: wso2.failover.targets } ] }
        fallback_policy:            # used only if our policy wrote nothing
          policies:
          - typed_extension_config:
              name: envoy.load_balancing_policies.round_robin
load_assignment:
  endpoints:
  - lb_endpoints:
    - endpoint: { address: { envoy_internal_address: { server_listener_name: provider-openai } } }
      metadata: { filter_metadata: { wso2: { model: gpt-4o, provider: openai } } }
    # ... one per provider

# On the route
retry_policy:
  retry_on: "5xx,connect-failure,reset,retriable-status-codes"
  retriable_status_codes: [429, 500, 502, 503, 504]
  num_retries: 2                    # exactly chain length - 1
  per_try_timeout: 30s
  per_try_idle_timeout: 10s
  retry_host_predicate:
  - name: envoy.retry_host_predicates.previous_hosts
  host_selection_retry_max_attempts: 5
  # rate_limited_retry_back_off deliberately NOT set
```

## 7. What must be proven before we build

| # | Question | If the answer is no |
|---|---|---|
| 1 | Does Override Host follow our written order across retries? | The whole option fails |
| 2 | **Can an internal listener endpoint be selected from the override list?** | Providers must be addressed another way — resolved IPs, or a narrower launch scope |
| 3 | Can the upstream chain tell which provider it landed on? | We cannot apply the right credentials |
| 4 | Does the upstream chain see the body our policies already modified? | Masking and guardrails would silently lapse — blocking |

Question 2 is the one that can force a redesign.

## 8. Known limitations

**Health is not checked by Envoy.** The code that finds an address carries a developer note —
*"Validate that host health status did not change"* — meaning it does not. An unhealthy provider that
appears in our list will still be tried. Suspension is therefore **entirely our policy's job**: a
suspended target must simply be left out of the list we write.

**Running past the end of the list does not fail.** When the list is used up, Envoy falls back to
round robin over all endpoints — possibly one already tried. This is why `num_retries` must be
exactly right, and why it is a real footgun if our policy writes fewer addresses than Envoy is
allowed to attempt.

**Envoy cannot tell us which endpoint it chose.** A newer Envoy field reports the selected endpoint
back into metadata; the version we pin does not have it. We work around this by carrying
`{model, provider}` across the internal hop ourselves.

## 9. Cost and risk summary

| | |
|---|---|
| Envoy upgrade needed | **None** — everything used is in Envoy 1.35.0+, we run 1.39.0 |
| Library upgrade needed | **None** — all protos are in the `go-control-plane` version we pin |
| New moving parts | Failover cluster, internal listener per provider, upstream filter, policy |
| Biggest risk | Internal listener addressing (§7 question 2) is undocumented behaviour |
| Secondary risks | Health ignored by Envoy; overrun falls back to round robin |

**In one line:** it works with what we ship today, but it leans on one behaviour Envoy does not
promise, and we carry the health logic ourselves.
