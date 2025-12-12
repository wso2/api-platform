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
	"log/slog"
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

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	slog.Debug("JWT Auth Policy: GetPolicy called")
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
	slog.Debug("JWT Auth Policy: OnRequest started",
		"path", ctx.Path,
		"method", ctx.Method,
	)

	// Get system configuration
	headerName := getStringParam(params, "headerName", "Authorization")
	authHeaderScheme := getStringParam(params, "authHeaderScheme", "Bearer")
	onFailureStatusCode := getIntParam(params, "onFailureStatusCode", 401)
	errorMessageFormat := getStringParam(params, "errorMessageFormat", "json")
	errorMessage := getStringParam(params, "errorMessage", "Authentication failed")
	leewayStr := getStringParam(params, "leeway", "30s")
	allowedAlgorithms := getStringArrayParam(params, "allowedAlgorithms", []string{"RS256", "ES256"})
	jwksCacheTtlStr := getStringParam(params, "jwksCacheTtl", "5m")
	jwksFetchTimeoutStr := getStringParam(params, "jwksFetchTimeout", "5s")
	jwksFetchRetryCount := getIntParam(params, "jwksFetchRetryCount", 3)
	jwksFetchRetryIntervalStr := getStringParam(params, "jwksFetchRetryInterval", "2s")
	validateIssuer := getBoolParam(params, "validateIssuer", true)

	slog.Debug("JWT Auth Policy: Configuration loaded",
		"headerName", headerName,
		"authHeaderScheme", authHeaderScheme,
		"onFailureStatusCode", onFailureStatusCode,
		"errorMessageFormat", errorMessageFormat,
		"errorMessage", errorMessage,
		"leeway", leewayStr,
		"allowedAlgorithms", allowedAlgorithms,
		"jwksCacheTtl", jwksCacheTtlStr,
		"jwksFetchTimeout", jwksFetchTimeoutStr,
		"jwksFetchRetryCount", jwksFetchRetryCount,
		"jwksFetchRetryInterval", jwksFetchRetryIntervalStr,
		"validateIssuer", validateIssuer,
	)

	// Parse durations
	leeway, err := time.ParseDuration(leewayStr)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse leeway duration, using default",
			"leewayStr", leewayStr,
			"error", err,
			"defaultLeeway", "30s",
		)
		leeway = 30 * time.Second
	}

	jwksCacheTtl, err := time.ParseDuration(jwksCacheTtlStr)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse jwksCacheTtl duration, using default",
			"jwksCacheTtlStr", jwksCacheTtlStr,
			"error", err,
			"defaultCacheTtl", "5m",
		)
		jwksCacheTtl = 5 * time.Minute
	}

	jwksFetchTimeout, err := time.ParseDuration(jwksFetchTimeoutStr)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse jwksFetchTimeout duration, using default",
			"jwksFetchTimeoutStr", jwksFetchTimeoutStr,
			"error", err,
			"defaultTimeout", "5s",
		)
		jwksFetchTimeout = 5 * time.Second
	}

	jwksFetchRetryInterval, err := time.ParseDuration(jwksFetchRetryIntervalStr)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse jwksFetchRetryInterval duration, using default",
			"jwksFetchRetryIntervalStr", jwksFetchRetryIntervalStr,
			"error", err,
			"defaultInterval", "2s",
		)
		jwksFetchRetryInterval = 2 * time.Second
	}

	slog.Debug("JWT Auth Policy: Parsed duration values",
		"leeway", leeway,
		"jwksCacheTtl", jwksCacheTtl,
		"jwksFetchTimeout", jwksFetchTimeout,
		"jwksFetchRetryInterval", jwksFetchRetryInterval,
	)

	// Get key managers configuration
	keyManagersRaw, ok := params["keyManagers"]
	if !ok {
		slog.Debug("JWT Auth Policy: Key managers not configured in params")
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, "key managers not configured")
	}

	slog.Debug("JWT Auth Policy: Starting to parse key managers configuration")

	// Parse keyManagers
	keyManagers := make(map[string]*KeyManager)
	keyManagersList, ok := keyManagersRaw.([]interface{})
	if ok {
		for _, km := range keyManagersList {
			if kmMap, ok := km.(map[string]interface{}); ok {
				name := getString(kmMap["name"])
				issuer := getString(kmMap["issuer"])

				if name == "" {
					slog.Debug("JWT Auth Policy: Skipping key manager with empty name")
					continue
				}

				slog.Debug("JWT Auth Policy: Processing key manager",
					"name", name,
					"issuer", issuer,
				)

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
							slog.Debug("JWT Auth Policy: Configuring remote JWKS",
								"keyManager", name,
								"uri", uri,
								"certificatePath", certPath,
								"skipTlsVerify", skipTlsVerify,
							)
							remoteJWKS := &RemoteJWKS{
								URI:             uri,
								CertificatePath: certPath,
								SkipTlsVerify:   skipTlsVerify,
							}
							// Load and cache TLS config if certificate path is provided
							if certPath != "" {
								tlsConfig, err := loadTLSConfig(certPath)
								if err != nil {
									slog.Debug("JWT Auth Policy: Failed to load TLS config for remote JWKS",
										"keyManager", name,
										"certificatePath", certPath,
										"error", err,
									)
									continue // Skip this key manager if cert loading fails
								}
								slog.Debug("JWT Auth Policy: Successfully loaded TLS config for remote JWKS",
									"keyManager", name,
									"certificatePath", certPath,
								)
								remoteJWKS.tlsConfig = tlsConfig
							} else if skipTlsVerify {
								slog.Debug("JWT Auth Policy: Configuring TLS to skip verification",
									"keyManager", name,
								)
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

						slog.Debug("JWT Auth Policy: Processing local certificate configuration",
							"keyManager", name,
							"hasInline", inline != "",
							"certificatePath", certPath,
						)

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
								slog.Debug("JWT Auth Policy: Parsing inline certificate",
									"keyManager", name,
								)
								publicKey, err = parsePublicKeyFromString(inline)
							} else if certPath != "" {
								slog.Debug("JWT Auth Policy: Loading certificate from file",
									"keyManager", name,
									"certificatePath", certPath,
								)
								publicKey, err = loadPublicKeyFromCertificate(certPath)
							}

							if err != nil {
								slog.Debug("JWT Auth Policy: Failed to load local certificate",
									"keyManager", name,
									"error", err,
								)
								continue // Skip this key manager if cert loading fails
							}
							slog.Debug("JWT Auth Policy: Successfully loaded local certificate",
								"keyManager", name,
							)
							localCert.PublicKey = publicKey
							jwksConfig.Local = localCert
						}
					}

					// Accept key manager if either remote or local is configured
					if jwksConfig.Remote != nil || jwksConfig.Local != nil {
						keyManager.JWKS = jwksConfig
						keyManagers[name] = keyManager
						slog.Debug("JWT Auth Policy: Key manager added successfully",
							"keyManager", name,
							"hasRemote", jwksConfig.Remote != nil,
							"hasLocal", jwksConfig.Local != nil,
						)
					} else {
						slog.Debug("JWT Auth Policy: Key manager skipped - no remote or local JWKS configured",
							"keyManager", name,
						)
					}
				}
			}
		}
	}

	if len(keyManagers) == 0 {
		slog.Debug("JWT Auth Policy: No key managers configured after parsing")
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, "no key managers configured")
	}

	slog.Debug("JWT Auth Policy: Key managers configured",
		"count", len(keyManagers),
	)

	// Get user configuration
	userIssuers := getStringArrayParam(params, "issuers", []string{})
	userAudiences := getStringArrayParam(params, "audiences", []string{})
	userRequiredScopes := getStringArrayParam(params, "requiredScopes", []string{})
	userRequiredClaims := getStringMapParam(params, "requiredClaims", map[string]string{})
	userClaimMappings := getStringMapParam(params, "claimMappings", map[string]string{})
	userAuthHeaderPrefix := getStringParam(params, "authHeaderPrefix", "")

	slog.Debug("JWT Auth Policy: User configuration loaded",
		"issuers", userIssuers,
		"audiences", userAudiences,
		"requiredScopes", userRequiredScopes,
		"requiredClaimsCount", len(userRequiredClaims),
		"claimMappingsCount", len(userClaimMappings),
		"authHeaderPrefix", userAuthHeaderPrefix,
	)

	// Use user override if provided
	if userAuthHeaderPrefix != "" {
		slog.Debug("JWT Auth Policy: Overriding auth header scheme with user prefix",
			"originalScheme", authHeaderScheme,
			"newScheme", userAuthHeaderPrefix,
		)
		authHeaderScheme = userAuthHeaderPrefix
	}

	// Extract token from header
	authHeaders := ctx.Headers.Get(strings.ToLower(headerName))
	if len(authHeaders) == 0 {
		slog.Debug("JWT Auth Policy: Missing authorization header",
			"headerName", headerName,
		)
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, "missing authorization header")
	}

	authHeader := authHeaders[0]
	slog.Debug("JWT Auth Policy: Authorization header found",
		"headerName", headerName,
		"headerValueLength", len(authHeader),
	)

	token := extractToken(authHeader, authHeaderScheme)
	if token == "" {
		slog.Debug("JWT Auth Policy: Failed to extract token from authorization header",
			"authHeaderScheme", authHeaderScheme,
		)
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, "invalid authorization header format")
	}

	slog.Debug("JWT Auth Policy: Token extracted successfully",
		"tokenLength", len(token),
	)

	// Parse token to get header info
	unverifiedToken, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse token",
			"error", err,
		)
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, "invalid token format")
	}

	slog.Debug("JWT Auth Policy: Token parsed successfully",
		"algorithm", unverifiedToken.Header["alg"],
		"keyId", unverifiedToken.Header["kid"],
		"type", unverifiedToken.Header["typ"],
	)

	// Validate token and signature
	claims, err := p.validateTokenWithSignature(token, unverifiedToken, keyManagers, userIssuers, validateIssuer,
		allowedAlgorithms, leeway, jwksCacheTtl, jwksFetchTimeout, jwksFetchRetryCount, jwksFetchRetryInterval)
	if err != nil {
		slog.Debug("JWT Auth Policy: Token validation failed",
			"error", err,
		)
		return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, fmt.Sprintf("token validation failed: %v", err))
	}

	slog.Debug("JWT Auth Policy: Token signature validated successfully")

	// Note: Issuer validation is now done in validateTokenWithSignature when userIssuers is specified
	// or when validateIssuer is true. No need for duplicate validation here.

	// Validate audience if specified
	if len(userAudiences) > 0 {
		aud := parseAudience(claims["aud"])
		slog.Debug("JWT Auth Policy: Validating audience",
			"tokenAudiences", aud,
			"requiredAudiences", userAudiences,
		)
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
			slog.Debug("JWT Auth Policy: No valid audience found in token",
				"tokenAudiences", aud,
			)
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, "no valid audience found in token")
		}
		slog.Debug("JWT Auth Policy: Audience validation passed")
	}

	// Validate required scopes if specified
	if len(userRequiredScopes) > 0 {
		scopes := parseScopes(claims["scope"], claims["scp"])
		slog.Debug("JWT Auth Policy: Validating required scopes",
			"tokenScopes", scopes,
			"requiredScopes", userRequiredScopes,
		)
		for _, requiredScope := range userRequiredScopes {
			found := false
			for _, tokenScope := range scopes {
				if tokenScope == requiredScope {
					found = true
					break
				}
			}
			if !found {
				slog.Debug("JWT Auth Policy: Required scope not found",
					"missingScope", requiredScope,
					"tokenScopes", scopes,
				)
				return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, fmt.Sprintf("required scope '%s' not found", requiredScope))
			}
		}
		slog.Debug("JWT Auth Policy: Scope validation passed")
	}

	// Validate required claims if specified
	if len(userRequiredClaims) > 0 {
		slog.Debug("JWT Auth Policy: Validating required claims",
			"requiredClaimsCount", len(userRequiredClaims),
		)
	}
	for claimName, expectedValue := range userRequiredClaims {
		claimValue := getString(claims[claimName])
		slog.Debug("JWT Auth Policy: Checking required claim",
			"claimName", claimName,
			"expectedValue", expectedValue,
			"actualValue", claimValue,
		)
		if claimValue != expectedValue {
			slog.Debug("JWT Auth Policy: Required claim validation failed",
				"claimName", claimName,
				"expectedValue", expectedValue,
				"actualValue", claimValue,
			)
			return p.handleAuthFailure(ctx, onFailureStatusCode, errorMessageFormat, errorMessage, fmt.Sprintf("claim '%s' validation failed", claimName))
		}
	}

	slog.Debug("JWT Auth Policy: All validations passed, authentication successful")

	// Authentication successful - apply claim mappings and set metadata
	return p.handleAuthSuccess(ctx, claims, userClaimMappings)
}

