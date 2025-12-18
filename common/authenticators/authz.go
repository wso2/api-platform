package authenticators

import (
	"net/http"

	"github.com/gin-gonic/gin"
	commonerrors "github.com/wso2/api-platform/common/errors"
	"github.com/wso2/api-platform/common/models"
	"go.uber.org/zap"
)

var (
	AuthRolesKey = "roles"
)

// AuthorizationMiddleware enforces resource->roles mapping stored in this package.
func AuthorizationMiddleware(config models.AuthConfig, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authorization if authentication was skipped
		if v, ok := c.Get("auth_skipped"); ok {
			if skipped, ok2 := v.(bool); ok2 && skipped {
				c.Next()
				return
			}
		}

		// Use config.ResourceRoles if provided, else fallback to DefaultResourceRoles
		resourceRoles := config.ResourceRoles
		logger.Sugar().Infof("Resource roles %v", resourceRoles)
		if len(resourceRoles) == 0 {
			c.Next()
			return
		}
		// Retrieve user roles from context (set by auth middleware)
		var userRoles []string
		if v, ok := c.Get(AuthRolesKey); ok {
			if ur, ok2 := v.([]string); ok2 {
				userRoles = ur
			}
		}
		logger.Sugar().Infof("User roles %v", userRoles)

		// Determine resource key
		resourcePath := c.FullPath()
		if resourcePath == "" {
			// FullPath may be empty for some middleware ordering; fallback to raw path
			resourcePath = c.Request.URL.Path
		}

		// Try METHOD + path first
		methodKey := c.Request.Method + " " + resourcePath
		logger.Sugar().Infof("method key %v", methodKey)
		allowed, found := resourceRoles[methodKey]
		if !found {
			// Resource not defined -> reject
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": commonerrors.ErrForbidden.Error()})
			return
		}

		// Check for role intersection
		allowedSet := make(map[string]struct{}, len(allowed))
		for _, r := range allowed {
			allowedSet[r] = struct{}{}
		}

		for _, ur := range userRoles {
			if _, ok := allowedSet[ur]; ok {
				c.Next()
				return
			}
		}

		// No matching role -> forbidden
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": commonerrors.ErrForbidden.Error()})
	}
}
