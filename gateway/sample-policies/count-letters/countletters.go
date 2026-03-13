package countletters

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// CountLettersPolicy counts occurrences of specified letters in the response body
type CountLettersPolicy struct {
	letters       []string
	caseSensitive bool
	outputFormat  string
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	slog.Debug("[Count Letters]: GetPolicy called")

	lettersRaw, ok := params["letters"].([]interface{})
	if !ok || len(lettersRaw) == 0 {
		return nil, fmt.Errorf("letters parameter is required and must be a non-empty array")
	}
	letters := make([]string, len(lettersRaw))
	for i, letterRaw := range lettersRaw {
		s, ok := letterRaw.(string)
		if !ok {
			return nil, fmt.Errorf("letters must be an array of strings")
		}
		if s == "" {
			return nil, fmt.Errorf("letters must be a non-empty array of strings")
		}
		letters[i] = s
	}

	caseSensitive := false
	if v, ok := params["caseSensitive"].(bool); ok {
		caseSensitive = v
	}

	outputFormat := "json"
	if v, ok := params["outputFormat"].(string); ok {
		outputFormat = strings.ToLower(v)
	}

	return &CountLettersPolicy{
		letters:       letters,
		caseSensitive: caseSensitive,
		outputFormat:  outputFormat,
	}, nil
}

// OnResponseBody counts letters in the response body and replaces it with the count
func (p *CountLettersPolicy) OnResponseBody(ctx *policy.ResponseContext) policy.ResponseAction {
	slog.Debug("[Count Letters]: OnResponseBody called", "hasBody", ctx.ResponseBody != nil && ctx.ResponseBody.Present)

	if ctx.ResponseBody == nil || !ctx.ResponseBody.Present {
		slog.Info("[Count Letters]: No response body present, returning empty count")
		return p.generateEmptyResponse()
	}

	slog.Info("[Count Letters]: Processing response body",
		"letters", p.letters,
		"caseSensitive", p.caseSensitive,
		"outputFormat", p.outputFormat,
		"bodySize", len(ctx.ResponseBody.Content))

	bodyText := string(ctx.ResponseBody.Content)
	counts := p.countLetters(bodyText)
	slog.Info("[Count Letters]: Letter counts calculated", "counts", counts)

	var outputBody []byte
	var err error
	if p.outputFormat == "json" {
		outputBody, err = p.generateJSONOutput(counts)
		if err != nil {
			slog.Error("[Count Letters]: Failed to generate JSON output, falling back to text", "error", err)
			outputBody = p.generateTextOutput(counts)
		} else {
			slog.Debug("[Count Letters]: Generated JSON output", "size", len(outputBody))
		}
	} else {
		outputBody = p.generateTextOutput(counts)
		slog.Debug("[Count Letters]: Generated text output", "size", len(outputBody))
	}

	return policy.DownstreamResponseModifications{Body: outputBody}
}

// countLetters counts occurrences of each letter in the text
func (p *CountLettersPolicy) countLetters(text string) map[string]int {
	counts := make(map[string]int)
	for _, letter := range p.letters {
		key := letter
		if !p.caseSensitive {
			key = strings.ToLower(letter)
		}
		counts[key] = 0
	}

	searchText := text
	if !p.caseSensitive {
		searchText = strings.ToLower(text)
	}

	for _, letter := range p.letters {
		searchLetter := letter
		if !p.caseSensitive {
			searchLetter = strings.ToLower(letter)
		}
		counts[searchLetter] = strings.Count(searchText, searchLetter)
	}

	return counts
}

func (p *CountLettersPolicy) generateJSONOutput(counts map[string]int) ([]byte, error) {
	result := map[string]interface{}{"letterCounts": counts}
	return json.MarshalIndent(result, "", "  ")
}

func (p *CountLettersPolicy) generateTextOutput(counts map[string]int) []byte {
	var output strings.Builder
	output.WriteString("Letter Counts:\n")
	for letter, count := range counts {
		output.WriteString(fmt.Sprintf("%s: %d\n", letter, count))
	}
	return []byte(output.String())
}

func (p *CountLettersPolicy) generateEmptyResponse() policy.ResponseAction {
	var outputBody []byte
	if p.outputFormat == "json" {
		outputBody = []byte(`{"letterCounts": {}}`)
	} else {
		outputBody = []byte("Letter Counts:\n(no response body)")
	}
	return policy.DownstreamResponseModifications{Body: outputBody}
}
