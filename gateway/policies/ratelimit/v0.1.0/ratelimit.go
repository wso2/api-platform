package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/policy-engine/policies/ratelimit/algorithms/fixedwindow" // Register Fixed Window algorithm
	_ "github.com/policy-engine/policies/ratelimit/algorithms/gcra"        // Register GCRA algorithm
	"github.com/policy-engine/policies/ratelimit/limiter"
	"github.com/redis/go-redis/v9"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// memoryLimiterCache caches memory-backed limiters to preserve state across xDS rebuilds.
// Only memory backend limiters are cached; Redis-backed limiters maintain state externally.
var memoryLimiterCache sync.Map // map[string]limiter.Limiter

// KeyComponent represents a single component for building rate limit keys
type KeyComponent struct {
	Type string // "header", "metadata", "ip", "apiname", "apiversion", "routename"
	Key  string // header name or metadata key (required for header/metadata)
}

// LimitConfig holds parsed rate limit configuration
type LimitConfig struct {
	Limit    int64
	Duration time.Duration
	Burst    int64
}

// RateLimitPolicy defines the policy for rate limiting
type RateLimitPolicy struct {
	keyExtraction  []KeyComponent
	routeName      string // From metadata, used as default key
	apiId          string // From metadata, API identifier
	apiName        string // From metadata, API name for scope-based caching
	apiVersion     string // From metadata, API version
	statusCode     int
	responseBody   string
	responseFormat string
	backend        string
	limiter        limiter.Limiter
	redisClient    *redis.Client
	redisFailOpen  bool
	includeXRL     bool
	includeIETF    bool
	includeRetry   bool
	// Cost extraction for post-response rate limiting
	costExtractor         *CostExtractor
	costExtractionEnabled bool
}

