package semanticpromptguard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
	embeddingproviders "github.com/wso2/api-platform/sdk/utils/embeddingproviders"
)

const (
	GuardrailErrorCode              = 422
	defaultAllowSimilarityThreshold = 0.65 // Similarity threshold (0-1), higher = stricter matching
	defaultDenySimilarityThreshold  = 0.65 // Similarity threshold (0-1), higher = stricter blocking
	defaultTimeoutMs                = 5000 // Default timeout in milliseconds
)

// PhraseEmbedding represents a configured phrase and its embedding vector.
type PhraseEmbedding struct {
	Phrase    string
	Embedding []float32
}

// SemanticPromptGuardPolicyParams holds configuration for the policy
type SemanticPromptGuardPolicyParams struct {
	JsonPath                 string
	AllowSimilarityThreshold float64 // Similarity threshold (0-1), higher = stricter
	DenySimilarityThreshold  float64 // Similarity threshold (0-1), higher = stricter blocking
	AllowedPhrases           []PhraseEmbedding
	DeniedPhrases            []PhraseEmbedding
	ShowAssessment           bool
}

// SemanticPromptGuardPolicy performs semantic similarity checks against allow/deny lists.
type SemanticPromptGuardPolicy struct {
	embeddingProvider embeddingproviders.EmbeddingProvider
	embeddingConfig   embeddingproviders.EmbeddingProviderConfig
	params            SemanticPromptGuardPolicyParams
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	p := &SemanticPromptGuardPolicy{}

	// Parse and validate embedding provider configuration (from systemParameters)
	if err := parseEmbeddingConfig(params, p); err != nil {
		return nil, fmt.Errorf("invalid embedding config: %w", err)
	}

	// Initialize embedding provider
	embeddingProvider, err := createEmbeddingProvider(p.embeddingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}
	p.embeddingProvider = embeddingProvider

	// Parse policy parameters
	requestParams, err := parseParams(params, p)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}
	p.params = requestParams

	return p, nil
}

// parseEmbeddingConfig parses and validates embedding provider configuration
func parseEmbeddingConfig(params map[string]interface{}, p *SemanticPromptGuardPolicy) error {
	provider, ok := params["embeddingProvider"].(string)
	if !ok || provider == "" {
		return fmt.Errorf("'embeddingProvider' is required")
	}

	embeddingEndpoint, ok := params["embeddingEndpoint"].(string)
	if !ok || embeddingEndpoint == "" {
		return fmt.Errorf("'embeddingEndpoint' is required")
	}

	// embeddingModel is required for OPENAI and MISTRAL, but not for AZURE_OPENAI
	embeddingModel, ok := params["embeddingModel"].(string)
	if !ok || embeddingModel == "" {
		providerUpper := strings.ToUpper(provider)
		if providerUpper == "OPENAI" || providerUpper == "MISTRAL" {
			return fmt.Errorf("'embeddingModel' is required for %s provider", provider)
		}
		// For AZURE_OPENAI, embeddingModel is optional (deployment name is in endpoint)
		embeddingModel = ""
	}

	apiKey, ok := params["apiKey"].(string)
	if !ok || apiKey == "" {
		return fmt.Errorf("'apiKey' is required")
	}

	// Extract optional timeoutMs parameter (default: 5000ms)
	timeoutMs := defaultTimeoutMs
	if timeoutMsRaw, ok := params["timeoutMs"]; ok {
		switch v := timeoutMsRaw.(type) {
		case int:
			timeoutMs = v
		case int64:
			timeoutMs = int(v)
		case float64:
			timeoutMs = int(v)
		}
	}
	if timeoutMs < 1 || timeoutMs > 60000 {
		return fmt.Errorf("'timeoutMs' must be between 1 and 60000")
	}

	// Set header name based on provider type
	// Azure OpenAI uses "api-key", others use "Authorization"
	authHeaderName := "Authorization"
	if strings.ToUpper(provider) == "AZURE_OPENAI" {
		authHeaderName = "api-key"
	}

	p.embeddingConfig = embeddingproviders.EmbeddingProviderConfig{
		EmbeddingProvider: strings.ToUpper(provider),
		EmbeddingEndpoint: embeddingEndpoint,
		APIKey:            apiKey,
		AuthHeaderName:    authHeaderName,
		TimeOut:           strconv.Itoa(timeoutMs),
	}
	// Only set EmbeddingModel if it was provided (required for OPENAI/MISTRAL)
	if embeddingModel != "" {
		p.embeddingConfig.EmbeddingModel = embeddingModel
	}

	return nil
}

