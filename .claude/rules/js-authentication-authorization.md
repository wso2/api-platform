# Rule: JavaScript (Express/Passport) Authentication and Authorization Standards

## Context & Scope

Apply whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` covering authentication middleware, Passport.js strategies, JWT verification (`jose`/`jsonwebtoken`), session handling, route protection, and multi-tenant Sequelize access. JS counterpart to `authentication_authorization.md` (Go) — each rule below pairs 1:1 with a `GO-AUTH-*` rule of the same exploit class.

---

## JS-AUTH-001: Fail-Closed Authentication — Critical (counterpart: GO-AUTH-001)

Express middleware chains continue to the next handler unless execution is explicitly halted. A missing `return` before/after `res.status(401)` in async code, or a Passport `done(null, false)` call with no `return`, lets the request fall through to protected handlers even though auth failed.

```js
// BAD: no return — falls through to next() even when err is truthy
function authMiddleware(req, res, next) {
  verifyToken(req.headers['authorization'], (err) => {
    if (err) res.status(401).json({ error: 'unauthorized' }); // missing return
    next();
  });
}

// GOOD: return halts execution on every failure path
function authMiddleware(req, res, next) {
  verifyToken(req.headers['authorization'], (err) => {
    if (err) {
      return res.status(401).json({ error: 'unauthorized', message: 'Invalid or expired credentials.' });
    }
    next();
  });
}
// Same applies to Passport strategies: return done(null, false) / return done(err), always.
```

---

## JS-AUTH-002: Strict Asymmetric JWT Verification — Critical (counterpart: GO-AUTH-002)

JWT verification must explicitly allowlist asymmetric algorithms (`RS256`/`RS384`/`RS512`/`PS256`/`EdDSA`) and reject symmetric (`HS*`) and `none`. Without this, an attacker can forge a token using the server's RSA public key as an HMAC secret, bypassing signature verification entirely.

```js
// BAD: no algorithms restriction — accepts whatever the token header claims
const { payload } = await jwtVerify(tokenStr, publicKey);

// GOOD: explicit asymmetric-only allowlist, plus issuer/audience checks
const { payload } = await jwtVerify(tokenStr, JWKS, {
  algorithms: ['RS256', 'RS384', 'RS512', 'PS256'], // never 'HS256' or 'none'
  issuer: process.env.JWT_ISSUER,
  audience: process.env.JWT_AUDIENCE,
});
```

---

## JS-AUTH-003: Secure Token Handling and Logging — Medium (counterpart: GO-AUTH-003)

Raw JWTs, passwords, session tokens, or secrets must never reach Winston log output — this project forwards logs to Application Insights, which is external telemetry. Mask to a short prefix/suffix when a correlation identifier is needed; redact passwords entirely.

```js
// BAD: full token/password in log output
logger.warn(`Failed to verify token: ${req.headers['authorization']}`);

// GOOD: mask before logging
const maskToken = (t) => (!t || t.length <= 8 ? '[MASKED]' : `${t.slice(0, 4)}...${t.slice(-4)}`);
logger.warn('Token verification failed', { token: maskToken(req.headers['authorization']), reason: err.code });
logger.error('Login failed', { email: req.body.email, password: '[REDACTED]' }); // never log the actual password
```

---

## JS-AUTH-004: Routing and Path Traversal Protection — High (counterpart: GO-AUTH-004)

Auth-bypass path decisions must not rely on raw `req.url`/`req.path` string prefix checks — encoded traversal sequences (`%2F`, `%2E%2E`) can defeat a naive `startsWith` guard because Express normalizes `req.path` but `req.url` retains the encoded form. Use structural router-group scoping instead.

```js
// BAD: string prefix check on raw req.url — bypassable via encoded traversal
app.use((req, res, next) => {
  if (req.url.startsWith('/public/')) return next(); // skips auth
  authMiddleware(req, res, next);
});

// GOOD: structural scoping — Express normalizes the path before matching
const publicRouter = express.Router();
publicRouter.get('/health', healthHandler);
app.use('/public', publicRouter);

const protectedRouter = express.Router();
protectedRouter.use(authMiddleware); // applies to every route below
protectedRouter.get('/profile', profileHandler);
app.use('/api', protectedRouter);
// Do NOT add a second decodeURIComponent() pass on req.path — Express already
// decodes it once; a double-decode reopens a double-encoded traversal bypass.
```

---

## JS-AUTH-005: Multi-Tenant Isolation (Anti-Privilege Escalation) — Critical (counterpart: GO-AUTH-005)

Sequelize queries touching tenant-scoped data must key on `organizationId` sourced from verified JWT claims in `req.user` — never from `req.query`/`req.body`/`req.params`. Trusting a request-supplied org ID lets an attacker swap it to read or delete another tenant's data (IDOR).

```js
// BAD: org id taken from user-controlled query parameter
const orgId = req.query.org_id; // attacker-controlled
await User.destroy({ where: { id: req.params.userId, organizationId: orgId } });

