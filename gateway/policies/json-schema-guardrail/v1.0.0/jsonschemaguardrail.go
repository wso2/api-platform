package jsonschemaguardrail

import (
	"encoding/json"
	"fmt"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
	"github.com/xeipuuv/gojsonschema"
)

const (
	GuardrailErrorCode         = 446
	GuardrailAPIMExceptionCode = 900514
)

// JSONSchemaGuardrailPolicy implements JSON schema validation
type JSONSchemaGuardrailPolicy struct{}

// NewPolicy creates a new JSONSchemaGuardrailPolicy instance
func NewPolicy(
	metadata policy.PolicyMetadata,
	initParams map[string]interface{},
	params map[string]interface{},
) (policy.Policy, error) {
	return &JSONSchemaGuardrailPolicy{}, nil
}

// Mode returns the processing mode for this policy
func (p *JSONSchemaGuardrailPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest validates request body against JSON schema
func (p *JSONSchemaGuardrailPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate parameters
	if err := p.validateParams(requestParams); err != nil {
		return p.buildErrorResponse("Parameter validation failed", err, false, false, nil).(policy.RequestAction)
	}

	content := []byte{}
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, requestParams, false).(policy.RequestAction)
}

// OnResponse validates response body against JSON schema
func (p *JSONSchemaGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate parameters
	if err := p.validateParams(responseParams); err != nil {
		return p.buildErrorResponse("Parameter validation failed", err, true, false, nil).(policy.ResponseAction)
	}

	content := []byte{}
	if ctx.ResponseBody != nil {
		content = ctx.ResponseBody.Content
	}
	return p.validatePayload(content, responseParams, true).(policy.ResponseAction)
}

// validateParams validates the actual policy parameters
func (p *JSONSchemaGuardrailPolicy) validateParams(params map[string]interface{}) error {
	// Validate schema parameter (required)
	schemaRaw, ok := params["schema"]
	if !ok {
		return fmt.Errorf("'schema' parameter is required")
	}
	schema, ok := schemaRaw.(string)
	if !ok {
		return fmt.Errorf("'schema' must be a string")
	}
	if schema == "" {
		return fmt.Errorf("'schema' cannot be empty")
	}

	// Validate schema is valid JSON
	var schemaJSON interface{}
	if err := json.Unmarshal([]byte(schema), &schemaJSON); err != nil {
		return fmt.Errorf("'schema' must be valid JSON: %v", err)
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

// validatePayload validates payload against JSON schema
func (p *JSONSchemaGuardrailPolicy) validatePayload(payload []byte, params map[string]interface{}, isResponse bool) interface{} {
	schemaRaw, _ := params["schema"].(string)
	jsonPath, _ := params["jsonPath"].(string)
	invert, _ := params["invert"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	// Parse schema
	schemaLoader := gojsonschema.NewStringLoader(schemaRaw)

	// Extract value using JSONPath if specified
	var documentLoader gojsonschema.JSONLoader
	if jsonPath != "" {
		extractedValue, err := extractValueFromJSONPathForSchema(payload, jsonPath)
		if err != nil {
			return p.buildErrorResponse("Error extracting value from JSONPath", err, isResponse, showAssessment, nil)
		}
		documentLoader = gojsonschema.NewBytesLoader(extractedValue)
	} else {
		documentLoader = gojsonschema.NewBytesLoader(payload)
	}

	// Validate against schema
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return p.buildErrorResponse("Error validating schema", err, isResponse, showAssessment, nil)
	}

	// Apply inversion logic
	var validationPassed bool
	if invert {
		validationPassed = !result.Valid() // Inverted: pass if NOT valid
	} else {
		validationPassed = result.Valid() // Normal: pass if valid
	}

	if !validationPassed {
		var reason string
		if invert {
			reason = "JSON schema validation passed but invert is enabled"
		} else {
			reason = "JSON schema validation failed"
		}
		return p.buildErrorResponse(reason, nil, isResponse, showAssessment, result.Errors())
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// extractValueFromJSONPathForSchema extracts a value from JSON using JSONPath and returns as JSON bytes
func extractValueFromJSONPathForSchema(payload []byte, jsonPath string) ([]byte, error) {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	value, err := utils.ExtractValueFromJsonpath(jsonData, jsonPath)
	if err != nil {
		return nil, err
	}

	// Marshal back to JSON
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("error marshaling extracted value: %w", err)
	}

	return valueBytes, nil
}

// buildErrorResponse builds an error response for both request and response phases
func (p *JSONSchemaGuardrailPolicy) buildErrorResponse(reason string, validationError error, isResponse bool, showAssessment bool, errors []gojsonschema.ResultError) interface{} {
	assessment := p.buildAssessmentObject(reason, validationError, isResponse, showAssessment, errors)

	responseBody := map[string]interface{}{
		"code":    GuardrailAPIMExceptionCode,
		"type":    "JSON_SCHEMA_GUARDRAIL",
		"message": assessment,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		// Fallback to minimal error response
		bodyBytes = []byte(fmt.Sprintf(`{"code":%d,"type":"JSON_SCHEMA_GUARDRAIL","message":"Internal error"}`, GuardrailAPIMExceptionCode))
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
func (p *JSONSchemaGuardrailPolicy) buildAssessmentObject(reason string, validationError error, isResponse bool, showAssessment bool, errors []gojsonschema.ResultError) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "JSONSchemaGuardrail",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if validationError != nil {
		assessment["actionReason"] = reason
	} else {
		assessment["actionReason"] = "Violation of JSON schema detected."
	}

	if showAssessment {
		if validationError != nil {
			assessment["assessments"] = []string{validationError.Error()}
		} else if len(errors) > 0 {
			errorDetails := make([]map[string]interface{}, 0, len(errors))
			for _, err := range errors {
				errorDetails = append(errorDetails, map[string]interface{}{
					"field":       err.Field(),
					"description": err.Description(),
					"value":       err.Value(),
				})
			}
			assessment["assessments"] = errorDetails
		}
	}

	return assessment
}
