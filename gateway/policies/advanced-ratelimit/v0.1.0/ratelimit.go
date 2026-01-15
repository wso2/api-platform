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

	_ "github.com/policy-engine/policies/advanced-ratelimit/algorithms/fixedwindow" // Register Fixed Window algorithm
	_ "github.com/policy-engine/policies/advanced-ratelimit/algorithms/gcra"        // Register GCRA algorithm
	"github.com/policy-engine/policies/advanced-ratelimit/limiter"
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

// QuotaRuntime holds per-quota runtime configuration and limiter instance.
// Each quota is a self-contained rate limit dimension with its own key extraction,
// cost extraction, and limiter.
type QuotaRuntime struct {
	Name                  string          // Optional name for logging/headers
	Limits                []LimitConfig   // Rate limits for this quota
	KeyExtraction         []KeyComponent  // Per-quota key extraction
	Limiter               limiter.Limiter // Limiter instance for this quota
	CostExtractor         *CostExtractor  // Per-quota cost extractor
	CostExtractionEnabled bool            // Whether cost extraction is enabled
}

// RateLimitPolicy defines the policy for rate limiting
type RateLimitPolicy struct {
	quotas         []QuotaRuntime // Per-quota configurations with independent limiters
	routeName      string         // From metadata, used as default key
	apiId          string         // From metadata, API identifier
	apiName        string         // From metadata, API name for scope-based caching
	apiVersion     string         // From metadata, API version
	statusCode     int
	responseBody   string
	responseFormat string
	backend        string
	redisClient    *redis.Client
	redisFailOpen  bool
	includeXRL     bool
	includeIETF    bool
	includeRetry   bool
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

	// Parse system parameters
	algorithm := getStringParam(params, "algorithm", "gcra")
	backend := getStringParam(params, "backend", "memory")

	// Header configuration
	includeXRL := getBoolParam(params, "headers.includeXRateLimit", true)
	includeIETF := getBoolParam(params, "headers.includeIETF", true)
	includeRetry := getBoolParam(params, "headers.includeRetryAfter", true)

	// Parse global keyExtraction (used as default for quotas missing keyExtraction)
	globalKeyExtraction, err := parseKeyExtraction(params["keyExtraction"])
	if err != nil {
		return nil, fmt.Errorf("invalid keyExtraction: %w", err)
	}

	// Default keyExtraction when nothing is specified
	defaultKeyExtraction := []KeyComponent{{Type: "routename"}}

	// Parse quotas config (required)
	quotas, err := parseQuotas(params)
	if err != nil {
		return nil, err
	}

	if len(quotas) == 0 {
		return nil, fmt.Errorf("quotas configuration is required")
	}

	// Fill in missing keyExtraction from global or default
	for i := range quotas {
		if len(quotas[i].KeyExtraction) == 0 {
			if len(globalKeyExtraction) > 0 {
				quotas[i].KeyExtraction = globalKeyExtraction
			} else {
				quotas[i].KeyExtraction = defaultKeyExtraction
			}
		}
	}

	// Initialize limiters for each quota based on backend
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

		// Create a limiter per quota
		for i := range quotas {
			q := &quotas[i]
			limiterLimits := make([]limiter.LimitConfig, len(q.Limits))
			for j, lim := range q.Limits {
				limiterLimits[j] = limiter.LimitConfig{
					Limit:    lim.Limit,
					Duration: lim.Duration,
					Burst:    lim.Burst,
				}
			}

			rlLimiter, err := limiter.CreateLimiter(limiter.Config{
				Algorithm:       algorithm,
				Limits:          limiterLimits,
				Backend:         backend,
				RedisClient:     redisClient,
				KeyPrefix:       keyPrefix,
				CleanupInterval: 0, // Not used for Redis
			})
			if err != nil {
				quotaName := q.Name
				if quotaName == "" {
					quotaName = fmt.Sprintf("quota-%d", i)
				}
				return nil, fmt.Errorf("failed to create Redis limiter for quota %q: %w", quotaName, err)
			}

			q.Limiter = rlLimiter
		}
	} else {
		// Memory backend - create limiter per quota with caching
		cleanupInterval := getDurationParam(params, "memory.cleanupInterval", 5*time.Minute)
		baseCacheKey := getBaseCacheKey(routeName, apiName, algorithm, params)

		for i := range quotas {
			q := &quotas[i]
			limiterLimits := make([]limiter.LimitConfig, len(q.Limits))
			for j, lim := range q.Limits {
				limiterLimits[j] = limiter.LimitConfig{
					Limit:    lim.Limit,
					Duration: lim.Duration,
					Burst:    lim.Burst,
				}
			}

			cacheKey := getQuotaCacheKey(baseCacheKey, apiName, q, i)

			// Try to get cached limiter
			if cached, ok := memoryLimiterCache.Load(cacheKey); ok {
				q.Limiter = cached.(limiter.Limiter)
				slog.Debug("Reusing cached memory limiter",
					"route", routeName, "apiName", apiName,
					"quota", q.Name, "cacheKey", cacheKey[:16])
			} else {
				// Create new limiter
				rlLimiter, err := limiter.CreateLimiter(limiter.Config{
					Algorithm:       algorithm,
					Limits:          limiterLimits,
					Backend:         backend,
					CleanupInterval: cleanupInterval,
				})
				if err != nil {
					quotaName := q.Name
					if quotaName == "" {
						quotaName = fmt.Sprintf("quota-%d", i)
					}
					return nil, fmt.Errorf("failed to create memory limiter for quota %q: %w", quotaName, err)
				}

				// Store in cache
				memoryLimiterCache.Store(cacheKey, rlLimiter)
				q.Limiter = rlLimiter
				slog.Debug("Created and cached new memory limiter",
					"route", routeName, "apiName", apiName,
					"quota", q.Name, "cacheKey", cacheKey[:16])
			}
		}
	}

	// Return configured policy instance
	return &RateLimitPolicy{
		quotas:         quotas,
		routeName:      routeName,
		apiId:          apiId,
		apiName:        apiName,
		apiVersion:     apiVersion,
		statusCode:     statusCode,
		responseBody:   responseBody,
		responseFormat: responseFormat,
		backend:        backend,
		redisClient:    redisClient,
		redisFailOpen:  redisFailOpen,
		includeXRL:     includeXRL,
		includeIETF:    includeIETF,
		includeRetry:   includeRetry,
	}, nil
}

