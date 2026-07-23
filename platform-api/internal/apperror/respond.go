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

package apperror

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// FromError coerces err into an *Error. A non-catalog error becomes a generic
// Internal wrapping it, so internal details never reach the client.
//
// A handler fallback may have wrapped a more specific typed error produced
// deeper in the stack (service layer) in a generic Internal — the loop prefers
// the specific one so service-origin errors keep their code and status.
func FromError(err error) *Error {
	var appErr *Error
	if !errors.As(err, &appErr) {
		return Internal.Wrap(err)
	}
	for appErr.Code == CodeCommonInternalError && appErr.Cause != nil {
		var inner *Error
		if !errors.As(appErr.Cause, &inner) {
			break
		}
		appErr = inner
	}
	return appErr
}

// LogAndWrite is the single implementation of the log-then-respond severity
// split, shared by middleware.MapErrors and the event-gateway plugin's
// respondCatalogError so the two paths cannot drift.
//
// A 4xx is a client outcome, not a system fault — it logs at WARN without the
// stack, and no tracking ID is exposed. A 5xx is a system fault — it logs at
// ERROR with the origin stack captured at the Def.New/Def.Wrap call site, and
// the tracking ID is echoed in the response body so the client can quote it
// back for correlation.
//
// extra supplies additional slog key/value pairs (e.g. "path", "method") and
// is appended after the status field so field order stays stable across
// callers.
func LogAndWrite(w http.ResponseWriter, slogger *slog.Logger, e *Error, extra ...any) {
	trackID := uuid.NewString()

	logFields := []any{
		"trackingId", trackID,
		"code", e.Code,
		"status", e.HTTPStatus,
	}
	logFields = append(logFields, extra...)
	if e.LogMessage != "" {
		logFields = append(logFields, "detail", e.LogMessage)
	}
	if e.Cause != nil {
		logFields = append(logFields, "cause", e.Cause.Error())
	}
	// origin is the file:line where the error was actually constructed
	// (Def.New/Def.Wrap call site) — not to be confused with slog's own
	// "source" attribute, which always points at this log call.
	if origin := e.Origin(); origin != "" {
		logFields = append(logFields, "origin", origin)
	}

	if e.HTTPStatus >= http.StatusInternalServerError {
		if len(e.Stack) > 0 {
			logFields = append(logFields, "stack", e.StackString())
		}
		slogger.Error("request failed", logFields...)
		WriteHTTP(w, e, trackID)
		return
	}

	slogger.Warn("request failed", logFields...)
	WriteHTTP(w, e, "")
}
