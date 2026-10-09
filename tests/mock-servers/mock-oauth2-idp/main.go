// Command mock-oauth2-idp is a minimal, in-memory OAuth2 identity provider
// used to manually test the oauth2 gateway policy end to end. It is
// intentionally NOT a spec-complete OAuth2 server — it implements exactly
// the surface the supported policies need across four grants (RFC 6749
// Section 4.4 client_credentials, Section 4.3 password, RFC 8693 Token
// Exchange, and RFC 7523 JWT Bearer), plus a small debug API so test flows
// can assert on gateway behavior (caching, refresh, failure handling) from
// the outside.
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
// For grant_type=urn:ietf:params:oauth:grant-type:token-exchange (RFC 8693),
// `subject_token` and `subject_token_type` are required form fields;
// `subject_token_type`/`requested_token_type` (if present) must each be one
// of the three urn:ietf:params:oauth:token-type:* values this mock knows
// about (access_token, jwt, id_token). `audience`/`resource` may be repeated
// form fields and `scope` is space-delimited, same as the other grants. The
// sentinel subject_token value "invalid-subject-token" returns 400
// invalid_grant, for exercising the exchange-failure path without needing a
// real, independently-forgeable credential.
//
// For grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer (RFC 7523), the
// `assertion` form field is required and is not cryptographically verified —
// this mock trusts whatever is handed to it. The sentinel assertion value
// "invalid-assertion" returns 400 invalid_grant, for the same reason as
// token-exchange's sentinel above.
//
// CLIENT_ID / CLIENT_SECRET default to "test-client" / "test-secret" and can
// be overridden via environment variables of the same name.
//
// Endpoints:
//
//	POST /oauth2/token   grant_type=client_credentials, password,
//	                     urn:ietf:params:oauth:grant-type:token-exchange, or
//	                     urn:ietf:params:oauth:grant-type:jwt-bearer, with
//	                     client_secret_basic OR client_secret_post, optional
//	                     `ttl` (seconds, default 300), `scope`, `delayMs`
//	                     (artificially delay the response - test
//	                     tokenRequestTimeout), `omitExpiresIn` (drop expires_in
//	                     from the response entirely - test defaultTokenTTL), and
//	                     `failFirstN` (fail this many requests with a transient
//	                     500 before succeeding - test tokenRequestMaxRetries) params.
//	                     password grant additionally requires `username`/`password`
//	                     form fields; token-exchange requires `subject_token` and
//	                     `subject_token_type` (plus optional `requested_token_type`,
//	                     `audience`, `resource`); jwt-bearer requires `assertion`.
//	                     Every non-standard request header (anything
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

// maxTokenRequestBytes bounds handleToken's form body - a slow-loris-style
// oversized request must not be able to tie up this mock's single process.
const maxTokenRequestBytes = 1 << 20 // 1 MiB

// grantTypeTokenExchange and grantTypeJWTBearer are the RFC 8693 / RFC 7523
// grant-type URNs this mock accepts in addition to the two plain RFC 6749
// grant names (client_credentials, password).
const (
	grantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
	grantTypeJWTBearer     = "urn:ietf:params:oauth:grant-type:jwt-bearer"
)

// invalidSubjectToken and invalidAssertion are sentinel subject_token/
// assertion values that deliberately fail the exchange with invalid_grant -
// this mock never validates these values cryptographically, so a test that
// wants to exercise an exchange-failure path needs some other way to signal
// "this one should fail" than a real signature check.
const (
	invalidSubjectToken = "invalid-subject-token"
	invalidAssertion    = "invalid-assertion"
)