// Metadata keys for storing data across request/response phases
const (
	rateLimitResultKey = "ratelimit:result"
	rateLimitKeysKey   = "ratelimit:keys" // Store extracted keys for post-response cost extraction
)

// Mode returns the processing mode for this policy
func (p *RateLimitPolicy) Mode() policy.ProcessingMode {
	requestBodyMode := policy.BodyModeSkip
	responseBodyMode := policy.BodyModeSkip

	// Check if any quota needs request or response body
	for _, q := range p.quotas {
		if q.CostExtractionEnabled && q.CostExtractor != nil {
			if q.CostExtractor.RequiresRequestBody() {
				requestBodyMode = policy.BodyModeBuffer
			}
			if q.CostExtractor.RequiresResponseBody() {
				responseBodyMode = policy.BodyModeBuffer
			}
		}
	}

	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Need headers for key extraction
		RequestBodyMode:    requestBodyMode,          // Buffer if cost extraction from request body is configured
		ResponseHeaderMode: policy.HeaderModeProcess, // Need to add rate limit headers to response
		ResponseBodyMode:   responseBodyMode,         // Buffer if cost extraction from response body is configured
	}
}

// quotaResult stores the result of checking a single quota
type quotaResult struct {
	QuotaName string
	Result    *limiter.Result
	Key       string
	Duration  time.Duration // Window duration for IETF RateLimit-Policy header
}

