package semanticpromptguard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	defaultAllowDistanceThreshold = 0.25
	defaultDenyDistanceThreshold  = 0.25
	defaultRequestTimeout         = 5 * time.Second
)

// PhraseEmbedding represents a configured phrase and its embedding vector.
type PhraseEmbedding struct {
	Phrase    string
	Embedding []float64
}

// SemanticPromptGuardPolicy performs semantic similarity checks against allow/deny lists.
type SemanticPromptGuardPolicy struct {
	client          *http.Client
	embeddingCache  map[string][]float64
	embeddingLocker sync.RWMutex
}

// Global cache: reuse instances for routes with identical config
var (
	instanceCache = make(map[string]*SemanticPromptGuardPolicy)
	cacheMu       sync.RWMutex
)

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	// Create cache key from params (e.g., Embedding provider, allowed/denied phrases)
	cacheKey := fmt.Sprintf("%v", params)

	cacheMu.RLock()
	if instance, exists := instanceCache[cacheKey]; exists {
		cacheMu.RUnlock()
		return instance, nil // Reuse existing instance
	}
	cacheMu.RUnlock()

	embeddingEndpoint, ok := params["embeddingEndpoint"].(string)
	if !ok {
		return nil, fmt.Errorf("embeddingEndpoint is required")
	}
	requestTimeout := parseDurationParam(params, "timeoutMs", defaultRequestTimeout)

	// Parse allowed phrases from configuration
	allowedPhrases, err := parsePhraseEmbeddings(params["allowedPhrases"], "allowedPhrases")
	if err != nil {
		return nil, fmt.Errorf("Error parsing allowedPhrases: %v", err)
	}

	deniedPhrases, err := parsePhraseEmbeddings(params["deniedPhrases"], "deniedPhrases")
	if err != nil {
		return nil, fmt.Errorf("Error parsing deniedPhrases: %v", err)
	}

	if len(allowedPhrases) == 0 && len(deniedPhrases) == 0 {
		return nil, fmt.Errorf("At least one allowedPhrases or deniedPhrases entry is required")
	}

	log.Printf("[SemanticPromptGuard] Loaded %d allowed phrases, %d denied phrases", len(allowedPhrases), len(deniedPhrases))

	provider := getString(params, "provider")
	if provider == "" {
		provider = "azure-openai"
	}

	apiKey := getString(params, "apiKey")
	var azureDeployment, azureAPIVersion, openAIModel string

	switch strings.ToLower(provider) {
	case "azure-openai":
		azureDeployment = getString(params, "azureDeployment")
		azureAPIVersion = getString(params, "azureAPIVersion")
	case "openai":
		openAIModel = getString(params, "openAIModel")
	}

	allowedPhrases, err = ensureEmbeddings(provider, embeddingEndpoint, requestTimeout, apiKey, azureDeployment, azureAPIVersion, openAIModel, allowedPhrases)
	if err != nil {
		return nil, fmt.Errorf("Error ensuring embeddings for allowed phrases: %v", err)
	}

	deniedPhrases, err = ensureEmbeddings(provider, embeddingEndpoint, requestTimeout, apiKey, azureDeployment, azureAPIVersion, openAIModel, deniedPhrases)
	if err != nil {
		return nil, fmt.Errorf("Error ensuring embeddings for denied phrases: %v", err)
	}

	// Parse and cache public key (expensive operation done once)
	publicKeyPEM, ok := params["publicKey"].(string)
	if !ok {
		return nil, fmt.Errorf("publicKey parameter required")
	}
	publicKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	// Extract other configuration
	issuer, _ := params["issuer"].(string)
	audience, _ := params["audience"].(string)

	// Create new instance with cached data
	instance := &JWTAuthPolicy{
		routeName: metadata.RouteName,
		publicKey: publicKey,
		issuer:    issuer,
		audience:  audience,
	}

	cacheMu.Lock()
	instanceCache[cacheKey] = instance
	cacheMu.Unlock()

	return instance, nil
}

