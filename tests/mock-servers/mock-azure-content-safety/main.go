package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	validAPIKey = "test-subscription-key"
)

type AnalyzeTextRequest struct {
	Text               string   `json:"text"`
	Categories         []string `json:"categories"`
	HaltOnBlocklistHit bool     `json:"haltOnBlocklistHit"`
	OutputType         string   `json:"outputType"`
}

type CategoryAnalysis struct {
	Category string `json:"category"`
	Severity int    `json:"severity"`
}

type AnalyzeTextResponse struct {
	CategoriesAnalysis []CategoryAnalysis `json:"categoriesAnalysis"`
}

func handleAnalyzeText(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Validate API key
	apiKey := r.Header.Get("Ocp-Apim-Subscription-Key")
	if apiKey == "" || apiKey != validAPIKey {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid or missing API key",
		})
		return
	}

	// Read and parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req AnalyzeTextRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// Check for error simulation keyword
	lowerText := strings.ToLower(req.Text)
	if strings.Contains(lowerText, "error") && strings.Contains(lowerText, "simulate") {
		http.Error(w, "Simulated Azure Content Safety API error", http.StatusInternalServerError)
		return
	}

	// Build response based on content
	response := AnalyzeTextResponse{
		CategoriesAnalysis: make([]CategoryAnalysis, 0),
	}

	// Check each requested category
	for _, category := range req.Categories {
		severity := analyzeCategoryContent(lowerText, category)
		response.CategoriesAnalysis = append(response.CategoriesAnalysis, CategoryAnalysis{
			Category: category,
			Severity: severity,
		})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func analyzeCategoryContent(text, category string) int {
	// Default safe content
	severity := 0

	switch category {
	case "Hate":
		if containsAny(text, []string{"hate", "racist", "discrimination", "bigot"}) {
			severity = 6
		}
	case "Sexual":
		if containsAny(text, []string{"sexual", "explicit", "inappropriate", "nsfw"}) {
			severity = 6
		}
	case "SelfHarm":
		if containsAny(text, []string{"self-harm", "suicide", "cutting"}) {
			severity = 6
		}
	case "Violence":
		if containsAny(text, []string{"violence", "violent", "kill", "murder", "attack"}) {
			severity = 6
		}
	}

	return severity
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func main() {
	http.HandleFunc("/contentsafety/text:analyze", handleAnalyzeText)

	log.Println("Mock Azure Content Safety server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
