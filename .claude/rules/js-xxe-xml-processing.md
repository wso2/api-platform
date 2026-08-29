# Rule: JavaScript (Node.js) XML External Entity (XXE) Prevention Standards

## Context & Scope

Apply whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that parses XML from a user-supplied source — a WSDL/SOAP spec upload or import-by-URL feature, an XML API document preview, or any use of `xml2js`, `fast-xml-parser`, `libxmljs2`, or `DOMParser`/`xmldom` fed by `req.file.buffer`, a request body, or a fetched URL. JS counterpart to `xxe-xml-processing.md` (Go). Written proactively — there's no XML parsing in the portal today, but WSDL/SOAP import is a standard feature category, and the parser config must be correct before that code ships, not after.

## Directives

1. **Prefer a parser with no entity-expansion engine.** Use `fast-xml-parser` — it has no DTD/entity support at all, removing the XXE class structurally rather than via a flag that can regress. If a DTD-aware library is genuinely required (e.g. `libxmljs2` for XSD validation), explicitly disable entity expansion, network access, and DTD loading, and assert those settings in a unit test so an upgrade can't silently re-enable them. `xml2js`/`sax` don't expand external entities by default but enforce no depth/size ceiling, so directive 2 still applies in full.
2. **Bound resource consumption.** Enforce a byte ceiling (config-sourced, e.g. `DP_MAX_XML_BYTES`) on the buffer *before* it reaches the parser — not just multer's `limits.fileSize`, since a fetched-by-URL or chunk-assembled document bypasses that. Cap nesting depth where the parser exposes it. Offload parsing to a worker thread with its own timeout so a pathological document can't block the Node.js event loop. Reject any `<!DOCTYPE` declaration outright before parsing, as defense-in-depth independent of parser config.
3. **Validate only against a server-bundled schema.** Never resolve `xsi:schemaLocation` or any in-document schema hint — load the expected schema from a local file at startup. Resolving a document-supplied location is both an XXE and an SSRF vector (see `js-ssrf-prevention.md`).
4. **Treat every XML source, and every caller, identically.** An "import from URL" feature carries the fetch's SSRF risk on top of the response's XXE risk — harden both. Apply parser hardening the same regardless of caller privilege; XXE is exploitable by authenticated publishers/admins uploading documents, not just unauthenticated callers.
5. **No deferring a violation behind a code comment.** Never resolve a missing size ceiling, DOCTYPE check, or schema-location validation by adding a `// TODO`/`FIXME`-style comment and shipping the parser anyway — a comment does not bound a parse or block a schema fetch. Fix it before merging, or raise the gap explicitly for an approved exception rather than leaving it annotated in the source.

## Example

```js
// BAD: xml2js, no size/depth bound, no DOCTYPE check, parses the raw buffer directly.
const xml2js = require('xml2js');
app.post('/api/specs/wsdl-import', upload.single('wsdl'), async (req, res) => {
  const result = await new xml2js.Parser().parseStringPromise(req.file.buffer.toString());
  res.json(result);
});

// GOOD: DOCTYPE rejection + size ceiling + fast-xml-parser (no entity-expansion engine).
const { XMLParser } = require('fast-xml-parser');
const MAX_XML_BYTES = parseInt(process.env.DP_MAX_XML_BYTES) || 2 * 1024 * 1024;
const parser = new XMLParser({ ignoreAttributes: false });

function parseUntrustedXml(buffer) {
  if (buffer.length > MAX_XML_BYTES) {
    throw Object.assign(new Error('XML document exceeds maximum allowed size'), { statusCode: 413 });
  }
  const xmlString = buffer.toString('utf8');
  if (/<!DOCTYPE/i.test(xmlString)) {
    throw Object.assign(new Error('XML documents containing a DOCTYPE are not allowed'), { statusCode: 422 });
  }
  return parser.parse(xmlString); // For large/untrusted docs, run this inside a worker_threads
}                                  // Worker with its own timeout, so the event loop can't be blocked.
```

> **Verification Checklist before outputting code:**
> * Is a DTD-aware library used without entity/network/DTD-loading disabled and asserted in a test? (Prefer `fast-xml-parser` instead.)
> * Is the XML size-checked before parsing, independent of multer's `limits.fileSize`?
> * Is a DOCTYPE-rejection check applied before parsing, and is parsing offloaded to a worker thread with its own timeout?
> * Does schema validation ever resolve a document-supplied `xsi:schemaLocation` instead of a bundled schema?
> * Is a gap in this rule "resolved" by a `// TODO`/`FIXME`-style comment instead of an actual fix or an explicitly raised, approved exception?
