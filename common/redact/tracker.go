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

package redact

import "sync"

// SecretTracker records resolved secret values during template rendering.
// It is safe for concurrent use.
type SecretTracker struct {
	mu     sync.Mutex
	values map[string]bool
}

// NewSecretTracker creates a new SecretTracker.
func NewSecretTracker() *SecretTracker {
	return &SecretTracker{values: make(map[string]bool)}
}

// Track records a resolved secret value for later redaction.
// Empty strings are ignored to avoid false-positive redaction.
func (st *SecretTracker) Track(value string) {
	if value == "" {
		return
	}
	st.mu.Lock()
	st.values[value] = true
	st.mu.Unlock()
}

// Values returns a copy of all tracked secret values.
func (st *SecretTracker) Values() []string {
	st.mu.Lock()
	defer st.mu.Unlock()

	vals := make([]string, 0, len(st.values))
	for v := range st.values {
		vals = append(vals, v)
	}
	return vals
}
