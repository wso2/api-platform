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
	"testing"
	"time"
)

// A v1 throttle limit becomes a request-count condition with one window; a plan with no limit gets
// no condition row.
func TestPlanConditionValues(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	row := planRow{uuid: "p1", orgUUID: "org-1", throttleCount: sql.NullInt64{Int64: 50, Valid: true},
		throttleUnit: sql.NullString{String: "Hour", Valid: true}, stopOnQuota: sql.NullBool{Bool: false, Valid: true}}

	values := planConditionValues(row, createdAt)
	if len(values) != len(planConditionColumns) {
		t.Fatalf("values = %v, want one per column", values)
	}
	if values[1] != "p1" || values[2] != "org-1" || values[3] != "requests" || values[4] != "request-count" || values[6] != 0 {
		t.Fatalf("values = %v", values)
	}
	if got, want := string(values[5].([]byte)), `{"limits":[{"count":50,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow"}`; got != want {
		t.Fatalf("params = %s, want %s", got, want)
	}
	if again := planConditionValues(row, createdAt); again[0] != values[0] {
		t.Fatalf("condition uuid %v then %v, want the same on a rerun", values[0], again[0])
	}

	row.throttleCount = sql.NullInt64{}
	if values := planConditionValues(row, createdAt); values != nil {
		t.Fatalf("a plan with no limit: values = %v, want none", values)
	}
}
