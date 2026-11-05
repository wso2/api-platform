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

// End-to-end coverage of plan conditions over /subscription-plans: the real handler, service and
// SQLite-backed repository. They pin what a client sees: conditions and description in and out, and
// the "limits" of clients that predate conditions.

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// goldConditions is a plan with all three built-in types and a custom condition.
const goldConditions = `"conditions":[
	{"type":"request-count","params":{"limits":[{"count":100,"timeUnit":"MINUTE"},{"count":10000,"timeUnit":"DAY"}]}},
	{"id":"tool-calls","type":"tool-call-count","params":{"limits":[{"count":500,"timeAmount":1,"timeUnit":"HOUR"}],"onExceed":"allow"}},
	{"type":"token-count","params":{"limits":[{"count":100000,"timeUnit":"MONTH"}],"tokenType":"prompt"}},
	{"id":"ip-ranges","type":"custom","params":{"cidr":"10.0.0.0/8","limit":50}}]`

// planConditions returns the conditions of a plan response, each as {id, type, params}.
func planConditions(t *testing.T, plan map[string]any) []map[string]any {
	t.Helper()
	raw, ok := plan["conditions"].([]any)
	if !ok {
		t.Fatalf("conditions = %#v, want an array", plan["conditions"])
	}
	out := make([]map[string]any, len(raw))
	for i, c := range raw {
		out[i] = c.(map[string]any)
	}
	return out
}

// conditionIDs lists the IDs of a plan response's conditions, in order.
func conditionIDs(t *testing.T, plan map[string]any) []string {
	t.Helper()
	var ids []string
	for _, c := range planConditions(t, plan) {
		ids = append(ids, c["id"].(string))
	}
	return ids
}

