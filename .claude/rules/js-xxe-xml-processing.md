# Rule: JavaScript (Node.js) XML External Entity (XXE) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that parses XML from a user- or tenant-supplied source — a WSDL/SOAP spec upload or import-by-URL feature, an XML API document preview, or any use of an XML-parsing library (`xml2js`, `fast-xml-parser`, `libxmljs2`, or the DOM `DOMParser`/`xmldom` family) fed by `req.file.buffer`, a request body, or a fetched URL. This is the JavaScript counterpart to `xxe-xml-processing.md` (Go). As with the Go rule, this is written proactively — there is no XML parsing in `portals/developer-portal` today, but WSDL/SOAP import is a standard feature for an API developer portal, and the parser configuration must be correct before that code is written, not after.

XXE is one of the most frequently recurring vulnerability classes in API management products, exploitable through both unauthenticated endpoints and publisher/developer-facing document uploads — exactly the kind of feature this portal provides.

---

## Directives

### 1. Choose and Configure a Parser That Disables External Entities by Default

* **Prefer `fast-xml-parser`:** It has no DTD/entity-expansion support at all, which removes the entire XXE class structurally rather than via a configuration flag that could be misconfigured or regress on upgrade. Prefer it over DTD-aware alternatives for any new XML-handling code.
* **If a DTD-Aware Library Is Required** (e.g. `libxmljs2` for full XML Schema validation): explicitly set `noent: false`, disable network access (`nonet: true` or equivalent), and disable DTD loading (`noblanks`/`nocdata`/`dtdload: false` per the library's actual option names). Never rely on defaults — assert the configuration in a unit test so a dependency bump cannot silently re-enable entity expansion.
* **Reject `xml2js` for Untrusted Input Unless Hardened:** `xml2js` delegates to `sax`, which does not expand external entities by default, but does not enforce any depth or size ceiling either — directive 2 still applies in full.
* **Reject DOCTYPE Declarations Outright:** Before handing bytes to any parser, scan for and reject a `<!DOCTYPE` declaration as a defense-in-depth layer independent of parser configuration — legitimate WSDL/XSD documents from this portal's use cases never need one.

### 2. Bound Parser Resource Consumption

* **Size Ceiling Before Parsing:** Enforce the multer/upload byte ceiling from `js-file-access.md` *before* the buffer reaches the XML parser, not just at the HTTP layer — a document assembled from multiple upload chunks or fetched from a URL bypasses multer's own limit.
* **Depth Ceiling:** Configure the parser's maximum nesting depth if it exposes one (`fast-xml-parser`'s `stopNodes`/depth options), or wrap parsing in a manual depth-tracking pass otherwise. An attacker-crafted document can cause pathological CPU time via deep nesting alone, without any entity expansion.
* **Parse-Specific Timeout:** Wrap the parse call so it cannot block the Node.js event loop indefinitely — for large or attacker-controlled documents, offload parsing to a worker thread with its own timeout rather than parsing synchronously on the request-handling thread.

### 3. Validate Structure Against a Server-Bundled Schema Only

* **Never Resolve `xsi:schemaLocation` or an In-Document Schema Hint:** If schema validation is implemented, validate against a schema shipped with the application (loaded from a local file at startup), never against a URL or path extracted from the untrusted document itself. Resolving a document-supplied schema location is both an XXE vector and an SSRF vector (see `js-ssrf-prevention.md`).

### 4. Treat Every Source of XML Bytes the Same

* **Uploads, Request Bodies, and Fetched URLs:** A "import WSDL from URL" feature carries the SSRF risk of the fetch itself (`js-ssrf-prevention.md`) in addition to the XXE risk of parsing what comes back — harden both, not just one.
* **Privilege Does Not Exempt a Code Path:** XXE is exploitable by authenticated publishers and admins uploading documents, not only unauthenticated callers. Apply parser hardening uniformly regardless of the caller's role — authorization decides who may use the upload feature, not whether the parser is configured safely.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```js
// BAD: xml2js with no size/depth bound, parsing directly from the upload buffer.
const xml2js = require('xml2js');

app.post('/api/specs/wsdl-import', upload.single('wsdl'), async (req, res) => {
  const parser = new xml2js.Parser();
  const result = await parser.parseStringPromise(req.file.buffer.toString());
  res.json(result); // No DOCTYPE check, no depth limit, no parse timeout
});

