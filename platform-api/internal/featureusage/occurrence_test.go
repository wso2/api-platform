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

package featureusage

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	testOrgUUID = "8f1c2b3a-4d5e-4f60-8a7b-9c0d1e2f3a4b"
	testAPIUUID = "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"
)

const testURI = "https://platform-api.example.com/api/v0.9/rest-apis"

var testRequestTime = time.Date(2026, 11, 6, 8, 15, 0, 0, time.FixedZone("IST", 19800))

func validInvocation() Invocation {
	return Invocation{
		Method:       "post",
		URI:          testURI,
		Status:       201,
		RequestTime:  testRequestTime,
		ResponseTime: testRequestTime.Add(250 * time.Millisecond),
	}
}

func validCreation() Occurrence {
	return Occurrence{
		Feature:      FeatureCreateRESTAPI,
		OrgUUID:      testOrgUUID,
		ActorSubject: testUserUUID,
		OccurrenceID: testAPIUUID,
		Invocation:   validInvocation(),
		Dimensions:   map[string]any{},
	}
}

func mustTaxonomy(t *testing.T) *Taxonomy {
	t.Helper()
	tax, err := DefaultTaxonomy()
	if err != nil {
		t.Fatal(err)
	}
	return tax
}

func TestValidateBuildsTaxonomyMetadata(t *testing.T) {
	event, err := validate(mustTaxonomy(t), enabledFeatures, validCreation(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The keys Agent Manager's API events carry, and nothing else: the
	// feature has no dimensions, so the event is the endpoint invocation.
	want := map[string]any{
		"growth_action": FeatureCreateRESTAPI,
		"platform":      "WSO2 API Platform",
		"source":        "api",
		"outcome":       "success",
	}
	if !reflect.DeepEqual(event.metadata, want) {
		t.Errorf("metadata = %v, want %v", event.metadata, want)
	}
	inv := event.invocation
	if inv.Method != "POST" || inv.URI != testURI || inv.Status != 201 {
		t.Errorf("invocation = %+v, want POST %s 201 (verb upper-cased)", inv, testURI)
	}
	if inv.RequestTime.Location() != time.UTC || inv.RequestTime.Format(time.RFC3339) != "2026-11-06T02:45:00Z" {
		t.Errorf("request time = %s, want 2026-11-06T02:45:00Z in UTC", inv.RequestTime)
	}
	if !inv.ResponseTime.Equal(testRequestTime.Add(250 * time.Millisecond)) {
		t.Errorf("response time = %s", inv.ResponseTime)
	}
}

// Failed calls are reported too; the status tells them apart.
func TestValidateAcceptsFailedCalls(t *testing.T) {
	for _, status := range []int{400, 404, 409, 500, 503} {
		o := validCreation()
		o.Invocation.Status = status
		event, err := validate(mustTaxonomy(t), enabledFeatures, o, time.Now())
		if err != nil {
			t.Fatalf("status %d rejected: %v", status, err)
		}
		if event.invocation.Status != status {
			t.Errorf("status = %d, want %d", event.invocation.Status, status)
		}
		if event.metadata["outcome"] != "failure" {
			t.Errorf("outcome = %v for status %d, want failure", event.metadata["outcome"], status)
		}
	}
}

// outcome comes from the status alone: a value the occurrence carries is
// replaced, and a feature that does not declare outcome never gets one.
func TestOutcomeComesFromTheStatus(t *testing.T) {
	for status, want := range map[int]string{200: "success", 201: "success", 299: "success", 302: "failure", 400: "failure", 500: "failure"} {
		o := validCreation()
		o.Invocation.Status = status
		o.Dimensions["outcome"] = "success"
		event, err := validate(mustTaxonomy(t), enabledFeatures, o, time.Now())
		if err != nil {
			t.Fatalf("status %d rejected: %v", status, err)
		}
		if event.metadata["outcome"] != want {
			t.Errorf("outcome = %v for status %d, want %s", event.metadata["outcome"], status, want)
		}
	}
	if o := validCreation(); len(o.Dimensions) != 0 {
		t.Fatal("validCreation must not carry dimensions")
	}
	o := validCreation()
	before := len(o.Dimensions)
	if _, err := validate(mustTaxonomy(t), enabledFeatures, o, time.Now()); err != nil || len(o.Dimensions) != before {
		t.Errorf("validate changed the occurrence's own dimensions (err %v)", err)
	}
}

func TestValidateDefaultsTheResponseTime(t *testing.T) {
	o := validCreation()
	o.Invocation.ResponseTime = time.Time{}
	now := testRequestTime.Add(time.Second)
	event, err := validate(mustTaxonomy(t), enabledFeatures, o, now)
	if err != nil {
		t.Fatal(err)
	}
	if !event.invocation.ResponseTime.Equal(now) {
		t.Errorf("response time = %s, want now", event.invocation.ResponseTime)
	}
}

func TestTransactionIDIsStableAndOpaque(t *testing.T) {
	tax := mustTaxonomy(t)
	first, _ := validate(tax, enabledFeatures, validCreation(), time.Now())
	again, _ := validate(tax, enabledFeatures, validCreation(), time.Now())
	if first.transactionID != again.transactionID {
		t.Error("the same occurrence must always produce the same transaction_id, or a resend cannot be deduplicated")
	}
	if first.transactionID == testAPIUUID || !canonicalUUID(first.transactionID) {
		t.Errorf("transaction_id %q must be a derived UUID, not the API's own UUID", first.transactionID)
	}
	other := validCreation()
	other.OccurrenceID = "another-api"
	second, _ := validate(tax, enabledFeatures, other, time.Now())
	if second.transactionID == first.transactionID {
		t.Error("different occurrences must not share a transaction_id")
	}
}

func TestValidateRejects(t *testing.T) {
	const secretLooking = "Alice Smith <alice@example.com>"
	tests := map[string]func(o *Occurrence){
		"a feature that is not allowlisted": func(o *Occurrence) { o.Feature = "apip.manage-apis.delete-api" },
		"a feature not in the taxonomy":     func(o *Occurrence) { o.Feature = "apip.manage-apis.made-up" },
		"an undeclared dimension":           func(o *Occurrence) { o.Dimensions["api_name"] = secretLooking },
		"a missing organization":            func(o *Occurrence) { o.OrgUUID = "" },
		"a missing occurrence ID":           func(o *Occurrence) { o.OccurrenceID = "" },
		"a status that is not HTTP":         func(o *Occurrence) { o.Invocation.Status = 999 },
		"no status":                         func(o *Occurrence) { o.Invocation.Status = 0 },
		"an unsupported verb":               func(o *Occurrence) { o.Invocation.Method = "TRACE" },
		"a URI with a query":                func(o *Occurrence) { o.Invocation.URI = testURI + "?owner=alice" },
		"a URI with user info":              func(o *Occurrence) { o.Invocation.URI = "https://alice:pw@platform-api/api/v0.9/rest-apis" },
		"a URI with a fragment":             func(o *Occurrence) { o.Invocation.URI = testURI + "#alice" },
		"a relative URI":                    func(o *Occurrence) { o.Invocation.URI = "/api/v0.9/rest-apis" },
		"a non-http URI":                    func(o *Occurrence) { o.Invocation.URI = "file:///alice" },
		"a missing request time":            func(o *Occurrence) { o.Invocation.RequestTime = time.Time{} },
	}
	tax := mustTaxonomy(t)
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			o := validCreation()
			mutate(&o)
			_, err := validate(tax, enabledFeatures, o, time.Now())
			var rejection *RejectionError
			if !errors.As(err, &rejection) {
				t.Fatalf("validate accepted %s (err = %v)", name, err)
			}
			if strings.Contains(err.Error(), "alice") {
				t.Errorf("rejection reason echoes the rejected value: %v", err)
			}
		})
	}
}

