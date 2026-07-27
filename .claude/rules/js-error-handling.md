# Rule: JavaScript (Express) Error & Payload Validation Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` responsible for Express route handlers, error middleware, response construction, and request processing. The goal is to enforce strict security boundaries, prevent information disclosure, and mitigate user enumeration attacks. This is the JavaScript counterpart to `error-handling.md` (Go).

## Directives

1. **Data Leakage & Internal Exposure.** Never forward raw ORM/database errors (Sequelize `ValidationError`, `DatabaseError`, `UniqueConstraintError`), stack traces, internal service names, or file system paths to the client. Route every unhandled error through a single, last-registered Express error middleware `(err, req, res, next)` — handlers call `next(err)`, never serialize the raw `Error` object. Explicitly strip `err.stack`, `err.message`, and `err.sql` before any response is sent.
2. **Vendor Header Abstraction.** Never forward or set headers that disclose infrastructure details (`X-Amz-*`, `X-Powered-By`, `Server`, `X-Vercel-*`, `Cf-Ray`). Call `app.disable('x-powered-by')` in bootstrap. Respond only with standard HTTP headers or the platform's `X-Request-ID` correlation header.
3. **Dynamic Value Generation & Source Obfuscation.** Generate tracking IDs, correlation tokens, and error tokens with `crypto.randomUUID()` (or the `uuid` package) — never concatenate file names, line numbers, function names, or environment labels into them. A raw `Date.now()` is not a tracking ID (guessable, time-leaking); combine with a UUID or use a UUID alone.
4. **Unified Authentication Failures.** Every authentication failure — wrong password, expired/missing/revoked token, invalid signature — must return the identical `HTTP 401` status and payload (`{"error": "unauthorized", "message": "Invalid or expired credentials."}`); never branch the response on failure type. Log the specific reason (`token expired`, `user not found`) internally via Winston only, never in the body.

## Example

```js
// app.js bootstrap
app.disable('x-powered-by'); // Directive 2

// middleware/errorHandler.js — single exit point (Directive 1)
function errorHandler(err, req, res, next) { // eslint-disable-line no-unused-vars
  const trackingId = crypto.randomUUID(); // Directive 3 — no source tags, no Date.now() alone
  logger.error('Unhandled error', { trackingId, message: err.message, stack: err.stack, path: req.path });
  const statusCode = err.statusCode ?? 500;
  res.status(statusCode).json({ error: err.clientMessage ?? 'An unexpected error occurred.', tracking_id: trackingId });
  // GOOD: no err.stack/err.message/err.sql in the response, no leaky headers set here
}
app.use(errorHandler);

// routes/auth.js
app.post('/login', async (req, res, next) => {
  try {
    const user = await User.findOne({ where: { email: req.body.email } });
    const valid = user && await bcrypt.compare(req.body.password, user.password);
    if (!valid) {
      logger.warn('Authentication failed', { reason: user ? 'invalid_password' : 'user_not_found' }); // internal only
      // GOOD: identical response regardless of cause (Directive 4)
      // BAD would be: res.status(401).json({ error: 'User not found' }) / { error: 'Wrong password' } — enumeration leak
      return res.status(401).json({ error: 'unauthorized', message: 'Invalid or expired credentials.' });
    }
    res.json({ token: generateToken(user) });
  } catch (err) {
    next(err); // BAD would be: res.status(500).json({ error: err.message, stack: err.stack })
  }
});
```

> **Verification Checklist before outputting code:**
> * Does any error response branch reveal *why* auth failed? (Collapse to one generic 401 response.)
> * Is `err.message`, `err.stack`, or an ORM error object present in any `res.json()` call? (Remove; route through `errorHandler`.)
> * Is `app.disable('x-powered-by')` set, and are `X-Amz-*`/`X-Vercel-*`/`Cf-Ray`-style headers absent? (Add/strip as needed.)
> * Does any generated ID embed file names, line numbers, or a bare timestamp? (Replace with `crypto.randomUUID()`.)
