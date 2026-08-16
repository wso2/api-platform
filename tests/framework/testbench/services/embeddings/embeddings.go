/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package embeddings provides a deterministic OpenAI-compatible embedding service
// for integration tests.
package embeddings

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

// Port is the container port used by the testbench.
const Port = 3004

// Dimension is the length of each embedding vector.
const Dimension = 1536

// request is an OpenAI embedding request.
type request struct {
	Input any    `json:"input"`
	Model string `json:"model"`
}

type response struct {
	Object string          `json:"object"`
	Data   []embeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  usage           `json:"usage"`
}

type embeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Service implements testbench.Service.
type Service struct{}

// New returns a new embeddings service.
func New() *Service { return &Service{} }

// Name returns the service registration name.
func (s *Service) Name() string { return "embeddings" }

// Port returns the service's listening port.
func (s *Service) Port() int { return Port }

// Stateful reports whether the service keeps request-specific state.
func (s *Service) Stateful() bool { return false }

// Handler returns the embedding, health, and debug endpoints.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/v1/embeddings", s.embeddings)
	mux.HandleFunc("/debug/similarity", s.debugSimilarity)
	mux.HandleFunc("/debug/embedding", s.debugEmbedding)
	return mux
}

func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !getOnly(w, r) {
		return
	}
	writeJSON(w, map[string]string{
		"status":  "ok",
		"service": "embeddings",
	})
}

func (s *Service) embeddings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req request
	if err := decodeJSON(r.Body, &req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	inputs, err := parseInputs(req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Preserve the simulated provider-error scenario used by integration tests.
	for _, input := range inputs {
		lower := strings.ToLower(input)
		if strings.Contains(lower, "error") && strings.Contains(lower, "simulate") {
			http.Error(w, "Simulated embedding provider error", http.StatusInternalServerError)
			return
		}
	}

	data := make([]embeddingData, len(inputs))
	totalTokens := 0
	for i, input := range inputs {
		data[i] = embeddingData{
			Object:    "embedding",
			Embedding: vector(input),
			Index:     i,
		}
		tokens := len(input) / 4
		if tokens < 1 {
			tokens = 1
		}
		totalTokens += tokens
	}

	writeJSON(w, response{
		Object: "list",
		Data:   data,
		Model:  req.Model,
		Usage:  usage{PromptTokens: totalTokens, TotalTokens: totalTokens},
	})
}

// debugSimilarity reports the cosine similarity of two texts.
func (s *Service) debugSimilarity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !getOnly(w, r) {
		return
	}

	text1 := r.URL.Query().Get("text1")
	text2 := r.URL.Query().Get("text2")
	if text1 == "" || text2 == "" {
		http.Error(w, "Both text1 and text2 query parameters are required", http.StatusBadRequest)
		return
	}

	first, second := vector(text1), vector(text2)
	var dot float64
	for i := 0; i < Dimension; i++ {
		dot += float64(first[i] * second[i])
	}

	writeJSON(w, map[string]any{
		"text1":      text1,
		"text2":      text2,
		"similarity": dot,
	})
}

// debugEmbedding reports the normalized text and selected vector details.
func (s *Service) debugEmbedding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !getOnly(w, r) {
		return
	}

	text := r.URL.Query().Get("text")
	if text == "" {
		http.Error(w, "text query parameter is required", http.StatusBadRequest)
		return
	}

	embedding := vector(text)
	normalized := normalize(text)
	hash := sha256.Sum256([]byte(normalized))

	writeJSON(w, map[string]any{
		"text":           text,
		"normalized":     normalized,
		"hash_hex":       fmt.Sprintf("%x", hash),
		"embedding_size": len(embedding),
		"first_10":       embedding[:10],
		"last_10":        embedding[len(embedding)-10:],
	})
}

func decodeJSON(r io.Reader, dst any) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}

func getOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

// parseInputs accepts OpenAI's two input forms and nothing else.
func parseInputs(input any) ([]string, error) {
	switch v := input.(type) {
	case string:
		return []string{v}, nil
	case []any:
		inputs := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input array element %d is not a string", i)
			}
			inputs[i] = str
		}
		return inputs, nil
	default:
		return nil, fmt.Errorf("input must be a string or array of strings")
	}
}

// normalize trims and lowercases input before vectorization.
func normalize(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

// vector derives a deterministic unit vector from normalized words.
func vector(input string) []float32 {
	words := strings.Fields(normalize(input))

	embedding := make([]float32, Dimension)
	if len(words) == 0 {
		return embedding
	}

	for _, word := range words {
		wordHash := sha256.Sum256([]byte(word))
		for i := 0; i < Dimension; i++ {
			embedding[i] += float32(int8(wordHash[i%32])) / 128.0
		}
	}

	for i := 0; i < Dimension; i++ {
		embedding[i] /= float32(len(words))
		embedding[i] += float32(math.Sin(float64(i)*0.1)) * 0.05
	}

	normalizeVector(embedding)
	return embedding
}

// normalizeVector scales v to unit length, so a dot product is a cosine similarity.
func normalizeVector(v []float32) {
	var sum float64
	for _, val := range v {
		sum += float64(val * val)
	}
	magnitude := float32(math.Sqrt(sum))
	if magnitude == 0 {
		return
	}
	for i := range v {
		v[i] /= magnitude
	}
}

// writeJSON encodes a response body after the caller sets its status and content type.
func writeJSON(w http.ResponseWriter, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("embeddings: failed to encode response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("embeddings: failed to write response: %v", err)
	}
}
