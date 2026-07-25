# Rule: JavaScript (Express/Passport) Authentication and Authorization Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` related to authentication middleware, Passport.js strategies, JWT verification (`jose`, `jsonwebtoken`), session handling, route protection, and multi-tenant data access via Sequelize. This is the JavaScript counterpart to `authentication_authorization.md` (Go).

---

## JS-AUTH-001: Fail-Closed Authentication

### Severity

Critical

### Description

Every authentication middleware and Passport strategy must terminate the request immediately on failure. Missing `return` before `next()` in error branches, or calling `next()` without error propagation, creates silent bypasses where unauthenticated requests reach protected handlers.

### Rationale

Express middleware chains continue to the next handler unless execution is explicitly halted. An omitted `return` after a `res.status(401)` call in async code, or after `next()` in a Passport `done(null, false)` path without a `failureRedirect`/`failWithError`, allows the request to fall through to protected route handlers.

### Non-Compliant Code

```js
// BAD: Missing return — execution falls through after 401
function authMiddleware(req, res, next) {
  const token = req.headers['authorization'];
  verifyToken(token, (err) => {
    if (err) {
      res.status(401).json({ error: 'unauthorized' }); // No return — falls through
    }
    next(); // Executes even when err was truthy
  });
}

// BAD: Passport strategy calls next() after done(null, false)
passport.use(new CustomStrategy(async (req, done) => {
  const user = await User.findOne({ where: { email: req.body.email } });
  if (!user) done(null, false); // Missing return — may continue
  done(null, user);
}));
```

### Compliant Code

```js
// GOOD: Explicit return halts execution on every failure path
function authMiddleware(req, res, next) {
  const token = req.headers['authorization'];
  verifyToken(token, (err) => {
    if (err) {
      return res.status(401).json({   // return prevents fall-through
        error: 'unauthorized',
        message: 'Invalid or expired credentials.',
      });
    }
    next();
  });
}

// GOOD: Passport strategy uses return on every done() path
passport.use(new CustomStrategy(async (req, done) => {
  try {
    const user = await User.findOne({ where: { email: req.body.email } });
    if (!user) return done(null, false); // return terminates this branch
    return done(null, user);
  } catch (err) {
    return done(err);
  }
}));
```

---

## JS-AUTH-002: Strict Asymmetric JWT Verification

### Severity

Critical

### Description

JWT verification using `jose` or `jsonwebtoken` must explicitly restrict the allowed signing algorithm to an asymmetric algorithm (`RS256`, `RS384`, `RS512`, `PS256`, `EdDSA`). Symmetric algorithms (`HS256`, `HS384`, `HS512`) and the `none` algorithm must be rejected. Never accept the algorithm from the token header without an explicit allowlist.

### Rationale

An attacker can forge a JWT signed with the server's RSA public key using HMAC-SHA256 (treating the public key as the HMAC secret) if symmetric algorithms are not rejected, completely bypassing signature verification.

### Non-Compliant Code

```js
// BAD (jsonwebtoken): No algorithm restriction — accepts whatever the token header claims
const decoded = jwt.verify(tokenStr, publicKey);

// BAD (jose): Missing algorithms option — defaults to accepting anything
const { payload } = await jwtVerify(tokenStr, publicKey);

// BAD: Accepts 'none' algorithm — signs nothing, verifies nothing
const decoded = jwt.verify(tokenStr, '', { algorithms: ['none'] });
```

### Compliant Code

```js
// GOOD (jose — preferred, already in project dependencies)
import { createRemoteJWKSet, jwtVerify } from 'jose';

const JWKS = createRemoteJWKSet(new URL(process.env.JWKS_URI));

async function verifyJwt(tokenStr) {
  const { payload } = await jwtVerify(tokenStr, JWKS, {
    algorithms: ['RS256', 'RS384', 'RS512', 'PS256'],  // Explicit asymmetric-only allowlist
    issuer: process.env.JWT_ISSUER,
    audience: process.env.JWT_AUDIENCE,
  });
  return payload;
}

// GOOD (jsonwebtoken — if used in legacy code paths)
const decoded = jwt.verify(tokenStr, publicKey, {
  algorithms: ['RS256', 'RS384', 'RS512'],  // Explicit allowlist; 'none' is never present
});
```

