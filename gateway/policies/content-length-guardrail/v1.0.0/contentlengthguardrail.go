package contentlengthguardrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	GuardrailErrorCode         = 446
	GuardrailAPIMExceptionCode = 900514
	TextCleanRegex             = "^\"|\"$"
)

var textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)

// ContentLengthGuardrailPolicy implements content length validation
type ContentLengthGuardrailPolicy struct{}

// NewPolicy creates a new ContentLengthGuardrailPolicy instance
func NewPolicy() policy.Policy {
	return &ContentLengthGuardrailPolicy{}
}

// Mode returns the processing mode for this policy
func (p *ContentLengthGuardrailPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest validates request body content length
func (p *ContentLengthGuardrailPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Extract request-specific parameters
	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate parameters
	if err := p.validateParams(requestParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), false, false, 0, 0).(policy.RequestAction)
	}

	return p.validatePayload(ctx.Body.Content, requestParams, false).(policy.RequestAction)
}

// OnResponse validates response body content length
func (p *ContentLengthGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Extract response-specific parameters
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate parameters
	if err := p.validateParams(responseParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), true, false, 0, 0).(policy.ResponseAction)
	}

	return p.validatePayloadResponse(ctx.ResponseBody.Content, responseParams, true).(policy.ResponseAction)
}

