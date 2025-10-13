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
