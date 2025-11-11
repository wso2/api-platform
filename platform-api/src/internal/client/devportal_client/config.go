package devportal_client

import "time"

// DevPortalConfig contains per-DevPortal configuration used to create clients
type DevPortalConfig struct {
	BaseURL    string        // full base URL including scheme, e.g. https://devportal.example
	APIKey     string        // API key or token to use in requests
	HeaderName string        // header name for API key (defaults to x-wso2-api-key if empty)
	Timeout    time.Duration // per-request timeout
	MaxRetries int           // max retry attempts for transient errors
	// Future fields: TLSConfig, ProxyURL, TokenProvider, etc.
}

// DefaultHeaderName is used when no header name is provided
const DefaultHeaderName = "x-wso2-api-key"

// DefaultTimeout is the default client timeout
const DefaultTimeout = 10 * time.Second

// DefaultMaxRetries is the default retry attempts for transient errors
const DefaultMaxRetries = 3