---

## JS-AUTH-003: Secure Token Handling and Logging

### Severity

Medium

### Description

Raw JWT strings, passwords, session tokens, or OAuth client secrets must never appear in Winston log output. When a token identifier is needed for log correlation, mask it to show only a short prefix and suffix.

### Rationale

Log aggregation systems (e.g., ELK, Loki, Application Insights — which is configured in this project) persist log lines. A full token in a log line becomes an attack surface if logs are leaked, and Application Insights forwards them to external telemetry.

### Non-Compliant Code

```js
// BAD: Full token in log output, forwarded to Application Insights
logger.warn(`Failed to verify token: ${req.headers['authorization']}`);

// BAD: Logging the raw password on failure
logger.error(`Login failed for ${req.body.email} with password ${req.body.password}`);
```

### Compliant Code

```js
// utils/maskSensitive.js
function maskToken(token) {
  if (!token || token.length <= 8) return '[MASKED]';
  return `${token.slice(0, 4)}...${token.slice(-4)}`;
}

function maskPassword(_password) {
  return '[REDACTED]';
}

// routes/auth.js — GOOD: Only masked values reach the logger
logger.warn('Token verification failed', {
  token: maskToken(req.headers['authorization']),
  reason: err.code,  // jose error codes like 'ERR_JWT_EXPIRED' are safe to log
});

logger.error('Login failed', {
  email: req.body.email,
  password: maskPassword(req.body.password), // Never log the actual password
});
```

---

## JS-AUTH-004: Routing and Path Traversal Protection

### Severity

High

### Description

Route path matching for authentication bypass decisions must operate on normalized, decoded paths. Raw `req.url` or `req.path` values can contain URL-encoded traversal sequences (`%2F`, `%2E%2E`) that confuse naive `startsWith` prefix guards.

### Rationale

A request to `//public/%2e%2e/private/secret` can bypass a guard checking `req.path.startsWith('/public/')` on some middleware stacks because Express normalizes `req.path` but `req.url` retains the encoded form.

### Non-Compliant Code

```js
// BAD: Raw string prefix check on req.url — bypassable with encoded traversals
app.use((req, res, next) => {
  if (req.url.startsWith('/public/')) {
    return next(); // Skips auth — bypassable via an encoded traversal sequence
  }
  authMiddleware(req, res, next);
});
```

### Compliant Code

```js
// GOOD: Use Express router groups to scope middleware structurally,
// not string matching on raw URLs.
const publicRouter = express.Router();
publicRouter.get('/health', healthHandler);
publicRouter.get('/docs', docsHandler);
app.use('/public', publicRouter); // Express normalizes path before matching

const protectedRouter = express.Router();
protectedRouter.use(authMiddleware);         // Applied to ALL routes in this router
protectedRouter.get('/profile', profileHandler);
protectedRouter.delete('/users/:id', deleteUserHandler);
app.use('/api', protectedRouter);

// NOTE: Do NOT apply a second decodeURIComponent() pass to req.path — Express already
// decodes it. A double-decode can be exploited via double-encoded traversal sequences.
// Prefer router group scoping (above) as the only safe pattern.
```

---

## JS-AUTH-005: Multi-Tenant Isolation (Anti-Privilege Escalation)

### Severity

Critical

### Description

Sequelize queries that access or mutate tenant-scoped data must include the `organizationId` (or equivalent tenant scope) sourced from the verified JWT claims stored in `req.user`, never from user-supplied request parameters, body fields, or query strings.

### Rationale

A user can craft a request with a different `org_id` in the query string or body to read or delete another tenant's data (Insecure Direct Object Reference). The only trustworthy tenant identifier is the one extracted from the signed JWT by `authMiddleware`.

### Non-Compliant Code

```js
// BAD: orgId is taken from user-controlled query parameter
async function deleteUserHandler(req, res) {
  const orgId = req.query.org_id;     // Attacker-controlled
  const userId = req.params.userId;

  await User.destroy({ where: { id: userId, organizationId: orgId } });
  res.status(204).send();
}
```

