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

// LoggingMiddleware logs each request (method, path, status, latency) after
// the downstream handler completes. Uses the correlation-aware logger stored
// by CorrelationIDMiddleware when available.
//
// Must be registered after CorrelationIDMiddleware.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return gohttpkit.LoggingMiddleware(logger)
}
