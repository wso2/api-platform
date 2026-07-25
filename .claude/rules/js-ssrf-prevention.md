# Rule: JavaScript (Node.js/Express) Server-Side Request Forgery (SSRF) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that makes an outbound HTTP request (via `axios` or `node-fetch`, both already project dependencies) whose target URL, in whole or in part, comes from user input — an API spec import-by-URL feature, a "Try It" / test-invoke console, a proxy target override, or any tenant-configurable endpoint. This is the JavaScript counterpart to `ssrf-prevention.md` (Go), which covers the same class of bug in the Go gateway/event-gateway WebSub delivery path.

---

## Directives

### 1. Treat Every User-Supplied URL as Untrusted

* **No Direct Request:** Never pass a URL sourced from `req.body`, `req.query`, `req.params`, or a stored-but-user-editable config value directly into `axios.get/post` or `fetch()` without validation.
* **Scheme Allowlist:** Only allow `https:` (and `http:` only where explicitly required for local/dev callback testing, gated by config). Reject `file:`, `data:`, `gopher:`, and any non-HTTP(S) scheme.
* **Resolve and Check the IP, Not Just the Hostname String:** A hostname can pass a naive string check (`!host.includes('localhost')`) and still resolve to a private or metadata address. Resolve via `dns.lookup()` and validate the resulting IP before connecting — and be aware that the IP can change between the check and the actual request (DNS rebinding), so the check must be as close to the request as possible.

### 2. Block Private, Loopback, Link-Local, and Cloud Metadata Addresses

