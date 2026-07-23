# Rule: Go File Access Security Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code that handles file reads, file uploads, archive extraction, database storage of file metadata, or any operation that touches the filesystem or processes byte streams from user-provided input. The goal is to prevent path traversal attacks, information disclosure via filesystem access, and resource exhaustion via unbounded stream consumption.

---

## Directives

### 1. Path Traversal Prevention (Clean Path Validation)

* **Enforce Containment:** Any file read operation must resolve the final absolute path and assert it is strictly within the intended root directory. A request for `../../etc/passwd` must be rejected before `os.Open` is ever called.
* **Use `filepath.Clean` + prefix check:** Clean the joined path and verify it has the expected directory prefix, with a path separator suffix to prevent partial prefix matches (e.g., `/allowed/dir` matching `/allowed/directory-other`).
* **Reject Null Bytes and Encoded Traversals:** Strip or reject inputs containing `\x00`, `%2e%2e`, `%2f`, or any URL-encoded traversal sequences before path resolution.

### 2. Database / Storage — Filename Only, No Paths

* **Strip Directory Component:** Before persisting any file reference to a database, cache, or configuration store, call `filepath.Base()` to discard any directory prefix supplied by the user.
* **Re-derive Paths at Runtime:** The full access path must be constructed server-side by joining a server-controlled root with the stored bare filename. The stored value must never be used as-is to open a file.

### 3. In-Memory File Processing (No Intermediate Filesystem Writes)

* **Prefer `bytes.Buffer` / `io.Reader` Pipelines:** When parsing, transforming, or hashing uploaded content in Go, pipe data through in-memory readers rather than writing to `os.TempFile` or a persistent path.
* **`os.CreateTemp` is Last Resort:** Only write to disk if a third-party library requires a file path. Immediately `defer os.Remove(tmp.Name())` and keep the file in a tightly scoped directory controlled by the application.
* **No User-Controlled Filenames on Disk:** Never derive a temp file path from user input. Use OS-generated names (e.g., `os.CreateTemp("", "upload-*")`).

### 4. ZIP / Archive File Handling — Specific-File Restriction

* **Allowlist Entry Paths:** When processing a ZIP or tar archive, validate every entry's `Name` field against an explicit allowlist or regex of permitted relative paths. Reject any entry whose cleaned path escapes the destination root (the "zip slip" attack).
* **Single-File Extraction:** If the API is designed to read one specific file from an archive, locate that entry by exact name and discard all others without extracting them.
* **Reject Absolute Paths and Traversals in Entry Names:** Any archive entry whose name starts with `/`, contains `..`, or resolves outside the target directory must be rejected immediately and an error returned — partial extraction must not occur.
* **Limit Entry Count and Compressed Ratio:** Apply a maximum entry count and a decompression ratio guard to mitigate zip bomb attacks.

### 5. Configurable Input Stream Size Limits

* **Never Read Unbounded Streams:** Wrap every `io.Reader` that originates from user or network input with `io.LimitReader` before reading into memory.
* **Externalize Limits to Configuration:** The byte ceiling must come from application configuration (environment variable, config file) — never hardcoded. Provide a safe default that is used when the configuration key is absent.
* **Return a Meaningful Error on Overflow:** If the limit is hit, return `HTTP 413 Request Entity Too Large` with a generic message. Do not expose the configured limit value in the error response.

### 6. Uploaded Content Type Allowlisting and No Dynamic Code Execution on User Input

