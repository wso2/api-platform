package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// resourceRoles holds the mapping of resource -> allowed local roles.
// Keys may be either "METHOD /path" (preferred) or just "/path".
// Edit this map to define permissions; an undefined resource will be rejected.
var resourceRoles = map[string][]string{
	// Controller API resources mapped to default allowed roles.
	// Most management operations are admin-only by default.
	"POST /apis":       {"admin"},
	"GET /apis":        {"admin"},
	"GET /apis/:id":    {"admin"},
	"PUT /apis/:id":    {"admin"},
	"DELETE /apis/:id": {"admin"},

	"GET /certificates":         {"admin"},
	"POST /certificates":        {"admin"},
	"DELETE /certificates/:id":  {"admin"},
	"POST /certificates/reload": {"admin"},

	"GET /policies": {"admin"},

	"POST /mcp-proxies":       {"admin"},
	"GET /mcp-proxies":        {"admin"},
	"GET /mcp-proxies/:id":    {"admin"},
	"PUT /mcp-proxies/:id":    {"admin"},
	"DELETE /mcp-proxies/:id": {"admin"},

	"POST /llm-provider-templates":         {"admin"},
	"GET /llm-provider-templates":          {"admin"},
	"GET /llm-provider-templates/:name":    {"admin"},
	"PUT /llm-provider-templates/:name":    {"admin"},
	"DELETE /llm-provider-templates/:name": {"admin"},

	"POST /llm-providers":                  {"admin"},
	"GET /llm-providers":                   {"admin"},
	"GET /llm-providers/:name/:version":    {"admin"},
	"PUT /llm-providers/:name/:version":    {"admin"},
	"DELETE /llm-providers/:name/:version": {"admin"},

	"GET /config_dump": {"admin"},
}

// AuthorizationMiddleware enforces resource->roles mapping stored in this package.
func AuthorizationMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow unauthenticated health endpoint
		if strings.HasPrefix(c.Request.URL.Path, "/health") {
			c.Next()
			return
		}

		// If no mapping configured, reject all requests to be safe
		if resourceRoles == nil || len(resourceRoles) == 0 {
			logger.Debug("authorization: no resourceRoles configured; rejecting request", zap.String("path", c.Request.URL.Path), zap.String("method", c.Request.Method))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// Retrieve user roles from context (set by auth middleware)
		var userRoles []string
		if v, ok := c.Get(AuthRolesKey); ok {
			if ur, ok2 := v.([]string); ok2 {
				userRoles = ur
			}
		}

		if len(userRoles) == 0 {
			logger.Debug("authorization: no user roles found in context; rejecting request", zap.String("path", c.Request.URL.Path))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// Determine resource key
		resourcePath := c.FullPath()
		if resourcePath == "" {
			// FullPath may be empty for some middleware ordering; fallback to raw path
			resourcePath = c.Request.URL.Path
		}

		// Try METHOD + path first
		methodKey := c.Request.Method + " " + resourcePath

		allowed, found := resourceRoles[methodKey]
		if !found {
			// Try path-only key
			allowed, found = resourceRoles[resourcePath]
		}

		if !found {
			// Resource not defined -> reject
			logger.Debug("authorization: resource not defined in resourceRoles", zap.String("resource", resourcePath), zap.String("method", c.Request.Method))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
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
		logger.Debug("authorization: user roles do not include allowed roles", zap.Strings("user_roles", userRoles), zap.Strings("allowed_roles", allowed))
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}
