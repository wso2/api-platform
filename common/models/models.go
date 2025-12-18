package models

// AuthConfig holds configuration for the authentication middleware
type AuthConfig struct {
	// Basic Auth Configuration
	BasicAuth *BasicAuth

	// JWT/Bearer Auth Configuration
	JWTConfig *IDPConfig

	// Paths to skip authentication
	SkipPaths []string

	// Allow either basic or bearer (if true), require both (if false and both configured)
	AllowEither bool
}

type BasicAuth struct {
	Enabled bool   `json:"enabled"`
	Users   []User `json:"users"`
}

// User represents a user in the system
type User struct {
	UserID         string   `json:"user_id"`
	Password       string   `json:"password"`
	PasswordHashed bool     `json:"password_hashed"`
	Roles          []string `json:"roles"`
}

// IDPConfig holds identity provider configuration
type IDPConfig struct {
	IssuerURL         string               `json:"issuer_url"`
	JWKSUrl           string               `json:"jwks_url"`
	ScopeClaim        string               `json:"scope_claim"`
	UsernameClaim     string               `json:"username_claim"`
	Audience          *string              `json:"audience"`
	Certificate       *string              `json:"certificate"`
	ClaimMapping      *map[string]string   `json:"claim_mapping"`
	PermissionMapping *map[string][]string `json:"permission_mapping"`
}
