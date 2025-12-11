package jwtauth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// TestJWTAuthPolicy_ValidToken tests successful JWT authentication
func TestJWTAuthPolicy_ValidToken(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)

	// Create JWKS server
	jwksServer := createJWKSServer(t, publicKey, "test-kid")
	defer jwksServer.Close()

	// Create test token
	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub":   "user123",
		"iss":   "https://issuer.example.com",
		"aud":   "api-audience",
		"scope": "read write",
		"name":  "John Doe",
	})

	// Create request context with Authorization header
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	// Create params
	params := map[string]interface{}{
		"headerName":             "Authorization",
		"authHeaderScheme":       "Bearer",
		"onFailureStatusCode":    401,
		"errorMessageFormat":     "json",
		"leeway":                 "30s",
		"allowedAlgorithms":      []interface{}{"RS256", "ES256"},
		"jwksCacheTtl":           "5m",
		"jwksFetchTimeout":       "5s",
		"jwksFetchRetryCount":    3,
		"jwksFetchRetryInterval": "2s",
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name":   "test-issuer",
				"issuer": "https://issuer.example.com",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{
						"uri": jwksServer.URL + "/jwks.json",
					},
				},
			},
		},
		"audiences":      []interface{}{"api-audience"},
		"requiredScopes": []interface{}{"read"},
		"claimMappings": map[string]interface{}{
			"sub":  "X-User-ID",
			"name": "X-User-Name",
		},
	}

	// Create policy instance
	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication
	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true, got %v", ctx.Metadata["auth.success"])
	}

	if ctx.Metadata["auth.method"] != "jwt" {
		t.Errorf("Expected auth.method to be 'jwt', got %v", ctx.Metadata["auth.method"])
	}

	// Verify it's an UpstreamRequestModifications action
	modifications, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}

	// Verify claim mappings were applied as headers
	if modifications.SetHeaders["X-User-ID"] != "user123" {
		t.Errorf("Expected X-User-ID header to be 'user123', got %s", modifications.SetHeaders["X-User-ID"])
	}

	if modifications.SetHeaders["X-User-Name"] != "John Doe" {
		t.Errorf("Expected X-User-Name header to be 'John Doe', got %s", modifications.SetHeaders["X-User-Name"])
	}
}

// TestJWTAuthPolicy_MissingToken tests authentication failure when Authorization header is missing
func TestJWTAuthPolicy_MissingToken(t *testing.T) {
	// Create request context without Authorization header
	ctx := createMockRequestContext(map[string][]string{})

	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"leeway":              "30s",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{
						"remote": map[string]interface{}{"uri": "http://localhost:8888/jwks.json"},
					},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Verify authentication failed
	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false, got %v", ctx.Metadata["auth.success"])
	}

	// Verify it's an ImmediateResponse
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestJWTAuthPolicy_InvalidTokenFormat tests with malformed token
func TestJWTAuthPolicy_InvalidTokenFormat(t *testing.T) {
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {"Bearer invalid.token"},
	})

	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": "http://localhost:8888/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false for invalid token format")
	}

	_, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse for invalid token, got %T", action)
	}
}

// TestJWTAuthPolicy_ExpiredToken tests with expired token
func TestJWTAuthPolicy_ExpiredToken(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	jwksServer := createJWKSServer(t, publicKey, "test-kid")
	defer jwksServer.Close()

	// Create expired token (expired 1 hour ago)
	expiredTime := time.Now().Add(-time.Hour)
	token := createTestTokenWithExpiry(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
	}, expiredTime)

	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	params := map[string]interface{}{
		"headerName":        "Authorization",
		"authHeaderScheme":  "Bearer",
		"leeway":            "30s",
		"allowedAlgorithms": []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": jwksServer.URL + "/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false for expired token")
	}

	_, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse for expired token")
	}
}

