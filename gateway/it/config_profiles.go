package it

// ConfigProfile defines a named configuration set for the gateway
type ConfigProfile struct {
	Name        string
	EnvVars     map[string]string
	Description string
}

// ConfigProfileRegistry manages the available configuration profiles
type ConfigProfileRegistry struct {
	profiles       map[string]*ConfigProfile
	defaultProfile string
}

// NewConfigProfileRegistry creates a new registry with standard profiles
func NewConfigProfileRegistry() *ConfigProfileRegistry {
	registry := &ConfigProfileRegistry{
		profiles:       make(map[string]*ConfigProfile),
		defaultProfile: "default",
	}

	// Register standard profiles
	registry.Register(&ConfigProfile{
		Name: "default",
		EnvVars: map[string]string{
			"GATEWAY_LOGGING_LEVEL": "info",
			"GATEWAY_STORAGE_TYPE":  "sqlite",
		},
		Description: "Standard configuration using SQLite and Info logging",
	})

	registry.Register(&ConfigProfile{
		Name: "debug",
		EnvVars: map[string]string{
			"GATEWAY_LOGGING_LEVEL": "debug",
			"GATEWAY_STORAGE_TYPE":  "sqlite",
		},
		Description: "Debug configuration enabling verbose logging",
	})

	registry.Register(&ConfigProfile{
		Name: "memory",
		EnvVars: map[string]string{
			"GATEWAY_LOGGING_LEVEL": "info",
			"GATEWAY_STORAGE_TYPE":  "memory",
		},
		Description: "In-memory storage configuration (non-persistent)",
	})

	registry.Register(&ConfigProfile{
		Name: "tracing",
		EnvVars: map[string]string{
			"GATEWAY_LOGGING_LEVEL":   "info",
			"GATEWAY_STORAGE_TYPE":    "memory",
			"GATEWAY_TRACING_ENABLED": "true",
		},
		Description: "Configuration with OpenTelemetry tracing enabled",
	})

	return registry

}

// Register adds a profile to the registry
func (r *ConfigProfileRegistry) Register(profile *ConfigProfile) {
	r.profiles[profile.Name] = profile
}

// Get retrieves a profile by name
func (r *ConfigProfileRegistry) Get(name string) (*ConfigProfile, bool) {
	profile, ok := r.profiles[name]
	return profile, ok
}
