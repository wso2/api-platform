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

package service

import "github.com/wso2/api-platform/platform-api/internal/pagination"

// paginateSlice returns the window [offset, offset+limit) of items, clamped to
// the slice bounds. It is used for small, bounded collections whose endpoints
// declare limit/offset query parameters but return a bare array (no pagination
// envelope), so honoring the parameters at the database is unnecessary. For
// large collections, pagination is pushed into SQL at the repository layer
// instead.
func paginateSlice[T any](items []T, limit, offset int) []T {
	return pagination.Window(items, limit, offset)
}