// A feature that declares dimensions gets them type- and enum-checked; the
// embedded create-rest-api has none, so this uses a test taxonomy.
func TestValidateChecksDimensions(t *testing.T) {
	tax, err := ParseTaxonomy([]byte(`product:
  id: "apip"
  name: "WSO2 API Platform"
features:
  - code: "apip.manage-apis.deploy-api"
    name: "API Deployment"
    category: "manage-apis"
    tier: "core"
    tracking:
      event_name: "apip.manage-apis.deploy-api"
      event_category: "lifecycle"
      platforms: ["api"]
      dimensions:
        - name: "target"
          type: "enum"
          values: ["gateway", "portal"]
          required: true
        - name: "first_deployment"
          type: "boolean"
`))
	if err != nil {
		t.Fatal(err)
	}
	enabled := map[string]bool{"apip.manage-apis.deploy-api": true}
	occurrence := func(dims map[string]any) Occurrence {
		return Occurrence{Feature: "apip.manage-apis.deploy-api", OrgUUID: testOrgUUID, OccurrenceID: "x",
			Invocation: validInvocation(), Dimensions: dims}
	}

	event, err := validate(tax, enabled, occurrence(map[string]any{"target": "gateway", "first_deployment": true}), time.Now())
	if err != nil {
		t.Fatalf("valid dimensions were rejected: %v", err)
	}
	if event.metadata["target"] != "gateway" || event.metadata["first_deployment"] != true {
		t.Errorf("metadata = %v, want the dimensions in it", event.metadata)
	}
	if _, present := event.metadata["outcome"]; present {
		t.Errorf("metadata = %v; a feature that declares no outcome must not get one", event.metadata)
	}
	if _, err := validate(tax, enabled, occurrence(map[string]any{"target": "portal"}), time.Now()); err != nil {
		t.Errorf("an optional dimension must be omittable: %v", err)
	}
	for name, dims := range map[string]map[string]any{
		"an enum value outside the enum":  {"target": "Alice Smith <alice@example.com>"},
		"an enum value of the wrong type": {"target": 1},
		"a boolean of the wrong type":     {"target": "gateway", "first_deployment": "true"},
		"a missing required dimension":    {"first_deployment": true},
	} {
		_, err := validate(tax, enabled, occurrence(dims), time.Now())
		if err == nil {
			t.Errorf("validate accepted %s", name)
		} else if strings.Contains(err.Error(), "alice") {
			t.Errorf("rejection reason echoes the rejected value: %v", err)
		}
	}
}

