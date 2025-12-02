package admin

import "time"

// ConfigDumpResponse is the top-level response structure for the config_dump endpoint
type ConfigDumpResponse struct {
	Timestamp      time.Time             `json:"timestamp"`
	PolicyRegistry PolicyRegistryDump    `json:"policy_registry"`
	Routes         RoutesDump            `json:"routes"`
}

// PolicyRegistryDump contains information about all registered policies
type PolicyRegistryDump struct {
	TotalPolicies int                  `json:"total_policies"`
	Policies      []PolicyInfo         `json:"policies"`
}

// PolicyInfo contains information about a single policy
type PolicyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// RoutesDump contains information about all configured routes
type RoutesDump struct {
	TotalRoutes  int           `json:"total_routes"`
	RouteConfigs []RouteConfig `json:"route_configs"`
}

// RouteConfig contains configuration for a single route
type RouteConfig struct {
	RouteKey             string       `json:"route_key"`
	RequiresRequestBody  bool         `json:"requires_request_body"`
	RequiresResponseBody bool         `json:"requires_response_body"`
	TotalPolicies        int          `json:"total_policies"`
	Policies             []PolicySpec `json:"policies"`
}

// PolicySpec contains specification for a policy instance
type PolicySpec struct {
	Name               string                 `json:"name"`
	Version            string                 `json:"version"`
	Enabled            bool                   `json:"enabled"`
	ExecutionCondition *string                `json:"execution_condition"`
	Parameters         map[string]interface{} `json:"parameters"`
}
