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

package budget

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	testBudgetUUID = "0b5e4f8a-6d1c-4c8e-9a52-3f1d2c7b9e10"
	otherUUID      = "9a41c7e0-1b3d-4e8f-a6c5-0d9b2e7f1a34"
)

// fakeSource stands in for the lazy-resource store.
type fakeSource struct {
	version   uint64
	resources map[string]*policy.LazyResource
}

func (f *fakeSource) Version() uint64 { return f.version }

func (f *fakeSource) GetResourcesByType(resourceType string) (map[string]*policy.LazyResource, error) {
	out := map[string]*policy.LazyResource{}
	for id, r := range f.resources {
		if r.ResourceType == resourceType {
			out[id] = r
		}
	}
	return out, nil
}

// set publishes budgets, replacing what was there, as the controller does.
func (f *fakeSource) set(bodies ...map[string]any) {
	f.resources = map[string]*policy.LazyResource{}
	for _, body := range bodies {
		name := body["name"].(string)
		f.resources[name] = &policy.LazyResource{ID: name, ResourceType: LazyResourceType, Resource: body}
	}
	f.version++
}

// budgetBody is a published budget, in the form it arrives over xDS.
func budgetBody(name, id, amount, period, action string, resource map[string]any, extra map[string]any) map[string]any {
	body := map[string]any{
		"uuid": id, "name": name,
		"limit":    map[string]any{"amount": amount, "currency": "USD", "period": period, "action": action, "blockAt": float64(100)},
		"resource": resource,
	}
	for k, v := range extra {
		if k == "blockAt" {
			body["limit"].(map[string]any)["blockAt"] = v
			continue
		}
		body[k] = v
	}
	return body
}

func oneProvider(id string) map[string]any {
	return map[string]any{"type": "llmProvider", "allocation": "shared", "id": id}
}

var (
	allProviders  = map[string]any{"type": "llmProvider", "allocation": "shared"}
	eachProvider  = map[string]any{"type": "llmProvider", "allocation": "separate"}
	wednesdayNoon = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
)

func consumers(entries ...map[string]any) map[string]any {
	list := make([]any, len(entries))
	for i, e := range entries {
		list[i] = e
	}
	return map[string]any{"consumers": list}
}

func attribute(allocation, expression string) map[string]any {
	return map[string]any{"type": "attribute", "allocation": allocation, "expression": expression}
}

// testPolicy is a policy over a fake source with a fixed clock.
func testPolicy(t *testing.T, bodies ...map[string]any) (*BudgetSystemPolicy, *fakeSource, *time.Time) {
	t.Helper()
	source := &fakeSource{}
	source.set(bodies...)
	p := newPolicy(source)
	now := wednesdayNoon
	p.now = func() time.Time { return now }
	return p, source, &now
}

// call is one request: its provider, token claims and headers.
type call struct {
	provider string
	subject  string
	claims   map[string]any
	headers  map[string][]string
}

func (c call) context() *policy.RequestContext {
	shared := &policy.SharedContext{Metadata: map[string]any{}}
	if c.provider != "" {
		shared.Metadata[metadataProviderName] = c.provider
	}
	if c.subject != "" || c.claims != nil {
		shared.AuthContext = &policy.AuthContext{Authenticated: true, AuthType: "jwt", Subject: c.subject, TypedProperties: c.claims}
	}
	return &policy.RequestContext{SharedContext: shared, Headers: policy.NewHeaders(c.headers), Path: "/chat/completions", Method: http.MethodPost}
}

// send admits the call and, when it is admitted, ends its response with the given cost.
// It returns the refusal, or nil when the call was admitted.
func send(t *testing.T, p *BudgetSystemPolicy, c call, cost string) *policy.ImmediateResponse {
	t.Helper()
	req := c.context()
	action := p.OnRequestBody(context.Background(), req, nil)
	if refused, ok := action.(policy.ImmediateResponse); ok {
		return &refused
	}
	if cost != "" {
		req.Metadata[metadataLLMCost] = cost
		req.Metadata[metadataLLMCostStatus] = costStatusCalculated
	}
	p.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: req.SharedContext}, &policy.StreamBody{EndOfStream: true}, nil)
	return nil
}