// TestJWTAuthPolicy_InvalidAudience tests audience validation
func TestJWTAuthPolicy_InvalidAudience(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	jwksServer := createJWKSServer(t, publicKey, "test-kid")
	defer jwksServer.Close()

	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"aud": "wrong-audience",
		"iss": "https://issuer.example.com",
	})

	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	params := map[string]interface{}{
		"headerName":        "Authorization",
		"authHeaderScheme":  "Bearer",
		"allowedAlgorithms": []interface{}{"RS256"},
		"audiences":         []interface{}{"api-audience"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": jwksServer.URL + "/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false for invalid audience")
	}

	_, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse for invalid audience")
	}
}

// TestJWTAuthPolicy_CustomClaims tests custom required claims validation
func TestJWTAuthPolicy_CustomClaims(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	jwksServer := createJWKSServer(t, publicKey, "test-kid")
	defer jwksServer.Close()

	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub":  "user123",
		"role": "admin",
		"iss":  "https://issuer.example.com",
	})

	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	params := map[string]interface{}{
		"headerName":        "Authorization",
		"authHeaderScheme":  "Bearer",
		"allowedAlgorithms": []interface{}{"RS256"},
		"requiredClaims": map[string]interface{}{
			"role": "admin",
		},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": jwksServer.URL + "/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true when required claims match")
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications for valid token with matching claims")
	}
}

// TestJWTAuthPolicy_InvalidCustomClaims tests failure with invalid required claims
func TestJWTAuthPolicy_InvalidCustomClaims(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	jwksServer := createJWKSServer(t, publicKey, "test-kid")
	defer jwksServer.Close()

	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub":  "user123",
		"role": "user",
		"iss":  "https://issuer.example.com",
	})

	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	params := map[string]interface{}{
		"headerName":        "Authorization",
		"authHeaderScheme":  "Bearer",
		"allowedAlgorithms": []interface{}{"RS256"},
		"requiredClaims": map[string]interface{}{
			"role": "admin",
		},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": jwksServer.URL + "/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false for mismatched required claims")
	}

	_, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse for invalid claims")
	}
}

// TestJWTAuthPolicy_InvalidSignature tests rejection of token signed with wrong key
func TestJWTAuthPolicy_InvalidSignature(t *testing.T) {
	// Generate two different key pairs
	_, validPublicKey := generateTestKeys(t)
	invalidPrivateKey, _ := generateTestKeys(t)

	// Create JWKS server with the VALID public key
	jwksServer := createJWKSServer(t, validPublicKey, "test-kid")
	defer jwksServer.Close()

	// Create token signed with the INVALID private key
	token := createTestToken(t, invalidPrivateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
	})

	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	params := map[string]interface{}{
		"headerName":        "Authorization",
		"authHeaderScheme":  "Bearer",
		"allowedAlgorithms": []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": jwksServer.URL + "/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	// Should fail because signature doesn't match the JWKS public key
	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false for token signed with invalid key")
	}

	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse for invalid signature, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestJWTAuthPolicy_CustomHeaderPrefix tests custom Authorization header prefix
func TestJWTAuthPolicy_CustomHeaderPrefix(t *testing.T) {
	privateKey, publicKey := generateTestKeys(t)
	jwksServer := createJWKSServer(t, publicKey, "test-kid")
	defer jwksServer.Close()

	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
	})

	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("JWT %s", token)},
	})

	params := map[string]interface{}{
		"headerName":        "Authorization",
		"authHeaderScheme":  "Bearer", // System default
		"authHeaderPrefix":  "JWT",    // User override
		"allowedAlgorithms": []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": jwksServer.URL + "/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true with custom prefix override")
	}

	_, ok := action.(policy.UpstreamRequestModifications)
	if !ok {
		t.Fatalf("Expected UpstreamRequestModifications with custom header prefix")
	}
}