// validateTokenWithSignature validates JWT signature using JWKS
func (p *JwtAuthPolicy) validateTokenWithSignature(tokenString string, unverifiedToken *jwt.Token,
	keyManagers map[string]*KeyManager, userIssuers []string, validateIssuer bool, allowedAlgorithms []string,
	leeway time.Duration, cacheTTL time.Duration, fetchTimeout time.Duration, retryCount int, retryInterval time.Duration) (jwt.MapClaims, error) {

	slog.Debug("JWT Auth Policy: Starting token signature validation",
		"keyManagersCount", len(keyManagers),
		"userIssuersCount", len(userIssuers),
		"validateIssuer", validateIssuer,
		"allowedAlgorithms", allowedAlgorithms,
	)

	unverifiedClaims, ok := unverifiedToken.Claims.(jwt.MapClaims)
	if !ok {
		slog.Debug("JWT Auth Policy: Invalid token claims format")
		return nil, fmt.Errorf("invalid token claims format")
	}

	// Check allowed algorithms
	alg, ok := unverifiedToken.Header["alg"].(string)
	if !ok {
		slog.Debug("JWT Auth Policy: Missing algorithm in token header")
		return nil, fmt.Errorf("missing algorithm in token")
	}

	slog.Debug("JWT Auth Policy: Checking algorithm",
		"tokenAlgorithm", alg,
		"allowedAlgorithms", allowedAlgorithms,
	)

	algorithmAllowed := false
	for _, allowed := range allowedAlgorithms {
		if alg == allowed {
			algorithmAllowed = true
			break
		}
	}
	if !algorithmAllowed {
		slog.Debug("JWT Auth Policy: Algorithm not in allowed list",
			"tokenAlgorithm", alg,
		)
		return nil, fmt.Errorf("algorithm '%s' not in allowed list", alg)
	}

	slog.Debug("JWT Auth Policy: Algorithm check passed",
		"algorithm", alg,
	)

	// Validate exp and nbf with leeway
	now := time.Now()
	if exp, ok := unverifiedClaims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		slog.Debug("JWT Auth Policy: Checking token expiration",
			"expTime", expTime,
			"now", now,
			"leeway", leeway,
		)
		if now.After(expTime.Add(leeway)) {
			slog.Debug("JWT Auth Policy: Token has expired",
				"expTime", expTime,
				"now", now,
			)
			return nil, fmt.Errorf("token expired")
		}
		slog.Debug("JWT Auth Policy: Token expiration check passed")
	} else {
		slog.Debug("JWT Auth Policy: No 'exp' claim found in token")
	}

	if nbf, ok := unverifiedClaims["nbf"].(float64); ok {
		nbfTime := time.Unix(int64(nbf), 0)
		slog.Debug("JWT Auth Policy: Checking token not-before time",
			"nbfTime", nbfTime,
			"now", now,
			"leeway", leeway,
		)
		if now.Before(nbfTime.Add(-leeway)) {
			slog.Debug("JWT Auth Policy: Token not yet valid",
				"nbfTime", nbfTime,
				"now", now,
			)
			return nil, fmt.Errorf("token not yet valid")
		}
		slog.Debug("JWT Auth Policy: Token not-before check passed")
	} else {
		slog.Debug("JWT Auth Policy: No 'nbf' claim found in token")
	}

	// Get issuer from token
	tokenIssuer := getString(unverifiedClaims["iss"])
	slog.Debug("JWT Auth Policy: Token issuer",
		"issuer", tokenIssuer,
	)

	// Determine which key managers to use
	var applicableKeyManagers []*KeyManager
	if len(userIssuers) > 0 {
		// User specified issuers - these could be actual issuer values or key manager names
		slog.Debug("JWT Auth Policy: User-specified issuers provided",
			"userIssuers", userIssuers,
			"tokenIssuer", tokenIssuer,
		)

		// First, check if token issuer matches one of the user-specified issuers (actual issuer match)
		var matchedIssuer string
		for _, userIssuer := range userIssuers {
			if tokenIssuer == userIssuer {
				matchedIssuer = userIssuer
				break
			}
		}

		if matchedIssuer != "" {
			// Token issuer matches a user-specified issuer, find the key manager
			slog.Debug("JWT Auth Policy: Token issuer matches user-specified issuer",
				"matchedIssuer", matchedIssuer,
			)

			// Find key manager by issuer field
			for _, km := range keyManagers {
				if km.Issuer == matchedIssuer {
					applicableKeyManagers = append(applicableKeyManagers, km)
					slog.Debug("JWT Auth Policy: Found key manager by issuer field",
						"keyManager", km.Name,
						"issuer", matchedIssuer,
					)
					break
				}
			}
		}

		// If no key manager found by issuer match, try to find by key manager name
		// (userIssuers might contain key manager names instead of actual issuers)
		if len(applicableKeyManagers) == 0 {
			for _, userIssuer := range userIssuers {
				if km, ok := keyManagers[userIssuer]; ok {
					// Found key manager by name
					// Now verify: if the key manager has an issuer configured, token issuer must match it
					// If no issuer configured on key manager, we accept the token
					if km.Issuer == "" || km.Issuer == tokenIssuer {
						applicableKeyManagers = append(applicableKeyManagers, km)
						slog.Debug("JWT Auth Policy: Found key manager by name",
							"keyManager", km.Name,
							"userIssuer", userIssuer,
							"tokenIssuer", tokenIssuer,
							"kmIssuer", km.Issuer,
						)
						break
					} else {
						slog.Debug("JWT Auth Policy: Key manager found by name but issuer mismatch",
							"keyManager", km.Name,
							"tokenIssuer", tokenIssuer,
							"expectedIssuer", km.Issuer,
						)
					}
				}
			}
		}

		// If still no applicable key managers found, reject the token
		if len(applicableKeyManagers) == 0 {
			slog.Debug("JWT Auth Policy: No matching key manager found for user-specified issuers",
				"tokenIssuer", tokenIssuer,
				"userIssuers", userIssuers,
			)
			return nil, fmt.Errorf("token issuer '%s' does not match any configured issuer or key manager", tokenIssuer)
		}
	} else if tokenIssuer != "" {
		// No user issuers specified, but token has issuer claim
		slog.Debug("JWT Auth Policy: Matching token issuer to key managers",
			"tokenIssuer", tokenIssuer,
			"validateIssuer", validateIssuer,
		)
		for _, km := range keyManagers {
			if km.Issuer == tokenIssuer {
				applicableKeyManagers = append(applicableKeyManagers, km)
				slog.Debug("JWT Auth Policy: Found matching key manager by issuer",
					"keyManager", km.Name,
					"issuer", km.Issuer,
				)
				break
			}
		}

		// If no issuer match found
		if len(applicableKeyManagers) == 0 {
			if validateIssuer {
				// Strict mode: reject if token issuer doesn't match any key manager
				slog.Debug("JWT Auth Policy: No key manager found for token issuer (validateIssuer=true)",
					"tokenIssuer", tokenIssuer,
				)
				return nil, fmt.Errorf("no key manager configured for token issuer '%s'", tokenIssuer)
			} else {
				// Lenient mode: try all key managers
				slog.Debug("JWT Auth Policy: No issuer match found, using all key managers (validateIssuer=false)")
				for _, km := range keyManagers {
					applicableKeyManagers = append(applicableKeyManagers, km)
				}
			}
		}
	} else {
		// No issuer in token
		if validateIssuer {
			slog.Debug("JWT Auth Policy: Token has no issuer claim (validateIssuer=true)")
			return nil, fmt.Errorf("token does not contain an issuer claim")
		}
		// Lenient mode: try all key managers
		slog.Debug("JWT Auth Policy: No issuer in token, using all key managers (validateIssuer=false)")
		for _, km := range keyManagers {
			applicableKeyManagers = append(applicableKeyManagers, km)
		}
	}

	slog.Debug("JWT Auth Policy: Applicable key managers determined",
		"count", len(applicableKeyManagers),
	)

	// Get kid from token header
	kid, ok := unverifiedToken.Header["kid"].(string)
	if !ok {
		// Kid is optional for certificate-based validation
		kid = ""
		slog.Debug("JWT Auth Policy: No 'kid' found in token header")
	} else {
		slog.Debug("JWT Auth Policy: Token key ID found",
			"kid", kid,
		)
	}

	// Try to verify signature with applicable key managers
	var lastErr error
	for _, km := range applicableKeyManagers {
		slog.Debug("JWT Auth Policy: Attempting signature verification with key manager",
			"keyManager", km.Name,
			"issuer", km.Issuer,
		)

		if km.JWKS == nil {
			slog.Debug("JWT Auth Policy: Key manager has no JWKS configuration, skipping",
				"keyManager", km.Name,
			)
			continue
		}

		// Try local certificate validation first if available
		if km.JWKS.Local != nil && km.JWKS.Local.PublicKey != nil {
			slog.Debug("JWT Auth Policy: Attempting signature verification with local certificate",
				"keyManager", km.Name,
			)
			verifiedToken, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
				return km.JWKS.Local.PublicKey, nil
			})

			if err == nil {
				// Signature verified successfully with local certificate
				slog.Debug("JWT Auth Policy: Signature verified successfully with local certificate",
					"keyManager", km.Name,
				)
				if claims, ok := verifiedToken.Claims.(jwt.MapClaims); ok {
					return claims, nil
				}
			}
			slog.Debug("JWT Auth Policy: Signature verification failed with local certificate",
				"keyManager", km.Name,
				"error", err,
			)
			lastErr = fmt.Errorf("signature verification failed with local certificate: %w", err)
			continue
		}

		// Fall back to remote JWKS-based validation
		if km.JWKS.Remote != nil {
			slog.Debug("JWT Auth Policy: Attempting signature verification with remote JWKS",
				"keyManager", km.Name,
				"jwksUri", km.JWKS.Remote.URI,
			)
			// Get JWKS with retry logic
			jwks, err := p.fetchJWKSWithRetry(km.JWKS.Remote, cacheTTL, fetchTimeout, retryCount, retryInterval)
			if err != nil {
				slog.Debug("JWT Auth Policy: Failed to fetch JWKS",
					"keyManager", km.Name,
					"jwksUri", km.JWKS.Remote.URI,
					"error", err,
				)
				lastErr = fmt.Errorf("failed to fetch JWKS from %s: %w", km.JWKS.Remote.URI, err)
				continue
			}

			slog.Debug("JWT Auth Policy: JWKS fetched successfully",
				"keyManager", km.Name,
				"keysCount", len(jwks.Keys),
			)

			// If kid is present, find the key with matching kid
			if kid != "" {
				slog.Debug("JWT Auth Policy: Looking for key with matching kid",
					"kid", kid,
				)
				publicKey, ok := jwks.Keys[kid]
				if !ok {
					slog.Debug("JWT Auth Policy: Key ID not found in JWKS",
						"kid", kid,
						"availableKids", getKeyIds(jwks.Keys),
					)
					lastErr = fmt.Errorf("key id '%s' not found in JWKS from %s", kid, km.JWKS.Remote.URI)
					continue
				}

				slog.Debug("JWT Auth Policy: Found key with matching kid, verifying signature",
					"kid", kid,
				)
				// Verify signature
				verifiedToken, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
					return publicKey, nil
				})

				if err != nil {
					slog.Debug("JWT Auth Policy: Signature verification failed",
						"kid", kid,
						"error", err,
					)
					lastErr = fmt.Errorf("signature verification failed: %w", err)
					continue
				}

				// Signature verified successfully
				slog.Debug("JWT Auth Policy: Signature verified successfully with kid",
					"kid", kid,
					"keyManager", km.Name,
				)
				if claims, ok := verifiedToken.Claims.(jwt.MapClaims); ok {
					return claims, nil
				}
			} else {
				// No kid, try all keys in JWKS
				slog.Debug("JWT Auth Policy: No kid in token, trying all keys in JWKS",
					"keysCount", len(jwks.Keys),
				)
				for keyId, publicKey := range jwks.Keys {
					slog.Debug("JWT Auth Policy: Trying key from JWKS",
						"keyId", keyId,
					)
					verifiedToken, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
						return publicKey, nil
					})

					if err == nil {
						// Signature verified successfully
						slog.Debug("JWT Auth Policy: Signature verified successfully",
							"keyId", keyId,
							"keyManager", km.Name,
						)
						if claims, ok := verifiedToken.Claims.(jwt.MapClaims); ok {
							return claims, nil
						}
					} else {
						slog.Debug("JWT Auth Policy: Signature verification failed with key",
							"keyId", keyId,
							"error", err,
						)
					}
				}
				lastErr = fmt.Errorf("token signature verification failed with all keys from %s", km.JWKS.Remote.URI)
				slog.Debug("JWT Auth Policy: Failed to verify signature with any key from JWKS",
					"keyManager", km.Name,
				)
			}
		}
	}

	// If no key manager succeeded
	if lastErr != nil {
		slog.Debug("JWT Auth Policy: All key managers failed to verify signature",
			"lastError", lastErr,
		)
		return nil, lastErr
	}
	slog.Debug("JWT Auth Policy: Unable to verify token signature with any available key manager")
	return nil, fmt.Errorf("unable to verify token signature with available key managers")
}

