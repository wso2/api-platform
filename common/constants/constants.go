package constants

const (
	// AuthorizationHeader is the HTTP header for authorization
	AuthorizationHeader = "Authorization"
	// BearerPrefix is the prefix for Bearer tokens in the Authorization header
	BearerPrefix = "Bearer "
	// BasicPrefix is the prefix for Basic auth in the Authorization header
	BasicPrefix = "Basic "
	// AuthzSkipKey is the context key to indicate to skip
	AuthzSkipKey = "skip_authz"
	// AuthContextKey is the context key for packed authentication context
	AuthContextKey = "auth_context"
)

const (
	// Server Header manipulation strategies for HCM Listener
	APPEND_IF_ABSENT = "APPEND_IF_ABSENT"
	OVERWRITE        = "OVERWRITE"
	PASS_THROUGH     = "PASS_THROUGH"

	// ServerName for HCM listener in Gateway
	ServerName = "WSO2 API Platform"
)