* **Content-Sniff Before Trusting an Extension or Declared MIME Type:** Validate an uploaded file's actual byte content (magic-byte/structure sniffing) against an explicit allowlist of accepted types before storing or processing it — never trust the client-declared `Content-Type` header or the filename extension alone.
* **Never Feed User Input to a Script/Expression/Template Engine Without a Sandbox:** If a feature accepts a script, expression, or template body from a request (a policy mediator, a transformation rule, an "execute this snippet" feature), never `eval`-equivalent it against the full language runtime. Execute it only inside an engine configured with an explicit allowlist of reachable classes/methods/built-ins — a blocklist of "dangerous" symbols is not sufficient, because the reachable surface of a general-purpose runtime is too large to enumerate exhaustively.
* **Allowlist, Not Blocklist, for Reflective/Class Access:** Where a scripting or mediation feature must expose part of the Go runtime or a plugin API to user-supplied code, gate it with an explicit allowlist of permitted symbols. A blocklist approach only stops the specific bypasses already known at the time it was written.
* **Built-in Template Functions That Access Secrets or Environment Are Not Exempt:** A template-engine builtin such as `{{ secret "handle" }}` or `{{ env "NAME" }}` is itself a reflective-access primitive under this directive, even when the surrounding engine is otherwise sandboxed against arbitrary code execution. Scope `env`-style functions to an explicit allowlist of permitted variable names (reject and log any name not on the list — never return an unlisted environment variable's value). Scope `secret`-style functions to check the requesting resource's own owner/tenant handle against the secret's recorded owner *before* resolving it — never resolve any secret reachable within the same tenant purely because the caller is authenticated within that tenant; require an explicit per-resource or per-grant ACL, not tenant-wide reachability.
* **Treat "Admin-Only" Script Features the Same as Any Other Untrusted Input:** A feature reachable only by an authenticated administrator is still processing untrusted input from this rule's perspective — the authorization boundary controls *who* can reach the feature, not whether the feature itself is safe to execute arbitrary code.

### 7. Streaming Decompression-Bomb Protection (Not Just ZIP Archives)

* **Cap Decompressed Output Size for Any Streamed Content-Encoding:** Any code that decompresses a `Content-Encoding: gzip`/`br`/`deflate` body passing through the service — a proxy body-transformation step, an `ext_proc`-style body phase, a request/response rewriting policy — must bound the *decompressed* output, not just the compressed input. Wrap the decompressing reader with `io.LimitReader` (or an equivalent running byte counter) sized from configuration, and reject with a `413`-equivalent response once the ceiling is exceeded, rather than accumulating an unbounded `io.ReadAll` of the decompressed stream.
* **Guard the Ratio Before Committing to Full Decompression:** Compare the compressed size (`Content-Length` or observed byte count) against the configured decompressed ceiling using a maximum allowed ratio, and reject before decompression begins if the theoretical worst case would exceed it — the same ratio-guard principle directive 4 requires for ZIP entries, applied to any streaming decompressor a request or response body passes through.
* **Streaming (Chunked) Decompression Needs the Same Ceiling as Buffered:** A streaming decompression path (a goroutine emitting decompressed chunks as they arrive) must track *cumulative* emitted bytes across the entire stream and abort the instant the cap is hit. A cap checked only once, at the start of the stream, does not protect a streaming path — the bomb is in the bytes emitted over the stream's lifetime, not in its first chunk.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: Path traversal, storing full path, unbounded read, zip slip, hardcoded limit.
func ServeUserFile(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("file")
    path := "/var/app/uploads/" + name // No path cleaning or containment check
    data, _ := os.ReadFile(path)
    w.Write(data)
}

func SaveFileMeta(db *sql.DB, uploadPath string) {
    db.Exec("INSERT INTO files (path) VALUES (?)", uploadPath) // Stores full path, not just filename
}

func ProcessUpload(r *http.Request) {
    body, _ := io.ReadAll(r.Body) // Unbounded — susceptible to OOM
    processBytes(body)
}

func ExtractZip(src, destDir string) {
    zr, _ := zip.OpenReader(src)
    for _, f := range zr.File {
        outPath := filepath.Join(destDir, f.Name) // No entry-path validation — zip slip possible
        rc, _ := f.Open()
        out, _ := os.Create(outPath)
        io.Copy(out, rc) // No decompression ratio guard
    }
}

func AcceptUpload(w http.ResponseWriter, r *http.Request, upload []byte, declaredType string) {
    // Trusts the client-declared Content-Type / extension with no byte-level check.
    saveUploadedFile(upload, declaredType)
}

func RunScriptMediator(userScript string, ctx *MessageContext) error {
    // Executes user-supplied script text against the full scripting-engine runtime
    // with no allowlist of reachable classes/methods — arbitrary code execution.
    engine := scripting.NewEngine()
    return engine.Eval(userScript, ctx)
}
```

### Best Practice (What to Generate)

```go
// config.go — stream limits come from application configuration
type FileConfig struct {
    UploadRootDir      string
    MaxUploadBytes     int64
    MaxZipEntries      int
    MaxZipRatio        float64
}

func LoadFileConfig() FileConfig {
    maxBytes, _ := strconv.ParseInt(os.Getenv("MAX_UPLOAD_BYTES"), 10, 64)
    if maxBytes <= 0 {
        maxBytes = 10 << 20 // 10 MiB safe default
    }
    maxEntries, _ := strconv.Atoi(os.Getenv("MAX_ZIP_ENTRIES"))
    if maxEntries <= 0 {
        maxEntries = 500
    }
    maxRatio, _ := strconv.ParseFloat(os.Getenv("MAX_ZIP_RATIO"), 64)
    if maxRatio <= 0 {
        maxRatio = 20.0
    }
    return FileConfig{
        UploadRootDir:  os.Getenv("UPLOAD_ROOT_DIR"),
        MaxUploadBytes: maxBytes,
        MaxZipEntries:  maxEntries,
        MaxZipRatio:    maxRatio,
    }
}

// GOOD: Path traversal prevention — containment check.
func safeJoin(root, userInput string) (string, error) {
    // Strip any null bytes or encoded separators before joining
    cleaned := filepath.Clean(filepath.Join(root, filepath.FromSlash(path.Clean("/"+userInput))))
    // root must end with separator for prefix check to be exact
    rootWithSep := filepath.Clean(root) + string(filepath.Separator)
    if !strings.HasPrefix(cleaned, rootWithSep) {
        return "", fmt.Errorf("path escapes root directory")
    }
    return cleaned, nil
}

func ServeUserFile(cfg FileConfig, w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("file")
    safePath, err := safeJoin(cfg.UploadRootDir, name)
    if err != nil {
        http.Error(w, "Not Found", http.StatusNotFound)
        return
    }
    http.ServeFile(w, r, safePath)
}

// GOOD: Store only the bare filename, never the full path.
func SaveFileMeta(db *sql.DB, uploadPath string) error {
    bareFilename := filepath.Base(uploadPath)
    _, err := db.Exec("INSERT INTO files (name) VALUES (?)", bareFilename)
    return err
}

// GOOD: Derive the real path server-side from the stored bare filename.
func OpenStoredFile(cfg FileConfig, db *sql.DB, fileID int) ([]byte, error) {
    var name string
    if err := db.QueryRow("SELECT name FROM files WHERE id = ?", fileID).Scan(&name); err != nil {
        return nil, err
    }
    safePath, err := safeJoin(cfg.UploadRootDir, name)
    if err != nil {
        return nil, fmt.Errorf("invalid stored filename")
    }
    return os.ReadFile(safePath)
}

// GOOD: In-memory processing — no intermediate disk write.
func ProcessUpload(cfg FileConfig, r *http.Request) ([]byte, error) {
    limited := io.LimitReader(r.Body, cfg.MaxUploadBytes+1)
    data, err := io.ReadAll(limited)
    if err != nil {
        return nil, err
    }
    if int64(len(data)) > cfg.MaxUploadBytes {
        return nil, fmt.Errorf("payload exceeds maximum allowed size")
    }
    // Work on data entirely in memory; no os.WriteFile / os.TempFile here.
    return processBytes(data), nil
}

// GOOD: ZIP extraction with zip-slip protection, entry limit, and decompression ratio guard.
var ErrZipSlip = fmt.Errorf("archive entry escapes destination directory")

func ExtractSingleEntry(cfg FileConfig, zipData []byte, entryName, destDir string) error {
    zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
    if err != nil {
        return err
    }

    if len(zr.File) > cfg.MaxZipEntries {
        return fmt.Errorf("archive exceeds maximum entry count")
    }

    for _, f := range zr.File {
        // Reject absolute paths and traversal sequences in entry names
        cleanedEntry := path.Clean("/" + f.Name)
        if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
            return ErrZipSlip
        }

        // Strip the leading slash added by path.Clean so comparison is against a relative path
        relEntry := strings.TrimPrefix(cleanedEntry, "/")

        // Only extract the one requested entry — match by full normalized relative path,
        // not just basename, to avoid picking the wrong entry when names collide across dirs.
        if relEntry != path.Clean(entryName) {
            continue
        }

        destPath, err := safeJoin(destDir, relEntry)
        if err != nil {
            return ErrZipSlip
        }

        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer rc.Close()

        // Decompression ratio guard: take the tighter of ratio-based limit and MaxUploadBytes.
        // Cap (not widen) at MaxUploadBytes so small compressed entries cannot expand arbitrarily.
        maxDecompressed := int64(float64(f.CompressedSize64) * cfg.MaxZipRatio)
        if maxDecompressed > cfg.MaxUploadBytes {
            maxDecompressed = cfg.MaxUploadBytes
        }
        limited := io.LimitReader(rc, maxDecompressed+1)
        data, err := io.ReadAll(limited)
        if err != nil {
            return err
        }
        if int64(len(data)) > maxDecompressed {
            return fmt.Errorf("decompressed entry exceeds allowed ratio")
        }

        return os.WriteFile(destPath, data, 0600)
    }
    return fmt.Errorf("requested entry not found in archive")
}

