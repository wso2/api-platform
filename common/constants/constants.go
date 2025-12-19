package constants

const (
	// AuthorizationHeader is the HTTP header for authorization
	AuthorizationHeader = "Authorization"
	// BearerPrefix is the prefix for Bearer tokens in the Authorization header
	BearerPrefix = "Bearer "
	// BasicPrefix is the prefix for Basic auth in the Authorization header
	BasicPrefix = "Basic "
	// AuthSkippedKey is the context key to indicate if authentication was skipped
	AuthSkippedKey = "auth_skipped"
	// AuthenticatedKey is the context key to indicate if the user is authenticated
	AuthenticatedKey = "authenticated"
	// UserIDKey is the context key for the authenticated user's ID
	UserIDKey = "userID"
	// AuthRolesKey is the context key for the authenticated user's roles
	AuthRolesKey = "roles"
	// ClaimsKey is the context key for JWT claims
	ClaimsKey = "claims"
)
