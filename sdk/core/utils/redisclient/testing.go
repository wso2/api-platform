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

package redisclient

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

// SetSharedForTesting overrides the process-wide Shared client for the
// duration of t, restoring the previous state automatically via t.Cleanup.
// Test-only - never call this from production code. Since Shared is a
// single global, tests using this helper must not run in parallel with each
// other (no t.Parallel) or they will race on the same override.
func SetSharedForTesting(t testing.TB, client *redis.Client) {
	t.Helper()
	shared.mu.Lock()
	prevClient, prevInited := shared.client, shared.inited
	shared.client, shared.inited = client, true
	shared.mu.Unlock()
	t.Cleanup(func() {
		shared.mu.Lock()
		shared.client, shared.inited = prevClient, prevInited
		shared.mu.Unlock()
	})
}