// fetchJWKSWithRetry fetches JWKS with caching and retry logic
func (p *JwtAuthPolicy) fetchJWKSWithRetry(remote *RemoteJWKS, cacheTTL time.Duration, fetchTimeout time.Duration, retryCount int, retryInterval time.Duration) (*CachedJWKS, error) {
	slog.Debug("JWT Auth Policy: fetchJWKSWithRetry called",
		"uri", remote.URI,
		"cacheTTL", cacheTTL,
		"fetchTimeout", fetchTimeout,
		"retryCount", retryCount,
		"retryInterval", retryInterval,
	)

	// Check cache first
	p.cacheMutex.RLock()
	if cached, ok := p.cacheStore[remote.URI]; ok {
		if ttl, ok := p.cacheTTLs[remote.URI]; ok && time.Now().Before(ttl) {
			p.cacheMutex.RUnlock()
			slog.Debug("JWT Auth Policy: JWKS cache hit",
				"uri", remote.URI,
				"cacheExpiry", ttl,
				"keysCount", len(cached.Keys),
			)
			return cached, nil
		}
		slog.Debug("JWT Auth Policy: JWKS cache expired",
			"uri", remote.URI,
		)
	} else {
		slog.Debug("JWT Auth Policy: JWKS not in cache",
			"uri", remote.URI,
		)
	}
	p.cacheMutex.RUnlock()

	// Not in cache or expired, fetch from server
	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		slog.Debug("JWT Auth Policy: Fetching JWKS from server",
			"uri", remote.URI,
			"attempt", attempt+1,
			"maxAttempts", retryCount+1,
		)
		jwks, err := p.fetchJWKS(remote, fetchTimeout)
		if err == nil {
			// Cache the result
			p.cacheMutex.Lock()
			p.cacheStore[remote.URI] = jwks
			p.cacheTTLs[remote.URI] = time.Now().Add(cacheTTL)
			p.cacheMutex.Unlock()
			slog.Debug("JWT Auth Policy: JWKS fetched and cached successfully",
				"uri", remote.URI,
				"keysCount", len(jwks.Keys),
				"cacheExpiry", time.Now().Add(cacheTTL),
			)
			return jwks, nil
		}

		slog.Debug("JWT Auth Policy: JWKS fetch attempt failed",
			"uri", remote.URI,
			"attempt", attempt+1,
			"error", err,
		)
		lastErr = err
		if attempt < retryCount {
			slog.Debug("JWT Auth Policy: Waiting before retry",
				"retryInterval", retryInterval,
			)
			time.Sleep(retryInterval)
		}
	}

	slog.Debug("JWT Auth Policy: All JWKS fetch attempts failed",
		"uri", remote.URI,
		"lastError", lastErr,
	)
	return nil, lastErr
}