// GetPolicy creates and initializes a rate limit policy instance
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	// Store route name for default key
	routeName := metadata.RouteName
	if routeName == "" {
		routeName = "unknown-route"
	}

	// Extract API metadata for scope-based caching
	apiId := metadata.APIId
	apiName := metadata.APIName
	apiVersion := metadata.APIVersion

	// 1. Parse user parameters
	limits, err := parseLimits(params["limits"])
	if err != nil {
		return nil, fmt.Errorf("invalid limits: %w", err)
	}

	// Parse keyExtraction (optional, defaults to route name)
	keyExtraction, err := parseKeyExtraction(params["keyExtraction"])
	if err != nil {
		return nil, fmt.Errorf("invalid keyExtraction: %w", err)
	}
	if len(keyExtraction) == 0 {
		// Default to route name
		keyExtraction = []KeyComponent{{Type: "routename"}}
	}

	// Parse onRateLimitExceeded (optional)
	statusCode := 429
	responseBody := `{"error": "Too Many Requests", "message": "Rate limit exceeded. Please try again later."}`
	responseFormat := "json"
	if exceeded, ok := params["onRateLimitExceeded"].(map[string]interface{}); ok {
		if sc, ok := exceeded["statusCode"].(float64); ok {
			statusCode = int(sc)
		}
		if body, ok := exceeded["body"].(string); ok {
			responseBody = body
		}
		if format, ok := exceeded["bodyFormat"].(string); ok {
			responseFormat = format
		}
	}

	// 2. Parse system parameters
	algorithm := getStringParam(params, "algorithm", "gcra")
	backend := getStringParam(params, "backend", "memory")

	// Header configuration
	includeXRL := getBoolParam(params, "headers.includeXRateLimit", true)
	includeIETF := getBoolParam(params, "headers.includeIETF", true)
	includeRetry := getBoolParam(params, "headers.includeRetryAfter", true)

	// Parse cost extraction configuration
	costExtractionConfig, err := parseCostExtractionConfig(params)
	if err != nil {
		return nil, fmt.Errorf("invalid costExtraction config: %w", err)
	}
	var costExtractor *CostExtractor
	costExtractionEnabled := false
	if costExtractionConfig != nil && costExtractionConfig.Enabled {
		costExtractor = NewCostExtractor(*costExtractionConfig)
		costExtractionEnabled = true
	}

	// 3. Initialize limiter based on backend
	var rlLimiter limiter.Limiter
	var redisClient *redis.Client
	redisFailOpen := true

	if backend == "redis" {
		// Parse Redis configuration
		redisHost := getStringParam(params, "redis.host", "localhost")
		redisPort := getIntParam(params, "redis.port", 6379)
		redisPassword := getStringParam(params, "redis.password", "")
		redisUsername := getStringParam(params, "redis.username", "")
		redisDB := getIntParam(params, "redis.db", 0)
		keyPrefix := getStringParam(params, "redis.keyPrefix", "ratelimit:v1:")
		failureMode := getStringParam(params, "redis.failureMode", "open")
		redisFailOpen = (failureMode == "open")

		connTimeout := getDurationParam(params, "redis.connectionTimeout", 5*time.Second)
		readTimeout := getDurationParam(params, "redis.readTimeout", 3*time.Second)
		writeTimeout := getDurationParam(params, "redis.writeTimeout", 3*time.Second)

		// Create Redis client
		redisClient = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%d", redisHost, redisPort),
			Username:     redisUsername,
			Password:     redisPassword,
			DB:           redisDB,
			DialTimeout:  connTimeout,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		})

		// Test connection (fail-fast if configured to fail closed)
		ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			if !redisFailOpen {
				return nil, fmt.Errorf("redis connection failed and failureMode=closed: %w", err)
			}
			slog.Warn("Redis connection failed but failureMode=open", "error", err)
		}

		// Convert limits to limiter.LimitConfig
		limiterLimits := make([]limiter.LimitConfig, len(limits))
		for i, lim := range limits {
			limiterLimits[i] = limiter.LimitConfig{
				Limit:    lim.Limit,
				Duration: lim.Duration,
				Burst:    lim.Burst,
			}
		}

		// Create limiter using factory pattern
		rlLimiter, err = limiter.CreateLimiter(limiter.Config{
			Algorithm:       algorithm,
			Limits:          limiterLimits,
			Backend:         backend,
			RedisClient:     redisClient,
			KeyPrefix:       keyPrefix,
			CleanupInterval: 0, // Not used for Redis
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis limiter: %w", err)
		}
	} else {
		// Memory backend - check cache first to preserve state across xDS rebuilds
		cacheKey := getCacheKey(routeName, apiName, keyExtraction, params)

		// Try to get cached limiter
		if cached, ok := memoryLimiterCache.Load(cacheKey); ok {
			rlLimiter = cached.(limiter.Limiter)
			slog.Debug("Reusing cached memory limiter", "route", routeName, "apiName", apiName, "cacheKey", cacheKey[:16])
		} else {
			// Create new limiter
			cleanupInterval := getDurationParam(params, "memory.cleanupInterval", 5*time.Minute)

			// Convert limits to limiter.LimitConfig
			limiterLimits := make([]limiter.LimitConfig, len(limits))
			for i, lim := range limits {
				limiterLimits[i] = limiter.LimitConfig{
					Limit:    lim.Limit,
					Duration: lim.Duration,
					Burst:    lim.Burst,
				}
			}

			// Create limiter using factory pattern
			rlLimiter, err = limiter.CreateLimiter(limiter.Config{
				Algorithm:       algorithm,
				Limits:          limiterLimits,
				Backend:         backend,
				CleanupInterval: cleanupInterval,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create memory limiter: %w", err)
			}

			// Store in cache
			memoryLimiterCache.Store(cacheKey, rlLimiter)
			slog.Debug("Created and cached new memory limiter", "route", routeName, "apiName", apiName, "cacheKey", cacheKey[:16])
		}
	}

	// 4. Return configured policy instance
	return &RateLimitPolicy{
		keyExtraction:         keyExtraction,
		routeName:             routeName,
		apiId:                 apiId,
		apiName:               apiName,
		apiVersion:            apiVersion,
		statusCode:            statusCode,
		responseBody:          responseBody,
		responseFormat:        responseFormat,
		backend:               backend,
		limiter:               rlLimiter,
		redisClient:           redisClient,
		redisFailOpen:         redisFailOpen,
		includeXRL:            includeXRL,
		includeIETF:           includeIETF,
		includeRetry:          includeRetry,
		costExtractor:         costExtractor,
		costExtractionEnabled: costExtractionEnabled,
	}, nil
}