// GOOD: Content-sniff the actual bytes against an explicit allowlist — never
// trust the client-declared Content-Type header or filename extension alone.
var allowedUploadTypes = map[string]bool{
    "image/png":  true,
    "image/jpeg": true,
    // "image/svg+xml" deliberately excluded — SVG is XML and can carry
    // executable content; see js-output-encoding-xss.md for the JS-side
    // handling required if SVG upload is ever added.
}

func AcceptUpload(w http.ResponseWriter, r *http.Request, upload []byte) error {
    sniffed := http.DetectContentType(upload) // Inspects the actual bytes, not the header
    if !allowedUploadTypes[sniffed] {
        return fmt.Errorf("unsupported upload content type")
    }
    return saveUploadedFile(upload, sniffed)
}

// GOOD: A scripting/mediation feature restricted to an explicit allowlist of
// reachable symbols — never the full runtime, and never gated by a blocklist
// of "known dangerous" symbols alone.
var allowedScriptSymbols = map[string]bool{
    "ctx.GetProperty": true,
    "ctx.SetProperty": true,
    "math.Round":      true,
    // Anything not explicitly listed here is unreachable from user scripts —
    // e.g. no filesystem, network, process, or reflection access.
}

func RunScriptMediator(userScript string, ctx *MessageContext) error {
    engine := scripting.NewSandboxedEngine(allowedScriptSymbols) // Allowlist enforced by the engine itself
    return engine.Eval(userScript, ctx)
}