### Compliant Code

```js
// middleware/auth.js — GOOD: authMiddleware injects verified claims into req.user
async function authMiddleware(req, res, next) {
  try {
    const token = (req.headers['authorization'] || '').replace(/^Bearer\s+/, '');
    const { payload } = await jwtVerify(token, JWKS, {
      algorithms: ['RS256', 'RS384', 'RS512'],
      issuer: process.env.JWT_ISSUER,
    });
    req.user = {
      id: payload.sub,
      organizationId: payload.org_id,  // From JWT — not from the request
      roles: payload.roles ?? [],
    };
    next();
  } catch (err) {
    logger.warn('Auth failed', { reason: err.code });
    return res.status(401).json({
      error: 'unauthorized',
      message: 'Invalid or expired credentials.',
    });
  }
}

// routes/users.js — GOOD: tenant scope is always from req.user
async function deleteUserHandler(req, res, next) {
  try {
    const { organizationId } = req.user;  // From verified JWT
    if (!organizationId) {
      return res.status(403).json({ error: 'forbidden' });
    }

    const userId = req.params.userId;

    // Query is strictly sandboxed to the authenticated tenant
    const deleted = await User.destroy({
      where: { id: userId, organizationId },
    });

    if (!deleted) return res.status(404).json({ error: 'not_found' });
    res.status(204).send();
  } catch (err) {
    next(err);
  }
}
```

---

## JS-AUTH-006: HTTP Method Case-Insensitive Normalization

### Severity

High

### Description

HTTP method strings sourced from user input — API definitions, OpenAPI spec submissions, policy configurations, and access control exception lists — must be normalized to uppercase with `.toUpperCase()` at the earliest point of extraction, before any comparison, object key construction, or route/policy configuration.

### Rationale

RFC 7231 defines HTTP methods as case-sensitive and standard methods (`GET`, `POST`, etc.) are uppercase. However, user-supplied method values (e.g., from OpenAPI spec path keys, API policy bodies, or exception lists) may arrive in any case. Two classes of exploit are possible when normalization is missing:

1. **Access control bypass:** Security registries (deny lists, scope maps, exception sets) are built from one code path while incoming request methods come from another. If one path stores `"get"` and the other stores `"GET"`, object property lookups and `Set.has()` calls silently miss, causing deny rules to never fire.
2. **Route and policy mismatch:** Express router method matching (`.get()`, `.post()`) and OpenAPI operation lookups use lowercase keys by convention. If a user-supplied method is compared directly without normalization, the lookup silently returns `undefined`, causing the operation's policy or schema to be skipped entirely.

The JavaScript counterpart of the Go rule (GO-AUTH-006) — same exploit class, same fix, different syntax.

### Non-Compliant Code

```js
// BAD: OpenAPI path method keys from user spec compared without normalization
const HTTP_METHODS = new Set(['get', 'post', 'put', 'delete', 'patch']);
const operations = Object.keys(pathItem)
  .filter(method => HTTP_METHODS.has(method));  // 'GET' silently excluded from Set

// BAD: Exception/deny-list method stored without normalization
const deniedMethods = exceptionList.map(ex => ex.method);  // May be 'get', 'Get', 'GET'
if (deniedMethods.includes(incomingMethod)) { /* may silently miss */ }

// BAD: req.method checked with mixed-case string literal
if (req.method === 'options') {   // Express sets req.method to uppercase — always misses
  return res.sendStatus(204);
}

// BAD: Method from user-submitted config compared case-sensitively
if (operationMethod === 'POST') { /* fails if user submitted 'post' */ }
```

### Compliant Code

