package vectordbproviders

import (
	"fmt"
	"regexp"
	"time"
)

// CacheResponse holds everything needed to cache an HTTP response.
type CacheResponse struct {
	ResponsePayload     map[string]interface{}
	RequestHash         string
	Timeout             time.Duration
	HeaderProperties    map[string]interface{}
	StatusCode          string
	StatusReason        string
	JSON                bool
	ProtocolType        string
	HTTPMethod          string
	ResponseCodePattern string
	responseCodeRegex   *regexp.Regexp
	ResponseFetchedTime time.Time
	CacheControlEnabled bool
	AddAgeHeaderEnabled bool
}

// NewCacheResponse creates a new CacheResponse with optional initialization of responseCodeRegex.
// If responseCodePattern is provided and non-empty, it will be compiled into responseCodeRegex.
func NewCacheResponse(responseCodePattern string) (*CacheResponse, error) {
	c := &CacheResponse{
		ResponseCodePattern: responseCodePattern,
	}

	if responseCodePattern != "" {
		if err := c.InitResponseCodeRegex(); err != nil {
			return nil, fmt.Errorf("failed to initialize response code regex: %w", err)
		}
	}

	return c, nil
}

// InitResponseCodeRegex compiles ResponseCodePattern into responseCodeRegex.
// This method should be called if ResponseCodePattern is set after struct initialization.
// Returns an error if the pattern is invalid.
func (c *CacheResponse) InitResponseCodeRegex() error {
	if c.ResponseCodePattern == "" {
		c.responseCodeRegex = nil
		return nil
	}

	compiled, err := regexp.Compile(c.ResponseCodePattern)
	if err != nil {
		return fmt.Errorf("invalid response code pattern '%s': %w", c.ResponseCodePattern, err)
	}

	c.responseCodeRegex = compiled
	return nil
}

// GetResponseCodeRegex returns the compiled regex for ResponseCodePattern.
// If the regex hasn't been initialized and ResponseCodePattern is set, it will be compiled lazily.
// Returns nil if ResponseCodePattern is empty or if compilation failed.
func (c *CacheResponse) GetResponseCodeRegex() *regexp.Regexp {
	if c.responseCodeRegex != nil {
		return c.responseCodeRegex
	}

	if c.ResponseCodePattern != "" {
		// Lazy initialization - try to compile if not already done
		if err := c.InitResponseCodeRegex(); err != nil {
			// If compilation fails, return nil to avoid panics
			return nil
		}
		return c.responseCodeRegex
	}

	return nil
}

// Clean resets payload and headers to free up memory.
func (c *CacheResponse) Clean() {
	c.ResponsePayload = nil
	c.HeaderProperties = nil
}