func TestBlockBudgetRefusesAtItsBlockPoint(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("team-daily", testBudgetUUID, "10.00", "day", "block", oneProvider("openai"),
		map[string]any{"blockAt": float64(50)}))
	openai := call{provider: "openai", subject: "alice"}

	if refused := send(t, p, openai, "4.9900000000"); refused != nil {
		t.Fatalf("refused below the block point: %s", refused.Body)
	}
	if refused := send(t, p, openai, "0.0100000000"); refused != nil {
		t.Fatalf("refused below the block point: %s", refused.Body)
	}
	refused := send(t, p, openai, "")
	if refused == nil {
		t.Fatal("admitted at the block point ($5 of $10 at 50%)")
	}
	if refused.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d", refused.StatusCode)
	}
	var body refusalBody
	if err := json.Unmarshal(refused.Body, &body); err != nil {
		t.Fatal(err)
	}
	want := refusedBudget{Budget: "team-daily", Limit: "10.00", Currency: "USD", Period: "day", ResetsAt: "2026-11-12T00:00:00Z"}
	if body.Code != "BUDGET_EXCEEDED" || len(body.Budgets) != 1 || body.Budgets[0] != want {
		t.Fatalf("body %+v", body)
	}
	if got := refused.Headers["Retry-After"]; got != "43200" {
		t.Fatalf("Retry-After %q, want 43200 (12 hours)", got)
	}

	// Another provider is not covered.
	if refused := send(t, p, call{provider: "anthropic", subject: "alice"}, ""); refused != nil {
		t.Fatal("refused a provider the budget does not cover")
	}
}

func TestAllowBudgetNeverRefuses(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("watch", testBudgetUUID, "1", "day", "allow", allProviders, nil))
	for i := 0; i < 3; i++ {
		if refused := send(t, p, call{provider: "openai"}, "5.0"); refused != nil {
			t.Fatal("an Allow budget refused")
		}
	}
	shared := uuid.NewSHA1(uuid.MustParse(testBudgetUUID), nil).String()
	if got := p.tracker.spentIn(spendKey{shared, periodStart(periodDay, wednesdayNoon)}); got != 15 {
		t.Fatalf("spent %v, want 15", got)
	}
}

func TestEveryRefusingBudgetIsListedOnceInHandleOrder(t *testing.T) {
	p, _, _ := testPolicy(t,
		budgetBody("z-monthly", otherUUID, "1", "month", "block", allProviders, nil),
		budgetBody("a-weekly", testBudgetUUID, "1", "week", "block", allProviders, nil),
	)
	send(t, p, call{provider: "openai"}, "2")
	refused := send(t, p, call{provider: "openai"}, "")
	if refused == nil {
		t.Fatal("admitted")
	}
	var body refusalBody
	_ = json.Unmarshal(refused.Body, &body)
	if len(body.Budgets) != 2 || body.Budgets[0].Budget != "a-weekly" || body.Budgets[1].Budget != "z-monthly" {
		t.Fatalf("budgets %+v", body.Budgets)
	}
	if body.Budgets[0].ResetsAt != "2026-11-16T00:00:00Z" || body.Budgets[1].ResetsAt != "2026-12-01T00:00:00Z" {
		t.Fatalf("resets %+v", body.Budgets)
	}
	// Retry after the last one resets: 1 Dec 00:00 is 19.5 days away.
	if got := refused.Headers["Retry-After"]; got != "1684800" {
		t.Fatalf("Retry-After %q", got)
	}
}

func TestConsumersAndExemptions(t *testing.T) {
	research := attribute("shared", `'research' in request.Auth.Claims["groups"]`)
	p, _, _ := testPolicy(t, budgetBody("research", testBudgetUUID, "1", "day", "block", allProviders, map[string]any{
		"consumers":  []any{research},
		"exemptions": []any{map[string]any{"type": "attribute", "expression": `request.Auth.Claims["sub"] == "admin"`}},
	}))
	member := call{provider: "openai", subject: "alice", claims: map[string]any{"groups": []any{"research", "ml"}}}
	outsider := call{provider: "openai", subject: "bob", claims: map[string]any{"groups": []any{"sales"}}}
	admin := call{provider: "openai", subject: "admin", claims: map[string]any{"groups": []any{"research"}}}
	anonymous := call{provider: "openai"}

	send(t, p, member, "1")
	if send(t, p, member, "") == nil {
		t.Fatal("a member was admitted after the budget was used up")
	}
	if send(t, p, outsider, "") != nil {
		t.Fatal("a non-member was refused")
	}
	if send(t, p, admin, "") != nil {
		t.Fatal("an exempt caller was refused")
	}
	if send(t, p, anonymous, "") != nil {
		t.Fatal("a request without the claim failed or was refused")
	}
}

