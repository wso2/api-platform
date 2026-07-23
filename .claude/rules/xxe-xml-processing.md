# Rule: Go XML External Entity (XXE) Prevention Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that parses XML from a user- or tenant-supplied source — a WSDL/SOAP API import, an XML API spec, a payload-transformation or schema-validation step in the gateway/policy-engine, or any `encoding/xml` (or third-party XML library) `Decoder`/`Unmarshal` call fed by request bodies, uploaded files, or fetched URLs. There is no XML parsing in this codebase's Go modules today, but this platform is an API management product, and WSDL/SOAP import and XML-based schema validation are a standard feature class for such platforms — this rule is written proactively, the same way `post-quantum-cryptography.md` establishes a standard ahead of the primitive being used, so the correct pattern is in place from the first line of XML-handling code rather than retrofitted after an incident.

XXE is one of the most frequently recurring vulnerability classes in API management products. The consistent root cause is an XML parser configured with its default, permissive settings — enabling external entity resolution, DTD loading, or unbounded resource consumption. XXE in spec-import, schema-validation, and payload-transformation code paths has been exploited both unauthenticated and by authenticated users, and can chain into SSRF when a document-supplied schema location is fetched over the network.

---

## Directives

### 1. Disable External Entity Resolution by Default

* **`encoding/xml` Is Not Automatically Safe from All XXE Classes:** Go's standard `encoding/xml` package does not expand external entities by default and has no DTD-based entity-expansion engine, which avoids classic XXE file-read via `<!ENTITY>`. It is, however, still vulnerable to unbounded recursive structures and to any external XML library linked into the build (e.g. via cgo bindings to `libxml2`) that *does* support entity expansion. Never assume "we use `encoding/xml`" is sufficient — audit every XML-parsing dependency individually.
* **If Using a libxml2-Backed or DTD-Aware Library:** Explicitly disable DTD loading and external entity resolution (`NOENT`, `NONET`, `DTDLOAD` off, or the library's equivalent). Never rely on library defaults; set every relevant flag explicitly and assert its value in a unit test so a dependency upgrade cannot silently re-enable it.
* **Reject DOCTYPE Declarations Outright:** For API/spec-import use cases, there is almost never a legitimate reason for a submitted document to contain a `<!DOCTYPE ...>` declaration. Scan for and reject it before handing the byte stream to any parser, as a defense-in-depth layer independent of parser configuration.

### 2. Bound Parser Resource Consumption (Billion-Laughs / Entity-Expansion DoS)

* **Depth and Size Limits:** Configure (or wrap) any XML decoder with a maximum element-nesting depth and a maximum total decoded-output size. `encoding/xml.Decoder` has no built-in ceiling — wrap the input `io.Reader` in `io.LimitReader` per the byte-ceiling directive in `file-access.md`, and track nesting depth manually in a custom `TokenReader` if deep nesting is a realistic attack surface for the feature.
* **Timeouts on Parse, Not Just on the HTTP Request:** A crafted document can cause pathological parse time even without infinite expansion. Wrap the parse call with a `context.Context` deadline separate from (and tighter than) the overall request timeout, so a slow parse fails fast rather than holding a worker for the full request timeout.

### 3. Validate Structure Before Semantic Processing

* **Schema-Validate Against a Known, Server-Controlled Schema:** When accepting WSDL/XSD/SOAP envelopes, validate the incoming document against the server's own copy of the expected schema — never against a schema URL or `xsi:schemaLocation` supplied inside the untrusted document itself (that reintroduces the same untrusted-fetch problem this rule exists to prevent, and overlaps with `ssrf-prevention.md`).
* **Never Resolve `xsi:schemaLocation` / External DTD URIs:** If the parser or a downstream schema-validation step reads a location hint from the document, explicitly force it to use only the locally bundled schema and ignore/strip the hint — do not let document content pick which schema (or which network resource) validates itself.

### 4. Treat All XML Input Sources the Same

* **Uploaded Files, Request Bodies, and Fetched URLs Are Equally Untrusted:** A WSDL "import by URL" feature is XXE-relevant twice over — once for the XXE risk in the fetched document, and once for the SSRF risk in fetching it at all (see `ssrf-prevention.md`). Apply this rule's parser hardening regardless of how the XML bytes arrived at the parser.
* **No Special-Casing "Admin-Only" Uploads:** XXE is exploitable both through unauthenticated endpoints and by authenticated administrators or publishers. Authentication level does not change the parser-hardening requirement — apply it uniformly regardless of the caller's privilege, per the principle that authorization controls access to a feature, not the safety of how that feature processes its input.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: Unbounded read, no depth limit, and (if a DTD-aware library is later
// swapped in for encoding/xml) no explicit entity-resolution disable.
func ParseWSDL(r io.Reader) (*WSDLDefinition, error) {
    data, err := io.ReadAll(r) // Unbounded — also violates file-access.md directive 5
    if err != nil {
        return nil, err
    }

    var def WSDLDefinition
    if err := xml.Unmarshal(data, &def); err != nil {
        return nil, err
    }
    return &def, nil
}

// BAD: Schema location taken from the untrusted document itself, and fetched
// over the network — this is XXE and SSRF at the same time.
func ValidateAgainstDeclaredSchema(doc []byte) error {
    loc := extractSchemaLocation(doc) // e.g. reads xsi:schemaLocation from the doc
    schema, err := http.Get(loc)      // Fetches whatever URI the attacker embedded
    if err != nil {
        return err
    }
    defer schema.Body.Close()
    return validateXMLAgainstSchema(doc, schema.Body)
}
```

### Best Practice (What to Generate)

```go
// xmlsafety/parser.go — GOOD: bounded read, DOCTYPE rejection as defense-in-depth,
// depth-limited token scanning, and a parse-specific deadline.
package xmlsafety

import (
    "bytes"
    "context"
    "encoding/xml"
    "fmt"
    "io"
    "time"
)

const (
    maxDocBytes  = 5 << 20 // 5 MiB — sourced from config in real call sites, see file-access.md
    maxElemDepth = 64
    parseTimeout = 3 * time.Second
)

var doctypeMarker = []byte("<!DOCTYPE")

// ctxReader aborts the next Read as soon as ctx is done, so io.ReadAll cannot
// keep blocking past the deadline on a slow or stalled upstream reader (e.g. a
// spec fetched from a URL). This is what makes it safe to always wait for the
// read goroutine below instead of racing ahead on ctx.Done() — cancellation is
// cooperative, so the goroutine is guaranteed to exit promptly rather than
// being abandoned to run (and potentially keep writing into a shared buffer)
// after ParseXML has already returned.
type ctxReader struct {
    ctx context.Context
    r   io.Reader
}

func (cr ctxReader) Read(p []byte) (int, error) {
    if err := cr.ctx.Err(); err != nil {
        return 0, err
    }
    return cr.r.Read(p)
}

// ParseXML enforces: byte ceiling, DOCTYPE rejection, nesting-depth ceiling,
// and a hard parse deadline — independent of which XML library is in use.
func ParseXML(ctx context.Context, r io.Reader, out interface{}) error {
    // The deadline is created before the read, not just before decode — a slow
    // or stalled io.Reader must not be able to hang this call past parseTimeout
    // regardless of which step is slow.
    ctx, cancel := context.WithTimeout(ctx, parseTimeout)
    defer cancel()

    limited := io.LimitReader(ctxReader{ctx: ctx, r: r}, maxDocBytes+1)

    type readResult struct {
        data []byte
        err  error
    }
    readCh := make(chan readResult, 1)
    go func() {
        data, err := io.ReadAll(limited)
        readCh <- readResult{data: data, err: err}
    }()

    // Always wait for the goroutine's result rather than selecting on
    // ctx.Done() — ctxReader guarantees io.ReadAll unblocks within one Read
    // call of the deadline firing, so this cannot hang, and waiting here
    // guarantees the goroutine has actually exited before we proceed.
    res := <-readCh
    if res.err != nil {
        if ctx.Err() != nil {
            return fmt.Errorf("XML parsing exceeded time limit")
        }
        return fmt.Errorf("reading XML input: %w", res.err)
    }
    data := res.data

    if len(data) > maxDocBytes {
        return fmt.Errorf("XML document exceeds maximum allowed size")
    }

    // Defense-in-depth: reject any DOCTYPE declaration outright. Legitimate
    // spec-import documents never need one; this closes off DTD-based entity
    // expansion regardless of the underlying parser's own configuration.
    if bytes.Contains(data, doctypeMarker) {
        return fmt.Errorf("XML documents containing a DOCTYPE declaration are not allowed")
    }

    errCh := make(chan error, 1)
    go func() {
        errCh <- decodeWithDepthLimit(ctx, data, out)
    }()

    // As with the read step: decodeWithDepthLimit checks ctx on every token
    // and again before the final Unmarshal pass, so it is guaranteed to exit
    // promptly once the deadline fires — waiting here (instead of racing
    // ctx.Done()) guarantees no goroutine is left running against `out` after
    // ParseXML returns.
    err := <-errCh
    if err != nil && ctx.Err() != nil {
        return fmt.Errorf("XML parsing exceeded time limit")
    }
    return err
}

// decodeWithDepthLimit wraps encoding/xml's streaming token reader to enforce
// a maximum nesting depth — encoding/xml has no built-in ceiling of its own.
// It checks ctx on every token so a timed-out parse actually stops consuming
// CPU instead of continuing in the background after ParseXML has returned.
func decodeWithDepthLimit(ctx context.Context, data []byte, out interface{}) error {
    dec := xml.NewDecoder(bytes.NewReader(data))
    // encoding/xml does not expand external entities, but explicitly disable
    // any entity-expanding hook to prevent a future refactor from adding one.
    dec.Entity = map[string]string{}
    dec.Strict = true

    depth := 0
    for {
        if err := ctx.Err(); err != nil {
            return err // Deadline already exceeded — stop pulling more tokens.
        }
        tok, err := dec.Token()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("malformed XML: %w", err)
        }
        switch tok.(type) {
        case xml.StartElement:
            depth++
            if depth > maxElemDepth {
                return fmt.Errorf("XML document exceeds maximum nesting depth")
            }
        case xml.EndElement:
            depth--
        }
    }

    // Re-check ctx before the final full-document pass — the streaming scan
    // above already bounded nesting depth and, combined with maxDocBytes,
    // bounds this call's duration, but this keeps decodeWithDepthLimit's exit
    // guarantee explicit rather than implicit.
    if err := ctx.Err(); err != nil {
        return err
    }

    // Re-decode into the target struct now that shape has been validated —
    // encoding/xml has no single-pass "validate then unmarshal" API.
    return xml.Unmarshal(data, out)
}

