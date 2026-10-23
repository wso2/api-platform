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

package webhook

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

type fakeMonetizationService struct {
	meters         []*model.Meter
	meterDeletes   []string
	lastOccurredAt time.Time
}

func (f *fakeMonetizationService) UpsertMeterFromWebhook(m *model.Meter) error {
	f.meters = append(f.meters, m)
	return nil
}

func (f *fakeMonetizationService) DeleteMeterFromWebhook(orgUUID, handle string, occurredAt time.Time) error {
	f.meterDeletes = append(f.meterDeletes, orgUUID+"/"+handle)
	f.lastOccurredAt = occurredAt
	return nil
}

func newMonetizationTestReceiver(t *testing.T) (*Receiver, *fakeMonetizationService, config.Webhook) {
	t.Helper()
	cfg := config.Webhook{
		Enabled:            true,
		Secret:             "monetization-test-secret",
		SignatureTolerance: 5 * time.Minute,
		MaxBodySize:        1 << 20,
		SignatureHeader:    "X-Api-Portal-Signature",
	}
	m := &fakeMonetizationService{}
	r, err := NewReceiver(cfg, nil, nil, nil, nil, m, planTestOrgs{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	return r, m, cfg
}

func postMonetizationEvent(t *testing.T, r *Receiver, cfg config.Webhook, eventType, occurredAt, data string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	body := []byte(`{"event_id":"evt-m","event_type":"` + eventType + `","occurred_at":"` + occurredAt +
		`","org":{"ref_id":"acme"},"data":` + data + `}`)
	req := httptest.NewRequest(http.MethodPost, RoutePath, bytes.NewReader(body))
	req.Header.Set(cfg.SignatureHeader, sign(cfg.Secret, time.Now().Unix(), body))
	rec := httptest.NewRecorder()
	return rec, r.ReceiveEvent(rec, req)
}

func TestMeterCreated_CopiesTheMeterWithTheTimeOfTheChange(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	rec, err := postMonetizationEvent(t, r, cfg, EventMeterCreated, "2026-10-07T12:30:00.250Z",
		`{"handle":"calls","display_name":"Successful calls","description":null,"type":"request_count","type_key":null}`)
	if err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status = %d, err = %v", rec.Code, err)
	}
	got := m.meters[0]
	if got.OrganizationUUID != planTestOrgUUID || got.Handle != "calls" || got.Type != "REQUEST_COUNT" ||
		got.TypeKey != nil || got.FilterFields == nil || len(got.FilterFields) != 0 {
		t.Errorf("meter = %+v", got)
	}
	if want := time.Date(2026, 10, 7, 12, 30, 0, 250000000, time.UTC); !got.SourceUpdatedAt.Equal(want) {
		t.Errorf("sourceUpdatedAt = %v, want %v", got.SourceUpdatedAt, want)
	}
}

// The meter's event keys are its own, editable in the API Portal: they are copied as they are,
// defaults and custom keys alike.
func TestMeterUpdated_CopiesTheEventKeys(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	if _, err := postMonetizationEvent(t, r, cfg, EventMeterUpdated, "2026-10-07T12:30:00Z",
		`{"handle":"llm-output","display_name":"Output tokens","type":"TOKENS",
		  "type_key":" usage.output_tokens ",
		  "filter_fields":[{"field":"api_type","event_key":"metadata.apiType"},{"field":"HTTP_METHOD","event_key":"custom.verb"}]}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	got := m.meters[0]
	if got.Type != "TOKENS" || got.TypeKey == nil || *got.TypeKey != "usage.output_tokens" {
		t.Errorf("meter = %+v, want a TOKENS meter adding up usage.output_tokens", got)
	}
	// Field codes upper-cased; event keys kept as received (trimmed), in order.
	want := []model.MeterFilterField{{Field: "API_TYPE", EventKey: "metadata.apiType"}, {Field: "HTTP_METHOD", EventKey: "custom.verb"}}
	if !reflect.DeepEqual(got.FilterFields, want) {
		t.Errorf("filter fields = %+v, want %+v", got.FilterFields, want)
	}
}

// API type is a filter field like any other, on any meter, with its own key.
func TestMeterUpdated_CopiesTheAPITypeField(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	if _, err := postMonetizationEvent(t, r, cfg, EventMeterUpdated, "2026-10-07T12:30:00Z",
		`{"handle":"api-calls","display_name":"API requests","type":"REQUEST_COUNT",
		  "filter_fields":[{"field":"api_type","event_key":"metadata.apiType"},{"field":"HTTP_METHOD","event_key":"request.verb"}]}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []model.MeterFilterField{{Field: "API_TYPE", EventKey: "metadata.apiType"}, {Field: "HTTP_METHOD", EventKey: "request.verb"}}
	if got := m.meters[0].FilterFields; !reflect.DeepEqual(got, want) {
		t.Errorf("filter fields = %+v, want %+v", got, want)
	}
}

// A meter may list the Token field several times, once per key.
func TestMeterUpdated_CopiesTokenFields(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	if _, err := postMonetizationEvent(t, r, cfg, EventMeterUpdated, "2026-10-07T12:30:00Z",
		`{"handle":"llm-input","display_name":"Input tokens","type":"TOKENS","type_key":"metadata.aiTokenUsage.promptTokens",
		  "filter_fields":[{"field":"token","event_key":"metadata.aiTokenUsage.totalTokens"},{"field":"TOKEN","event_key":"custom.cached"}]}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []model.MeterFilterField{{Field: "TOKEN", EventKey: "metadata.aiTokenUsage.totalTokens"}, {Field: "TOKEN", EventKey: "custom.cached"}}
	if got := m.meters[0].FilterFields; !reflect.DeepEqual(got, want) {
		t.Errorf("filter fields = %+v, want %+v", got, want)
	}
}

func TestMeterDeleted_ReadsTheSQLiteTimestampForm(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	if _, err := postMonetizationEvent(t, r, cfg, EventMeterDeleted, "2026-10-07 12:30:00", `{"handle":"calls"}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(m.meterDeletes) != 1 || m.meterDeletes[0] != planTestOrgUUID+"/calls" {
		t.Errorf("deletes = %v", m.meterDeletes)
	}
	if want := time.Date(2026, 10, 7, 12, 30, 0, 0, time.UTC); !m.lastOccurredAt.Equal(want) {
		t.Errorf("occurredAt = %v, want %v", m.lastOccurredAt, want)
	}
}

// plan_pricing.* is not an event type here: a plan's pricing comes with its subscription_plan.*
// events.
func TestPlanPricingEvents_AreUnknown(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	for _, eventType := range []string{"plan_pricing.updated", "plan_pricing.deleted"} {
		_, err := postMonetizationEvent(t, r, cfg, eventType, "2026-10-07T12:30:00Z", `{"plan_handle":"gold"}`)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed || appErr.Message != "Unsupported event type" {
			t.Fatalf("%s: want 400 Unsupported event type, got %v", eventType, err)
		}
	}
	if len(m.meters) != 0 {
		t.Fatalf("nothing may reach the service, got %d meters", len(m.meters))
	}
}

func TestMonetizationEvents_RejectMalformedData(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)

	for _, tc := range []struct{ eventType, data, reason string }{
		{EventMeterCreated, `{"display_name":"No handle","type":"REQUEST_COUNT"}`, "data.handle is required"},
		{EventMeterCreated, `{"handle":"calls","display_name":"Calls"}`, "type must be one of REQUEST_COUNT, TOKENS"},
		{EventMeterUpdated, `{"handle":"this-handle-is-much-longer-than-forty-characters","display_name":"x","type":"REQUEST_COUNT"}`,
			"data.handle must be at most 40 characters"},
		{EventMeterUpdated, `{"handle":"tokens","display_name":"Tokens","type":"TOKENS","type_key":null}`,
			"A TOKENS meter needs a typeKey"},
		{EventMeterUpdated, `{"handle":"tokens","display_name":"Tokens","type":"TOKENS","type_key":"  "}`,
			"typeKey must be 1 to 255 characters with no spaces"},
		{EventMeterUpdated, `{"handle":"tokens","display_name":"Tokens","type":"TOKENS","type_key":"metadata.ai tokens"}`,
			"typeKey must be 1 to 255 characters with no spaces"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","type_key":"metadata.aiTokenUsage.totalTokens"}`,
			"typeKey applies only to a TOKENS meter"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"TOKEN_TYPE","event_key":"tokenType"}]}`,
			"filterFields: unknown field TOKEN_TYPE"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"API_TYPE","event_key":"metadata.kind"},{"field":"api_type","event_key":"metadata.kind"}]}`,
			"filterFields: API_TYPE is listed twice"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"TOKEN"}]}`,
			"A Token field needs an eventKey"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"TOKEN","event_key":"t.total"},{"field":"TOKEN","event_key":"t.total"}]}`,
			"The meter lists the Token key t.total more than once"},
		{EventMeterUpdated, `{"handle":"tokens","display_name":"Tokens","type":"TOKENS","type_key":"t.prompt","filter_fields":[{"field":"TOKEN","event_key":"t.prompt"}]}`,
			"The key t.prompt is already the meter's Token key"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"HTTP_METHOD"}]}`,
			"The event key of HTTP method must be 1 to 255 characters with no spaces"},
		{EventMeterUpdated, `{"handle":"calls","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"HTTP_STATUS_CODE","event_key":"response status"}]}`,
			"The event key of Status code must be 1 to 255 characters with no spaces"},
	} {
		_, err := postMonetizationEvent(t, r, cfg, tc.eventType, "2026-10-07T12:30:00Z", tc.data)
		var appErr *apperror.Error
		if !errors.As(err, &appErr) || appErr.Code != apperror.CodeCommonValidationFailed || !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s %s: want a validation error for %q, got %v", tc.eventType, tc.data, tc.reason, err)
		}
	}
	if len(m.meters) != 0 {
		t.Errorf("invalid events must not reach the service")
	}
}