// GOOD: streaming decompression of a proxied request/response body, bounded on
// the *decompressed* side with a running byte counter, a ratio guard, and a
// hard reject rather than an unbounded io.ReadAll of the inflated stream.
type DecompressionConfig struct {
    MaxDecompressedBytes int64
    MaxRatio             float64
}

func DecompressBoundedGzip(cfg DecompressionConfig, compressedSize int64, r io.Reader) ([]byte, error) {
    if cfg.MaxDecompressedBytes <= 0 {
        return nil, fmt.Errorf("invalid decompression config: MaxDecompressedBytes must be positive")
    }
    if cfg.MaxRatio <= 0 {
        return nil, fmt.Errorf("invalid decompression config: MaxRatio must be positive")
    }
    if compressedSize < 0 {
        return nil, fmt.Errorf("invalid compressed size: must not be negative")
    }

    // Ratio guard before committing to decompression at all. Computed in
    // float64 and range-checked before converting to int64 — compressedSize *
    // MaxRatio can exceed math.MaxInt64, and an out-of-range float-to-int64
    // conversion is undefined by the Go spec, not merely clamped.
    ceiling := cfg.MaxDecompressedBytes
    if compressedSize > 0 {
        ratioLimitFloat := float64(compressedSize) * cfg.MaxRatio
        if ratioLimitFloat > 0 && ratioLimitFloat < float64(ceiling) {
            if ratioLimitFloat > float64(math.MaxInt64) {
                return nil, fmt.Errorf("ratio limit overflow for compressed size %d", compressedSize)
            }
            ceiling = int64(ratioLimitFloat)
        }
    }
    if ceiling >= math.MaxInt64 {
        return nil, fmt.Errorf("invalid decompression ceiling: too large")
    }

    gz, err := gzip.NewReader(r)
    if err != nil {
        return nil, fmt.Errorf("invalid gzip stream: %w", err)
    }
    defer gz.Close()

    limited := io.LimitReader(gz, ceiling+1) // Bounds the DECOMPRESSED side; safe from overflow since ceiling < math.MaxInt64 is checked above
    data, err := io.ReadAll(limited)
    if err != nil {
        return nil, err
    }
    if int64(len(data)) > ceiling {
        return nil, fmt.Errorf("decompressed body exceeds allowed size") // 413-equivalent to the caller
    }
    return data, nil
}