// TestJWTAuthPolicy_ErrorResponseFormat tests different error response formats
func TestJWTAuthPolicy_ErrorResponseFormatJSON(t *testing.T) {
	ctx := createMockRequestContext(map[string][]string{})

	params := map[string]interface{}{
		"errorMessageFormat":  "json",
		"onFailureStatusCode": 401,
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": "http://localhost:8888/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	response := action.(policy.ImmediateResponse)
	if response.Headers["content-type"] != "application/json" {
		t.Errorf("Expected content-type to be application/json")
	}

	var errBody map[string]interface{}
	if err := json.Unmarshal(response.Body, &errBody); err != nil {
		t.Errorf("Expected JSON error response, got: %s", string(response.Body))
	}
}

func TestJWTAuthPolicy_ErrorResponseFormatPlain(t *testing.T) {
	ctx := createMockRequestContext(map[string][]string{})

	params := map[string]interface{}{
		"errorMessageFormat":  "plain",
		"onFailureStatusCode": 401,
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name": "test-issuer",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{"uri": "http://localhost:8888/jwks.json"},
				},
			},
		},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	action := p.OnRequest(ctx, params)

	response := action.(policy.ImmediateResponse)
	if response.Headers["content-type"] != "text/plain" {
		t.Errorf("Expected content-type to be text/plain")
	}
}

// TestJWTAuthPolicy_RemoteWithSelfSignedCert tests JWT validation with remote JWKS and self-signed certificate configuration
func TestJWTAuthPolicy_RemoteWithSelfSignedCert(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)

	// Create an unstarted HTTPS server (without TLS yet)
	unstarted := createHTTPSJWKSServerUnstarted(t, publicKey, "test-kid")

	// Create a self-signed certificate for localhost (the server will be on localhost)
	certKeyPath, _, caPath := createSelfSignedCertForHost(t, "https://localhost:443")
	defer func() {
		parts := strings.Split(certKeyPath, ":")
		if len(parts) == 2 {
			os.Remove(parts[0])
			os.Remove(parts[1])
		}
		os.Remove(caPath)
	}()

	// Load the certificate and configure TLS on the server
	parts := strings.Split(certKeyPath, ":")
	if len(parts) != 2 {
		t.Fatalf("Expected cert:key format, got %s", certKeyPath)
	}

	tlsCert, err := tls.LoadX509KeyPair(parts[0], parts[1])
	if err != nil {
		t.Fatalf("Failed to load TLS certificate: %v", err)
	}

	unstarted.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	// Now start the HTTPS server
	unstarted.StartTLS()
	jwksServer := unstarted
	defer jwksServer.Close()

	// Create test token
	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "api-audience",
	})

	// Create request context
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	// Create params with certificate path to validate self-signed JWKS endpoint
	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"leeway":              "30s",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name":   "test-issuer",
				"issuer": "https://issuer.example.com",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{
						"uri":             jwksServer.URL + "/jwks.json",
						"certificatePath": caPath, // CA certificate for validating self-signed JWKS endpoint
					},
				},
			},
		},
		"audiences": []interface{}{"api-audience"},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication - token validated against self-signed JWKS
	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true with self-signed certificate, got %v", ctx.Metadata["auth.success"])
	}

	// Verify it's an UpstreamRequestModifications action
	if _, ok := action.(policy.UpstreamRequestModifications); !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestJWTAuthPolicy_SkipTlsVerify_Success tests JWT validation when skipTlsVerify is true with self-signed JWKS endpoint
func TestJWTAuthPolicy_SkipTlsVerify_Success(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)

	// Create an unstarted HTTPS server
	unstarted := createHTTPSJWKSServerUnstarted(t, publicKey, "test-kid")

	// Create a self-signed certificate for localhost
	certKeyPath, _, caPath := createSelfSignedCertForHost(t, "https://localhost:443")
	defer func() {
		parts := strings.Split(certKeyPath, ":")
		if len(parts) == 2 {
			os.Remove(parts[0])
			os.Remove(parts[1])
		}
		os.Remove(caPath)
	}()

	// Load the certificate and configure TLS on the server
	parts := strings.Split(certKeyPath, ":")
	if len(parts) != 2 {
		t.Fatalf("Expected cert:key format, got %s", certKeyPath)
	}

	tlsCert, err := tls.LoadX509KeyPair(parts[0], parts[1])
	if err != nil {
		t.Fatalf("Failed to load TLS certificate: %v", err)
	}

	unstarted.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	// Start the HTTPS server
	unstarted.StartTLS()
	jwksServer := unstarted
	defer jwksServer.Close()

	// Create test token
	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "api-audience",
	})

	// Create request context
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	// Create params with skipTlsVerify=true (no certificatePath needed)
	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"leeway":              "30s",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name":   "test-issuer",
				"issuer": "https://issuer.example.com",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{
						"uri":           jwksServer.URL + "/jwks.json",
						"skipTlsVerify": true, // Skip TLS verification
					},
				},
			},
		},
		"audiences": []interface{}{"api-audience"},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication - TLS verification was skipped
	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true with skipTlsVerify=true, got %v", ctx.Metadata["auth.success"])
	}

	// Verify it's an UpstreamRequestModifications action
	if _, ok := action.(policy.UpstreamRequestModifications); !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestJWTAuthPolicy_SkipTlsVerify_False_Fails tests JWT validation fails when skipTlsVerify is false with self-signed JWKS endpoint