// fetchJWKS fetches JWKS from the given remote configuration
func (p *JwtAuthPolicy) fetchJWKS(remote *RemoteJWKS, fetchTimeout time.Duration) (*CachedJWKS, error) {
	slog.Debug("JWT Auth Policy: fetchJWKS called",
		"uri", remote.URI,
		"timeout", fetchTimeout,
		"hasTlsConfig", remote.tlsConfig != nil,
	)

	// Create a new HTTP client per request to avoid race conditions on shared state
	var client *http.Client
	if remote.tlsConfig != nil {
		// Create a new client with custom TLS config
		slog.Debug("JWT Auth Policy: Creating HTTP client with custom TLS config",
			"uri", remote.URI,
		)
		customTransport := &http.Transport{
			TLSClientConfig: remote.tlsConfig,
		}
		client = &http.Client{
			Transport: customTransport,
			Timeout:   fetchTimeout,
		}
	} else {
		// Create a new client with default transport
		slog.Debug("JWT Auth Policy: Creating HTTP client with default transport",
			"uri", remote.URI,
		)
		client = &http.Client{
			Timeout: fetchTimeout,
		}
	}

	slog.Debug("JWT Auth Policy: Sending HTTP GET request to JWKS endpoint",
		"uri", remote.URI,
	)
	resp, err := client.Get(remote.URI)
	if err != nil {
		slog.Debug("JWT Auth Policy: HTTP request to JWKS endpoint failed",
			"uri", remote.URI,
			"error", err,
		)
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("JWT Auth Policy: JWKS endpoint response received",
		"uri", remote.URI,
		"statusCode", resp.StatusCode,
	)

	if resp.StatusCode != http.StatusOK {
		slog.Debug("JWT Auth Policy: JWKS endpoint returned non-OK status",
			"uri", remote.URI,
			"statusCode", resp.StatusCode,
		)
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to read JWKS response body",
			"uri", remote.URI,
			"error", err,
		)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Debug("JWT Auth Policy: JWKS response body read successfully",
		"uri", remote.URI,
		"bodyLength", len(body),
	)

	var keySet JWKSKeySet
	if err := json.Unmarshal(body, &keySet); err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse JWKS JSON",
			"uri", remote.URI,
			"error", err,
		)
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	slog.Debug("JWT Auth Policy: JWKS JSON parsed successfully",
		"uri", remote.URI,
		"keysInResponse", len(keySet.Keys),
	)

	// Convert JWKS keys to RSA public keys
	cachedJWKS := &CachedJWKS{
		Keys: make(map[string]*rsa.PublicKey),
	}

	for _, key := range keySet.Keys {
		slog.Debug("JWT Auth Policy: Processing JWKS key",
			"kid", key.Kid,
			"kty", key.Kty,
			"alg", key.Alg,
			"use", key.Use,
		)
		if key.Kty != "RSA" {
			slog.Debug("JWT Auth Policy: Skipping non-RSA key",
				"kid", key.Kid,
				"kty", key.Kty,
			)
			continue // Only support RSA for now
		}
		if key.Kid == "" {
			slog.Debug("JWT Auth Policy: Skipping key without kid")
			continue // Skip keys without kid
		}

		// Parse RSA public key from N and E
		publicKey, err := parseRSAPublicKey(key.N, key.E)
		if err != nil {
			slog.Debug("JWT Auth Policy: Failed to parse RSA public key",
				"kid", key.Kid,
				"error", err,
			)
			continue // Skip invalid keys
		}

		cachedJWKS.Keys[key.Kid] = publicKey
		slog.Debug("JWT Auth Policy: RSA public key parsed successfully",
			"kid", key.Kid,
		)
	}

	if len(cachedJWKS.Keys) == 0 {
		slog.Debug("JWT Auth Policy: No valid RSA keys found in JWKS",
			"uri", remote.URI,
		)
		return nil, fmt.Errorf("no valid RSA keys found in JWKS")
	}

	slog.Debug("JWT Auth Policy: JWKS processing complete",
		"uri", remote.URI,
		"validKeysCount", len(cachedJWKS.Keys),
	)

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
	slog.Debug("JWT Auth Policy: handleAuthSuccess called",
		"claimMappingsCount", len(claimMappings),
	)

	// Set metadata indicating successful authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = true
	ctx.Metadata[MetadataKeyAuthMethod] = "jwt"
	ctx.Metadata[MetadataKeyTokenClaims] = claims

	// Set standard metadata
	if iss, ok := claims["iss"].(string); ok {
		ctx.Metadata[MetadataKeyIssuer] = iss
		slog.Debug("JWT Auth Policy: Set issuer metadata",
			"issuer", iss,
		)
	}
	if sub, ok := claims["sub"].(string); ok {
		ctx.Metadata[MetadataKeySubject] = sub
		slog.Debug("JWT Auth Policy: Set subject metadata",
			"subject", sub,
		)
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
			slog.Debug("JWT Auth Policy: Mapped claim to header",
				"claimName", claimName,
				"headerName", headerName,
				"valueLength", len(valueStr),
			)
		} else {
			slog.Debug("JWT Auth Policy: Claim not found for mapping",
				"claimName", claimName,
				"headerName", headerName,
			)
		}
	}

	slog.Debug("JWT Auth Policy: Authentication successful, returning modifications",
		"headersToSet", len(modifications.SetHeaders),
	)

	return modifications
}

