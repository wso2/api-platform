# Rule: Go Error & Payload Validation Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing Go (`.go`) code responsible for handling HTTP/gRPC responses, error generation, middleware, and payload construction. The goal is to enforce strict security boundaries, prevent information disclosure, and mitigate user enumeration attacks.

---

## Directives

### 1. Data Leakage & Internal Exposure

* **Zero Internal Details:** Never expose raw database errors (e.g., `sql.ErrNoRows`), stack traces, internal microservice names, network topologies, or file system paths to the client.
* **Sanitization:** Wrap internal errors using standard Go idiomatic patterns (`fmt.Errorf("something went wrong: %w", err)`) for internal logging, but map them to sterile, user-facing error objects before JSON/XML marshaling.

### 2. Vendor Header Abstraction

* **No Leaky Headers:** Ensure no custom HTTP headers specific to cloud vendors, API gateways, or third-party tools (e.g., `X-Amz-*`, `X-Cloudflare-*`, `Cf-Ray`, `X-Vercel-*`) are forwarded or generated in client-facing error responses.
* **Standardization:** Use only standard HTTP headers or predefined, platform-agnostic internal custom headers (e.g., `X-Request-ID`).

### 3. Dynamic Value Generation & Source Obfuscation

* **No Source Identifiers:** Dynamically generated strings (such as tracking IDs, error tokens, or correlation keys) must not contain hardcoded substrings that identify the source file, function name, environment name, or developer aliases.
* **Implementation:** Use high-entropy random generators (e.g., UUIDv4, crypto/rand, or ULID) for tokens rather than concatenating strings like `"ERR_USER_SERVICE_LINE_42_" + timestamp`.

### 4. Unified Authentication Failures

* **Constant-Time/Constant-Response Auth:** Do not differentiate client-facing responses for authentication failures. Whether a token/credential is invalid, expired, missing, or revoked, the API must return the exact same payload and HTTP status code.
* **Allowed Status:** `HTTP 401 Unauthorized`
* **Standard Payload:**
  ```json
  {
    "error": "unauthorized",
    "message": "Invalid or expired credentials."
  }
  ```

* *Note:* Internal logs can log the specific reason (e.g., "token expired") for debugging, but the HTTP response writer must remain completely generic to prevent credential probing.

### 5. Secret and Sensitive-Handle Non-Disclosure

* **Never Echo a Secret/Resource Handle Back on Resolution Failure:** An error path that fails to resolve a secret, key, credential, or similarly sensitive handle must not include that handle — or any substring of it — in the client-facing response body, even though the handle is not the secret's *value*. A handle is frequently sufficient to confirm the existence (or non-existence) of a specific tenant's resource, which is an enumeration primitive in its own right, and can be correlated with other leaked data. Do not write the raw handle into the standard internal-error log either — that log is typically readable by a broad engineering audience and often forwarded to a third-party log aggregation platform, the same concern GO-AUTH-003 raises for raw tokens. Log a redacted or keyed-hash form of the handle for correlation, and reserve the raw handle for a narrowly access-controlled audit sink, used only when forensic investigation strictly requires it.
* **Uniform Response Shape for Present-vs-Absent Resources:** Where practical, make the client-facing failure response — and its approximate latency — the same whether a referenced secret/resource exists but failed to resolve for an internal reason, or does not exist at all. This is the same unified-response principle as Directive 4 (constant-response auth failures), applied to any resource-existence-sensitive lookup, not only login.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: Leaks internal DB state, uses a vendor header, reveals specific auth failure, and hardcodes source tags.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
    err := authenticateUser(r)
    if err == ErrTokenExpired {
        w.Header().Set("X-AWS-Gateway-Error", "true") // Leaky vendor header
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "code":    "AUTH_FAILED_EXPIRED_TOKEN_MAIN_GO_L82", // Source-tagged, guessable ID
            "message": "Your token has expired. Please log in again.", // Reveals specific failure reason
        })
        return
    }
    if err == sql.ErrNoRows {
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) // Raw DB error exposed
        return
    }
}

// BAD: secret-resolution failure echoes the secret handle back to the caller —
// an enumeration primitive even though the secret's value itself isn't leaked.
func HandleTemplateSecretResolution(w http.ResponseWriter, handle string, err error) {
    if err != nil {
        json.NewEncoder(w).Encode(map[string]string{
            "error": fmt.Sprintf("failed to resolve secret %q: %v", handle, err),
        })
    }
}

```

### Best Practice (What to Generate)

```go
// GOOD: Sterile payloads, generic auth responses, clean headers, and anonymous tracking IDs.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    err := authenticateUser(r)
    
    if err != nil {
        // Log the specific detailed reason internally for developers
        logger.LogInternalError(ctx, "Authentication failed deeply: %v", err)
        
        // Generate a pure, anonymous correlation ID
        trackID := uuid.New().String() 
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "error":        "unauthorized",
            "message":      "Invalid or expired credentials.",
            "tracking_id": trackID,
        })
        return
    }
}

// GOOD: the standard internal log gets only a keyed-hash correlation ID for
// the handle, never the raw handle itself — that log is read by a broad
// engineering audience and often forwarded to a third-party aggregator (the
// same concern GO-AUTH-003 raises for tokens). The raw handle goes only to
// auditLogger, a narrowly access-controlled sink, for forensic escalation.
// The client-facing response stays sterile — identical in shape whether the
// handle doesn't exist or exists but failed to resolve for another reason.
func HandleTemplateSecretResolution(ctx context.Context, w http.ResponseWriter, handle string, err error) {
    if err != nil {
        logger.LogInternalError(ctx, "secret resolution failed for handle_hash %s: %v", hashHandle(handle), err)
        auditLogger.Record(ctx, "secret_resolution_failed", handle, err) // Restricted audit sink only
        w.WriteHeader(http.StatusUnprocessableEntity)
        json.NewEncoder(w).Encode(map[string]string{
            "error":   "resolution_failed",
            "message": "The referenced secret could not be resolved.", // No handle, no distinction by cause
        })
        return
    }
}

// hashHandle gives standard logs a stable correlation identifier without
// disclosing the handle itself to their broad readership.
func hashHandle(handle string) string {
    mac := hmac.New(sha256.New, handleLogHMACKey) // Key sourced from config/secret store, never hardcoded
    mac.Write([]byte(handle))
    return hex.EncodeToString(mac.Sum(nil))[:16]
}

```

---

> **Verification Checklist before outputting code:**
> * Does this error message reveal *why* the auth failed to the client? (If yes, make it generic).
> * Does the generated ID contain hardcoded source markers? (If yes, use a random crypto string/UUID).
> * Are there any `X-Amz` or similar infrastructure headers bleeding through? (If yes, strip them).
> * Does any client-facing error response include a secret handle, key identifier, or other sensitive resource reference — even without the underlying secret value? (If yes, strip it from the response body.) Does the standard internal-error log then get the raw handle instead? (Log a redacted/keyed-hash form there, and reserve the raw handle for a narrowly access-controlled audit sink.)
> 