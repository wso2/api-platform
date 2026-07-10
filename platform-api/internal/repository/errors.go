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

package repository

import "strings"

// IsUniqueViolation reports whether err is a database unique-constraint (or
// primary-key) violation. It is dialect-aware across the three databases the
// control plane supports — SQLite, PostgreSQL and SQL Server — matching on the
// stable substrings/error codes each driver surfaces.
//
// Detection is by err.Error() substring, so a violation wrapped with
// fmt.Errorf("...: %w", err) is still detected without unwrapping.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	switch {
	// SQLite: "UNIQUE constraint failed: <table>.<column>"
	case strings.Contains(s, "UNIQUE constraint failed"):
		return true
	// PostgreSQL (pgx): SQLSTATE 23505, "duplicate key value violates unique constraint".
	case strings.Contains(s, "duplicate key value violates unique constraint"),
		strings.Contains(s, "23505"):
		return true
	// SQL Server: error 2627 ("Violation of UNIQUE KEY constraint" / PRIMARY KEY) and
	// error 2601 ("Cannot insert duplicate key row in object ... with unique index").
	case strings.Contains(s, "Violation of UNIQUE KEY constraint"),
		strings.Contains(s, "Cannot insert duplicate key"):
		return true
	default:
		return false
	}
}
