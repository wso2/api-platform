# Rule: Go Error & Payload Validation Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code responsible for handling HTTP/gRPC responses, error generation, middleware, and payload construction. The goal is to enforce strict security boundaries, prevent information disclosure, and mitigate user enumeration attacks.

## Directives

1. **Zero internal exposure.** Never expose raw database errors (e.g. `sql.ErrNoRows`), stack traces, internal microservice names, network topologies, or file system paths to the client. Wrap internal errors with `fmt.Errorf("...: %w", err)` for internal logging only, and map them to a sterile, user-facing error object before marshaling.
2. **No vendor/infrastructure headers.** Never forward or generate cloud/gateway/CDN-specific headers in client-facing responses (`X-Amz-*`, `X-Cloudflare-*`, `Cf-Ray`, `X-Vercel-*`). Use only standard HTTP headers or a platform-agnostic internal header like `X-Request-ID`.
3. **No source-tagged dynamic IDs.** Tracking IDs, error tokens, and correlation keys must never embed source file, function name, environment, or developer identifiers. Generate them with a high-entropy random source (UUIDv4, `crypto/rand`, ULID) — never `"AUTH_FAILED_LINE_82_" + timestamp`.
4. **Unified authentication failures.** Every auth failure — invalid, expired, missing, or revoked credential — must return the identical `HTTP 401` status and payload: `{"error": "unauthorized", "message": "Invalid or expired credentials."}`. Log the specific reason internally for debugging; the response writer itself must never branch on failure type.
5. **Never disclose a secret/resource handle on resolution failure.** A failed secret/key/credential lookup must not echo the handle (or any substring) back in the client response — a handle alone can confirm a tenant resource's existence, an enumeration primitive. Don't log the raw handle in the standard internal log either (broad readership, often forwarded to third-party aggregation); log a keyed-hash form there and reserve the raw handle for a narrowly access-controlled audit sink. Where practical, make the response shape (and latency) identical whether the resource doesn't exist or exists but failed to resolve — the same unified-response principle as directive 4, applied to any existence-sensitive lookup.

## Example

```go
// BAD: leaks DB error, vendor header, distinct auth-failure reasons, source-tagged ID, echoes secret handle.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
    if err := authenticateUser(r); err == ErrTokenExpired {
        w.Header().Set("X-AWS-Gateway-Error", "true")
        json.NewEncoder(w).Encode(map[string]string{
            "code": "AUTH_FAILED_EXPIRED_TOKEN_MAIN_GO_L82",
            "message": "Your token has expired. Please log in again.",
        })
    } else if err == sql.ErrNoRows {
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) // raw DB error
    }
}
func HandleSecretResolution(w http.ResponseWriter, handle string, err error) {
    json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to resolve secret %q: %v", handle, err)})
}

// GOOD: sterile payload, generic auth response, anonymous tracking ID, hashed handle for logs only.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    if err := authenticateUser(r); err != nil {
        logger.LogInternalError(ctx, "authentication failed: %v", err) // specific reason, internal only
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "unauthorized", "message": "Invalid or expired credentials.",
            "tracking_id": uuid.New().String(), // pure random ID, no source markers
        })
        return
    }
}

func HandleSecretResolution(ctx context.Context, w http.ResponseWriter, handle string, err error) {
    if err != nil {
        logger.LogInternalError(ctx, "secret resolution failed for handle_hash %s: %v", hashHandle(handle), err)
        auditLogger.Record(ctx, "secret_resolution_failed", handle, err) // restricted sink only
        w.WriteHeader(http.StatusUnprocessableEntity)
        json.NewEncoder(w).Encode(map[string]string{
            "error": "resolution_failed", "message": "The referenced secret could not be resolved.",
        })
    }
}

func hashHandle(handle string) string {
    mac := hmac.New(sha256.New, handleLogHMACKey) // keyed hash, key from config/secret store
    mac.Write([]byte(handle))
    return hex.EncodeToString(mac.Sum(nil))[:16]
}
```

> **Verification Checklist before outputting code:**
> * Does any response reveal raw DB errors, stack traces, or internal paths?
> * Does any response set or forward a vendor/infrastructure header?
> * Does a generated tracking ID/token embed a source file, function, or timestamp-only value?
> * Do different auth-failure causes produce different status codes or payloads?
> * Does an error response (or its internal log) include a secret/resource handle instead of a hashed, audit-only reference?
