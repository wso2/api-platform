/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package controlplane

// Note: Reconnection logic is implemented in client.go
// This file exists for future reconnection-specific utilities
// and to maintain the project structure as defined in the specification.

// Key reconnection features implemented in client.go:
// - Exponential backoff calculation with jitter (±25%)
// - Retry sequence: 1s → 2s → 4s → 8s → 16s → 32s → 64s → 128s → 256s → 300s (capped)
// - Automatic reconnection after network failures
// - State transitions: Connected → Reconnecting → Connected
// - Backoff reset after successful connection >60s (implemented in connectionLoop)
