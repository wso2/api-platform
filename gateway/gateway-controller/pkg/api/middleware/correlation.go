/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package middleware

import (
	"log/slog"
	"net/http"

	gohttpkit "github.com/wso2/go-httpkit/middleware"
)

const (
	// CorrelationIDHeader is the HTTP header name for correlation ID.
	CorrelationIDHeader = gohttpkit.CorrelationIDHeader
)

// CorrelationIDMiddleware reads or generates X-Correlation-ID, stores it and a
// correlation-aware *slog.Logger in the request context, and echoes the ID in
// the response header.
//
// Must be the outermost middleware so all subsequent handlers have access to
// a correlation-aware logger.
func CorrelationIDMiddleware(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	return gohttpkit.CorrelationIDMiddleware(baseLogger)
}

// GetCorrelationID returns the correlation ID stored by CorrelationIDMiddleware.
func GetCorrelationID(r *http.Request) string {
	return gohttpkit.GetCorrelationID(r)
}

// GetLogger returns the correlation-aware logger stored by CorrelationIDMiddleware,
// falling back to fallback when the middleware has not run.
func GetLogger(r *http.Request, fallback *slog.Logger) *slog.Logger {
	return gohttpkit.GetLogger(r, fallback)
}
