/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
)

// respondCatalogError logs err and writes it as the standard ErrorResponse when
// it is an *apperror.Error, reporting whether it did so the caller can simply
// return.
//
// The services these handlers call build their failures from the error catalog,
// so the error already carries the HTTP status, the stable machine-readable
// code, and a client-sterile message. Responding through apperror.LogAndWrite —
// the same helper the main error mapper uses — keeps both the wire format and
// the log severity from drifting between the plugin and the rest of the API.
// That matters most for a 5xx: this plugin does not run behind the error
// mapper, so writing the response directly (as an earlier version did) served a
// 500 with no log line and no stack trace at all.
//
// Every client-facing condition these services raise — including the WebSub,
// WebBroker, and HMAC-secret ones — now comes from the catalog, so a false
// return means the error is an unmapped internal failure and the caller should
// log it and answer 500.
func respondCatalogError(w http.ResponseWriter, slogger *slog.Logger, err error) bool {
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		return false
	}
	apperror.LogAndWrite(w, slogger, apperror.FromError(err))
	return true
}
