/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// Package apperror defines the typed application error that handlers,
// services, and repositories return instead of writing HTTP error responses
// inline. A single mapper at the routing layer (middleware.MapErrors) catches
// the error, logs it, and serializes it onto the wire via WriteHTTP — see
// resources/ERROR_HANDLING_IMPLEMENTATION_PLAN.md.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Error carries everything the error mapper needs to log a failure and write
// the client-facing ErrorResponse: the catalog code and HTTP status,
// the sanitized client message, optional field-level validation errors and
// structured details, plus internal-only diagnostics (log message, wrapped
// cause, and the call stack captured at construction time).
type Error struct {
	Code        string       // catalog code, e.g. CodeCommonValidationFailed
	HTTPStatus  int          // status to write
	Message     string       // client-facing message
	FieldErrors []FieldError // optional, for validation failures
	Details     any          // optional structured metadata
	LogMessage  string       // internal-only detail; never sent to client
	Cause       error        // wrapped original error, for logging/unwrapping
	Stack       []uintptr    // captured at construction time, see New
}

// Error satisfies the error interface. It intentionally includes only the
// code and client message — internal diagnostics stay in LogMessage/Cause.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause to errors.Is/errors.As chains.
func (e *Error) Unwrap() error { return e.Cause }

// NewValidation converts a request-validation failure into an Error,
// mirroring NewValidationErrorResponse: validator.ValidationErrors
// become field-level errors under VALIDATION_FAILED; anything else
// maps to the generic "Invalid input." message with the original error kept
// as the cause for internal logging.
func NewValidation(err error) *Error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fieldErrors := make([]FieldError, 0, len(ve))
		for _, fe := range ve {
			fieldErrors = append(fieldErrors, FieldError{
				Field:   fe.Field(),
				Message: fmt.Sprintf("The field %s is %s", fe.Field(), fe.Tag()),
			})
		}
		return newWithSkip(3, CodeCommonValidationFailed, http.StatusBadRequest,
			"Validation failed for the request parameters.").WithFieldErrors(fieldErrors)
	}
	return newWithSkip(3, CodeCommonValidationFailed, http.StatusBadRequest, "Invalid input.").WithCause(err)
}

// newWithSkip captures the stack skipping the given number of frames so the
// first frame is the exported constructor's caller. It is the only way an
// *Error comes into existence — exported construction goes through the
// catalog (Def.New / Def.Wrap, see catalog.go) or NewValidation, so the
// stack capture can never be skipped and code/status/message always come
// from a declared catalog entry.
//
// LogMessage defaults to the client message so the mapper always has
// something to log, even if the call site never chains WithLogMessage.
// WithLogMessage overrides this default with a more specific internal
// detail when the client message is too generic to be useful for diagnosis
// (e.g. Unauthorized's fixed message).
func newWithSkip(skip int, code string, status int, message string) *Error {
	var pcs [32]uintptr
	n := runtime.Callers(skip, pcs[:])
	return &Error{Code: code, HTTPStatus: status, Message: message, LogMessage: message, Stack: pcs[:n]}
}

// WithLogMessage overrides the default internal-only detail message (see
// newWithSkip) that the mapper logs but never sends to the client.
func (e *Error) WithLogMessage(msg string) *Error {
	e.LogMessage = msg
	return e
}

// WithFieldErrors attaches field-level validation errors surfaced to the
// client in the errors[] array.
func (e *Error) WithFieldErrors(fe []FieldError) *Error {
	e.FieldErrors = fe
	return e
}

// WithDetails attaches optional structured metadata surfaced to the client
// in the details field.
func (e *Error) WithDetails(details any) *Error {
	e.Details = details
	return e
}

// WithCause wraps the original error for logging and errors.Is/As chains.
func (e *Error) WithCause(err error) *Error {
	e.Cause = err
	return e
}

// StackString symbolizes the stack captured at construction time. Kept as
// raw []uintptr on the struct (cheap to capture) and only symbolized lazily
// when the mapper actually logs it, rather than paying runtime.CallersFrames
// cost on every construction.
func (e *Error) StackString() string {
	if len(e.Stack) == 0 {
		return ""
	}
	var sb strings.Builder
	frames := runtime.CallersFrames(e.Stack)
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return sb.String()
}
