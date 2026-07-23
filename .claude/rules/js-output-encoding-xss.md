# Rule: JavaScript (Node.js/Express) Output Encoding & Cross-Site Scripting (XSS) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that renders server-side templates, reflects request input (query parameters, headers, form fields) back into an HTML response, or accepts and later displays user-uploaded content (API documentation files, icons/thumbnails, SVG assets). The goal is to prevent both reflected and stored Cross-Site Scripting, which is the single most frequently recurring vulnerability class in API management portals — reflected XSS in authentication/console endpoints and stored XSS via file upload each appear across a large number of releases.

The consistent root cause is untrusted input reaching an HTML or SVG context without encoding appropriate to that context. `httpOnly` session cookies are a safety net, not a substitute for output encoding — they limit blast radius but do not prevent the injection itself.

---

## Directives

### 1. Encode on Output, Contextually, Every Time

* **Never Concatenate Untrusted Input into an HTML String:** Template output (EJS, Handlebars, JSX, or manual string building) must pass untrusted values through the templating engine's escaping mechanism (EJS `<%= %>`, Handlebars `{{ }}`, React's default JSX interpolation) — never through the engine's *raw*/unescaped form (EJS `<%- %>`, Handlebars `{{{ }}}`, React `dangerouslySetInnerHTML`) unless the value has been independently sanitized per directive 2.
* **Match Encoding to Context:** HTML-body encoding is not sufficient for every position a value can land in. Use attribute encoding for values placed inside an HTML attribute, URL encoding (`encodeURIComponent`) for values placed inside a URL/query string, and JavaScript-string encoding (never simple HTML-escaping) for values interpolated into an inline `<script>` block. Prefer avoiding inline `<script>` interpolation entirely in favor of a `data-*` attribute read by a separately loaded script.
* **Redirects and Reflected Parameters:** Any query parameter, header value, or form field that is echoed back into the response body (error pages, "you searched for X" banners, pre-filled form values) must go through the same contextual encoding as any other untrusted value — this is the exact shape of the reflected-XSS cluster in the advisories above (authentication endpoints reflecting a parameter into the rendered page).

### 2. Sanitize Rather Than Trust for Rich/Structured Content

* **User-Controlled HTML/Markdown Rendering:** If a feature must render user-authored HTML or Markdown-to-HTML (e.g. API documentation, changelog notes), run the output through a dedicated sanitizer (`dompurify` via `jsdom`, or `sanitize-html`) with an explicit allowlist of tags/attributes — never a denylist, and never "render as-is because it's Markdown so it's probably safe" (Markdown-to-HTML converters routinely pass raw inline HTML through unchanged).
* **SVG Uploads Are HTML, Not Images, for XSS Purposes:** SVG is an XML format that can contain `<script>` elements and event-handler attributes (`onload`, `onclick`). Treat any user-uploaded SVG the same as user-authored HTML: sanitize it with an SVG-aware allowlist before storage, and independently, serve uploaded SVGs with a `Content-Disposition: attachment` header or from a separate, cookie-less origin so that even an unsanitized SVG cannot execute in the context of an authenticated session.
* **Never Trust the Client-Declared MIME Type:** Validate uploaded file content against its claimed type server-side (magic-byte/structure sniffing) before deciding how to sanitize or serve it — a `.png` extension with SVG/HTML content inside must be caught, not rendered as an image.

### 3. Set Response Headers That Constrain Injection Impact

* **Content-Security-Policy:** Set a restrictive `Content-Security-Policy` (at minimum `script-src 'self'`, no `unsafe-inline`, no `unsafe-eval`) on HTML-serving responses. A CSP does not replace output encoding, but it materially reduces the impact of an encoding gap that slips through.
* **`X-Content-Type-Options: nosniff`:** Always set this so the browser cannot be tricked into interpreting an uploaded file (e.g. a `.txt` containing HTML) as HTML based on content sniffing.
* **`httpOnly` and `SameSite` on Session Cookies:** Set `httpOnly` and `SameSite=Strict`/`Lax` as appropriate — this is a safety net for session theft, not a substitute for directives 1–2, and per `js-authentication-authorization.md` JS-AUTH-004, `SameSite=Lax` alone does not stop state-changing `GET`-based CSRF.

