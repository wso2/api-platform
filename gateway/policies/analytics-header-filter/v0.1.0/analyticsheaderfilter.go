package analyticsheaderfilter

import (
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

var ins = &AnalyticsHeaderFilterPolicy{}

// GetPolicy returns the policy instance
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// AnalyticsHeaderFilterPolicy implements header exclusion from analytics
type AnalyticsHeaderFilterPolicy struct{}

// Mode returns the processing mode for this policy
func (p *AnalyticsHeaderFilterPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeProcess, // Process response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// parseHeaderList parses a list of header names from parameters
func (p *AnalyticsHeaderFilterPolicy) parseHeaderList(headersRaw interface{}) []string {
	if headersRaw == nil {
		return nil
	}

	headers, ok := headersRaw.([]interface{})
	if !ok {
		return nil
	}

	headerList := make([]string, 0, len(headers))
	for _, headerRaw := range headers {
		header, ok := headerRaw.(string)
		if !ok || strings.TrimSpace(header) == "" {
			continue
		}
		// Normalize to lowercase for consistent matching
		headerList = append(headerList, strings.ToLower(strings.TrimSpace(header)))
	}

	return headerList
}

// parseOperation parses and validates the operation parameter
func (p *AnalyticsHeaderFilterPolicy) parseOperation(operationRaw interface{}) (string, error) {
	if operationRaw == nil {
		return "", fmt.Errorf("operation is required")
	}

	operation, ok := operationRaw.(string)
	if !ok {
		return "", fmt.Errorf("'operation' must be a string")
	}

	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "allow" && operation != "deny" {
		return "", fmt.Errorf("'operation' must be either 'allow' or 'deny', got: %s", operation)
	}

	return operation, nil
}

// parseHeaderFilterConfig parses the header filter configuration object
// Expected structure: { "operation": "allow"|"deny", "headers": ["header1", "header2"] }
func (p *AnalyticsHeaderFilterPolicy) parseHeaderFilterConfig(configRaw interface{}) (operation string, headers []string, err error) {
	if configRaw == nil {
		return "", nil, nil // No configuration provided
	}

	config, ok := configRaw.(map[string]interface{})
	if !ok {
		return "", nil, fmt.Errorf("header filter config must be an object")
	}

	// Parse operation (required)
	operationRaw, hasOperation := config["operation"]
	if !hasOperation || operationRaw == nil {
		return "", nil, fmt.Errorf("'operation' is required in header filter config")
	}
	operation, err = p.parseOperation(operationRaw)
	if err != nil {
		return "", nil, err
	}

	// Parse headers (optional, defaults to empty array)
	headersRaw, _ := config["headers"]
	headers = p.parseHeaderList(headersRaw)

	return operation, headers, nil
}

// OnRequest processes request headers and marks them for exclusion from analytics
func (p *AnalyticsHeaderFilterPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	requestConfigRaw, hasRequestConfig := params["requestHeadersToFilter"]
	if !hasRequestConfig || requestConfigRaw == nil {
		// No request headers filter configuration, return empty action
		return policy.UpstreamRequestModifications{}
	}

	operation, specifiedHeaders, err := p.parseHeaderFilterConfig(requestConfigRaw)
	if err != nil {
		slog.Warn("Analytics Header Filter Policy: Failed to parse request headers filter config", "error", err)
		return policy.UpstreamRequestModifications{}
	}

	slog.Debug("Analytics Header Filter Policy: Parsed request config",
		"operation", operation,
		"headers", specifiedHeaders)

	// Set DropHeadersFromAnalytics action (no processing, just pass the config)
	return policy.UpstreamRequestModifications{
		DropHeadersFromAnalytics: policy.DropHeaderAction{
			Action:  operation,
			Headers: specifiedHeaders,
		},
	}
}

// OnResponse processes response headers and marks them for exclusion from analytics
func (p *AnalyticsHeaderFilterPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	responseConfigRaw, hasResponseConfig := params["responseHeadersToFilter"]
	if !hasResponseConfig || responseConfigRaw == nil {
		// No response headers filter configuration, return empty action
		return policy.UpstreamResponseModifications{}
	}

	operation, specifiedHeaders, err := p.parseHeaderFilterConfig(responseConfigRaw)
	if err != nil {
		slog.Warn("Analytics Header Filter Policy: Failed to parse response headers filter config", "error", err)
		return policy.UpstreamResponseModifications{}
	}

	slog.Debug("Analytics Header Filter Policy: Parsed response config",
		"operation", operation,
		"headers", specifiedHeaders)

	// Set DropHeadersFromAnalytics action (no processing, just pass the config)
	return policy.UpstreamResponseModifications{
		DropHeadersFromAnalytics: policy.DropHeaderAction{
			Action:  operation,
			Headers: specifiedHeaders,
		},
	}
}