func TestStringDimensionsAcceptOnlyCodes(t *testing.T) {
	tax, err := ParseTaxonomy([]byte(`product:
  id: "apip"
  name: "WSO2 API Platform"
features:
  - code: "apip.policies.rate-limiting"
    name: "Rate Limiting"
    category: "policies"
    tier: "core"
    tracking:
      event_name: "apip.policies.rate-limiting"
      event_category: "engagement"
      platforms: ["api"]
      dimensions:
        - name: "policy_name"
          type: "string"
          required: true
`))
	if err != nil {
		t.Fatal(err)
	}
	enabled := map[string]bool{"apip.policies.rate-limiting": true}
	occurrence := func(value any) Occurrence {
		return Occurrence{Feature: "apip.policies.rate-limiting", OrgUUID: testOrgUUID, OccurrenceID: "x",
			Invocation: validInvocation(), Dimensions: map[string]any{"policy_name": value}}
	}
	if _, err := validate(tax, enabled, occurrence("advanced-ratelimit"), time.Now()); err != nil {
		t.Errorf("a policy code was rejected: %v", err)
	}
	for _, bad := range []any{"My Policy", "alice@example.com", strings.Repeat("a", 65), "", 42} {
		if _, err := validate(tax, enabled, occurrence(bad), time.Now()); err == nil {
			t.Errorf("string dimension accepted %v", bad)
		}
	}
}

