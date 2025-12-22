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
