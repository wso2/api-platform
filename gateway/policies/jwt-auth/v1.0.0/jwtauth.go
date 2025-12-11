package jwtauth

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	// Metadata keys for context storage
	MetadataKeyAuthSuccess = "auth.success"
	MetadataKeyAuthMethod  = "auth.method"
	MetadataKeyTokenClaims = "auth.claims"
	MetadataKeyIssuer      = "auth.issuer"
	MetadataKeySubject     = "auth.subject"
)

// JwtAuthPolicy implements JWT Authentication with JWKS support
type JwtAuthPolicy struct {
	cacheMutex sync.RWMutex
	cacheStore map[string]*CachedJWKS
	cacheTTLs  map[string]time.Time
	httpClient *http.Client
}

// CachedJWKS stores cached JWKS data
type CachedJWKS struct {
	Keys map[string]*rsa.PublicKey
}

// KeyManager represents a key manager with either remote JWKS or local certificate
type KeyManager struct {
	Name   string      // Unique name for this key manager
	Issuer string      // Optional issuer value
	JWKS   *JWKSConfig // JWKS configuration (remote and/or local)
}

// JWKSConfig holds both remote and local key configurations
type JWKSConfig struct {
	Remote *RemoteJWKS // Remote JWKS endpoint configuration
	Local  *LocalCert  // Local certificate configuration
}

// RemoteJWKS holds remote JWKS endpoint configuration
type RemoteJWKS struct {
	URI             string      // JWKS endpoint URL
	CertificatePath string      // Optional CA certificate path for self-signed endpoints
	SkipTlsVerify   bool        // Skip TLS certificate verification (use with caution)
	tlsConfig       *tls.Config // Cached TLS config for this endpoint
}

// LocalCert holds local certificate configuration
type LocalCert struct {
	Inline          string         // Inline PEM-encoded certificate
	CertificatePath string         // Path to certificate file
	PublicKey       *rsa.PublicKey // Parsed public key
}

// JWKSKeySet represents the JWKS response from server
type JWKSKeySet struct {
	Keys []JWKSKey `json:"keys"`
}

// JWKSKey represents a single key in JWKS
type JWKSKey struct {
	Kty string `json:"kty"` // Key type (RSA, EC, etc.)
	Use string `json:"use"` // Public key use
	Kid string `json:"kid"` // Key ID
	N   string `json:"n"`   // RSA modulus
	E   string `json:"e"`   // RSA exponent
	Alg string `json:"alg"` // Algorithm
}

var ins = &JwtAuthPolicy{
	cacheStore: make(map[string]*CachedJWKS),
	cacheTTLs:  make(map[string]time.Time),
	httpClient: &http.Client{
		Timeout: 5 * time.Second,
	},
}

