# Test-console egress policy

Where the Test console's relay is allowed to connect.

The browser never names a URL. It names an API and a gateway; the BFF asks
Platform API — with the caller's own token — which address that is, then dials
it. This package is what bounds that dial.

---

## Two layers

**Layer 1 — resolution.** Platform API answers with the caller's token, so a
user who cannot read this API gets nothing. The gateway must be one the API is
actually deployed to. This is the primary containment.

**Layer 2 — this policy.** A gateway's endpoint is tenant-settable data, so
layer 1 can only promise "a gateway someone registered", not "a sensible
address". This layer says what a gateway address is allowed to *look like*,
and is what holds if layer 1 is ever weakened.

---

## The model

Two rules. That's all of it.

1. **`deny` always wins.**
2. **`allow` narrows when set, and is ignored when empty.**

There is no setting that re-opens something `deny` closed.

```toml
[api_control_plane.test_console.egress]

# A target must match every list you set. Empty list = dimension unrestricted.
allow_hosts = ["*.gw.svc.cluster.local"]
allow_cidrs = ["10.42.0.0/16"]
allow_ports = [443, 9443]

# Never reachable, whatever allow says.
# Takes CIDRs and the names "private", "loopback", "cgnat".
deny = ["10.42.9.0/24", "loopback"]
```

| Key | Default | Effect |
|---|---|---|
| `allow_hosts` | unset | Narrows by hostname. Exact host, or one leading `*.` |
| `allow_cidrs` | unset | Narrows by resolved address |
| `allow_ports` | unset | Narrows by port |
| `deny` | unset | Always refused. CIDRs and/or group names |

**Empty means "no constraint", not "allow nothing."** All four lists ship
empty, so a deployment that configures nothing keeps working.

### Deny groups

| Name | Covers |
|---|---|
| `private` | RFC 1918 **and** IPv6 unique-local (`fc00::/7`) |
| `loopback` | `127.0.0.0/8`, `::1` |
| `cgnat` | `100.64.0.0/10` |

Prefer the group names over hand-written ranges — `private` includes
`fc00::/7`, which a CIDR list almost always forgets.

---

## Always refused

Not configurable. Nothing reaches these.

| Range | Why |
|---|---|
| `169.254.0.0/16`, `fe80::/10` | Link-local — where the cloud metadata endpoint `169.254.169.254` lives |
| `0.0.0.0`, `::` | Unspecified; the OS can read it as "local host" |
| multicast, broadcast | Never a meaningful HTTP peer |
| `2002::/16` (6to4), `64:ff9b::/96` (NAT64) | Embed an IPv4 address and route to it — `2002:a9fe:a9fe::` reaches the metadata endpoint without being link-local or IPv4 |

---

## Decision order

Per resolved address, top to bottom:

```
1. always-refused ranges            → DENY
2. allow_cidrs set, address outside → DENY
3. deny match                       → DENY
                                    → ALLOW
```

Host and port are checked separately, before the dial.

Because there is no widening list, no entry can move a target *up* this list.
That's the design: an earlier version had a carve-out evaluated first, which
meant naming the link-local range in it re-opened the metadata endpoint. The
fix wasn't to re-order that list — it was to not have one.

---

## Where each check runs

| Check | When | Why there |
|---|---|---|
| host, port | at resolution, before dialing | Neither depends on DNS, so checking early gives a specific `403` with the reason in the log, instead of a vague `502` |
| address | inside the dial | The dial resolves and connects in one step, so a name cannot resolve to an approved address during the check and a different one at connect time (DNS rebinding) |

The dial function lives in this package rather than using `netguard.DialContext`
directly, for two reasons: netguard has no "address must be inside one of these
ranges" concept (`allow_cidrs`), and its error names neither the address nor the
reason, which leaves a refused dial indistinguishable from a gateway that is
simply down.

It does **not** restate netguard's checks. Each resolved address is passed to
`netguard.Validate` as an IP literal — which costs no DNS query, since the
resolver short-circuits literals — so the categorical decisions stay in
netguard and only the local narrowing is added on top. `httpkit` is unmodified.

---

## When something is refused

The caller always gets the same generic answer:

```
403  {"status":"error","code":"TARGET_NOT_ALLOWED","message":"this gateway cannot be tested from the portal"}
```

The real reason is logged server-side only — telling the caller which check
fired would map the tenant's network for them:

```
WARN test console egress refused host=gw.internal addr=169.254.169.254 reason=link-local address
WARN test console target refused by egress policy err="gateway host is not in the configured allow_hosts: gw.other.net"
```

At startup the effective policy is logged once, so you can confirm what is in
force without re-reading config:

```
INFO test-console egress policy policy="allow_hosts=1 allow_cidrs=1 allow_ports=2 deny_cidrs=1 deny_groups=loopback"
```

---

## Refused at startup

The process will not boot on any of these, rather than running with a policy
that is weaker — or more broken — than it reads:

- a `deny` entry that is neither a CIDR nor a known group name (a typo must
  not silently widen the policy by being ignored)
- `0.0.0.0/0` or `::/0` in `allow_cidrs` — a constraint that constrains nothing
- `0.0.0.0/0` or `::/0` in `deny` — it disables the console behind a generic
  `403`; use `[test_console] enabled = false` instead
- a port outside 1–65535
- an over-broad `allow_hosts` pattern: bare `*`, `*.com`, an interior wildcard

---

## Examples

**Kubernetes — gateways on ClusterIPs**

```toml
allow_hosts = ["*.gw.svc.cluster.local"]
allow_ports = [8443, 9443]
deny        = ["10.42.9.0/24"]   # the control plane's own subnet
```

**Cloud — all gateways public**

```toml
allow_hosts = ["gw.api.example.com"]
allow_ports = [443]
deny        = ["private", "loopback", "cgnat"]
```

**Local development** — defaults. Nothing to set.

---

## Why ports matter

Private space has to stay dialable, or gateways on ClusterIPs stop working.
That leaves the **port** as the only thing distinguishing a gateway from
anything else HTTP in the same subnet — Elasticsearch `:9200`, etcd `:2379`,
kubelet `:10250`, a Spring actuator. If you can state your gateway ports, set
`allow_ports`. It is the highest-value line in this file.

