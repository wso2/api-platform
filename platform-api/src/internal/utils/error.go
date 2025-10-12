package utils

// ErrorResponse represents the standard error response format
type ErrorResponse struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(code int, message string, description ...string) ErrorResponse {
	resp := ErrorResponse{
		Code:    code,
		Message: message,
	}
	if len(description) > 0 {
		resp.Description = description[0]
	}
	return resp
}