// OnResponse is not used by this policy (authentication is request-only)
func (p *JwtAuthPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil // No response processing needed
}

// handleAuthFailure handles authentication failure
func (p *JwtAuthPolicy) handleAuthFailure(ctx *policy.RequestContext, statusCode int, errorFormat, errorMessage, reason string) policy.RequestAction {
	slog.Debug("JWT Auth Policy: handleAuthFailure called",
		"statusCode", statusCode,
		"errorFormat", errorFormat,
		"errorMessage", errorMessage,
		"reason", reason,
	)

	// Set metadata indicating failed authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = false
	ctx.Metadata[MetadataKeyAuthMethod] = "jwt"

	headers := map[string]string{
		"content-type": "application/json",
	}

	var body string
	switch errorFormat {
	case "plain":
		body = errorMessage
		headers["content-type"] = "text/plain"
	default: // json
		errResponse := map[string]interface{}{
			"error":   "Unauthorized",
			"message": errorMessage,
		}
		bodyBytes, _ := json.Marshal(errResponse)
		body = string(bodyBytes)
	}

	slog.Debug("JWT Auth Policy: Returning immediate response",
		"statusCode", statusCode,
		"contentType", headers["content-type"],
		"bodyLength", len(body),
	)

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

func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := params[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
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

// getKeyIds returns a slice of key IDs from a map of RSA public keys
func getKeyIds(keys map[string]*rsa.PublicKey) []string {
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	return ids
}

// loadTLSConfig loads TLS configuration from a certificate file for validating self-signed certificates
// When a custom CA certificate is provided, hostname verification is skipped to allow
// self-signed certificates with any hostname to be used (useful for development/testing)
func loadTLSConfig(certPath string) (*tls.Config, error) {
	slog.Debug("JWT Auth Policy: loadTLSConfig called",
		"certPath", certPath,
	)

	certData, err := os.ReadFile(certPath)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to read certificate file",
			"certPath", certPath,
			"error", err,
		)
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	slog.Debug("JWT Auth Policy: Certificate file read successfully",
		"certPath", certPath,
		"dataLength", len(certData),
	)

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(certData) {
		slog.Debug("JWT Auth Policy: Failed to parse PEM certificate",
			"certPath", certPath,
		)
		return nil, fmt.Errorf("failed to parse PEM certificate from %s", certPath)
	}

	slog.Debug("JWT Auth Policy: TLS config created successfully",
		"certPath", certPath,
	)

	return &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}, nil
}

