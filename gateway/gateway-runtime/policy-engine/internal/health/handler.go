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

package health

import (
	"encoding/json"
	"net/http"
	"time"
)

// body is the JSON document served as the health answer.
type body struct {
	Status    string               `json:"status"`
	Timestamp string               `json:"timestamp"`
	Checks    map[string]checkBody `json:"checks"`
}

// checkBody is the JSON form of one check. Error and Since are left out while the check
// is up.
type checkBody struct {
	Status CheckStatus `json:"status"`
	Error  string      `json:"error,omitempty"`
	Since  string      `json:"since,omitempty"`
}

// StatusCode returns the HTTP status for a result: 200 when healthy, 503 otherwise.
func StatusCode(res Result) int {
	if res.Healthy {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
}

// RenderBody returns the JSON health answer for a result. Only the safe failure reason of
// each check is rendered, never the raw error behind it.
func RenderBody(res Result) ([]byte, error) {
	doc := body{
		Status:    res.Status,
		Timestamp: formatTime(res.EvaluatedAt),
		Checks:    make(map[string]checkBody, len(res.Checks)),
	}
	for name, chk := range res.Checks {
		cb := checkBody{Status: chk.Status, Error: chk.Error}
		if !chk.Since.IsZero() {
			cb.Since = formatTime(chk.Since)
		}
		doc.Checks[name] = cb
	}
	return json.Marshal(doc)
}

// formatTime renders t in UTC as RFC 3339 with second precision.
func formatTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}