// GOOD: a streaming (chunked) decompression path tracks cumulative emitted
// bytes across the whole stream — a cap checked only at stream start would
// not catch a bomb that expands gradually over many chunks. It also takes
// compressedSize and applies the SAME ratio guard as DecompressBoundedGzip —
// per directive 7, a streaming path needs the identical ceiling as a buffered
// one, not merely the flat MaxDecompressedBytes cap on its own.
func StreamDecompressBounded(cfg DecompressionConfig, compressedSize int64, r io.Reader, emit func([]byte) error) error {
    // Ratio guard before committing to decompression at all — identical to
    // DecompressBoundedGzip's guard, just applied to the streaming path too.
    ratioLimit := int64(float64(compressedSize) * cfg.MaxRatio)
    ceiling := cfg.MaxDecompressedBytes
    if ratioLimit > 0 && ratioLimit < ceiling {
        ceiling = ratioLimit
    }

    gz, err := gzip.NewReader(r)
    if err != nil {
        return fmt.Errorf("invalid gzip stream: %w", err)
    }
    defer gz.Close()

    var totalEmitted int64
    buf := make([]byte, 32*1024)
    for {
        n, readErr := gz.Read(buf)
        if n > 0 {
            totalEmitted += int64(n)
            if totalEmitted > ceiling {
                return fmt.Errorf("decompressed stream exceeds allowed size") // Abort mid-stream
            }
            if err := emit(buf[:n]); err != nil {
                return err
            }
        }
        if readErr == io.EOF {
            return nil
        }
        if readErr != nil {
            return readErr
        }
    }
}
```

---

> **Verification Checklist before outputting code:**
> * Is every file path resolved with `filepath.Clean` and checked against the root with a separator-suffixed prefix? (If no, add `safeJoin`).
> * Is only the bare filename (`filepath.Base`) stored in the database or any external storage? (If the full path is stored, strip it).
> * Is file processing done via `io.Reader` pipelines without intermediate `os.WriteFile` / `os.TempFile`? (If disk writes exist, remove them unless a third-party library strictly requires a path).
> * Are all archive entries validated against the destination root before extraction? (If not, apply `safeJoin` on every entry).
> * Does any code path decompress a `gzip`/`br`/`deflate` body (proxy transformation, `ext_proc` body phase, response rewriting) with `io.ReadAll` on the decompressed reader and no size ceiling? (If yes, wrap it per directive 7 — bound the decompressed side, not just the compressed input, and apply the same bound to streaming/chunked decompression paths.)
> * Is every inbound `io.Reader` wrapped in `io.LimitReader` with a limit sourced from configuration? (If hardcoded or absent, externalize to config with a safe default).
> * Is an uploaded file's actual content sniffed (`http.DetectContentType`) against an allowlist, rather than trusting the declared `Content-Type` header or filename extension? (If trusted as-is, add content-sniffing.)
> * Does any feature evaluate user-supplied script/expression/template text against the full scripting-engine runtime, or against a blocklist of "dangerous" symbols? (Both are insufficient — require an explicit allowlist of reachable classes/methods enforced by the engine itself.)