func TestCanonicalUUID(t *testing.T) {
	for value, want := range map[string]bool{
		"0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b":                true,
		"0193E7A2-4C5D-7E6F-8A9B-0C1D2E3F4A5B":                true,
		"urn:uuid:0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b":       false,
		"{0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b}":              false,
		"0193e7a24c5d7e6f8a9b0c1d2e3f4a5b":                    false,
		"alice@example.com":                                   false,
		"sa:acme:ci-bot:0193e7a2-4c5d-7e6f-8a9b-0c1d2e3f4a5b": false,
		"":             false,
		"113246589012": false,
	} {
		if got := canonicalUUID(value); got != want {
			t.Errorf("canonicalUUID(%q) = %v, want %v", value, got, want)
		}
	}
}

// A feature the taxonomy tracks only from the web (a browser) is never
// reported from the backend.
func TestValidateRejectsAWebOnlyFeature(t *testing.T) {
	tax, err := ParseTaxonomy([]byte(`product:
  id: "apip"
  name: "WSO2 API Platform"
features:
  - code: "apip.manage-apis.test-console"
    name: "Test Console"
    category: "manage-apis"
    tier: "core"
    tracking:
      event_name: "apip.manage-apis.test-console"
      event_category: "engagement"
      platforms: ["web"]
      dimensions: []
`))
	if err != nil {
		t.Fatal(err)
	}
	o := Occurrence{Feature: "apip.manage-apis.test-console", OrgUUID: testOrgUUID, OccurrenceID: "x", Invocation: validInvocation()}
	if _, err := validate(tax, map[string]bool{o.Feature: true}, o, time.Now()); err == nil {
		t.Fatal("a web-only feature was accepted for an API event")
	}
	if err := checkEnabledFeatures(tax, map[string]bool{o.Feature: true}); err == nil {
		t.Fatal("allowlisting a web-only feature must be a startup error")
	}
}

// The reported URI is the route template under a fixed host: neither the
// Host header nor X-Forwarded-Proto, both set by whoever sends the request,
// nor any path value or query reaches the event.
func TestInvocationFromRequest(t *testing.T) {
	started := time.Now()
	var got Invocation
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v0.9/rest-apis/{restApiId}/x", func(w http.ResponseWriter, r *http.Request) {
		var ok bool
		if got, ok = InvocationFromRequest(r, http.StatusCreated, started); !ok {
			t.Error("a request matched by a route pattern must be reportable")
		}
	})

	const wantURI = "https://platform-api/api/v0.9/rest-apis/{restApiId}/x"
	for name, host := range map[string]string{
		"a public hostname":      "platform-api.example.com",
		"an IP address and port": "10.0.0.7:9243",
		"an IPv6 address":        "[::1]:9243",
		"caller-chosen text":     "alice-smith.example.com",
		"a host that is no host": "alice smith@evil",
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v0.9/rest-apis/orders-api-by-alice/x?owner=alice", nil)
			req.Host = host
			req.Header.Set("X-Forwarded-Proto", "http")
			mux.ServeHTTP(httptest.NewRecorder(), req)

			if got.URI != wantURI {
				t.Errorf("URI = %q, want %q", got.URI, wantURI)
			}
			if got.Method != http.MethodPost || got.Status != http.StatusCreated || !got.RequestTime.Equal(started) {
				t.Errorf("invocation = %+v", got)
			}
			if _, err := got.normalize(time.Now()); err != nil {
				t.Errorf("a request-derived invocation failed validation: %v", err)
			}
		})
	}
}

// A request that matched no route pattern is not reported: its raw path may
// carry values, so it is never sent in place of a template.
func TestInvocationFromRequestWithoutARoutePattern(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v0.9/rest-apis/orders-api-by-alice/x", nil)
	if inv, ok := InvocationFromRequest(r, http.StatusCreated, time.Now()); ok || inv.URI != "" {
		t.Fatalf("InvocationFromRequest = %+v, %v; want nothing for a request with no route pattern", inv, ok)
	}
}
