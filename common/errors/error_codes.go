package errors

import "encoding/json"

// ErrorCode bundles a numeric code, HTTP status, and message for an error
// that is returned to clients. The numeric code is intentionally opaque —
// clients should not infer root cause from the message; use the code instead.
type ErrorCode struct {
	Code       int
	HTTPStatus int
	Message    string
}

// Policy-engine error codes. 5xxxxx range reserved for gateway errors.
var (
	// ErrCodeRouteNotFound (510001) — no policy chain exists for the requested route.
	ErrCodeRouteNotFound = ErrorCode{Code: 510001, HTTPStatus: 500, Message: "Internal Server Error"}

	// ErrCodePolicyExecutionFailed (510002) — a policy in the chain failed during request/response processing.
	ErrCodePolicyExecutionFailed = ErrorCode{Code: 510002, HTTPStatus: 500, Message: "Internal Server Error"}

	// ErrCodeUnknownRequestType (510003) — the ext_proc request type is not recognised.
	ErrCodeUnknownRequestType = ErrorCode{Code: 510003, HTTPStatus: 500, Message: "Internal Server Error"}
)

// BuildErrorBody returns a JSON-encoded error response body.
// If correlationID is empty it is omitted from the output.
func BuildErrorBody(e ErrorCode, correlationID string) []byte {
	payload := struct {
		Error         string `json:"error"`
		Code          int    `json:"code"`
		CorrelationID string `json:"correlation_id,omitempty"`
	}{
		Error:         e.Message,
		Code:          e.Code,
		CorrelationID: correlationID,
	}
	body, _ := json.Marshal(payload)
	return body
}
