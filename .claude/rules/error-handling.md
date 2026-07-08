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

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```go
// BAD: Leaks internal DB state, uses vendor headers, reveals specific auth failure, and hardcodes source tags.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
    err := authenticateUser(r)
    if err == ErrTokenExpired {
        w.Header().Set("X-AWS-Gateway-Error", "true")
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{
            "code":    "AUTH_FAILED_EXPIRED_TOKEN_MAIN_GO_L82",
            "message": "Your token has expired. Please log in again.",
        })
        return
    }
    if err == sql.ErrNoRows {
        w.WriteHeader(http.StatusNotFound)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
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

```

---

> **Verification Checklist before outputting code:**
> * Does this error message reveal *why* the auth failed to the client? (If yes, make it generic).
> * Does the generated ID contain hardcoded source markers? (If yes, use a random crypto string/UUID).
> * Are there any `X-Amz` or similar infrastructure headers bleeding through? (If yes, strip them).
> 
>