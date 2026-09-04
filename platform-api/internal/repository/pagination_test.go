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

import (
	"strings"
	"testing"
)

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

// wantSearchClause is the expected fragment from handleSearchClause for
// non-empty terms. It's parenthesized so callers can AND-join it safely.
const wantSearchClause = ` AND (LOWER(display_name) LIKE ? ESCAPE '\' OR LOWER(handle) LIKE ? ESCAPE '\')`

// assertSearchArgs checks that the clause bound exactly the expected patterns,
// in order. Both matched columns take the same pattern, so a caller passes it
// twice.
func assertSearchArgs(t *testing.T, got []any, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args = %v (len %d), want %v (len %d)", got, len(got), want, len(want))
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("args[%d] = %v, want %q", i, got[i], w)
		}
	}
}

// TestHandleSearchClause verifies the empty-input short circuit, that the term
// is matched against both the display name and the handle, that the two matched
// columns are grouped so the fragment stays safe to AND-join, and that LIKE
// metacharacters supplied by the client are escaped so they match literally
// rather than acting as wildcards.
func TestHandleSearchClause(t *testing.T) {
	t.Run("empty search yields no clause", func(t *testing.T) {
		clause, args := handleSearchClause("   ")
		if clause != "" || args != nil {
			t.Fatalf("expected empty clause and nil args, got %q / %v", clause, args)
		}
	})

	t.Run("plain term is lowercased, wrapped, and bound once per column", func(t *testing.T) {
		clause, args := handleSearchClause("Payment")
		if clause != wantSearchClause {
			t.Fatalf("clause = %q, want %q", clause, wantSearchClause)
		}
		assertSearchArgs(t, args, "%payment%", "%payment%")
	})

	t.Run("term matches display name as well as handle", func(t *testing.T) {
		clause, _ := handleSearchClause("Payment")
		if !strings.Contains(clause, "LOWER(display_name) LIKE ?") {
			t.Errorf("clause %q does not filter on display_name", clause)
		}
		if !strings.Contains(clause, "LOWER(handle) LIKE ?") {
			t.Errorf("clause %q no longer filters on handle", clause)
		}
	})

	// Guards the precedence bug independently of wantSearchClause, which a
	// future reword would update in lockstep: the fragment must remain a single
	// parenthesized group so callers can AND-join it without OR leaking rows
	// past their organization filter.
	t.Run("matched columns are grouped so the clause is safe to AND-join", func(t *testing.T) {
		clause, _ := handleSearchClause("Payment")
		if !strings.HasPrefix(clause, " AND (") || !strings.HasSuffix(clause, ")") {
			t.Fatalf("clause %q must be a parenthesized group prefixed with \" AND \"", clause)
		}
	})

	// The reported bug: a display name is prose, not a slug. The term must reach
	// the bound pattern verbatim (lowercased only) so "Payment API" can match a
	// display_name of "Payment API"; it can never match the handle "payment-api".
	t.Run("multi-word display name term is preserved verbatim", func(t *testing.T) {
		_, args := handleSearchClause("Payment API")
		assertSearchArgs(t, args, "%payment api%", "%payment api%")
	})

	t.Run("wildcards are escaped", func(t *testing.T) {
		_, args := handleSearchClause("a_b%c")
		assertSearchArgs(t, args, `%a\_b\%c%`, `%a\_b\%c%`)
	})

	t.Run("backslash is escaped before the metacharacters", func(t *testing.T) {
		_, args := handleSearchClause(`a\b`)
		assertSearchArgs(t, args, `%a\\b%`, `%a\\b%`)
	})
}
