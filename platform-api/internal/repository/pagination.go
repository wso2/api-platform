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

import "strings"

// listSortColumns is the allowlist mapping the sort tokens exposed by the
// standard collection list endpoints (rest-apis, projects, applications,
// gateways) to their backing columns. Every one of those tables carries a
// display_name and created_at column, so the mapping is shared. A resource that
// needs a different set of sortable fields should define its own allowlist
// rather than widening this one.
var listSortColumns = map[string]string{
	"name":      "display_name",
	"createdAt": "created_at",
}

// ListOptions carries the pagination, sorting, and search inputs shared by
// collection list queries. Limit/Offset are already normalized by the handler
// layer; SortBy/SortOrder/Search are raw client tokens that MUST be resolved
// through resolveSort / handleSearchClause before touching SQL — they are never
// interpolated into a query verbatim.
type ListOptions struct {
	Limit     int
	Offset    int
	SortBy    string // API sort token (e.g. "name", "createdAt"); "" = default
	SortOrder string // "asc" or "desc"; anything else defaults to descending
	Search    string // case-insensitive handle substring; "" = no filter
}

// resolveSort maps the client SortBy token to a real column name via the
// per-table allowlist, falling back to defaultColumn when the token is empty or
// unrecognized, and normalizes the direction to ASC/DESC (default DESC).
//
// Both return values originate exclusively from server-controlled inputs (the
// allowlist's values and the two direction constants) — never from the raw
// client string — so they are safe to embed directly into an ORDER BY clause,
// which cannot be parameterized with a bind argument.
func (o ListOptions) resolveSort(allowed map[string]string, defaultColumn string) (column, direction string) {
	column = defaultColumn
	if c, ok := allowed[o.SortBy]; ok && c != "" {
		column = c
	}
	direction = "DESC"
	if strings.EqualFold(o.SortOrder, "asc") {
		direction = "ASC"
	}
	return column, direction
}

// handleSearchClause returns a SQL fragment (with a leading " AND ") and its
// single bound argument for a case-insensitive substring match against the
// `handle` column, or ("", nil) when search is empty. LIKE metacharacters in the
// input are escaped via ESCAPE '\' so a literal '%' or '_' matches itself rather
// than acting as a wildcard. The escape clause is standard SQL honored by
// SQLite, PostgreSQL, and SQL Server alike.
func handleSearchClause(search string) (string, []any) {
	s := strings.TrimSpace(search)
	if s == "" {
		return "", nil
	}
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(s))
	return ` AND LOWER(handle) LIKE ? ESCAPE '\'`, []any{"%" + esc + "%"}
}
