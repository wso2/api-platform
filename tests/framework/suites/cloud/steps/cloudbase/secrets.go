/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"encoding/json"
	"strings"
)

// SecretsCollection is the collection a secret is listed in and deleted from.
const SecretsCollection = "/secrets"

// EchoedHeader reads a request header from the JSON an echo backend returns, ignoring case.
func EchoedHeader(body []byte, name string) (string, bool) {
	var echoed struct {
		Headers map[string]any `json:"headers"`
	}
	if json.Unmarshal(body, &echoed) != nil {
		return "", false
	}
	for key, raw := range echoed.Headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		switch typed := raw.(type) {
		case string:
			return typed, true
		case []any:
			parts := make([]string, 0, len(typed))
			for _, part := range typed {
				if text, ok := part.(string); ok {
					parts = append(parts, text)
				}
			}
			return strings.Join(parts, ","), true
		}
	}
	return "", false
}