// OnRequest performs rate limit check across all quotas
func (p *RateLimitPolicy) OnRequest(
	ctx *policy.RequestContext,
	params map[string]interface{},
) policy.RequestAction {
	var quotaResults []quotaResult
	var quotaKeys = make(map[string]string) // Store keys for response phase

	for i := range p.quotas {
		q := &p.quotas[i]

		// Extract rate limit key for this quota
		key := p.extractQuotaKey(ctx, q)
		quotaName := q.Name
		if quotaName == "" {
			quotaName = fmt.Sprintf("quota-%d", i)
		}
		quotaKeys[quotaName] = key

		// If cost extraction is enabled, handle based on whether we have request-phase or response-phase sources
		if q.CostExtractionEnabled && q.CostExtractor != nil {
			// Check if this quota has request-phase sources (can be processed now)
			if q.CostExtractor.HasRequestPhaseSources() {
				// Extract cost from request (headers, metadata, or body)
				requestCost, extracted := q.CostExtractor.ExtractRequestCost(ctx)
				if !extracted {
					slog.Debug("Request cost extraction failed, using default",
						"key", key, "quota", quotaName, "defaultCost", requestCost)
				}

				// Clamp cost to minimum of 0
				if requestCost < 0 {
					requestCost = 0
				}

				// Consume tokens based on extracted request cost
				cost := int64(requestCost)
				result, err := q.Limiter.AllowN(context.Background(), key, cost)
				if err != nil {
					if p.backend == "redis" && p.redisFailOpen {
						slog.Warn("Rate limit check failed (fail-open)", "error", err, "quota", quotaName)
						continue
					}
					slog.Error("Rate limit check failed (fail-closed)", "error", err, "quota", quotaName)
					return p.buildRateLimitResponse(nil, quotaName, quotaResults)
				}

				if !result.Allowed {
					slog.Debug("Rate limit exceeded", "key", key, "cost", cost, "quota", quotaName)
					return p.buildRateLimitResponse(result, quotaName, quotaResults)
				}

				quotaResults = append(quotaResults, quotaResult{
					QuotaName: quotaName,
					Result:    result,
					Key:       key,
					Duration:  result.Duration,
				})
				continue
			}

			// Response-phase cost extraction: pre-check if quota is already exhausted
			// Pre-check: if remaining quota is already <= 0, block the request
			result, err := q.Limiter.AllowN(context.Background(), key, 0)
			if err != nil {
				if p.backend == "redis" && p.redisFailOpen {
					slog.Warn("Rate limit pre-check failed (fail-open)", "error", err, "key", key, "quota", quotaName)
					continue
				}
				slog.Error("Rate limit pre-check failed (fail-closed)", "error", err, "key", key, "quota", quotaName)
				return p.buildRateLimitResponse(nil, quotaName, quotaResults)
			}

			// If remaining <= 0, quota is exhausted - block the request
			if result != nil && result.Remaining <= 0 {
				slog.Debug("Cost extraction mode: quota exhausted, blocking request",
					"key", key, "remaining", result.Remaining, "quota", quotaName)
				return p.buildRateLimitResponse(result, quotaName, quotaResults)
			}

			quotaResults = append(quotaResults, quotaResult{
				QuotaName: quotaName,
				Result:    result,
				Key:       key,
				Duration:  result.Duration,
			})
			continue
		}

		// Standard mode (no cost extraction): consume 1 token per request
		cost := int64(1)

		result, err := q.Limiter.AllowN(context.Background(), key, cost)
		if err != nil {
			if p.backend == "redis" && p.redisFailOpen {
				slog.Warn("Rate limit check failed (fail-open)", "error", err, "quota", quotaName)
				continue
			}
			slog.Error("Rate limit check failed (fail-closed)", "error", err, "quota", quotaName)
			return p.buildRateLimitResponse(nil, quotaName, quotaResults)
		}

		if !result.Allowed {
			slog.Debug("Rate limit exceeded", "key", key, "quota", quotaName)
			return p.buildRateLimitResponse(result, quotaName, quotaResults)
		}

		quotaResults = append(quotaResults, quotaResult{
			QuotaName: quotaName,
			Result:    result,
			Key:       key,
			Duration:  result.Duration,
		})
	}

	// Store results and keys in metadata for response phase
	ctx.Metadata[rateLimitResultKey] = quotaResults
	ctx.Metadata[rateLimitKeysKey] = quotaKeys

	return policy.UpstreamRequestModifications{}
}

