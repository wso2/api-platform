// Command mock-oauth2-idp is a minimal, in-memory OAuth2 identity provider
// used to manually test the oauth2 gateway policy end to end. It is
// intentionally NOT a spec-complete OAuth2 server — it implements exactly
// the surface the policy needs for both grants it supports (RFC 6749
// Section 4.4 client_credentials and Section 4.3 password), plus a small
// debug API so test flows can assert on gateway behavior (caching,
// refresh, failure handling) from the outside.
//
// Configured clients:
//   - valid client:      id=<CLIENT_ID>      secret=<CLIENT_SECRET>      -> 200 OK, fresh/cached token
//   - broken client:     id="broken-client"                              -> 500 Internal Server Error (simulates IdP outage)
//   - malformed client:  id="malformed-client"                           -> 200 OK, body missing access_token
//   - any other id/secret combination                                    -> 400, {"error":"invalid_client"}
//
// For grant_type=password, the resource owner's username/password are
// additionally checked against RESOURCE_OWNER_USERNAME/RESOURCE_OWNER_PASSWORD
// (default "resource-owner"/"hunter2") - a mismatch returns 400 invalid_grant.
//
// CLIENT_ID / CLIENT_SECRET default to "test-client" / "test-secret" and can
// be overridden via environment variables of the same name.
//
// Endpoints:
//
//	POST /oauth2/token   grant_type=client_credentials or password, client_secret_basic
//	                     OR client_secret_post, optional `ttl` (seconds, default 300),
//	                     `scope`, `delayMs` (artificially delay the response - test
//	                     tokenRequestTimeout), `omitExpiresIn` (drop expires_in
//	                     from the response entirely - test defaultTokenTTL), and
//	                     `failFirstN` (fail this many requests with a transient
//	                     500 before succeeding - test tokenRequestMaxRetries) params.
//	                     password grant additionally requires `username`/`password`
//	                     form fields. Every non-standard request header (anything
//	                     other than Authorization/Content-Type/Content-Length) is
//	                     captured and echoed back via GET /debug/stats - test
//	                     tokenRequestHeaders.
//	GET  /debug/stats    JSON summary of every token request received so far —
//	                     use this to confirm the gateway cached a token instead
//	                     of calling the IdP on every request, and to confirm a
//	                     refresh happened after expiry.
//	POST /debug/reset    Clears the request history (call between test flows).
//	GET  /healthz        Liveness probe.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// defaultTTLSeconds matches the doc comment above ("default 300"). Must stay
// comfortably above the oauth2-generator policy's own default expiryBuffer
// (30s) - every test that relies on the mock's default expires_in (i.e.
// doesn't pass its own ?ttl=) also relies on repeated calls within that
// window being served from cache, which expiryBuffer would otherwise defeat
// the moment this value gets close to (or below) 30s.
const defaultTTLSeconds = 300

var (
	validClientID     = envOr("CLIENT_ID", "test-client")
	validClientSecret = envOr("CLIENT_SECRET", "test-secret")

	// validUsername/validPassword are the resource-owner credentials
	// accepted for grant_type=password (RFC 6749 Section 4.3).
	validUsername = envOr("RESOURCE_OWNER_USERNAME", "resource-owner")
	validPassword = envOr("RESOURCE_OWNER_PASSWORD", "hunter2")

	mu          sync.Mutex
	tokenSeq    int
	history     []tokenRequestRecord
	failCounter int // requests failed so far under the current failFirstN - see handleToken
)

// tokenRequestRecord captures one /oauth2/token call for later inspection via
// GET /debug/stats — this is what lets a curl-driven test prove caching or
// refresh behavior without reading gateway logs.
type tokenRequestRecord struct {
	Time      time.Time         `json:"time"`
	ClientID  string            `json:"clientId"`
	AuthStyle string            `json:"authStyle"` // "basic" or "post"
	Scope     string            `json:"scope,omitempty"`
	Outcome   string            `json:"outcome"` // "issued", "invalid_client", "malformed", "server_error", "forced_failure"
	Token     string            `json:"token,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"` // non-standard request headers - see extractCustomHeaders
}

// standardTokenRequestHeaders are excluded from the captured Headers map -
// they're either already represented elsewhere in tokenRequestRecord
// (Authorization -> AuthStyle) or are plain HTTP/transport mechanics with no
// test value.
var standardTokenRequestHeaders = map[string]bool{
	"Authorization":   true,
	"Content-Type":    true,
	"Content-Length":  true,
	"Accept-Encoding": true,
	"User-Agent":      true,
	"Host":            true,
}