// Metadata keys for storing data across request/response phases
const (
	rateLimitResultKey = "ratelimit:result"
	rateLimitKeyKey    = "ratelimit:key" // Store extracted key for post-response cost extraction
)

// Mode returns the processing mode for this policy
func (p *RateLimitPolicy) Mode() policy.ProcessingMode {
	responseBodyMode := policy.BodyModeSkip
	// If cost extraction is enabled and requires response body, buffer it
	if p.costExtractionEnabled && p.costExtractor != nil && p.costExtractor.RequiresResponseBody() {
		responseBodyMode = policy.BodyModeBuffer
	}

	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Need headers for key extraction
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need body
		ResponseHeaderMode: policy.HeaderModeProcess, // Need to add rate limit headers to response
		ResponseBodyMode:   responseBodyMode,         // Buffer if cost extraction from body is configured
	}
}

// OnRequest performs rate limit check
func (p *RateLimitPolicy) OnRequest(
	ctx *policy.RequestContext,
	params map[string]interface{},
) policy.RequestAction {
	// 1. Extract rate limit key
	key := p.extractRateLimitKey(ctx)

	// 2. If cost extraction is enabled, check if quota is already exhausted before sending to upstream
	// We use AllowN with n=0 to peek at remaining quota without consuming tokens
	if p.costExtractionEnabled {
		ctx.Metadata[rateLimitKeyKey] = key

		// Pre-check: if remaining quota is already <= 0, block the request
		result, err := p.limiter.AllowN(context.Background(), key, 0)
		if err != nil {
			if p.backend == "redis" && p.redisFailOpen {
				slog.Warn("Rate limit pre-check failed (fail-open)", "error", err, "key", key)
				return policy.UpstreamRequestModifications{}
			}
			slog.Error("Rate limit pre-check failed (fail-closed)", "error", err, "key", key)
			return p.buildRateLimitResponse(nil)
		}

		// If remaining <= 0, quota is exhausted - block the request
		if result != nil && result.Remaining <= 0 {
			slog.Debug("Cost extraction mode: quota exhausted, blocking request", "key", key, "remaining", result.Remaining)
			return p.buildRateLimitResponse(result)
		}

		slog.Debug("Cost extraction enabled, deferring cost consumption to response phase", "key", key, "remaining", result.Remaining)
		return policy.UpstreamRequestModifications{}
	}

	// 3. Extract cost parameter (defaults to 1 for backwards compatibility)
	cost := int64(1)
	if costVal, ok := params["cost"].(float64); ok {
		cost = int64(costVal)
		if cost < 1 {
			cost = 1 // Ensure minimum cost of 1
		}
	}

	// 4. Check rate limit with cost (weighted rate limiting)
	result, err := p.limiter.AllowN(context.Background(), key, cost)

	// 5. Handle errors (Redis failures, etc.)
	if err != nil {
		if p.backend == "redis" && p.redisFailOpen {
			// Fail open: allow request through on Redis errors
			slog.Warn("Rate limit check failed (fail-open)", "error", err)
			return policy.UpstreamRequestModifications{}
		}
		// Fail closed: deny request
		slog.Error("Rate limit check failed (fail-closed)", "error", err)
		return p.buildRateLimitResponse(nil)
	}

	// 6. Check if allowed
	if result.Allowed {
		// Request allowed - store result in metadata for response phase
		// Rate limit headers will be added to the response (not upstream request)
		ctx.Metadata[rateLimitResultKey] = result
		return policy.UpstreamRequestModifications{}
	}

	// 7. Request denied - return 429 with headers
	return p.buildRateLimitResponse(result)
}