```js
// GOOD: Normalize OpenAPI path method keys at extraction — filter then uppercase
const HTTP_METHODS = new Set(['get', 'post', 'put', 'delete', 'patch']);
const operations = Object.keys(pathItem)
  .filter(method => HTTP_METHODS.has(method.toLowerCase()))  // Accept any case on input
  .map(method => ({
    method: method.toUpperCase(),  // Store/compare as uppercase from here on
    schema: pathItem[method],
  }));

// GOOD: Deny-list normalized at build time
const deniedMethods = new Set(
  exceptionList.map(ex => ex.method.toUpperCase())  // Normalize once on ingestion
);
if (deniedMethods.has(incomingMethod.toUpperCase())) { /* always matches */ }

// GOOD: req.method from Express is always uppercase — compare with uppercase literals
if (req.method === 'OPTIONS') {   // Correct — Express guarantees uppercase
  return res.sendStatus(204);
}

// GOOD: Normalize user-submitted method at ingestion before any comparison or storage
function normalizeMethod(raw) {
  const upper = raw.toUpperCase();
  const VALID = new Set(['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']);
  if (!VALID.has(upper)) {
    throw Object.assign(new Error(`Invalid HTTP method: ${raw}`), { statusCode: 400 });
  }
  return upper;
}

// In route/policy builders:
const method = normalizeMethod(userInput.method);  // Normalized; safe for all downstream use
```

> **Verification Checklist before outputting code:**
> * Is every method string from a user-submitted body, OpenAPI spec key, or policy config wrapped in `.toUpperCase()` at the point of extraction? (If no, add normalization there.)
> * Does `req.method` appear in comparisons against lowercase string literals (e.g., `'options'`, `'get'`)? (Express always uppercases `req.method` — the literal must be uppercase too.)
> * Are deny-list or scope-map lookups using a `Set` or object keyed on normalized (uppercase) methods? (A mixed-case key silently misses every lookup.)
> * Does any OpenAPI path-method key filtering use `.toLowerCase()` for the `Set.has()` check and `.toUpperCase()` for the stored/compared value? (Both are needed — filter input in lowercase, store output in uppercase.)

---

## JS-AUTH-007: Deny-by-Default Authorization on Admin/Internal Routes

### Severity

Critical

### Description

Every admin-only or internal-only Express route must perform an explicit role/scope check inside its handler (or a middleware wrapping that specific router), independent of router-group placement (JS-AUTH-004) or JWT signature validity (JS-AUTH-002). Router scoping and valid-token checks establish *authentication*; they do not by themselves establish that this specific caller is *authorized* for this specific privileged operation.

### Rationale

The JavaScript counterpart of `authentication_authorization.md` GO-AUTH-007 — same exploit class, same fix, different syntax. This failure mode appears repeatedly at critical severity across API management platforms: JWT algorithm-confusion bypasses, self-registered users obtaining elevated tokens via shared Key Managers, unauthenticated access to System REST APIs, and registration endpoints issuing tokens without access control. In each case, a valid, correctly-signed token was treated as sufficient to reach a privileged operation, when it should only have been treated as sufficient to identify the caller.

### Non-Compliant Code

```js
// BAD: Router scoping (JS-AUTH-004) authenticates the caller, but the handler
// never checks that this specific authenticated user holds the admin role
// required for this specific operation.
const adminRouter = express.Router();
adminRouter.use(authMiddleware); // Only proves the token is valid — not that the role fits
adminRouter.post('/tenants/:id/suspend', async (req, res, next) => {
  await Tenant.update({ suspended: true }, { where: { id: req.params.id } });
  res.status(204).send();
});
app.use('/admin', adminRouter);
```

### Compliant Code

```js
// GOOD: Explicit, per-route scope check inside the handler (or a small
// middleware factory) — deny-by-default, independent of router placement.
function requireScope(scope) {
  return (req, res, next) => {
    if (!req.user?.scopes?.includes(scope)) {
      // Generic 403 — do not reveal which scope was expected.
      return res.status(403).json({ error: 'forbidden' });
    }
    next();
  };
}

const adminRouter = express.Router();
adminRouter.use(authMiddleware);
adminRouter.post(
  '/tenants/:id/suspend',
  requireScope('admin:tenant:suspend'), // Explicit scope required for THIS operation
  async (req, res, next) => {
    try {
      await Tenant.update({ suspended: true }, { where: { id: req.params.id } });
      res.status(204).send();
    } catch (err) {
      next(err);
    }
  }
);
app.use('/admin', adminRouter);
```