// NewPolicy creates a new BasicAuthPolicy instance
func NewPolicy(
	metadata policy.PolicyMetadata,
	initParams map[string]interface{},
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *JwtAuthPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers for token
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip,    // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// OnRequest performs JWT validation
func (p *JwtAuthPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Get system configuration
	headerName := getStringParam(params, "headerName", "Authorization")
	authHeaderScheme := getStringParam(params, "authHeaderScheme", "Bearer")
	onFailureStatusCode := getIntParam(params, "onFailureStatusCode", 401)
	errorMessageFormat := getStringParam(params, "errorMessageFormat", "json")
	leewayStr := getStringParam(params, "leeway", "30s")
	allowedAlgorithms := getStringArrayParam(params, "allowedAlgorithms", []string{"RS256", "ES256"})
	jwksCacheTtlStr := getStringParam(params, "jwksCacheTtl", "5m")
	jwksFetchTimeoutStr := getStringParam(params, "jwksFetchTimeout", "5s")
	jwksFetchRetryCount := getIntParam(params, "jwksFetchRetryCount", 3)
	jwksFetchRetryIntervalStr := getStringParam(params, "jwksFetchRetryInterval", "2s")

	// Parse durations
	leeway, err := time.ParseDuration(leewayStr)
	if err != nil {
		leeway = 30 * time.Second
	}

	jwksCacheTtl, err := time.ParseDuration(jwksCacheTtlStr)
	if err != nil {
		jwksCacheTtl = 5 * time.Minute
	}

	jwksFetchTimeout, err := time.ParseDuration(jwksFetchTimeoutStr)
	if err != nil {
		jwksFetchTimeout = 5 * time.Second
	}

	jwksFetchRetryInterval, err := time.ParseDuration(jwksFetchRetryIntervalStr)
	if err != nil {
		jwksFetchRetryInterval = 2 * time.Second
	}

	// Get key managers configuration
	keyManagersRaw, ok := params["keyManagers"]
	if !ok {
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "key managers not configured")
	}

	// Parse keyManagers
	keyManagers := make(map[string]*KeyManager)
	keyManagersList, ok := keyManagersRaw.([]interface{})
	if ok {
		for _, km := range keyManagersList {
			if kmMap, ok := km.(map[string]interface{}); ok {
				name := getString(kmMap["name"])
				issuer := getString(kmMap["issuer"])

				if name == "" {
					continue
				}

				keyManager := &KeyManager{
					Name:   name,
					Issuer: issuer,
				}

				// Try to parse JWKS configuration
				if jwksRaw, ok := kmMap["jwks"].(map[string]interface{}); ok {
					jwksConfig := &JWKSConfig{}

					// Parse remote JWKS configuration
					if remoteRaw, ok := jwksRaw["remote"].(map[string]interface{}); ok {
						uri := getString(remoteRaw["uri"])
						certPath := getString(remoteRaw["certificatePath"])
						skipTlsVerify := getBool(remoteRaw["skipTlsVerify"])

						if uri != "" {
							remoteJWKS := &RemoteJWKS{
								URI:             uri,
								CertificatePath: certPath,
								SkipTlsVerify:   skipTlsVerify,
							}
							// Load and cache TLS config if certificate path is provided
							if certPath != "" {
								tlsConfig, err := loadTLSConfig(certPath)
								if err != nil {
									continue // Skip this key manager if cert loading fails
								}
								remoteJWKS.tlsConfig = tlsConfig
							} else if skipTlsVerify {
								// Configure TLS to skip verification
								remoteJWKS.tlsConfig = &tls.Config{
									InsecureSkipVerify: true,
									MinVersion:         tls.VersionTLS12,
								}
							}
							jwksConfig.Remote = remoteJWKS
						}
					}

					// Parse local certificate configuration
					if localRaw, ok := jwksRaw["local"].(map[string]interface{}); ok {
						inline := getString(localRaw["inline"])
						certPath := getString(localRaw["certificatePath"])

						// Either inline or certificate path must be provided
						if inline != "" || certPath != "" {
							localCert := &LocalCert{
								Inline:          inline,
								CertificatePath: certPath,
							}

							// Load certificate/key
							var publicKey *rsa.PublicKey
							var err error

							if inline != "" {
								publicKey, err = parsePublicKeyFromString(inline)
							} else if certPath != "" {
								publicKey, err = loadPublicKeyFromCertificate(certPath)
							}

							if err != nil {
								continue // Skip this key manager if cert loading fails
							}
							localCert.PublicKey = publicKey
							jwksConfig.Local = localCert
						}
					}

					// Accept key manager if either remote or local is configured
					if jwksConfig.Remote != nil || jwksConfig.Local != nil {
						keyManager.JWKS = jwksConfig
						keyManagers[name] = keyManager
					}
				}
			}
		}
	}

	if len(keyManagers) == 0 {
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "no key managers configured")
	}

	// Get user configuration
	userIssuers := getStringArrayParam(params, "issuers", []string{})
	userAudiences := getStringArrayParam(params, "audiences", []string{})
	userRequiredScopes := getStringArrayParam(params, "requiredScopes", []string{})
	userRequiredClaims := getStringMapParam(params, "requiredClaims", map[string]string{})
	userClaimMappings := getStringMapParam(params, "claimMappings", map[string]string{})
	userAuthHeaderPrefix := getStringParam(params, "authHeaderPrefix", "")

	// Use user override if provided
	if userAuthHeaderPrefix != "" {
		authHeaderScheme = userAuthHeaderPrefix
	}

	// Extract token from header
	authHeaders := ctx.Headers.Get(strings.ToLower(headerName))
	if len(authHeaders) == 0 {
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "missing authorization header")
	}

	authHeader := authHeaders[0]
	token := extractToken(authHeader, authHeaderScheme)
	if token == "" {
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "invalid authorization header format")
	}

	// Parse token to get header info
	unverifiedToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "invalid token format")
	}

	// Validate token and signature
	claims, err := p.validateTokenWithSignature(token, unverifiedToken, keyManagers, userIssuers,
		allowedAlgorithms, leeway, jwksCacheTtl, jwksFetchTimeout, jwksFetchRetryCount, jwksFetchRetryInterval)
	if err != nil {
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, fmt.Sprintf("token validation failed: %v", err))
	}

	// Validate issuer if specified by user
	if len(userIssuers) > 0 {
		issuer := getString(claims["iss"])
		found := false
		for _, userIssuer := range userIssuers {
			if issuer == userIssuer {
				found = true
				break
			}
		}
		if !found {
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "issuer not in allowed list")
		}
	}

	// Validate audience if specified
	if len(userAudiences) > 0 {
		aud := parseAudience(claims["aud"])
		found := false
		for _, userAud := range userAudiences {
			for _, tokenAud := range aud {
				if tokenAud == userAud {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, "no valid audience found in token")
		}
	}

	// Validate required scopes if specified
	if len(userRequiredScopes) > 0 {
		scopes := parseScopes(claims["scope"], claims["scp"])
		for _, requiredScope := range userRequiredScopes {
			found := false
			for _, tokenScope := range scopes {
				if tokenScope == requiredScope {
					found = true
					break
				}
			}
			if !found {
				return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, fmt.Sprintf("required scope '%s' not found", requiredScope))
			}
		}
	}

	// Validate required claims if specified
	for claimName, expectedValue := range userRequiredClaims {
		claimValue := getString(claims[claimName])
		if claimValue != expectedValue {
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, fmt.Sprintf("claim '%s' validation failed", claimName))
		}
	}

	// Authentication successful - apply claim mappings and set metadata
	return p.handleAuthSuccess(ctx, claims, userClaimMappings)
}