func TestJWTAuthPolicy_SkipTlsVerify_False_Fails(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)

	// Create an unstarted HTTPS server
	unstarted := createHTTPSJWKSServerUnstarted(t, publicKey, "test-kid")

	// Create a self-signed certificate for localhost
	certKeyPath, _, caPath := createSelfSignedCertForHost(t, "https://localhost:443")
	defer func() {
		parts := strings.Split(certKeyPath, ":")
		if len(parts) == 2 {
			os.Remove(parts[0])
			os.Remove(parts[1])
		}
		os.Remove(caPath)
	}()

	// Load the certificate and configure TLS on the server
	parts := strings.Split(certKeyPath, ":")
	if len(parts) != 2 {
		t.Fatalf("Expected cert:key format, got %s", certKeyPath)
	}

	tlsCert, err := tls.LoadX509KeyPair(parts[0], parts[1])
	if err != nil {
		t.Fatalf("Failed to load TLS certificate: %v", err)
	}

	unstarted.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	// Start the HTTPS server
	unstarted.StartTLS()
	jwksServer := unstarted
	defer jwksServer.Close()

	// Create test token
	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "api-audience",
	})

	// Create request context
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	// Create params WITHOUT skipTlsVerify or certificatePath - should fail TLS verification
	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"leeway":              "30s",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name":   "test-issuer",
				"issuer": "https://issuer.example.com",
				"jwks": map[string]interface{}{
					"remote": map[string]interface{}{
						"uri":           jwksServer.URL + "/jwks.json",
						"skipTlsVerify": false, // Explicitly set to false (default)
					},
				},
			},
		},
		"audiences":           []interface{}{"api-audience"},
		"jwksFetchRetryCount": 0,
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify authentication failed - TLS verification should fail for self-signed cert
	if ctx.Metadata["auth.success"] != false {
		t.Errorf("Expected auth.success to be false with skipTlsVerify=false and self-signed cert, got %v", ctx.Metadata["auth.success"])
	}

	// Verify it's an ImmediateResponse (error)
	response, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("Expected ImmediateResponse for TLS verification failure, got %T", action)
	}

	if response.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", response.StatusCode)
	}
}

// TestJWTAuthPolicy_LocalInlineCertificate tests JWT validation with inline certificate
func TestJWTAuthPolicy_LocalInlineCertificate(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)

	// Create test token
	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "api-audience",
	})

	// Convert public key to PEM format for inline use
	pubKeyPEM := publicKeyToPEM(t, publicKey)

	// Create request context
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	// Create params with inline certificate
	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"leeway":              "30s",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name":   "test-issuer",
				"issuer": "https://issuer.example.com",
				"jwks": map[string]interface{}{
					"local": map[string]interface{}{
						"inline": pubKeyPEM,
					},
				},
			},
		},
		"audiences": []interface{}{"api-audience"},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication
	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true with inline certificate, got %v", ctx.Metadata["auth.success"])
	}

	// Verify it's an UpstreamRequestModifications action
	if _, ok := action.(policy.UpstreamRequestModifications); !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// TestJWTAuthPolicy_LocalCertificateFile tests JWT validation with certificate file path
