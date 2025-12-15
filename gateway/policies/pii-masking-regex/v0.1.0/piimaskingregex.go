package piimaskingregex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

const (
	APIMInternalErrorCode     = 500
	APIMInternalExceptionCode = 900967
	TextCleanRegex            = "^\"|\"$"
	MetadataKeyPIIEntities    = "piimaskingregex:pii_entities"
)

var textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)

// PIIMaskingRegexPolicy implements regex-based PII masking
type PIIMaskingRegexPolicy struct {
	params PIIMaskingRegexPolicyParams
}

type PIIMaskingRegexPolicyParams struct {
	PIIEntities map[string]*regexp.Regexp
	JsonPath    string
	RedactPII   bool
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &PIIMaskingRegexPolicy{}

	// Parse parameters (piiEntities is required)
	policyParams, err := parseParams(params, true) // true = piiEntities is required
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	p.params = policyParams

	return p, nil
}

// parseParams parses and validates parameters from map to struct
// requirePIIEntities: true for request params (required), false for response params (optional)
func parseParams(params map[string]interface{}, requirePIIEntities bool) (PIIMaskingRegexPolicyParams, error) {
	var result PIIMaskingRegexPolicyParams

	// Validate and extract piiEntities parameter
	piiEntitiesRaw, ok := params["piiEntities"]
	if !ok && requirePIIEntities {
		return result, fmt.Errorf("'piiEntities' parameter is required")
	}
	if ok {
		// Parse PII entities
		var piiEntitiesArray []map[string]interface{}
		switch v := piiEntitiesRaw.(type) {
		case string:
			if err := json.Unmarshal([]byte(v), &piiEntitiesArray); err != nil {
				return result, fmt.Errorf("error unmarshaling PII entities: %w", err)
			}
		case []interface{}:
			piiEntitiesArray = make([]map[string]interface{}, 0, len(v))
			for idx, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					piiEntitiesArray = append(piiEntitiesArray, itemMap)
				} else {
					return result, fmt.Errorf("'piiEntities[%d]' must be an object", idx)
				}
			}
		default:
			return result, fmt.Errorf("'piiEntities' must be an array or JSON string")
		}

		// Validate each PII entity
		piiEntities := make(map[string]*regexp.Regexp)
		for i, entityConfig := range piiEntitiesArray {
			piiEntity, ok := entityConfig["piiEntity"].(string)
			if !ok || piiEntity == "" {
				return result, fmt.Errorf("'piiEntities[%d].piiEntity' is required and must be a non-empty string", i)
			}

			if !regexp.MustCompile(`^[A-Z_]+$`).MatchString(piiEntity) {
				return result, fmt.Errorf("'piiEntities[%d].piiEntity' must match ^[A-Z_]+$", i)
			}

			piiRegex, ok := entityConfig["piiRegex"].(string)
			if !ok || piiRegex == "" {
				return result, fmt.Errorf("'piiEntities[%d].piiRegex' is required and must be a non-empty string", i)
			}

			compiledPattern, err := regexp.Compile(piiRegex)
			if err != nil {
				return result, fmt.Errorf("'piiEntities[%d].piiRegex' is invalid: %w", i, err)
			}

			if _, exists := piiEntities[piiEntity]; exists {
				return result, fmt.Errorf("duplicate piiEntity: %q", piiEntity)
			}
			piiEntities[piiEntity] = compiledPattern
		}

		if len(piiEntities) == 0 {
			return result, fmt.Errorf("'piiEntities' cannot be empty")
		}

		result.PIIEntities = piiEntities
	}

	// Extract optional jsonPath parameter
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		if jsonPath, ok := jsonPathRaw.(string); ok {
			result.JsonPath = jsonPath
		} else {
			return result, fmt.Errorf("'jsonPath' must be a string")
		}
	}

	// Extract optional redactPII parameter
	if redactPIIRaw, ok := params["redactPII"]; ok {
		if redactPII, ok := redactPIIRaw.(bool); ok {
			result.RedactPII = redactPII
		} else {
			return result, fmt.Errorf("'redactPII' must be a boolean")
		}
	}

	return result, nil
}

// Mode returns the processing mode for this policy
func (p *PIIMaskingRegexPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest masks PII in request body
func (p *PIIMaskingRegexPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if len(p.params.PIIEntities) == 0 {
		// No PII entities configured, pass through
		return policy.UpstreamRequestModifications{}
	}

	if ctx.Body == nil || ctx.Body.Content == nil {
		return policy.UpstreamRequestModifications{}
	}
	payload := ctx.Body.Content

	// Extract value using JSONPath
	extractedValue, err := utils.ExtractStringValueFromJsonpath(payload, p.params.JsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err)).(policy.RequestAction)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	var modifiedContent string
	if p.params.RedactPII {
		// Redaction mode: replace with *****
		modifiedContent = p.redactPIIFromContent(extractedValue, p.params.PIIEntities)
	} else {
		// Masking mode: replace with placeholders and store mappings
		modifiedContent, err = p.maskPIIFromContent(extractedValue, p.params.PIIEntities, ctx.Metadata)
		if err != nil {
			return p.buildErrorResponse(fmt.Sprintf("error masking PII: %v", err)).(policy.RequestAction)
		}
	}

	// If content was modified, update the payload
	if modifiedContent != "" && modifiedContent != extractedValue {
		modifiedPayload := p.updatePayloadWithMaskedContent(payload, extractedValue, modifiedContent, p.params.JsonPath)
		return policy.UpstreamRequestModifications{
			Body: modifiedPayload,
		}
	}

	return policy.UpstreamRequestModifications{}
}

