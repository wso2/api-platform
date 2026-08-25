# Rule: JavaScript (Node.js/Express) Server-Side Request Forgery (SSRF) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that makes an outbound HTTP request (`axios`/`node-fetch`) whose target URL, in whole or in part, comes from user input — a spec import-by-URL feature, a "Try It" console, a proxy target override, or any admin-configurable endpoint. JS counterpart to `ssrf-prevention.md` (Go), which covers the same bug class in the gateway/event-gateway WebSub delivery path.

## Directives

1. **Treat every user-supplied URL as untrusted.** Never pass a URL from `req.body`/`req.query`/`req.params`/admin config directly into `axios.get/post` or `fetch()`. Allowlist scheme to `https:` only (`http:` solely for gated local/dev testing) and reject `file:`, `data:`, `gopher:`. A hostname string check (`!host.includes('localhost')`) is insufficient — resolve via `dns.lookup()` and validate the IP, as close to the actual connection as possible, since DNS rebinding can change the answer between check and request.
2. **Block private, loopback, link-local, and metadata addresses at dial time — and up front for IP literals.** Deny `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`, `169.254.0.0/16` (covers the `169.254.169.254` cloud metadata address), `::1`, `fe80::/10`, `fc00::/7`. Enforce via a custom `lookup` function passed to `axios`/`node-fetch`'s agent (Node's `dns.lookup` signature) so the check happens at actual connection time, not just on the raw input string — closing the DNS-rebinding gap. Normalize IPv4-mapped IPv6 addresses (`::ffff:a.b.c.d`) to IPv4 before matching, or a mapped private/loopback address slips past IPv4-only CIDRs. The `lookup` hook alone is not sufficient: Node's `net` internals special-case a hostname that is already an IP literal (`net.isIP(host)`) and connect directly, without ever invoking `lookup` — so a target like `https://169.254.169.254/` reaches the socket with the denylist never consulted. Parse the URL once and additionally assert the resolved `url.hostname` isn't a denied IP literal *before* issuing the request, independent of the dial-time hook.
3. **Disable or re-validate redirects.** Set `maxRedirects: 0` on `axios` (or `redirect: 'manual'` for `fetch`); if a hop is genuinely needed, validate the `Location` target with the exact same scheme+IP checks before following it manually, one hop at a time, capped at a small maximum.
4. **Bound every outbound request.** Set a short `timeout` (5-10s) and use `AbortController` for `fetch`; cap response size via `maxContentLength`/`maxBodyLength` on `axios` rather than buffering an unbounded body — a slow or malicious target must not hang a worker or exhaust memory.
5. **Never let response data trigger a second unvalidated request.** If a fetched resource (e.g. an imported OpenAPI/WSDL spec) contains further URLs (server URLs, `$ref`s, external docs links), don't auto-dereference them server-side — return them to the client to open, where the browser's own origin policy applies. Don't forward upstream response headers verbatim when proxying back to the browser (see `js-error-handling.md`, Vendor Header Abstraction).
6. **Config over hardcoding, generic rejection response.** Ship the denylist as a safe built-in default, extensible via config for internal ranges; never let a deployment setting widen it without an explicit, off-by-default admin flag. On rejection, respond with a generic `400`/`422` that doesn't reveal the resolved IP or reason (helps attackers map internal topology) — log the concrete reason server-side only.
7. **No deferring a violation behind a code comment.** Never resolve a missing scheme/IP/redirect check by adding a `// TODO`/`FIXME`-style comment and shipping the request path anyway — a comment does not validate a destination. Fix it before merging, or raise the gap explicitly for an approved exception rather than leaving it annotated in the source.

## Example