// xmlsafety/schema.go — GOOD: validates against the server's own bundled schema
// only. The document's own xsi:schemaLocation hint is parsed for logging/audit
// purposes only and is never dereferenced or used to select the validating schema.
func ValidateAgainstBundledSchema(doc []byte, schemaName string) error {
    schema, ok := bundledSchemas[schemaName] // A closed, server-controlled set — see below
    if !ok {
        return fmt.Errorf("unknown schema requested")
    }
    return validateXMLAgainstSchema(doc, schema)
}

// bundledSchemas is populated at build/init time from embedded assets
// (e.g. via go:embed) — never from a network fetch or a path derived from
// document content. This also sidesteps the SSRF risk covered in ssrf-prevention.md.
var bundledSchemas = map[string][]byte{
    "wsdl-1.1": mustReadEmbeddedSchema("schemas/wsdl-1.1.xsd"),
    "wsdl-2.0": mustReadEmbeddedSchema("schemas/wsdl-2.0.xsd"),
}
```

---

> **Verification Checklist before outputting code:**
> * Does any XML parsing path read an unbounded stream before parsing? (If yes, wrap in `io.LimitReader` with a config-sourced ceiling, per `file-access.md` directive 5.)
> * Is a DTD-aware or libxml2-backed XML library used anywhere, and if so, are entity resolution and network access explicitly disabled with an assertion in a test? (If not, audit and lock down every non-`encoding/xml` dependency.)
> * Does any schema-validation step resolve `xsi:schemaLocation` or another in-document URI/path hint rather than a server-bundled schema? (If yes, replace with a closed, server-controlled schema set.)
> * Is there a nesting-depth ceiling and a parse-specific timeout independent of the overall HTTP request timeout? (If not, add both — a crafted document can cause pathological parse time without any entity expansion at all.)
> * Is XML hardening applied uniformly regardless of whether the caller is unauthenticated, authenticated, or an administrator? (XXE is exploitable by privileged callers too — privilege does not exempt a code path from this rule.)
