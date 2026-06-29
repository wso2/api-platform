package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DecodeJSON decodes the JSON request body into T.
// Returns a descriptive error for EOF, syntax errors, and type mismatches
// that callers can map to a 400 response.
func DecodeJSON[T any](r *http.Request) (T, error) {
	var v T
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		var syntaxErr *json.SyntaxError
		var unmarshalErr *json.UnmarshalTypeError
		switch {
		case errors.Is(err, io.EOF):
			return v, fmt.Errorf("request body is empty")
		case errors.As(err, &syntaxErr):
			return v, fmt.Errorf("malformed JSON at position %d", syntaxErr.Offset)
		case errors.As(err, &unmarshalErr):
			return v, fmt.Errorf("field %q expects %s, got %s", unmarshalErr.Field, unmarshalErr.Type, unmarshalErr.Value)
		default:
			return v, err
		}
	}
	return v, nil
}

// PathParam returns the named path parameter from the Go 1.22 ServeMux.
// Equivalent to r.PathValue(key).
func PathParam(r *http.Request, key string) string {
	return r.PathValue(key)
}

// QueryParam returns a single query string value.
func QueryParam(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}