func TestSeparateAllowances(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("per-user", testBudgetUUID, "1", "day", "block", allProviders,
		consumers(attribute("separate", `request.Auth.Claims["sub"]`))))
	alice := call{provider: "openai", subject: "alice"}
	bob := call{provider: "openai", subject: "bob"}

	send(t, p, alice, "1")
	if send(t, p, alice, "") == nil {
		t.Fatal("alice was admitted after using up her allowance")
	}
	if send(t, p, bob, "") != nil {
		t.Fatal("bob was refused for alice's spend")
	}
	if send(t, p, call{provider: "openai"}, "") != nil {
		t.Fatal("a request without a subject was refused instead of not matching")
	}
}

func TestSeparateListClaimChargesEveryValue(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("per-team", testBudgetUUID, "1", "day", "block", allProviders,
		consumers(attribute("separate", `request.Auth.Claims["groups"]`))))

	send(t, p, call{provider: "openai", subject: "alice", claims: map[string]any{"groups": []any{"research", "ml"}}}, "1")

	for _, team := range []string{"research", "ml"} {
		if send(t, p, call{provider: "openai", subject: "carol", claims: map[string]any{"groups": []any{team}}}, "") == nil {
			t.Fatalf("team %s was not charged", team)
		}
	}
	if send(t, p, call{provider: "openai", subject: "dave", claims: map[string]any{"groups": []any{"sales"}}}, "") != nil {
		t.Fatal("an uncharged team was refused")
	}
}

func TestSeparateProviders(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("per-provider", testBudgetUUID, "1", "day", "block", eachProvider, nil))
	send(t, p, call{provider: "openai"}, "1")
	if send(t, p, call{provider: "openai"}, "") == nil {
		t.Fatal("openai's allowance was not used up")
	}
	if send(t, p, call{provider: "anthropic"}, "") != nil {
		t.Fatal("anthropic was refused for openai's spend")
	}
}

func TestAllowanceUUIDs(t *testing.T) {
	b := &budget{uuid: uuid.MustParse(testBudgetUUID)}
	if got, want := b.allowanceUUID(nil), uuid.NewSHA1(b.uuid, []byte("")).String(); got != want {
		t.Fatalf("shared %s, want %s", got, want)
	}
	got := b.allowanceUUID([]allocated{{valueTypeLLMProvider, "openai"}, {valueTypeAttribute, "alice@wso2.com"}})
	want := uuid.NewSHA1(b.uuid, []byte("attribute=alice%40wso2.com|llmProvider=openai")).String()
	if got != want {
		t.Fatalf("separate %s, want %s", got, want)
	}
}

func TestProxyLoopbackCallsAreCountedAndOtherRoutesSkipped(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("all", testBudgetUUID, "1", "day", "block", allProviders, nil))
	if send(t, p, call{}, "5") != nil {
		t.Fatal("a route without a provider was matched")
	}
	// An LLM proxy's call reaches the provider route over the loopback; that is where it counts.
	send(t, p, call{provider: "openai", headers: map[string][]string{"x-wso2-internal-loopback": {"1"}}}, "1")
	if send(t, p, call{provider: "openai"}, "") == nil {
		t.Fatal("the proxy's call was not charged")
	}
}

func TestUnpricedRequestsChargeNothingButAreCounted(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("all", testBudgetUUID, "1", "day", "block", allProviders, nil))
	req := call{provider: "openai"}.context()
	p.OnRequestBody(context.Background(), req, nil)
	req.Metadata[metadataLLMCost] = "0.0000000000"
	req.Metadata[metadataLLMCostStatus] = "not_calculated"
	p.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: req.SharedContext}, nil)

	key := spendKey{uuid.NewSHA1(uuid.MustParse(testBudgetUUID), nil).String(), periodStart(periodDay, wednesdayNoon)}
	if p.tracker.spentIn(key) != 0 || p.tracker.unpricedIn(key) != 1 {
		t.Fatalf("spent %v unpriced %d", p.tracker.spentIn(key), p.tracker.unpricedIn(key))
	}
}

func TestARequestIsChargedOnce(t *testing.T) {
	p, _, _ := testPolicy(t, budgetBody("all", testBudgetUUID, "10", "day", "block", allProviders, nil))
	req := call{provider: "openai"}.context()
	p.OnRequestBody(context.Background(), req, nil)
	req.Metadata[metadataLLMCost] = "1"
	req.Metadata[metadataLLMCostStatus] = costStatusCalculated
	p.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: req.SharedContext}, &policy.StreamBody{Chunk: []byte("x")}, nil)
	p.OnResponseBodyChunk(context.Background(), &policy.ResponseStreamContext{SharedContext: req.SharedContext}, &policy.StreamBody{EndOfStream: true}, nil)
	p.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: req.SharedContext}, nil)

	key := spendKey{uuid.NewSHA1(uuid.MustParse(testBudgetUUID), nil).String(), periodStart(periodDay, wednesdayNoon)}
	if got := p.tracker.spentIn(key); got != 1 {
		t.Fatalf("spent %v, want 1", got)
	}
}