* **Deny Reserved Ranges:** Reject destinations resolving to `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`, `169.254.0.0/16` (this includes `169.254.169.254`, the AWS/GCP/Azure metadata address), `::1`, and `fe80::/10`.
* **Enforce at the Socket, Not Just at Input Validation:** `axios`/`node-fetch` both support a custom `lookup` function (Node's `dns.lookup` signature) — use it to reject resolution to a denied IP at connection time, not only when the raw URL string is first received. This closes the gap a pure input-validation check leaves open to DNS rebinding.
* **Disable or Re-Validate Redirects:** Set `maxRedirects: 0` on `axios` (or intercept `fetch`'s `redirect: 'manual'`) and, if a hop is genuinely required, validate the `Location` target with the exact same allowlist before manually following it — one hop at a time, capped at a small maximum.

### 3. Bound Every Outbound Request

* **Timeouts:** Always set a short `timeout` (e.g. 5–10s) on `axios` requests and use `AbortController` with `fetch`/`node-fetch`. A malicious or slow target must not be able to hang a request-handling worker indefinitely.
* **Response Size Cap:** Stream the response and abort once a byte ceiling is exceeded (e.g. via `maxContentLength`/`maxBodyLength` on `axios`), rather than buffering an unbounded body in memory — this doubles as a defense against a malicious target trying to exhaust memory.

### 4. Never Let Response Data Trigger a Second Unvalidated Request

* **No Chained Fetches from Untrusted Content:** If a fetched resource (e.g. an imported OpenAPI/WSDL spec) contains further URLs (server URLs, `$ref` links, external docs links), do not automatically dereference them server-side without applying the exact same validation as directive 1–2. Render/link them for the user to open client-side instead, where the browser's own origin and network policies apply.
* **Strip Hop-by-Hop Trust:** When proxying a response back to the browser (e.g. a spec-import preview), do not forward upstream response headers verbatim — this both prevents SSRF-adjacent header smuggling and avoids leaking infrastructure details (see `js-error-handling.md`, Vendor Header Abstraction).

### 5. Configuration Over Hardcoding

* **Externalize the Denylist:** Ship the private-range denylist as a safe built-in default, but let operators extend it via config for environment-specific internal ranges. Never let a per-tenant setting widen the denylist without an explicit, off-by-default administrative flag.
* **Generic Rejection Response:** On rejection, respond with a generic `400`/`422` and a message that does not reveal the resolved IP or the specific reason (which would help an attacker map internal topology); log the concrete reason server-side only.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```js
// BAD: Fetches a user-supplied URL directly — classic SSRF via a spec-import feature.
app.post('/api/specs/import-from-url', async (req, res) => {
  const response = await axios.get(req.body.specUrl); // No scheme/IP validation at all
  res.json(response.data);
});

// BAD: String-only hostname check — bypassable via DNS rebinding or an
// alternate IP-literal encoding of a denied address.
function isSafeUrl(raw) {
  const { hostname } = new URL(raw);
  return hostname !== 'localhost' && !hostname.startsWith('127.');
}

// BAD: "Try it" console forwards an admin-supplied test URL with no validation
// and no bound on response size or redirects.
app.post('/api/try-it', async (req, res) => {
  const result = await fetch(req.body.targetUrl, { redirect: 'follow' });
  res.send(await result.text());
});
```

### Best Practice (What to Generate)

```js
// utils/ssrfGuard.js — GOOD: IP-range denylist enforced via the dns lookup hook,
// not merely as a one-time string check on the raw URL.
const dns = require('node:dns');
const net = require('node:net');
const ipaddr = require('ipaddr.js'); // small, well-audited CIDR-matching library

const DENIED_CIDRS = [
  '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16',
  '127.0.0.0/8', '169.254.0.0/16', '0.0.0.0/8', '::1/128', 'fe80::/10',
  'fc00::/7', // IPv6 unique-local addresses — the RFC 1918 analogue for IPv6
  '::/128', // IPv6 unspecified address
];

function isDenied(ip) {
  let addr = ipaddr.parse(ip);
  // Normalize IPv4-mapped IPv6 addresses (::ffff:a.b.c.d) to their IPv4 form
  // before matching — otherwise a mapped private/loopback address slips past
  // the IPv4 CIDRs above because addr.kind() reports 'ipv6', not 'ipv4'.
  if (addr.kind() === 'ipv6' && addr.isIPv4MappedAddress()) {
    addr = addr.toIPv4Address();
  }
  return DENIED_CIDRS.some((cidr) => {
    const [range, bits] = ipaddr.parseCIDR(cidr);
    return addr.kind() === range.kind() && addr.match(range, bits);
  });
}

// Custom `lookup` — passed to axios/node-fetch so the check happens at the moment
// of DNS resolution for the actual connection, closing the DNS-rebinding gap.
function guardedLookup(hostname, options, callback) {
  dns.lookup(hostname, options, (err, address, family) => {
    if (err) return callback(err);

    // Node's Happy Eyeballs / autoSelectFamily dual-stack connection attempts
    // (default-on since Node 20) invoke a custom `lookup` with `{ all: true }`,
    // in which case `address` is an array of { address, family } objects, not
    // a single address string — validate every candidate before allowing any.
    if (options && options.all) {
      const addresses = address;
      const denied = addresses.some((entry) => isDenied(entry.address));
      if (denied) {
        return callback(new Error('Destination is not allowed'));
      }
      return callback(null, addresses);
    }

    if (isDenied(address)) {
      return callback(new Error('Destination is not allowed'));
    }
    callback(null, address, family);
  });
}

function assertAllowedScheme(raw, { allowHttp = false } = {}) {
  const { protocol } = new URL(raw);
  const allowed = allowHttp ? ['https:', 'http:'] : ['https:'];
  if (!allowed.includes(protocol)) {
    throw Object.assign(new Error('URL scheme is not allowed'), { statusCode: 422 });
  }
}

module.exports = { guardedLookup, assertAllowedScheme, isDenied };

// utils/ssrfGuard.test.js — GOOD: regression coverage for IPv4-mapped IPv6
// addresses, so a future refactor of isDenied cannot silently reopen this gap.
describe('isDenied', () => {
  test.each([
    ['::ffff:127.0.0.1', true],  // mapped loopback
    ['::ffff:10.0.0.5', true],   // mapped RFC 1918 private range
    ['::ffff:8.8.8.8', false],   // mapped public address stays allowed
  ])('treats mapped %s as denied=%s', (ip, expected) => {
    expect(isDenied(ip)).toBe(expected);
  });
});

// utils/safeHttpClient.js — GOOD: axios instance with dial-time IP validation,
// no automatic redirect-following, timeout, and a response-size ceiling.
const axios = require('axios');
const http = require('node:http');
const https = require('node:https');
const { guardedLookup } = require('./ssrfGuard');

const safeHttpClient = axios.create({
  timeout: 8000,
  maxRedirects: 0, // Directive 2: no silent redirect-following
  maxContentLength: 5 * 1024 * 1024, // 5 MiB response cap
  maxBodyLength: 5 * 1024 * 1024,
  httpAgent: new http.Agent({ lookup: guardedLookup }),
  httpsAgent: new https.Agent({ lookup: guardedLookup }),
});

module.exports = { safeHttpClient };

// routes/specs.js — GOOD: validated scheme, guarded client, bounded response,
// no forwarding of raw upstream headers.
const { safeHttpClient } = require('../utils/safeHttpClient');
const { assertAllowedScheme } = require('../utils/ssrfGuard');

app.post('/api/specs/import-from-url', async (req, res, next) => {
  try {
    assertAllowedScheme(req.body.specUrl); // Directive 1: scheme allowlist

    const response = await safeHttpClient.get(req.body.specUrl);

    // Directive 4: do not auto-dereference nested $ref/server URLs found in the
    // fetched spec — return them to the client for the user to review/open.
    res.json({ spec: response.data });
  } catch (err) {
    logger.warn('Spec import rejected', { reason: err.message }); // Internal detail only
    return res.status(422).json({
      error: 'invalid_request',
      message: 'The provided URL could not be used.', // Generic — no resolved-IP detail
    });
  }
});
```

---

> **Verification Checklist before outputting code:**
> * Does any `axios`/`fetch`/`node-fetch` call use a URL that originated from `req.body`/`req.query`/`req.params`/tenant config? (If yes, it must go through `safeHttpClient` with `guardedLookup`, never the bare library default.)
> * Is URL validation only a string check on the hostname (`includes('localhost')`, `startsWith('127.')`) with no IP-level, dial-time enforcement? (If yes, add the `dns.lookup`-hooked guard — string checks alone are bypassable via DNS rebinding and alternate IP encodings.)
> * Does the HTTP client used for user-triggered requests allow automatic redirects (`maxRedirects` unset or `redirect: 'follow'`)? (If yes, disable and re-validate each hop explicitly, or leave disabled if redirects aren't required.)
> * Is every such request bounded by both a timeout and a response-size cap? (If not, add both.)
> * Does the response to the caller on rejection reveal the resolved IP or specific validation reason? (If yes, generalize the message and log the reason internally only.)