// OnResponse adds rate limit headers to the response sent to the client
func (p *RateLimitPolicy) OnResponse(
	ctx *policy.ResponseContext,
	params map[string]interface{},
) policy.ResponseAction {
	// Handle post-response cost extraction mode
	if p.costExtractionEnabled {
		return p.handleCostExtractionResponse(ctx)
	}

	// Standard mode: retrieve rate limit result stored during request phase
	resultRaw, ok := ctx.Metadata[rateLimitResultKey]
	if !ok {
		// No rate limit result stored (e.g., fail-open on Redis error)
		return nil
	}

	result, ok := resultRaw.(*limiter.Result)
	if !ok {
		slog.Warn("Invalid rate limit result in metadata")
		return nil
	}

	// Add rate limit headers to the response
	headers := p.buildRateLimitHeaders(result, false)
	if len(headers) == 0 {
		return nil
	}

	return policy.UpstreamResponseModifications{
		SetHeaders: headers,
	}
}

// handleCostExtractionResponse handles post-response cost extraction and rate limiting
func (p *RateLimitPolicy) handleCostExtractionResponse(ctx *policy.ResponseContext) policy.ResponseAction {
	// Retrieve the key stored during request phase
	keyRaw, ok := ctx.Metadata[rateLimitKeyKey]
	if !ok {
		slog.Warn("Rate limit key not found in metadata for cost extraction")
		return nil
	}

	key, ok := keyRaw.(string)
	if !ok {
		slog.Warn("Invalid rate limit key type in metadata")
		return nil
	}

	// Extract actual cost from response
	actualCost, extracted := p.costExtractor.ExtractCost(ctx)
	if !extracted {
		slog.Debug("Cost extraction failed, using default", "key", key, "defaultCost", actualCost)
	}

	// Consume tokens now using existing AllowN
	result, err := p.limiter.AllowN(context.Background(), key, actualCost)

	if err != nil {
		if p.backend == "redis" && p.redisFailOpen {
			// Fail open: just log and continue without headers
			slog.Warn("Post-response rate limit check failed (fail-open)", "error", err, "key", key, "cost", actualCost)
			return nil
		}
		// Fail closed: log error (request already completed, can't block)
		slog.Error("Post-response rate limit check failed (fail-closed)", "error", err, "key", key, "cost", actualCost)
		return nil
	}

	if result != nil && !result.Allowed {
		// Note: Request already sent to upstream, so we just log and add headers
		// The rate limit will apply to subsequent requests
		slog.Warn("Rate limit exceeded post-response",
			"key", key,
			"cost", actualCost,
			"limit", result.Limit,
			"remaining", result.Remaining)
	}

	// Add rate limit headers
	headers := p.buildRateLimitHeaders(result, false)
	if len(headers) == 0 {
		return nil
	}

	return policy.UpstreamResponseModifications{
		SetHeaders: headers,
	}
}

// extractRateLimitKey builds the rate limit key from components
//
// IMPORTANT: The order of components in keyExtraction matters! Components are joined
// in the exact order specified in the configuration. For example:
//
//	[{type: "header", key: "X-User-ID"}, {type: "ip"}] creates key "user123:192.168.1.1"
//	[{type: "ip"}, {type: "header", key: "X-User-ID"}] creates key "192.168.1.1:user123"
//
// These are treated as DIFFERENT rate limit buckets. Changing the order in configuration
// will reset rate limit counters. Always maintain consistent ordering across deployments.
func (p *RateLimitPolicy) extractRateLimitKey(ctx *policy.RequestContext) string {
	if len(p.keyExtraction) == 0 {
		// Fallback to route name (should not happen due to default in GetPolicy)
		return p.routeName
	}

	if len(p.keyExtraction) == 1 {
		// Single component - no need to join
		return p.extractKeyComponent(ctx, p.keyExtraction[0])
	}

	// Multiple components - join with ':' in the order specified
	parts := make([]string, 0, len(p.keyExtraction))
	for _, comp := range p.keyExtraction {
		part := p.extractKeyComponent(ctx, comp)
		parts = append(parts, part)
	}
	return strings.Join(parts, ":")
}