```js
// BAD: direct fetch of a user-supplied URL — classic SSRF, no validation, follows redirects.
app.post('/api/specs/import-from-url', async (req, res) => {
  const response = await axios.get(req.body.specUrl);
  res.json(response.data);
});

// GOOD: scheme allowlist + dial-time IP guard + bounded client.
const dns = require('node:dns');
const net = require('node:net');
const http = require('node:http'), https = require('node:https');
const ipaddr = require('ipaddr.js');
const DENIED_CIDRS = ['10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16', '127.0.0.0/8',
  '169.254.0.0/16', '0.0.0.0/8', '::1/128', 'fe80::/10', 'fc00::/7', '::/128'];

function isDenied(ip) {
  let addr = ipaddr.parse(ip);
  if (addr.kind() === 'ipv6' && addr.isIPv4MappedAddress()) addr = addr.toIPv4Address(); // mapped addrs bypass IPv4 CIDRs otherwise
  return DENIED_CIDRS.some((cidr) => {
    const [range, bits] = ipaddr.parseCIDR(cidr);
    return addr.kind() === range.kind() && addr.match(range, bits);
  });
}

// Custom `lookup`: validated at the moment of DNS resolution, not just on the raw URL string.
// This covers hostnames — it does NOT cover IP literals, see assertAllowedHost below.
function guardedLookup(hostname, options, callback) {
  dns.lookup(hostname, options, (err, address, family) => {
    if (err) return callback(err);
    const candidates = options?.all ? address : [{ address }]; // Happy Eyeballs passes { all: true }
    if (candidates.some((c) => isDenied(c.address))) return callback(new Error('Destination is not allowed'));
    callback(null, address, family);
  });
}

// Node's net internals special-case a host that is already an IP literal
// (net.isIP(host)) and connect directly, WITHOUT ever calling the Agent's
// `lookup` hook — so "https://169.254.169.254/" would bypass guardedLookup
// entirely. This up-front check is what covers that case; a non-literal
// hostname is a no-op here since the dial-time hook handles it instead.
function assertAllowedHost(hostname) {
  const host = String(hostname || '').replace(/^\[|\]$/g, ''); // URL.hostname keeps brackets on IPv6 literals
  if (!net.isIP(host)) return; // not a literal — guardedLookup will validate it at dial time
  if (isDenied(host)) throw new Error('Destination is not allowed');
}

const safeHttpClient = axios.create({
  timeout: 8000,
  maxRedirects: 0,
  maxContentLength: 5 * 1024 * 1024,
  maxBodyLength: 5 * 1024 * 1024,
  httpAgent: new http.Agent({ lookup: guardedLookup }),
  httpsAgent: new https.Agent({ lookup: guardedLookup }),
});

app.post('/api/specs/import-from-url', async (req, res) => {
  try {
    const target = new URL(req.body.specUrl); // parsed once, reused for every check below
    if (target.protocol !== 'https:') throw new Error('scheme not allowed');
    assertAllowedHost(target.hostname); // literal-IP check — guardedLookup alone can't see this
    const response = await safeHttpClient.get(target.toString()); // guarded client — dial-time check for hostnames
    res.json({ spec: response.data }); // nested $refs returned as-is, not auto-dereferenced
  } catch (err) {
    logger.warn('Spec import rejected', { reason: err.message });
    res.status(422).json({ error: 'invalid_request', message: 'The provided URL could not be used.' });
  }
});
```

> **Verification Checklist before outputting code:**
> * Does any `axios`/`fetch` call use a request/admin-configured URL without going through `safeHttpClient` + `guardedLookup`?
> * Is URL validation only a hostname string check, with no IP-level, dial-time enforcement (and no IPv4-mapped IPv6 normalization)?
> * Is the target URL parsed once and its hostname checked via `assertAllowedHost` (or equivalent) before the request, so an IP literal can't reach the socket without `guardedLookup` ever being invoked?
> * Does the client auto-follow redirects (`maxRedirects` unset or `redirect: 'follow'`)?
> * Is every such request bounded by both a timeout and a response-size cap?
> * Does a fetched document's embedded URLs get auto-dereferenced server-side, or does the rejection response leak the resolved IP/reason?
> * Is a gap in this rule "resolved" by a `// TODO`/`FIXME`-style comment instead of an actual fix or an explicitly raised, approved exception?
