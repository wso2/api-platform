package piimaskingregex

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
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
	patternMu sync.RWMutex
}

// NewPolicy creates a new PIIMaskingRegexPolicy instance
func NewPolicy() policy.Policy {
	return &PIIMaskingRegexPolicy{}
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
	// Extract request-specific parameters
	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate parameters
	if err := p.validateParams(requestParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err)).(policy.RequestAction)
	}

	jsonPath, _ := requestParams["jsonPath"].(string)
	redactPII, _ := requestParams["redactPII"].(bool)

	// Parse PII entities
	piiEntities, err := p.parsePIIEntities(requestParams)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error parsing PII entities: %v", err)).(policy.RequestAction)
	}

	if len(piiEntities) == 0 {
		// No PII entities configured, pass through
		return policy.UpstreamRequestModifications{}
	}

	if ctx.Body == nil || ctx.Body.Content == nil {
		return policy.UpstreamRequestModifications{}
	}
	payload := ctx.Body.Content

	// Extract value using JSONPath
	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err)).(policy.RequestAction)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	var modifiedContent string
	if redactPII {
		// Redaction mode: replace with *****
		modifiedContent = p.redactPIIFromContent(extractedValue, piiEntities)
	} else {
		// Masking mode: replace with placeholders and store mappings
		modifiedContent, err = p.maskPIIFromContent(extractedValue, piiEntities, ctx.Metadata)
		if err != nil {
			return p.buildErrorResponse(fmt.Sprintf("error masking PII: %v", err)).(policy.RequestAction)
		}
	}

	// If content was modified, update the payload
	if modifiedContent != "" && modifiedContent != extractedValue {
		modifiedPayload := p.updatePayloadWithMaskedContent(payload, extractedValue, modifiedContent, jsonPath)
		return policy.UpstreamRequestModifications{
			Body: modifiedPayload,
		}
	}

	return policy.UpstreamRequestModifications{}
}

// OnResponse restores PII in response body (if redactPII is false)
func (p *PIIMaskingRegexPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Extract response-specific parameters
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate parameters (only redactPII is used in response)
	if redactPIIRaw, ok := responseParams["redactPII"]; ok {
		_, ok := redactPIIRaw.(bool)
		if !ok {
			return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: 'redactPII' must be a boolean")).(policy.ResponseAction)
		}
	}

	redactPII, _ := responseParams["redactPII"].(bool)

	// If redactPII is true, no restoration needed
	if redactPII {
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

// validateParams validates the actual policy parameters
func (p *PIIMaskingRegexPolicy) validateParams(params map[string]interface{}) error {
	// Validate piiEntities parameter (required)
	piiEntitiesRaw, ok := params["piiEntities"]
	if !ok {
		return fmt.Errorf("'piiEntities' parameter is required")
	}
	piiEntitiesArray, ok := piiEntitiesRaw.([]interface{})
	if !ok {
		return fmt.Errorf("'piiEntities' must be an array")
	}
	if len(piiEntitiesArray) == 0 {
		return fmt.Errorf("'piiEntities' cannot be empty")
	}

	// Validate each PII entity in the array
	for i, entityRaw := range piiEntitiesArray {
		entity, ok := entityRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("'piiEntities[%d]' must be an object", i)
		}

		// Validate piiEntity field
		piiEntityRaw, ok := entity["piiEntity"]
		if !ok {
			return fmt.Errorf("'piiEntities[%d].piiEntity' is required", i)
		}
		piiEntity, ok := piiEntityRaw.(string)
		if !ok {
			return fmt.Errorf("'piiEntities[%d].piiEntity' must be a string", i)
		}
		if piiEntity == "" {
			return fmt.Errorf("'piiEntities[%d].piiEntity' cannot be empty", i)
		}

		// Validate piiRegex field
		piiRegexRaw, ok := entity["piiRegex"]
		if !ok {
			return fmt.Errorf("'piiEntities[%d].piiRegex' is required", i)
		}
		piiRegex, ok := piiRegexRaw.(string)
		if !ok {
			return fmt.Errorf("'piiEntities[%d].piiRegex' must be a string", i)
		}
		if piiRegex == "" {
			return fmt.Errorf("'piiEntities[%d].piiRegex' cannot be empty", i)
		}

		// Validate regex is compilable
		_, err := regexp.Compile(piiRegex)
		if err != nil {
			return fmt.Errorf("'piiEntities[%d].piiRegex' is invalid: %v", i, err)
		}
	}

	// Validate optional parameters
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		_, ok := jsonPathRaw.(string)
		if !ok {
			return fmt.Errorf("'jsonPath' must be a string")
		}
	}

	if redactPIIRaw, ok := params["redactPII"]; ok {
		_, ok := redactPIIRaw.(bool)
		if !ok {
			return fmt.Errorf("'redactPII' must be a boolean")
		}
	}

	return nil
}