func createGold(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	rec := planRequest(t, h, http.MethodPost, planITPath, `{"id":"gold","displayName":"Gold","description":" For partners ",`+goldConditions+`}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}
	return decodePlan(t, rec)
}

func TestPlanHandler_CreateWithConditions(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	created := createGold(t, h)

	if created["description"] != "For partners" {
		t.Errorf("description = %v, want it trimmed", created["description"])
	}
	if ids := conditionIDs(t, created); !reflect.DeepEqual(ids, []string{"requests", "tool-calls", "tokens", "ip-ranges"}) {
		t.Fatalf("condition ids = %v", ids)
	}
	conditions := planConditions(t, created)
	wantRequests := map[string]any{"limits": []any{
		map[string]any{"count": float64(100), "timeAmount": float64(1), "timeUnit": "MINUTE"},
		map[string]any{"count": float64(10000), "timeAmount": float64(1), "timeUnit": "DAY"},
	}, "onExceed": "reject"}
	if !reflect.DeepEqual(conditions[0]["params"], wantRequests) {
		t.Errorf("requests params = %v, want the defaults filled in: %v", conditions[0]["params"], wantRequests)
	}
	if params := conditions[2]["params"].(map[string]any); params["tokenType"] != "prompt" || params["onExceed"] != "reject" {
		t.Errorf("tokens params = %v", params)
	}
	// "limits" is the request-count condition's windows, for clients that predate conditions.
	limits := created["limits"].([]any)
	if len(limits) != 2 || limits[1].(map[string]any)["limitCount"] != float64(10000) || limits[1].(map[string]any)["timeUnit"] != "DAY" {
		t.Errorf("limits = %v, want both request windows", limits)
	}

	// A read and the list return the same conditions.
	got := decodePlan(t, planRequest(t, h, http.MethodGet, planITPath+"/gold", ""))
	if !reflect.DeepEqual(got["conditions"], created["conditions"]) || got["description"] != "For partners" {
		t.Errorf("GET = %v, want what the create returned", got)
	}
	list := decodePlan(t, planRequest(t, h, http.MethodGet, planITPath, ""))
	item := list["list"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(item["conditions"], created["conditions"]) {
		t.Errorf("list item conditions = %v", item["conditions"])
	}
}

func TestPlanHandler_CreateRefusesBadConditions(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	for name, body := range map[string]string{
		"conditions and limits": `{"id":"p1","displayName":"P1","conditions":[{"type":"custom","id":"a","params":{}}],` +
			`"limits":[{"limitCount":1,"timeUnit":"MINUTE"}]}`,
		"conditions null":            `{"id":"p2","displayName":"P2","conditions":null}`,
		"conditions not an array":    `{"id":"p3","displayName":"P3","conditions":{"type":"custom"}}`,
		"two request-count":          `{"id":"p4","displayName":"P4","conditions":[{"type":"request-count","params":{"limits":[{"count":1,"timeUnit":"MINUTE"}]}},{"type":"request-count","params":{"limits":[{"count":2,"timeUnit":"HOUR"}]}}]}`,
		"bandwidth":                  `{"id":"p5","displayName":"P5","conditions":[{"type":"bandwidth","params":{"limits":[{"count":1,"timeUnit":"MINUTE"}]}}]}`,
		"overlapping windows":        `{"id":"p6","displayName":"P6","conditions":[{"type":"request-count","params":{"limits":[{"count":1,"timeAmount":60,"timeUnit":"MINUTE"},{"count":2,"timeUnit":"HOUR"}]}}]}`,
		"custom without an id":       `{"id":"p7","displayName":"P7","conditions":[{"type":"custom","params":{"k":"v"}}]}`,
		"invalid first legacy limit": `{"id":"p8","displayName":"P8","limits":[{"limitCount":1,"timeUnit":"WEEK"}]}`,
	} {
		rec := planRequest(t, h, http.MethodPost, planITPath, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, body %s; want 400", name, rec.Code, rec.Body.String())
		}
	}
	if total, _ := listPlans(t, h, ""); total != 0 {
		t.Errorf("a refused plan was stored: %d plans", total)
	}
}

// An omitted field leaves the plan's value; conditions replace them all; limits replace only the
// request-count condition; [] removes every condition.
func TestPlanHandler_UpdateConditions(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	createGold(t, h)
	put := func(body string) map[string]any {
		t.Helper()
		rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT %s: status %d, body %s", body, rec.Code, rec.Body.String())
		}
		return decodePlan(t, rec)
	}

	got := put(`{"id":"gold","displayName":"Gold"}`)
	if len(planConditions(t, got)) != 4 || got["description"] != "For partners" {
		t.Fatalf("an update without conditions or description changed them: %v", got)
	}

	got = put(`{"id":"gold","displayName":"Gold","description":"Renamed tiers"}`)
	if got["description"] != "Renamed tiers" || len(planConditions(t, got)) != 4 {
		t.Fatalf("description update: %v", got)
	}

	got = put(`{"id":"gold","displayName":"Gold","limits":[{"limitType":"REQUEST_COUNT","limitCount":20,"timeUnit":"HOUR","timeAmount":1}]}`)
	if ids := conditionIDs(t, got); !reflect.DeepEqual(ids, []string{"requests", "tool-calls", "tokens", "ip-ranges"}) {
		t.Fatalf("limits update: condition ids = %v, want the others kept", ids)
	}
	if limits := got["limits"].([]any); len(limits) != 1 || limits[0].(map[string]any)["limitCount"] != float64(20) {
		t.Fatalf("limits update: limits = %v", limits)
	}

	got = put(`{"id":"gold","displayName":"Gold","conditions":[{"id":"promo","type":"custom","params":{"until":"2026-12-31"}}]}`)
	if ids := conditionIDs(t, got); !reflect.DeepEqual(ids, []string{"promo"}) || len(got["limits"].([]any)) != 0 {
		t.Fatalf("conditions update: ids = %v, limits = %v", ids, got["limits"])
	}

	got = put(`{"id":"gold","displayName":"Gold","conditions":[]}`)
	if len(planConditions(t, got)) != 0 {
		t.Fatalf("conditions [] left %v", got["conditions"])
	}

	rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", `{"id":"gold","displayName":"Gold","conditions":null}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("conditions null: status %d, want 400", rec.Code)
	}
}

