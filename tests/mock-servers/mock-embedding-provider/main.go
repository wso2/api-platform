// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
)

const (
	// EmbeddingDimension is the dimension of the embedding vectors (OpenAI ada-002 compatible)
	EmbeddingDimension = 1536
)

// EmbeddingRequest represents the OpenAI-compatible embedding request format
type EmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// EmbeddingResponse represents the OpenAI-compatible embedding response format
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  Usage           `json:"usage"`
}

// EmbeddingData represents a single embedding in the response
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// generateDeterministicEmbedding generates a deterministic embedding based on input text
// Same input always produces the same embedding for test reproducibility
// Similar inputs produce similar embeddings (for semantic similarity testing)
func generateDeterministicEmbedding(input string) []float32 {
	// Normalize input: lowercase and trim whitespace
	normalized := strings.ToLower(strings.TrimSpace(input))

	// Create a hash of the normalized input
	hash := sha256.Sum256([]byte(normalized))

	// Generate embedding vector from hash
	embedding := make([]float32, EmbeddingDimension)

	// Use hash bytes to seed the embedding values
	for i := 0; i < EmbeddingDimension; i++ {
		// Use different parts of the hash for different dimensions
		hashIndex := i % 32
		// Convert hash byte to float in range [-1, 1]
		embedding[i] = float32(int8(hash[hashIndex])) / 128.0

		// Add some variation based on position
		embedding[i] += float32(math.Sin(float64(i)*0.1)) * 0.1
	}

	// Normalize the vector to unit length for cosine similarity
	normalizeVector(embedding)

	return embedding
}

// generateSimilarEmbedding generates an embedding similar to the base embedding
// Used for testing semantic similarity with different threshold values
func generateSimilarEmbedding(input string, similarityLevel float32) []float32 {
	baseEmbedding := generateDeterministicEmbedding(input)

	// Create a perturbation based on input hash
	perturbHash := sha256.Sum256([]byte(input + "_perturb"))

	embedding := make([]float32, EmbeddingDimension)
	for i := 0; i < EmbeddingDimension; i++ {
		// Mix base embedding with perturbation based on similarity level
		perturbValue := float32(int8(perturbHash[i%32])) / 128.0
		embedding[i] = baseEmbedding[i]*similarityLevel + perturbValue*(1-similarityLevel)
	}

	normalizeVector(embedding)
	return embedding
}

// normalizeVector normalizes a vector to unit length
func normalizeVector(v []float32) {
	var sum float64
	for _, val := range v {
		sum += float64(val * val)
	}
	magnitude := float32(math.Sqrt(sum))
	if magnitude > 0 {
		for i := range v {
			v[i] /= magnitude
		}
	}
}

// handleEmbeddings handles the /v1/embeddings endpoint (OpenAI-compatible)
func handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Mock Embedding Provider: Failed to read request body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	log.Printf("Mock Embedding Provider: Received request: %s", string(body))

	var req EmbeddingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Mock Embedding Provider: Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// Check for error simulation keyword
	if strings.Contains(strings.ToLower(req.Input), "error") && strings.Contains(strings.ToLower(req.Input), "simulate") {
		log.Printf("Mock Embedding Provider: Simulating error for input: %s", req.Input)
		http.Error(w, "Simulated embedding provider error", http.StatusInternalServerError)
		return
	}

	// Generate deterministic embedding
	embedding := generateDeterministicEmbedding(req.Input)

	// Calculate approximate token count (rough estimate: 1 token per 4 chars)
	tokenCount := len(req.Input) / 4
	if tokenCount < 1 {
		tokenCount = 1
	}

	response := EmbeddingResponse{
		Object: "list",
		Data: []EmbeddingData{
			{
				Object:    "embedding",
				Embedding: embedding,
				Index:     0,
			},
		},
		Model: req.Model,
		Usage: Usage{
			PromptTokens: tokenCount,
			TotalTokens:  tokenCount,
		},
	}

	log.Printf("Mock Embedding Provider: Generated embedding for input (first 50 chars): %.50s...", req.Input)
	json.NewEncoder(w).Encode(response)
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleSimilarityTest is a debug endpoint to test similarity between two texts
func handleSimilarityTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	text1 := r.URL.Query().Get("text1")
	text2 := r.URL.Query().Get("text2")

	if text1 == "" || text2 == "" {
		http.Error(w, "Both text1 and text2 query parameters are required", http.StatusBadRequest)
		return
	}

	emb1 := generateDeterministicEmbedding(text1)
	emb2 := generateDeterministicEmbedding(text2)

	// Calculate cosine similarity
	var dotProduct float64
	for i := 0; i < EmbeddingDimension; i++ {
		dotProduct += float64(emb1[i] * emb2[i])
	}

	response := map[string]interface{}{
		"text1":      text1,
		"text2":      text2,
		"similarity": dotProduct,
	}

	json.NewEncoder(w).Encode(response)
}

// handleDebugEmbedding returns embedding details for debugging
func handleDebugEmbedding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	text := r.URL.Query().Get("text")
	if text == "" {
		http.Error(w, "text query parameter is required", http.StatusBadRequest)
		return
	}

	embedding := generateDeterministicEmbedding(text)

	// Calculate hash for debugging
	hash := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(text))))

	response := map[string]interface{}{
		"text":           text,
		"normalized":     strings.ToLower(strings.TrimSpace(text)),
		"hash_hex":       fmt.Sprintf("%x", hash),
		"embedding_size": len(embedding),
		"first_10":       embedding[:10],
		"last_10":        embedding[len(embedding)-10:],
	}

	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/v1/embeddings", handleEmbeddings)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/debug/similarity", handleSimilarityTest)
	http.HandleFunc("/debug/embedding", handleDebugEmbedding)

	log.Println("Mock Embedding Provider server listening on :8080")
	log.Println("Endpoints:")
	log.Println("  POST /v1/embeddings - OpenAI-compatible embedding endpoint")
	log.Println("  GET  /health - Health check")
	log.Println("  GET  /debug/similarity?text1=...&text2=... - Test similarity between texts")
	log.Println("  GET  /debug/embedding?text=... - Debug embedding generation")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