// BAD: schema location taken from the document itself and fetched — XXE and SSRF
// in the same code path.
async function validateAgainstDeclaredSchema(xmlString) {
  const schemaLocation = extractSchemaLocation(xmlString);
  const schemaResp = await axios.get(schemaLocation); // Fetches an attacker-chosen URI
  return validateAgainstSchema(xmlString, schemaResp.data);
}
```

### Best Practice (What to Generate)

```js
// utils/xmlSafety.js — GOOD: DOCTYPE rejection, size ceiling, and a
// DTD-free parser (fast-xml-parser) used instead of a DTD-aware alternative.
const { XMLParser } = require('fast-xml-parser');

const MAX_XML_BYTES = parseInt(process.env.DP_MAX_XML_BYTES) || 2 * 1024 * 1024;
const DOCTYPE_PATTERN = /<!DOCTYPE/i;

const parser = new XMLParser({
  allowBooleanAttributes: true,
  ignoreAttributes: false,
  // fast-xml-parser has no DTD/entity-expansion engine — external entities
  // are structurally unsupported rather than merely disabled by a flag.
});

function parseUntrustedXml(buffer) {
  if (buffer.length > MAX_XML_BYTES) {
    throw Object.assign(new Error('XML document exceeds maximum allowed size'), { statusCode: 413 });
  }

  const xmlString = buffer.toString('utf8');

  // Defense-in-depth: reject any DOCTYPE outright, independent of parser config.
  if (DOCTYPE_PATTERN.test(xmlString)) {
    throw Object.assign(new Error('XML documents containing a DOCTYPE are not allowed'), { statusCode: 422 });
  }

  return parser.parse(xmlString);
}

module.exports = { parseUntrustedXml };

// utils/xmlSafetyTimeout.js — GOOD: parsing offloaded to a worker thread with
// its own timeout, so a pathological document cannot block the event loop.
const { Worker } = require('node:worker_threads');
const path = require('node:path');

function parseWithTimeout(buffer, timeoutMs = 3000) {
  return new Promise((resolve, reject) => {
    const worker = new Worker(path.join(__dirname, 'xmlParseWorker.js'), {
      workerData: { buffer },
    });

    const timer = setTimeout(() => {
      worker.terminate();
      reject(Object.assign(new Error('XML parsing exceeded time limit'), { statusCode: 408 }));
    }, timeoutMs);

    worker.once('message', (result) => {
      clearTimeout(timer);
      worker.terminate();
      resolve(result);
    });
    worker.once('error', (err) => {
      clearTimeout(timer);
      worker.terminate();
      reject(err);
    });
  });
}

module.exports = { parseWithTimeout };

// routes/specs.js — GOOD: bounded, hardened parsing; schema validated only
// against a schema bundled with the application, never a document-supplied hint.
const { parseWithTimeout } = require('../utils/xmlSafetyTimeout');
const fs = require('node:fs');
const path = require('node:path');

const BUNDLED_SCHEMAS = {
  'wsdl-1.1': fs.readFileSync(path.join(__dirname, '../schemas/wsdl-1.1.xsd'), 'utf8'),
};

app.post('/api/specs/wsdl-import', upload.single('wsdl'), async (req, res, next) => {
  try {
    const parsed = await parseWithTimeout(req.file.buffer);
    // Directive 3: validate against BUNDLED_SCHEMAS only — never a URL or path
    // read out of the uploaded document itself.
    validateAgainstBundledSchema(parsed, BUNDLED_SCHEMAS['wsdl-1.1']);
    res.json({ spec: parsed });
  } catch (err) {
    logger.warn('WSDL import rejected', { reason: err.message, statusCode: err.statusCode });
    return res.status(err.statusCode ?? 422).json({
      error: 'invalid_document',
      message: 'The uploaded document could not be processed.',
    });
  }
});
```

---

> **Verification Checklist before outputting code:**
> * Is a DTD-aware XML library (`libxmljs2`, `xmldom`, etc.) used, and if so, is entity expansion and network access explicitly disabled and asserted in a test? (Prefer `fast-xml-parser`, which has no entity-expansion engine at all.)
> * Is the uploaded/fetched XML size-checked *before* it reaches the parser, independent of multer's own `limits.fileSize`? (If not, add an explicit ceiling.)
> * Is there a DOCTYPE-rejection check applied before parsing, as defense-in-depth regardless of parser configuration? (If not, add it.)
> * Does schema validation resolve a URL or path read out of the untrusted document (`xsi:schemaLocation` or similar)? (If yes, replace with a schema bundled with the application.)
> * Can a pathologically nested or sized document block the Node.js event loop during parsing? (If the parse isn't offloaded to a worker thread with its own timeout, add one for any parser invocation on untrusted input.)
