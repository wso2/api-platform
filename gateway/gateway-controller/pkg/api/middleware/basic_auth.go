package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// BasicAuthMiddleware implements HTTP Basic auth against locally configured users.
// It supports plain-text passwords and bcrypt-hashed passwords (when PasswordHashed is true).
func BasicAuthMiddleware(cfg *config.Config, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If auth is not configured or no users are defined, skip auth
		if len(cfg.GatewayController.Auth.Users) == 0 {
			c.Next()
			return
		}

		// Extract basic auth credentials
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			logger.Debug("missing basic auth header")
			c.Header("WWW-Authenticate", `Basic realm="Restricted"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Find user in config
		var matched *config.AuthUser
		for i := range cfg.GatewayController.Auth.Users {
			u := &cfg.GatewayController.Auth.Users[i]
			if strings.EqualFold(u.Username, username) {
				matched = u
				break
			}
		}

		if matched == nil {
			logger.Debug("unknown user", zap.String("user", username))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Validate password
		if matched.PasswordHashed {
			// bcrypt
			if err := bcrypt.CompareHashAndPassword([]byte(matched.Password), []byte(password)); err != nil {
				logger.Debug("password mismatch (bcrypt)", zap.String("user", username))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		} else {
			if matched.Password != password {
				logger.Debug("password mismatch", zap.String("user", username))
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}

		// Attach user info to context
		c.Set(AuthUserKey, matched.Username)
		c.Set(AuthRolesKey, matched.Roles)

		// Add to logger for downstream handlers
		log := GetLogger(c, logger).With(zap.String("auth_user", matched.Username))
		c.Set(LoggerKey, log)

		c.Next()
	}
}
