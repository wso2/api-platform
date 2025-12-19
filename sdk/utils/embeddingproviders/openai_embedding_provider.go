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

// OpenAIEmbeddingProvider implements the EmbeddingProvider interface for OpenAI
type OpenAIEmbeddingProvider struct {
	authHeaderName string
	openAiAPIKey   string
	endpointURL    string
	model          string
	client         *http.Client
}

// Init initializes the OpenAI embedding provider with configuration
func (o *OpenAIEmbeddingProvider) Init(config EmbeddingProviderConfig) error {
	err := ValidateEmbeddingProviderConfigProps(config)
	if err != nil {
		return fmt.Errorf("invalid embedding provider config properties: %v", err)
	}
	o.openAiAPIKey = config.APIKey
	o.endpointURL = config.EmbeddingEndpoint
	o.model = config.EmbeddingModel
	o.authHeaderName = config.AuthHeaderName
	timeout := DefaultRequestTimeout // Use DefaultRequestTimeout (in seconds)
	if v, err := strconv.Atoi(config.TimeOut); err == nil {
		timeout = v
	}

	o.client = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	return nil
}

// GetType returns the type of the embedding provider
func (o *OpenAIEmbeddingProvider) GetType() string {
	return "OPENAI"
}

// GetEmbedding generates an embedding vector for a single input text
func (o *OpenAIEmbeddingProvider) GetEmbedding(input string) ([]float32, error) {
	requestBody := map[string]interface{}{
		"model": o.model,
		"input": input,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", o.endpointURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(o.authHeaderName, "Bearer "+o.openAiAPIKey) // Header should be "Authorization"
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
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
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for error in response
	if errorObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := "unknown error"
		if msg, ok := errorObj["message"].(string); ok {
			errorMsg = msg
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errorMsg)
	}

	// Safely extract data field
	dataField, ok := response["data"]
	if !ok || dataField == nil {
		return nil, fmt.Errorf("missing 'data' field in response")
	}

	dataArray, ok := dataField.([]interface{})
	if !ok || len(dataArray) == 0 {
		return nil, fmt.Errorf("invalid 'data' field: expected non-empty array")
	}

	data, ok := dataArray[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid data array element: expected object")
	}

	embeddingField, ok := data["embedding"]
	if !ok || embeddingField == nil {
		return nil, fmt.Errorf("missing 'embedding' field in response data")
	}

	embedding, ok := embeddingField.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid 'embedding' field: expected array")
	}

	embeddingResult := make([]float32, len(embedding))
	for i, value := range embedding {
		val, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid embedding value at index %d: expected number", i)
		}
		embeddingResult[i] = float32(val)
	}

	return embeddingResult, nil
}

// GetEmbeddings generates embedding vectors for multiple input texts
func (o *OpenAIEmbeddingProvider) GetEmbeddings(inputs []string) ([][]float32, error) {
	requestBody := map[string]interface{}{
		"model": o.model,
		"input": inputs,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", o.endpointURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set(o.authHeaderName, "Bearer "+o.openAiAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
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
		return nil, fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for error in response
	if errorObj, ok := response["error"].(map[string]interface{}); ok {
		errorMsg := "unknown error"
		if msg, ok := errorObj["message"].(string); ok {
			errorMsg = msg
		}
		return nil, fmt.Errorf("OpenAI API error: %s", errorMsg)
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