// validateTokenWithSignature validates JWT signature using JWKS
func (p *JwtAuthPolicy) validateTokenWithSignature(tokenString string, unverifiedToken *jwt.Token,
	keyManagers map[string]*KeyManager, userIssuers []string, allowedAlgorithms []string,
	leeway time.Duration, cacheTTL time.Duration, fetchTimeout time.Duration, retryCount int, retryInterval time.Duration) (jwt.MapClaims, error) {

	unverifiedClaims, ok := unverifiedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims format")
	}

	// Check allowed algorithms
	alg, ok := unverifiedToken.Header["alg"].(string)
	if !ok {
		return nil, fmt.Errorf("missing algorithm in token")
	}

	algorithmAllowed := false
	for _, allowed := range allowedAlgorithms {
		if alg == allowed {
			algorithmAllowed = true
			break
		}
	}
	if !algorithmAllowed {
		return nil, fmt.Errorf("algorithm '%s' not in allowed list", alg)
	}

	// Validate exp and nbf with leeway
	now := time.Now()
	if exp, ok := unverifiedClaims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		if now.After(expTime.Add(leeway)) {
			return nil, fmt.Errorf("token expired")
		}
	}

	if nbf, ok := unverifiedClaims["nbf"].(float64); ok {
		nbfTime := time.Unix(int64(nbf), 0)
		if now.Before(nbfTime.Add(-leeway)) {
			return nil, fmt.Errorf("token not yet valid")
		}
	}

	// Get issuer from token
	tokenIssuer := getString(unverifiedClaims["iss"])

	// Determine which key managers to use
	var applicableKeyManagers []*KeyManager
	if len(userIssuers) > 0 {
		// Use only specified issuers
		for _, userIssuer := range userIssuers {
			if km, ok := keyManagers[userIssuer]; ok {
				applicableKeyManagers = append(applicableKeyManagers, km)
			}
		}
	} else if tokenIssuer != "" {
		// Try to match token issuer to key managers
		for _, km := range keyManagers {
			if km.Issuer == tokenIssuer {
				applicableKeyManagers = append(applicableKeyManagers, km)
				break
			}
		}
		// If no issuer match, try all key managers
		if len(applicableKeyManagers) == 0 {
			for _, km := range keyManagers {
				applicableKeyManagers = append(applicableKeyManagers, km)
			}
		}
	} else {
		// No issuer specified, try all key managers
		for _, km := range keyManagers {
			applicableKeyManagers = append(applicableKeyManagers, km)
		}
	}

	// Get kid from token header
	kid, ok := unverifiedToken.Header["kid"].(string)
	if !ok {
		// Kid is optional for certificate-based validation
		kid = ""
	}

	// Try to verify signature with applicable key managers
	var lastErr error
	for _, km := range applicableKeyManagers {
		if km.JWKS == nil {
			continue
		}

		// Try local certificate validation first if available
		if km.JWKS.Local != nil && km.JWKS.Local.PublicKey != nil {
			verifiedToken, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
				return km.JWKS.Local.PublicKey, nil
			})

			if err == nil {
				// Signature verified successfully with local certificate
				if claims, ok := verifiedToken.Claims.(jwt.MapClaims); ok {
					return claims, nil
				}
			}
			lastErr = fmt.Errorf("signature verification failed with local certificate: %w", err)
			continue
		}

		// Fall back to remote JWKS-based validation
		if km.JWKS.Remote != nil {
			// Get JWKS with retry logic
			jwks, err := p.fetchJWKSWithRetry(km.JWKS.Remote, cacheTTL, fetchTimeout, retryCount, retryInterval)
			if err != nil {
				lastErr = fmt.Errorf("failed to fetch JWKS from %s: %w", km.JWKS.Remote.URI, err)
				continue
			}

			// If kid is present, find the key with matching kid
			if kid != "" {
				publicKey, ok := jwks.Keys[kid]
				if !ok {
					lastErr = fmt.Errorf("key id '%s' not found in JWKS from %s", kid, km.JWKS.Remote.URI)
					continue
				}

				// Verify signature
				verifiedToken, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
					return publicKey, nil
				})

				if err != nil {
					lastErr = fmt.Errorf("signature verification failed: %w", err)
					continue
				}

				// Signature verified successfully
				if claims, ok := verifiedToken.Claims.(jwt.MapClaims); ok {
					return claims, nil
				}
			} else {
				// No kid, try all keys in JWKS
				for _, publicKey := range jwks.Keys {
					verifiedToken, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
						return publicKey, nil
					})

					if err == nil {
						// Signature verified successfully
						if claims, ok := verifiedToken.Claims.(jwt.MapClaims); ok {
							return claims, nil
						}
					}
				}
				lastErr = fmt.Errorf("token signature verification failed with all keys from %s", km.JWKS.Remote.URI)
			}
		}
	}

	// If no key manager succeeded
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("unable to verify token signature with available key managers")
}