// OnResponse adds rate limit headers to the response sent to the client
func (p *RateLimitPolicy) OnResponse(
	ctx *policy.ResponseContext,
	params map[string]interface{},
) policy.ResponseAction {
	// Retrieve stored keys for cost extraction
	quotaKeysRaw, hasKeys := ctx.Metadata[rateLimitKeysKey]
	quotaKeys := make(map[string]string)
	if hasKeys {
		if keys, ok := quotaKeysRaw.(map[string]string); ok {
			quotaKeys = keys
		}
	}

	// Retrieve stored results from request phase
	resultsRaw, hasResults := ctx.Metadata[rateLimitResultKey]
	var storedResults []quotaResult
	if hasResults {
		if results, ok := resultsRaw.([]quotaResult); ok {
			storedResults = results
		}
	}

	// Create a map for quick lookup of stored results (preserving full quotaResult)
	storedResultsMap := make(map[string]quotaResult)
	for _, r := range storedResults {
		storedResultsMap[r.QuotaName] = r
	}

	// Process each quota for post-response cost extraction
	// Collect full quotaResult structs to preserve quota names and durations for headers
	var allQuotaResults []quotaResult

	for i := range p.quotas {
		q := &p.quotas[i]
		quotaName := q.Name
		if quotaName == "" {
			quotaName = fmt.Sprintf("quota-%d", i)
		}

		// Handle post-response cost extraction for quotas that have it enabled
		if q.CostExtractionEnabled && q.CostExtractor != nil && q.CostExtractor.HasResponsePhaseSources() {
			key := quotaKeys[quotaName]
			if key == "" {
				slog.Warn("Rate limit key not found for cost extraction", "quota", quotaName)
				continue
			}

			// Extract actual cost from response
			actualCost, extracted := q.CostExtractor.ExtractResponseCost(ctx)
			if !extracted {
				slog.Debug("Cost extraction failed, using default", "key", key, "quota", quotaName, "defaultCost", actualCost)
			}

			// Clamp cost to minimum of 0 (allow 0 cost for free operations)
			if actualCost < 0 {
				actualCost = 0
			}

			// Skip if cost is 0
			if actualCost == 0 {
				// Still include stored result for headers
				if stored, ok := storedResultsMap[quotaName]; ok && stored.Result != nil {
					allQuotaResults = append(allQuotaResults, stored)
				}
				continue
			}

			// Consume tokens now
			result, err := q.Limiter.AllowN(context.Background(), key, int64(actualCost))
			if err != nil {
				if p.backend == "redis" && p.redisFailOpen {
					slog.Warn("Post-response rate limit check failed (fail-open)",
						"error", err, "key", key, "cost", actualCost, "quota", quotaName)
					continue
				}
				slog.Error("Post-response rate limit check failed (fail-closed)",
					"error", err, "key", key, "cost", actualCost, "quota", quotaName)
				continue
			}

			if result != nil && !result.Allowed {
				slog.Warn("Rate limit exceeded post-response",
					"key", key, "cost", actualCost, "limit", result.Limit,
					"remaining", result.Remaining, "quota", quotaName)
			}

			allQuotaResults = append(allQuotaResults, quotaResult{
				QuotaName: quotaName,
				Result:    result,
				Key:       key,
				Duration:  result.Duration,
			})
		} else {
			// Use stored result from request phase
			if stored, ok := storedResultsMap[quotaName]; ok && stored.Result != nil {
				allQuotaResults = append(allQuotaResults, stored)
			}
		}
	}

	// Build headers for all quotas using the new multi-quota function
	if len(allQuotaResults) == 0 {
		return nil
	}

	headers := p.buildMultiQuotaHeaders(allQuotaResults, false, "")
	if len(headers) == 0 {
		return nil
	}

	return policy.UpstreamResponseModifications{
		SetHeaders: headers,
	}
}

// getMostRestrictiveResult returns the result with the lowest remaining quota
func (p *RateLimitPolicy) getMostRestrictiveResult(results []*limiter.Result) *limiter.Result {
	if len(results) == 0 {
		return nil
	}

	var mostRestrictive *limiter.Result
	for _, r := range results {
		if r == nil {
			continue
		}
		if mostRestrictive == nil || r.Remaining < mostRestrictive.Remaining {
			mostRestrictive = r
		}
	}

	return mostRestrictive
}