// extractCustomHeaders captures every header on a token request that isn't
// one of the standard/already-tracked ones above - this is how a test proves
// tokenRequestHeaders actually reached the token endpoint.
func extractCustomHeaders(r *http.Request) map[string]string {
	captured := map[string]string{}
	for name, values := range r.Header {
		if standardTokenRequestHeaders[http.CanonicalHeaderKey(name)] || len(values) == 0 {
			continue
		}
		captured[name] = values[0]
	}
	if len(captured) == 0 {
		return nil
	}
	return captured
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// maskSecret keeps only enough of a credential/header to correlate log lines
// without leaking the value itself (see GO-AUTH-003).
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "[MASKED]"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

// loggingMiddleware logs every inbound request (method, path, remote addr,
// masked Authorization header) so a manual test run has a full audit trail
// of what actually reached the mock IdP.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("request: method=%s path=%s remote=%s authorization=%s",
			r.Method, r.URL.Path, r.RemoteAddr, maskSecret(r.Header.Get("Authorization")))
		next.ServeHTTP(w, r)
	})
}

func main() {
	addr := envOr("ADDR", ":9601")
	tlsCertFile := os.Getenv("TLS_CERT_FILE")
	tlsKeyFile := os.Getenv("TLS_KEY_FILE")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth2/token", handleToken)
	mux.HandleFunc("GET /debug/stats", handleStats)
	mux.HandleFunc("POST /debug/reset", handleReset)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("mock-oauth2-idp listening on %s (valid client: %s / %s)", addr, validClientID, validClientSecret)

	// TLS_CERT_FILE/TLS_KEY_FILE (both required together) switch this mock
	// to HTTPS - used to test the policy's tlsCaCert (trust a private
	// CA) and tlsInsecureSkipVerify params, neither of which have any
	// effect against a plain-HTTP token endpoint. See TESTING.md for how to
	// generate a self-signed cert for this.
	if tlsCertFile != "" && tlsKeyFile != "" {
		log.Print("TLS enabled - token endpoint: https://<this-host>/oauth2/token")
		log.Fatal(http.ListenAndServeTLS(addr, tlsCertFile, tlsKeyFile, loggingMiddleware(mux)))
	}
	log.Print("token endpoint: http://<this-host>/oauth2/token")
	log.Fatal(http.ListenAndServe(addr, loggingMiddleware(mux)))
}

func handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "failed to parse form body")
		return
	}

	// delayMs (query param or form field, like ttl) artificially delays this
	// handler before doing anything else - simulates a slow/hung IdP to
	// exercise the policy's tokenRequestTimeout. Applied first, before any
	// validation, so it delays the response regardless of whether the
	// request would otherwise succeed or fail. Cancelable via the request's
	// context - once the caller (the gateway's own tokenRequestTimeout)
	// gives up and disconnects, this returns immediately instead of running
	// to completion and recording a stray, late entry into whatever test's
	// debug history happens to be open several seconds later.
	if v := r.FormValue("delayMs"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			select {
			case <-time.After(time.Duration(parsed) * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		}
	}

	clientID, clientSecret, authStyle, err := extractClientCredentials(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_client", err.Error())
		recordRequest(r, clientID, authStyle, r.PostForm.Get("scope"), "invalid_client", "")
		return
	}

	grantType := r.PostForm.Get("grant_type")
	if grantType != "client_credentials" && grantType != "password" {
		writeJSONError(w, http.StatusBadRequest, "unsupported_grant_type", "only client_credentials and password are supported by this mock")
		recordRequest(r, clientID, authStyle, r.PostForm.Get("scope"), "invalid_client", "")
		return
	}

	// For the password grant, the resource owner's username/password are
	// additional required fields alongside client authentication - checked
	// against the same valid client_id/client_secret below, plus a fixed
	// valid resource-owner pair (overridable via RESOURCE_OWNER_USERNAME /
	// RESOURCE_OWNER_PASSWORD).
	if grantType == "password" {
		username := r.PostForm.Get("username")
		password := r.PostForm.Get("password")
		if username != validUsername || password != validPassword {
			writeJSONError(w, http.StatusBadRequest, "invalid_grant", "resource owner credentials are invalid")
			recordRequest(r, clientID, authStyle, r.PostForm.Get("scope"), "invalid_client", "")
			return
		}
	}

	scope := r.PostForm.Get("scope")

	switch clientID {
	case "broken-client":
		recordRequest(r, clientID, authStyle, scope, "server_error", "")
		http.Error(w, "internal server error (simulated IdP outage)", http.StatusInternalServerError)
		return

	case "malformed-client":
		// 200 OK but the body is missing access_token — exercises the
		// policy's "successful fetch, malformed response" failure path.
		recordRequest(r, clientID, authStyle, scope, "malformed", "")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token_type":"Bearer","expires_in":300}`))
		return

	case validClientID:
		if clientSecret != validClientSecret {
			writeJSONError(w, http.StatusBadRequest, "invalid_client", "client secret does not match")
			recordRequest(r, clientID, authStyle, scope, "invalid_client", "")
			return
		}
		// fall through to issue a token

	default:
		writeJSONError(w, http.StatusBadRequest, "invalid_client", "unknown client_id")
		recordRequest(r, clientID, authStyle, scope, "invalid_client", "")
		return
	}

	// failFirstN (query param or form field, like ttl) fails this many
	// otherwise-valid requests with a transient 500 before letting one
	// through - exercises the policy's tokenRequestMaxRetries. The counter
	// is process-wide (reset via POST /debug/reset), not per-client, since
	// a test only ever drives one client through this at a time.
	if v := r.FormValue("failFirstN"); v != "" {
		if failFirstN, err := strconv.Atoi(v); err == nil && failFirstN > 0 {
			mu.Lock()
			shouldFail := failCounter < failFirstN
			if shouldFail {
				failCounter++
			}
			mu.Unlock()
			if shouldFail {
				recordRequest(r, clientID, authStyle, scope, "forced_failure", "")
				http.Error(w, "internal server error (simulated transient failure)", http.StatusInternalServerError)
				return
			}
		}
	}

	// FormValue (not PostForm.Get) so `ttl` can be supplied either as a form
	// field in the token request body, or as a query parameter appended to
	// the configured tokenEndpoint (e.g. "...?ttl=2") — the latter is the
	// only practical way to drive a short TTL through a real OAuth2 client
	// library, since libraries generally don't expose a way to add an
	// arbitrary extra body field per grant request.
	ttl := defaultTTLSeconds
	if v := r.FormValue("ttl"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			ttl = parsed
		}
	}

	mu.Lock()
	tokenSeq++
	seq := tokenSeq
	mu.Unlock()

	// The token value embeds a sequence number and issue time so a test can
	// tell, just by comparing the string returned to the gateway's upstream
	// call, whether a cached token was reused or a fresh one was minted.
	token := fmt.Sprintf("mock-token-%d-issued-%d", seq, time.Now().UnixNano())
	recordRequest(r, clientID, authStyle, scope, "issued", token)

	resp := map[string]interface{}{
		"access_token": token,
		"token_type":   "Bearer",
	}
	// omitExpiresIn simulates an IdP that doesn't return expires_in at all -
	// exercises the policy's defaultTokenTTL fallback. ttl still governs
	// nothing about the response in that case; it's simply not sent.
	if r.FormValue("omitExpiresIn") != "true" {
		resp["expires_in"] = ttl
	}
	if scope != "" {
		resp["scope"] = scope
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// extractClientCredentials supports both RFC 6749 client authentication
// conventions: client_secret_basic (Authorization: Basic header) and
// client_secret_post (client_id/client_secret as form fields).
func extractClientCredentials(r *http.Request) (clientID, clientSecret, authStyle string, err error) {
	if user, pass, ok := r.BasicAuth(); ok {
		return user, pass, "basic", nil
	}

	// r.BasicAuth() only succeeds for a well-formed "Basic <base64>" header;
	// if an Authorization header is present but doesn't parse, surface that
	// distinctly rather than silently falling through to POST-body auth.
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Basic ") {
			if _, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic ")); decodeErr != nil {
				return "", "", "basic", fmt.Errorf("malformed Basic authorization header")
			}
		}
	}

	clientID = r.PostForm.Get("client_id")
	clientSecret = r.PostForm.Get("client_secret")
	if clientID == "" {
		return "", "", "post", fmt.Errorf("no client credentials presented (neither Basic auth nor client_id/client_secret form fields)")
	}
	return clientID, clientSecret, "post", nil
}

func recordRequest(r *http.Request, clientID, authStyle, scope, outcome, token string) {
	headers := extractCustomHeaders(r)
	mu.Lock()
	defer mu.Unlock()
	history = append(history, tokenRequestRecord{
		Time:      time.Now().UTC(),
		ClientID:  clientID,
		AuthStyle: authStyle,
		Scope:     scope,
		Outcome:   outcome,
		Token:     token,
		Headers:   headers,
	})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tokenRequestCount": len(history),
		"history":           history,
	})
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	history = nil
	tokenSeq = 0
	failCounter = 0
	mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func writeJSONError(w http.ResponseWriter, status int, errCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}
