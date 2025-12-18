package authenticators

import (
	"errors"

	"github.com/gin-gonic/gin"
)

var (
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrNoAuthenticators     = errors.New("no authenticators registered")
)

// Credentials represents generic authentication credentials
type Credentials interface{}

// AuthResult contains the result of an authentication attempt
type AuthResult struct {
	Success      bool
	UserID       string
	Roles        []string
	Claims       map[string]interface{}
	ErrorMessage string
}

// Authenticator defines the interface for authentication methods
type Authenticator interface {
	// Authenticate verifies the provided credentials
	Authenticate(ctx *gin.Context) (*AuthResult, error)

	// Name returns the name of the authenticator
	Name() string

	// CanHandle checks if this authenticator can handle the given credentials
	CanHandle(ctx *gin.Context) bool
}
