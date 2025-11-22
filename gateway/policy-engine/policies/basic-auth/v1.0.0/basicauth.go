package basicauth

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/policy-engine/sdk/policies"
)

const (
	// Metadata keys for context storage
	MetadataKeyAuthSuccess = "auth.success"
	MetadataKeyAuthUser    = "auth.username"
	MetadataKeyAuthMethod  = "auth.method"
)

// BasicAuthPolicy implements HTTP Basic Authentication
type BasicAuthPolicy struct{}

// NewPolicy creates a new BasicAuthPolicy instance
func NewPolicy() policies.Policy {
	return &BasicAuthPolicy{}
}

// Validate validates the policy configuration
func (p *BasicAuthPolicy) Validate(params map[string]interface{}) error {
	// Validate username parameter (required)
	usernameRaw, ok := params["username"]
	if !ok {
		return fmt.Errorf("'username' parameter is required")
	}
	username, ok := usernameRaw.(string)
	if !ok {
		return fmt.Errorf("'username' must be a string")
	}
	if len(username) == 0 {
		return fmt.Errorf("'username' cannot be empty")
	}

	// Validate password parameter (required)
	passwordRaw, ok := params["password"]
	if !ok {
		return fmt.Errorf("'password' parameter is required")
	}
	password, ok := passwordRaw.(string)
	if !ok {
		return fmt.Errorf("'password' must be a string")
	}
	if len(password) == 0 {
		return fmt.Errorf("'password' cannot be empty")
	}

	// Validate allowUnauthenticated parameter (optional, defaults to false)
	if allowUnauthRaw, ok := params["allowUnauthenticated"]; ok {
		_, ok := allowUnauthRaw.(bool)
		if !ok {
			return fmt.Errorf("'allowUnauthenticated' must be a boolean")
		}
	}

	// Validate realm parameter (optional)
	if realmRaw, ok := params["realm"]; ok {
		realm, ok := realmRaw.(string)
		if !ok {
			return fmt.Errorf("'realm' must be a string")
		}
		if len(realm) == 0 {
			return fmt.Errorf("'realm' cannot be empty")
		}
	}

	return nil
}

// OnRequest performs Basic Authentication
func (p *BasicAuthPolicy) OnRequest(ctx *policies.RequestContext, params map[string]interface{}) policies.RequestAction {
	// Get configuration parameters
	expectedUsername := params["username"].(string)
	expectedPassword := params["password"].(string)

	allowUnauthenticated := false
	if allowUnauthRaw, ok := params["allowUnauthenticated"]; ok {
		allowUnauthenticated = allowUnauthRaw.(bool)
	}

	realm := "Restricted"
	if realmRaw, ok := params["realm"]; ok {
		realm = realmRaw.(string)
	}

	// Extract and validate Authorization header
	authHeaders, ok := ctx.Headers["authorization"]
	if !ok || len(authHeaders) == 0 {
		return p.handleAuthFailure(ctx, allowUnauthenticated, realm, "missing authorization header")
	}

	authHeader := authHeaders[0]

	// Check if it's Basic auth
	if !strings.HasPrefix(authHeader, "Basic ") {
		return p.handleAuthFailure(ctx, allowUnauthenticated, realm, "invalid authorization scheme")
	}

	// Decode base64 credentials
	encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return p.handleAuthFailure(ctx, allowUnauthenticated, realm, "invalid base64 encoding")
	}

	// Parse username:password
	credentials := string(decodedBytes)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return p.handleAuthFailure(ctx, allowUnauthenticated, realm, "invalid credentials format")
	}

	providedUsername := parts[0]
	providedPassword := parts[1]

	// Validate credentials
	if providedUsername != expectedUsername || providedPassword != expectedPassword {
		return p.handleAuthFailure(ctx, allowUnauthenticated, realm, "invalid credentials")
	}

	// Authentication successful
	return p.handleAuthSuccess(ctx, providedUsername)
}

// handleAuthSuccess handles successful authentication
func (p *BasicAuthPolicy) handleAuthSuccess(ctx *policies.RequestContext, username string) policies.RequestAction {
	// Set metadata indicating successful authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = true
	ctx.Metadata[MetadataKeyAuthUser] = username
	ctx.Metadata[MetadataKeyAuthMethod] = "basic"

	// Continue to upstream with no modifications
	return policies.UpstreamRequestModifications{}
}

// handleAuthFailure handles authentication failure
func (p *BasicAuthPolicy) handleAuthFailure(ctx *policies.RequestContext, allowUnauthenticated bool, realm string, reason string) policies.RequestAction {
	// Set metadata indicating failed authentication
	ctx.Metadata[MetadataKeyAuthSuccess] = false
	ctx.Metadata[MetadataKeyAuthMethod] = "basic"

	// If allowUnauthenticated is true, allow request to proceed
	if allowUnauthenticated {
		return policies.UpstreamRequestModifications{}
	}

	// Return 401 Unauthorized response
	headers := map[string]string{
		"www-authenticate": fmt.Sprintf("Basic realm=\"%s\"", realm),
		"content-type":     "application/json",
	}

	body := fmt.Sprintf(`{"error": "Unauthorized", "message": "Authentication required"}`)

	return policies.ImmediateResponse{
		StatusCode: 401,
		Headers:    headers,
		Body:       []byte(body),
	}
}