> **Verification Checklist before outputting code:**
> * Does an admin/internal route rely solely on `authMiddleware`/router scoping (JS-AUTH-004) with no additional scope/role check for the specific operation? (If yes, add `requireScope`-style enforcement in the handler chain.)
> * Does a self-registration, Dynamic Client Registration, or other "low trust" flow ever mint a token whose scopes reach an admin route? (If yes, cap the issuable scopes for that flow independently of downstream route checks.)

---

## JS-AUTH-008: Parameterized Sequelize Queries for Administrative Data Access

### Severity

Critical

### Description

Every Sequelize query built from request input must use parameter binding — `where` clause objects, `sequelize.escape`, or bound `replacements`/`bind` in `sequelize.query` — never raw template-literal interpolation of a request value into a `sequelize.query` string.

### Rationale

The JavaScript counterpart of GO-AUTH-008. Authenticated SQL injection in Admin REST APIs is an exploitable bug class: an administrator manipulating database queries can exfiltrate data or disrupt availability. Sequelize's query builder (`where: {...}`) already parameterizes automatically; the risk is entirely concentrated in `sequelize.query(rawSQL)` calls where a developer reaches for raw SQL (commonly for a dynamic sort/filter feature) and interpolates a value directly into the string.

### Non-Compliant Code

```js
// BAD: Request value interpolated directly into a raw SQL string.
async function searchTenantsHandler(req, res, next) {
  const { name } = req.query;
  const [tenants] = await sequelize.query(
    `SELECT id, name, status FROM tenants WHERE name LIKE '%${name}%'`
  ); // Injectable — filter can be defeated
  res.json(tenants);
}

// BAD: Dynamic sort column interpolated directly — still injectable even
// though it "looks like" metadata rather than a value.
async function listApisHandler(req, res, next) {
  const sortCol = req.query.sort;
  const [apis] = await sequelize.query(`SELECT * FROM apis ORDER BY ${sortCol}`);
  res.json(apis);
}
```

### Compliant Code

```js
// GOOD: Sequelize's query-builder `where` clause parameterizes automatically.
async function searchTenantsHandler(req, res, next) {
  try {
    const { name } = req.query;
    const tenants = await Tenant.findAll({
      where: { name: { [Op.like]: `%${name}%` } }, // Bound, not interpolated
      attributes: ['id', 'name', 'status'],
    });
    res.json(tenants);
  } catch (err) {
    next(err);
  }
}

// GOOD: If raw SQL is genuinely required, use bound `replacements` — never
// template-literal interpolation of the value itself.
async function searchTenantsRawHandler(req, res, next) {
  try {
    const [tenants] = await sequelize.query(
      'SELECT id, name, status FROM tenants WHERE name LIKE :name',
      { replacements: { name: `%${req.query.name}%` }, type: QueryTypes.SELECT }
    );
    res.json(tenants);
  } catch (err) {
    next(err);
  }
}

// GOOD: Dynamic sort column resolved against an explicit allowlist — binding
// cannot parameterize an identifier, so this is the only safe pattern for it.
const ALLOWED_SORT_COLUMNS = new Set(['name', 'createdAt', 'status']);

async function listApisHandler(req, res, next) {
  try {
    const sortCol = ALLOWED_SORT_COLUMNS.has(req.query.sort) ? req.query.sort : 'createdAt';
    const apis = await Api.findAll({ order: [[sortCol, 'ASC']] }); // sortCol is now a known-safe constant
    res.json(apis);
  } catch (err) {
    next(err);
  }
}
```

> **Verification Checklist before outputting code:**
> * Does any `sequelize.query()` call build its SQL string with a template literal or `+` concatenation of a request-derived value? (If yes, switch to the `where`-clause builder or bound `replacements`.)
> * Does a dynamic sort column, table name, or field list come from request input? (Binding cannot parameterize identifiers — resolve against `ALLOWED_SORT_COLUMNS`-style allowlist instead.)
> * Is this query reachable from an admin/internal route? (Authenticated-admin-only reachability is not a reason to relax this directive — JS-AUTH-007 governs *who* can reach the handler; this directive governs how the handler builds its query regardless of who called it.)

---

## JS-AUTH-009: Token and Session Invalidation on Security-State Change

### Severity

High

### Description

