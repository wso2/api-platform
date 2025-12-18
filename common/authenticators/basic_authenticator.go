package authenticators

import (
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/models"
	bcrypt "golang.org/x/crypto/bcrypt"
)

// BasicAuthenticator implements HTTP Basic Authentication
type BasicAuthenticator struct {
	authConfig models.AuthConfig
}

// NewBasicAuthenticator creates a new BasicAuthenticator
func NewBasicAuthenticator(authConfig models.AuthConfig) *BasicAuthenticator {
	return &BasicAuthenticator{
		authConfig: authConfig,
	}
}

// Authenticate verifies basic authentication credentials from context
func (b *BasicAuthenticator) Authenticate(c *gin.Context) (*AuthResult, error) {
	authHeader := c.GetHeader(string("Authorization"))
	if authHeader == "" {
		return nil, ErrAuthenticationFailed
	}
	// Extract and decode credentials
	encodedCredentials := strings.TrimPrefix(authHeader, "Basic ")
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCredentials)
	if err != nil {
		return nil, ErrAuthenticationFailed
	}
	credentials := string(decodedBytes)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return nil, ErrAuthenticationFailed
	}
	username := parts[0]
	password := parts[1]

	// Validate credentials against configured users
	for _, user := range b.authConfig.BasicAuth.Users {
		if user.UserID == username {
			// Check password
			if user.PasswordHashed {
				err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
				if err != nil {
					return nil, ErrAuthenticationFailed
				}
				return &AuthResult{
					Success: true,
					UserID:  user.UserID,
					Roles:   user.Roles,
				}, nil
			} else {
				if user.Password != password {
					return nil, ErrAuthenticationFailed
				} else {

					return &AuthResult{
						Success: true,
						UserID:  user.UserID,
						Roles:   user.Roles,
					}, nil
				}
			}
		}
	}
	return nil, ErrAuthenticationFailed
}

// Name returns the authenticator name
func (b *BasicAuthenticator) Name() string {
	return "BasicAuthenticator"
}

// CanHandle checks if credentials in context are BasicCredentials
func (b *BasicAuthenticator) CanHandle(c *gin.Context) bool {

	authHeader := c.GetHeader(string("Authorization"))
	if authHeader == "" {
		return false
	}
	// Determine auth type from header
	return strings.HasPrefix(authHeader, "Basic ")
}
