package azurecontentsafetycontentmoderation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	GuardrailErrorCode         = 446
	GuardrailAPIMExceptionCode = 900514
	TextCleanRegex             = "^\"|\"$"
	endpointSuffix             = "/contentsafety/text:analyze?api-version=2024-09-01"
	requestTimeout             = 30 * time.Second
	maxRetries                 = 5
	retryDelay                 = 1 * time.Second
)

var textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)

// AzureContentSafetyContentModerationPolicy implements Azure Content Safety content moderation
type AzureContentSafetyContentModerationPolicy struct{}

// NewPolicy creates a new AzureContentSafetyContentModerationPolicy instance
func NewPolicy() policy.Policy {
	return &AzureContentSafetyContentModerationPolicy{}
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

	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate parameters
	if err := p.validateParams(requestParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), false, false, nil).(policy.RequestAction)
	}

	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, requestParams, false).(policy.RequestAction)
}

// OnResponse validates response body content
func (p *AzureContentSafetyContentModerationPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate parameters
	if err := p.validateParams(responseParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), true, false, nil).(policy.ResponseAction)
	}

	return p.validatePayload(ctx.ResponseBody.Content, responseParams, true).(policy.ResponseAction)
}

// validateParams validates the actual policy parameters
func (p *AzureContentSafetyContentModerationPolicy) validateParams(params map[string]interface{}) error {
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

	// Validate category thresholds (optional, -1 to 7)
	categories := []string{"hateCategory", "sexualCategory", "selfHarmCategory", "violenceCategory"}
	for _, catName := range categories {
		if catRaw, ok := params[catName]; ok {
			cat, ok := catRaw.(float64)
			if !ok {
				if catInt, ok := catRaw.(int); ok {
					cat = float64(catInt)
				} else if catStr, ok := catRaw.(string); ok {
					var err error
					cat, err = strconv.ParseFloat(catStr, 64)
					if err != nil {
						return fmt.Errorf("'%s' must be a number", catName)
					}
				} else {
					return fmt.Errorf("'%s' must be a number", catName)
				}
			}
			if cat < -1 || cat > 7 {
				return fmt.Errorf("'%s' must be between -1 and 7", catName)
			}
		}
	}

	// Validate optional parameters
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		_, ok := jsonPathRaw.(string)
		if !ok {
			return fmt.Errorf("'jsonPath' must be a string")
		}
	}

	if passthroughOnErrorRaw, ok := params["passthroughOnError"]; ok {
		_, ok := passthroughOnErrorRaw.(bool)
		if !ok {
			return fmt.Errorf("'passthroughOnError' must be a boolean")
		}
	}

	if showAssessmentRaw, ok := params["showAssessment"]; ok {
		_, ok := showAssessmentRaw.(bool)
		if !ok {
			return fmt.Errorf("'showAssessment' must be a boolean")
		}
	}

	return nil
}

