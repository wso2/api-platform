package azurecontentsafetycontentmoderation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

const (
	GuardrailErrorCode = 422
	TextCleanRegex     = "^\"|\"$"
	endpointSuffix     = "/contentsafety/text:analyze?api-version=2024-09-01"
	requestTimeout     = 30 * time.Second
	maxRetries         = 5
	retryDelay         = 1 * time.Second
)

var textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)

// AzureContentSafetyContentModerationPolicy implements Azure Content Safety content moderation
type AzureContentSafetyContentModerationPolicy struct {
	// Static configuration from params
	endpoint string
	apiKey   string

	// HTTP client for connection reuse and pooling
	client *http.Client

	// Dynamic configuration from params
	hasRequestParams  bool
	hasResponseParams bool
	requestParams     AzureContentSafetyPolicyParams
	responseParams    AzureContentSafetyPolicyParams
}

type AzureContentSafetyPolicyParams struct {
	JsonPath           string
	PassthroughOnError bool
	ShowAssessment     bool
	HateCategory       int
	SexualCategory     int
	SelfHarmCategory   int
	ViolenceCategory   int
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	// Validate and extract static configuration from params
	if err := validateAzureConfigParams(params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	p := &AzureContentSafetyContentModerationPolicy{
		endpoint: getStringParam(params, "azureContentSafetyEndpoint"),
		apiKey:   getStringParam(params, "azureContentSafetyKey"),
		client: &http.Client{
			Timeout: requestTimeout,
		},
	}

	// Extract and parse request parameters if present
	if requestParamsRaw, ok := params["request"].(map[string]interface{}); ok {
		requestParams, err := parseRequestResponseParams(requestParamsRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid request parameters: %w", err)
		}
		p.hasRequestParams = true
		p.requestParams = requestParams
	}

	// Extract and parse response parameters if present
	if responseParamsRaw, ok := params["response"].(map[string]interface{}); ok {
		responseParams, err := parseRequestResponseParams(responseParamsRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid response parameters: %w", err)
		}
		p.hasResponseParams = true
		p.responseParams = responseParams
	}

	// At least one of request or response must be present
	if !p.hasRequestParams && !p.hasResponseParams {
		return nil, fmt.Errorf("at least one of 'request' or 'response' parameters must be provided")
	}

	slog.Debug("AzureContentSafety: Policy initialized", "endpoint", p.endpoint, "hasRequestParams", p.hasRequestParams, "hasResponseParams", p.hasResponseParams)

	return p, nil
}

// parseRequestResponseParams parses and validates request/response parameters from map to struct
func parseRequestResponseParams(params map[string]interface{}) (AzureContentSafetyPolicyParams, error) {
	var result AzureContentSafetyPolicyParams

	// Initialize category thresholds to -1 (disabled by default)
	result.HateCategory = -1
	result.SexualCategory = -1
	result.SelfHarmCategory = -1
	result.ViolenceCategory = -1

	// Extract optional jsonPath parameter
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		if jsonPath, ok := jsonPathRaw.(string); ok {
			result.JsonPath = jsonPath
		} else {
			return result, fmt.Errorf("'jsonPath' must be a string")
		}
	}

	// Extract optional passthroughOnError parameter
	if passthroughOnErrorRaw, ok := params["passthroughOnError"]; ok {
		if passthroughOnError, ok := passthroughOnErrorRaw.(bool); ok {
			result.PassthroughOnError = passthroughOnError
		} else {
			return result, fmt.Errorf("'passthroughOnError' must be a boolean")
		}
	}

	// Extract optional showAssessment parameter
	if showAssessmentRaw, ok := params["showAssessment"]; ok {
		if showAssessment, ok := showAssessmentRaw.(bool); ok {
			result.ShowAssessment = showAssessment
		} else {
			return result, fmt.Errorf("'showAssessment' must be a boolean")
		}
	}

	// Extract optional category thresholds
	categories := []struct {
		name  string
		value *int
	}{
		{"hateCategory", &result.HateCategory},
		{"sexualCategory", &result.SexualCategory},
		{"selfHarmCategory", &result.SelfHarmCategory},
		{"violenceCategory", &result.ViolenceCategory},
	}

	for _, cat := range categories {
		if catRaw, ok := params[cat.name]; ok {
			catValue, err := extractInt(catRaw)
			if err != nil {
				return result, fmt.Errorf("'%s' must be a number: %w", cat.name, err)
			}
			if catValue < -1 || catValue > 7 {
				return result, fmt.Errorf("'%s' must be between -1 and 7", cat.name)
			}
			*cat.value = catValue
		}
	}

	return result, nil
}

// extractInt safely extracts an integer from various types
func extractInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("expected an integer but got %v", v)
		}
		return int(v), nil
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, err
		}
		if parsed != float64(int(parsed)) {
			return 0, fmt.Errorf("expected an integer but got %v", v)
		}
		return int(parsed), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// getStringParam safely extracts a string parameter