Whenever a security-relevant state change occurs — logout, account lock, password reset, role change, or user/tenant deletion — all of that identity's active sessions and previously issued tokens must be actively revoked (destroy the Sequelize-backed session via `connect-session-sequelize`, and revoke/blacklist any outstanding JWTs), not merely left to expire naturally.

### Rationale

The JavaScript counterpart of GO-AUTH-009. Failure to revoke tokens on security-state changes is a recurring vulnerability pattern: session tokens not revoked when a session ends, tokens issued before a lock remaining valid after it, role removal not invalidating previously issued tokens, and tokens for users not revoked on password reset or disablement. In each case the authentication check was correct at issuance time; nothing revoked the token when the state that justified it later changed.

### Non-Compliant Code

```js
// BAD: Locks the account but leaves the session store and any issued JWTs
// untouched — both remain valid until natural expiry.
async function lockAccountHandler(req, res, next) {
  try {
    await User.update({ status: 'locked' }, { where: { id: req.params.userId } });
    res.status(204).send();
    // Missing: destroy active sessions / revoke issued tokens for this user
  } catch (err) {
    next(err);
  }
}
```

### Compliant Code

```js
// GOOD: Revocation is part of the same operation, not a separate manual step.
// Sessions are stored via connect-session-sequelize — destroy them for this user;
// JWTs are checked against a revocation store (e.g. a `tokenVersion` column bumped
// on every security-state change, checked during verification).
async function lockAccountHandler(req, res, next) {
  try {
    const user = await User.findByPk(req.params.userId);
    if (!user) return res.status(404).json({ error: 'not_found' });

    await sequelize.transaction(async (t) => {
      // sequelize.literal issues a single atomic `tokenVersion = tokenVersion + 1`
      // UPDATE at the DB level — reading user.tokenVersion into JS and writing
      // back `+ 1` would race under concurrent requests and could reuse a version.
      await user.update(
        { status: 'locked', tokenVersion: sequelize.literal('tokenVersion + 1') },
        { transaction: t }
      );
      await Session.destroy({ where: { userId: user.id }, transaction: t }); // connect-session-sequelize table
    });

    res.status(204).send();
  } catch (err) {
    logger.error('Failed to lock account and revoke sessions', { userId: req.params.userId, reason: err.message });
    next(err);
  }
}

// middleware/auth.js — GOOD: JWT verification checks the token's embedded
// tokenVersion against the current value, so a token minted before a lock/role
// change/password reset stops working immediately, not at its own expiry.
async function authMiddleware(req, res, next) {
  try {
    const token = (req.headers['authorization'] || '').replace(/^Bearer\s+/, '');
    const { payload } = await jwtVerify(token, JWKS, { algorithms: ['RS256'] });
    const user = await User.findByPk(payload.sub);
    if (!user || user.status === 'locked' || user.tokenVersion !== payload.tokenVersion) {
      return res.status(401).json({ error: 'unauthorized', message: 'Invalid or expired credentials.' });
    }
    req.user = { id: user.id, organizationId: user.organizationId, scopes: payload.scopes ?? [] };
    next();
  } catch (err) {
    return res.status(401).json({ error: 'unauthorized', message: 'Invalid or expired credentials.' });
  }
}
```

> **Verification Checklist before outputting code:**
> * Does a handler change account status, role, or delete a user/tenant without also destroying that user's sessions and invalidating outstanding tokens (e.g. bumping `tokenVersion`)? (If yes, add revocation as part of the same transaction.)
> * Does `authMiddleware` validate a JWT's signature/expiry only, with no live check against the current account/token state? (If so, add a `tokenVersion`-style check so revocation actually takes effect before natural expiry.)
> * On a revocation/session-destroy failure, does the handler still respond with success? (It must not — treat revocation failure as an overall operation failure.)

---

## JS-AUTH-010: Redirect and Callback URL Allowlisting (Open Redirect Prevention)

### Severity

Medium

### Description

Any `res.redirect()` call whose target is derived from request input — a post-login `returnTo` query parameter, an OIDC `redirect_uri`, a logout redirect — must be validated against an explicit allowlist of registered destinations before use. Never rely on a substring/prefix check against the request value.

