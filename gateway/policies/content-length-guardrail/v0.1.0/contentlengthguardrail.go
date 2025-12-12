package contentlengthguardrail

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

const (
	GuardrailErrorCode = 422
	TextCleanRegex     = "^\"|\"$"
)

var textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)

// ContentLengthGuardrailPolicy implements content length validation
type ContentLengthGuardrailPolicy struct {
	hasRequestParams  bool
	hasResponseParams bool
	requestParams     ContentLengthGuardrailPolicyParams
	responseParams    ContentLengthGuardrailPolicyParams
}

type ContentLengthGuardrailPolicyParams struct {
	Min            int
	Max            int
	JsonPath       string
	Invert         bool
	ShowAssessment bool
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &ContentLengthGuardrailPolicy{}

	// Extract and parse request parameters if present
	if requestParamsRaw, ok := params["request"].(map[string]interface{}); ok {
		requestParams, err := parseParams(requestParamsRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid request parameters: %w", err)
		}
		p.hasRequestParams = true
		p.requestParams = requestParams
	}

	// Extract and parse response parameters if present
	if responseParamsRaw, ok := params["response"].(map[string]interface{}); ok {
		responseParams, err := parseParams(responseParamsRaw)
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

	return p, nil
}

// parseParams parses and validates parameters from map to struct
func parseParams(params map[string]interface{}) (ContentLengthGuardrailPolicyParams, error) {
	var result ContentLengthGuardrailPolicyParams

	// Validate and extract min parameter (required)
	minRaw, ok := params["min"]
	if !ok {
		return result, fmt.Errorf("'min' parameter is required")
	}
	min, err := extractInt(minRaw)
	if err != nil {
		return result, fmt.Errorf("'min' must be a number: %w", err)
	}
	if min < 0 {
		return result, fmt.Errorf("'min' cannot be negative")
	}
	result.Min = min

	// Validate and extract max parameter (required)
	maxRaw, ok := params["max"]
	if !ok {
		return result, fmt.Errorf("'max' parameter is required")
	}
	max, err := extractInt(maxRaw)
	if err != nil {
		return result, fmt.Errorf("'max' must be a number: %w", err)
	}
	if max <= 0 {
		return result, fmt.Errorf("'max' must be greater than 0")
	}
	if min > max {
		return result, fmt.Errorf("'min' cannot be greater than 'max'")
	}
	result.Max = max

	// Extract optional jsonPath parameter
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		if jsonPath, ok := jsonPathRaw.(string); ok {
			result.JsonPath = jsonPath
		} else {
			return result, fmt.Errorf("'jsonPath' must be a string")
		}
	}

	// Extract optional invert parameter
	if invertRaw, ok := params["invert"]; ok {
		if invert, ok := invertRaw.(bool); ok {
			result.Invert = invert
		} else {
			return result, fmt.Errorf("'invert' must be a boolean")
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
	if !p.hasRequestParams {
		return policy.UpstreamRequestModifications{}
	}

	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, p.requestParams, false).(policy.RequestAction)
}

// OnResponse validates response body content length
func (p *ContentLengthGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	if !p.hasResponseParams {
		return policy.UpstreamResponseModifications{}
	}

	content := []byte{}
	if ctx.ResponseBody != nil {
		content = ctx.ResponseBody.Content
	}
	return p.validatePayload(content, p.responseParams, true).(policy.ResponseAction)
}

// validatePayload validates payload content length (request phase)
func (p *ContentLengthGuardrailPolicy) validatePayload(payload []byte, params ContentLengthGuardrailPolicyParams, isResponse bool) interface{} {
	// Extract value using JSONPath
	extractedValue, err := utils.ExtractStringValueFromJsonpath(payload, params.JsonPath)
	if err != nil {
		return p.buildErrorResponse("Error extracting value from JSONPath", err, isResponse, params.ShowAssessment, params.Min, params.Max)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Count bytes
	byteCount := len([]byte(extractedValue))

	// Check if within range
	isWithinRange := byteCount >= params.Min && byteCount <= params.Max

	var validationPassed bool
	if params.Invert {
		validationPassed = !isWithinRange // Inverted: pass if NOT in range
	} else {
		validationPassed = isWithinRange // Normal: pass if in range
	}

	if !validationPassed {
		var reason string
		if params.Invert {
			reason = fmt.Sprintf("content length %d bytes is within the excluded range %d-%d bytes", byteCount, params.Min, params.Max)
		} else {
			reason = fmt.Sprintf("content length %d bytes is outside the allowed range %d-%d bytes", byteCount, params.Min, params.Max)
		}
		return p.buildErrorResponse(reason, nil, isResponse, params.ShowAssessment, params.Min, params.Max)
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// buildErrorResponse builds an error response for both request and response phases
func (p *ContentLengthGuardrailPolicy) buildErrorResponse(reason string, validationError error, isResponse bool, showAssessment bool, min, max int) interface{} {
	assessment := p.buildAssessmentObject(reason, validationError, isResponse, showAssessment, min, max)

	responseBody := map[string]interface{}{
		"type":    "CONTENT_LENGTH_GUARDRAIL",
		"message": assessment,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(`{"type":"CONTENT_LENGTH_GUARDRAIL","message":"Internal error"}`)
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
func (p *ContentLengthGuardrailPolicy) buildAssessmentObject(reason string, validationError error, isResponse bool, showAssessment bool, min, max int) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "ContentLengthGuardrail",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if validationError != nil {
		assessment["actionReason"] = reason
	} else {
		assessment["actionReason"] = "Violation of applied content length constraints detected."
	}

	if showAssessment {
		if validationError != nil {
			assessment["assessments"] = validationError.Error()
		} else {
			var assessmentMessage string
			if strings.Contains(reason, "excluded range") {
				assessmentMessage = fmt.Sprintf("Violation of content length detected. Expected content length to be outside the range of %d to %d bytes.", min, max)
			} else {
				assessmentMessage = fmt.Sprintf("Violation of content length detected. Expected content length to be between %d and %d bytes.", min, max)
			}
			assessment["assessments"] = assessmentMessage
		}
	}

	return assessment
}
