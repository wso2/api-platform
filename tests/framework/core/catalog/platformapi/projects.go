/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package platformapi

import (
	"encoding/json"
	"net/http"
)

// IsLastProjectRetention reports whether a project DELETE was refused because the project is
// its organization's last one. The control plane always keeps one project per organization,
// so a cleanup sweep treats this refusal as the project being intentionally retained.
func IsLastProjectRetention(status int, body []byte) bool {
	if status != http.StatusBadRequest {
		return false
	}
	var refusal struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &refusal); err != nil {
		return false
	}
	return refusal.Code == "VALIDATION_FAILED" && refusal.Message == "Organization must have at least one project"
}