func getStringParam(params map[string]interface{}, key string) string {
	if val, ok := params[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// validateAzureConfigParams validates Azure configuration parameters (from params)
func validateAzureConfigParams(params map[string]interface{}) error {
	// Validate azureContentSafetyEndpoint (required)
	endpointRaw, ok := params["azureContentSafetyEndpoint"]
	if !ok {
		return fmt.Errorf("'azureContentSafetyEndpoint' parameter is required")
	}
	endpoint, ok := endpointRaw.(string)
	if !ok {
		return fmt.Errorf("'azureContentSafetyEndpoint' must be a string")
	}
	if endpoint == "" {
		return fmt.Errorf("'azureContentSafetyEndpoint' cannot be empty")
	}

	// Validate azureContentSafetyKey (required)
	apiKeyRaw, ok := params["azureContentSafetyKey"]
	if !ok {
		return fmt.Errorf("'azureContentSafetyKey' parameter is required")
	}
	apiKey, ok := apiKeyRaw.(string)
	if !ok {
		return fmt.Errorf("'azureContentSafetyKey' must be a string")
	}
	if apiKey == "" {
		return fmt.Errorf("'azureContentSafetyKey' cannot be empty")
	}

	return nil
}

// Mode returns the processing mode for this policy
func (p *AzureContentSafetyContentModerationPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest validates request body content
func (p *AzureContentSafetyContentModerationPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if !p.hasRequestParams {
		return policy.UpstreamRequestModifications{}
	}

	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, p.requestParams, false).(policy.RequestAction)
}

// OnResponse validates response body content
func (p *AzureContentSafetyContentModerationPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	if !p.hasResponseParams {
		return policy.UpstreamResponseModifications{}
	}

	var content []byte
	if ctx.ResponseBody != nil {
		content = ctx.ResponseBody.Content
	}
	return p.validatePayload(content, p.responseParams, true).(policy.ResponseAction)
}

// validatePayload validates payload against Azure Content Safety
func (p *AzureContentSafetyContentModerationPolicy) validatePayload(payload []byte, params AzureContentSafetyPolicyParams, isResponse bool) interface{} {
	// Build category thresholds from params
	categoryMap := p.buildCategoryMap(params)
	categories := p.getValidCategories(categoryMap)

	if len(categories) == 0 {
		slog.Debug("AzureContentSafety: No valid categories configured, passing through", "isResponse", isResponse)
		// No valid categories, pass through
		if isResponse {
			return policy.UpstreamResponseModifications{}
		}
		return policy.UpstreamRequestModifications{}
	}

	if payload == nil {
		if isResponse {
			return policy.UpstreamResponseModifications{}
		}
		return policy.UpstreamRequestModifications{}
	}

	// Extract value using JSONPath
	extractedValue, err := utils.ExtractStringValueFromJsonpath(payload, params.JsonPath)
	if err != nil {
		if params.PassthroughOnError {
			slog.Debug("AzureContentSafety: JSONPath extraction error, passthrough enabled", "jsonPath", params.JsonPath, "error", err, "isResponse", isResponse)
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		slog.Debug("AzureContentSafety: Error extracting value from JSONPath", "jsonPath", params.JsonPath, "error", err, "isResponse", isResponse)
		return p.buildErrorResponse("Error extracting value from JSONPath", err, isResponse, params.ShowAssessment, nil, "")
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Call Azure Content Safety API
	categoriesAnalysis, err := p.callAzureContentSafetyAPI(p.endpoint, p.apiKey, extractedValue, categories)
	if err != nil {
		if params.PassthroughOnError {
			slog.Debug("AzureContentSafety: API call error, passthrough enabled", "error", err, "isResponse", isResponse)
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		slog.Debug("AzureContentSafety: Error calling Azure Content Safety API", "error", err, "isResponse", isResponse)
		return p.buildErrorResponse("Error calling Azure Content Safety API", err, isResponse, params.ShowAssessment, nil, "")
	}

	// Check for violations
	for _, analysis := range categoriesAnalysis {
		category, _ := analysis["category"].(string)
		severityFloat, _ := analysis["severity"].(float64)
		severity := int(severityFloat)
		threshold := categoryMap[category]

		if threshold >= 0 && severity >= threshold {
			slog.Debug("AzureContentSafety: Violation detected", "category", category, "severity", severity, "threshold", threshold, "isResponse", isResponse)
			// Violation detected
			return p.buildErrorResponse("Violation of Azure content safety content moderation detected", nil, isResponse, params.ShowAssessment, categoriesAnalysis, extractedValue)
		}
	}

	slog.Debug("AzureContentSafety: Validation passed", "categoryCount", len(categoriesAnalysis), "isResponse", isResponse)

	// No violations, continue
	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// buildCategoryMap builds category threshold map from parameters
func (p *AzureContentSafetyContentModerationPolicy) buildCategoryMap(params AzureContentSafetyPolicyParams) map[string]int {
	return map[string]int{
		"Hate":     params.HateCategory,
		"Sexual":   params.SexualCategory,
		"SelfHarm": params.SelfHarmCategory,
		"Violence": params.ViolenceCategory,
	}
}

// getValidCategories returns list of valid categories (threshold between 0-7)
func (p *AzureContentSafetyContentModerationPolicy) getValidCategories(categoryMap map[string]int) []string {
	categories := []string{}
	for name, val := range categoryMap {
		if val >= 0 && val <= 7 {
			categories = append(categories, name)
		}
	}
	return categories
}

// callAzureContentSafetyAPI calls Azure Content Safety API
func (p *AzureContentSafetyContentModerationPolicy) callAzureContentSafetyAPI(endpoint, apiKey, text string, categories []string) ([]map[string]interface{}, error) {
	// Ensure endpoint doesn't end with /
	if strings.HasSuffix(endpoint, "/") {
		endpoint = strings.TrimSuffix(endpoint, "/")
	}

	serviceURL := endpoint + endpointSuffix

	requestBody := map[string]interface{}{
		"text":               text,
		"categories":         categories,
		"haltOnBlocklistHit": true,
		"outputType":         "EightSeverityLevels",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	headers := map[string]string{
		"Content-Type":              "application/json",
		"Ocp-Apim-Subscription-Key": apiKey,
	}

	// Make HTTP request with retry
	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
		}

		resp, lastErr = p.makeHTTPRequest("POST", serviceURL, headers, bodyBytes)
		if lastErr != nil {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			break
		}

		// Read error response body before closing
		errorBodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if attempt == maxRetries-1 {
			// Last attempt, return error with body content
			return nil, fmt.Errorf("Azure Content Safety API returned non-200 status code: %d, body: %s", resp.StatusCode, string(errorBodyBytes))
		}
		// Not the last attempt, continue retrying
		resp = nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to call Azure Content Safety API after %d attempts: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	responseBody := make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	categoriesAnalysisRaw, ok := responseBody["categoriesAnalysis"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("categoriesAnalysis missing or invalid in Azure Content Safety API response")
	}

	// Convert []interface{} to []map[string]interface{}
	var categoriesAnalysis []map[string]interface{}
	for _, item := range categoriesAnalysisRaw {
		if analysis, ok := item.(map[string]interface{}); ok {
			categoriesAnalysis = append(categoriesAnalysis, analysis)
		}
	}

	return categoriesAnalysis, nil
}

// makeHTTPRequest makes an HTTP request
func (p *AzureContentSafetyContentModerationPolicy) makeHTTPRequest(method, url string, headers map[string]string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// buildErrorResponse builds an error response for both request and response phases
func (p *AzureContentSafetyContentModerationPolicy) buildErrorResponse(reason string, validationError error, isResponse bool, showAssessment bool, categoriesAnalysis []map[string]interface{}, inspectedContent string) interface{} {
	assessment := p.buildAssessmentObject(reason, validationError, isResponse, showAssessment, categoriesAnalysis, inspectedContent)

	responseBody := map[string]interface{}{
		"type":    "AZURE_CONTENT_SAFETY_CONTENT_MODERATION",
		"message": assessment,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(`{"type":"AZURE_CONTENT_SAFETY_CONTENT_MODERATION","message":"Internal error"}`)
	}

	if isResponse {
		statusCode := GuardrailErrorCode
		return policy.UpstreamResponseModifications{
			StatusCode: &statusCode,
			Body:       bodyBytes,
			SetHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		}
	}

	return policy.ImmediateResponse{
		StatusCode: GuardrailErrorCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
}

// buildAssessmentObject builds the assessment object
func (p *AzureContentSafetyContentModerationPolicy) buildAssessmentObject(reason string, validationError error, isResponse bool, showAssessment bool, categoriesAnalysis []map[string]interface{}, inspectedContent string) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "AzureContentSafetyContentModeration",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if validationError != nil {
		assessment["actionReason"] = reason
	} else {
		assessment["actionReason"] = "Violation of Azure content safety content moderation detected."
	}

	if showAssessment {
		if validationError != nil {
			assessment["assessments"] = []string{validationError.Error()}
		} else if len(categoriesAnalysis) > 0 {
			assessmentsWrapper := map[string]interface{}{
				"inspectedContent": inspectedContent,
			}

			var assessmentsArray []map[string]interface{}
			for _, analysis := range categoriesAnalysis {
				category, _ := analysis["category"].(string)
				severityFloat, _ := analysis["severity"].(float64)
				severity := int(severityFloat)

				categoryAssessment := map[string]interface{}{
					"category": category,
					"severity": severity,
					"result":   "FAIL", // If we're here, it's a violation
				}
				assessmentsArray = append(assessmentsArray, categoryAssessment)
			}

			assessmentsWrapper["categories"] = assessmentsArray
			assessment["assessments"] = assessmentsWrapper
		}
	}

	return assessment
}
