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
	"strings"
	"testing"
)

// Request limits sent the way clients before conditions send them: only the first is checked, as
// before. Later valid limits of a new length become more windows; the others are left out.
func TestLimitsToRequestCondition_AsEarlierReleases(t *testing.T) {
	stop := false
	c, errMsg := limitsToRequestCondition([]SubscriptionPlanLimitRequest{
		{TimeUnit: "MINUTE", LimitCount: 100, StopOnQuotaReach: &stop},
		{LimitType: "BANDWIDTH", TimeUnit: "DAY", LimitCount: 5},
		{TimeUnit: "HOUR", TimeAmount: 1, LimitCount: 5000},
		{TimeUnit: "MINUTE", TimeAmount: 60, LimitCount: 7000},
		{TimeUnit: "WEEK", LimitCount: 1},
	})
	if errMsg != "" || c == nil {
		t.Fatalf("limitsToRequestCondition = %v, %q", c, errMsg)
	}
	want := `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"},{"count":5000,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow"}`
	if c.ID != "requests" || string(c.Params) != want {
		t.Fatalf("condition = %s %s, want %s", c.ID, c.Params, want)
	}

	if _, errMsg := limitsToRequestCondition([]SubscriptionPlanLimitRequest{{TimeUnit: "WEEK", LimitCount: 1}}); !strings.Contains(errMsg, "timeUnit") {
		t.Fatalf("an invalid first limit: %q, want it refused as before", errMsg)
	}
	if c, errMsg := limitsToRequestCondition(nil); c != nil || errMsg != "" {
		t.Fatalf("no limits: %v, %q", c, errMsg)
	}
}
