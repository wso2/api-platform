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

package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/google/uuid"
)

// ErrorHandlerFunc is the handler signature for routes that participate in
// centralized error mapping: instead of writing HTTP error responses inline,
// the handler returns an error (ideally an *apperror.Error) and MapErrors
// logs it and writes the standard utils.ErrorResponse. Success responses are
// still written directly by the handler — the mapper only owns the error path.
type ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request) error

// MapErrors adapts an ErrorHandlerFunc to http.HandlerFunc for registration
// on the mux. It is the single catch point for handler errors: it recovers
// panics into a structured 500, maps *apperror.Error values onto the wire
// via apperror.WriteHTTP, and collapses any other error into a generic 500
// COMMON_INTERNAL_ERROR so internal details never reach the client.
func MapErrors(slogger *slog.Logger, next ErrorHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				trackID := uuid.NewString()
				slogger.Error("panic recovered",
					"trackingId", trackID,
					"panic", rec,
					"path", r.URL.Path,
					"method", r.Method,
					"stack", string(debug.Stack()))
				apperror.WriteHTTP(w, apperror.Internal.New(), trackID)
			}
		}()

		if err := next(w, r); err != nil {
			writeMappedError(w, r, slogger, err)
		}
	}
}

// writeMappedError logs the failure with its internal diagnostics (log
// message, cause, origin stack, tracking ID) and writes the sanitized
// client-facing response. Errors that are not *apperror.Error fall back to a
// generic 500 per the "zero internal details" rule in error-handling.md.
//
// Severity split: a 4xx is a client outcome, not a system fault — it logs at
// WARN without the stack. A 5xx is a system fault — it logs at ERROR with the
// origin stack, and the tracking ID is echoed in the response body so the
// client can quote it back for correlation.
func writeMappedError(w http.ResponseWriter, r *http.Request, slogger *slog.Logger, err error) {
	trackID := uuid.NewString()
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		appErr = apperror.Internal.Wrap(err)
	}
	// A handler fallback may have wrapped a more specific typed error produced
	// deeper in the stack (service layer) in a generic Internal — prefer the
	// specific one so service-origin errors keep their code and status.
	for appErr.Code == utils.CodeCommonInternalError && appErr.Cause != nil {
		var inner *apperror.Error
		if !errors.As(appErr.Cause, &inner) {
			break
		}
		appErr = inner
	}

	logFields := []any{
		"trackingId", trackID,
		"code", appErr.Code,
		"status", appErr.HTTPStatus,
		"path", r.URL.Path,
		"method", r.Method,
	}
	if appErr.LogMessage != "" {
		logFields = append(logFields, "detail", appErr.LogMessage)
	}
	if appErr.Cause != nil {
		logFields = append(logFields, "cause", appErr.Cause.Error())
	}

	if appErr.HTTPStatus >= http.StatusInternalServerError {
		if len(appErr.Stack) > 0 {
			logFields = append(logFields, "stack", appErr.StackString())
		}
		slogger.Error("request failed", logFields...)
		apperror.WriteHTTP(w, appErr, trackID)
		return
	}

	slogger.Warn("request failed", logFields...)
	apperror.WriteHTTP(w, appErr, "")
}
