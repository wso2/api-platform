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
package migration

import (
	"database/sql"
	"strings"
	"testing"
)

func dep(id, base string) deploymentRow {
	return deploymentRow{deploymentID: id, baseDeploymentID: sql.NullString{String: base, Valid: base != ""}}
}

func deploymentIDs(rows []deploymentRow) string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.deploymentID
	}
	return strings.Join(ids, ",")
}

func TestOrderDeploymentsParentFirst(t *testing.T) {
	// c is based on b, which was created after it; "x" is based on a row
	// outside the set (orphan / migrated earlier) and keeps its place.
	in := []deploymentRow{dep("a", ""), dep("c", "b"), dep("x", "missing"), dep("b", "a"), dep("d", "")}
	out, err := orderDeploymentsParentFirst(in)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if got, want := deploymentIDs(out), "a,b,c,x,d"; got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}
}

func TestOrderDeploymentsParentFirst_CycleFails(t *testing.T) {
	for name, in := range map[string][]deploymentRow{
		"two-node": {dep("a", "b"), dep("b", "a")},
		"self":     {dep("a", "a")},
	} {
		if _, err := orderDeploymentsParentFirst(in); err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("%s: expected a cycle error, got %v", name, err)
		}
	}
}
