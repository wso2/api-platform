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
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/utils"

	"github.com/wso2/go-httpkit/httputil"
)

// WriteHTTP is the one place in the codebase that serializes an *Error onto
// an http.ResponseWriter as the standard utils.ErrorResponse shape. Both the
// error mapper (middleware.MapErrors) and the pre-routing auth middleware
// call it, so the wire format cannot drift between the two paths.
//
// trackingID, when non-empty, is echoed in the response body so the client
// can quote it back for log correlation; the mapper passes it only for 5xx
// responses. Callers with no correlation ID to expose pass "".
func WriteHTTP(w http.ResponseWriter, e *Error, trackingID string) {
	httputil.WriteJSON(w, e.HTTPStatus, utils.ErrorResponse{
		Status:     "error",
		Code:       e.Code,
		Message:    e.Message,
		Errors:     e.FieldErrors,
		Details:    e.Details,
		TrackingID: trackingID,
	})
}
