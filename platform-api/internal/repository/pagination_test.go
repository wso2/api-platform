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

package repository

import "testing"

// TestResolveSort verifies that only allowlisted sort tokens map to columns and
// that everything else — including attempted SQL injection via the sort token —
// falls back to the default column, and that the direction is constrained to the
// two safe constants. This is what makes it safe to interpolate the results into
// an ORDER BY clause (which cannot be a bind parameter).
func TestResolveSort(t *testing.T) {
	allowed := map[string]string{"name": "display_name", "createdAt": "created_at"}

	tests := []struct {
		name          string
		sortBy        string
		sortOrder     string
		wantColumn    string
		wantDirection string
	}{
		{"known field asc", "name", "asc", "display_name", "ASC"},
		{"known field desc", "createdAt", "desc", "created_at", "DESC"},
		{"empty falls back to default column and desc", "", "", "created_at", "DESC"},
		{"unknown field falls back to default", "password", "asc", "created_at", "ASC"},
		{"injection attempt falls back to default", "created_at; DROP TABLE rest_apis", "asc", "created_at", "ASC"},
		{"direction is case-insensitive", "name", "ASC", "display_name", "ASC"},
		{"garbage direction defaults to desc", "name", "sideways", "display_name", "DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ListOptions{SortBy: tt.sortBy, SortOrder: tt.sortOrder}
			col, dir := opts.resolveSort(allowed, "created_at")
			if col != tt.wantColumn {
				t.Errorf("column = %q, want %q", col, tt.wantColumn)
			}
			if dir != tt.wantDirection {
				t.Errorf("direction = %q, want %q", dir, tt.wantDirection)
			}
		})
	}
}

// TestHandleSearchClause verifies the empty-input short circuit and that LIKE
// metacharacters supplied by the client are escaped so they match literally
// rather than acting as wildcards.
func TestHandleSearchClause(t *testing.T) {
	t.Run("empty search yields no clause", func(t *testing.T) {
		clause, args := handleSearchClause("   ")
		if clause != "" || args != nil {
			t.Fatalf("expected empty clause and nil args, got %q / %v", clause, args)
		}
	})

	t.Run("plain term is lowercased and wrapped", func(t *testing.T) {
		clause, args := handleSearchClause("Payment")
		if clause == "" {
			t.Fatal("expected a non-empty clause")
		}
		if len(args) != 1 || args[0] != "%payment%" {
			t.Fatalf("args = %v, want [%%payment%%]", args)
		}
	})

	t.Run("wildcards are escaped", func(t *testing.T) {
		_, args := handleSearchClause("a_b%c")
		if len(args) != 1 || args[0] != `%a\_b\%c%` {
			t.Fatalf("args = %v, want [%%a\\_b\\%%c%%]", args)
		}
	})

	t.Run("backslash is escaped before the metacharacters", func(t *testing.T) {
		_, args := handleSearchClause(`a\b`)
		if len(args) != 1 || args[0] != `%a\\b%` {
			t.Fatalf("args = %v, want [%%a\\\\b%%]", args)
		}
	})
}
