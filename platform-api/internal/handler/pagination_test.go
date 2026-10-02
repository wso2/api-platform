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

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestParsePagination pins parsePagination's clamping contract — shared by
// every kind's list handler (including GraphQL's ListGraphQLAPIs), and
// previously untested anywhere in the repo despite being the one place that
// stands between a client-supplied limit/offset and an unbounded query.
func TestParsePagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
	}{
		{"defaults when absent", "", defaultPageLimit, defaultPageOffset},
		{"limit clamped at the upper bound", "limit=999", maxPageLimit, defaultPageOffset},
		{"limit clamped at the lower bound (zero)", "limit=0", minPageLimit, defaultPageOffset},
		{"limit clamped at the lower bound (negative)", "limit=-5", minPageLimit, defaultPageOffset},
		{"limit within range is respected", "limit=42", 42, defaultPageOffset},
		{"malformed limit falls back to default", "limit=not-a-number", defaultPageLimit, defaultPageOffset},
		{"offset respected when non-negative", "offset=15", defaultPageLimit, 15},
		{"negative offset falls back to default", "offset=-1", defaultPageLimit, defaultPageOffset},
		{"malformed offset falls back to default", "offset=not-a-number", defaultPageLimit, defaultPageOffset},
		{"limit and offset combined", "limit=999&offset=15", maxPageLimit, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/graphql-apis?"+tt.query, nil)
			limit, offset := parsePagination(r)
			if limit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tt.wantLimit)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}
