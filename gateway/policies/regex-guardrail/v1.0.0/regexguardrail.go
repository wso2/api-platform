package regexguardrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	GuardrailErrorCode         = 446
	GuardrailAPIMExceptionCode = 900514
)

// RegexGuardrailPolicy implements regex-based content validation
type RegexGuardrailPolicy struct{}

// NewPolicy creates a new RegexGuardrailPolicy instance
func NewPolicy() policy.Policy {
	return &RegexGuardrailPolicy{}
}

// Mode returns the processing mode for this policy
func (p *RegexGuardrailPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer, // Need full body for validation
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer, // Need full body for validation
	}
}

// OnRequest validates request body against regex pattern
func (p *RegexGuardrailPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Extract request-specific parameters
	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate parameters
	if err := p.validateParams(requestParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), false, false).(policy.RequestAction)
	}

	regexPattern, _ := requestParams["regex"].(string)
	jsonPath, _ := requestParams["jsonPath"].(string)
	invert, _ := requestParams["invert"].(bool)
	showAssessment, _ := requestParams["showAssessment"].(bool)

	// Compile regex pattern
	compiledRegex, err := regexp.Compile(regexPattern)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("invalid regex pattern: %v", err), false, showAssessment).(policy.RequestAction)
	}

	// Extract value from payload using JSONPath
	if ctx.Body == nil || ctx.Body.Content == nil {
		return p.buildErrorResponse("request body is empty", false, showAssessment).(policy.RequestAction)
	}
	payload := ctx.Body.Content

	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err), false, showAssessment).(policy.RequestAction)
	}

	// Perform regex matching
	matched := compiledRegex.MatchString(extractedValue)

	// Apply inversion logic
	var validationPassed bool
	if invert {
		validationPassed = !matched // Inverted: pass if NOT matched
	} else {
		validationPassed = matched // Normal: pass if matched
	}

	if !validationPassed {
		return p.buildErrorResponse("regex validation failed", false, showAssessment).(policy.RequestAction)
	}

	// Validation passed, continue to upstream
	return policy.UpstreamRequestModifications{}
}

// OnResponse validates response body against regex pattern
func (p *RegexGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Extract response-specific parameters
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate parameters
	if err := p.validateParams(responseParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), true, false).(policy.ResponseAction)
	}

	regexPattern, _ := responseParams["regex"].(string)
	jsonPath, _ := responseParams["jsonPath"].(string)
	invert, _ := responseParams["invert"].(bool)
	showAssessment, _ := responseParams["showAssessment"].(bool)

	// Compile regex pattern
	compiledRegex, err := regexp.Compile(regexPattern)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("invalid regex pattern: %v", err), true, showAssessment).(policy.ResponseAction)
	}

	// Extract value from payload using JSONPath
	if ctx.ResponseBody == nil || ctx.ResponseBody.Content == nil {
		return p.buildErrorResponse("response body is empty", true, showAssessment).(policy.ResponseAction)
	}
	payload := ctx.ResponseBody.Content

	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err), true, showAssessment).(policy.ResponseAction)
	}

	// Perform regex matching
	matched := compiledRegex.MatchString(extractedValue)

	// Apply inversion logic
	var validationPassed bool
	if invert {
		validationPassed = !matched // Inverted: pass if NOT matched
	} else {
		validationPassed = matched // Normal: pass if matched
	}

	if !validationPassed {
		return p.buildErrorResponse("regex validation failed", true, showAssessment).(policy.ResponseAction)
	}

	// Validation passed, continue
	return policy.UpstreamResponseModifications{}
}

// buildErrorResponse builds an error response for both request and response phases
func (p *RegexGuardrailPolicy) buildErrorResponse(reason string, isResponse bool, showAssessment bool) interface{} {
	assessment := p.buildAssessmentObject(isResponse, reason, showAssessment)

	responseBody := map[string]interface{}{
		"code":    GuardrailAPIMExceptionCode,
		"type":    "REGEX_GUARDRAIL",
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
func (p *RegexGuardrailPolicy) buildAssessmentObject(isResponse bool, reason string, showAssessment bool) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "RegexGuardrail",
		"actionReason":         reason,
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if showAssessment {
		assessment["assessments"] = reason
	}

	return assessment
}

// validateParams validates the actual policy parameters
func (p *RegexGuardrailPolicy) validateParams(params map[string]interface{}) error {
	// Validate regex parameter (required)
	regexRaw, ok := params["regex"]
	if !ok {
		return fmt.Errorf("'regex' parameter is required")
	}
	regexPattern, ok := regexRaw.(string)
	if !ok {
		return fmt.Errorf("'regex' must be a string")
	}
	if regexPattern == "" {
		return fmt.Errorf("'regex' cannot be empty")
	}

	// Validate optional parameters
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		_, ok := jsonPathRaw.(string)
		if !ok {
			return fmt.Errorf("'jsonPath' must be a string")
		}
	}

	if invertRaw, ok := params["invert"]; ok {
		_, ok := invertRaw.(bool)
		if !ok {
			return fmt.Errorf("'invert' must be a boolean")
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
		return fmt.Sprintf("%.0f", v), nil
	case int:
		return fmt.Sprintf("%d", v), nil
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

	// Handle array indexing (e.g., "items[0]")
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