// seedPlanSubscription puts the plan in use with one subscription.
func seedPlanSubscription(t *testing.T, db *database.DB, handle string) {
	t.Helper()
	var planUUID string
	if err := db.QueryRow(`SELECT uuid FROM subscription_plans WHERE handle = ?`, handle).Scan(&planUUID); err != nil {
		t.Fatalf("read plan uuid: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('api-1', 'RestApi', ?)`, planITOrgID); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO subscriptions (uuid, artifact_uuid, subscriber_id, subscription_token,
		subscription_token_hash, subscription_plan_uuid, organization_uuid) VALUES ('sub-1', 'api-1', 'dev', 'tok', 'hash', ?, ?)`,
		planUUID, planITOrgID); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
}

// A plan in use keeps its conditions; its description can still change, and a client that sends the
// conditions it read, or the "limits" it read, changes nothing.
func TestPlanHandler_InUsePlanKeepsItsConditions(t *testing.T) {
	h, db := setupPlanHandlerTestEnvWithDB(t)
	created := createGold(t, h)
	seedPlanSubscription(t, db, "gold")

	read, err := json.Marshal(created["conditions"])
	if err != nil {
		t.Fatal(err)
	}
	limits, err := json.Marshal(created["limits"])
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"the conditions it read":   `{"id":"gold","displayName":"Gold","conditions":` + string(read) + `}`,
		"the limits it read":       `{"id":"gold","displayName":"Gold","limits":` + string(limits) + `}`,
		"a description change":     `{"id":"gold","displayName":"Gold","description":"Still Gold"}`,
		"a status change":          `{"id":"gold","displayName":"Gold","status":"INACTIVE"}`,
		"nothing about conditions": `{"id":"gold","displayName":"Gold"}`,
	} {
		if rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", body); rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, body %s; want 200", name, rec.Code, rec.Body.String())
		}
	}

	for name, body := range map[string]string{
		"a condition added":        `{"id":"gold","displayName":"Gold","conditions":` + strings.Replace(string(read), `[`, `[{"id":"promo","type":"custom","params":{}},`, 1) + `}`,
		"a condition removed":      `{"id":"gold","displayName":"Gold","conditions":[]}`,
		"a request window changed": `{"id":"gold","displayName":"Gold","limits":[{"limitType":"REQUEST_COUNT","limitCount":1,"timeUnit":"MINUTE","timeAmount":1}]}`,
	} {
		rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", body)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "To change its name or conditions, create a new plan.") {
			t.Errorf("%s: status %d, body %s; want the in-use conflict", name, rec.Code, rec.Body.String())
		}
	}
	if got := decodePlan(t, planRequest(t, h, http.MethodGet, planITPath+"/gold", "")); len(planConditions(t, got)) != 4 {
		t.Fatalf("a refused update changed the conditions: %v", got["conditions"])
	}
}

// Clients that predate conditions create plans with "limits"; every valid window is kept, and a
// later limit that earlier releases ignored is still ignored.
func TestPlanHandler_CreateWithLegacyLimits(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	rec := planRequest(t, h, http.MethodPost, planITPath, `{"id":"silver","displayName":"Silver","limits":[
		{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"MINUTE","stopOnQuotaReach":false},
		{"limitType":"BANDWIDTH","limitCount":5,"timeUnit":"DAY"},
		{"limitType":"REQUEST_COUNT","limitCount":3000,"timeUnit":"HOUR","timeAmount":2}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}
	got := decodePlan(t, rec)
	conditions := planConditions(t, got)
	if len(conditions) != 1 || conditions[0]["id"] != "requests" {
		t.Fatalf("conditions = %v, want one request-count condition", conditions)
	}
	want := map[string]any{"limits": []any{
		map[string]any{"count": float64(100), "timeAmount": float64(1), "timeUnit": "MINUTE"},
		map[string]any{"count": float64(3000), "timeAmount": float64(2), "timeUnit": "HOUR"},
	}, "onExceed": "allow"}
	if !reflect.DeepEqual(conditions[0]["params"], want) {
		t.Fatalf("params = %v, want %v", conditions[0]["params"], want)
	}
	limits := got["limits"].([]any)
	if len(limits) != 2 || limits[1].(map[string]any)["timeAmount"] != float64(2) || limits[0].(map[string]any)["stopOnQuotaReach"] != false {
		t.Fatalf("limits = %v", limits)
	}
}

// A description longer than 1023 characters is refused, on create and on update.
func TestPlanHandler_RefusesAnOverlongDescription(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	long := strings.Repeat("é", 1024)
	if rec := planRequest(t, h, http.MethodPost, planITPath, `{"id":"gold","displayName":"Gold","description":"`+long+`"}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "at most 1023 characters") {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodPost, planITPath, `{"id":"gold","displayName":"Gold","description":"`+long[:2046]+`"}`); rec.Code != http.StatusCreated {
		t.Fatalf("1023 characters: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", `{"id":"gold","displayName":"Gold","description":"`+long+`"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("update: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// A plan with the most conditions a plan can have, each with the largest params, fits the body limit.
func TestPlanHandler_AcceptsTheLargestPlan(t *testing.T) {
	h := setupPlanHandlerTestEnv(t)
	var conditions []string
	value := strings.Repeat("x", 4000)
	for i := 0; i < 20; i++ {
		conditions = append(conditions, fmt.Sprintf(`{"id":"c%02d","type":"custom","params":{"v":"%s"}}`, i, value))
	}
	body := `{"id":"big","displayName":"Big","conditions":[` + strings.Join(conditions, ",") + `]}`
	if rec := planRequest(t, h, http.MethodPost, planITPath, body); rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %.300s", rec.Code, rec.Body.String())
	}
}
