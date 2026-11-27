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

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

const planConditionsBody = `{"planName":"Agents","conditions":[
	{"type":"request-count","params":{"limits":[{"count":100,"timeUnit":"MINUTE"},{"count":5000,"timeUnit":"DAY"}]}},
	{"type":"tool-call-count","params":{"limits":[{"count":10,"timeUnit":"HOUR"}],"onExceed":"allow"}},
	{"id":"ip-ranges","type":"custom","params":{"cidr":"10.0.0.0/8"}}]}`

func createPlanViaHandler(t *testing.T, server *APIServer, body string) (int, []byte) {
	t.Helper()
	w, r := createTestContextWithHeader("POST", "/subscription-plans", []byte(body), map[string]string{"Content-Type": "application/json"})
	server.CreateSubscriptionPlan(w, r)
	return w.Code, w.Body.Bytes()
}

func planResponse(t *testing.T, body []byte) api.SubscriptionPlanResponse {
	t.Helper()
	var resp api.SubscriptionPlanResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp
}

func responseConditionIDs(resp api.SubscriptionPlanResponse) []string {
	var ids []string
	if resp.Conditions != nil {
		for _, c := range *resp.Conditions {
			ids = append(ids, *c.Id)
		}
	}
	return ids
}

// A plan created with conditions stores them normalized and fills the throttle limit that
// subscription-validation v1 reads.
func TestCreateSubscriptionPlan_WithConditions(t *testing.T) {
	server, _ := newPlanTestServer(t)
	code, body := createPlanViaHandler(t, server, planConditionsBody)
	require.Equal(t, http.StatusCreated, code, string(body))

	resp := planResponse(t, body)
	assert.Equal(t, []string{"requests", "tool-calls", "ip-ranges"}, responseConditionIDs(resp))
	assert.Equal(t, 100, *resp.ThrottleLimitCount)
	assert.Equal(t, "Min", *resp.ThrottleLimitUnit)

	stored := storedPlan(t, server, *resp.Id)
	require.Len(t, stored.Conditions, 3)
	assert.JSONEq(t, `{"limits":[{"count":100,"timeAmount":1,"timeUnit":"MINUTE"},{"count":5000,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`,
		string(stored.Conditions[0].Params))
}

func TestCreateSubscriptionPlan_RefusesBadConditions(t *testing.T) {
	server, _ := newPlanTestServer(t)
	for name, body := range map[string]string{
		"conditions and throttle limit": `{"planName":"P1","throttleLimitCount":5,"throttleLimitUnit":"Min","conditions":[]}`,
		"unknown type":                  `{"planName":"P2","conditions":[{"type":"bandwidth","params":{"limits":[{"count":1,"timeUnit":"DAY"}]}}]}`,
		"two request-count":             `{"planName":"P3","conditions":[{"type":"request-count","params":{"limits":[{"count":1,"timeUnit":"DAY"}]}},{"type":"request-count","params":{"limits":[{"count":2,"timeUnit":"HOUR"}]}}]}`,
		"custom without id":             `{"planName":"P4","conditions":[{"type":"custom","params":{}}]}`,
		"null conditions":               `{"planName":"P5","conditions":null}`,
	} {
		code, resp := createPlanViaHandler(t, server, body)
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", name, resp)
	}
}

// A plan created with only the throttle limit works as before, and reads with that limit as a
// request-count condition.
func TestCreateSubscriptionPlan_ThrottleLimitOnly(t *testing.T) {
	server, _ := newPlanTestServer(t)
	code, body := createPlanViaHandler(t, server, `{"planName":"Classic","throttleLimitCount":30,"throttleLimitUnit":"Hour"}`)
	require.Equal(t, http.StatusCreated, code, string(body))

	resp := planResponse(t, body)
	require.NotNil(t, resp.Conditions)
	require.Len(t, *resp.Conditions, 1)
	c := (*resp.Conditions)[0]
	assert.Equal(t, "requests", *c.Id)
	limits := c.Params["limits"].([]interface{})
	assert.Equal(t, "HOUR", limits[0].(map[string]interface{})["timeUnit"])
	assert.Nil(t, storedPlan(t, server, *resp.Id).Conditions, "the stored plan keeps no conditions of its own")
}