### Rationale

The JavaScript counterpart of GO-AUTH-010. Open redirect via weak callback URL validation, unvalidated redirect construction, and open redirects in logout flows are consistently used as phishing primitives precisely because the redirect originates from a trusted, recognizable domain.

### Non-Compliant Code

```js
// BAD: Same-host substring check — bypassable via a crafted lookalike/userinfo host.
app.get('/auth/callback', (req, res) => {
  const returnTo = req.query.returnTo || '/';
  if (returnTo.includes('portal.example.com')) {
    return res.redirect(returnTo);
  }
  res.redirect('/');
});
```

### Compliant Code

```js
// GOOD: Parsed-URL host comparison against an explicit allowlist — the same
// pattern OIDC redirect_uri validation already requires, applied uniformly.
const ALLOWED_REDIRECT_HOSTS = new Set(['portal.example.com', 'console.example.com']);

function safeRedirectTarget(raw) {
  if (typeof raw !== 'string' || raw.length === 0) return '/'; // Guards arrays/objects from parsed query strings (e.g. ?returnTo[]=x)
  let parsed;
  try {
    parsed = new URL(raw, 'https://portal.example.com'); // Base resolves relative paths safely
  } catch {
    return '/';
  }
  // Reject scheme-relative URLs ("//attacker.com") implicitly — URL parsing
  // normalizes them, and the host check below still applies.
  const isRelative = raw.startsWith('/') && !raw.startsWith('//');
  if (isRelative) return parsed.pathname + parsed.search;
  // Reject any userinfo component outright — "https://attacker.com@portal.example.com"
  // parses with a legitimate, allowlisted host but still carries a userinfo
  // segment that other URL parsers/clients along the redirect path may not
  // interpret identically. There is no legitimate reason for a redirect
  // target to carry credentials.
  if (parsed.username || parsed.password) {
    return '/';
  }
  if (parsed.protocol !== 'https:' || !ALLOWED_REDIRECT_HOSTS.has(parsed.host)) {
    return '/'; // Fall back to a safe default — never echo the rejected value back
  }
  return parsed.toString();
}

app.get('/auth/callback', (req, res) => {
  res.redirect(safeRedirectTarget(req.query.returnTo));
});
```

> **Verification Checklist before outputting code:**
> * Is a redirect target validated with `.includes()`/`.startsWith()` rather than parsing the URL and comparing `host` against an explicit allowlist? (Substring checks are bypassable — replace with `new URL()` + exact host match.)
> * Does the validated target allow a scheme-relative URL (`//attacker.com`) or userinfo trick (`https://trusted.com@attacker.com`) to pass? (Both must be rejected by the host-comparison logic, not assumed away.)
> * On rejection, does the handler redirect to a safe default rather than reflecting the rejected value back into an error page? (Reflecting it back risks reintroducing the XSS surface covered in `js-output-encoding-xss.md`.)

---

> **Verification Checklist before outputting code:**
> * Does every authentication error branch have a `return` before `next()` or `res.status()`? (If no, add `return`).
> * Does JWT verification include an explicit `algorithms` array containing only asymmetric algorithms? (If no, add the allowlist).
> * Is any raw token, password, or secret passed directly to `logger.*`? (If yes, apply `maskToken` / `maskPassword`).
> * Is route protection applied via Express router group scoping rather than raw `req.url` string matching? (If not, restructure to router groups).
> * Does every Sequelize query for tenant-scoped data use `organizationId` from `req.user`, not from `req.query`/`req.body`/`req.params`? (If not, source it from `req.user`).
> * Is every HTTP method string from user input normalized with `.toUpperCase()` before comparison, map/Set lookup, or policy registration? (If no, add normalization at the ingestion point.)
> * Does an admin/internal route check a specific scope/role beyond token validity (JS-AUTH-007)? Does every `sequelize.query()` call use bound `replacements` rather than string interpolation (JS-AUTH-008)? Does a lock/role-change/deletion handler revoke sessions and bump `tokenVersion` (JS-AUTH-009)? Is every `res.redirect()` target validated against a host allowlist (JS-AUTH-010)?