func TestJWTAuthPolicy_LocalCertificateFile(t *testing.T) {
	// Generate test keys
	privateKey, publicKey := generateTestKeys(t)

	// Create test token
	token := createTestToken(t, privateKey, map[string]interface{}{
		"sub": "user123",
		"iss": "https://issuer.example.com",
		"aud": "api-audience",
	})

	// Save public key to temporary file
	certPath := writeCertificateToFile(t, publicKey)
	defer os.Remove(certPath)

	// Create request context
	ctx := createMockRequestContext(map[string][]string{
		"authorization": {fmt.Sprintf("Bearer %s", token)},
	})

	// Create params with certificate file path
	params := map[string]interface{}{
		"headerName":          "Authorization",
		"authHeaderScheme":    "Bearer",
		"onFailureStatusCode": 401,
		"errorMessageFormat":  "json",
		"leeway":              "30s",
		"allowedAlgorithms":   []interface{}{"RS256"},
		"keyManagers": []interface{}{
			map[string]interface{}{
				"name":   "test-issuer",
				"issuer": "https://issuer.example.com",
				"jwks": map[string]interface{}{
					"local": map[string]interface{}{
						"certificatePath": certPath,
					},
				},
			},
		},
		"audiences": []interface{}{"api-audience"},
	}

	p, err := NewPolicy(policy.PolicyMetadata{}, map[string]interface{}{}, params)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Execute policy
	action := p.OnRequest(ctx, params)

	// Verify successful authentication
	if ctx.Metadata["auth.success"] != true {
		t.Errorf("Expected auth.success to be true with certificate file, got %v", ctx.Metadata["auth.success"])
	}

	// Verify it's an UpstreamRequestModifications action
	if _, ok := action.(policy.UpstreamRequestModifications); !ok {
		t.Fatalf("Expected UpstreamRequestModifications, got %T", action)
	}
}

// Helper functions

func generateTestKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]interface{}) string {
	return createTestTokenWithExpiry(t, privateKey, claims, time.Now().Add(time.Hour))
}

func createTestTokenWithExpiry(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]interface{}, expiryTime time.Time) string {
	// Set default claims
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = expiryTime.Unix()
	}
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = time.Now().Unix()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims(claims))
	token.Header["kid"] = "test-kid"

	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	return tokenString
}