// Names and descriptions are limited in characters, as the API Portal counts them, not bytes: a
// 255-character name in a non-Latin script (about 510 bytes) is copied, and 256 characters are not.
func TestMeterCreated_LimitsTheNameInCharactersNotBytes(t *testing.T) {
	r, m, cfg := newMonetizationTestReceiver(t)
	name := strings.Repeat("é", 255)
	description := strings.Repeat("ü", 1023)

	rec, err := postMonetizationEvent(t, r, cfg, EventMeterCreated, "2026-10-07T12:30:00Z",
		`{"handle":"calls","display_name":"`+name+`","description":"`+description+`","type":"REQUEST_COUNT"}`)
	if err != nil || rec.Code != http.StatusOK {
		t.Fatalf("a 255-character multibyte name: status = %d, err = %v", rec.Code, err)
	}
	if len(m.meters) != 1 || m.meters[0].DisplayName != name {
		t.Fatalf("the meter must be copied with its name")
	}

	_, err = postMonetizationEvent(t, r, cfg, EventMeterCreated, "2026-10-07T12:31:00Z",
		`{"handle":"calls","display_name":"`+name+`é","type":"REQUEST_COUNT"}`)
	if err == nil || !strings.Contains(err.Error(), "at most 255 characters") {
		t.Fatalf("a 256-character name: err = %v, want it refused", err)
	}
}
