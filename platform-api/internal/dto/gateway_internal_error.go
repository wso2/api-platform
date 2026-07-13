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

package dto

// InternalErrorResponse is the error body of the Gateway Internal API
// (/api/internal/v1/*), as declared by the ErrorResponse schema in
// resources/gateway-internal-api.yaml: an integer HTTP status in Code, a
// short reason phrase in Message, and the specific detail in Description.
//
// It is deliberately NOT apperror.ErrorResponse. The public API
// (resources/openapi.yaml) returns {status, code, message} where code is a
// stable machine-readable catalog string; the gateway data plane consumes
// this older {code, message, description} contract instead. Keeping the two
// shapes in separate types is what stops a change to one from silently
// rewriting the wire format of the other — which is exactly what happened
// when both surfaces shared a single ErrorResponse struct.
type InternalErrorResponse struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
}

// NewInternalErrorResponse builds a Gateway Internal API error body. code is
// the HTTP status, message the reason phrase ("Not Found"), and the optional
// description the specific detail ("API not found").
func NewInternalErrorResponse(code int, message string, description ...string) InternalErrorResponse {
	resp := InternalErrorResponse{Code: code, Message: message}
	if len(description) > 0 {
		resp.Description = description[0]
	}
	return resp
}