// validateParams validates the actual policy parameters
func (p *ContentLengthGuardrailPolicy) validateParams(params map[string]interface{}) error {
	// Validate min parameter
	minRaw, ok := params["min"]
	if !ok {
		return fmt.Errorf("'min' parameter is required")
	}
	min, ok := minRaw.(float64)
	if !ok {
		if minInt, ok := minRaw.(int); ok {
			min = float64(minInt)
		} else if minStr, ok := minRaw.(string); ok {
			var err error
			min, err = strconv.ParseFloat(minStr, 64)
			if err != nil {
				return fmt.Errorf("'min' must be a number")
			}
		} else {
			return fmt.Errorf("'min' must be a number")
		}
	}
	if min < 0 {
		return fmt.Errorf("'min' cannot be negative")
	}

	// Validate max parameter
	maxRaw, ok := params["max"]
	if !ok {
		return fmt.Errorf("'max' parameter is required")
	}
	max, ok := maxRaw.(float64)
	if !ok {
		if maxInt, ok := maxRaw.(int); ok {
			max = float64(maxInt)
		} else if maxStr, ok := maxRaw.(string); ok {
			var err error
			max, err = strconv.ParseFloat(maxStr, 64)
			if err != nil {
				return fmt.Errorf("'max' must be a number")
			}
		} else {
			return fmt.Errorf("'max' must be a number")
		}
	}
	if max <= 0 {
		return fmt.Errorf("'max' must be greater than 0")
	}
	if min > max {
		return fmt.Errorf("'min' cannot be greater than 'max'")
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

// validatePayload validates payload content length (request phase)
func (p *ContentLengthGuardrailPolicy) validatePayload(payload []byte, params map[string]interface{}, isResponse bool) interface{} {
	jsonPath, _ := params["jsonPath"].(string)
	invert, _ := params["invert"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	// Extract min and max
	min := 0
	max := 0
	if minRaw, ok := params["min"]; ok {
		if minFloat, ok := minRaw.(float64); ok {
			min = int(minFloat)
		} else if minInt, ok := minRaw.(int); ok {
			min = minInt
		}
	}
	if maxRaw, ok := params["max"]; ok {
		if maxFloat, ok := maxRaw.(float64); ok {
			max = int(maxFloat)
		} else if maxInt, ok := maxRaw.(int); ok {
			max = maxInt
		}
	}

	// Validate range
	if min > max || min < 0 || max <= 0 {
		return p.buildErrorResponse("invalid content length range", isResponse, showAssessment, min, max)
	}

	if payload == nil {
		return p.buildErrorResponse("body is empty", isResponse, showAssessment, min, max)
	}

	// Extract value using JSONPath
	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err), isResponse, showAssessment, min, max)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Count bytes
	byteCount := len([]byte(extractedValue))

	// Check if within range
	isWithinRange := byteCount >= min && byteCount <= max

	var validationPassed bool
	if invert {
		validationPassed = !isWithinRange // Inverted: pass if NOT in range
	} else {
		validationPassed = isWithinRange // Normal: pass if in range
	}

	if !validationPassed {
		var reason string
		if invert {
			reason = fmt.Sprintf("content length %d bytes is within the excluded range %d-%d bytes", byteCount, min, max)
		} else {
			reason = fmt.Sprintf("content length %d bytes is outside the allowed range %d-%d bytes", byteCount, min, max)
		}
		return p.buildErrorResponse(reason, isResponse, showAssessment, min, max)
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// validatePayloadResponse validates payload content length (response phase)
func (p *ContentLengthGuardrailPolicy) validatePayloadResponse(payload []byte, params map[string]interface{}, isResponse bool) interface{} {
	jsonPath, _ := params["jsonPath"].(string)
	invert, _ := params["invert"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	// Extract min and max
	min := 0
	max := 0
	if minRaw, ok := params["min"]; ok {
		if minFloat, ok := minRaw.(float64); ok {
			min = int(minFloat)
		} else if minInt, ok := minRaw.(int); ok {
			min = minInt
		}
	}
	if maxRaw, ok := params["max"]; ok {
		if maxFloat, ok := maxRaw.(float64); ok {
			max = int(maxFloat)
		} else if maxInt, ok := maxRaw.(int); ok {
			max = maxInt
		}
	}

	// Validate range
	if min > max || min < 0 || max <= 0 {
		return p.buildErrorResponse("invalid content length range", isResponse, showAssessment, min, max)
	}

	if payload == nil {
		return p.buildErrorResponse("body is empty", isResponse, showAssessment, min, max)
	}

	// Extract value using JSONPath
	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err), isResponse, showAssessment, min, max)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Count bytes
	byteCount := len([]byte(extractedValue))

	// Check if within range
	isWithinRange := byteCount >= min && byteCount <= max

	var validationPassed bool
	if invert {
		validationPassed = !isWithinRange // Inverted: pass if NOT in range
	} else {
		validationPassed = isWithinRange // Normal: pass if in range
	}

	if !validationPassed {
		var reason string
		if invert {
			reason = fmt.Sprintf("content length %d bytes is within the excluded range %d-%d bytes", byteCount, min, max)
		} else {
			reason = fmt.Sprintf("content length %d bytes is outside the allowed range %d-%d bytes", byteCount, min, max)
		}
		return p.buildErrorResponse(reason, isResponse, showAssessment, min, max)
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// buildErrorResponse builds an error response for both request and response phases
func (p *ContentLengthGuardrailPolicy) buildErrorResponse(reason string, isResponse bool, showAssessment bool, min, max int) interface{} {
	assessment := p.buildAssessmentObject(isResponse, reason, showAssessment, min, max)

	responseBody := map[string]interface{}{
		"code":    GuardrailAPIMExceptionCode,
		"type":    "CONTENT_LENGTH_GUARDRAIL",
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
func (p *ContentLengthGuardrailPolicy) buildAssessmentObject(isResponse bool, reason string, showAssessment bool, min, max int) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "ContentLengthGuardrail",
		"actionReason":         "Violation of applied content length constraints detected.",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if showAssessment {
		var assessmentMessage string
		if strings.Contains(reason, "excluded range") {
			assessmentMessage = fmt.Sprintf("Violation of content length detected. Expected content length to be outside the range of %d to %d bytes.", min, max)
		} else {
			assessmentMessage = fmt.Sprintf("Violation of content length detected. Expected content length to be between %d and %d bytes.", min, max)
		}
		assessment["assessments"] = assessmentMessage
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