// NewPolicy creates a new SemanticPromptGuardPolicy instance.
func NewPolicy() policy.Policy {
	return &SemanticPromptGuardPolicy{
		client:         &http.Client{Timeout: defaultRequestTimeout},
		embeddingCache: make(map[string][]float64),
	}
}

// Mode declares this policy needs buffered request bodies for embedding.
func (p *SemanticPromptGuardPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequest performs semantic filtering of the incoming prompt.
func (p *SemanticPromptGuardPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	log.Printf("[SemanticPromptGuard] Starting policy execution")

	// Check if embeddingEndpoint is configured and fail if not provided
	embeddingEndpoint := getString(params, "embeddingEndpoint")
	if embeddingEndpoint == "" {
		log.Printf("[SemanticPromptGuard] Error: embeddingEndpoint is required")
		return policy.ImmediateResponse{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"content-type": "application/json"},
			Body:       []byte(`{"error":"invalid configuration","message":"embeddingEndpoint is required"}`),
		}
	}

	// Parse rest of the configuration parameters with defults as fallback
	promptField := getString(params, "promptField")
	allowThreshold := parseFloatParam(params, "allowDistanceThreshold", defaultAllowDistanceThreshold)
	denyThreshold := parseFloatParam(params, "denyDistanceThreshold", defaultDenyDistanceThreshold)
	showAssessment := parseBoolParam(params, "showAssessment", false)
	requestTimeout := parseDurationParam(params, "timeoutMs", defaultRequestTimeout)

	log.Printf("[SemanticPromptGuard] Configuration: provider=%s, endpoint=%s, promptField=%s, allowThreshold=%.4f, denyThreshold=%.4f, showAssessment=%v",
		getString(params, "provider"), embeddingEndpoint, promptField, allowThreshold, denyThreshold, showAssessment)

	provider := getString(params, "provider")
	if provider == "" {
		provider = "azure-openai"
	}

	apiKey := getString(params, "apiKey")
	var azureDeployment, azureAPIVersion, openAIModel string

	switch strings.ToLower(provider) {
	case "azure-openai":
		azureDeployment = getString(params, "azureDeployment")
		azureAPIVersion = getString(params, "azureAPIVersion")
	case "openai":
		openAIModel = getString(params, "openAIModel")
	}

	prompt, err := extractPrompt(ctx.Body, promptField)
	if err != nil {
		log.Printf("[SemanticPromptGuard] Error extracting prompt: %v", err)
		return policy.ImmediateResponse{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"content-type": "application/json"},
			Body:       []byte(fmt.Sprintf(`{"error":"invalid prompt","message":"%s"}`, err.Error())),
		}
	}

	log.Printf("[SemanticPromptGuard] Extracted prompt (length: %d chars)", len(prompt))

	promptEmbedding, err := p.fetchEmbedding(provider, embeddingEndpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, prompt, requestTimeout)
	if err != nil {
		log.Printf("[SemanticPromptGuard] Error fetching prompt embedding: %v", err)
		return policy.ImmediateResponse{
			StatusCode: http.StatusBadGateway,
			Headers:    map[string]string{"content-type": "application/json"},
			Body:       []byte(fmt.Sprintf(`{"error":"embedding_failed","message":"%s"}`, err.Error())),
		}
	}

	log.Printf("[SemanticPromptGuard] Fetched prompt embedding (dimension: %d)", len(promptEmbedding))

	// Determine matching strategy based on what lists are configured
	if len(deniedPhrases) > 0 && len(allowedPhrases) == 0 {
		// Only denied list: block if matches denied phrases, allow otherwise
		if dist, phrase, err := minDistance(promptEmbedding, deniedPhrases); err == nil {
			log.Printf("[SemanticPromptGuard] Checking denied phrases only: min distance=%.4f, threshold=%.4f", dist, denyThreshold)
			if dist <= denyThreshold {
				log.Printf("[SemanticPromptGuard] BLOCKED: prompt too similar to denied phrase '%s' (distance=%.4f <= threshold=%.4f)", phrase.Phrase, dist, denyThreshold)
				var msg string
				if showAssessment {
					msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is too similar to denied phrase","distance":%.4f`, dist)
					if phrase.Phrase != "" {
						msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is too similar to denied phrase '%s'","distance":%.4f`, phrase.Phrase, dist)
					}
					msg += "}"
				} else {
					msg = `{"error":"prompt_blocked","message":"request denied"}`
				}
				return policy.ImmediateResponse{
					StatusCode: http.StatusForbidden,
					Headers:    map[string]string{"content-type": "application/json"},
					Body:       []byte(msg),
				}
			}
		} else if err != nil {
			log.Printf("[SemanticPromptGuard] Error calculating distance to denied phrases: %v", err)
			return policy.ImmediateResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"content-type": "application/json"},
				Body:       []byte(fmt.Sprintf(`{"error":"invalid configuration","message":"%s"}`, err.Error())),
			}
		}
		log.Printf("[SemanticPromptGuard] ALLOWED: prompt does not match denied phrases")
		return nil
	} else if len(allowedPhrases) > 0 && len(deniedPhrases) == 0 {
		// Only allowed list: allow if matches allowed phrases, block otherwise
		allowedDist, phrase, err := minDistance(promptEmbedding, allowedPhrases)
		if err != nil {
			log.Printf("[SemanticPromptGuard] Error calculating distance to allowed phrases: %v", err)
			return policy.ImmediateResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"content-type": "application/json"},
				Body:       []byte(fmt.Sprintf(`{"error":"invalid configuration","message":"%s"}`, err.Error())),
			}
		}
		log.Printf("[SemanticPromptGuard] Checking allowed phrases only: min distance=%.4f, threshold=%.4f", allowedDist, allowThreshold)
		if allowedDist <= allowThreshold {
			log.Printf("[SemanticPromptGuard] ALLOWED: prompt matches allowed phrase '%s' (distance=%.4f <= threshold=%.4f)", phrase.Phrase, allowedDist, allowThreshold)
			return nil
		}
		log.Printf("[SemanticPromptGuard] BLOCKED: prompt does not match allowed phrases (distance=%.4f > threshold=%.4f)", allowedDist, allowThreshold)
		var msg string
		if showAssessment {
			msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is not close enough to allowed phrases","distance":%.4f`, allowedDist)
			if phrase.Phrase != "" {
				msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is not close enough to allowed phrase '%s'","distance":%.4f`, phrase.Phrase, allowedDist)
			}
			msg += "}"
		} else {
			msg = `{"error":"prompt_blocked","message":"request denied"}`
		}
		return policy.ImmediateResponse{
			StatusCode: http.StatusForbidden,
			Headers:    map[string]string{"content-type": "application/json"},
			Body:       []byte(msg),
		}
	} else {
		// Both allowed and denied lists are configured: check both
		if dist, phrase, err := minDistance(promptEmbedding, deniedPhrases); err == nil {
			log.Printf("[SemanticPromptGuard] Checking denied phrases: min distance=%.4f, threshold=%.4f", dist, denyThreshold)
			if dist <= denyThreshold {
				log.Printf("[SemanticPromptGuard] BLOCKED: prompt too similar to denied phrase '%s' (distance=%.4f <= threshold=%.4f)", phrase.Phrase, dist, denyThreshold)
				var msg string
				if showAssessment {
					msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is too similar to denied phrase","distance":%.4f`, dist)
					if phrase.Phrase != "" {
						msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is too similar to denied phrase '%s'","distance":%.4f`, phrase.Phrase, dist)
					}
					msg += "}"
				} else {
					msg = `{"error":"prompt_blocked","message":"request denied"}`
				}
				return policy.ImmediateResponse{
					StatusCode: http.StatusForbidden,
					Headers:    map[string]string{"content-type": "application/json"},
					Body:       []byte(msg),
				}
			}
		} else if err != nil {
			log.Printf("[SemanticPromptGuard] Error calculating distance to denied phrases: %v", err)
			return policy.ImmediateResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"content-type": "application/json"},
				Body:       []byte(fmt.Sprintf(`{"error":"invalid configuration","message":"%s"}`, err.Error())),
			}
		}

		log.Printf("[SemanticPromptGuard] Prompt passed denied phrases check, now checking allowed phrases")
		allowedDist, phrase, err := minDistance(promptEmbedding, allowedPhrases)
		if err != nil {
			log.Printf("[SemanticPromptGuard] Error calculating distance to allowed phrases: %v", err)
			return policy.ImmediateResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"content-type": "application/json"},
				Body:       []byte(fmt.Sprintf(`{"error":"invalid configuration","message":"%s"}`, err.Error())),
			}
		}
		log.Printf("[SemanticPromptGuard] Checking allowed phrases: min distance=%.4f, threshold=%.4f", allowedDist, allowThreshold)
		if allowedDist <= allowThreshold {
			log.Printf("[SemanticPromptGuard] ALLOWED: prompt matches allowed phrase '%s' (distance=%.4f <= threshold=%.4f)", phrase.Phrase, allowedDist, allowThreshold)
			return nil
		}
		log.Printf("[SemanticPromptGuard] BLOCKED: prompt does not match allowed phrases (distance=%.4f > threshold=%.4f)", allowedDist, allowThreshold)
		var msg string
		if showAssessment {
			msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is not close enough to allowed phrases","distance":%.4f`, allowedDist)
			if phrase.Phrase != "" {
				msg = fmt.Sprintf(`{"error":"prompt_blocked","message":"prompt is not close enough to allowed phrase '%s'","distance":%.4f`, phrase.Phrase, allowedDist)
			}
			msg += "}"
		} else {
			msg = `{"error":"prompt_blocked","message":"request denied"}`
		}
		return policy.ImmediateResponse{
			StatusCode: http.StatusForbidden,
			Headers:    map[string]string{"content-type": "application/json"},
			Body:       []byte(msg),
		}
	}
	log.Printf("[SemanticPromptGuard] ALLOWED: prompt passed all checks (distance=%.4f > threshold=%.4f, but requireAllowedMatch=false)", allowedDist, allowThreshold)
	return policy.UpstreamRequestModifications{}
}

