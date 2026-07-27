# Rule: Go File Access Security Standards

## Context & Scope

Apply whenever writing, refactoring, or reviewing Go (`.go`) code that handles file reads, uploads, archive extraction, database storage of file metadata, or any operation touching the filesystem or processing byte streams from user input. Goal: prevent path traversal, information disclosure via filesystem access, and resource exhaustion via unbounded stream consumption.

## Directives

1. **Path traversal prevention.** Resolve the final absolute path and assert it is strictly within the intended root before any `os.Open`. Use `filepath.Clean` + a prefix check against the root with a trailing separator (so `/allowed/dir` can't match `/allowed/directory-other`), and reject null bytes / URL-encoded traversal sequences (`%2e%2e`, `%2f`) before path resolution.
2. **Filename only in storage.** Before persisting any file reference to a DB/cache/config store, strip the directory component with `filepath.Base()`. Re-derive the real path at runtime by joining a server-controlled root with the stored bare filename — never open the stored value as-is.
3. **In-memory processing, no intermediate disk writes.** Pipe uploaded/parsed content through `bytes.Buffer`/`io.Reader` pipelines rather than `os.TempFile`. Treat `os.CreateTemp` as last resort (only when a third-party lib requires a path), always with OS-generated names and `defer os.Remove` — never a user-supplied filename on disk.
4. **ZIP/archive handling (zip-slip).** Validate every entry's `Name` against an explicit allowlist/regex of permitted relative paths; reject any entry whose cleaned path is absolute or contains `..` — return an error rather than skipping, so partial extraction never occurs. If only one file is needed, locate it by exact normalized path and discard the rest. Apply a max entry count and a decompression-ratio guard to stop zip bombs.
5. **Configurable stream size limits.** Wrap every inbound `io.Reader` in `io.LimitReader` before reading into memory. Source the byte ceiling from configuration (env var/config file) with a safe default, never hardcode it, and return `413 Request Entity Too Large` with a generic message on overflow — don't echo the configured limit back.
6. **Content-type allowlisting and no dynamic code execution on user input.** Content-sniff uploaded bytes (`http.DetectContentType`) against an explicit allowlist — never trust the declared `Content-Type` header or extension alone. Never `eval`-equivalent a user-supplied script/expression/template against the full runtime; execute it only inside an engine gated by an explicit allowlist of reachable symbols (a blocklist of "dangerous" symbols is never sufficient). This includes builtins like `{{ secret "handle" }}`/`{{ env "NAME" }}` — scope `env` to an allowlisted variable set and `secret` to an ACL check against the resource's own owner, never tenant-wide reachability. Admin-only script features are still untrusted input under this directive — authorization gates *who* reaches the feature, not whether executing it is safe.
7. **Streaming decompression-bomb protection.** Any code decompressing a `gzip`/`br`/`deflate` body (proxy transform, `ext_proc` phase, response rewrite) must bound the *decompressed* output via `io.LimitReader`/a running counter sized from config — never an unbounded `io.ReadAll` of the inflated stream. Guard the ratio (compressed size × max ratio) before committing to decompression, and for a streaming/chunked path track *cumulative* emitted bytes across the whole stream, not just a check at the first chunk.

## Example

```go
// BAD: path traversal, full path stored, unbounded read, zip slip.
func ServeUserFile(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("file")
    data, _ := os.ReadFile("/var/app/uploads/" + name) // no containment check
    w.Write(data)
}
func ExtractZip(src, destDir string) {
    zr, _ := zip.OpenReader(src)
    for _, f := range zr.File {
        outPath := filepath.Join(destDir, f.Name) // no entry validation — zip slip
        rc, _ := f.Open()
        out, _ := os.Create(outPath)
        io.Copy(out, rc) // no ratio guard
    }
}

// GOOD: containment check + zip-slip protection + ratio guard.
func safeJoin(root, userInput string) (string, error) {
    cleaned := filepath.Clean(filepath.Join(root, filepath.FromSlash(path.Clean("/"+userInput))))
    rootWithSep := filepath.Clean(root) + string(filepath.Separator)
    if !strings.HasPrefix(cleaned, rootWithSep) {
        return "", fmt.Errorf("path escapes root directory")
    }
    return cleaned, nil
}

func ExtractSingleEntry(cfg FileConfig, zipData []byte, entryName, destDir string) error {
    zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
    if err != nil || len(zr.File) > cfg.MaxZipEntries {
        return fmt.Errorf("invalid or oversized archive")
    }
    for _, f := range zr.File {
        if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
            return fmt.Errorf("archive entry escapes destination directory")
        }
        rel := strings.TrimPrefix(path.Clean("/"+f.Name), "/")
        if rel != path.Clean(entryName) {
            continue // skip non-target entries without extracting them
        }
        destPath, err := safeJoin(destDir, rel)
        if err != nil {
            return err
        }
        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer rc.Close()
        maxDecompressed := int64(float64(f.CompressedSize64) * cfg.MaxZipRatio)
        if maxDecompressed > cfg.MaxUploadBytes {
            maxDecompressed = cfg.MaxUploadBytes // cap, never widen
        }
        data, err := io.ReadAll(io.LimitReader(rc, maxDecompressed+1))
        if err != nil || int64(len(data)) > maxDecompressed {
            return fmt.Errorf("decompressed entry exceeds allowed ratio")
        }
        return os.WriteFile(destPath, data, 0600)
    }
    return fmt.Errorf("requested entry not found in archive")
}

// GOOD: store bare filename only; content-sniff against an allowlist.
func SaveFileMeta(db *sql.DB, uploadPath string) error {
    _, err := db.Exec("INSERT INTO files (name) VALUES (?)", filepath.Base(uploadPath))
    return err
}

var allowedUploadTypes = map[string]bool{"image/png": true, "image/jpeg": true}

func AcceptUpload(upload []byte) error {
    if sniffed := http.DetectContentType(upload); !allowedUploadTypes[sniffed] {
        return fmt.Errorf("unsupported upload content type")
    }
    return nil
}
```

> **Verification Checklist before outputting code:**
> * Is every file path resolved with `filepath.Clean` and checked against the root via a separator-suffixed prefix (`safeJoin`)?
> * Is only `filepath.Base()` of the filename stored in the DB, with the real path re-derived server-side?
> * Is upload/parse processing done via in-memory `io.Reader` pipelines, with `os.CreateTemp` only as a last resort and never a user-supplied filename?
> * Are all archive entries validated (no `..`/absolute paths) before extraction, with entry-count and decompression-ratio limits enforced?
> * Is every inbound `io.Reader` wrapped in `io.LimitReader` with a config-sourced limit, returning a generic `413` on overflow?
> * Is uploaded content sniffed via `http.DetectContentType` against an allowlist, and is any user-supplied script/template gated by an explicit symbol allowlist (never a blocklist), including for admin-only features?
> * Does any gzip/br/deflate decompression path bound the decompressed output (ratio guard + cumulative streaming cap), not just an unbounded `io.ReadAll`?