// parsePIIEntities parses PII entities from parameters
func (p *PIIMaskingRegexPolicy) parsePIIEntities(params map[string]interface{}) (map[string]*regexp.Regexp, error) {
	piiEntitiesRaw, ok := params["piiEntities"]
	if !ok {
		return make(map[string]*regexp.Regexp), nil
	}

	// Handle JSON string format
	var piiEntitiesArray []map[string]interface{}
	switch v := piiEntitiesRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &piiEntitiesArray); err != nil {
			return nil, fmt.Errorf("error unmarshaling PII entities: %w", err)
		}
	case []interface{}:
		piiEntitiesArray = make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				piiEntitiesArray = append(piiEntitiesArray, itemMap)
			}
		}
	default:
		return nil, fmt.Errorf("invalid PII entities format")
	}

	piiEntities := make(map[string]*regexp.Regexp)
	for _, entityConfig := range piiEntitiesArray {
		piiEntity, _ := entityConfig["piiEntity"].(string)
		piiRegex, _ := entityConfig["piiRegex"].(string)

		if piiEntity == "" || piiRegex == "" {
			continue
		}

		compiledPattern, err := regexp.Compile(piiRegex)
		if err != nil {
			return nil, fmt.Errorf("error compiling regex for PII entity '%s': %w", piiEntity, err)
		}

		piiEntities[piiEntity] = compiledPattern
	}

	return piiEntities, nil
}

// maskPIIFromContent masks PII from content using regex patterns
func (p *PIIMaskingRegexPolicy) maskPIIFromContent(content string, piiEntities map[string]*regexp.Regexp, metadata map[string]interface{}) (string, error) {
	if content == "" {
		return "", nil
	}

	maskedContent := content
	maskedPIIEntities := make(map[string]string)
	counter := 0

	// First pass: find all matches without replacing to avoid nested replacements
	allMatches := make(map[string]string) // original -> placeholder
	for key, pattern := range piiEntities {
		matches := pattern.FindAllString(maskedContent, -1)
		for _, match := range matches {
			// Skip if already processed or if it matches the placeholder format [TYPE_XXXX]
			placeholderPattern := regexp.MustCompile(`^\[[A-Z_]+_[0-9a-f]{4}\]$`)
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
	for original, placeholder := range allMatches {
		maskedContent = strings.ReplaceAll(maskedContent, original, placeholder)
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
	if maskedPIIEntities == nil || len(maskedPIIEntities) == 0 {
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
	err := setValueAtJSONPath(jsonData, jsonPath, modifiedContent)
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

// setValueAtJSONPath sets a value at the specified JSONPath in the given JSON object
func setValueAtJSONPath(jsonData map[string]interface{}, jsonPath, value string) error {
	// Remove the leading "$." if present
	path := strings.TrimPrefix(jsonPath, "$.")
	if path == "" {
		return fmt.Errorf("invalid empty path")
	}

	// Split the path into components
	pathComponents := strings.Split(path, ".")

	// Navigate to the parent object/array
	current := interface{}(jsonData)
	arrayIndexRegex := regexp.MustCompile(`^([a-zA-Z0-9_]+)\[(-?\d+)\]$`)

	for i := 0; i < len(pathComponents)-1; i++ {
		key := pathComponents[i]

		// Check if this key contains array indexing
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
							return fmt.Errorf("array index out of range: %s", idxStr)
						}
						current = arr[idx]
					} else {
						return fmt.Errorf("not an array: %s", arrayName)
					}
				} else {
					return fmt.Errorf("key not found: %s", arrayName)
				}
			} else {
				return fmt.Errorf("invalid structure for key: %s", arrayName)
			}
		} else {
			// Regular object key
			if node, ok := current.(map[string]interface{}); ok {
				if val, exists := node[key]; exists {
					current = val
				} else {
					return fmt.Errorf("key not found: %s", key)
				}
			} else {
				return fmt.Errorf("invalid structure for key: %s", key)
			}
		}
	}

	// Handle the final key (could be array index or object key)
	finalKey := pathComponents[len(pathComponents)-1]

	// Check if the final key contains array indexing
	if matches := arrayIndexRegex.FindStringSubmatch(finalKey); len(matches) == 3 {
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
						return fmt.Errorf("array index out of range: %s", idxStr)
					}
					arr[idx] = value
				} else {
					return fmt.Errorf("not an array: %s", arrayName)
				}
			} else {
				return fmt.Errorf("key not found: %s", arrayName)
			}
		} else {
			return fmt.Errorf("invalid structure for key: %s", arrayName)
		}
	} else {
		// Regular object key
		if node, ok := current.(map[string]interface{}); ok {
			node[finalKey] = value
		} else {
			return fmt.Errorf("invalid structure for final key: %s", finalKey)
		}
	}

	return nil
}

// buildErrorResponse builds an error response for both request and response phases
func (p *PIIMaskingRegexPolicy) buildErrorResponse(reason string) interface{} {
	responseBody := map[string]interface{}{
		"code":    APIMInternalExceptionCode,
		"message": "Error occurred during PIIMaskingRegex mediation: " + reason,
	}

	bodyBytes, _ := json.Marshal(responseBody)

	// For PII masking, errors typically occur in request phase, but return as ImmediateResponse
	return policy.ImmediateResponse{
		StatusCode: APIMInternalErrorCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
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
