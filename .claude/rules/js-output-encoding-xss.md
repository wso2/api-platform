# Rule: JavaScript (Node.js/Express) Output Encoding & Cross-Site Scripting (XSS) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that renders server-side templates, reflects request input (query parameters, headers, form fields) back into an HTML response, or accepts and later displays user-uploaded content (API docs, icons, SVG assets). Reflected and stored XSS are the most frequently recurring vulnerability class in API management portals. `httpOnly` session cookies are a safety net, not a substitute for output encoding — they limit blast radius but do not prevent the injection itself.

## Directives

1. **Encode on output, contextually, every time.** Pass untrusted values through the templating engine's escaping mechanism (EJS `<%= %>`, Handlebars `{{ }}`, JSX default interpolation) — never the raw/unescaped form (EJS `<%- %>`, Handlebars `{{{ }}}`, `dangerouslySetInnerHTML`) unless independently sanitized per directive 2. Match encoding to position: attribute-encode values in HTML attributes, `encodeURIComponent` for URL/query positions, JS-string-encode (never HTML-escape) for inline `<script>` interpolation — prefer a `data-*` attribute over inline script entirely. Any reflected query parameter, header, or form field (error pages, "you searched for X" banners, pre-filled forms) gets the same contextual encoding as any other untrusted value.
2. **Sanitize rather than trust for rich/structured content.** User-authored HTML or Markdown-to-HTML (docs, changelogs) must go through `sanitize-html`/`dompurify` with an explicit tag/attribute allowlist — never a denylist, never "it's Markdown so it's probably safe" (converters pass raw inline HTML through unchanged). Treat SVG uploads as HTML, not images: sanitize with an SVG-aware allowlist (strips `<script>`, `on*` handlers, `foreignObject`) AND independently serve them with `Content-Disposition: attachment` or from a cookie-less origin — sanitization alone is not sufficient, both must be present. Never trust `req.file.mimetype`; content-sniff the actual bytes (SVG has no magic bytes, so check for an `<svg>` root) before deciding how to sanitize/serve, and persist the detected type, not the client-declared one.
3. **Set response headers that constrain injection impact.** Apply a restrictive `Content-Security-Policy` via `helmet.contentSecurityPolicy` (`script-src 'self'`, no `unsafe-inline`/`unsafe-eval`) on HTML-serving responses — defense-in-depth, not a replacement for encoding. Always set `X-Content-Type-Options: nosniff` (`helmet.noSniff()`) so uploads can't be browser-sniffed into HTML. Set `httpOnly` and `SameSite=Strict`/`Lax` on session cookies, noting `SameSite=Lax` alone doesn't stop state-changing GET-based CSRF (`js-authentication-authorization.md`).
4. **Treat every reflection surface the same, regardless of where it lives.** A generic error handler echoing `req.query` or a validation message into an HTML error page is exactly as reflectable as an application view — apply the same encoding rules there too (see `js-error-handling.md`'s sterile-payload requirement). Fix self-XSS findings too: an injection that "only" affects the injecting user's own session is frequently exploitable via a shared/admin-visible surface elsewhere — don't deprioritize it.

## Example

```js
// BAD: raw string interpolation / <%- %> — reflected & stored XSS
app.get('/auth/login', (req, res) => {
  const returnTo = req.query.returnTo || '';
  res.send(`<p>Redirecting to: ${returnTo}</p>`); // No encoding at all
});
// views/docs.ejs:  <%- doc.rawHtml %>   // Raw interpolation — never for untrusted content

// GOOD: EJS auto-escaping + allowlist sanitization for rich content
// views/login.ejs:  <p>Redirecting to: <%= returnTo %></p>   // Escaped automatically
app.get('/auth/login', (req, res) => {
  const returnTo = typeof req.query.returnTo === 'string' ? req.query.returnTo : '';
  res.render('login', { returnTo });
});

const sanitizeHtml = require('sanitize-html');
function sanitizeApiDocHtml(rawHtml) {
  return sanitizeHtml(rawHtml, {
    allowedTags: ['p', 'a', 'b', 'i', 'code', 'pre', 'ul', 'li'],
    allowedAttributes: { a: ['href', 'title'] },
    allowedSchemes: ['http', 'https', 'mailto'], // No javascript: scheme
  });
}
app.get('/apis/:id/docs', async (req, res) => {
  const doc = await ApiDoc.findByPk(req.params.id);
  res.render('docs', { htmlContent: sanitizeApiDocHtml(doc.rawHtml) }); // Sanitized before render
});
```

> **Verification Checklist before outputting code:**
> * Does any template use raw/unescaped interpolation (`<%- %>`, `{{{ }}}`, `dangerouslySetInnerHTML`) on an unsanitized value? (Switch to escaping syntax or add sanitization.)
> * Is a reflected query parameter/header/form field concatenated into HTML rather than routed through the template's escaping? (Route it through the template.)
> * Are uploaded SVGs both sanitized with an SVG-aware allowlist AND served as `attachment`/from a cookie-less origin — not just one or the other? (`nosniff` is defense-in-depth only.)
> * Is `Content-Security-Policy` (no `unsafe-inline`/`unsafe-eval`) set on HTML responses via `helmet`? (Add if missing.)
> * Is an encoding gap being deprioritized as "self-XSS only"? (Fix it regardless.)
