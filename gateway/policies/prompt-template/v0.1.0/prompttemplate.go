package prompttemplate

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

var (
	// promptTemplateRegex matches template://<template-name>?<params> patterns
	// Example: template://translate?from=english&to=spanish or template://translate
	promptTemplateRegex = regexp.MustCompile(`template://[a-zA-Z0-9_-]+(?:\?[^\s"']*)?`)
	// textCleanRegex removes leading and trailing quotes from JSON-escaped strings
	textCleanRegex = regexp.MustCompile(`^"|"$`)
)

// PromptTemplatePolicy implements prompt templating by applying custom templates
type PromptTemplatePolicy struct {
	params PromptTemplatePolicyParams
}

type TemplateConfig struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

type PromptTemplatePolicyParams struct {
	PromptTemplateConfig []TemplateConfig
	// Templates map for quick lookup by name
	templates map[string]string
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &PromptTemplatePolicy{}

	// Parse parameters
	policyParams, err := parseParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	p.params = policyParams

	return p, nil
}

// parseParams parses and validates parameters from map to struct
func parseParams(params map[string]interface{}) (PromptTemplatePolicyParams, error) {
	var result PromptTemplatePolicyParams
	// Extract required promptTemplateConfig parameter
	promptTemplateConfigRaw, ok := params["promptTemplateConfig"]
	if !ok {
		return result, fmt.Errorf("'promptTemplateConfig' parameter is required")
	}

	var promptTemplateConfig []TemplateConfig
	switch v := promptTemplateConfigRaw.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &promptTemplateConfig); err != nil {
			return result, fmt.Errorf("error unmarshaling promptTemplateConfig: %w", err)
		}
	case []interface{}:
		// Convert array of interfaces to TemplateConfig array
		promptTemplateConfig = make([]TemplateConfig, 0, len(v))
		for idx, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				var templateConfig TemplateConfig
				jsonBytes, err := json.Marshal(itemMap)
				if err != nil {
					return result, fmt.Errorf("error marshaling promptTemplateConfig[%d]: %w", idx, err)
				}
				if err := json.Unmarshal(jsonBytes, &templateConfig); err != nil {
					return result, fmt.Errorf("error unmarshaling promptTemplateConfig[%d]: %w", idx, err)
				}
				promptTemplateConfig = append(promptTemplateConfig, templateConfig)
			} else {
				return result, fmt.Errorf("'promptTemplateConfig[%d]' must be an object", idx)
			}
		}
	default:
		return result, fmt.Errorf("'promptTemplateConfig' must be a JSON string or array")
	}

	if len(promptTemplateConfig) == 0 {
		return result, fmt.Errorf("'promptTemplateConfig' cannot be empty")
	}
	result.PromptTemplateConfig = promptTemplateConfig

	// Build templates map for quick lookup by name
	result.templates = make(map[string]string)
	for _, templateConfig := range promptTemplateConfig {
		if templateConfig.Name == "" {
			return result, fmt.Errorf("template name cannot be empty")
		}
		if templateConfig.Prompt == "" {
			return result, fmt.Errorf("template prompt cannot be empty for template '%s'", templateConfig.Name)
		}
		result.templates[templateConfig.Name] = templateConfig.Prompt
	}

	// Collect template names for logging
	templateNames := make([]string, 0, len(result.templates))
	for name := range result.templates {
		templateNames = append(templateNames, name)
	}
	slog.Debug("PromptTemplate: Policy initialized", "templateCount", len(result.templates), "templateNames", templateNames)

	return result, nil
}

// Mode returns the processing mode for this policy
func (p *PromptTemplatePolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequest applies template to request body
func (p *PromptTemplatePolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}

	// Convert to string to search for template:// patterns
	jsonContent := string(content)
	if jsonContent == "" {
		return policy.UpstreamRequestModifications{}
	}

	updatedJsonContent := jsonContent

	// Find all template://<template-name>?<params> patterns (query params are optional)
	matches := promptTemplateRegex.FindAllString(jsonContent, -1)
	if len(matches) > 0 {
		slog.Debug("PromptTemplate: Found template patterns", "count", len(matches))
	}
	for _, matched := range matches {
		// Parse the matched string as a URI
		// Example: template://translate?from=english&to=spanish or template://translate
		parsedURL, err := url.Parse(matched)
		if err != nil {
			// Skip invalid URIs
			continue
		}

		templateName := parsedURL.Host // "translate"
		query := parsedURL.RawQuery    // "from=english&to=spanish"

		// Look up template by name
		templatePrompt, exists := p.params.templates[templateName]
		if !exists {
			slog.Debug("PromptTemplate: Template not found", "templateName", templateName)
			// Template not found, return error
			return p.buildErrorResponse(fmt.Sprintf("Template '%s' not found", templateName), nil)
		}

		// Parse query parameters
		paramsMap := make(map[string]string)
		if query != "" {
			queryParams, err := url.ParseQuery(query)
			if err == nil {
				for key, values := range queryParams {
					if len(values) > 0 {
						// URL decode the value
						decodedValue, err := url.QueryUnescape(values[0])
						if err == nil {
							paramsMap[key] = decodedValue
						} else {
							paramsMap[key] = values[0]
						}
					}
				}
			}
		}

		// Replace placeholders in template (format: [[parameter-name]])
		resolvedPrompt := templatePrompt
		for key, value := range paramsMap {
			placeholder := "[[" + key + "]]"
			resolvedPrompt = strings.ReplaceAll(resolvedPrompt, placeholder, value)
		}

		// Escape the resolved prompt for JSON (add quotes and escape special chars)
		escapedPromptBytes, err := json.Marshal(resolvedPrompt)
		if err != nil {
			slog.Debug("PromptTemplate: Error marshaling resolved prompt to JSON", "templateName", templateName, "error", err)
			// If marshaling fails, return error
			return p.buildErrorResponse("Error marshaling resolved prompt to JSON", err)
		}
		escapedPrompt := string(escapedPromptBytes)
		escapedPrompt = textCleanRegex.ReplaceAllString(escapedPrompt, "")

		slog.Debug("PromptTemplate: Resolved template", "templateName", templateName, "paramCount", len(paramsMap), "resolvedLength", len(resolvedPrompt))
		// Replace the matched template:// pattern with the resolved prompt
		updatedJsonContent = strings.Replace(updatedJsonContent, matched, escapedPrompt, 1)
	}

	// Convert back to bytes
	updatedPayload := []byte(updatedJsonContent)

	return policy.UpstreamRequestModifications{
		Body: updatedPayload,
	}
}

// OnResponse is not used for this policy
func (p *PromptTemplatePolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{}
}

// buildErrorResponse builds an error response
func (p *PromptTemplatePolicy) buildErrorResponse(reason string, validationError error) policy.RequestAction {
	errorMessage := reason
	if validationError != nil {
		errorMessage = fmt.Sprintf("%s: %v", reason, validationError)
	}

	responseBody := map[string]interface{}{
		"type":    "PROMPT_TEMPLATE_ERROR",
		"message": errorMessage,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(`{"type":"PROMPT_TEMPLATE_ERROR","message":"Internal error"}`)
	}

	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
}