// fetchJWKSWithRetry fetches JWKS with caching and retry logic
func (p *JwtAuthPolicy) fetchJWKSWithRetry(remote *RemoteJWKS, cacheTTL time.Duration, fetchTimeout time.Duration, retryCount int, retryInterval time.Duration) (*CachedJWKS, error) {
	// Check cache first
	p.cacheMutex.RLock()
	if cached, ok := p.cacheStore[remote.URI]; ok {
		if ttl, ok := p.cacheTTLs[remote.URI]; ok && time.Now().Before(ttl) {
			p.cacheMutex.RUnlock()
			return cached, nil
		}
	}
	p.cacheMutex.RUnlock()

	// Not in cache or expired, fetch from server
	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		jwks, err := p.fetchJWKS(remote, fetchTimeout)
		if err == nil {
			// Cache the result
			p.cacheMutex.Lock()
			p.cacheStore[remote.URI] = jwks
			p.cacheTTLs[remote.URI] = time.Now().Add(cacheTTL)
			p.cacheMutex.Unlock()
			return jwks, nil
		}

		lastErr = err
		if attempt < retryCount {
			time.Sleep(retryInterval)
		}
	}

	return nil, lastErr
}

// fetchJWKS fetches JWKS from the given remote configuration
func (p *JwtAuthPolicy) fetchJWKS(remote *RemoteJWKS, fetchTimeout time.Duration) (*CachedJWKS, error) {
	// Create a new HTTP client per request to avoid race conditions on shared state
	var client *http.Client
	if remote.tlsConfig != nil {
		// Create a new client with custom TLS config
		customTransport := &http.Transport{
			TLSClientConfig: remote.tlsConfig,
		}
		client = &http.Client{
			Transport: customTransport,
			Timeout:   fetchTimeout,
		}
	} else {
		// Create a new client with default transport
		client = &http.Client{
			Timeout: fetchTimeout,
		}
	}

	resp, err := client.Get(remote.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var keySet JWKSKeySet
	if err := json.Unmarshal(body, &keySet); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	// Convert JWKS keys to RSA public keys
	cachedJWKS := &CachedJWKS{
		Keys: make(map[string]*rsa.PublicKey),
	}

	for _, key := range keySet.Keys {
		if key.Kty != "RSA" {
			continue // Only support RSA for now
		}
		if key.Kid == "" {
			continue // Skip keys without kid
		}

		// Parse RSA public key from N and E
		publicKey, err := parseRSAPublicKey(key.N, key.E)
		if err != nil {
			continue // Skip invalid keys
		}

		cachedJWKS.Keys[key.Kid] = publicKey
	}

	if len(cachedJWKS.Keys) == 0 {
		return nil, fmt.Errorf("no valid RSA keys found in JWKS")
	}

	return cachedJWKS, nil
}

// extractToken extracts JWT token from authorization header
func extractToken(authHeader, scheme string) string {
	authHeader = strings.TrimSpace(authHeader)
	if scheme != "" {
		prefix := scheme + " "
		if strings.HasPrefix(authHeader, prefix) {
			return strings.TrimPrefix(authHeader, prefix)
		}
		// If scheme is specified but not found, return empty
		return ""
	}
	// If no scheme specified, accept raw token or try to strip known schemes
	if strings.Contains(authHeader, " ") {
		parts := strings.SplitN(authHeader, " ", 2)
		return parts[1]
	}
	return authHeader
}

// parseRSAPublicKey parses RSA public key from modulus and exponent
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	// Decode modulus from base64url
	nBytes, err := decodeBase64URL(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode exponent from base64url
	eBytes, err := decodeBase64URL(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert bytes to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := bytesToInt(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

// decodeBase64URL decodes base64url encoded string
func decodeBase64URL(s string) ([]byte, error) {
	// Add padding if necessary
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	// Replace URL-safe characters
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	return base64.StdEncoding.DecodeString(s)
}

// bytesToInt converts bytes to int
func bytesToInt(b []byte) int {
	result := 0
	for _, byte := range b {
		result = (result << 8) | int(byte)
	}
	return result
}

// parseAudience parses audience claim which can be string or array
func parseAudience(audClaim interface{}) []string {
	if audStr, ok := audClaim.(string); ok {
		return []string{audStr}
	}
	if audArr, ok := audClaim.([]interface{}); ok {
		var result []string
		for _, a := range audArr {
			if aStr, ok := a.(string); ok {
				result = append(result, aStr)
			}
		}
		return result
	}
	return []string{}
}

// parseScopes parses scope claim (space-delimited string or array)
func parseScopes(scopeClaim, scpClaim interface{}) []string {
	var scopes []string

	// Check scope claim (space-delimited)
	if scopeStr, ok := scopeClaim.(string); ok {
		scopes = append(scopes, strings.Fields(scopeStr)...)
	}

	// Check scp claim (array)
	if scpArr, ok := scpClaim.([]interface{}); ok {
		for _, s := range scpArr {
			if sStr, ok := s.(string); ok {
				scopes = append(scopes, sStr)
			}
		}
	}

	return scopes
}

// handleAuthSuccess handles successful authentication
func (p *JwtAuthPolicy) handleAuthSuccess(ctx *policy.RequestContext, claims jwt.MapClaims, claimMappings map[string]string) policy.RequestAction {
	// Set metadata indicating successful authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = true
	ctx.Metadata[MetadataKeyAuthMethod] = "jwt"
	ctx.Metadata[MetadataKeyTokenClaims] = claims

	// Set standard metadata
	if iss, ok := claims["iss"].(string); ok {
		ctx.Metadata[MetadataKeyIssuer] = iss
	}
	if sub, ok := claims["sub"].(string); ok {
		ctx.Metadata[MetadataKeySubject] = sub
	}

	// Apply claim mappings as headers
	modifications := policy.UpstreamRequestModifications{
		SetHeaders: make(map[string]string),
	}

	for claimName, headerName := range claimMappings {
		if claimValue, ok := claims[claimName]; ok {
			// Convert claim value to string
			valueStr := claimValueToString(claimValue)
			modifications.SetHeaders[headerName] = valueStr
		}
	}

	return modifications
}

// OnResponse is not used by this policy (authentication is request-only)
func (p *JwtAuthPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil // No response processing needed
}

// handleAuthFailure handles authentication failure
func (p *JwtAuthPolicy) handleAuthFailure(ctx *policy.RequestContext, statusCode int, errorFormat, reason string) policy.RequestAction {
	// Set metadata indicating failed authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = false
	ctx.Metadata[MetadataKeyAuthMethod] = "jwt"

	headers := map[string]string{
		"content-type": "application/json",
	}

	var body string
	switch errorFormat {
	case "plain":
		body = fmt.Sprintf("Authentication failed: %s", reason)
		headers["content-type"] = "text/plain"
	case "minimal":
		body = "Unauthorized"
	default: // json
		errResponse := map[string]interface{}{
			"error":   "Unauthorized",
			"message": fmt.Sprintf("JWT authentication failed: %s", reason),
		}
		bodyBytes, _ := json.Marshal(errResponse)
		body = string(bodyBytes)
	}

	return policy.ImmediateResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       []byte(body),
	}
}

// Helper functions for type assertions
func getString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if v, ok := params[key]; ok {
		if i, ok := v.(int); ok {
			return i
		}
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}

func getStringArrayParam(params map[string]interface{}, key string, defaultValue []string) []string {
	if v, ok := params[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			var result []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}

func getStringMapParam(params map[string]interface{}, key string, defaultValue map[string]string) map[string]string {
	if v, ok := params[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			result := make(map[string]string)
			for k, val := range m {
				if s, ok := val.(string); ok {
					result[k] = s
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}

func claimValueToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", int64(val))
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		bytes, _ := json.Marshal(val)
		return string(bytes)
	}
}

// loadTLSConfig loads TLS configuration from a certificate file for validating self-signed certificates
// When a custom CA certificate is provided, hostname verification is skipped to allow
// self-signed certificates with any hostname to be used (useful for development/testing)
func loadTLSConfig(certPath string) (*tls.Config, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(certData) {
		return nil, fmt.Errorf("failed to parse PEM certificate from %s", certPath)
	}

	return &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}, nil
}

// loadPublicKeyFromCertificate loads an RSA public key from a certificate file
func loadPublicKeyFromCertificate(certPath string) (*rsa.PublicKey, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	return parsePublicKeyFromString(string(certData))
}

// parsePublicKeyFromString parses an RSA public key from a PEM-encoded string
func parsePublicKeyFromString(pemData string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from certificate data")
	}

	// Try to parse as a certificate first
	cert, err := x509.ParseCertificate(block.Bytes)
	if err == nil {
		// Extract public key from certificate
		if publicKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			return publicKey, nil
		}
		return nil, fmt.Errorf("certificate does not contain an RSA public key")
	}

	// If certificate parsing fails, try to parse as a public key directly
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key from certificate data: %w", err)
	}

	if rsaPublicKey, ok := publicKey.(*rsa.PublicKey); ok {
		return rsaPublicKey, nil
	}

	return nil, fmt.Errorf("certificate data does not contain an RSA public key")
}
