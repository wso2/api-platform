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
 */
package model

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// A configuration stored before plan details were kept lists handle strings; a new one lists
// objects; a list may mix both. All read the same handles.
func TestAttachedPlans_ReadsStringsObjectsAndMixed(t *testing.T) {
	cases := map[string]string{
		"strings": `["gold","silver"]`,
		"objects": `[{"id":"gold","displayName":"Gold","attachedAt":"2026-11-01T10:00:00Z"},{"id":"silver"}]`,
		"mixed":   `["gold",{"id":"silver","displayName":"Silver"}]`,
	}
	for name, raw := range cases {
		var plans AttachedPlans
		if err := json.Unmarshal([]byte(raw), &plans); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := plans.Handles(); !reflect.DeepEqual(got, []string{"gold", "silver"}) {
			t.Errorf("%s: handles = %v", name, got)
		}
	}
}

func TestAttachedPlans_KeepsTheDetailsAndWritesObjects(t *testing.T) {
	var plans AttachedPlans
	if err := json.Unmarshal([]byte(`[{"id":"gold","displayName":"Gold","attachedAt":"2026-11-01T10:00:00Z"}]`), &plans); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	if plans[0].DisplayName != "Gold" || plans[0].AttachedAt == nil || !plans[0].AttachedAt.Equal(want) {
		t.Fatalf("got %+v", plans[0])
	}
	out, err := json.Marshal(AttachedPlansFromHandles([]string{"gold"}))
	if err != nil || string(out) != `[{"id":"gold"}]` {
		t.Fatalf("a plan with only its handle is written as {\"id\":...}: %s, %v", out, err)
	}
}

func TestAttachedPlans_NullAndBadEntries(t *testing.T) {
	var plans AttachedPlans
	if err := json.Unmarshal([]byte(`null`), &plans); err != nil || plans != nil {
		t.Fatalf("null = %v, %v", plans, err)
	}
	if err := json.Unmarshal([]byte(`[42]`), &plans); err == nil {
		t.Fatal("want a number entry refused")
	}
	if AttachedPlans(nil).Handles() != nil || AttachedPlansFromHandles(nil) != nil {
		t.Fatal("nil stays nil, so an API without plans leaves the field out")
	}
}