// OnResponse is not used by this policy.
func (p *SemanticPromptGuardPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil
}

func parseFloatParam(params map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return defaultVal
}

func parseBoolParam(params map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := params[key]; ok {
		if v, ok := val.(bool); ok {
			return v
		}
	}
	return defaultVal
}

func parseDurationParam(params map[string]interface{}, key string, defaultVal time.Duration) time.Duration {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return time.Duration(v) * time.Millisecond
		case int:
			return time.Duration(v) * time.Millisecond
		}
	}
	return defaultVal
}

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
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", fieldName, i)
		}

		var phrase string
		if phraseRaw, ok := entryMap["phrase"]; ok {
			if phraseStr, ok := phraseRaw.(string); ok {
				phrase = phraseStr
			}
		}

		var embedding []float64
		if embeddingRaw, ok := entryMap["embedding"]; ok {
			var err error
			embedding, err = parseEmbeddingVector(embeddingRaw, fmt.Sprintf("%s[%d].embedding", fieldName, i))
			if err != nil {
				return nil, err
			}
		}

		phrases = append(phrases, PhraseEmbedding{
			Phrase:    phrase,
			Embedding: embedding,
		})
	}

	return phrases, nil
}

func parseEmbeddingVector(raw interface{}, fieldName string) ([]float64, error) {
	rawSlice, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of numbers", fieldName)
	}

	if len(rawSlice) == 0 {
		return nil, fmt.Errorf("%s cannot be empty", fieldName)
	}

	vector := make([]float64, 0, len(rawSlice))
	for i, v := range rawSlice {
		switch n := v.(type) {
		case float64:
			vector = append(vector, n)
		case int:
			vector = append(vector, float64(n))
		default:
			return nil, fmt.Errorf("%s[%d] must be a number", fieldName, i)
		}
	}

	return vector, nil
}