// extractQuotaKey builds the rate limit key from quota's key extraction components
func (p *RateLimitPolicy) extractQuotaKey(ctx *policy.RequestContext, q *QuotaRuntime) string {
	if len(q.KeyExtraction) == 0 {
		return p.routeName
	}

	if len(q.KeyExtraction) == 1 {
		return p.extractKeyComponent(ctx, q.KeyExtraction[0])
	}

	// Multiple components - join with ':' in the order specified
	parts := make([]string, 0, len(q.KeyExtraction))
	for _, comp := range q.KeyExtraction {
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

// buildMultiQuotaHeaders creates rate limit headers for all quotas.
// For IETF headers, uses Structured Fields format to report all quotas.
// For X-RateLimit-* headers, uses the most restrictive quota (legacy compatibility).
// Reference: draft-ietf-httpapi-ratelimit-headers-10
func (p *RateLimitPolicy) buildMultiQuotaHeaders(
	allResults []quotaResult,
	rateLimited bool,
	violatedQuota string,
) map[string]string {
	headers := make(map[string]string)

	if len(allResults) == 0 {
		return headers
	}

	// Find the most restrictive result for X-RateLimit-* headers (legacy)
	var mostRestrictive *quotaResult
	for i := range allResults {
		r := &allResults[i]
		if r.Result == nil {
			continue
		}
		if mostRestrictive == nil || r.Result.Remaining < mostRestrictive.Result.Remaining {
			mostRestrictive = r
		}
	}

	// X-RateLimit-* headers (de facto standard) - most restrictive only for backward compatibility
	if p.includeXRL && mostRestrictive != nil && mostRestrictive.Result != nil {
		headers["x-ratelimit-limit"] = strconv.FormatInt(mostRestrictive.Result.Limit, 10)
		headers["x-ratelimit-remaining"] = strconv.FormatInt(mostRestrictive.Result.Remaining, 10)
		headers["x-ratelimit-reset"] = strconv.FormatInt(mostRestrictive.Result.Reset.Unix(), 10)
	}

	// IETF RateLimit headers (draft standard) - all quotas using Structured Fields format
	// Format: RateLimit-Policy: "quota1";q=100;w=60, "quota2";q=1000;w=86400
	// Format: RateLimit: "quota1";r=90;t=45, "quota2";r=950;t=3600
	if p.includeIETF {
		var policyParts []string
		var limitParts []string

		for _, qr := range allResults {
			if qr.Result == nil {
				continue
			}

			// Sanitize quota name for use in Structured Fields string
			quotaName := qr.QuotaName
			if quotaName == "" {
				quotaName = "default"
			}

			// RateLimit-Policy: "<name>";q=<limit>;w=<window>
			windowSeconds := int64(qr.Duration.Seconds())
			if windowSeconds <= 0 && qr.Result.Duration > 0 {
				windowSeconds = int64(qr.Result.Duration.Seconds())
			}
			policyPart := fmt.Sprintf(`"%s";q=%d;w=%d`,
				quotaName,
				qr.Result.Limit,
				windowSeconds)
			policyParts = append(policyParts, policyPart)

			// RateLimit: "<name>";r=<remaining>;t=<reset_seconds>
			resetSeconds := int64(time.Until(qr.Result.Reset).Seconds())
			if resetSeconds < 0 {
				resetSeconds = 0
			}
			limitPart := fmt.Sprintf(`"%s";r=%d;t=%d`,
				quotaName,
				qr.Result.Remaining,
				resetSeconds)
			limitParts = append(limitParts, limitPart)
		}

		if len(policyParts) > 0 {
			headers["ratelimit-policy"] = strings.Join(policyParts, ", ")
		}
		if len(limitParts) > 0 {
			headers["ratelimit"] = strings.Join(limitParts, ", ")
		}
	}

	// Retry-After header (only on 429 responses) - use violated quota or most restrictive
	if rateLimited && p.includeRetry {
		var retryResult *limiter.Result

		// Prefer the violated quota for retry-after
		if violatedQuota != "" {
			for _, qr := range allResults {
				if qr.QuotaName == violatedQuota && qr.Result != nil {
					retryResult = qr.Result
					break
				}
			}
		}

		// Fall back to most restrictive
		if retryResult == nil && mostRestrictive != nil {
			retryResult = mostRestrictive.Result
		}

		if retryResult != nil && retryResult.RetryAfter > 0 {
			seconds := int64(retryResult.RetryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			headers["retry-after"] = strconv.FormatInt(seconds, 10)
		}
	}

	return headers
}

// buildRateLimitResponse creates a 429 response with multi-quota headers
func (p *RateLimitPolicy) buildRateLimitResponse(
	violatedResult *limiter.Result,
	violatedQuotaName string,
	allResults []quotaResult,
) policy.ImmediateResponse {
	// If we have all results, use the multi-quota header builder
	var headers map[string]string
	if len(allResults) > 0 {
		// Add the violated quota to the results if not already present
		hasViolated := false
		for _, qr := range allResults {
			if qr.QuotaName == violatedQuotaName {
				hasViolated = true
				break
			}
		}
		if !hasViolated && violatedResult != nil {
			allResults = append(allResults, quotaResult{
				QuotaName: violatedQuotaName,
				Result:    violatedResult,
				Duration:  violatedResult.Duration,
			})
		}
		headers = p.buildMultiQuotaHeaders(allResults, true, violatedQuotaName)
	} else if violatedResult != nil {
		// Fallback to single result
		headers = p.buildMultiQuotaHeaders([]quotaResult{
			{
				QuotaName: violatedQuotaName,
				Result:    violatedResult,
				Duration:  violatedResult.Duration,
			},
		}, true, violatedQuotaName)
	} else {
		headers = make(map[string]string)
	}

	// Set content-type based on format
	if p.responseFormat == "json" {
		headers["content-type"] = "application/json"
	} else {
		headers["content-type"] = "text/plain"
	}

	// Add violated quota name to headers for debugging
	if violatedQuotaName != "" {
		headers["x-ratelimit-quota"] = violatedQuotaName
	}

	return policy.ImmediateResponse{
		StatusCode: p.statusCode,
		Headers:    headers,
		Body:       []byte(p.responseBody),
	}
}

// parseQuotas parses the new "quotas" array. If absent, returns nil, nil.
func parseQuotas(params map[string]interface{}) ([]QuotaRuntime, error) {
	raw, ok := params["quotas"]
	if !ok || raw == nil {
		return nil, nil
	}

	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("quotas must be an array")
	}

	quotas := make([]QuotaRuntime, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("quotas[%d] must be an object", i)
		}

		// Name (optional)
		name, _ := m["name"].(string)

		// Parse limits array (required)
		limitsRaw, hasLimits := m["limits"]
		if !hasLimits {
			return nil, fmt.Errorf("quotas[%d].limits is required", i)
		}

		limits, err := parseLimits(limitsRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid quotas[%d].limits: %w", i, err)
		}
		if len(limits) == 0 {
			return nil, fmt.Errorf("quotas[%d].limits must not be empty", i)
		}

		// Per-quota keyExtraction
		quotaKeyExtraction, err := parseKeyExtraction(m["keyExtraction"])
		if err != nil {
			return nil, fmt.Errorf("invalid quotas[%d].keyExtraction: %w", i, err)
		}

		// Per-quota costExtraction
		ceCfg, err := parseCostExtractionConfig(m["costExtraction"])
		if err != nil {
			return nil, fmt.Errorf("invalid quotas[%d].costExtraction: %w", i, err)
		}

		var ce *CostExtractor
		enabled := false
		if ceCfg != nil && ceCfg.Enabled {
			ce = NewCostExtractor(*ceCfg)
			enabled = true
		}

		quotas = append(quotas, QuotaRuntime{
			Name:                  name,
			Limits:                limits,
			KeyExtraction:         quotaKeyExtraction,
			CostExtractor:         ce,
			CostExtractionEnabled: enabled,
		})
	}

	return quotas, nil
}

// parseSingleLimit parses a single limit configuration
func parseSingleLimit(limitVal, durationVal, burstVal interface{}) (*LimitConfig, error) {
	limitFloat, ok := limitVal.(float64)
	if !ok {
		return nil, fmt.Errorf("limit must be a number")
	}
	limit := int64(limitFloat)

	durationStr, ok := durationVal.(string)
	if !ok {
		return nil, fmt.Errorf("duration must be a string")
	}
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}

	// Parse burst (optional, defaults to limit)
	burst := limit
	if burstVal != nil {
		burstFloat, ok := burstVal.(float64)
		if !ok {
			return nil, fmt.Errorf("burst must be a number")
		}
		burst = int64(burstFloat)
	}

	return &LimitConfig{
		Limit:    limit,
		Duration: duration,
		Burst:    burst,
	}, nil
}

