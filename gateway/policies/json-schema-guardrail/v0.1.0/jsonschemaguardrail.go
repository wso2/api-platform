package jsonschemaguardrail

import (
	"encoding/json"
	"fmt"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
	"github.com/xeipuuv/gojsonschema"
)

const (
	GuardrailErrorCode = 422
)

// JSONSchemaGuardrailPolicy implements JSON schema validation
type JSONSchemaGuardrailPolicy struct {
	hasRequestParams  bool
	hasResponseParams bool
	requestParams     JSONSchemaGuardrailPolicyParams
	responseParams    JSONSchemaGuardrailPolicyParams
}

type JSONSchemaGuardrailPolicyParams struct {
	Schema         string
	JsonPath       string
	Invert         bool
	ShowAssessment bool
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &JSONSchemaGuardrailPolicy{}

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
func parseParams(params map[string]interface{}) (JSONSchemaGuardrailPolicyParams, error) {
	var result JSONSchemaGuardrailPolicyParams

	// Validate and extract schema parameter (required)
	schemaRaw, ok := params["schema"]
	if !ok {
		return result, fmt.Errorf("'schema' parameter is required")
	}
	schema, ok := schemaRaw.(string)
	if !ok {
		return result, fmt.Errorf("'schema' must be a string")
	}
	if schema == "" {
		return result, fmt.Errorf("'schema' cannot be empty")
	}

	// Validate schema is valid JSON
	var schemaJSON interface{}
	if err := json.Unmarshal([]byte(schema), &schemaJSON); err != nil {
		return result, fmt.Errorf("'schema' must be valid JSON: %v", err)
	}
	result.Schema = schema

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
	if !p.hasRequestParams {
		return policy.UpstreamRequestModifications{}
	}

	content := []byte{}
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, p.requestParams, false).(policy.RequestAction)
}

// OnResponse validates response body against JSON schema
func (p *JSONSchemaGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	if !p.hasResponseParams {
		return policy.UpstreamResponseModifications{}
	}

	content := []byte{}
	if ctx.ResponseBody != nil {
		content = ctx.ResponseBody.Content
	}
	return p.validatePayload(content, p.responseParams, true).(policy.ResponseAction)
}

// validatePayload validates payload against JSON schema
func (p *JSONSchemaGuardrailPolicy) validatePayload(payload []byte, params JSONSchemaGuardrailPolicyParams, isResponse bool) interface{} {
	// Parse schema
	schemaLoader := gojsonschema.NewStringLoader(params.Schema)

	// Extract value using JSONPath if specified
	var documentLoader gojsonschema.JSONLoader
	if params.JsonPath != "" {
		extractedValue, err := extractValueFromJSONPathForSchema(payload, params.JsonPath)
		if err != nil {
			return p.buildErrorResponse("Error extracting value from JSONPath", err, isResponse, params.ShowAssessment, nil)
		}
		documentLoader = gojsonschema.NewBytesLoader(extractedValue)
	} else {
		documentLoader = gojsonschema.NewBytesLoader(payload)
	}

	// Validate against schema
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return p.buildErrorResponse("Error validating schema", err, isResponse, params.ShowAssessment, nil)
	}

	// Apply inversion logic
	var validationPassed bool
	if params.Invert {
		validationPassed = !result.Valid() // Inverted: pass if NOT valid
	} else {
		validationPassed = result.Valid() // Normal: pass if valid
	}

	if !validationPassed {
		var reason string
		if params.Invert {
			reason = "JSON schema validation passed but invert is enabled"
		} else {
			reason = "JSON schema validation failed"
		}
		return p.buildErrorResponse(reason, nil, isResponse, params.ShowAssessment, result.Errors())
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// extractValueFromJSONPathForSchema extracts a value from JSON using JSONPath and returns as JSON bytes
func extractValueFromJSONPathForSchema(payload []byte, jsonPath string) ([]byte, error) {
	var any interface{}
	if err := json.Unmarshal(payload, &any); err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %w", err)
	}
	jsonData, ok := any.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("jsonPath extraction requires a JSON object payload (got %T)", any)
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
		"type":    "JSON_SCHEMA_GUARDRAIL",
		"message": assessment,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		// Fallback to minimal error response
		bodyBytes = []byte(`{"type":"JSON_SCHEMA_GUARDRAIL","message":"Internal error"}`)
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
			assessment["assessments"] = validationError.Error()
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
