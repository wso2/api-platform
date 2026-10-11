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

package analytics

import (
	"context"
	"reflect"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// TestOnFault_SatisfiesFaultPolicy pins the interface. The collector is appended to every
// API's fault chain by the controller, and an entry that does not satisfy this is dropped at
// chain-build time with a warning and then silently records nothing.
func TestOnFault_SatisfiesFaultPolicy(t *testing.T) {
	var _ policy.FaultPolicy = &AnalyticsPolicy{}
}

// TestOnFault_NeverEscalates is the contract's hardest requirement: the client is already
// receiving an error, so a fault policy must not turn one failure into two. Every input here
// is degenerate on purpose.
func TestOnFault_NeverEscalates(t *testing.T) {
	p := &AnalyticsPolicy{}
	cases := []struct {
		name   string
		ctx    *policy.FaultContext
		params map[string]interface{}
	}{
		{"nil context", nil, nil},
		{"nil context and params", nil, map[string]interface{}{}},
		// A bare FaultContext leaves the embedded *SharedContext nil. This is the exact
		// shape that panicked log-message during this feature's development, via a field
		// promoted from an embedded pointer.
		{"bare context", &policy.FaultContext{}, nil},
		{"no fault details", &policy.FaultContext{ResponseStatus: 503}, nil},
		{"malformed params", &policy.FaultContext{ResponseStatus: 500}, map[string]interface{}{
			"request_headers": "not-a-bool", "response_headers": 42,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("OnFault panicked on %s: %v", c.name, r)
				}
			}()
			// Only requirement: it returns without panicking and never asks to change the
			// response. A nil return is legal and is the common case.
			got := p.OnFault(context.Background(), c.ctx, c.params)
			if got == nil {
				return
			}
			if got.Body != nil || got.StatusCode != nil {
				t.Fatalf("the collector must observe only, got %+v", got)
			}
		})
	}
}

// TestOnFault_CarriesTheResolvedFailure covers what this policy contributes: the fields a
// status code cannot supply.
func TestOnFault_CarriesTheResolvedFailure(t *testing.T) {
	p := &AnalyticsPolicy{}
	faultCtx := &policy.FaultContext{
		SharedContext:  &policy.SharedContext{},
		ResponseStatus: 446,
		OriginalStatus: 200,
		Policy:         "word-count-guardrail",
		PolicyVersion:  "v1",
		PolicyPhase:    "response_body",
		Source:         "gateway",
		Fault: &policy.FaultDetails{
			Code:      "906201",
			Type:      policy.FaultTypeGuardrail,
			Direction: policy.DirectionResponse,
			Message:   "Response blocked by word count guardrail",
			Policy:    "word-count-guardrail",
			Guardrail: &policy.GuardrailDetails{
				InterveningGuardrail: "word-count-guardrail",
				Action:               "INTERVENED",
				ActionReason:         "word count out of range",
			},
		},
	}

	got := p.OnFault(context.Background(), faultCtx, nil)
	if got == nil {
		t.Fatal("a described fault must produce metadata")
	}
	md := got.AnalyticsMetadata

	for key, want := range map[string]any{
		FaultCodeMetadataKey:            "906201",
		FaultTypeMetadataKey:            policy.FaultTypeGuardrail,
		FaultDirectionMetadataKey:       policy.DirectionResponse,
		FaultMessageMetadataKey:         "Response blocked by word count guardrail",
		FaultPolicyMetadataKey:          "word-count-guardrail",
		FaultPolicyPhaseMetadataKey:     "response_body",
		FaultSourceMetadataKey:          "gateway",
		FaultStatusMetadataKey:          446,
		FaultOriginalStatusMetadataKey:  200,
		FaultGuardrailMetadataKey:       "word-count-guardrail",
		FaultGuardrailActionMetadataKey: "INTERVENED",
		FaultGuardrailReasonMetadataKey: "word count out of range",
	} {
		if md[key] != want {
			t.Errorf("%s = %v, want %v", key, md[key], want)
		}
	}
}

