package main

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type invocationContext struct {
	Path string `json:"path,omitempty"`
}

type handleRequestBody struct {
	InvocationContext invocationContext `json:"invocationContext"`
}

type handleResponseBody struct {
	InvocationContext  invocationContext `json:"invocationContext"`
	InterceptorContext map[string]string `json:"interceptorContext,omitempty"`
}

type interceptorReply struct {
	DirectRespond      bool              `json:"directRespond,omitempty"`
	ResponseCode       int               `json:"responseCode,omitempty"`
	HeadersToAdd       map[string]string `json:"headersToAdd,omitempty"`
	PathToRewrite      string            `json:"pathToRewrite,omitempty"`
	Body               string            `json:"body,omitempty"`
	InterceptorContext map[string]string `json:"interceptorContext,omitempty"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/handle-request", handleRequest)
	mux.HandleFunc("/handle-response", handleResponse)

	log.Println("mock-interceptor-service listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "mock-interceptor-service",
	})
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req handleRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	path := req.InvocationContext.Path
	switch {
	case strings.Contains(path, "/block"):
		respondJSON(w, http.StatusOK, interceptorReply{
			DirectRespond: true,
			ResponseCode:  http.StatusForbidden,
			HeadersToAdd: map[string]string{
				"Content-Type":              "application/json",
				"X-Interceptor-Decision":    "blocked",
				"X-Interceptor-RequestHook": "true",
			},
			Body: base64.StdEncoding.EncodeToString([]byte(`{"error":"blocked by interceptor"}`)),
		})
	case strings.Contains(path, "/mutate"):
		respondJSON(w, http.StatusOK, interceptorReply{
			HeadersToAdd: map[string]string{
				"X-Interceptor-Request": "true",
			},
			PathToRewrite: "/anything/intercepted",
			Body:          base64.StdEncoding.EncodeToString([]byte(`{"message":"mutated-by-interceptor"}`)),
			InterceptorContext: map[string]string{
				"trace": "request-phase",
			},
		})
	case strings.Contains(path, "/response-rewrite"):
		respondJSON(w, http.StatusOK, interceptorReply{
			InterceptorContext: map[string]string{
				"trace": "request-phase",
			},
		})
	default:
		respondJSON(w, http.StatusOK, interceptorReply{})
	}
}

func handleResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req handleResponseBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	path := req.InvocationContext.Path
	if strings.Contains(path, "/response-rewrite") {
		trace := req.InterceptorContext["trace"]
		if trace == "" {
			trace = "missing"
		}
		respondJSON(w, http.StatusOK, interceptorReply{
			ResponseCode: http.StatusAccepted,
			HeadersToAdd: map[string]string{
				"Content-Type":           "application/json",
				"X-Interceptor-Response": "true",
				"X-Interceptor-Trace":    trace,
			},
			Body: base64.StdEncoding.EncodeToString([]byte(`{"message":"response-overridden"}`)),
		})
		return
	}

	respondJSON(w, http.StatusOK, interceptorReply{})
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