func ensureEmbeddings(provider, endpoint string, timeout time.Duration, apiKey, azureDeployment, azureAPIVersion, openAIModel string, phrases []PhraseEmbedding) ([]PhraseEmbedding, error) {
	if len(phrases) == 0 {
		return phrases, nil
	}

	for i := range phrases {
		if len(phrases[i].Embedding) > 0 {
			continue
		}

		text := phrases[i].Phrase
		if text == "" {
			return nil, fmt.Errorf("phrase '%s' is missing embedding and text", phrases[i].Phrase)
		}

		log.Printf("[SemanticPromptGuard] Fetching embedding for phrase: '%s'", text)
		embedding, err := getOrFetchEmbedding(provider, endpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, text, timeout)
		if err != nil {
			return nil, err
		}
		phrases[i].Embedding = embedding
		log.Printf("[SemanticPromptGuard] Successfully fetched embedding for phrase '%s' (dimension: %d)", text, len(embedding))
	}

	return phrases, nil
}

func extractPrompt(body *policy.Body, promptField string) (string, error) {
	if body == nil || !body.Present {
		return "", errors.New("request body is required to extract prompt")
	}

	if len(body.Content) == 0 {
		return "", errors.New("request body is empty")
	}

	if promptField == "" {
		return string(body.Content), nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body.Content, &payload); err != nil {
		return "", fmt.Errorf("failed to parse body JSON: %w", err)
	}

	value, ok := payload[promptField]
	if !ok {
		return "", fmt.Errorf("prompt field '%s' not found in body", promptField)
	}

	prompt, ok := value.(string)
	if !ok || prompt == "" {
		return "", fmt.Errorf("prompt field '%s' must be a non-empty string", promptField)
	}

	return prompt, nil
}