// TestOnFault_WithholdsTheBlockedContent is the security property, and the reason this file
// exists rather than the coverage being folded into the test above.
//
// An analytics event is forwarded to external publishers. For a response guardrail,
// FaultDetails.Description and GuardrailDetails.Assessments hold the very content the
// guardrail existed to stop from leaving — which is why every gateway renderer withholds
// Description from the response body. Publishing it to a third party as a side effect of
// blocking it would defeat the guardrail entirely, so no key in this policy may carry it.
func TestOnFault_WithholdsTheBlockedContent(t *testing.T) {
	const secret = "SSN 123-45-6789 leaked by the upstream"
	p := &AnalyticsPolicy{}
	faultCtx := &policy.FaultContext{
		SharedContext:  &policy.SharedContext{},
		ResponseStatus: 446,
		Fault: &policy.FaultDetails{
			Code:        "906101",
			Type:        policy.FaultTypeGuardrail,
			Message:     "Response blocked",
			Description: secret,
			Guardrail: &policy.GuardrailDetails{
				InterveningGuardrail: "pii-guardrail",
				Action:               "INTERVENED",
				ActionReason:         "PII detected",
				Assessments:          map[string]any{"matched": secret},
			},
		},
	}

	got := p.OnFault(context.Background(), faultCtx, nil)
	if got == nil {
		t.Fatal("expected metadata")
	}
	// Checked across every key and value rather than by naming the two fields, so a future
	// key that happens to carry the content is caught too.
	for key, value := range got.AnalyticsMetadata {
		if s, ok := value.(string); ok && strings.Contains(s, "123-45-6789") {
			t.Fatalf("key %q leaked the blocked content: %q", key, s)
		}
		if key == "x-wso2-fault-description" || key == "x-wso2-fault-guardrail-assessments" {
			t.Fatalf("key %q must not exist: the blocked content is never published", key)
		}
	}
	// The useful, non-sensitive part still arrives.
	if got.AnalyticsMetadata[FaultGuardrailReasonMetadataKey] != "PII detected" {
		t.Error("the guardrail's reason is not the blocked content and must be carried")
	}
}

// TestOnFault_CarriesTheResponseSideMetadata pins the reason OnFault delegates to
// OnResponseHeaders. For an upstream or router failure the collector is excluded from the
// response chain so it can run here instead, so if OnFault did not contribute this set it
// would be absent from exactly the events an operator most wants it on.
func TestOnFault_CarriesTheResponseSideMetadata(t *testing.T) {
	p := &AnalyticsPolicy{}
	faultCtx := &policy.FaultContext{
		SharedContext: &policy.SharedContext{
			AuthContext: &policy.AuthContext{AuthType: "jwt", Subject: "alice", Authenticated: true},
			Metadata:    map[string]interface{}{"tenant": "acme"},
		},
		ResponseStatus:  503,
		ResponseHeaders: policy.NewHeaders(map[string][]string{"Content-Type": {"application/json"}}),
	}

	md := p.OnFault(context.Background(), faultCtx, nil).AnalyticsMetadata

	if md[AuthTypeMetadataKey] != "jwt" {
		t.Errorf("auth type missing: a backend 5xx must still say who was calling, got %v", md[AuthTypeMetadataKey])
	}
	if md["response_content_type"] != "application/json" {
		t.Errorf("response content type missing, got %v", md["response_content_type"])
	}
	raw, ok := md[GenericMetadataKey].(string)
	if !ok || !strings.Contains(raw, "acme") {
		t.Errorf("the shared metadata bag must be serialised here too, got %v", md[GenericMetadataKey])
	}
}

// TestOnFault_DelegationLosesNothing is what makes delegating to OnResponseHeaders safe to
// rely on.
//
// It asserts the property rather than a fixed list of keys: everything the response hook
// produces for a given input must also appear, with the same value, in what OnFault produces
// for the equivalent input. So if someone adds a field to OnResponseHeaders, this holds
// OnFault to it automatically — no second implementation to remember to update, and no list
// in a test to keep in step either.
func TestOnFault_DelegationLosesNothing(t *testing.T) {
	p := &AnalyticsPolicy{}
	shared := &policy.SharedContext{
		AuthContext: &policy.AuthContext{
			AuthType: "jwt", Subject: "alice", Issuer: "https://idp.example.com",
			Authenticated: true, Authorized: true,
		},
		Metadata: map[string]interface{}{
			"tenant":                         "acme",
			BillingCustomerIDMetadataKey:     "cus_1",
			BillingSubscriptionIDMetadataKey: "sub_1",
			SubscriptionStatusMetadataKey:    "active",
			SubscriptionPlanNameMetadataKey:  "gold",
		},
		APIKind: policy.APIKindMCP,
	}
	headers := policy.NewHeaders(map[string][]string{
		"Content-Type":   {"application/json"},
		"mcp-session-id": {"sess-1"},
		"X-Upstream":     {"orders"},
	})
	// Header capture on, so the widest possible set is produced on both sides.
	params := map[string]interface{}{"request_headers": true, "response_headers": true}

	respOnly := p.OnResponseHeaders(context.Background(), &policy.ResponseHeaderContext{
		SharedContext:   shared,
		ResponseHeaders: headers,
		ResponseStatus:  503,
	}, params)
	mods, ok := respOnly.(policy.DownstreamResponseHeaderModifications)
	if !ok {
		t.Fatalf("expected DownstreamResponseHeaderModifications, got %T", respOnly)
	}
	if len(mods.AnalyticsMetadata) == 0 {
		t.Fatal("fixture produced nothing from the response hook, so the test proves nothing")
	}

	got := p.OnFault(context.Background(), &policy.FaultContext{
		SharedContext:   shared,
		ResponseHeaders: headers,
		ResponseStatus:  503,
	}, params)
	if got == nil {
		t.Fatal("OnFault returned nil")
	}

	for key, want := range mods.AnalyticsMetadata {
		// Captured headers are maps, so compare by value rather than with !=.
		if !reflect.DeepEqual(got.AnalyticsMetadata[key], want) {
			t.Errorf("key %q: OnFault has %v, OnResponseHeaders had %v — the fault path "+
				"must not lose response-side metadata", key, got.AnalyticsMetadata[key], want)
		}
	}
}

