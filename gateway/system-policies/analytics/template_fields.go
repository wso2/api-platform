package analytics

import (
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// A provider template declares where each usage field lives at its root, and may override those
// locations per resource under resourceMappings (OpenAI's /responses reports input_tokens where
// /chat/completions reports prompt_tokens). Each location may also list fallbackIdentifiers, tried
// in order after the primary identifier. The resolution here follows the sdk/ai/llmusage library
// the llm-cost policy uses, so analytics and cost read the same fields for the same request.

// llmResourcePath returns the request path relative to the API's context, which is the path a
// template's resourceMappings are declared against.
func llmResourcePath(sc *policy.SharedContext, requestPath string) string {
	if i := strings.IndexByte(requestPath, '?'); i >= 0 {
		requestPath = requestPath[:i]
	}
	if sc != nil {
		apiContext := strings.ReplaceAll(sc.APIContext, "$version", sc.APIVersion)
		if apiContext != "" && apiContext != "/" {
			requestPath = strings.TrimPrefix(requestPath, apiContext)
		}
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	return requestPath
}

// effectiveLLMSpec returns the template spec with the fields of the resourceMappings entry that
// best matches resourcePath laid over it. A field the entry does not declare keeps its root location.
func effectiveLLMSpec(spec map[string]interface{}, resourcePath string) map[string]interface{} {
	mapping := selectLLMResourceMapping(spec, resourcePath)
	if mapping == nil {
		return spec
	}
	merged := make(map[string]interface{}, len(spec)+len(mapping))
	for key, value := range spec {
		merged[key] = value
	}
	for key, value := range mapping {
		if _, isField := value.(map[string]interface{}); isField {
			merged[key] = value
		}
	}
	return merged
}

// selectLLMResourceMapping returns the resourceMappings entry that best matches resourcePath,
// preferring an exact pattern over a wildcard and a longer pattern over a shorter one.
func selectLLMResourceMapping(spec map[string]interface{}, resourcePath string) map[string]interface{} {
	mappings, _ := spec["resourceMappings"].(map[string]interface{})
	resources, _ := mappings["resources"].([]interface{})
	var selected map[string]interface{}
	var selectedPattern string
	for _, entry := range resources {
		candidate, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		pattern, ok := candidate["resource"].(string)
		if !ok || !llmPathsMatch(resourcePath, pattern) {
			continue
		}
		if selected == nil || preferMoreSpecificLLMPath(pattern, selectedPattern) {
			selected, selectedPattern = candidate, pattern
		}
	}
	return selected
}

// llmPathsMatch reports whether a resource path is covered by a pattern. A pattern may end in a
// single wildcard to cover everything beneath a prefix; an embedded wildcard matches nothing.
func llmPathsMatch(resourcePath, pattern string) bool {
	if pattern == "/*" || resourcePath == pattern {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	if !strings.HasSuffix(pattern, "*") || strings.Count(pattern, "*") != 1 {
		return false
	}
	return strings.HasPrefix(resourcePath, strings.TrimSuffix(pattern, "*"))
}

func preferMoreSpecificLLMPath(candidate, current string) bool {
	candidateWildcard := strings.Contains(candidate, "*")
	currentWildcard := strings.Contains(current, "*")
	if candidateWildcard != currentWildcard {
		return currentWildcard
	}
	return len(candidate) > len(current)
}

// llmFieldIdentifiers returns a field's primary identifier followed by its fallbackIdentifiers.
func llmFieldIdentifiers(fieldCfg map[string]interface{}) []string {
	identifier, _ := fieldCfg["identifier"].(string)
	if identifier == "" {
		return nil
	}
	identifiers := []string{identifier}
	fallbacks, _ := fieldCfg["fallbackIdentifiers"].([]interface{})
	for _, fallback := range fallbacks {
		if s, ok := fallback.(string); ok && s != "" {
			identifiers = append(identifiers, s)
		}
	}
	return identifiers
}
