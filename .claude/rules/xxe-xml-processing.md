# Rule: Go XML External Entity (XXE) Prevention Standards

## Context & Scope

Apply whenever writing, refactoring, or reviewing Go (`.go`) code that parses XML from a user- or tenant-supplied source — a WSDL/SOAP API import, an XML spec upload, a payload-transformation/schema-validation step in the gateway/policy-engine, or any `encoding/xml`/third-party `Decoder`/`Unmarshal` call fed by request bodies, uploaded files, or fetched URLs. There is no XML parsing in this codebase today, but WSDL/SOAP import is a standard feature for an API management product, so this rule is written proactively — the same way `post-quantum-cryptography.md` gets ahead of a primitive before it's used — to get the pattern right from the first line of XML-handling code. XXE is one of the most frequently recurring vulnerability classes in API management products, and can chain into SSRF when a document-supplied schema location is fetched over the network.

## Directives

1. **`encoding/xml` isn't automatically safe.** It doesn't expand external entities and has no DTD engine, so it avoids classic `<!ENTITY>` file-read XXE — but it's still vulnerable to unbounded recursive structures, and any other XML library linked into the build (e.g. cgo bindings to `libxml2`) may support entity expansion. Audit every XML-parsing dependency individually; never assume "we use `encoding/xml`" is enough. If a DTD-aware/libxml2-backed library is used, explicitly disable DTD loading and external entity/network resolution (`NOENT`, `NONET`, `DTDLOAD` off) and assert those flags in a test so an upgrade can't silently re-enable them. Reject any `<!DOCTYPE ...>` declaration outright before parsing, as defense-in-depth independent of parser config — legitimate spec-import documents never need one.
2. **Bound parser resource consumption.** `encoding/xml.Decoder` has no built-in ceiling on size or nesting depth — wrap the input in `io.LimitReader` (config-sourced byte ceiling, per `file-access.md`) and track element depth manually via the streaming `Token()` API. A crafted document can also cause pathological parse time without any entity expansion, so wrap the read+parse in a `context.Context` deadline tighter than the overall request timeout, so a slow parse fails fast instead of holding a worker.
3. **Validate structure against a server-bundled schema only.** When accepting WSDL/XSD/SOAP, validate against the server's own embedded copy of the schema — never a schema URL or `xsi:schemaLocation` read out of the untrusted document itself, which reintroduces the fetch-driven SSRF problem `ssrf-prevention.md` covers. If a location hint is present in the document, ignore or strip it; the server picks the schema, never the document.
4. **Treat every XML input source, and every caller, the same.** A WSDL "import by URL" feature carries fetch-time SSRF risk in addition to parse-time XXE risk in what comes back — harden both. Apply parser hardening uniformly regardless of whether the caller is unauthenticated or an authenticated admin/publisher; authorization controls who can reach a feature, not whether that feature's input handling is safe.
5. **No deferring a violation behind a code comment.** Never resolve a missing size ceiling, DOCTYPE check, or schema-location validation by adding a `// TODO`/`FIXME`-style comment and shipping the parser anyway — a comment does not bound a parse or block a schema fetch. Fix it before merging, or raise the gap explicitly for an approved exception rather than leaving it annotated in the source.

## Example

```go
// BAD: unbounded read, no DOCTYPE check, no parse deadline.
func ParseWSDL(r io.Reader) (*WSDLDefinition, error) {
    data, _ := io.ReadAll(r) // Unbounded — also violates file-access.md directive 5
    var def WSDLDefinition
    return &def, xml.Unmarshal(data, &def) // No DOCTYPE rejection, no depth/time bound
}

// GOOD: byte ceiling, DOCTYPE rejection, and a deadline enforced around the
// whole read+parse — implement via context.WithTimeout wrapping both the
// io.LimitReader read and the token-scanning decode pass (track ctx.Err() on
// every Token() call so a timed-out parse actually stops consuming CPU;
// see decodeWithDepthLimit-style helpers for the full pattern).
const maxDocBytes = 5 << 20 // config-sourced in real call sites
var doctypeMarker = []byte("<!DOCTYPE")

func ParseXML(ctx context.Context, r io.Reader, out interface{}) error {
    ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    data, err := io.ReadAll(io.LimitReader(r, maxDocBytes+1))
    if err != nil {
        return fmt.Errorf("reading XML input: %w", err)
    }
    if len(data) > maxDocBytes {
        return fmt.Errorf("XML document exceeds maximum allowed size")
    }
    if bytes.Contains(data, doctypeMarker) {
        return fmt.Errorf("XML documents containing a DOCTYPE declaration are not allowed")
    }
    return decodeWithDepthLimitAndDeadline(ctx, data, out) // enforces max nesting depth + ctx.Err() checks
}
```

Schema validation must resolve against a closed, `go:embed`-loaded map (`bundledSchemas["wsdl-1.1"]`), never a network fetch or document-supplied path.

> **Verification Checklist before outputting code:**
> * Is every XML read wrapped in a config-sourced `io.LimitReader` before parsing?
> * Is any non-`encoding/xml` library's entity resolution/network access explicitly disabled and asserted in a test?
> * Does schema validation ever resolve `xsi:schemaLocation` or an in-document URI instead of a bundled schema?
> * Is there a nesting-depth ceiling and a parse-specific deadline separate from the HTTP request timeout?
> * Is a gap in this rule "resolved" by a `// TODO`/`FIXME`-style comment instead of an actual fix or an explicitly raised, approved exception?