// Conditions replace all of a plan's conditions; the throttle limit fields alone change only the
// request-count condition; stopOnQuotaReach alone changes only its onExceed.
func TestUpdateSubscriptionPlan_Conditions(t *testing.T) {
	server, _ := newPlanTestServer(t)
	code, body := createPlanViaHandler(t, server, planConditionsBody)
	require.Equal(t, http.StatusCreated, code, string(body))
	id := *planResponse(t, body).Id

	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"throttleLimitCount":7,"throttleLimitUnit":"Day"}`)
	require.Equal(t, http.StatusOK, code, string(body))
	stored := storedPlan(t, server, id)
	require.Len(t, stored.Conditions, 3, "the other conditions are kept")
	assert.JSONEq(t, `{"limits":[{"count":7,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"reject"}`, string(stored.Conditions[0].Params))

	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"throttleLimitCount":8,"throttleLimitUnit":"Day","stopOnQuotaReach":false}`)
	require.Equal(t, http.StatusOK, code, string(body))
	assert.JSONEq(t, `{"limits":[{"count":8,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}`, string(storedPlan(t, server, id).Conditions[0].Params))
	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"throttleLimitCount":7,"throttleLimitUnit":"Day","stopOnQuotaReach":true}`)
	require.Equal(t, http.StatusOK, code, string(body))

	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"stopOnQuotaReach":false}`)
	require.Equal(t, http.StatusOK, code, string(body))
	stored = storedPlan(t, server, id)
	assert.JSONEq(t, `{"limits":[{"count":7,"timeAmount":1,"timeUnit":"DAY"}],"onExceed":"allow"}`, string(stored.Conditions[0].Params))
	assert.False(t, stored.StopOnQuotaReach)

	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"conditions":[{"id":"promo","type":"custom","params":{"until":"2026-12-31"}}]}`)
	require.Equal(t, http.StatusOK, code, string(body))
	resp := planResponse(t, body)
	assert.Equal(t, []string{"promo"}, responseConditionIDs(resp))
	assert.Nil(t, resp.ThrottleLimitCount, "no request-count condition leaves no throttle limit")

	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"conditions":[]}`)
	require.Equal(t, http.StatusOK, code, string(body))
	assert.Empty(t, storedPlan(t, server, id).Conditions)

	for name, update := range map[string]string{
		"conditions null":               `{"conditions":null}`,
		"conditions and throttle limit": `{"conditions":[],"throttleLimitCount":1,"throttleLimitUnit":"Min"}`,
		"invalid condition":             `{"conditions":[{"type":"request-count","params":{"limits":[]}}]}`,
	} {
		code, resp := putSubscriptionPlan(t, server, id, "application/json", update)
		assert.Equal(t, http.StatusBadRequest, code, "%s: %s", name, resp)
	}
}

// A plan created with only a 2-hour window gives subscription-validation v1 that window's count per
// hour, as the Platform API does.
func TestCreateSubscriptionPlan_LongerWindowOnly(t *testing.T) {
	server, _ := newPlanTestServer(t)
	code, body := createPlanViaHandler(t, server, `{"planName":"Two hours","conditions":[
		{"type":"request-count","params":{"limits":[{"count":1000,"timeAmount":2,"timeUnit":"HOUR"}]}}]}`)
	require.Equal(t, http.StatusCreated, code, string(body))
	resp := planResponse(t, body)
	assert.Equal(t, 1000, *resp.ThrottleLimitCount)
	assert.Equal(t, "Hour", *resp.ThrottleLimitUnit)
	stored := storedPlan(t, server, *resp.Id)
	assert.JSONEq(t, `{"limits":[{"count":1000,"timeAmount":2,"timeUnit":"HOUR"}],"onExceed":"reject"}`, string(stored.Conditions[0].Params))
}

// Setting the throttle limit adds a request-count condition, so a plan with 20 conditions can't take
// one: the merged list is checked before it's saved.
func TestUpdateSubscriptionPlan_ThrottleLimitKeepsTheConditionCap(t *testing.T) {
	server, _ := newPlanTestServer(t)
	customs := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		customs = append(customs, fmt.Sprintf(`{"id":"c%d","type":"custom","params":{}}`, i))
	}
	code, body := createPlanViaHandler(t, server, `{"planName":"Full","conditions":[`+strings.Join(customs, ",")+`]}`)
	require.Equal(t, http.StatusCreated, code, string(body))
	id := *planResponse(t, body).Id

	code, body = putSubscriptionPlan(t, server, id, "application/json", `{"throttleLimitCount":5,"throttleLimitUnit":"Min"}`)
	require.Equal(t, http.StatusBadRequest, code, string(body))
	assert.Contains(t, errorMessage(t, body), "at most 20 conditions")
	assert.Len(t, storedPlan(t, server, id).Conditions, 20, "nothing saved")
}

// event-count is not a condition type: a plan with one is refused.
func TestSubscriptionPlan_EventCountRefused(t *testing.T) {
	server, _ := newPlanTestServer(t)
	code, body := createPlanViaHandler(t, server, `{"planName":"Streams","conditions":[
		{"type":"event-count","params":{"limits":[{"count":500,"timeUnit":"MINUTE"}]}}]}`)
	require.Equal(t, http.StatusBadRequest, code, string(body))
	assert.Contains(t, errorMessage(t, body), "type must be one of request-count, tool-call-count, token-count or custom")
}