// createEmbeddingProvider creates a new embedding provider based on the config
func createEmbeddingProvider(config embeddingproviders.EmbeddingProviderConfig) (embeddingproviders.EmbeddingProvider, error) {
	var provider embeddingproviders.EmbeddingProvider

	switch config.EmbeddingProvider {
	case "OPENAI":
		provider = &embeddingproviders.OpenAIEmbeddingProvider{}
	case "MISTRAL":
		provider = &embeddingproviders.MistralEmbeddingProvider{}
	case "AZURE_OPENAI":
		provider = &embeddingproviders.AzureOpenAIEmbeddingProvider{}
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.EmbeddingProvider)
	}

	if err := provider.Init(config); err != nil {
		return nil, fmt.Errorf("failed to initialize embedding provider: %w", err)
	}

	return provider, nil
}

// parseParams parses and validates policy parameters
func parseParams(params map[string]interface{}, p *SemanticPromptGuardPolicy) (SemanticPromptGuardPolicyParams, error) {
	var result SemanticPromptGuardPolicyParams

	// Extract optional jsonPath parameter
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		if jsonPath, ok := jsonPathRaw.(string); ok {
			result.JsonPath = jsonPath
		} else {
			return result, fmt.Errorf("'jsonPath' must be a string")
		}
	}

	result.AllowSimilarityThreshold = parseFloatParam(params, "allowSimilarityThreshold", defaultAllowSimilarityThreshold)
	if result.AllowSimilarityThreshold < 0.0 || result.AllowSimilarityThreshold > 1.0 {
		return result, fmt.Errorf("'allowSimilarityThreshold' must be between 0.0 and 1.0 (similarity range)")
	}

	result.DenySimilarityThreshold = parseFloatParam(params, "denySimilarityThreshold", defaultDenySimilarityThreshold)
	if result.DenySimilarityThreshold < 0.0 || result.DenySimilarityThreshold > 1.0 {
		return result, fmt.Errorf("'denySimilarityThreshold' must be between 0.0 and 1.0 (similarity range)")
	}

	// Extract optional showAssessment parameter
	if showAssessmentRaw, ok := params["showAssessment"]; ok {
		if showAssessment, ok := showAssessmentRaw.(bool); ok {
			result.ShowAssessment = showAssessment
		} else {
			return result, fmt.Errorf("'showAssessment' must be a boolean")
		}
	}

	// Parse allowed phrases from configuration
	allowedPhrases, err := parsePhraseEmbeddings(params["allowedPhrases"], "allowedPhrases")
	if err != nil {
		return result, fmt.Errorf("error parsing allowedPhrases: %w", err)
	}

	// Parse denied phrases from configuration
	deniedPhrases, err := parsePhraseEmbeddings(params["deniedPhrases"], "deniedPhrases")
	if err != nil {
		return result, fmt.Errorf("error parsing deniedPhrases: %w", err)
	}

	if len(allowedPhrases) == 0 && len(deniedPhrases) == 0 {
		return result, fmt.Errorf("at least one allowedPhrases or deniedPhrases entry is required")
	}

	slog.Debug("SemanticPromptGuard: Loaded phrases", "allowedCount", len(allowedPhrases), "deniedCount", len(deniedPhrases))

	// Ensure embeddings for phrases that don't have them
	allowedPhrases, err = p.ensureEmbeddings(allowedPhrases)
	if err != nil {
		return result, fmt.Errorf("error ensuring embeddings for allowed phrases: %w", err)
	}

	deniedPhrases, err = p.ensureEmbeddings(deniedPhrases)
	if err != nil {
		return result, fmt.Errorf("error ensuring embeddings for denied phrases: %w", err)
	}

	result.AllowedPhrases = allowedPhrases
	result.DeniedPhrases = deniedPhrases

	return result, nil
}

// parseFloatParam safely extracts a float64 from various types
func parseFloatParam(params map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return defaultVal
}

// parsePhraseEmbeddings parses phrases from configuration (array of strings)
func parsePhraseEmbeddings(raw interface{}, fieldName string) ([]PhraseEmbedding, error) {
	if raw == nil {
		return nil, nil
	}

	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array", fieldName)
	}

	phrases := make([]PhraseEmbedding, 0, len(list))
	for i, entry := range list {
		phrase, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a string", fieldName, i)
		}

		if phrase == "" {
			return nil, fmt.Errorf("%s[%d] cannot be empty", fieldName, i)
		}

		phrases = append(phrases, PhraseEmbedding{
			Phrase:    phrase,
			Embedding: nil, // Will be fetched later
		})
	}

	return phrases, nil
}

