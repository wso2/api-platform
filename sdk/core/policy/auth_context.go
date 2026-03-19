package policy

// AuthContext contains structured authentication information populated by auth policies.
// It replaces the unstructured map[string]string previously used for inter-policy
// communication about authentication results.
type AuthContext struct {
	// Authenticated indicates whether the request was successfully authenticated.
	Authenticated bool

	// Authorized indicates the request passed an authorization check.
	// Set by authorization policies (e.g., mcp-authz); always false for authentication-only policies.
	Authorized bool

	// AuthType identifies the authentication mechanism used.
	// Common values: "jwt", "basic", "apikey".
	// MCP convention: "mcp/oauth" for MCP OAuth authentication; "mcp/oauth+authz" after MCP authorization passes.
	AuthType string

	// Subject is the principal identity — JWT "sub" claim, basic-auth username,
	// or API key owner.
	Subject string

	// Issuer is the JWT "iss" claim. Empty for basic auth and API key auth.
	Issuer string

	// Audience holds the JWT "aud" claim values. Nil for basic auth and API key auth.
	Audience []string

	// Scopes holds the granted OAuth2/JWT scopes. The map key is the scope name
	// and the value is always true. Nil for basic auth and API key auth.
	Scopes map[string]bool

	// CredentialID is an opaque identifier for the credential used — for example,
	// an API key application ID or an OAuth2 client_id. Empty if not applicable.
	CredentialID string

	// Properties holds additional claims or data that do not fit into the typed
	// fields above (e.g., custom JWT claims).
	Properties map[string]string

	// Previous points to the AuthContext set by an earlier auth policy in a
	// multi-layer auth chain. Nil if this is the only auth layer.
	Previous *AuthContext
}