func TestChargedToThePeriodItWasAdmittedIn(t *testing.T) {
	p, _, now := testPolicy(t, budgetBody("all", testBudgetUUID, "10", "day", "block", allProviders, nil))
	req := call{provider: "openai"}.context()
	*now = time.Date(2026, 11, 11, 23, 59, 59, 0, time.UTC)
	p.OnRequestBody(context.Background(), req, nil)
	*now = time.Date(2026, 11, 12, 0, 0, 1, 0, time.UTC)
	req.Metadata[metadataLLMCost] = "3"
	req.Metadata[metadataLLMCostStatus] = costStatusCalculated
	p.OnResponseBody(context.Background(), &policy.ResponseContext{SharedContext: req.SharedContext}, nil)

	shared := uuid.NewSHA1(uuid.MustParse(testBudgetUUID), nil).String()
	if got := p.tracker.spentIn(spendKey{shared, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)}); got != 3 {
		t.Fatalf("admitted period spent %v, want 3", got)
	}
}

func TestANewPeriodLiftsTheBlock(t *testing.T) {
	p, _, now := testPolicy(t, budgetBody("all", testBudgetUUID, "1", "day", "block", allProviders, nil))
	send(t, p, call{provider: "openai"}, "1")
	if send(t, p, call{provider: "openai"}, "") == nil {
		t.Fatal("admitted after the budget was used up")
	}
	*now = now.AddDate(0, 0, 1)
	if send(t, p, call{provider: "openai"}, "") != nil {
		t.Fatal("still refused in the next period")
	}
}

func TestEditsApplyAtOnceAndKeepSpend(t *testing.T) {
	p, source, _ := testPolicy(t, budgetBody("all", testBudgetUUID, "1", "day", "block", allProviders, nil))
	send(t, p, call{provider: "openai"}, "1")
	if send(t, p, call{provider: "openai"}, "") == nil {
		t.Fatal("admitted after the budget was used up")
	}

	source.set(budgetBody("all", testBudgetUUID, "2", "day", "block", allProviders, nil))
	if send(t, p, call{provider: "openai"}, "0.5") != nil {
		t.Fatal("raising the amount did not lift the block")
	}
	source.set(budgetBody("all", testBudgetUUID, "1", "day", "block", allProviders, nil))
	if send(t, p, call{provider: "openai"}, "") == nil {
		t.Fatal("lowering the amount below spend did not block at once")
	}
	source.set()
	if send(t, p, call{provider: "openai"}, "") != nil {
		t.Fatal("a withdrawn budget still refused")
	}
}

func TestAnInvalidBudgetIsSkipped(t *testing.T) {
	p, _, _ := testPolicy(t,
		budgetBody("broken", otherUUID, "1", "day", "block", allProviders, consumers(attribute("shared", `request.Auth.Claims[`))),
		budgetBody("wrong-type", uuid.NewString(), "1", "day", "block", allProviders, consumers(attribute("shared", `"text"`))),
		budgetBody("works", testBudgetUUID, "1", "day", "block", allProviders, nil),
	)
	if got := len(p.catalog.current()); got != 1 {
		t.Fatalf("%d budgets compiled, want 1", got)
	}
	send(t, p, call{provider: "openai"}, "1")
	if send(t, p, call{provider: "openai"}, "") == nil {
		t.Fatal("the valid budget did not apply")
	}
}

func TestPeriods(t *testing.T) {
	cases := []struct {
		period      string
		start, ends string
	}{
		{periodDay, "2026-11-11T00:00:00Z", "2026-11-12T00:00:00Z"},
		{periodWeek, "2026-11-09T00:00:00Z", "2026-11-16T00:00:00Z"},
		{periodMonth, "2026-11-01T00:00:00Z", "2026-12-01T00:00:00Z"},
	}
	for _, tc := range cases {
		start := periodStart(tc.period, wednesdayNoon)
		if got := start.Format(time.RFC3339); got != tc.start {
			t.Errorf("%s starts %s, want %s", tc.period, got, tc.start)
		}
		if got := periodEnd(tc.period, start).Format(time.RFC3339); got != tc.ends {
			t.Errorf("%s ends %s, want %s", tc.period, got, tc.ends)
		}
	}
	sunday := time.Date(2026, 11, 15, 23, 0, 0, 0, time.UTC)
	if got := periodStart(periodWeek, sunday).Format(time.RFC3339); got != "2026-11-09T00:00:00Z" {
		t.Errorf("a Sunday belongs to the week that started %s", got)
	}
}