// parseLimits parses the limits array from parameters
func parseLimits(raw interface{}) ([]LimitConfig, error) {
	if raw == nil {
		return nil, nil
	}

	limitsArray, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("limits must be an array")
	}

	if len(limitsArray) == 0 {
		return nil, nil
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

		limit, err := parseSingleLimit(limitVal, limitMap["duration"], limitMap["burst"])
		if err != nil {
			return nil, fmt.Errorf("invalid limits[%d]: %w", i, err)
		}

		limits = append(limits, *limit)
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

// getBaseCacheKey computes a stable hash key base for caching memory-backed limiters.
// This includes shared aspects like algorithm, headers config, etc.
func getBaseCacheKey(routeName, apiName, algorithm string, params map[string]interface{}) string {
	h := sha256.New()

	h.Write([]byte("route:"))
	h.Write([]byte(routeName))
	h.Write([]byte("|"))

	h.Write([]byte("api:"))
	h.Write([]byte(apiName))
	h.Write([]byte("|"))

	h.Write([]byte("algo:"))
	h.Write([]byte(algorithm))
	h.Write([]byte("|"))

	// Include memory cleanup interval
	cleanupInterval := getDurationParam(params, "memory.cleanupInterval", 5*time.Minute)
	h.Write([]byte("cleanup:"))
	h.Write([]byte(cleanupInterval.String()))
	h.Write([]byte("|"))

	// Include header configuration
	includeXRL := getBoolParam(params, "headers.includeXRateLimit", true)
	includeIETF := getBoolParam(params, "headers.includeIETF", true)
	includeRetry := getBoolParam(params, "headers.includeRetryAfter", true)
	h.Write([]byte(fmt.Sprintf("headers:xrl=%t,ietf=%t,retry=%t|", includeXRL, includeIETF, includeRetry)))

	// Include response configuration
	if exceeded, ok := params["onRateLimitExceeded"].(map[string]interface{}); ok {
		h.Write([]byte("exceeded:"))
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

// getQuotaCacheKey produces final key per quota using base + quota-specific config.
// apiName is passed separately to enable API-scoped cache keys for quotas using apiname keyExtraction.
func getQuotaCacheKey(base, apiName string, q *QuotaRuntime, index int) string {
	h := sha256.New()

	// Determine scope from keyExtraction
	hasApiName := false
	hasRouteName := false
	for _, comp := range q.KeyExtraction {
		if comp.Type == "apiname" {
			hasApiName = true
		}
		if comp.Type == "routename" {
			hasRouteName = true
		}
	}

	// For API-scoped quotas (apiname key extraction without routename),
	// use a stable API-based cache key so all routes under the same API share the limiter.
	// Otherwise, use the route-specific base cache key.
	if hasApiName && !hasRouteName {
		// API-scoped: use apiName instead of route-specific base
		h.Write([]byte("apiScope:"))
		h.Write([]byte(apiName))
		h.Write([]byte("|"))
	} else {
		// Route-scoped: use the full base key (includes route name)
		h.Write([]byte(base))
	}

	h.Write([]byte("|quota:"))
	if q.Name != "" {
		h.Write([]byte(q.Name))
	} else {
		h.Write([]byte(fmt.Sprintf("idx-%d", index)))
	}
	h.Write([]byte("|"))

	// Include limits
	h.Write([]byte("limits:"))
	for i, lim := range q.Limits {
		h.Write([]byte(fmt.Sprintf("[%d:l=%d,d=%s,b=%d]", i, lim.Limit, lim.Duration, lim.Burst)))
	}
	h.Write([]byte("|"))

	// Include key extraction
	h.Write([]byte("keyExtraction:"))
	for i, comp := range q.KeyExtraction {
		h.Write([]byte(fmt.Sprintf("[%d:t=%s,k=%s]", i, comp.Type, comp.Key)))
	}
	h.Write([]byte("|"))

	return hex.EncodeToString(h.Sum(nil))
}