func createJWKSServer(t *testing.T, publicKey *rsa.PublicKey, kid string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks.json" {
			// Extract N and E from public key
			nBytes := publicKey.N.Bytes()
			nB64 := base64.RawURLEncoding.EncodeToString(nBytes)

			// Encode E as big-endian bytes
			eBytes := make([]byte, 4)
			eBytes[0] = byte((publicKey.E >> 24) & 0xFF)
			eBytes[1] = byte((publicKey.E >> 16) & 0xFF)
			eBytes[2] = byte((publicKey.E >> 8) & 0xFF)
			eBytes[3] = byte(publicKey.E & 0xFF)
			// Remove leading zero bytes
			for len(eBytes) > 1 && eBytes[0] == 0 {
				eBytes = eBytes[1:]
			}
			eB64 := base64.RawURLEncoding.EncodeToString(eBytes)

			jwks := map[string]interface{}{
				"keys": []map[string]interface{}{
					{
						"kty": "RSA",
						"kid": kid,
						"use": "sig",
						"alg": "RS256",
						"n":   nB64,
						"e":   eB64,
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(jwks); err != nil {
				t.Logf("Failed to encode JWKS: %v", err)
			}
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	return server
}

func createMockRequestContext(headers map[string][]string) *policy.RequestContext {
	return &policy.RequestContext{
		SharedContext: &policy.SharedContext{
			RequestID: "test-request-id",
			Metadata:  make(map[string]interface{}),
		},
		Headers: policy.NewHeaders(headers),
		Body:    nil,
		Path:    "/api/test",
		Method:  "GET",
	}
}

// createTestHeaders is no longer needed with NewHeaders

// createHTTPSJWKSServerUnstarted creates an unstarted HTTPS server for initial hostname detection
func createHTTPSJWKSServerUnstarted(t *testing.T, publicKey *rsa.PublicKey, kid string) *httptest.Server {
	return httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks.json" {
			nBytes := publicKey.N.Bytes()
			nB64 := base64.RawURLEncoding.EncodeToString(nBytes)

			eBytes := make([]byte, 4)
			eBytes[0] = byte((publicKey.E >> 24) & 0xFF)
			eBytes[1] = byte((publicKey.E >> 16) & 0xFF)
			eBytes[2] = byte((publicKey.E >> 8) & 0xFF)
			eBytes[3] = byte(publicKey.E & 0xFF)
			for len(eBytes) > 1 && eBytes[0] == 0 {
				eBytes = eBytes[1:]
			}
			eB64 := base64.RawURLEncoding.EncodeToString(eBytes)

			jwks := map[string]interface{}{
				"keys": []map[string]interface{}{
					{
						"kty": "RSA",
						"kid": kid,
						"use": "sig",
						"alg": "RS256",
						"n":   nB64,
						"e":   eB64,
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(jwks); err != nil {
				t.Logf("Failed to encode JWKS: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// createSelfSignedCertForHost creates a self-signed certificate for a specific host
func createSelfSignedCertForHost(t *testing.T, hostURL string) (string, []byte, string) {
	// Parse the host from URL
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template for the specific hostname
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
			CommonName:   hostname,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{hostname, "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	// Encode private key to PEM
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("Failed to marshal private key: %v", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	})

	// Write certificate to temporary file
	certFile, err := os.CreateTemp("", "test-cert-*.pem")
	if err != nil {
		t.Fatalf("Failed to create cert temporary file: %v", err)
	}
	defer certFile.Close()

	if _, err := certFile.Write(certPEM); err != nil {
		t.Fatalf("Failed to write certificate to file: %v", err)
	}

	// Write private key to temporary file
	keyFile, err := os.CreateTemp("", "test-key-*.pem")
	if err != nil {
		t.Fatalf("Failed to create key temporary file: %v", err)
	}
	defer keyFile.Close()

	if _, err := keyFile.Write(keyPEM); err != nil {
		t.Fatalf("Failed to write key to file: %v", err)
	}

	// Write CA cert to separate temporary file (for client validation)
	caFile, err := os.CreateTemp("", "test-ca-*.pem")
	if err != nil {
		t.Fatalf("Failed to create CA temp file: %v", err)
	}
	defer caFile.Close()

	if _, err := caFile.Write(certPEM); err != nil {
		t.Fatalf("Failed to write CA cert to file: %v", err)
	}

	return certFile.Name() + ":" + keyFile.Name(), certPEM, caFile.Name()
}

// // createSelfSignedCert creates a self-signed certificate for testing HTTPS endpoints
// func createSelfSignedCert(t *testing.T) (string, []byte, string) {
// 	// Generate RSA key
// 	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
// 	if err != nil {
// 		t.Fatalf("Failed to generate private key: %v", err)
// 	}

// 	// Create certificate template
// 	template := &x509.Certificate{
// 		SerialNumber: big.NewInt(1),
// 		Subject: pkix.Name{
// 			Organization: []string{"Test"},
// 			CommonName:   "localhost",
// 		},
// 		NotBefore:             time.Now(),
// 		NotAfter:              time.Now().Add(24 * time.Hour),
// 		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
// 		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
// 		BasicConstraintsValid: true,
// 		IsCA:                  true,
// 		DNSNames:              []string{"localhost", "127.0.0.1"},
// 	}

// 	// Create self-signed certificate
// 	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
// 	if err != nil {
// 		t.Fatalf("Failed to create certificate: %v", err)
// 	}

// 	// Encode certificate to PEM
// 	certPEM := pem.EncodeToMemory(&pem.Block{
// 		Type:  "CERTIFICATE",
// 		Bytes: certBytes,
// 	})

// 	// Encode private key to PEM
// 	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
// 	if err != nil {
// 		t.Fatalf("Failed to marshal private key: %v", err)
// 	}

// 	keyPEM := pem.EncodeToMemory(&pem.Block{
// 		Type:  "PRIVATE KEY",
// 		Bytes: privKeyBytes,
// 	})

// 	// Write certificate to temporary file
// 	certFile, err := os.CreateTemp("", "test-cert-*.pem")
// 	if err != nil {
// 		t.Fatalf("Failed to create cert temporary file: %v", err)
// 	}
// 	defer certFile.Close()

// 	if _, err := certFile.Write(certPEM); err != nil {
// 		t.Fatalf("Failed to write certificate to file: %v", err)
// 	}

// 	// Write private key to temporary file
// 	keyFile, err := os.CreateTemp("", "test-key-*.pem")
// 	if err != nil {
// 		t.Fatalf("Failed to create key temporary file: %v", err)
// 	}
// 	defer keyFile.Close()

// 	if _, err := keyFile.Write(keyPEM); err != nil {
// 		t.Fatalf("Failed to write key to file: %v", err)
// 	}

// 	// Write CA cert to separate temporary file (for client validation)
// 	caFile, err := os.CreateTemp("", "test-ca-*.pem")
// 	if err != nil {
// 		t.Fatalf("Failed to create CA temp file: %v", err)
// 	}
// 	defer caFile.Close()

// 	if _, err := caFile.Write(certPEM); err != nil {
// 		t.Fatalf("Failed to write CA cert to file: %v", err)
// 	}

// 	return certFile.Name() + ":" + keyFile.Name(), certPEM, caFile.Name()
// }

// createHTTPSJWKSServer creates an HTTPS JWKS endpoint with self-signed certificate
func createHTTPSJWKSServer(t *testing.T, publicKey *rsa.PublicKey, kid string, certKeyPath string) *httptest.Server {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks.json" {
			// Extract N and E from public key
			nBytes := publicKey.N.Bytes()
			nB64 := base64.RawURLEncoding.EncodeToString(nBytes)

			// Encode E as big-endian bytes
			eBytes := make([]byte, 4)
			eBytes[0] = byte((publicKey.E >> 24) & 0xFF)
			eBytes[1] = byte((publicKey.E >> 16) & 0xFF)
			eBytes[2] = byte((publicKey.E >> 8) & 0xFF)
			eBytes[3] = byte(publicKey.E & 0xFF)
			// Remove leading zero bytes
			for len(eBytes) > 1 && eBytes[0] == 0 {
				eBytes = eBytes[1:]
			}
			eB64 := base64.RawURLEncoding.EncodeToString(eBytes)

			jwks := map[string]interface{}{
				"keys": []map[string]interface{}{
					{
						"kty": "RSA",
						"kid": kid,
						"use": "sig",
						"alg": "RS256",
						"n":   nB64,
						"e":   eB64,
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(jwks); err != nil {
				t.Logf("Failed to encode JWKS: %v", err)
			}
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	// Parse cert:key format
	parts := strings.Split(certKeyPath, ":")
	if len(parts) != 2 {
		t.Fatalf("Expected cert:key format, got %s", certKeyPath)
	}

	// Load TLS certificate
	tlsCert, err := tls.LoadX509KeyPair(parts[0], parts[1])
	if err != nil {
		t.Fatalf("Failed to load TLS certificate: %v", err)
	}

	// Configure TLS for the server
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	server.StartTLS()
	return server
}

// publicKeyToPEM converts an RSA public key to PEM format
func publicKeyToPEM(t *testing.T, publicKey *rsa.PublicKey) string {
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return string(pubPEM)
}

// writeCertificateToFile writes an RSA public key to a temporary file in PEM format
func writeCertificateToFile(t *testing.T, publicKey *rsa.PublicKey) string {
	pubKeyPEM := publicKeyToPEM(t, publicKey)

	tmpFile, err := os.CreateTemp("", "test-pubkey-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(pubKeyPEM); err != nil {
		t.Fatalf("Failed to write public key to file: %v", err)
	}

	return tmpFile.Name()
}
