package embeddingproviders

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// AzureOpenAIEmbeddingProvider implements the EmbeddingProvider interface for Azure OpenAI
type AzureOpenAIEmbeddingProvider struct {
	authHeaderName string
	azureAPIKey    string
	endpointURL    string
	client         *http.Client
}

// Init initializes the Azure OpenAI embedding provider with configuration
func (a *AzureOpenAIEmbeddingProvider) Init(config EmbeddingProviderConfig) error {
	err := ValidateEmbeddingProviderConfigProps(config)
	if err != nil {
		return fmt.Errorf("invalid embedding provider config properties: %v", err)
	}
	a.azureAPIKey = config.APIKey
	a.endpointURL = config.EmbeddingEndpoint
	a.authHeaderName = config.AuthHeaderName
	timeout := DefaultRequestTimeout // Use DefaultRequestTimeout (in seconds)
	if v, err := strconv.Atoi(config.TimeOut); err == nil {
		timeout = v
	}

	a.client = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
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
	req.Header.Set(a.authHeaderName, a.azureAPIKey) // Header should be "api-key"
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
		return nil, fmt.Errorf("Azure OpenAI API error: status %d, body: %s", resp.StatusCode, string(respBody))
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
	req.Header.Set(a.authHeaderName, a.azureAPIKey)
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

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}

	data := response["data"].([]interface{})
	var embeddings [][]float32
	for _, dataNode := range data {
		dataMap := dataNode.(map[string]interface{})
		embedding := dataMap["embedding"].([]interface{})
		embeddingResult := make([]float32, len(embedding))
		for i, value := range embedding {
			embeddingResult[i] = float32(value.(float64))
		}
		embeddings = append(embeddings, embeddingResult)
	}

	return embeddings, nil
}