// OnResponse restores PII in response body (if redactPII is false)
func (p *PIIMaskingRegexPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// If redactPII is true, no restoration needed
	if p.params.RedactPII {
		return policy.UpstreamResponseModifications{}
	}

	// Check if PII entities were masked in request
	maskedPII, exists := ctx.Metadata[MetadataKeyPIIEntities]
	if !exists {
		return policy.UpstreamResponseModifications{}
	}

	maskedPIIMap, ok := maskedPII.(map[string]string)
	if !ok {
		return policy.UpstreamResponseModifications{}
	}

	if ctx.ResponseBody == nil || ctx.ResponseBody.Content == nil {
		return policy.UpstreamResponseModifications{}
	}
	payload := ctx.ResponseBody.Content

	// Restore PII in response
	restoredContent := p.restorePIIInResponse(string(payload), maskedPIIMap)
	if restoredContent != string(payload) {
		return policy.UpstreamResponseModifications{
			Body: []byte(restoredContent),
		}
	}

	return policy.UpstreamResponseModifications{}
}

// maskPIIFromContent masks PII from content using regex patterns
func (p *PIIMaskingRegexPolicy) maskPIIFromContent(content string, piiEntities map[string]*regexp.Regexp, metadata map[string]interface{}) (string, error) {
	if content == "" {
		return "", nil
	}

	maskedContent := content
	maskedPIIEntities := make(map[string]string)
	counter := 0
	// Pre-compile placeholder pattern for efficiency
	placeholderPattern := regexp.MustCompile(`^\[[A-Z_]+_[0-9a-f]{4}\]$`)

	// First pass: find all matches without replacing to avoid nested replacements
	allMatches := make(map[string]string) // original -> placeholder
	for key, pattern := range piiEntities {
		matches := pattern.FindAllString(maskedContent, -1)
		for _, match := range matches {
			if _, exists := allMatches[match]; !exists && !placeholderPattern.MatchString(match) {
				// Generate unique placeholder like [EMAIL_0000]
				placeholder := fmt.Sprintf("[%s_%04x]", key, counter)
				allMatches[match] = placeholder
				maskedPIIEntities[match] = placeholder
				counter++
			}
		}
	}

	// Second pass: replace all matches
	originals := make([]string, 0, len(allMatches))
	for original := range allMatches {
		originals = append(originals, original)
	}
	sort.Slice(originals, func(i, j int) bool { return len(originals[i]) > len(originals[j]) })
	for _, original := range originals {
		maskedContent = strings.ReplaceAll(maskedContent, original, allMatches[original])
	}

	// Store PII mappings in metadata for response restoration
	if len(maskedPIIEntities) > 0 {
		metadata[MetadataKeyPIIEntities] = maskedPIIEntities
	}

	if len(allMatches) > 0 {
		return maskedContent, nil
	}

	return "", nil
}

// redactPIIFromContent redacts PII from content using regex patterns
func (p *PIIMaskingRegexPolicy) redactPIIFromContent(content string, piiEntities map[string]*regexp.Regexp) string {
	if content == "" {
		return ""
	}

	maskedContent := content
	foundAndMasked := false

	for _, pattern := range piiEntities {
		if pattern.MatchString(maskedContent) {
			foundAndMasked = true
			maskedContent = pattern.ReplaceAllString(maskedContent, "*****")
		}
	}

	if foundAndMasked {
		return maskedContent
	}

	return ""
}

// restorePIIInResponse handles PII restoration in responses when redactPII is disabled
func (p *PIIMaskingRegexPolicy) restorePIIInResponse(originalContent string, maskedPIIEntities map[string]string) string {
	if len(maskedPIIEntities) == 0 {
		return originalContent
	}

	transformedContent := originalContent

	for original, placeholder := range maskedPIIEntities {
		if strings.Contains(transformedContent, placeholder) {
			transformedContent = strings.ReplaceAll(transformedContent, placeholder, original)
		}
	}

	return transformedContent
}

// updatePayloadWithMaskedContent updates the original payload by replacing the extracted content
func (p *PIIMaskingRegexPolicy) updatePayloadWithMaskedContent(originalPayload []byte, extractedValue, modifiedContent string, jsonPath string) []byte {
	if jsonPath == "" {
		// If no JSONPath, the entire payload was processed, return the modified content
		return []byte(modifiedContent)
	}

	// If JSONPath is specified, update only the specific field in the JSON structure
	var jsonData map[string]interface{}
	if err := json.Unmarshal(originalPayload, &jsonData); err != nil {
		// Fallback to returning the modified content as-is
		return []byte(modifiedContent)
	}

	// Set the new value at the JSONPath location
	err := utils.SetValueAtJSONPath(jsonData, jsonPath, modifiedContent)
	if err != nil {
		// Fallback to returning the original payload
		return originalPayload
	}

	// Marshal back to JSON to get the full modified payload
	updatedPayload, err := json.Marshal(jsonData)
	if err != nil {
		// Fallback to returning the original payload
		return originalPayload
	}

	return updatedPayload
}

// buildErrorResponse builds an error response for both request and response phases
func (p *PIIMaskingRegexPolicy) buildErrorResponse(reason string) interface{} {
	responseBody := map[string]interface{}{
		"code":    APIMInternalExceptionCode,
		"message": "Error occurred during PIIMaskingRegex mediation: " + reason,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(fmt.Sprintf(`{"code":%d,"type":"PII_MASKING_REGEX","message":"Internal error"}`, APIMInternalExceptionCode))
	}

	// For PII masking, errors typically occur in request phase, but return as ImmediateResponse
	return policy.ImmediateResponse{
		StatusCode: APIMInternalErrorCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
}
