# Rule: JavaScript (Express) Error & Payload Validation Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` responsible for Express route handlers, error middleware, response construction, and request processing. The goal is to enforce strict security boundaries, prevent information disclosure, and mitigate user enumeration attacks. This is the JavaScript counterpart to `error-handling.md` (Go).

---

## Directives

### 1. Data Leakage & Internal Exposure

* **Zero Internal Details:** Never forward raw ORM/database errors (e.g., Sequelize `ValidationError`, `DatabaseError`, `UniqueConstraintError`), stack traces, internal service names, or file system paths to the client response.
* **Centralised Error Middleware:** All unhandled errors must flow through a single Express error-handling middleware `(err, req, res, next)` registered last. Route handlers must call `next(err)` — they must never serialise the raw `Error` object directly.
* **Strip `err.stack` and `err.message`:** Before sending any error response, explicitly omit `err.stack`, `err.message`, and `err.sql` (Sequelize) from the serialised payload.

### 2. Vendor Header Abstraction

* **No Leaky Headers:** Never forward or set response headers that disclose infrastructure details — including `X-Amz-*`, `X-Powered-By`, `Server`, `X-Vercel-*`, or `Cf-Ray`.
* **Disable Express Default:** Set `app.disable('x-powered-by')` in application bootstrap so Express does not advertise itself.
* **Standardisation:** Respond only with standard HTTP headers or the platform-specific `X-Request-ID` correlation header.

### 3. Dynamic Value Generation & Source Obfuscation

* **No Source Identifiers:** Tracking IDs, correlation tokens, and error tokens must be generated with `crypto.randomUUID()` (Node.js built-in, no extra dependency) or the `uuid` package already present in the project. Never concatenate file names, line numbers, function names, or environment labels into these values.
* **Avoid `Date.now()` Alone:** A raw timestamp is not a tracking ID — it is guessable and carries a time leak. Always combine with a UUID or use UUID alone.

### 4. Unified Authentication Failures

* **Constant-Response Auth:** All authentication failures — wrong password, expired token, missing token, revoked token, invalid signature — must return the same HTTP status and payload. Do not branch on the error type in the response path.
* **Allowed Status:** `HTTP 401 Unauthorized`
* **Standard Payload:**
  ```json
  {
    "error": "unauthorized",
    "message": "Invalid or expired credentials."
  }
  ```
* **Internal Logging Only:** Log the specific failure reason (`token expired`, `signature invalid`, `user not found`) internally via Winston before sending the generic response. Never include it in the body.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```js
// BAD: Leaks Sequelize error, sets X-Powered-By equivalent header, reveals auth failure reason,
// uses source-tagged tracking ID.
app.post('/login', async (req, res) => {
  try {
    const user = await User.findOne({ where: { email: req.body.email } });
    if (!user) {
      return res.status(401).json({ error: 'User not found' }); // Enumeration leak
    }
    const valid = await bcrypt.compare(req.body.password, user.password);
    if (!valid) {
      return res.status(401).json({ error: 'Wrong password' }); // Enumeration leak
    }
    res.json({ token: generateToken(user) });
  } catch (err) {
    res.setHeader('X-Error-Source', 'auth-service-login-handler'); // Leaky header
    res.status(500).json({
      error: err.message,                           // Exposes raw DB/stack info
      code: `AUTH_ROUTE_LOGIN_L18_${Date.now()}`,  // Source-tagged, guessable ID
      stack: err.stack,                             // Stack trace in response
    });
  }
});
```

### Best Practice (What to Generate)

```js
// app.js — bootstrap
const app = express();
app.disable('x-powered-by'); // Remove Express fingerprint header

// errors/AppError.js — sterile error class for internal propagation
class AppError extends Error {
  constructor(statusCode, clientMessage) {
    super(clientMessage);
    this.statusCode = statusCode;
    this.clientMessage = clientMessage;
  }
}

// middleware/errorHandler.js — single exit point for all errors
function errorHandler(err, req, res, next) { // eslint-disable-line no-unused-vars
  const trackingId = crypto.randomUUID(); // Node.js built-in — no source tags

  // Log the full internal detail, never send it to the client
  logger.error('Unhandled error', {
    trackingId,
    message: err.message,
    stack: err.stack,
    path: req.path,
    method: req.method,
  });

  const statusCode = err.statusCode ?? 500;
  const clientMessage = err.clientMessage ?? 'An unexpected error occurred.';

  res.status(statusCode).json({
    error: clientMessage,
    tracking_id: trackingId,
  });
}
app.use(errorHandler);

// routes/auth.js — GOOD: unified auth failure, generic response
app.post('/login', async (req, res, next) => {
  try {
    const user = await User.findOne({ where: { email: req.body.email } });
    const valid = user && await bcrypt.compare(req.body.password, user.password);

    if (!valid) {
      // Log the specific reason internally only — never log PII such as email
      logger.warn('Authentication failed', {
        reason: user ? 'invalid_password' : 'user_not_found',
      });
      // Return identical response regardless of reason
      return res.status(401).json({
        error: 'unauthorized',
        message: 'Invalid or expired credentials.',
      });
    }
    res.json({ token: generateToken(user) });
  } catch (err) {
    next(err); // Delegate to errorHandler — never serialise raw err here
  }
});
```

---

> **Verification Checklist before outputting code:**
> * Does any error response branch reveal *why* auth failed (user not found vs wrong password)? (If yes, collapse to a single generic response).
> * Is `err.message`, `err.stack`, or an ORM error object present in any `res.json()` call? (If yes, remove and route through `errorHandler`).
> * Does the generated ID contain file names, line numbers, or timestamps alone? (If yes, replace with `crypto.randomUUID()`).
> * Is `app.disable('x-powered-by')` present in bootstrap and are leaky headers absent from `res.setHeader` calls? (If no, add the disable call).