// GOOD: organizationId always comes from req.user, set by authMiddleware from JWT claims
// authMiddleware: req.user = { id: payload.sub, organizationId: payload.org_id, roles: payload.roles ?? [] };
async function deleteUserHandler(req, res, next) {
  const { organizationId } = req.user; // from verified JWT, not the request
  if (!organizationId) return res.status(403).json({ error: 'forbidden' });
  const deleted = await User.destroy({ where: { id: req.params.userId, organizationId } });
  if (!deleted) return res.status(404).json({ error: 'not_found' });
  res.status(204).send();
}
```

---

## JS-AUTH-006: HTTP Method Case-Insensitive Normalization — High (counterpart: GO-AUTH-006)

User-supplied HTTP method strings (OpenAPI spec keys, policy configs, exception lists) must be normalized with `.toUpperCase()` at the point of extraction, before any comparison, map/Set key build, or route/policy registration. Without it, a deny-list built from `"get"` silently misses an incoming `"GET"` (Express always uppercases `req.method`), and OpenAPI operation lookups keyed on lowercase can silently skip policy/schema enforcement.

```js
// BAD: mixed-case stored/compared — deny-list or lookup silently misses
const deniedMethods = exceptionList.map(ex => ex.method); // 'get', 'Get', 'GET' all possible
if (deniedMethods.includes(incomingMethod)) { /* may miss */ }
if (req.method === 'options') { /* Express always uppercases — this never matches */ }

// GOOD: normalize once at ingestion, compare/store uppercase everywhere after
function normalizeMethod(raw) {
  const upper = raw.toUpperCase();
  const VALID = new Set(['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']);
  if (!VALID.has(upper)) throw Object.assign(new Error('Invalid HTTP method'), { statusCode: 400 });
  return upper;
}
const deniedMethods = new Set(exceptionList.map(ex => ex.method.toUpperCase()));
if (deniedMethods.has(incomingMethod.toUpperCase())) { /* always matches */ }
if (req.method === 'OPTIONS') { /* correct */ }
```

---

## JS-AUTH-007: Deny-by-Default Authorization on Admin/Internal Routes — Critical (counterpart: GO-AUTH-007)

Router-group scoping (JS-AUTH-004) and JWT validity (JS-AUTH-002) establish only *authentication* — every admin/internal route must additionally perform an explicit scope/role check for that specific operation inside the handler chain, deny-by-default. This is the recurring root cause behind algorithm-confusion bypasses, elevated tokens from self-registration, and unauthenticated System-API access: a valid token being treated as sufficient for a privileged operation.

```js
// BAD: authMiddleware proves the token is valid, never that the role fits
adminRouter.use(authMiddleware);
adminRouter.post('/tenants/:id/suspend', suspendHandler); // any authenticated caller can reach this

// GOOD: explicit per-route scope check via a middleware factory
function requireScope(scope) {
  return (req, res, next) => {
    if (!req.user?.scopes?.includes(scope)) return res.status(403).json({ error: 'forbidden' }); // generic — don't reveal expected scope
    next();
  };
}
adminRouter.post('/tenants/:id/suspend', requireScope('admin:tenant:suspend'), suspendHandler);
```

---

## JS-AUTH-008: Parameterized Sequelize Queries for Administrative Data Access — Critical (counterpart: GO-AUTH-008)

Every Sequelize query built from request input must use parameter binding — `where` clause objects or bound `replacements`/`bind` — never template-literal interpolation into a `sequelize.query()` string. An authenticated admin performing SQLi is still a trust-boundary violation (exfiltration/availability impact), not a lower-risk case.

```js
// BAD: request value interpolated directly into raw SQL
const [tenants] = await sequelize.query(`SELECT * FROM tenants WHERE name LIKE '%${req.query.name}%'`);
// BAD: dynamic sort column interpolated — still injectable even as "metadata"
await sequelize.query(`SELECT * FROM apis ORDER BY ${req.query.sort}`);