// extractKeyComponent extracts a single component value
func (p *RateLimitPolicy) extractKeyComponent(ctx *policy.RequestContext, comp KeyComponent) string {
	switch comp.Type {
	case "header":
		values := ctx.Headers.Get(strings.ToLower(comp.Key))
		if len(values) > 0 && values[0] != "" {
			return values[0]
		}
		placeholder := fmt.Sprintf("_missing_header_%s_", comp.Key)
		slog.Warn("Header not found for rate limit key, using placeholder", "header", comp.Key, "type", comp.Type, "placeholder", placeholder)
		return placeholder

	case "metadata":
		if val, ok := ctx.Metadata[comp.Key]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				return strVal
			}
		}
		placeholder := fmt.Sprintf("_missing_metadata_%s_", comp.Key)
		slog.Warn("Metadata key not found for rate limit key, using placeholder", "key", comp.Key, "type", comp.Type, "placeholder", placeholder)
		return placeholder

	case "ip":
		return p.extractIPAddress(ctx)

	case "apiname":
		if ctx.APIName != "" {
			return ctx.APIName
		}
		slog.Warn("APIName not available for rate limit key, using empty string")
		return ""

	case "apiversion":
		if ctx.APIVersion != "" {
			return ctx.APIVersion
		}
		slog.Warn("APIVersion not available for rate limit key, using empty string")
		return ""

	case "routename":
		return p.routeName

	default:
		slog.Warn("Unknown key component type, using empty string", "type", comp.Type)
		return ""
	}
}

// extractIPAddress extracts client IP from headers
func (p *RateLimitPolicy) extractIPAddress(ctx *policy.RequestContext) string {
	// Try X-Forwarded-For first (most common)
	if xff := ctx.Headers.Get("x-forwarded-for"); len(xff) > 0 && xff[0] != "" {
		// Take the first IP (client)
		ips := strings.Split(xff[0], ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Try X-Real-IP
	if xri := ctx.Headers.Get("x-real-ip"); len(xri) > 0 && xri[0] != "" {
		return xri[0]
	}

	slog.Warn("Could not extract IP address for rate limit key, using 'unknown'")
	return "unknown"
}

// buildRateLimitHeaders creates rate limit headers
func (p *RateLimitPolicy) buildRateLimitHeaders(
	result *limiter.Result,
	rateLimited bool,
) map[string]string {
	headers := make(map[string]string)

	if result == nil {
		return headers
	}

	// X-RateLimit-* headers (de facto standard)
	if p.includeXRL {
		headers["x-ratelimit-limit"] = strconv.FormatInt(result.Limit, 10)
		headers["x-ratelimit-remaining"] = strconv.FormatInt(result.Remaining, 10)
		headers["x-ratelimit-reset"] = strconv.FormatInt(result.Reset.Unix(), 10)
	}

	// IETF RateLimit headers (draft standard)
	if p.includeIETF {
		headers["ratelimit-limit"] = strconv.FormatInt(result.Limit, 10)
		headers["ratelimit-remaining"] = strconv.FormatInt(result.Remaining, 10)

		resetSeconds := int64(time.Until(result.Reset).Seconds())
		if resetSeconds < 0 {
			resetSeconds = 0
		}
		headers["ratelimit-reset"] = strconv.FormatInt(resetSeconds, 10)

		// RateLimit-Policy format: <limit>;w=<window_in_seconds>
		if result.Policy != nil {
			policyValue := fmt.Sprintf("%d;w=%d",
				result.Limit,
				int64(result.Duration.Seconds()))
			headers["ratelimit-policy"] = policyValue
		}
	}

	// Retry-After header (only on 429 responses)
	if rateLimited && p.includeRetry && result.RetryAfter > 0 {
		seconds := int64(result.RetryAfter.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		headers["retry-after"] = strconv.FormatInt(seconds, 10)
	}

	return headers
}

// buildRateLimitResponse creates a 429 response
func (p *RateLimitPolicy) buildRateLimitResponse(result *limiter.Result) policy.ImmediateResponse {
	headers := p.buildRateLimitHeaders(result, true)

	// Set content-type based on format
	if p.responseFormat == "json" {
		headers["content-type"] = "application/json"
	} else {
		headers["content-type"] = "text/plain"
	}

	return policy.ImmediateResponse{
		StatusCode: p.statusCode,
		Headers:    headers,
		Body:       []byte(p.responseBody),
	}
}

// parseLimits parses the limits array from parameters
func parseLimits(raw interface{}) ([]LimitConfig, error) {
	limitsArray, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("limits must be an array")
	}

	if len(limitsArray) == 0 {
		return nil, fmt.Errorf("at least one limit must be specified")
	}

	limits := make([]LimitConfig, 0, len(limitsArray))
	for i, limitRaw := range limitsArray {
		limitMap, ok := limitRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("limits[%d] must be an object", i)
		}

		// Parse limit (required)
		limitVal, ok := limitMap["limit"]
		if !ok {
			return nil, fmt.Errorf("limits[%d].limit is required", i)
		}
		limitFloat, ok := limitVal.(float64)
		if !ok {
			return nil, fmt.Errorf("limits[%d].limit invalid type", i)
		}
		limit := int64(limitFloat) // JSON numbers come as float64

		// Parse duration (required)
		durationStr, ok := limitMap["duration"].(string)
		if !ok {
			return nil, fmt.Errorf("limits[%d].duration is required", i)
		}
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return nil, fmt.Errorf("limits[%d].duration invalid: %w", i, err)
		}

		// Parse burst (optional, defaults to limit)
		burst := limit
		if burstRaw, ok := limitMap["burst"]; ok {
			burstFloat, ok := burstRaw.(float64)
			if !ok {
				return nil, fmt.Errorf("limits[%d].burst invalid type", i)
			}
			burst = int64(burstFloat)
		}

		limits = append(limits, LimitConfig{
			Limit:    limit,
			Duration: duration,
			Burst:    burst,
		})
	}

	return limits, nil
}

