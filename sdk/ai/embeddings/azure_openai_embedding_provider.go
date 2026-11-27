package embeddings

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/wso2/api-platform/sdk/ai/azureauthentication"
)

// tokenSource gets Entra ID access tokens for the gateway's identity. *azureauthentication.Credential
// is one; unit tests fake it.
type tokenSource interface {
	Token(ctx context.Context, scope string, timeout time.Duration) (string, error)
}

// getTokenSource returns the gateway-wide credential for id, which every provider with the same
// identity shares. Tests replace it.
var getTokenSource = func(id azureauthentication.Identity) (tokenSource, error) {
	cred, err := azureauthentication.Get(id)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

// StatusError is the error for a non-200 answer from Azure OpenAI, so callers can tell, for
// example, a refused credential (401 or 403) from other failures.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("Azure OpenAI API error: status %d, body: %s", e.StatusCode, e.Body)
}

// AzureOpenAIEmbeddingProvider implements the EmbeddingProvider interface for Azure OpenAI
type AzureOpenAIEmbeddingProvider struct {
	authHeaderName string
	azureAPIKey    string
	endpointURL    string
	client         *http.Client
	tokens         tokenSource // identity modes only
	tokenScope     string      // identity modes only
}

// Init initializes the Azure OpenAI embedding provider with configuration
func (a *AzureOpenAIEmbeddingProvider) Init(config EmbeddingProviderConfig) error {
	err := ValidateEmbeddingProviderConfigProps(config)
	if err != nil {
		return fmt.Errorf("invalid embedding provider config properties: %v", err)
	}
	if config.UsesIdentity() {
		tokens, err := getTokenSource(azureauthentication.Identity{Type: config.AuthenticationType, ClientID: config.ClientID})
		if err != nil {
			return fmt.Errorf("authentication type %q can't be used: %w", config.AuthenticationType, err)
		}
		a.tokens = tokens
		a.tokenScope = cmp.Or(config.TokenScope, DefaultTokenScope)
		a.azureAPIKey = ""
	} else {
		a.tokens = nil
		a.azureAPIKey = config.APIKey
	}
	a.endpointURL = config.EmbeddingEndpoint
	a.authHeaderName = config.AuthHeaderName
	timeout := DefaultRequestTimeout // Use DefaultRequestTimeout (in seconds)
	if v, err := strconv.Atoi(config.TimeOut); err == nil {
		timeout = v
	}

	a.client = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		// Refuse every redirect: on a redirect to another host Go keeps custom headers such as
		// api-key, so following one could send the credential to a resource nobody configured.
		// The 3xx comes back as the response and fails as any other non-200 does.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return nil
}

// authorize sets the request's credentials: the key header in api-key mode, otherwise a Bearer
// token for the gateway's Azure identity, fetched for this call. The error of a failed token fetch
// keeps its *azureauthentication.TokenError, and never contains a token.
func (a *AzureOpenAIEmbeddingProvider) authorize(req *http.Request) error {
	if a.tokens == nil {
		req.Header.Set(a.authHeaderName, a.azureAPIKey)
		return nil
	}
	token, err := a.tokens.Token(context.Background(), a.tokenScope, azureauthentication.DefaultTokenTimeout)
	if err != nil {
		return fmt.Errorf("could not obtain an access token for the embedding provider: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// GetType returns the type of the embedding provider
func (a *AzureOpenAIEmbeddingProvider) GetType() string {
	return "AZURE_OPENAI"
}

// GetEmbedding generates an embedding vector for a single input text, with strict response checks
func (a *AzureOpenAIEmbeddingProvider) GetEmbedding(input string) ([]float32, error) {
	requestBody := map[string]interface{}{
		"input": input,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", a.endpointURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	if err := a.authorize(req); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	dataArr, ok := response["data"].([]interface{})
	if !ok || len(dataArr) == 0 {
		return nil, fmt.Errorf("invalid response structure: data field missing, invalid, or empty")
	}

	dataMap, ok := dataArr[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: data[0] is not an object")
	}

	embeddingRaw, ok := dataMap["embedding"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: data[0].embedding field missing or invalid")
	}

	embeddingResult := make([]float32, len(embeddingRaw))
	for i, value := range embeddingRaw {
		floatVal, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid embedding value at embedding[%d]: not a number", i)
		}
		embeddingResult[i] = float32(floatVal)
	}

	return embeddingResult, nil
}

// GetEmbeddings generates embedding vectors for multiple input texts
func (a *AzureOpenAIEmbeddingProvider) GetEmbeddings(inputs []string) ([][]float32, error) {
	requestBody := map[string]interface{}{
		"input": inputs,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", a.endpointURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	if err := a.authorize(req); err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Safely extract data field
	dataField, ok := response["data"]
	if !ok || dataField == nil {
		return nil, fmt.Errorf("missing 'data' field in response")
	}

	data, ok := dataField.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'data' field: expected array")
	}

	var embeddings [][]float32
	for idx, dataNode := range data {
		dataMap, ok := dataNode.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid data array element at index %d: expected object", idx)
		}

		embeddingField, ok := dataMap["embedding"]
		if !ok || embeddingField == nil {
			return nil, fmt.Errorf("missing 'embedding' field in response data at index %d", idx)
		}

		embedding, ok := embeddingField.([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid 'embedding' field at index %d: expected array", idx)
		}

		embeddingResult := make([]float32, len(embedding))
		for i, value := range embedding {
			val, ok := value.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid embedding value at index %d[%d]: expected number", idx, i)
			}
			embeddingResult[i] = float32(val)
		}
		embeddings = append(embeddings, embeddingResult)
	}

	return embeddings, nil
}