// loadPublicKeyFromCertificate loads an RSA public key from a certificate file
func loadPublicKeyFromCertificate(certPath string) (*rsa.PublicKey, error) {
	slog.Debug("JWT Auth Policy: loadPublicKeyFromCertificate called",
		"certPath", certPath,
	)

	certData, err := os.ReadFile(certPath)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to read certificate file for public key",
			"certPath", certPath,
			"error", err,
		)
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	slog.Debug("JWT Auth Policy: Certificate file read for public key extraction",
		"certPath", certPath,
		"dataLength", len(certData),
	)

	return parsePublicKeyFromString(string(certData))
}

// parsePublicKeyFromString parses an RSA public key from a PEM-encoded string
func parsePublicKeyFromString(pemData string) (*rsa.PublicKey, error) {
	slog.Debug("JWT Auth Policy: parsePublicKeyFromString called",
		"dataLength", len(pemData),
	)

	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		slog.Debug("JWT Auth Policy: Failed to decode PEM block")
		return nil, fmt.Errorf("failed to decode PEM block from certificate data")
	}

	slog.Debug("JWT Auth Policy: PEM block decoded successfully",
		"blockType", block.Type,
	)

	// Try to parse as a certificate first
	cert, err := x509.ParseCertificate(block.Bytes)
	if err == nil {
		slog.Debug("JWT Auth Policy: Parsed as X.509 certificate",
			"subject", cert.Subject.String(),
			"issuer", cert.Issuer.String(),
		)
		// Extract public key from certificate
		if publicKey, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			slog.Debug("JWT Auth Policy: Extracted RSA public key from certificate")
			return publicKey, nil
		}
		slog.Debug("JWT Auth Policy: Certificate does not contain RSA public key")
		return nil, fmt.Errorf("certificate does not contain an RSA public key")
	}

	slog.Debug("JWT Auth Policy: Not a certificate, trying to parse as public key directly",
		"parseError", err,
	)

	// If certificate parsing fails, try to parse as a public key directly
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		slog.Debug("JWT Auth Policy: Failed to parse as PKIX public key",
			"error", err,
		)
		return nil, fmt.Errorf("failed to parse public key from certificate data: %w", err)
	}

	if rsaPublicKey, ok := publicKey.(*rsa.PublicKey); ok {
		slog.Debug("JWT Auth Policy: Parsed PKIX RSA public key successfully")
		return rsaPublicKey, nil
	}

	slog.Debug("JWT Auth Policy: Parsed key is not RSA")
	return nil, fmt.Errorf("certificate data does not contain an RSA public key")
}
