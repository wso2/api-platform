package authenticators

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

var (
	ErrNoAuthenticator = errors.New("no suitable authenticator found")
)

// AuthMiddleware creates a unified authentication middleware supporting both Basic and Bearer auth
func AuthMiddleware(config models.AuthConfig, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for specified paths
		for _, path := range config.SkipPaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Set("auth_skipped", true)
				c.Next()
				return
			}
		}

		// Initialize authenticators
		authenticators := []Authenticator{}

		// Add Basic authenticator if configured
		if config.BasicAuth != nil && len(config.BasicAuth.Users) > 0 {
			authenticators = append(authenticators, NewBasicAuthenticator(config, logger))
		}

		// Add JWT authenticator if configured
		if config.JWTConfig != nil && config.JWTConfig.IssuerURL != "" {
			authenticators = append(authenticators, NewJWTAuthenticator(&config, logger))
		}

		// Find suitable authenticator
		var selectedAuth Authenticator
		for _, auth := range authenticators {
			if auth.CanHandle(c) {
				selectedAuth = auth
				break
			}
		}

		if selectedAuth == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "no valid authentication credentials provided",
			})
			c.Abort()
			return
		}

		// Authenticate
		result, err := selectedAuth.Authenticate(c)
		if err != nil {
			logger.Sugar().Errorf("Authentication error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication failed",
			})
			c.Abort()
			return
		}
		logger.Sugar().Infof("Authentication result %v", result)
		logger.Sugar().Infof("Authentication roles %v", result.Roles)
		// Set authentication context
		c.Set("authenticated", result.Success)
		c.Set("userID", result.UserID)
		c.Set("roles", result.Roles)
		if result.Claims != nil {
			c.Set("claims", result.Claims)
		}

		c.Next()
	}
}