// parseKeyExtraction parses the keyExtraction array
func parseKeyExtraction(raw interface{}) ([]KeyComponent, error) {
	if raw == nil {
		return []KeyComponent{}, nil
	}

	keArray, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("keyExtraction must be an array")
	}

	components := make([]KeyComponent, 0, len(keArray))
	for i, compRaw := range keArray {
		compMap, ok := compRaw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("keyExtraction[%d] must be an object", i)
		}

		compType, ok := compMap["type"].(string)
		if !ok {
			return nil, fmt.Errorf("keyExtraction[%d].type is required", i)
		}

		comp := KeyComponent{Type: compType}
		if keyRaw, ok := compMap["key"]; ok {
			if keyStr, ok := keyRaw.(string); ok {
				comp.Key = keyStr
			} else {
				return nil, fmt.Errorf("keyExtraction[%d].key must be a string", i)
			}
		}

		components = append(components, comp)
	}

	return components, nil
}

// Helper functions for extracting parameters with defaults

func getStringParam(params map[string]interface{}, key string, defaultVal string) string {
	// Support nested keys like "redis.host"
	keys := strings.Split(key, ".")
	current := params

	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key - get the value
			if val, ok := current[k].(string); ok {
				return val
			}
			return defaultVal
		}

		// Navigate to next level
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return defaultVal
		}
	}

	return defaultVal
}

func getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	keys := strings.Split(key, ".")
	current := params

	for i, k := range keys {
		if i == len(keys)-1 {
			if val, ok := current[k].(float64); ok {
				return int(val)
			}
			if val, ok := current[k].(int); ok {
				return val
			}
			return defaultVal
		}

		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return defaultVal
		}
	}

	return defaultVal
}

func getBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	keys := strings.Split(key, ".")
	current := params

	for i, k := range keys {
		if i == len(keys)-1 {
			if val, ok := current[k].(bool); ok {
				return val
			}
			return defaultVal
		}

		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return defaultVal
		}
	}

	return defaultVal
}

func getDurationParam(params map[string]interface{}, key string, defaultVal time.Duration) time.Duration {
	keys := strings.Split(key, ".")
	current := params

	for i, k := range keys {
		if i == len(keys)-1 {
			if val, ok := current[k].(string); ok {
				if duration, err := time.ParseDuration(val); err == nil {
					return duration
				}
			}
			return defaultVal
		}

		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return defaultVal
		}
	}

	return defaultVal
}

