/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// ErrorResponse is the standard error response shape mandated by APR-004:
// { status, code, message, errors[], details }. status is always "error";
// code is a stable, machine-readable string from the error catalog (see
// codes.go). Details carries optional structured metadata whose shape is
// specific to the code (e.g. the resources referencing a secret that
// blocked its deletion) — it is not a substitute for errors[], which is
// reserved for field-level validation failures.
type ErrorResponse struct {
	Status  string       `json:"status"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Errors  []FieldError `json:"errors,omitempty"`
	Details any          `json:"details,omitempty"`
}

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewErrorResponse builds a standard ErrorResponse for the given HTTP status.
// title is retained for call-site compatibility but is no longer surfaced in
// the response body; the catalog code is derived from httpStatus (see
// codeForStatus in codes.go), and description[0], when given, becomes the
// human-readable message. Handlers that need a specific catalog code should
// use NewErrorResponseWithCode instead.
func NewErrorResponse(httpStatus int, title string, description ...string) ErrorResponse {
	message := title
	if len(description) > 0 && description[0] != "" {
		message = description[0]
	}
	return NewErrorResponseWithCode(codeForStatus(httpStatus), message)
}

// NewErrorResponseWithCode builds a standard ErrorResponse using an explicit
// catalog code (see codes.go), for handlers that know the precise failure
// reason rather than just the HTTP status.
func NewErrorResponseWithCode(code, message string) ErrorResponse {
	return ErrorResponse{
		Status:  "error",
		Code:    code,
		Message: message,
	}
}

// NewValidationErrorResponse writes a 400 JSON error response for validation errors.
func NewValidationErrorResponse(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fieldErrors := make([]FieldError, 0, len(ve))
		for _, fe := range ve {
			fieldErrors = append(fieldErrors, FieldError{
				Field:   fe.Field(),
				Message: fmt.Sprintf("The field %s is %s", fe.Field(), fe.Tag()),
			})
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Status:  "error",
			Code:    CodeCommonValidationFailed,
			Message: "Validation failed for the request parameters.",
			Errors:  fieldErrors,
		})
		return
	}
	log.Printf("[ERROR] Request validation fallback error: %v", err)
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Status:  "error",
		Code:    CodeCommonValidationFailed,
		Message: "Invalid input.",
	})
}