// validTokenTypeURNs are the RFC 8693 token-type identifiers this mock
// accepts for subject_token_type/requested_token_type. It does not attempt
// to interpret the referenced token differently per type - the value only
// has to be one of these three for the request to be well-formed.
var validTokenTypeURNs = map[string]bool{
	"urn:ietf:params:oauth:token-type:access_token": true,
	"urn:ietf:params:oauth:token-type:jwt":          true,
	"urn:ietf:params:oauth:token-type:id_token":     true,
}

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
	Time     time.Time `json:"time"`
	ClientID string    `json:"clientId"`
	// AuthStyle is "basic" or "post".
	AuthStyle string `json:"authStyle"`
	// GrantType is the raw grant_type form value - the RFC 6749 names
	// ("client_credentials", "password") or one of the RFC 8693/7523 URNs.
	GrantType string `json:"grantType,omitempty"`
	Scope     string `json:"scope,omitempty"`
	// Outcome is one of "issued", "invalid_client", "invalid_request",
	// "invalid_grant", "malformed", "server_error", "forced_failure".
	Outcome string `json:"outcome"`
	Token   string `json:"token,omitempty"`
	// SubjectTokenPreview is a masked (never raw) preview of the
	// token-exchange subject_token / jwt-bearer assertion, present only so a
	// test can confirm the expected credential reached this mock without the
	// full value ever appearing in debug output - see maskSecret.
	SubjectTokenPreview string   `json:"subjectTokenPreview,omitempty"`
	SubjectTokenType    string   `json:"subjectTokenType,omitempty"`
	RequestedTokenType  string   `json:"requestedTokenType,omitempty"`
	Audiences           []string `json:"audiences,omitempty"`
	Resources           []string `json:"resources,omitempty"`
	// Headers holds non-standard request headers - see extractCustomHeaders.
	Headers map[string]string `json:"headers,omitempty"`
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

// envOrDuration parses a positive duration from key, falling back to
// fallback if unset, empty, unparseable, or non-positive.
func envOrDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
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

	log.Printf("mock-oauth2-idp listening on %s (valid client: %s)", addr, validClientID)

	srv := &http.Server{
		Addr:           addr,
		Handler:        loggingMiddleware(mux),
		ReadTimeout:    envOrDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:   envOrDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:    envOrDuration("IDLE_TIMEOUT", 60*time.Second),
		MaxHeaderBytes: 1 << 20,
	}

	// TLS_CERT_FILE/TLS_KEY_FILE (both required together) switch this mock
	// to HTTPS - used to test the policy's tlsCaCert (trust a private
	// CA) and tlsInsecureSkipVerify params, neither of which have any
	// effect against a plain-HTTP token endpoint. See TESTING.md for how to
	// generate a self-signed cert for this.
	if tlsCertFile != "" && tlsKeyFile != "" {
		log.Print("TLS enabled - token endpoint: https://<this-host>/oauth2/token")
		log.Fatal(srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile))
	}
	log.Print("token endpoint: http://<this-host>/oauth2/token")
	log.Fatal(srv.ListenAndServe())
}

func handleToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxTokenRequestBytes)
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
		recordRequest(r, tokenRequestRecord{ClientID: clientID, AuthStyle: authStyle, GrantType: r.PostForm.Get("grant_type"), Scope: r.PostForm.Get("scope"), Outcome: "invalid_client"})
		return
	}

	grantType := r.PostForm.Get("grant_type")
	scope := r.PostForm.Get("scope")
	rec := tokenRequestRecord{ClientID: clientID, AuthStyle: authStyle, GrantType: grantType, Scope: scope}

	switch grantType {
	case "client_credentials", "password", grantTypeTokenExchange, grantTypeJWTBearer:
		// supported - validated further below.
	default:
		writeJSONError(w, http.StatusBadRequest, "unsupported_grant_type", "only client_credentials, password, token-exchange, and jwt-bearer are supported by this mock")
		rec.Outcome = "invalid_client"
		recordRequest(r, rec)
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
			rec.Outcome = "invalid_client"
			recordRequest(r, rec)
			return
		}
	}

	// For token-exchange (RFC 8693), subject_token/subject_token_type are
	// required and must be well-formed; audience/resource/requested_token_type
	// are optional. For jwt-bearer (RFC 7523), assertion is required. Neither
	// credential is cryptographically verified by this mock - the sentinel
	// values below exist purely so a test can force the exchange to fail.
	if grantType == grantTypeTokenExchange {
		subjectToken := r.PostForm.Get("subject_token")
		subjectTokenType := r.PostForm.Get("subject_token_type")
		requestedTokenType := r.PostForm.Get("requested_token_type")
		rec.SubjectTokenPreview = maskSecret(subjectToken)
		rec.SubjectTokenType = subjectTokenType
		rec.RequestedTokenType = requestedTokenType
		rec.Audiences = r.PostForm["audience"]
		rec.Resources = r.PostForm["resource"]

		if subjectToken == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "subject_token is required for the token-exchange grant")
			rec.Outcome = "invalid_request"
			recordRequest(r, rec)
			return
		}
		if !validTokenTypeURNs[subjectTokenType] {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "subject_token_type must be a supported urn:ietf:params:oauth:token-type:* value")
			rec.Outcome = "invalid_request"
			recordRequest(r, rec)
			return
		}
		if requestedTokenType != "" && !validTokenTypeURNs[requestedTokenType] {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "requested_token_type must be a supported urn:ietf:params:oauth:token-type:* value")
			rec.Outcome = "invalid_request"
			recordRequest(r, rec)
			return
		}
		if subjectToken == invalidSubjectToken {
			writeJSONError(w, http.StatusBadRequest, "invalid_grant", "subject_token could not be validated")
			rec.Outcome = "invalid_grant"
			recordRequest(r, rec)
			return
		}
	}

	if grantType == grantTypeJWTBearer {
		assertion := r.PostForm.Get("assertion")
		rec.SubjectTokenPreview = maskSecret(assertion)

		if assertion == "" {
			writeJSONError(w, http.StatusBadRequest, "invalid_request", "assertion is required for the jwt-bearer grant")
			rec.Outcome = "invalid_request"
			recordRequest(r, rec)
			return
		}
		if assertion == invalidAssertion {
			writeJSONError(w, http.StatusBadRequest, "invalid_grant", "assertion could not be validated")
			rec.Outcome = "invalid_grant"
			recordRequest(r, rec)
			return
		}
	}

	switch clientID {
	case "broken-client":
		rec.Outcome = "server_error"
		recordRequest(r, rec)
		http.Error(w, "internal server error (simulated IdP outage)", http.StatusInternalServerError)
		return

	case "malformed-client":
		// 200 OK but the body is missing access_token — exercises the
		// policy's "successful fetch, malformed response" failure path.
		rec.Outcome = "malformed"
		recordRequest(r, rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token_type":"Bearer","expires_in":300}`))
		return

	case validClientID:
		if clientSecret != validClientSecret {
			writeJSONError(w, http.StatusBadRequest, "invalid_client", "client secret does not match")
			rec.Outcome = "invalid_client"
			recordRequest(r, rec)
			return
		}
		// fall through to issue a token

	default:
		writeJSONError(w, http.StatusBadRequest, "invalid_client", "unknown client_id")
		rec.Outcome = "invalid_client"
		recordRequest(r, rec)
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
				rec.Outcome = "forced_failure"
				recordRequest(r, rec)
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
	rec.Outcome = "issued"
	rec.Token = token
	recordRequest(r, rec)

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
	// RFC 8693 §2.2.1 requires issued_token_type on a token-exchange response;
	// it echoes the requested type when the caller asked for one, otherwise
	// this mock's default of a plain access token.
	if grantType == grantTypeTokenExchange {
		if rec.RequestedTokenType != "" {
			resp["issued_token_type"] = rec.RequestedTokenType
		} else {
			resp["issued_token_type"] = "urn:ietf:params:oauth:token-type:access_token"
		}
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

// recordRequest finalizes and appends rec to history. Callers fill in every
// field they know about (ClientID, AuthStyle, GrantType, Scope, Outcome,
// Token, and any grant-specific fields); this only adds the two fields that
// are always derived the same way, Time and Headers.
func recordRequest(r *http.Request, rec tokenRequestRecord) {
	rec.Time = time.Now().UTC()
	rec.Headers = extractCustomHeaders(r)
	mu.Lock()
	defer mu.Unlock()
	history = append(history, rec)
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
