package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Authorization is driven by resource->required scopes (resourceScopes)
// and role->scopes (roleScopes). The middleware validates that the request
// either carries scopes (AuthScopesKey) or a role (AuthRolesKey) that maps
// to scopes which satisfy the resource's required scopes.

// resourceScopes maps a resource (METHOD path or path) to the required scopes
// a caller must present (directly as scopes or indirectly via role->scopes).
var resourceScopes = map[string][]string{
	// APIs
	"POST /apis":       {"apis.manage"},
	"GET /apis":        {"apis.read"},
	"GET /apis/:id":    {"apis.read"},
	"PUT /apis/:id":    {"apis.manage"},
	"DELETE /apis/:id": {"apis.manage"},

	// Certificates
	"GET /certificates":         {"certificates.read"},
	"POST /certificates":        {"certificates.manage"},
	"DELETE /certificates/:id":  {"certificates.manage"},
	"POST /certificates/reload": {"certificates.manage"},

	// Policies
	"GET /policies": {"policies.read"},

	// MCP Proxies
	"POST /mcp-proxies":       {"mcp.manage"},
	"GET /mcp-proxies":        {"mcp.read"},
	"GET /mcp-proxies/:id":    {"mcp.read"},
	"PUT /mcp-proxies/:id":    {"mcp.manage"},
	"DELETE /mcp-proxies/:id": {"mcp.manage"},
	// LLM provider templates
	"POST /llm-provider-templates":         {"llm.templates.manage"},
	"GET /llm-provider-templates":          {"llm.templates.read"},
	"GET /llm-provider-templates/:name":    {"llm.templates.read"},
	"PUT /llm-provider-templates/:name":    {"llm.templates.manage"},
	"DELETE /llm-provider-templates/:name": {"llm.templates.manage"},

	// LLM providers
	"POST /llm-providers":                  {"llm.providers.manage"},
	"GET /llm-providers":                   {"llm.providers.read"},
	"GET /llm-providers/:name/:version":    {"llm.providers.read"},
	"PUT /llm-providers/:name/:version":    {"llm.providers.manage"},
	"DELETE /llm-providers/:name/:version": {"llm.providers.manage"},

	// Config dump
	"GET /config_dump": {"system.config_dump"},
}

// roleScopes defines which scopes each local role grants. By default admin
// is granted all controller management scopes. Add or adjust scopes as needed
// to grant finer-grained permissions to developer/consumer roles.
var roleScopes = map[string][]string{
	"admin": {
		// APIs
		"apis.manage", "apis.read",
		// Certificates
		"certificates.manage", "certificates.read",
		// Policies
		"policies.read",
		// MCP
		"mcp.manage", "mcp.read",
		// LLM provider templates
		"llm.templates.manage", "llm.templates.read",
		// LLM providers
		"llm.providers.manage", "llm.providers.read",
		// Config
		"system.config_dump",
	},
	// Developer: limited management for APIs and read access to related resources.
	"developer": {
		"apis.manage", "apis.read",
		"mcp.read", "mcp.manage",
		"llm.templates.read", "llm.providers.read",
		"policies.read",
	},
	// Consumer: no default scopes assigned; populate when needed.
	"consumer": {},
}

// AuthorizationMiddleware enforces resource->required-scopes mapping and
// role->scopes expansion stored in this package.
func AuthorizationMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// If upstream auth marked this request to skip, honor it
		if v, ok := c.Get(AuthSkipKey); ok {
			if skip, ok2 := v.(bool); ok2 && skip {
				c.Next()
				return
			}
		}

		// If no resource->scopes mapping configured, reject all requests to be safe
		if resourceScopes == nil || len(resourceScopes) == 0 {
			logger.Debug("authorization: no resourceScopes configured; rejecting request", zap.String("path", c.Request.URL.Path), zap.String("method", c.Request.Method))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// Retrieve either explicit scopes or roles from context.
		// If scopes are present they will be validated directly. Otherwise
		// roles are expanded into scopes via roleScopes.
		var providedScopes []string
		if v, ok := c.Get(AuthScopesKey); ok {
			if ps, ok2 := v.([]string); ok2 {
				providedScopes = ps
			}
		}

		// If no scopes provided, try roles -> scopes expansion
		if len(providedScopes) == 0 {
			if v, ok := c.Get(AuthRolesKey); ok {
				if ur, ok2 := v.([]string); ok2 {
					// gather scopes from each role
					scopeSet := make(map[string]struct{})
					for _, r := range ur {
						if rs, ok := roleScopes[r]; ok {
							for _, s := range rs {
								scopeSet[s] = struct{}{}
							}
						}
					}
					for s := range scopeSet {
						providedScopes = append(providedScopes, s)
					}
				}
			}
		}

		if len(providedScopes) == 0 {
			logger.Debug("authorization: no scopes or roles found in context; rejecting request", zap.String("path", c.Request.URL.Path))
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

		// Determine required scopes for resource
		requiredScopes, found := resourceScopes[methodKey]
		if !found {
			requiredScopes, found = resourceScopes[resourcePath]
		}

		if !found {
			logger.Debug("authorization: resource not defined in resourceScopes", zap.String("resource", resourcePath), zap.String("method", c.Request.Method))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// Validate that all required scopes are present in providedScopes
		providedSet := make(map[string]struct{}, len(providedScopes))
		for _, s := range providedScopes {
			providedSet[s] = struct{}{}
		}

		for _, rs := range requiredScopes {
			if _, ok := providedSet[rs]; !ok {
				logger.Debug("authorization: missing required scope for resource", zap.String("required_scope", rs), zap.Strings("provided_scopes", providedScopes))
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		// All required scopes satisfied
		c.Next()
	}
}
