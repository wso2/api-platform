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

// forbiddenBody is the answer to a client outside the allowed set. It carries no check
// names, reasons or times.
var forbiddenBody = []byte(`{"status":"forbidden"}`)

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

// Handler returns the HTTP handler that serves the health answer. It accepts GET and
// HEAD only, evaluates the checks (or reuses a recent result) on every request, and
// answers 200 when healthy and 503 otherwise. HEAD gets the same status and headers
// without a body.
//
// A client outside the allowed set, judged by the connection's peer address alone, gets
// 403 with a fixed body. No check runs for it, the cache is not refreshed and nothing is
// logged.
func (c *Checker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if !c.opts.AllowedClients.Allows(r.RemoteAddr) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusForbidden)
			if r.Method == http.MethodGet {
				_, _ = w.Write(forbiddenBody)
			}
			return
		}

		res := c.Evaluate()
		payload, err := RenderBody(res)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(StatusCode(res))
		if r.Method == http.MethodGet {
			_, _ = w.Write(payload)
		}
	})
}
