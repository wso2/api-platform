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

// Package pagination holds the in-memory windowing primitive shared by the
// handler and service layers, so the two cannot drift apart.
package pagination

// Window returns the [offset, offset+limit) slice of items, clamped to the
// bounds of items. It is used to window a fully-materialized, bounded
// collection. An out-of-range offset or a non-positive limit yields an empty
// (non-nil) slice.
func Window[T any](items []T, limit, offset int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) || limit <= 0 {
		return items[:0]
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}
