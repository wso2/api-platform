/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */
package api

import (
	"encoding/json"
	"testing"
)

// Clients that sent subscriptionPlans as handle strings keep working; the object form is read too.
func TestSubscriptionPlanRef_ReadsAStringOrAnObject(t *testing.T) {
	var refs []SubscriptionPlanRef
	if err := json.Unmarshal([]byte(`["gold",{"id":"silver","displayName":"ignored","sourceStatus":"DELETED"}]`), &refs); err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || refs[0].Id != "gold" || refs[0].DisplayName != nil || refs[1].Id != "silver" {
		t.Fatalf("got %+v", refs)
	}
	var body struct {
		SubscriptionPlans *[]SubscriptionPlanRef `json:"subscriptionPlans"`
	}
	if err := json.Unmarshal([]byte(`{"subscriptionPlans":[1]}`), &body); err == nil {
		t.Fatal("want a number refused")
	}
	out, err := json.Marshal(SubscriptionPlanRef{Id: "gold"})
	if err != nil || string(out) != `{"id":"gold"}` {
		t.Fatalf("written as an object: %s, %v", out, err)
	}
}
