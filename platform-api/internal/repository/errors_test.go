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

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("connection refused"), false},
		{"sqlite", errors.New("UNIQUE constraint failed: mcp_proxies.organization_uuid, mcp_proxies.handle"), true},
		{"postgres message", errors.New(`ERROR: duplicate key value violates unique constraint "artifacts_org_handle_key" (SQLSTATE 23505)`), true},
		{"postgres sqlstate only", errors.New("SQLSTATE 23505"), true},
		{"sqlserver 2627", errors.New("mssql: Violation of UNIQUE KEY constraint 'UQ_artifacts'. Cannot insert duplicate key in object 'dbo.artifacts'."), true},
		{"sqlserver 2601", errors.New("mssql: Cannot insert duplicate key row in object 'dbo.artifacts' with unique index 'IX_artifacts_handle'."), true},
		// A violation wrapped with %w (as the per-kind importers do) must still be detected.
		{"wrapped sqlite", fmt.Errorf("failed to create MCP proxy: %w", errors.New("UNIQUE constraint failed: mcp_proxies.handle")), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUniqueViolation(tc.err); got != tc.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