// ensureEmbeddings fetches embeddings for all phrases
func (p *SemanticPromptGuardPolicy) ensureEmbeddings(phrases []PhraseEmbedding) ([]PhraseEmbedding, error) {
	if len(phrases) == 0 {
		return phrases, nil
	}

	// Collect all phrase texts
	textsToFetch := make([]string, len(phrases))
	for i, phrase := range phrases {
		if phrase.Phrase == "" {
			return nil, fmt.Errorf("phrase at index %d is empty", i)
		}
		textsToFetch[i] = phrase.Phrase
	}

	// Fetch all embeddings in a single batch call
	slog.Debug("SemanticPromptGuard: Fetching embeddings", "phraseCount", len(textsToFetch))
	embeddingsFloat32, err := p.embeddingProvider.GetEmbeddings(textsToFetch)
	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings: %w", err)
	}

	if len(embeddingsFloat32) != len(textsToFetch) {
		return nil, fmt.Errorf("expected %d embeddings but got %d", len(textsToFetch), len(embeddingsFloat32))
	}

	// Map embeddings back to phrases
	for i, embedding := range embeddingsFloat32 {
		phrases[i].Embedding = embedding
	}

	return phrases, nil
}

// Mode returns the processing mode for this policy
func (p *SemanticPromptGuardPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest performs semantic filtering of the incoming prompt
func (p *SemanticPromptGuardPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, p.params).(policy.RequestAction)
}

// OnResponse is not used by this policy (validation is request-only)
func (p *SemanticPromptGuardPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{}
}

// validatePayload validates payload using semantic similarity
func (p *SemanticPromptGuardPolicy) validatePayload(payload []byte, params SemanticPromptGuardPolicyParams) interface{} {
	// Extract prompt using JSONPath
	prompt, err := utils.ExtractStringValueFromJsonpath(payload, params.JsonPath)
	if err != nil {
		return p.buildErrorResponse("Error extracting value from JSONPath", err, params.ShowAssessment)
	}

	if prompt == "" {
		return p.buildErrorResponse("Empty prompt extracted", nil, params.ShowAssessment)
	}

	// Get embedding using the provider
	promptEmbedding, err := p.embeddingProvider.GetEmbedding(prompt)
	if err != nil {
		slog.Debug("SemanticPromptGuard: Error fetching prompt embedding", "error", err)
		return p.buildErrorResponse("Failed to generate embedding for prompt", err, params.ShowAssessment)
	}

	// Determine matching strategy based on what lists are configured
	if len(params.DeniedPhrases) > 0 && len(params.AllowedPhrases) == 0 {
		// Only denied list: block if matches denied phrases, allow otherwise
		if similarity, phrase, err := maxSimilarity(promptEmbedding, params.DeniedPhrases); err == nil {
			if similarity >= params.DenySimilarityThreshold {
				slog.Debug("SemanticPromptGuard: BLOCKED - prompt too similar to denied phrase", "phrase", phrase.Phrase, "similarity", similarity, "threshold", params.DenySimilarityThreshold)
				reason := fmt.Sprintf("prompt is too similar to denied phrase '%s' (similarity=%.4f)", phrase.Phrase, similarity)
				return p.buildErrorResponse(reason, nil, params.ShowAssessment)
			}
			slog.Debug("SemanticPromptGuard: ALLOWED - prompt does not match denied phrases", "maxSimilarity", similarity, "threshold", params.DenySimilarityThreshold)
		} else {
			slog.Debug("SemanticPromptGuard: Error calculating similarity to denied phrases", "error", err)
			return p.buildErrorResponse("Error calculating semantic similarity", err, params.ShowAssessment)
		}
		return policy.UpstreamRequestModifications{}
	} else if len(params.AllowedPhrases) > 0 && len(params.DeniedPhrases) == 0 {
		// Only allowed list: allow if matches allowed phrases, block otherwise
		allowedSimilarity, phrase, err := maxSimilarity(promptEmbedding, params.AllowedPhrases)
		if err != nil {
			slog.Debug("SemanticPromptGuard: Error calculating similarity to allowed phrases", "error", err)
			return p.buildErrorResponse("Error calculating semantic similarity", err, params.ShowAssessment)
		}
		if allowedSimilarity >= params.AllowSimilarityThreshold {
			slog.Debug("SemanticPromptGuard: ALLOWED - prompt matches allowed phrase", "phrase", phrase.Phrase, "similarity", allowedSimilarity, "threshold", params.AllowSimilarityThreshold)
			return policy.UpstreamRequestModifications{}
		}
		slog.Debug("SemanticPromptGuard: BLOCKED - prompt does not match allowed phrases", "maxSimilarity", allowedSimilarity, "threshold", params.AllowSimilarityThreshold)
		reason := fmt.Sprintf("prompt is not similar enough to allowed phrases (similarity=%.4f < threshold=%.4f)", allowedSimilarity, params.AllowSimilarityThreshold)
		return p.buildErrorResponse(reason, nil, params.ShowAssessment)
	} else {
		// Both allowed and denied lists are configured: check both
		if similarity, phrase, err := maxSimilarity(promptEmbedding, params.DeniedPhrases); err == nil {
			if similarity >= params.DenySimilarityThreshold {
				slog.Debug("SemanticPromptGuard: BLOCKED - prompt too similar to denied phrase", "phrase", phrase.Phrase, "similarity", similarity, "threshold", params.DenySimilarityThreshold)
				reason := fmt.Sprintf("prompt is too similar to denied phrase '%s' (similarity=%.4f)", phrase.Phrase, similarity)
				return p.buildErrorResponse(reason, nil, params.ShowAssessment)
			}
		} else {
			slog.Debug("SemanticPromptGuard: Error calculating similarity to denied phrases", "error", err)
			return p.buildErrorResponse("Error calculating semantic similarity", err, params.ShowAssessment)
		}

		allowedSimilarity, phrase, err := maxSimilarity(promptEmbedding, params.AllowedPhrases)
		if err != nil {
			slog.Debug("SemanticPromptGuard: Error calculating similarity to allowed phrases", "error", err)
			return p.buildErrorResponse("Error calculating semantic similarity", err, params.ShowAssessment)
		}
		if allowedSimilarity >= params.AllowSimilarityThreshold {
			slog.Debug("SemanticPromptGuard: ALLOWED - prompt matches allowed phrase", "phrase", phrase.Phrase, "similarity", allowedSimilarity, "threshold", params.AllowSimilarityThreshold)
			return policy.UpstreamRequestModifications{}
		}
		slog.Debug("SemanticPromptGuard: BLOCKED - prompt does not match allowed phrases", "maxSimilarity", allowedSimilarity, "threshold", params.AllowSimilarityThreshold)
		reason := fmt.Sprintf("prompt is not similar enough to allowed phrases (similarity=%.4f < threshold=%.4f)", allowedSimilarity, params.AllowSimilarityThreshold)
		return p.buildErrorResponse(reason, nil, params.ShowAssessment)
	}
}

