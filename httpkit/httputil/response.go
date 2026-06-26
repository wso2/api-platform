package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// WriteJSON encodes v as JSON and writes it with the given status code.
// It encodes into a buffer first so that a marshal error does not leave a
// partial response body behind.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, `{"code":"internal_error","message":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// ErrorBody is the standard error envelope written by WriteError.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes a JSON error body { "code": ..., "message": ... }.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{Code: code, Message: message})
}

// WriteNoContent writes a 204 No Content response with no body.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
