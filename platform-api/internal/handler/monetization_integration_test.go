/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

// End-to-end coverage for meters and plan pricing over the real handler, webhook receiver, service
// and SQLite-backed repository stack. These pin the REST shape: a meter's type key and filter
// fields as its API Portal event stores them, and a plan's flat rate or usage prices with their
// filters as they are put, read back and shown on the plan.

package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/middleware"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/service"
	"github.com/wso2/api-platform/platform-api/internal/webhook"

	_ "github.com/mattn/go-sqlite3"
)

// monetizationITWebhook is the API Portal webhook the test environment receives meter events on.
var monetizationITWebhook = config.Webhook{
	Enabled:            true,
	Secret:             "monetization-it-secret",
	SignatureTolerance: 5 * time.Minute,
	MaxBodySize:        1 << 20,
	SignatureHeader:    "X-Api-Portal-Signature",
}

// setupMonetizationHandlerTestEnv builds the meter, pricing and plan handlers, and the API Portal
// webhook, over a real SQLite database carrying the production schema, with one organization and
// two meters: llm-input, adding up the input tokens of AI API requests, whose prices
// may filter on the API type, and api-calls, counting API requests, whose prices take no filters. The
// organization has no API Portal.
func setupMonetizationHandlerTestEnv(t *testing.T) http.Handler {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "monetization-handler-test.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db := &database.DB{DB: sqlDB}

	schema, err := os.ReadFile(filepath.Join("..", "database", "schema.sqlite.sql"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid)
		VALUES (?, 'plan-org', 'Plan Org', 'default', 'idp-ref')`, planITOrgID); err != nil {
		t.Fatalf("seed organization: %v", err)
	}

	meterRepo := repository.NewMeterRepo(db)
	inputKey := "metadata.aiTokenUsage.promptTokens"
	for _, meter := range []*model.Meter{
		{Handle: "llm-input", DisplayName: "LLM input tokens", Type: model.MeterTypeTokens,
			TypeKey: &inputKey, FilterFields: []model.MeterFilterField{{Field: "API type", EventKey: "metadata.apiType"},
				{Field: "Prompt tokens", EventKey: inputKey}}},
		{Handle: "api-calls", DisplayName: "API calls", Type: model.MeterTypeRequestCount},
	} {
		meter.OrganizationUUID = planITOrgID
		meter.SourceUpdatedAt = time.Now().UTC()
		if err := meterRepo.Upsert(meter); err != nil {
			t.Fatalf("seed meter %s: %v", meter.Handle, err)
		}
	}

	planRepo := repository.NewSubscriptionPlanRepo(db)
	pricingRepo := repository.NewSubscriptionPlanPricingRepo(db)
	identityService := service.NewIdentityService(repository.NewUserIdentityMappingRepo(db))
	planService := service.NewSubscriptionPlanService(
		planRepo,
		repository.NewGatewayRepo(db),
		repository.NewOrganizationRepo(db),
		service.NewGatewayEventsService(noopEventHub{}, identityService, slog.Default()),
		slog.Default(),
		nil,
	)
	planService.SetPricingRepositories(meterRepo, pricingRepo)
	// Meters and pricing are kept here, so no API Portal is involved.
	adminService := service.NewMonetizationAdminService(meterRepo, pricingRepo, planRepo, slog.Default())

	receiver, err := webhook.NewReceiver(monetizationITWebhook, nil, nil, nil, planService,
		service.NewMonetizationService(meterRepo, pricingRepo, slog.Default()), repository.NewOrganizationRepo(db), slog.Default())
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}

	mux := http.NewServeMux()
	planHandler := NewSubscriptionPlanHandler(planService, identityService, slog.Default())
	planHandler.SetPricingRepository(pricingRepo)
	planHandler.RegisterRoutes(mux)
	NewMonetizationHandler(adminService, slog.Default()).RegisterRoutes(mux)
	receiver.RegisterRoutes(mux)
	return middleware.NewTestContextMiddleware(mux)
}

// postPortalEvent delivers a signed API Portal event for the test organization, made now.
func postPortalEvent(t *testing.T, h http.Handler, eventType, data string) *httptest.ResponseRecorder {
	t.Helper()
	return postPortalEventAt(t, h, eventType, time.Now(), data)
}

// postPortalEventAt delivers a signed API Portal event for the test organization, made at occurredAt.
func postPortalEventAt(t *testing.T, h http.Handler, eventType string, occurredAt time.Time, data string) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{"event_id":"evt-` + strconv.FormatInt(time.Now().UnixNano(), 10) + `","event_type":"` + eventType +
		`","occurred_at":"` + occurredAt.UTC().Format(time.RFC3339Nano) + `","org":{"ref_id":"plan-org"},"data":` + data + `}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(monetizationITWebhook.Secret))
	mac.Write([]byte(ts + "." + string(body)))
	req := httptest.NewRequest(http.MethodPost, webhook.RoutePath, bytes.NewReader(body))
	req.Header.Set(monetizationITWebhook.SignatureHeader, "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeJSON decodes a response body into generic JSON values, for comparing whole shapes.
func decodeJSON(t *testing.T, body string) map[string]any {
	t.Helper()

	var v map[string]any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return v
}

func TestMonetizationHandler_MeterShape(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)

	rec := planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/llm-input", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get meter: status %d, body %s", rec.Code, rec.Body.String())
	}
	// The meter's own event key and each filter property's name and key, as the API Portal sent them.
	want := decodeJSON(t, `{"id":"llm-input","displayName":"LLM input tokens","description":"","type":"TOKENS",
		"typeKey":"metadata.aiTokenUsage.promptTokens",
		"filterFields":[{"field":"API type","eventKey":"metadata.apiType"},
		  {"field":"Prompt tokens","eventKey":"metadata.aiTokenUsage.promptTokens"}],"planCount":0}`)
	if got := withoutUpdatedAt(t, decodeJSON(t, rec.Body.String())); !reflect.DeepEqual(got, want) {
		t.Fatalf("meter = %v\nwant %v", got, want)
	}

	// A REQUEST_COUNT meter has no event key, and these prices take no filters.
	rec = planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/api-calls", "")
	want = decodeJSON(t, `{"id":"api-calls","displayName":"API calls","description":"","type":"REQUEST_COUNT",
		"typeKey":null,"filterFields":[],"planCount":0}`)
	if got := withoutUpdatedAt(t, decodeJSON(t, rec.Body.String())); !reflect.DeepEqual(got, want) {
		t.Fatalf("meter = %v\nwant %v", got, want)
	}
}

func TestMonetizationHandler_MeterEventIsStoredAndListed(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)

	// Custom keys, as for a gateway that names its analytics fields differently.
	rec := postPortalEvent(t, h, webhook.EventMeterUpdated, `{"handle":"llm-input","display_name":"LLM input tokens",
		"description":"Prompt tokens","type":"TOKENS","type_key":"usage.prompt_tokens",
		"filter_fields":[{"field":"API type","event_key":"custom.api.kind"},{"field":"HTTP method","event_key":"request.verb"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("meter.updated: status %d, body %s", rec.Code, rec.Body.String())
	}

	rec = planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list meters: status %d, body %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Count int              `json:"count"`
		List  []map[string]any `json:"list"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || page.Count != 2 || len(page.List) != 2 {
		t.Fatalf("list = %s, %v", rec.Body.String(), err)
	}
	// Listed by handle: api-calls, then llm-input with the keys of the event, in order.
	want := decodeJSON(t, `{"id":"llm-input","displayName":"LLM input tokens","description":"Prompt tokens","type":"TOKENS",
		"typeKey":"usage.prompt_tokens",
		"filterFields":[{"field":"API type","eventKey":"custom.api.kind"},{"field":"HTTP method","eventKey":"request.verb"}],
		"planCount":0}`)
	if got := withoutUpdatedAt(t, page.List[1]); !reflect.DeepEqual(got, want) {
		t.Fatalf("listed meter = %v\nwant %v", got, want)
	}
}

// The meters list is searched by name or id, and each meter names the plans that charge for it; the
// plans list can be narrowed to one status.
func TestMonetizationHandler_MetersAreSearchedAndNameTheirPlans(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	if rec := createPricedPlan(t, h, `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[
		{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"2.50","unitQuantity":1000000}]}`); rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}

	type meterPage struct {
		List []struct {
			ID        string `json:"id"`
			PlanCount int    `json:"planCount"`
		} `json:"list"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	list := func(query string) meterPage {
		t.Helper()
		rec := planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters"+query, "")
		var page meterPage
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
			t.Fatalf("list meters%s: status %d, body %s", query, rec.Code, rec.Body.String())
		}
		return page
	}
	page := list("?query=LLM")
	if page.Pagination.Total != 1 || len(page.List) != 1 || page.List[0].ID != "llm-input" ||
		page.List[0].PlanCount != 1 {
		t.Fatalf("search LLM = %+v", page)
	}
	if page := list("?query=calls"); page.Pagination.Total != 1 || page.List[0].ID != "api-calls" || page.List[0].PlanCount != 0 {
		t.Fatalf("search calls = %+v", page)
	}
	if page := list("?limit=1&offset=1"); page.Pagination.Total != 2 || len(page.List) != 1 || page.List[0].ID != "llm-input" {
		t.Fatalf("second page = %+v", page)
	}
	rec := planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/llm-input", "")
	if got := decodeJSON(t, rec.Body.String())["planCount"]; got != float64(1) {
		t.Fatalf("meter planCount = %v", got)
	}

	plans := func(query string) (int, int) {
		t.Helper()
		rec := planRequest(t, h, http.MethodGet, planITPath+query, "")
		var page struct {
			Pagination struct {
				Total int `json:"total"`
			} `json:"pagination"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &page)
		return rec.Code, page.Pagination.Total
	}
	if code, total := plans("?status=active"); code != http.StatusOK || total != 1 {
		t.Fatalf("active plans: %d, total %d", code, total)
	}
	if code, total := plans("?status=INACTIVE"); code != http.StatusOK || total != 0 {
		t.Fatalf("inactive plans: %d, total %d", code, total)
	}
	if code, _ := plans("?status=DELETED"); code != http.StatusBadRequest {
		t.Fatalf("an unknown status: %d, want 400", code)
	}
}

// withoutUpdatedAt drops the audit time from a decoded pricing, after checking that it is there.
func withoutUpdatedAt(t *testing.T, v map[string]any) map[string]any {
	t.Helper()

	if _, ok := v["updatedAt"].(string); !ok {
		t.Errorf("updatedAt missing from %v", v)
	}
	delete(v, "updatedAt")
	return v
}

// createPricedPlan creates the plan gold with the given pricing and returns the response.
func createPricedPlan(t *testing.T, h http.Handler, pricing string) *httptest.ResponseRecorder {
	t.Helper()
	return planRequest(t, h, http.MethodPost, planITPath, `{"id":"gold","displayName":"Gold","pricing":`+pricing+`}`)
}

func TestMonetizationHandler_UsagePricesRoundTrip(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	rec := createPricedPlan(t, h, `{"pricingType":"usage_based","currency":"usd","billingPeriod":"MONTH","prices":[
		{"meterId":"llm-input","filters":[{"field":" api type ","operator":"equals","values":[" LlmProvider "]}],
		 "pricingModel":"PER_UNIT","unitAmount":"2.50","unitQuantity":1000000},
		{"meterId":"llm-input","filters":[{"field":"API TYPE","operator":"IN","values":["Mcp","LlmProxy","Mcp"]}],
		 "pricingModel":"PER_UNIT","unitAmount":"0.15","unitQuantity":1000000},
		{"meterId":"api-calls","pricingModel":"GRADUATED",
		 "tiers":[{"upTo":10000,"unitAmount":"0"},{"upTo":null,"unitAmount":"0.001"}]}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}

	// One meter priced twice for different API types; codes upper-cased, each filter naming its property
	// as the meter spells it, filter values normalized, every field of a price present (null when its
	// pricing model does not use it), and no flat amount.
	want := decodeJSON(t, `{"subscriptionPlanId":"gold","pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH",
		"flatAmount":null,"prices":[
		{"meterId":"llm-input","filters":[{"field":"API type","operator":"EQUALS","values":["LlmProvider"]}],
		 "pricingModel":"PER_UNIT","unitAmount":"2.50","unitQuantity":1000000,"tiers":null},
		{"meterId":"llm-input","filters":[{"field":"API type","operator":"IN","values":["Mcp","LlmProxy"]}],
		 "pricingModel":"PER_UNIT","unitAmount":"0.15","unitQuantity":1000000,"tiers":null},
		{"meterId":"api-calls","filters":[],"pricingModel":"GRADUATED","unitAmount":null,
		 "unitQuantity":null,"tiers":[{"upTo":10000,"unitAmount":"0","flatAmount":null},{"upTo":null,"unitAmount":"0.001","flatAmount":null}]}]}`)
	created := decodeJSON(t, rec.Body.String())
	if pricing, _ := created["pricing"].(map[string]any); pricing == nil || !reflect.DeepEqual(withoutUpdatedAt(t, pricing), want) {
		t.Fatalf("create response pricing = %v\nwant %v", created["pricing"], want)
	}

	rec = planRequest(t, h, http.MethodGet, planITPath+"/gold/pricing", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get pricing: status %d, body %s", rec.Code, rec.Body.String())
	}
	if got := withoutUpdatedAt(t, decodeJSON(t, rec.Body.String())); !reflect.DeepEqual(got, want) {
		t.Fatalf("get response = %v\nwant %v", got, want)
	}

	rec = planRequest(t, h, http.MethodGet, planITPath+"/gold", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	plan := decodeJSON(t, rec.Body.String())
	pricing, _ := plan["pricing"].(map[string]any)
	if pricing == nil || !reflect.DeepEqual(withoutUpdatedAt(t, pricing), want) {
		t.Fatalf("plan pricing = %v\nwant %v", plan["pricing"], want)
	}
}

func TestMonetizationHandler_FlatRateRoundTrip(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	if rec := createPricedPlan(t, h, `{"pricingType":"FLAT_RATE","currency":"EUR","billingPeriod":"YEAR","flatAmount":"200.00"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	want := decodeJSON(t, `{"subscriptionPlanId":"gold","pricingType":"FLAT_RATE","currency":"EUR","billingPeriod":"YEAR",
		"flatAmount":"200.00","prices":[]}`)
	rec := planRequest(t, h, http.MethodGet, planITPath+"/gold/pricing", "")
	if got := withoutUpdatedAt(t, decodeJSON(t, rec.Body.String())); rec.Code != http.StatusOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("get pricing: status %d, %v\nwant %v", rec.Code, got, want)
	}
}

// The pricing of a plan the API Portal creates is dated when the portal made it, the event's
// occurred_at, not when the event was processed; it is stored and read back as the portal sent it.
func TestMonetizationHandler_PortalPlanPricingIsDatedWhenThePortalMadeIt(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	for handle, pricing := range map[string]string{
		"gold":   `{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"29.00","prices":[]}`,
		"silver": `{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[{"meter_handle":"api-calls","pricing_model":"PER_UNIT","unit_amount":"0.01"}]}`,
	} {
		madeAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanCreated, madeAt,
			`{"handle":"`+handle+`","display_name":"`+handle+`","limits":[],"pricing":`+pricing+`}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("subscription_plan.created %s: status %d, body %s", handle, rec.Code, rec.Body.String())
		}
		rec = planRequest(t, h, http.MethodGet, planITPath+"/"+handle+"/pricing", "")
		got := decodeJSON(t, rec.Body.String())
		if rec.Code != http.StatusOK || got["updatedAt"] != madeAt.Format(time.RFC3339) {
			t.Fatalf("pricing of %s: status %d, body %s; want updatedAt %s", handle, rec.Code, rec.Body.String(), madeAt.Format(time.RFC3339))
		}
		if handle == "gold" && (got["pricingType"] != "FLAT_RATE" || got["flatAmount"] != "29.00" || !reflect.DeepEqual(got["prices"], []any{})) {
			t.Errorf("pricing of gold = %v, want the flat rate 29.00 with no prices", got)
		}
		if handle == "silver" && (got["pricingType"] != "USAGE_BASED" || got["flatAmount"] != nil || len(got["prices"].([]any)) != 1) {
			t.Errorf("pricing of silver = %v, want one usage price and no flat amount", got)
		}
	}
}

// A plan created without pricing, or with a null one, is free.
func TestMonetizationHandler_PlanWithoutPricingIsFree(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	for _, body := range []string{`{"id":"free","displayName":"Free"}`, `{"id":"basic","displayName":"Basic","pricing":null}`} {
		rec := planRequest(t, h, http.MethodPost, planITPath, body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: status %d, body %s", body, rec.Code, rec.Body.String())
		}
		if _, ok := decodeJSON(t, rec.Body.String())["pricing"]; ok {
			t.Fatalf("create %s: a free plan has no pricing, got %s", body, rec.Body.String())
		}
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/basic/pricing", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get pricing of a free plan: status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestMonetizationHandler_OverlappingPricesAreRefused(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)

	// Requests to an Mcp API match both prices.
	price := `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"API type","operator":"IN","values":["LlmProvider","Mcp"]}]}`
	other := `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"2","filters":[{"field":"API type","operator":"NOT_EQUALS","values":["LlmProvider"]}]}`
	rec := createPricedPlan(t, h, `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[`+price+`,`+other+`]}`)
	if rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "Prices 1 and 2 both count the same requests of the meter llm-input; give them different filters.") {
		t.Fatalf("overlapping prices: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a plan with refused pricing must not be created: status %d, body %s", rec.Code, rec.Body.String())
	}
}

func TestMonetizationHandler_PricesFilterOnlyOnTheirMetersProperties(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)

	// llm-input has the properties API type and Prompt tokens only.
	calls := `{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"0.01"}`
	byMethod := `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"2","filters":[{"field":"HTTP method","operator":"EQUALS","values":["POST"]}]}`
	rec := createPricedPlan(t, h, `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[`+calls+`,`+byMethod+`]}`)
	if rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "price 2: the meter llm-input has no property HTTP method") {
		t.Fatalf("filter on a property the meter does not have: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a plan with refused pricing must not be created: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// Pricing is set only when a plan is created: a plan update carrying pricing is refused, and the
// pricing PUT and DELETE are gone.
func TestMonetizationHandler_PricingCannotChangeAfterCreate(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	if rec := createPricedPlan(t, h, `{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"9.00"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}

	for _, pricing := range []string{`{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"19.00"}`, `null`} {
		rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", `{"id":"gold","displayName":"Gold","pricing":`+pricing+`}`)
		got := decodeJSON(t, rec.Body.String())
		if rec.Code != http.StatusConflict || got["code"] != "SUBSCRIPTION_PLAN_PRICING_LOCKED" ||
			got["message"] != "Pricing can't be updated once a plan is created. Create a new plan with the new pricing." {
			t.Fatalf("update with pricing %s: status %d, body %s", pricing, rec.Code, rec.Body.String())
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := planRequest(t, h, method, planITPath+"/gold/pricing", `{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"19.00"}`)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s pricing: status %d, want 405", method, rec.Code)
		}
	}

	// An update without pricing keeps it.
	if rec := planRequest(t, h, http.MethodPut, planITPath+"/gold", `{"id":"gold","displayName":"Gold Plus"}`); rec.Code != http.StatusOK {
		t.Fatalf("update: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec := planRequest(t, h, http.MethodGet, planITPath+"/gold/pricing", "")
	if got := decodeJSON(t, rec.Body.String()); rec.Code != http.StatusOK || got["flatAmount"] != "9.00" {
		t.Fatalf("pricing after update: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// Any pricing here on a meter locks the whole meter, name and description included. A meter no
// pricing here uses can change or go.
func TestMonetizationHandler_MeterPricedHereCannotChange(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	if rec := createPricedPlan(t, h, `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH",
		"prices":[{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[{"field":"API type","operator":"EQUALS","values":["LlmProvider"]}]}]}`); rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}

	refused := func(rec *httptest.ResponseRecorder, code, message string) {
		t.Helper()
		got := decodeJSON(t, rec.Body.String())
		if rec.Code != http.StatusConflict || got["code"] != code || (message != "" && got["message"] != message) {
			t.Fatalf("status %d, body %s; want 409 %s %q", rec.Code, rec.Body.String(), code, message)
		}
	}
	meterPath := constants.APIBasePath + "/meters/llm-input"
	refused(planRequest(t, h, http.MethodPut, meterPath, `{"displayName":"LLM input tokens","type":"TOKENS",
		"typeKey":"metadata.aiTokenUsage.promptTokens"}`),
		"METER_IN_USE", "The meter is in use, so it can't be updated.")
	refused(planRequest(t, h, http.MethodPut, meterPath, `{"displayName":"Prompt tokens","description":"Renamed","type":"TOKENS",
		"typeKey":"metadata.aiTokenUsage.promptTokens","filterFields":[{"field":"API type","eventKey":"metadata.apiType"}]}`),
		"METER_IN_USE", "The meter is in use, so it can't be updated.")
	// The keys are part of the meter: changing only them is refused too.
	refused(planRequest(t, h, http.MethodPut, meterPath, `{"displayName":"LLM input tokens","type":"TOKENS",
		"typeKey":"usage.prompt_tokens","filterFields":[{"field":"API type","eventKey":"custom.api.kind"}]}`),
		"METER_IN_USE", "The meter is in use, so it can't be updated.")
	refused(planRequest(t, h, http.MethodDelete, meterPath, ""),
		"METER_IN_USE", "The meter is in use, so it can't be deleted.")

	// A meter no pricing here uses changes and goes here; no API Portal is involved.
	if rec := planRequest(t, h, http.MethodPut, constants.APIBasePath+"/meters/api-calls", `{"displayName":"Calls","type":"REQUEST_COUNT"}`); rec.Code != http.StatusOK {
		t.Fatalf("update an unpriced meter: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodDelete, constants.APIBasePath+"/meters/api-calls", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete an unpriced meter: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// The API Portal's deletions are followed: a deleted plan or meter takes its pricing with it, a
// repeated deletion and a deletion of something not here answer 200, and a later created event
// makes the plan again.
func TestMonetizationHandler_PortalDeletionsAreFollowed(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	if rec := createPricedPlan(t, h, `{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"9.00"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	deletedAt := time.Now().UTC()

	for i := 0; i < 2; i++ {
		if rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanDeleted, deletedAt, `{"handle":"gold"}`); rec.Code != http.StatusOK {
			t.Fatalf("subscription_plan.deleted #%d: status %d, body %s", i+1, rec.Code, rec.Body.String())
		}
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("the deleted plan must be gone: status %d", rec.Code)
	}
	created := `{"handle":"gold","display_name":"Gold","limits":[]}`
	if rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanCreated, deletedAt.Add(time.Second), created); rec.Code != http.StatusOK {
		t.Fatalf("newer subscription_plan.created: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold", ""); rec.Code != http.StatusOK {
		t.Fatalf("a newer event creates the plan again: status %d", rec.Code)
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold/pricing", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("the deleted plan's pricing must be gone: status %d", rec.Code)
	}

	if rec := postPortalEventAt(t, h, webhook.EventMeterDeleted, deletedAt, `{"handle":"api-calls"}`); rec.Code != http.StatusOK {
		t.Fatalf("meter.deleted: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/api-calls", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("the deleted meter must be gone: status %d", rec.Code)
	}
	if rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanDeleted, deletedAt, `{"handle":"never-here"}`); rec.Code != http.StatusOK {
		t.Fatalf("deleting a plan not here: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// A subscription_plan.deleted that happened before the plan here was created is of an older plan
// with the same handle: the plan made through the REST API afterwards stays.
func TestMonetizationHandler_LateDeletionLeavesAPlanCreatedAfterIt(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	deletedAt := time.Now().UTC().Add(-time.Minute)
	if rec := planRequest(t, h, http.MethodPost, planITPath, `{"id":"gold","displayName":"Gold"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanDeleted, deletedAt, `{"handle":"gold"}`); rec.Code != http.StatusOK {
		t.Fatalf("subscription_plan.deleted: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold", ""); rec.Code != http.StatusOK {
		t.Fatalf("the plan created after the deletion must stay: status %d", rec.Code)
	}
}

// A plan the API Portal creates and deletes moments later is deleted here too: its copy is dated
// when the portal made it, not when the created event was processed.
func TestMonetizationHandler_PortalPlanCreatedThenDeletedIsGone(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	now := time.Now().UTC()
	if rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanCreated, now.Add(-2*time.Second),
		`{"handle":"gold","display_name":"Gold","limits":[]}`); rec.Code != http.StatusOK {
		t.Fatalf("subscription_plan.created: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := postPortalEventAt(t, h, webhook.EventSubscriptionPlanDeleted, now.Add(-time.Second), `{"handle":"gold"}`); rec.Code != http.StatusOK {
		t.Fatalf("subscription_plan.deleted: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, planITPath+"/gold", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("the plan the portal deleted must be gone: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// A meter is kept here, like a plan, with no API Portal involved, and answered in the shape GET
// /meters/{meterId} gives: the filter properties' names and event keys as stored, trimmed.
func TestMonetizationHandler_MetersAreKeptHere(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	body := `{"displayName":"Output tokens","type":"TOKENS","typeKey":"usage.output_tokens",
		"filterFields":[{"field":"API type","eventKey":"custom.api.kind"},{"field":" HTTP method ","eventKey":"request.verb"}]}`
	rec := planRequest(t, h, http.MethodPost, constants.APIBasePath+"/meters", body)
	want := decodeJSON(t, `{"id":"output-tokens","displayName":"Output tokens","description":"","type":"TOKENS",
		"typeKey":"usage.output_tokens","planCount":0,
		"filterFields":[{"field":"API type","eventKey":"custom.api.kind"},{"field":"HTTP method","eventKey":"request.verb"}]}`)
	if rec.Code != http.StatusCreated || !reflect.DeepEqual(withoutUpdatedAt(t, decodeJSON(t, rec.Body.String())), want) {
		t.Fatalf("create meter: status %d, body %s; want %v", rec.Code, rec.Body.String(), want)
	}
	if got, want := rec.Header().Get("Location"), constants.APIBasePath+"/meters/output-tokens"; got != want {
		t.Fatalf("create meter: Location %q, want %q", got, want)
	}
	rec = planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/output-tokens", "")
	if rec.Code != http.StatusOK || !reflect.DeepEqual(withoutUpdatedAt(t, decodeJSON(t, rec.Body.String())), want) {
		t.Fatalf("read it back: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec = planRequest(t, h, http.MethodPost, constants.APIBasePath+"/meters", body)
	if got := decodeJSON(t, rec.Body.String()); rec.Code != http.StatusConflict || got["code"] != "METER_EXISTS" {
		t.Fatalf("the same id again: status %d, body %s; want 409 METER_EXISTS", rec.Code, rec.Body.String())
	}

	// A meter no pricing here uses can change its keys.
	update := `{"displayName":"API calls","type":"REQUEST_COUNT","filterFields":[{"field":"Status code","eventKey":"status_code"}]}`
	rec = planRequest(t, h, http.MethodPut, constants.APIBasePath+"/meters/api-calls", update)
	got := decodeJSON(t, rec.Body.String())
	if rec.Code != http.StatusOK || got["description"] != "" || got["planCount"] != float64(0) ||
		!reflect.DeepEqual(got["filterFields"], []any{map[string]any{"field": "Status code", "eventKey": "status_code"}}) {
		t.Fatalf("update meter: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodDelete, constants.APIBasePath+"/meters/output-tokens", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete meter: status %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/output-tokens", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a deleted meter: status %d", rec.Code)
	}

	// A refusal is the API Portal's own sentence; a body that is not a JSON object is refused too.
	rec = planRequest(t, h, http.MethodPost, constants.APIBasePath+"/meters", `{"displayName":"Calls","type":"REQUEST_COUNT","color":"red"}`)
	if got := decodeJSON(t, rec.Body.String()); rec.Code != http.StatusBadRequest || got["message"] != "color is not a known field" {
		t.Fatalf("an unknown field: status %d, body %s", rec.Code, rec.Body.String())
	}
	for _, nonObject := range []string{`null`, `[]`, `"meter"`} {
		rec = planRequest(t, h, http.MethodPost, constants.APIBasePath+"/meters", nonObject)
		if rec.Code != http.StatusBadRequest || decodeJSON(t, rec.Body.String())["code"] != "VALIDATION_FAILED" {
			t.Fatalf("POST /meters %s: status %d, body %s", nonObject, rec.Code, rec.Body.String())
		}
	}
}

// A meter may offer API type as a filter property, so that one plan prices the calls of each kind of
// API on its own; prices on the same API types still overlap.
func TestMonetizationHandler_PricesByAPIType(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	if rec := postPortalEvent(t, h, webhook.EventMeterUpdated, `{"handle":"api-calls","display_name":"API requests",
		"type":"REQUEST_COUNT","filter_fields":[{"field":"API type","event_key":"metadata.apiType"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("meter.updated: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec := planRequest(t, h, http.MethodGet, constants.APIBasePath+"/meters/api-calls", "")
	if got := decodeJSON(t, rec.Body.String())["filterFields"]; !reflect.DeepEqual(got,
		[]any{map[string]any{"field": "API type", "eventKey": "metadata.apiType"}}) {
		t.Fatalf("filter fields = %v", got)
	}

	byType := func(operator, value string) string {
		return `{"meterId":"api-calls","pricingModel":"PER_UNIT","unitAmount":"0.01",
			"filters":[{"field":"API type","operator":"` + operator + `","values":["` + value + `"]}]}`
	}
	rec = createPricedPlan(t, h, `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[`+
		byType("EQUALS", "RestApi")+`,`+byType("NOT_EQUALS", "RestApi")+`]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec = planRequest(t, h, http.MethodPost, planITPath, `{"id":"silver","displayName":"Silver","pricing":{"pricingType":"USAGE_BASED",
		"currency":"USD","billingPeriod":"MONTH","prices":[`+byType("EQUALS", "Mcp")+`,`+byType("NOT_EQUALS", "RestApi")+`]}}`)
	if rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "Prices 1 and 2 both count the same requests of the meter api-calls; give them different filters.") {
		t.Fatalf("overlapping API types: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// A price may count only the calls whose token counts fall in a range, by comparisons on one of the
// meter's properties; two prices are apart on it when their ranges share no number.
func TestMonetizationHandler_PricesByComparisons(t *testing.T) {
	h := setupMonetizationHandlerTestEnv(t)
	byPrompt := func(operator, value string) string {
		return `{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[
			{"field":"Prompt tokens","operator":"` + operator + `","values":["` + value + `"]}]}`
	}
	rec := createPricedPlan(t, h, `{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[`+
		byPrompt("LESS_THAN_OR_EQUAL", "333")+`,`+byPrompt("GREATER_THAN", "333")+`]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: status %d, body %s", rec.Code, rec.Body.String())
	}
	pricing, _ := decodeJSON(t, rec.Body.String())["pricing"].(map[string]any)
	prices, _ := pricing["prices"].([]any)
	if len(prices) != 2 || !reflect.DeepEqual(prices[1].(map[string]any)["filters"], []any{map[string]any{
		"field": "Prompt tokens", "operator": "GREATER_THAN", "values": []any{"333"}}}) {
		t.Fatalf("prices = %v", pricing["prices"])
	}

	rec = planRequest(t, h, http.MethodPost, planITPath, `{"id":"silver","displayName":"Silver","pricing":{"pricingType":"USAGE_BASED",
		"currency":"USD","billingPeriod":"MONTH","prices":[`+byPrompt("GREATER_THAN", "100")+`,`+byPrompt("GREATER_THAN", "333")+`]}}`)
	if rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "Prices 1 and 2 both count the same requests of the meter llm-input; give them different filters.") {
		t.Fatalf("overlapping ranges: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec = planRequest(t, h, http.MethodPost, planITPath, `{"id":"bronze","displayName":"Bronze","pricing":{"pricingType":"USAGE_BASED",
		"currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"llm-input","pricingModel":"PER_UNIT","unitAmount":"1","filters":[
			{"field":"Total tokens","operator":"GREATER_THAN","values":["0"]}]}]}}`)
	if rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "price 1: the meter llm-input has no property Total tokens") {
		t.Fatalf("a property the meter does not have: status %d, body %s", rec.Code, rec.Body.String())
	}
}