// TestOnFault_SynthesizedContextIsComplete guards the delegation's one real weakness.
//
// OnFault builds a ResponseHeaderContext out of the FaultContext by hand. If a field is
// added to ResponseHeaderContext and not copied, OnResponseHeaders would read a zero value
// on the fault path and quietly collect less — a difference no output assertion would catch,
// because both sides would simply be empty.
//
// So this compares the two structs by FIELD NAME: every field of ResponseHeaderContext must
// exist on FaultContext (which is what makes a complete copy possible at all) and must be
// non-zero in the synthesized context when its FaultContext source is non-zero.
func TestOnFault_SynthesizedContextIsComplete(t *testing.T) {
	faultType := reflect.TypeOf(policy.FaultContext{})
	respType := reflect.TypeOf(policy.ResponseHeaderContext{})

	for i := 0; i < respType.NumField(); i++ {
		name := respType.Field(i).Name
		if _, exists := faultType.FieldByName(name); !exists {
			t.Errorf("ResponseHeaderContext.%s has no counterpart on FaultContext, so the "+
				"synthesized context in OnFault cannot be complete — delegation must be "+
				"revisited", name)
		}
	}

	// Every source field populated, so any field OnFault forgot to copy shows up as a zero
	// in the synthesized context below.
	faultCtx := &policy.FaultContext{
		SharedContext:   &policy.SharedContext{APIName: "api"},
		RequestHeaders:  policy.NewHeaders(map[string][]string{"a": {"b"}}),
		RequestBody:     &policy.Body{Content: []byte("{}"), Present: true},
		RequestPath:     "/orders",
		RequestMethod:   "POST",
		ResponseHeaders: policy.NewHeaders(map[string][]string{"c": {"d"}}),
		ResponseStatus:  503,
		Downstream:      &policy.DownstreamContext{},
		Upstream:        &policy.UpstreamResponseContext{},
	}

	// Mirrors the construction inside OnFault. Kept here rather than exported so the
	// production path stays exactly as written.
	synthesized := &policy.ResponseHeaderContext{
		SharedContext:   faultCtx.SharedContext,
		RequestHeaders:  faultCtx.RequestHeaders,
		RequestBody:     faultCtx.RequestBody,
		RequestPath:     faultCtx.RequestPath,
		RequestMethod:   faultCtx.RequestMethod,
		ResponseHeaders: faultCtx.ResponseHeaders,
		ResponseStatus:  faultCtx.ResponseStatus,
		Downstream:      faultCtx.Downstream,
		Upstream:        faultCtx.Upstream,
	}

	v := reflect.ValueOf(*synthesized)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			t.Errorf("ResponseHeaderContext.%s is zero in the synthesized context: either "+
				"OnFault does not copy it, or this test's mirror is out of date — both are "+
				"defects worth failing on", respType.Field(i).Name)
		}
	}
}

// TestOnFault_RecordsTheJSONRPCCodeAsANumber pins the code's type, not just its presence.
// The engine converts analytics metadata through structpb, which rejects a pointer, and the
// classifier reads the code back only as a number — so a *int stored here reached the event
// as a string and was silently dropped. A nil Code means "derive from the status" and is left
// out rather than recorded as null.
func TestOnFault_RecordsTheJSONRPCCodeAsANumber(t *testing.T) {
	p := &AnalyticsPolicy{}
	fault := func(rpc *policy.JSONRPCError) *policy.FaultContext {
		return &policy.FaultContext{
			SharedContext:  &policy.SharedContext{},
			ResponseStatus: 400,
			Fault:          &policy.FaultDetails{Code: "900902", JSONRPC: rpc},
		}
	}

	code := -32602
	got := p.OnFault(context.Background(), fault(&policy.JSONRPCError{Code: &code}), nil)
	if got == nil {
		t.Fatal("expected metadata")
	}
	if v, ok := got.AnalyticsMetadata[FaultJSONRPCCodeMetadataKey].(int); !ok || v != code {
		t.Fatalf("%s = %#v, want the int %d", FaultJSONRPCCodeMetadataKey,
			got.AnalyticsMetadata[FaultJSONRPCCodeMetadataKey], code)
	}

	for name, rpc := range map[string]*policy.JSONRPCError{
		"nil code":    {ID: 7},
		"no JSON-RPC": nil,
	} {
		got := p.OnFault(context.Background(), fault(rpc), nil)
		if got == nil {
			continue
		}
		if v, present := got.AnalyticsMetadata[FaultJSONRPCCodeMetadataKey]; present {
			t.Errorf("%s: %s = %#v, want it omitted", name, FaultJSONRPCCodeMetadataKey, v)
		}
	}
}