func (p *SemanticPromptGuardPolicy) fetchEmbeddingWithProvider(provider, endpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, prompt string, timeout time.Duration) ([]float64, error) {
	switch strings.ToLower(provider) {
	case "azure-openai":
		return p.callAzureOpenAI(endpoint, azureDeployment, azureAPIVersion, apiKey, prompt, timeout)
	case "openai":
		return p.callOpenAI(endpoint, openAIModel, apiKey, prompt, timeout)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (p *SemanticPromptGuardPolicy) getOrFetchEmbedding(provider, endpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, prompt string, timeout time.Duration) ([]float64, error) {
	cacheKey := fmt.Sprintf("%s|%s|%s|%s|%s|%s", provider, endpoint, azureDeployment, azureAPIVersion, openAIModel, prompt)

	p.embeddingLocker.RLock()
	if emb, ok := p.embeddingCache[cacheKey]; ok {
		p.embeddingLocker.RUnlock()
		log.Printf("[SemanticPromptGuard] Cache HIT for embedding (prompt length: %d)", len(prompt))
		return emb, nil
	}
	p.embeddingLocker.RUnlock()

	log.Printf("[SemanticPromptGuard] Cache MISS, fetching embedding from provider '%s'", provider)
	emb, err := p.fetchEmbeddingWithProvider(provider, endpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, prompt, timeout)
	if err != nil {
		return nil, err
	}

	p.embeddingLocker.Lock()
	p.embeddingCache[cacheKey] = emb
	p.embeddingLocker.Unlock()
	log.Printf("[SemanticPromptGuard] Cached embedding (dimension: %d)", len(emb))
	return emb, nil
}

func (p *SemanticPromptGuardPolicy) fetchEmbedding(provider, endpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, prompt string, timeout time.Duration) ([]float64, error) {
	return p.fetchEmbeddingWithProvider(provider, endpoint, apiKey, azureDeployment, azureAPIVersion, openAIModel, prompt, timeout)
}

func (p *SemanticPromptGuardPolicy) doEmbeddingRequest(endpoint string, body any, timeout time.Duration, headers map[string]string) ([]float64, error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	client := p.client
	if timeout != 0 && timeout != client.Timeout {
		client = &http.Client{Timeout: timeout}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	log.Printf("[SemanticPromptGuard] Sending embedding request to %s", endpoint)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[SemanticPromptGuard] Embedding request failed: %v", err)
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[SemanticPromptGuard] Embedding response status: %d", resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding endpoint returned status %d", resp.StatusCode)
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Embedding []float64 `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		log.Printf("[SemanticPromptGuard] Failed to parse embedding response: %v", err)
		return nil, fmt.Errorf("failed to parse embedding response: %w", err)
	}

	var embedding []float64
	switch {
	case len(parsed.Embedding) > 0:
		embedding = parsed.Embedding
	case len(parsed.Data) > 0 && len(parsed.Data[0].Embedding) > 0:
		embedding = parsed.Data[0].Embedding
	default:
		return nil, errors.New("embedding response did not contain embedding data")
	}

	log.Printf("[SemanticPromptGuard] Successfully received embedding (dimension: %d)", len(embedding))
	return embedding, nil
}

func (p *SemanticPromptGuardPolicy) callAzureOpenAI(baseEndpoint, deployment, apiVersion, apiKey, prompt string, timeout time.Duration) ([]float64, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey is required for azure-openai provider")
	}
	if deployment == "" {
		return nil, errors.New("azureDeployment is required for azure-openai provider")
	}
	if apiVersion == "" {
		apiVersion = "2023-05-15"
	}

	base := strings.TrimSuffix(baseEndpoint, "/")
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid azure endpoint: %s", baseEndpoint)
	}

	urlStr := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s", base, deployment, apiVersion)
	log.Printf("[SemanticPromptGuard] Calling Azure OpenAI: deployment=%s, apiVersion=%s", deployment, apiVersion)
	return p.doEmbeddingRequest(urlStr, map[string]any{"input": prompt}, timeout, map[string]string{
		"api-key": apiKey,
	})
}

func (p *SemanticPromptGuardPolicy) callOpenAI(baseEndpoint, model, apiKey, prompt string, timeout time.Duration) ([]float64, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey is required for openai provider")
	}
	if model == "" {
		return nil, errors.New("openAIModel is required for openai provider")
	}
	if baseEndpoint == "" {
		baseEndpoint = "https://api.openai.com/v1/embeddings"
	}

	log.Printf("[SemanticPromptGuard] Calling OpenAI: model=%s, endpoint=%s", model, baseEndpoint)
	return p.doEmbeddingRequest(baseEndpoint, map[string]any{
		"model": model,
		"input": prompt,
	}, timeout, map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", apiKey),
	})
}

func minDistance(target []float64, phrases []PhraseEmbedding) (float64, PhraseEmbedding, error) {
	if len(phrases) == 0 {
		return math.Inf(1), PhraseEmbedding{}, nil
	}

	minDist := math.Inf(1)
	var closest PhraseEmbedding

	for _, phrase := range phrases {
		dist, err := cosineDistance(target, phrase.Embedding)
		if err != nil {
			return 0, PhraseEmbedding{}, err
		}

		if dist < minDist {
			minDist = dist
			closest = phrase
		}
	}

	return minDist, closest, nil
}

func cosineDistance(a, b []float64) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, errors.New("embedding vectors cannot be empty")
	}

	if len(a) != len(b) {
		return 0, fmt.Errorf("embedding dimensions do not match: %d vs %d", len(a), len(b))
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0, errors.New("embedding vector norm is zero")
	}

	return 1 - (dot / (math.Sqrt(normA) * math.Sqrt(normB))), nil
}

func getString(params map[string]interface{}, key string) string {
	if val, ok := params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