// GOOD: query-builder where clause parameterizes automatically
await Tenant.findAll({ where: { name: { [Op.like]: `%${req.query.name}%` } } });
// GOOD: if raw SQL is required, use bound replacements — never interpolate the value
await sequelize.query('SELECT * FROM tenants WHERE name LIKE :name', { replacements: { name: `%${req.query.name}%` } });
// GOOD: identifiers (sort column, table) can't be bound — resolve against an allowlist instead
const ALLOWED_SORT_COLUMNS = new Set(['name', 'createdAt', 'status']);
const sortCol = ALLOWED_SORT_COLUMNS.has(req.query.sort) ? req.query.sort : 'createdAt';
await Api.findAll({ order: [[sortCol, 'ASC']] });
```

---

## JS-AUTH-009: Token and Session Invalidation on Security-State Change — High (counterpart: GO-AUTH-009)

Logout, account lock, password reset, role change, or user/tenant deletion must actively revoke that identity's sessions (`connect-session-sequelize`) and outstanding JWTs — not merely let them expire naturally. Otherwise a token issued before the state change remains valid until its own expiry, independent of the account's new state.

```js
// BAD: locks the account but never touches issued sessions/tokens — both remain valid
await User.update({ status: 'locked' }, { where: { id: req.params.userId } });

// GOOD: state change + tokenVersion bump + session destroy, atomically
await sequelize.transaction(async (t) => {
  // sequelize.literal issues an atomic DB-level increment — reading+incrementing in JS would race
  await user.update({ status: 'locked', tokenVersion: sequelize.literal('tokenVersion + 1') }, { transaction: t });
  await Session.destroy({ where: { userId: user.id }, transaction: t });
});

// authMiddleware: reject if the token's tokenVersion no longer matches current state —
// this is what makes revocation take effect before the token's own expiry.
if (!user || user.status === 'locked' || user.tokenVersion !== payload.tokenVersion) {
  return res.status(401).json({ error: 'unauthorized', message: 'Invalid or expired credentials.' });
}
```

---

## JS-AUTH-010: Redirect and Callback URL Allowlisting (Open Redirect Prevention) — Medium (counterpart: GO-AUTH-010)

Any `res.redirect()` target derived from request input (`returnTo`, `redirect_uri`, logout redirect) must be validated by parsing the URL and comparing `host` against an explicit allowlist — never a substring/prefix check, which is trivially bypassed by a lookalike or userinfo-embedded host.

```js
// BAD: substring check — bypassable via "https://evil.com/?x=portal.example.com" etc.
if (returnTo.includes('portal.example.com')) return res.redirect(returnTo);

// GOOD: parsed-URL host comparison against an allowlist; reject userinfo tricks; safe fallback
const ALLOWED_REDIRECT_HOSTS = new Set(['portal.example.com', 'console.example.com']);
function safeRedirectTarget(raw) {
  if (typeof raw !== 'string' || !raw) return '/';
  let parsed;
  try { parsed = new URL(raw, 'https://portal.example.com'); } catch { return '/'; }
  if (raw.startsWith('/') && !raw.startsWith('//')) return parsed.pathname + parsed.search; // relative path is safe
  if (parsed.username || parsed.password) return '/'; // "https://attacker@portal.example.com" — reject outright
  if (parsed.protocol !== 'https:' || !ALLOWED_REDIRECT_HOSTS.has(parsed.host)) return '/'; // never echo the rejected value back
  return parsed.toString();
}
res.redirect(safeRedirectTarget(req.query.returnTo));
```

---

> **Verification Checklist before outputting code:**
> * JS-AUTH-001: Does every auth failure branch have a `return` before/with `next()`/`res.status()`?
> * JS-AUTH-002: Does JWT verification pass an explicit asymmetric-only `algorithms` array?
> * JS-AUTH-003: Is any raw token/password/secret passed to `logger.*` unmasked?
> * JS-AUTH-004: Is route protection done via router-group scoping rather than raw `req.url`/`req.path` string matching?
> * JS-AUTH-005: Does every tenant-scoped Sequelize query use `organizationId` from `req.user`, never from request input?
> * JS-AUTH-006: Is every user-supplied HTTP method `.toUpperCase()`-normalized before comparison/lookup/storage?
> * JS-AUTH-007: Does every admin/internal route check a specific scope/role beyond token validity?
> * JS-AUTH-008: Does every `sequelize.query()` use bound `replacements` (or the query builder) rather than string interpolation, with identifiers resolved via an allowlist?
> * JS-AUTH-009: Does a lock/role-change/deletion handler revoke sessions and bump `tokenVersion` in the same transaction?
> * JS-AUTH-010: Is every `res.redirect()` target validated via parsed-URL host allowlist rather than a substring check?