### 4. Treat Every Reflection Surface the Same, Regardless of Where It Lives

* **Error Pages and Framework-Generated Responses:** A generic error handler that echoes `req.query` or a validation-failure message into an HTML error page is just as reflectable as an application view — apply the same encoding rules from `js-error-handling.md`'s sterile-payload requirement to any HTML (not just JSON) error surface.
* **Self-XSS Is Still a Finding:** Even where an injected payload only affects the injecting user's own session, fix it — do not deprioritize output-encoding gaps solely because a particular exploitation path requires the victim to inject into their own view; the same missing encoding is frequently exploitable via a shared/admin-visible surface elsewhere.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```js
// BAD: Reflects a query parameter into an HTML response with no encoding —
// reflected XSS via raw string interpolation.
app.get('/auth/login', (req, res) => {
  const returnTo = req.query.returnTo || '';
  res.send(`<html><body><p>You will be redirected to: ${returnTo}</p></body></html>`);
});

// BAD: Renders user-authored Markdown/HTML for API documentation with no sanitization.
app.get('/apis/:id/docs', async (req, res) => {
  const doc = await ApiDoc.findByPk(req.params.id);
  res.render('docs', { htmlContent: doc.renderedHtml }); // Passed to <%- htmlContent %> in the view
});

// BAD: Serves a stored SVG upload inline, unsanitized — stored XSS via SVG upload.
app.get('/apis/:id/icon', async (req, res) => {
  const icon = await ApiIcon.findOne({ where: { apiId: req.params.id } });
  res.set('Content-Type', icon.mimeType).send(icon.data); // Renders inline, unsanitized
});
```

### Best Practice (What to Generate)

```js
// GOOD: EJS auto-escaping interpolation — <%= %>, never <%- %>, for untrusted values.
// views/login.ejs:  <p>You will be redirected to: <%= returnTo %></p>

app.get('/auth/login', (req, res) => {
  const returnTo = typeof req.query.returnTo === 'string' ? req.query.returnTo : '';
  res.render('login', { returnTo }); // EJS escapes returnTo automatically via <%= %>
});

// utils/sanitizeHtml.js — GOOD: allowlist-based sanitization for user-authored
// rich content, never a denylist and never "render as-is."
const sanitizeHtml = require('sanitize-html');

function sanitizeApiDocHtml(rawHtml) {
  return sanitizeHtml(rawHtml, {
    allowedTags: ['p', 'a', 'b', 'i', 'code', 'pre', 'ul', 'li', 'h1', 'h2', 'h3'],
    allowedAttributes: { a: ['href', 'title'] },
    allowedSchemes: ['http', 'https', 'mailto'], // No javascript: scheme
  });
}

app.get('/apis/:id/docs', async (req, res) => {
  const doc = await ApiDoc.findByPk(req.params.id);
  res.render('docs', { htmlContent: sanitizeApiDocHtml(doc.rawHtml) }); // Sanitized before render
});

// utils/svgSafety.js — GOOD: SVG sanitized with an SVG-aware allowlist AND
// served from a way that cannot execute in the authenticated session's context.
const sanitizeHtml = require('sanitize-html');

function sanitizeSvg(rawSvg) {
  return sanitizeHtml(rawSvg, {
    allowedTags: ['svg', 'path', 'circle', 'rect', 'g', 'defs', 'linearGradient', 'stop'],
    allowedAttributes: {
      '*': ['d', 'fill', 'stroke', 'viewBox', 'width', 'height', 'cx', 'cy', 'r', 'x', 'y'],
    },
    // Strips <script>, on* event-handler attributes, and any foreignObject —
    // the actual XSS surface inside an SVG document.
  });
}

const { fileTypeFromBuffer } = require('file-type');
const ALLOWED_ICON_TYPES = new Set(['image/png', 'image/jpeg', 'image/svg+xml']);