// getCacheKey computes a stable hash key for caching memory-backed limiters.
// The key includes scope (based on keyExtraction) and configuration.
// Scope logic: if keyExtraction has "apiname" AND NOT "routename" → API-level scope
// Otherwise → route-level scope (default, backward compatible)
func getCacheKey(routeName, apiName string, keyExtraction []KeyComponent, params map[string]interface{}) string {
	h := sha256.New()

	// Determine scope from keyExtraction
	hasApiName := false
	hasRouteName := false
	for _, comp := range keyExtraction {
		if comp.Type == "apiname" {
			hasApiName = true
		}
		if comp.Type == "routename" {
			hasRouteName = true
		}
	}

	if hasApiName && !hasRouteName {
		// API-level scope: share limiter across all routes of this API
		scopeKey := apiName
		if scopeKey == "" {
			scopeKey = "_unknown_api_"
		}
		h.Write([]byte("scope:api:"))
		h.Write([]byte(scopeKey))
		h.Write([]byte("|"))
	} else {
		// Route-level scope (default): separate limiter per route
		h.Write([]byte("scope:route:"))
		h.Write([]byte(routeName))
		h.Write([]byte("|"))
	}

	// Include algorithm
	algorithm := getStringParam(params, "algorithm", "gcra")
	h.Write([]byte("algo:"))
	h.Write([]byte(algorithm))
	h.Write([]byte("|"))

	// Include limits configuration (in order for stability)
	if limitsRaw, ok := params["limits"].([]interface{}); ok {
		h.Write([]byte("limits:"))
		for i, limitRaw := range limitsRaw {
			if limitMap, ok := limitRaw.(map[string]interface{}); ok {
				h.Write([]byte(fmt.Sprintf("[%d:", i)))
				if limit, ok := limitMap["limit"].(float64); ok {
					h.Write([]byte(fmt.Sprintf("l=%d,", int64(limit))))
				}
				if duration, ok := limitMap["duration"].(string); ok {
					h.Write([]byte(fmt.Sprintf("d=%s,", duration)))
				}
				if burst, ok := limitMap["burst"].(float64); ok {
					h.Write([]byte(fmt.Sprintf("b=%d", int64(burst))))
				}
				h.Write([]byte("]"))
			}
		}
		h.Write([]byte("|"))
	}

	// Include memory cleanup interval
	cleanupInterval := getDurationParam(params, "memory.cleanupInterval", 5*time.Minute)
	h.Write([]byte("cleanup:"))
	h.Write([]byte(cleanupInterval.String()))
	h.Write([]byte("|"))

	// Include key extraction configuration (in order for stability)
	if keRaw, ok := params["keyExtraction"].([]interface{}); ok {
		h.Write([]byte("keyExtraction:"))
		for i, compRaw := range keRaw {
			if compMap, ok := compRaw.(map[string]interface{}); ok {
				h.Write([]byte(fmt.Sprintf("[%d:", i)))
				if t, ok := compMap["type"].(string); ok {
					h.Write([]byte(fmt.Sprintf("t=%s", t)))
				}
				if k, ok := compMap["key"].(string); ok {
					h.Write([]byte(fmt.Sprintf(",k=%s", k)))
				}
				h.Write([]byte("]"))
			}
		}
		h.Write([]byte("|"))
	}

	// Include header configuration
	includeXRL := getBoolParam(params, "headers.includeXRateLimit", true)
	includeIETF := getBoolParam(params, "headers.includeIETF", true)
	includeRetry := getBoolParam(params, "headers.includeRetryAfter", true)
	h.Write([]byte(fmt.Sprintf("headers:xrl=%t,ietf=%t,retry=%t|", includeXRL, includeIETF, includeRetry)))

	// Include response configuration
	if exceeded, ok := params["onRateLimitExceeded"].(map[string]interface{}); ok {
		h.Write([]byte("exceeded:"))
		// Sort keys for stable ordering
		keys := make([]string, 0, len(exceeded))
		for k := range exceeded {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(fmt.Sprintf("%s=%v,", k, exceeded[k])))
		}
		h.Write([]byte("|"))
	}

	return hex.EncodeToString(h.Sum(nil))
}
