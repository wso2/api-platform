package regexguardrail

import (
	"encoding/json"
	"fmt"
	"regexp"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
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
		return p.buildErrorResponse("Parameter validation failed", err, false, false).(policy.RequestAction)
	}

	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, requestParams, false).(policy.RequestAction)
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
		return p.buildErrorResponse("Parameter validation failed", err, true, false).(policy.ResponseAction)
	}

	var content []byte
	if ctx.ResponseBody != nil {
		content = ctx.ResponseBody.Content
	}
	return p.validatePayload(content, responseParams, true).(policy.ResponseAction)
}

// validatePayload validates payload against regex pattern
func (p *RegexGuardrailPolicy) validatePayload(payload []byte, params map[string]interface{}, isResponse bool) interface{} {
	regexPattern, _ := params["regex"].(string)
	jsonPath, _ := params["jsonPath"].(string)
	invert, _ := params["invert"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	// Extract value using JSONPath
	extractedValue, err := utils.ExtractStringValueFromJsonpath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse("Error extracting value from JSONPath", err, isResponse, showAssessment)
	}

	// Compile regex pattern
	compiledRegex, _ := regexp.Compile(regexPattern)
	matched := compiledRegex.MatchString(extractedValue)

	// Apply inversion logic
	var validationPassed bool
	if invert {
		validationPassed = !matched // Inverted: pass if NOT matched
	} else {
		validationPassed = matched // Normal: pass if matched
	}

	if !validationPassed {
		return p.buildErrorResponse("Violated regular expression: "+regexPattern, nil, isResponse, showAssessment)
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// buildErrorResponse builds an error response for both request and response phases
func (p *RegexGuardrailPolicy) buildErrorResponse(reason string, validationError error, isResponse bool, showAssessment bool) interface{} {
	assessment := p.buildAssessmentObject(reason, validationError, isResponse, showAssessment)

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
func (p *RegexGuardrailPolicy) buildAssessmentObject(reason string, validationError error, isResponse bool, showAssessment bool) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "RegexGuardrail",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if validationError != nil {
		assessment["actionReason"] = reason
		if showAssessment {
			assessment["assessments"] = []string{validationError.Error()}
		}
	} else {
		assessment["actionReason"] = "Violation of regular expression detected."
		if showAssessment {
			assessment["assessments"] = reason
		}
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

	_, err := regexp.Compile(regexPattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
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