// SVG has no magic bytes for file-type to detect — it's text/XML, so sniff by
// checking for an <svg> root element instead of trusting the declared mimetype.
function isLikelySvg(buffer) {
  const head = buffer.subarray(0, 512).toString('utf8');
  return /^\s*(<\?xml[^>]*>\s*)?(<!--.*?-->\s*)*<svg[\s>]/i.test(head);
}

app.post('/apis/:id/icon', upload.single('icon'), async (req, res, next) => {
  try {
    if (!req.file) {
      return res.status(400).json({ error: 'invalid_request', message: 'No file was uploaded.' });
    }

    // Sniff the actual bytes — req.file.mimetype is client-declared and spoofable
    // (see js-file-access.md). SVG has no reliable magic bytes, so fall back to an
    // XML/SVG-root check on the declared type and verify by attempting sanitization.
    const sniffed = await fileTypeFromBuffer(req.file.buffer);
    const detectedMime = sniffed?.mime ?? (isLikelySvg(req.file.buffer) ? 'image/svg+xml' : null);

    if (!detectedMime || detectedMime !== req.file.mimetype || !ALLOWED_ICON_TYPES.has(detectedMime)) {
      return res.status(422).json({ error: 'invalid_request', message: 'Unsupported or mismatched file type.' });
    }

    const isSvg = detectedMime === 'image/svg+xml';
    const safeData = isSvg ? Buffer.from(sanitizeSvg(req.file.buffer.toString('utf8'))) : req.file.buffer;
    // Persist the validated mime type, never the client-declared one, so later
    // GET responses set Content-Type/Content-Disposition from a trusted value.
    await ApiIcon.create({ apiId: req.params.id, data: safeData, mimeType: detectedMime });
    res.status(201).send();
  } catch (err) {
    next(err);
  }
});

app.get('/apis/:id/icon', async (req, res) => {
  const icon = await ApiIcon.findOne({ where: { apiId: req.params.id } });
  if (!icon) {
    return res.status(404).json({ error: 'not_found' });
  }
  res
    .set('Content-Type', icon.mimeType)
    .set('X-Content-Type-Options', 'nosniff')
    // Even sanitized, force download/attachment for SVGs rather than inline
    // rendering in the app's own origin — belt-and-suspenders per directive 2.
    .set('Content-Disposition', icon.mimeType === 'image/svg+xml' ? 'attachment' : 'inline')
    .send(icon.data);
});

// app.js — GOOD: a restrictive CSP applied to all HTML-serving responses.
const helmet = require('helmet');
app.use(
  helmet.contentSecurityPolicy({
    directives: {
      defaultSrc: ["'self'"],
      scriptSrc: ["'self'"], // No 'unsafe-inline', no 'unsafe-eval'
      objectSrc: ["'none'"],
      frameAncestors: ["'none'"],
    },
  })
);
app.use(helmet.noSniff()); // X-Content-Type-Options: nosniff
```

---

> **Verification Checklist before outputting code:**
> * Does any template use the raw/unescaped interpolation syntax (`<%- %>`, `{{{ }}}`, `dangerouslySetInnerHTML`) on a value that isn't already sanitized via `sanitize-html`/`dompurify`? (If yes, switch to the escaping syntax or add sanitization.)
> * Is a query parameter, header, or form field ever concatenated into an HTML response string without going through the templating engine's escaping? (If yes, route it through the template instead of manual string building.)
> * Are user-uploaded SVGs sanitized with an SVG-aware allowlist AND served with `Content-Disposition: attachment` (or from a separate, cookie-less origin)? (`X-Content-Type-Options: nosniff` is defense-in-depth only — it does not substitute for either sanitization or `Content-Disposition: attachment`; all must be present together.)
> * Is `Content-Security-Policy` set on HTML-serving responses, without `unsafe-inline`/`unsafe-eval`? (If missing, add it via `helmet`.)
> * Is an output-encoding gap being deprioritized because it "only" reproduces in the injecting user's own session (self-XSS)? (Fix it regardless — the same missing encoding is frequently exploitable elsewhere.)
