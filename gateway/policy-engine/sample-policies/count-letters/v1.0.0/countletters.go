package countletters

import (
	"encoding/json"
	"fmt"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// CountLettersPolicy counts occurrences of specified letters in the response body
type CountLettersPolicy struct{}

// NewPolicy creates a new CountLettersPolicy instance
func NewPolicy() policy.Policy {
	return &CountLettersPolicy{}
}

// Mode returns the processing mode for this policy
func (p *CountLettersPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip, // Don't need request headers
		RequestBodyMode:    policy.BodyModeSkip,   // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip, // Don't process response headers
		ResponseBodyMode:   policy.BodyModeBuffer, // Need full buffered response body
	}
}

// Validate validates the policy configuration
func (p *CountLettersPolicy) Validate(params map[string]interface{}) error {
	// Validate letters parameter (required)
	lettersRaw, ok := params["letters"]
	if !ok {
		return fmt.Errorf("'letters' parameter is required")
	}

	letters, ok := lettersRaw.([]interface{})
	if !ok {
		return fmt.Errorf("'letters' must be an array")
	}

	if len(letters) == 0 {
		return fmt.Errorf("'letters' array cannot be empty")
	}

	for i, letterRaw := range letters {
		letter, ok := letterRaw.(string)
		if !ok {
			return fmt.Errorf("letters[%d] must be a string", i)
		}
		if len(letter) == 0 {
			return fmt.Errorf("letters[%d] cannot be empty", i)
		}
	}

	// Validate caseSensitive parameter (optional, defaults to false)
	if caseSensitiveRaw, ok := params["caseSensitive"]; ok {
		_, ok := caseSensitiveRaw.(bool)
		if !ok {
			return fmt.Errorf("'caseSensitive' must be a boolean")
		}
	}

	// Validate outputFormat parameter (optional, defaults to "json")
	if outputFormatRaw, ok := params["outputFormat"]; ok {
		outputFormat, ok := outputFormatRaw.(string)
		if !ok {
			return fmt.Errorf("'outputFormat' must be a string")
		}
		outputFormat = strings.ToLower(outputFormat)
		if outputFormat != "json" && outputFormat != "text" {
			return fmt.Errorf("'outputFormat' must be 'json' or 'text'")
		}
	}

	return nil
}

// OnRequest is not used by this policy (only processes response body)
func (p *CountLettersPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	return nil // No request processing needed
}

// OnResponse counts letters in the response body and replaces it with the count
func (p *CountLettersPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Check if response body is present
	if ctx.ResponseBody == nil || !ctx.ResponseBody.Present {
		// No body to process, return empty count
		return p.generateEmptyResponse(params)
	}

	// Get configuration parameters
	lettersRaw := params["letters"].([]interface{})
	letters := make([]string, len(lettersRaw))
	for i, letterRaw := range lettersRaw {
		letters[i] = letterRaw.(string)
	}

	caseSensitive := false
	if caseSensitiveRaw, ok := params["caseSensitive"]; ok {
		caseSensitive = caseSensitiveRaw.(bool)
	}

	outputFormat := "json"
	if outputFormatRaw, ok := params["outputFormat"]; ok {
		outputFormat = strings.ToLower(outputFormatRaw.(string))
	}

	// Count letters in the response body
	bodyText := string(ctx.ResponseBody.Content)
	counts := p.countLetters(bodyText, letters, caseSensitive)

	// Generate output based on format
	var outputBody []byte
	var err error

	if outputFormat == "json" {
		outputBody, err = p.generateJSONOutput(counts)
		if err != nil {
			// Fallback to text output on JSON error
			outputBody = p.generateTextOutput(counts)
		}
	} else {
		outputBody = p.generateTextOutput(counts)
	}

	return policy.UpstreamResponseModifications{
		Body: outputBody,
	}
}

// countLetters counts occurrences of each letter in the text
func (p *CountLettersPolicy) countLetters(text string, letters []string, caseSensitive bool) map[string]int {
	counts := make(map[string]int)

	// Initialize counts for all requested letters
	for _, letter := range letters {
		key := letter
		if !caseSensitive {
			key = strings.ToLower(letter)
		}
		counts[key] = 0
	}

	// Convert text to lowercase if case-insensitive
	searchText := text
	if !caseSensitive {
		searchText = strings.ToLower(text)
	}

	// Count each letter
	for _, letter := range letters {
		searchLetter := letter
		if !caseSensitive {
			searchLetter = strings.ToLower(letter)
		}

		count := strings.Count(searchText, searchLetter)
		counts[searchLetter] = count
	}

	return counts
}

// generateJSONOutput creates JSON output from counts
func (p *CountLettersPolicy) generateJSONOutput(counts map[string]int) ([]byte, error) {
	result := map[string]interface{}{
		"letterCounts": counts,
	}
	return json.MarshalIndent(result, "", "  ")
}

// generateTextOutput creates plain text output from counts
func (p *CountLettersPolicy) generateTextOutput(counts map[string]int) []byte {
	var output strings.Builder
	output.WriteString("Letter Counts:\n")
	for letter, count := range counts {
		output.WriteString(fmt.Sprintf("%s: %d\n", letter, count))
	}
	return []byte(output.String())
}

// generateEmptyResponse generates a response when no body is present
func (p *CountLettersPolicy) generateEmptyResponse(params map[string]interface{}) policy.ResponseAction {
	outputFormat := "json"
	if outputFormatRaw, ok := params["outputFormat"]; ok {
		outputFormat = strings.ToLower(outputFormatRaw.(string))
	}

	var outputBody []byte
	if outputFormat == "json" {
		outputBody = []byte(`{"letterCounts": {}}`)
	} else {
		outputBody = []byte("Letter Counts:\n(no response body)")
	}

	return policy.UpstreamResponseModifications{
		Body: outputBody,
	}
}