// buildErrorResponse builds an error response for request phase
func (p *SemanticPromptGuardPolicy) buildErrorResponse(reason string, validationError error, showAssessment bool) policy.RequestAction {
	assessment := p.buildAssessmentObject(reason, validationError, showAssessment)

	responseBody := map[string]interface{}{
		"type":    "SEMANTIC_PROMPT_GUARD",
		"message": assessment,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(`{"type":"SEMANTIC_PROMPT_GUARD","message":"Internal error"}`)
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
func (p *SemanticPromptGuardPolicy) buildAssessmentObject(reason string, validationError error, showAssessment bool) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "SemanticPromptGuard",
		"direction":            "REQUEST",
	}

	if validationError != nil {
		assessment["actionReason"] = reason
	} else {
		assessment["actionReason"] = "Violation of applied semantic prompt guard constraints detected."
	}

	if showAssessment {
		if validationError != nil {
			assessment["assessments"] = validationError.Error()
		} else {
			assessment["assessments"] = reason
		}
	}

	return assessment
}

// maxSimilarity finds the maximum cosine similarity between target and phrases
func maxSimilarity(target []float32, phrases []PhraseEmbedding) (float64, PhraseEmbedding, error) {
	if len(phrases) == 0 {
		return 0, PhraseEmbedding{}, nil
	}

	maxSim := -1.0
	var closest PhraseEmbedding

	for _, phrase := range phrases {
		similarity, err := cosineSimilarity(target, phrase.Embedding)
		if err != nil {
			return 0, PhraseEmbedding{}, err
		}

		if similarity > maxSim {
			maxSim = similarity
			closest = phrase
		}
	}

	return maxSim, closest, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float32) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, fmt.Errorf("embedding vectors cannot be empty")
	}

	if len(a) != len(b) {
		return 0, fmt.Errorf("embedding dimensions do not match: %d vs %d", len(a), len(b))
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0, fmt.Errorf("embedding vector norm is zero")
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB)), nil
}