// validatePayload validates payload against Azure Content Safety
func (p *AzureContentSafetyContentModerationPolicy) validatePayload(payload []byte, params map[string]interface{}, isResponse bool) interface{} {
	jsonPath, _ := params["jsonPath"].(string)
	passthroughOnError, _ := params["passthroughOnError"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	// Extract Azure configuration
	endpoint, _ := params["azureContentSafetyEndpoint"].(string)
	apiKey, _ := params["azureContentSafetyKey"].(string)

	if endpoint == "" || apiKey == "" {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse("azureContentSafetyEndpoint and azureContentSafetyKey are required", isResponse, showAssessment, nil)
	}

	// Extract category thresholds
	categoryMap := p.buildCategoryMap(params)
	categories := p.getValidCategories(categoryMap)

	if len(categories) == 0 {
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
	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err), isResponse, showAssessment, nil)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Call Azure Content Safety API
	categoriesAnalysis, err := p.callAzureContentSafetyAPI(endpoint, apiKey, extractedValue, categories)
	if err != nil {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse(fmt.Sprintf("error calling Azure Content Safety API: %v", err), isResponse, showAssessment, nil)
	}

	// Check for violations
	for _, analysis := range categoriesAnalysis {
		category, _ := analysis["category"].(string)
		severityFloat, _ := analysis["severity"].(float64)
		severity := int(severityFloat)
		threshold := categoryMap[category]

		if threshold >= 0 && severity >= threshold {
			// Violation detected
			return p.buildErrorResponse("violation of Azure content safety content moderation detected", isResponse, showAssessment, categoriesAnalysis)
		}
	}

	// No violations, continue
	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// buildCategoryMap builds category threshold map from parameters
func (p *AzureContentSafetyContentModerationPolicy) buildCategoryMap(params map[string]interface{}) map[string]int {
	categoryMap := map[string]int{
		"Hate":     -1,
		"Sexual":   -1,
		"SelfHarm": -1,
		"Violence": -1,
	}

	if hateRaw, ok := params["hateCategory"]; ok {
		if hateFloat, ok := hateRaw.(float64); ok {
			categoryMap["Hate"] = int(hateFloat)
		} else if hateInt, ok := hateRaw.(int); ok {
			categoryMap["Hate"] = hateInt
		}
	}

	if sexualRaw, ok := params["sexualCategory"]; ok {
		if sexualFloat, ok := sexualRaw.(float64); ok {
			categoryMap["Sexual"] = int(sexualFloat)
		} else if sexualInt, ok := sexualRaw.(int); ok {
			categoryMap["Sexual"] = sexualInt
		}
	}

	if selfHarmRaw, ok := params["selfHarmCategory"]; ok {
		if selfHarmFloat, ok := selfHarmRaw.(float64); ok {
			categoryMap["SelfHarm"] = int(selfHarmFloat)
		} else if selfHarmInt, ok := selfHarmRaw.(int); ok {
			categoryMap["SelfHarm"] = selfHarmInt
		}
	}

	if violenceRaw, ok := params["violenceCategory"]; ok {
		if violenceFloat, ok := violenceRaw.(float64); ok {
			categoryMap["Violence"] = int(violenceFloat)
		} else if violenceInt, ok := violenceRaw.(int); ok {
			categoryMap["Violence"] = violenceInt
		}
	}

	return categoryMap
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
		if lastErr == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to call Azure Content Safety API after %d attempts: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure Content Safety API returned non-200 status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

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
	client := &http.Client{
		Timeout: requestTimeout,
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// buildErrorResponse builds an error response for both request and response phases
func (p *AzureContentSafetyContentModerationPolicy) buildErrorResponse(reason string, isResponse bool, showAssessment bool, categoriesAnalysis []map[string]interface{}) interface{} {
	assessment := p.buildAssessmentObject(isResponse, reason, showAssessment, categoriesAnalysis)

	responseBody := map[string]interface{}{
		"code":    GuardrailAPIMExceptionCode,
		"type":    "AZURE_CONTENT_SAFETY_CONTENT_MODERATION",
		"message": assessment,
	}

	bodyBytes, _ := json.Marshal(responseBody)

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
func (p *AzureContentSafetyContentModerationPolicy) buildAssessmentObject(isResponse bool, reason string, showAssessment bool, categoriesAnalysis []map[string]interface{}) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "AzureContentSafetyContentModeration",
		"actionReason":         "Violation of Azure content safety content moderation detected.",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if showAssessment && len(categoriesAnalysis) > 0 {
		assessmentsWrapper := map[string]interface{}{
			"inspectedContent": reason,
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
	} else if showAssessment {
		assessment["assessments"] = categoriesAnalysis
	}

	return assessment
}

// extractStringValueFromJSONPath extracts a value from JSON using JSONPath
func extractStringValueFromJSONPath(payload []byte, jsonPath string) (string, error) {
	if jsonPath == "" {
		return string(payload), nil
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	value, err := extractValueFromJSONPath(jsonData, jsonPath)
	if err != nil {
		return "", err
	}

	// Convert to string
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// extractValueFromJSONPath extracts a value from a nested JSON structure based on a JSON path
func extractValueFromJSONPath(data map[string]interface{}, jsonPath string) (interface{}, error) {
	keys := strings.Split(jsonPath, ".")
	if len(keys) > 0 && keys[0] == "$" {
		keys = keys[1:]
	}

	return extractRecursive(data, keys)
}

func extractRecursive(current interface{}, keys []string) (interface{}, error) {
	if len(keys) == 0 {
		return current, nil
	}

	key := keys[0]
	remaining := keys[1:]

	// Handle array indexing
	arrayIndexRegex := regexp.MustCompile(`^([a-zA-Z0-9_]+)\[(-?\d+)\]$`)
	if matches := arrayIndexRegex.FindStringSubmatch(key); len(matches) == 3 {
		arrayName := matches[1]
		idxStr := matches[2]
		idx := 0
		fmt.Sscanf(idxStr, "%d", &idx)

		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idx < 0 {
						idx = len(arr) + idx
					}
					if idx < 0 || idx >= len(arr) {
						return nil, fmt.Errorf("array index out of range: %d", idx)
					}
					return extractRecursive(arr[idx], remaining)
				}
				return nil, fmt.Errorf("not an array: %s", arrayName)
			}
			return nil, fmt.Errorf("key not found: %s", arrayName)
		}
		return nil, fmt.Errorf("invalid structure for key: %s", arrayName)
	}

	// Handle wildcard
	if key == "*" {
		var results []interface{}
		switch node := current.(type) {
		case map[string]interface{}:
			for _, v := range node {
				res, err := extractRecursive(v, remaining)
				if err == nil {
					results = append(results, res)
				}
			}
		case []interface{}:
			for _, v := range node {
				res, err := extractRecursive(v, remaining)
				if err == nil {
					results = append(results, res)
				}
			}
		default:
			return nil, fmt.Errorf("wildcard used on non-iterable node")
		}
		return results, nil
	}

	// Regular object key
	if node, ok := current.(map[string]interface{}); ok {
		if val, exists := node[key]; exists {
			return extractRecursive(val, remaining)
		}
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return nil, fmt.Errorf("invalid structure for key: %s", key)
}
