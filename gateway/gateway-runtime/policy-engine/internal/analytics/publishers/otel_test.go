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

package publishers

import (
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
)

// Attribute names are written as literals throughout this file: they are the
// wire contract, and sharing a constant with the publisher would let a typo
// satisfy its own assertion.

func testOTelConfig(endpoint string) config.OTelPublisherConfig {
	return config.OTelPublisherConfig{
		Endpoint:      endpoint,
		ServiceName:   "policy-engine",
		BatchSize:     100,
		FlushInterval: 50 * time.Millisecond,
		QueueCapacity: 100,
		OnQueueFull:   config.QueueDropNew,
		Timeout:       2 * time.Second,
	}
}

func restEvent() *dto.Event {
	api := &dto.ExtendedAPI{
		OrganizationID: "org-1",
		ProjectID:      "default",
		EnvironmentID:  "env-1",
		APIContext:     "/petstore",
	}
	api.APIID = "api-1"
	api.APIName = "PetStore"
	api.APIVersion = "v1.0"
	api.APIType = "RestApi"
	api.SubType = "RestApi"

	return &dto.Event{
		API:               api,
		Operation:         &dto.Operation{APIMethod: "GET", APIResourceTemplate: "/petstore/pet/{petId}"},
		Target:            &dto.Target{TargetResponseCode: 200, Destination: "backend:8080/pet/1", ResponseCodeDetail: "via_upstream", ResponseCacheHit: true},
		Application:       &dto.Application{ApplicationID: "app-1", ApplicationName: "Web", ApplicationOwner: "alice", KeyType: "PRODUCTION"},
		Subscription:      &dto.Subscription{BillingSubscriptionID: "sub-1", BillingCustomerID: "cust-1", Status: "ACTIVE", PlanName: "Gold"},
		Latencies:         &dto.Latencies{ResponseLatency: 12, BackendLatency: 9, RequestMediationLatency: 2, ResponseMediationLatency: 1, Duration: 12},
		MetaInfo:          &dto.MetaInfo{CorrelationID: "corr-1", GatewayType: "Envoy", RegionID: "us-east"},
		ProxyResponseCode: 200,
		RequestTimestamp:  time.Unix(1788320847, 0),
		UserAgentHeader:   "curl/8.7.1",
		UserName:          "alice",
		UserIP:            "10.0.0.5",
		Properties: map[string]interface{}{
			dto.PropKeyAuthUserID: "user-1",
			"requestSize":         uint64(12),
			// The concrete path, as analytics.go now supplies it: the route
			// template is "/petstore/pet/{petId}", this is one request against it.
			constants.RequestPathPropertyKey: "/petstore/pet/12345",
			"responseSize":                   uint64(463),
			"responseContentType":            "application/json",
		},
	}
}

// attrMap flattens a record's attributes for assertion. Each OTLP AnyValue keeps
// exactly one field set, so the concrete type is asserted alongside the value.
func attrMap(t *testing.T, record *otelLogRecord) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	for _, kv := range record.Attributes {
		if _, dup := out[kv.Key]; dup {
			t.Fatalf("attribute %q emitted twice", kv.Key)
		}
		switch {
		case kv.Value.StringValue != nil:
			out[kv.Key] = *kv.Value.StringValue
		case kv.Value.IntValue != nil:
			out[kv.Key] = *kv.Value.IntValue
		case kv.Value.DoubleValue != nil:
			out[kv.Key] = *kv.Value.DoubleValue
		case kv.Value.BoolValue != nil:
			out[kv.Key] = *kv.Value.BoolValue
		case kv.Value.ArrayValue != nil:
			// Header attributes are string arrays; flatten to []string so a test
			// can assert on them directly.
			items := make([]string, 0, len(kv.Value.ArrayValue.Values))
			for _, item := range kv.Value.ArrayValue.Values {
				if item.StringValue == nil {
					t.Fatalf("attribute %q has a non-string array element", kv.Key)
				}
				items = append(items, *item.StringValue)
			}
			out[kv.Key] = items
		default:
			t.Fatalf("attribute %q has no value set", kv.Key)
		}
	}
	return out
}

func TestBuildRecordRestAPI(t *testing.T) {
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	record := o.buildRecord(restEvent())

	if record.EventName != "wso2.api.transaction" {
		t.Errorf("eventName = %q, want wso2.api.transaction", record.EventName)
	}
	if record.SeverityNumber != 9 || record.SeverityText != "INFO" {
		t.Errorf("severity = %d/%q, want 9/INFO", record.SeverityNumber, record.SeverityText)
	}
	// Nanoseconds, as a JSON string per the proto3 mapping.
	if record.TimeUnixNano != "1788320847000000000" {
		t.Errorf("timeUnixNano = %q", record.TimeUnixNano)
	}

	got := attrMap(t, record)
	want := map[string]interface{}{
		"event.name":          "wso2.api.transaction",
		"http.request.method": "GET",
		"http.route":          "/petstore/pet/{petId}",
		// The template groups requests; the path identifies one. Never equal.
		"url.path":                           "/petstore/pet/12345",
		"http.response.status_code":          "200",
		"client.address":                     "10.0.0.5",
		"user_agent.original":                "curl/8.7.1",
		"http.request.body.size":             "12",
		"http.response.body.size":            "463",
		"user.id":                            "user-1",
		"user.name":                          "alice",
		"error.type":                         nil,
		"server.address":                     "backend",
		"server.port":                        "8080",
		"wso2.upstream.destination":          "backend:8080/pet/1",
		"wso2.upstream.response.status_code": "200",
		"wso2.upstream.response.detail":      "via_upstream",
		"wso2.cache.hit":                     true,
		"wso2.response.content_type":         "application/json",
		"wso2.api.id":                        "api-1",
		"wso2.api.name":                      "PetStore",
		"wso2.api.version":                   "v1.0",
		"wso2.api.context":                   "/petstore",
		"wso2.api.type":                      "RestApi",
		"wso2.api.subtype":                   "RestApi",
		"wso2.project.id":                    "default",
		"wso2.organization.id":               "org-1",
		"wso2.environment.id":                "env-1",
		"wso2.application.id":                "app-1",
		"wso2.application.name":              "Web",
		"wso2.application.owner":             "alice",
		"wso2.application.key_type":          "PRODUCTION",
		"wso2.subscription.id":               "sub-1",
		"wso2.subscription.customer.id":      "cust-1",
		"wso2.subscription.status":           "ACTIVE",
		"wso2.subscription.plan":             "Gold",
		"wso2.correlation.id":                "corr-1",
		"wso2.gateway.type":                  "Envoy",
		"wso2.region.id":                     "us-east",
		"wso2.latency.response_ms":           "12",
		"wso2.latency.backend_ms":            "9",
		"wso2.latency.request_mediation_ms":  "2",
		"wso2.latency.response_mediation_ms": "1",
		"wso2.latency.duration_ms":           "12",
	}
	for key, expected := range want {
		actual, present := got[key]
		if expected == nil {
			if present {
				t.Errorf("%s should be absent, got %v", key, actual)
			}
			continue
		}
		if !present {
			t.Errorf("%s missing", key)
			continue
		}
		if actual != expected {
			t.Errorf("%s = %v (%T), want %v (%T)", key, actual, actual, expected, expected)
		}
	}
}

// The resource template already carries the API context; concatenating the two
// produced "/ctx/ctx/resource" in an earlier revision.
func TestBuildRecordDoesNotDuplicateContext(t *testing.T) {
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(restEvent()))
	if got["http.route"] != "/petstore/pet/{petId}" {
		t.Errorf("http.route = %v, want /petstore/pet/{petId}", got["http.route"])
	}
}

// No resource template means no route to report — but the client still asked for
// something, and url.path is the only record of what. This is the request that
// most needs a path: nothing matched, so triage has the template nowhere else.
// url.path deliberately does NOT fall back to the API context, which would put a
// value in the attribute that was never the requested path.
func TestBuildRecordWithNoRouteKeepsConcretePath(t *testing.T) {
	event := restEvent()
	event.Operation.APIResourceTemplate = ""
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))
	if got["url.path"] != "/petstore/pet/12345" {
		t.Errorf("url.path = %v, want /petstore/pet/12345", got["url.path"])
	}
	if _, present := got["http.route"]; present {
		t.Error("http.route should be absent when there is no resource template")
	}
}

// Absent rather than guessed: with no path on the event there is nothing
// truthful to put in url.path, and the API context is not the requested path.
func TestBuildRecordOmitsURLPathWhenUnavailable(t *testing.T) {
	event := restEvent()
	delete(event.Properties, constants.RequestPathPropertyKey)
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))
	if actual, present := got["url.path"]; present {
		t.Errorf("url.path = %v; it must be absent, not fall back to the context", actual)
	}
}

func TestBuildRecordFaults(t *testing.T) {
	event := restEvent()
	event.ProxyResponseCode = 401
	event.ErrorType = "AUTH"
	event.Error = &dto.Error{ErrorCode: 900901, ErrorMessage: dto.AuthenticationFailure}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, expected := range map[string]interface{}{
		"error.type":                "AUTH",
		"wso2.error.code":           "900901",
		"wso2.error.message":        "AUTHENTICATION_FAILURE",
		"http.response.status_code": "401",
	} {
		if got[key] != expected {
			t.Errorf("%s = %v, want %v", key, got[key], expected)
		}
	}
}

// error.type carries the fault category derived by analytics.classifyFault — the
// same value existing Moesif consumers read from errorType.
func TestBuildRecordFaultCategoryInErrorType(t *testing.T) {
	event := restEvent()
	event.ErrorType = string(dto.FaultCategoryTargetConnectivity)
	event.Error = &dto.Error{ErrorCode: 504, ErrorMessage: dto.TargetConnectivityConnectionTimeout}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, expected := range map[string]interface{}{
		"error.type":         "TARGET_CONNECTIVITY",
		"wso2.error.code":    "504",
		"wso2.error.message": "CONNECTION_TIMEOUT",
	} {
		if got[key] != expected {
			t.Errorf("%s = %v, want %v", key, got[key], expected)
		}
	}
	// The category/event-category enums belong to the in-development fault flow.
	for _, absent := range []string{"wso2.event.category", "wso2.error.category", "wso2.error.sub_category"} {
		if _, present := got[absent]; present {
			t.Errorf("%s must not be emitted", absent)
		}
	}
}

// A request that was not a gateway fault carries no error attributes at all —
// absence is what lets a consumer filter faults.
func TestBuildRecordSuccessOmitsErrorAttributes(t *testing.T) {
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(restEvent()))

	for _, key := range []string{"error.type", "wso2.error.code", "wso2.error.message"} {
		if _, present := got[key]; present {
			t.Errorf("%s is present on a successful record", key)
		}
	}
}

func TestBuildRecordGenAI(t *testing.T) {
	event := restEvent()
	event.API.APIType = "LlmProxy"
	event.Operation.APIResourceTemplate = "/ai/chat/completions"
	event.Properties["aiMetadata"] = dto.AIMetadata{
		Model:         "claude-opus-4",
		VendorName:    "awsbedrock",
		VendorVersion: "2024-10-01",
		LLMCost:       0.0421,
	}
	event.Properties["aiTokenUsage"] = dto.AITokenUsage{PromptToken: 1841, CompletionToken: 210, TotalToken: 2051}
	event.Properties[constants.RequestModelPropertyKey] = "claude-opus-4-20260101"
	event.Properties[constants.GuardrailHitMetadataKey] = true
	event.Properties[constants.GuardrailNameMetadataKey] = "pii-masking"
	event.Properties["isEgress"] = true

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, expected := range map[string]interface{}{
		// awsbedrock is not a gen_ai.provider.name enum member; aws.bedrock is.
		"gen_ai.provider.name":               "aws.bedrock",
		"wso2.gen_ai.provider.template_name": "awsbedrock",
		// aitoken:modelid resolves to the response model.
		"gen_ai.response.model":            "claude-opus-4",
		"gen_ai.request.model":             "claude-opus-4-20260101",
		"gen_ai.operation.name":            "chat",
		"gen_ai.usage.input_tokens":        "1841",
		"gen_ai.usage.output_tokens":       "210",
		"wso2.gen_ai.usage.total_tokens":   "2051",
		"wso2.gen_ai.cost.total":           0.0421,
		"wso2.gen_ai.provider.api_version": "2024-10-01",
		"wso2.guardrail.hit":               true,
		"wso2.guardrail.name":              "pii-masking",
		"wso2.gen_ai.egress":               true,
	} {
		if got[key] != expected {
			t.Errorf("%s = %v (%T), want %v (%T)", key, got[key], got[key], expected, expected)
		}
	}
}

func TestBuildRecordMCP(t *testing.T) {
	event := restEvent()
	event.API.APIType = "Mcp"
	// Nested exactly as the analytics policy serializes it: clientInfo and
	// serverInfo are sub-objects, and both carry "name" and "version". A flat
	// fixture here is what let the nested-lookup bug pass for so long.
	event.Properties["mcpAnalytics"] = map[string]interface{}{
		"jsonRpcMethod":  "tools/call",
		"jsonRpcId":      "7",
		"sessionId":      "sess-1",
		"capability":     "TOOL",
		"capabilityName": "search_docs",
		"errorCode":      -32602,
		"isError":        true,
		"clientInfo": map[string]interface{}{
			"name":                     "claude-desktop",
			"version":                  "1.2.0",
			"requestedProtocolVersion": "2025-03-26",
		},
		"serverInfo": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"name":            "everything-server",
			"version":         "0.9.1",
		},
	}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, expected := range map[string]interface{}{
		"mcp.method.name":                            "tools/call",
		"jsonrpc.request.id":                         "7",
		"mcp.session.id":                             "sess-1",
		"gen_ai.tool.name":                           "search_docs",
		"rpc.response.status_code":                   "-32602",
		"mcp.protocol.version":                       "2025-06-18",
		"wso2.mcp.client.requested_protocol_version": "2025-03-26",
		"wso2.mcp.client.name":                       "claude-desktop",
		"wso2.mcp.client.version":                    "1.2.0",
		"wso2.mcp.server.name":                       "everything-server",
		"wso2.mcp.server.version":                    "0.9.1",
	} {
		if got[key] != expected {
			t.Errorf("%s = %v, want %v", key, got[key], expected)
		}
	}
	// The capability decides which attribute the name lands on.
	for _, absent := range []string{"mcp.resource.uri", "gen_ai.prompt.name"} {
		if _, present := got[absent]; present {
			t.Errorf("%s should be absent for a TOOL capability", absent)
		}
	}
	// event.ErrorType is empty here, so the MCP error supplies error.type.
	if got["error.type"] != "mcp_error" {
		t.Errorf("error.type = %v, want mcp_error", got["error.type"])
	}
}

// A gateway-level ErrorType must not be overwritten by the MCP fallback.
func TestBuildRecordMCPKeepsGatewayErrorType(t *testing.T) {
	event := restEvent()
	event.ErrorType = "THROTTLED"
	event.Properties["mcpAnalytics"] = map[string]interface{}{"isError": true}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))
	if got["error.type"] != "THROTTLED" {
		t.Errorf("error.type = %v, want THROTTLED", got["error.type"])
	}
}

// Each capability's target goes on its own attribute, read from its own source
// field: tools and prompts are named at params.name, a resource is addressed by
// URI at params.uri.
func TestMCPCapabilityRouting(t *testing.T) {
	cases := []struct {
		capability string
		mcp        map[string]interface{}
		wantKey    string
		wantValue  string
	}{
		{"TOOL", map[string]interface{}{"capabilityName": "search_docs"},
			"gen_ai.tool.name", "search_docs"},
		{"PROMPT", map[string]interface{}{"capabilityName": "summarize"},
			"gen_ai.prompt.name", "summarize"},
		{"RESOURCE", map[string]interface{}{"resourceUri": "file:///docs/readme.md"},
			"mcp.resource.uri", "file:///docs/readme.md"},
	}
	for _, tc := range cases {
		t.Run(tc.capability, func(t *testing.T) {
			event := restEvent()
			tc.mcp["capability"] = tc.capability
			event.Properties["mcpAnalytics"] = tc.mcp

			o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
			got := attrMap(t, o.buildRecord(event))
			if got[tc.wantKey] != tc.wantValue {
				t.Errorf("%s = %v, want %v", tc.wantKey, got[tc.wantKey], tc.wantValue)
			}
		})
	}
}

// mcp.resource.uri must come from resourceUri only. Reading capabilityName here
// is the bug this replaced: params.name does not exist on a resources/read
// request, so the attribute was always absent.
func TestMCPResourceURIIgnoresCapabilityName(t *testing.T) {
	event := restEvent()
	event.Properties["mcpAnalytics"] = map[string]interface{}{
		"capability":     "RESOURCE",
		"capabilityName": "should-not-be-used",
		"resourceUri":    "file:///docs/readme.md",
	}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	if got["mcp.resource.uri"] != "file:///docs/readme.md" {
		t.Errorf("mcp.resource.uri = %v, want the resourceUri value", got["mcp.resource.uri"])
	}
	// A resource has no name, so neither name attribute may appear.
	for _, key := range []string{"gen_ai.tool.name", "gen_ai.prompt.name"} {
		if _, present := got[key]; present {
			t.Errorf("%s is present on a resource read", key)
		}
	}
}

// End-to-end over the real HTTP path: the payload a collector receives must be
// valid OTLP-JSON with the routing scope and the configured headers.
func TestExportPayloadAndHeaders(t *testing.T) {
	var (
		mu     sync.Mutex
		body   []byte
		header http.Header
	)
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		body, _ = io.ReadAll(r.Body)
		header = r.Header.Clone()
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		close(done)
	}))
	defer server.Close()

	cfg := testOTelConfig(server.URL + "/v1/logs")
	cfg.ServiceVersion = "1.2.0"
	cfg.Headers = map[string]string{"X-Api-Key": "secret"}
	cfg.ResourceAttributes = map[string]string{"deployment.environment.name": "test"}

	publisher, err := NewOTel(&cfg)
	if err != nil {
		t.Fatalf("NewOTel: %v", err)
	}
	publisher.Publish(restEvent())

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("no export received")
	}
	if err := publisher.Close(context.Background()); err != nil {
		t.Errorf("Close: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if got := header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := header.Get("X-Api-Key"); got != "secret" {
		t.Errorf("configured header not sent, got %q", got)
	}

	var payload otelExportRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if len(payload.ResourceLogs) != 1 || len(payload.ResourceLogs[0].ScopeLogs) != 1 {
		t.Fatalf("unexpected payload shape: %s", body)
	}
	scope := payload.ResourceLogs[0].ScopeLogs[0]
	if scope.Scope.Name != "wso2.analytics" {
		t.Errorf("scope.name = %q, want wso2.analytics — the collector routes on this", scope.Scope.Name)
	}
	if len(scope.LogRecords) != 1 {
		t.Fatalf("want 1 record, got %d", len(scope.LogRecords))
	}

	resource := map[string]string{}
	for _, kv := range payload.ResourceLogs[0].Resource.Attributes {
		if kv.Value.StringValue != nil {
			resource[kv.Key] = *kv.Value.StringValue
		}
	}
	for key, want := range map[string]string{
		"service.name":                "policy-engine",
		"service.version":             "1.2.0",
		"deployment.environment.name": "test",
	} {
		if resource[key] != want {
			t.Errorf("resource %s = %q, want %q", key, resource[key], want)
		}
	}
}

// correlatedEvent returns the REST fixture with a distinguishable correlation id,
// so a queue's surviving record can be identified.
func correlatedEvent(id string) *dto.Event {
	event := restEvent()
	event.MetaInfo.CorrelationID = id
	return event
}

// newTestOTel builds a publisher with no worker goroutine, mirroring the derived
// fields NewOTel computes. NewOTel cannot be used where a test drives export or
// the queue directly, because it starts the worker that would drain them.
func newTestOTel(t *testing.T, cfg config.OTelPublisherConfig) *OTel {
	t.Helper()
	return &OTel{
		cfg:             cfg,
		client:          &http.Client{Timeout: cfg.Timeout},
		queue:           make(chan *otelLogRecord, cfg.QueueCapacity),
		stop:            make(chan struct{}),
		workerDone:      make(chan struct{}),
		dropOldest:      strings.EqualFold(cfg.OnQueueFull, config.QueueDropOldest),
		gzip:            strings.EqualFold(cfg.Compression, config.OTelCompressionGzip),
		retryAbortDepth: cfg.EffectiveRetryAbortDepth(),
	}
}

// newUndrainedOTel builds a publisher whose queue nothing consumes, so Publish
// sees it full.
func newUndrainedOTel(t *testing.T, capacity int, onQueueFull string) *OTel {
	t.Helper()
	cfg := testOTelConfig("http://127.0.0.1:1/v1/logs")
	cfg.QueueCapacity = capacity
	cfg.BatchSize = capacity
	cfg.OnQueueFull = onQueueFull
	return newTestOTel(t, cfg)
}

func (o *OTel) droppedCount() int {
	o.droppedMu.Lock()
	defer o.droppedMu.Unlock()
	return o.dropped
}

// A full queue must drop rather than block the ALS ingest path — under either
// policy, and Publish must never block.
func TestPublishDropsWhenQueueFull(t *testing.T) {
	for _, policy := range []string{config.QueueDropNew, config.QueueDropOldest} {
		t.Run(policy, func(t *testing.T) {
			o := newUndrainedOTel(t, 1, policy)
			for i := 0; i < 10; i++ {
				o.Publish(restEvent())
			}

			if dropped := o.droppedCount(); dropped != 9 {
				t.Errorf("dropped = %d, want 9 (queue holds 1)", dropped)
			}
			if len(o.queue) != 1 {
				t.Errorf("queue holds %d records, want 1", len(o.queue))
			}
		})
	}
}

// The policy decides *which* record survives, which is the whole point of the
// setting: drop_new keeps the oldest queued record, drop_oldest keeps the newest.
func TestPublishQueueFullKeepsPolicysRecord(t *testing.T) {
	cases := []struct {
		policy   string
		survivor string
	}{
		{config.QueueDropNew, "first"},
		{config.QueueDropOldest, "last"},
	}
	for _, tc := range cases {
		t.Run(tc.policy, func(t *testing.T) {
			o := newUndrainedOTel(t, 1, tc.policy)
			o.Publish(correlatedEvent("first"))
			o.Publish(correlatedEvent("middle"))
			o.Publish(correlatedEvent("last"))

			if len(o.queue) != 1 {
				t.Fatalf("queue holds %d records, want 1", len(o.queue))
			}
			got := attrMap(t, <-o.queue)["wso2.correlation.id"]
			if got != tc.survivor {
				t.Errorf("surviving record = %v, want %q", got, tc.survivor)
			}
		})
	}
}

// drop_oldest evicts exactly once per Publish. A retry loop would spin while
// producers keep the queue full, turning a non-blocking Publish into a blocking
// one; a single eviction bounds the work per call.
func TestPublishDropOldestEvictsOncePerCall(t *testing.T) {
	o := newUndrainedOTel(t, 2, config.QueueDropOldest)
	o.Publish(correlatedEvent("a"))
	o.Publish(correlatedEvent("b"))
	o.Publish(correlatedEvent("c")) // evicts "a", enqueues "c"

	if len(o.queue) != 2 {
		t.Fatalf("queue holds %d records, want 2", len(o.queue))
	}
	var got []interface{}
	for len(o.queue) > 0 {
		got = append(got, attrMap(t, <-o.queue)["wso2.correlation.id"])
	}
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Errorf("queue = %v, want [b c]", got)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	cfg := testOTelConfig("http://127.0.0.1:1/v1/logs")
	publisher, err := NewOTel(&cfg)
	if err != nil {
		t.Fatalf("NewOTel: %v", err)
	}
	if err := publisher.Close(context.Background()); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := publisher.Close(context.Background()); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestNewOTelNilConfig(t *testing.T) {
	publisher, err := NewOTel(nil)
	if err == nil {
		t.Error("NewOTel(nil) should return an error")
	}
	if publisher != nil {
		t.Error("NewOTel(nil) should return a nil publisher")
	}
}

// --- TLS -------------------------------------------------------------------

// writeSelfSignedPair writes a throwaway self-signed certificate and its key,
// usable both as a CA bundle and as an mTLS client pair.
func writeSelfSignedPair(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "otel-publisher-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)
	return certPath, keyPath
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestBuildOTelTLSConfigDefaults(t *testing.T) {
	got, err := buildOTelTLSConfig(config.OTelTLSConfig{})
	if err != nil {
		t.Fatalf("buildOTelTLSConfig: %v", err)
	}
	if got.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want TLS 1.2", got.MinVersion)
	}
	// The hybrid must be offered first, with classical curves retained after it
	// so an endpoint without the hybrid still handshakes.
	if got.CurvePreferences[0] != tls.X25519MLKEM768 {
		t.Errorf("CurvePreferences[0] = %v, want X25519MLKEM768", got.CurvePreferences[0])
	}
	if len(got.CurvePreferences) < 2 {
		t.Error("no classical curve retained after the hybrid")
	}
	if got.InsecureSkipVerify {
		t.Error("InsecureSkipVerify must default to false")
	}
	if got.RootCAs != nil || len(got.Certificates) != 0 {
		t.Error("no TLS material configured, yet RootCAs/Certificates are set")
	}
}

func TestBuildOTelTLSConfigCAFile(t *testing.T) {
	certPath, _ := writeSelfSignedPair(t)
	got, err := buildOTelTLSConfig(config.OTelTLSConfig{CAFile: certPath})
	if err != nil {
		t.Fatalf("buildOTelTLSConfig: %v", err)
	}
	if got.RootCAs == nil {
		t.Error("RootCAs not populated from ca_file")
	}
}

func TestBuildOTelTLSConfigMTLSPair(t *testing.T) {
	certPath, keyPath := writeSelfSignedPair(t)
	got, err := buildOTelTLSConfig(config.OTelTLSConfig{CertFile: certPath, KeyFile: keyPath})
	if err != nil {
		t.Fatalf("buildOTelTLSConfig: %v", err)
	}
	if len(got.Certificates) != 1 {
		t.Errorf("Certificates = %d, want 1", len(got.Certificates))
	}
}

func TestBuildOTelTLSConfigRejections(t *testing.T) {
	certPath, keyPath := writeSelfSignedPair(t)
	garbage := filepath.Join(t.TempDir(), "garbage.pem")
	if err := os.WriteFile(garbage, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write garbage: %v", err)
	}

	cases := map[string]config.OTelTLSConfig{
		"missing ca file":  {CAFile: filepath.Join(t.TempDir(), "absent.pem")},
		"unusable ca file": {CAFile: garbage},
		"cert without key": {CertFile: certPath},
		"key without cert": {KeyFile: keyPath},
		"mismatched pair":  {CertFile: keyPath, KeyFile: certPath},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := buildOTelTLSConfig(cfg); err == nil {
				t.Error("expected an error, got nil")
			}
		})
	}
}

// TestExportOverTLSWithCAFile proves the ca_file path actually works against a
// server whose certificate the system trust store does not know.
func TestExportOverTLSWithCAFile(t *testing.T) {
	done := make(chan struct{})
	var once sync.Once
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		once.Do(func() { close(done) })
	}))
	defer server.Close()

	caPath := filepath.Join(t.TempDir(), "server-ca.pem")
	writePEM(t, caPath, "CERTIFICATE", server.Certificate().Raw)

	cfg := testOTelConfig(server.URL + "/v1/logs")
	cfg.TLS = config.OTelTLSConfig{CAFile: caPath}

	publisher, err := NewOTel(&cfg)
	if err != nil {
		t.Fatalf("NewOTel: %v", err)
	}
	defer publisher.Close(context.Background())
	publisher.Publish(restEvent())

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("no export received over TLS")
	}
}

// TestNewOTelFailsClosedOnBadTLSMaterial: a publisher that can never reach its
// endpoint must not be constructed.
func TestNewOTelFailsClosedOnBadTLSMaterial(t *testing.T) {
	cfg := testOTelConfig("https://collector.invalid/v1/logs")
	cfg.TLS = config.OTelTLSConfig{CAFile: filepath.Join(t.TempDir(), "absent.pem")}
	if _, err := NewOTel(&cfg); err == nil {
		t.Error("expected NewOTel to fail on an unreadable ca_file")
	}
}

// --- retry and export-failure handling -------------------------------------

// scriptedEndpoint serves the given statuses in order, repeating the last one
// once the script is exhausted, and records every request it received.
type scriptedEndpoint struct {
	mu       sync.Mutex
	statuses []int
	requests []*http.Request
	bodies   [][]byte
	headers  http.Header
}

func newScriptedEndpoint(t *testing.T, statuses ...int) (*scriptedEndpoint, string) {
	t.Helper()
	e := &scriptedEndpoint{statuses: statuses, headers: http.Header{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		e.mu.Lock()
		attempt := len(e.requests)
		e.requests = append(e.requests, r)
		e.bodies = append(e.bodies, body)
		status := e.statuses[len(e.statuses)-1]
		if attempt < len(e.statuses) {
			status = e.statuses[attempt]
		}
		e.mu.Unlock()

		for k, vs := range e.headers {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return e, server.URL + "/v1/logs"
}

func (e *scriptedEndpoint) attempts() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.requests)
}

// retryConfig enables retries with a short backoff so tests stay fast.
func retryConfig(endpoint string, maxRetries int) config.OTelPublisherConfig {
	cfg := testOTelConfig(endpoint)
	cfg.MaxRetries = maxRetries
	cfg.RetryBackoff = 5 * time.Millisecond
	return cfg
}

func (o *OTel) exportOne(t *testing.T) {
	t.Helper()
	o.exportN(t, 1)
}

// exportN exports a batch of n identical records. Any test asserting a rejected
// count needs this: the endpoint cannot reject more records than were sent, so a
// single-record batch can only ever exercise a rejected count of 1.
func (o *OTel) exportN(t *testing.T, n int) {
	t.Helper()
	batch := make([]*otelLogRecord, 0, n)
	for i := 0; i < n; i++ {
		batch = append(batch, o.buildRecord(restEvent()))
	}
	o.export(batch)
}

// A 5xx is transient: retry until it clears, and lose nothing when it does.
func TestExportRetriesOn5xxThenSucceeds(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 503, 500, 200)
	o := newTestOTel(t, retryConfig(url, 3))
	o.exportOne(t)

	if got := endpoint.attempts(); got != 3 {
		t.Errorf("attempts = %d, want 3 (two failures then success)", got)
	}
	if dropped := o.droppedCount(); dropped != 0 {
		t.Errorf("dropped = %d, want 0: the batch was delivered", dropped)
	}
}

// A 4xx other than 429 means the payload's shape was rejected. Retrying cannot
// fix that, and would multiply a permanent failure by the retry budget.
func TestExportDoesNotRetryPermanentRejection(t *testing.T) {
	for _, status := range []int{400, 401, 404, 422} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			endpoint, url := newScriptedEndpoint(t, status)
			o := newTestOTel(t, retryConfig(url, 3))
			o.exportOne(t)

			if got := endpoint.attempts(); got != 1 {
				t.Errorf("attempts = %d, want 1 (no retry on %d)", got, status)
			}
			if dropped := o.droppedCount(); dropped != 1 {
				t.Errorf("dropped = %d, want 1", dropped)
			}
		})
	}
}

// 429 is retryable, and the endpoint's Retry-After replaces our own backoff
// rather than adding to it.
func TestExportHonoursRetryAfterOn429(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 429, 200)
	endpoint.headers.Set("Retry-After", "1")

	cfg := retryConfig(url, 3)
	cfg.RetryBackoff = time.Millisecond // far shorter than Retry-After
	o := newTestOTel(t, cfg)

	start := time.Now()
	o.exportOne(t)
	elapsed := time.Since(start)

	if got := endpoint.attempts(); got != 2 {
		t.Errorf("attempts = %d, want 2", got)
	}
	// The wait must come from Retry-After, not the 1ms backoff.
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %s, want >= ~1s from Retry-After", elapsed)
	}
	if dropped := o.droppedCount(); dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
}

// Exhausting the budget drops the batch exactly once, counting every record.
func TestExportDropsBatchAfterBudgetExhausted(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 503)
	o := newTestOTel(t, retryConfig(url, 2))

	batch := []*otelLogRecord{
		o.buildRecord(restEvent()), o.buildRecord(restEvent()), o.buildRecord(restEvent()),
	}
	o.export(batch)

	if got := endpoint.attempts(); got != 3 {
		t.Errorf("attempts = %d, want 3 (initial + 2 retries)", got)
	}
	if dropped := o.droppedCount(); dropped != 3 {
		t.Errorf("dropped = %d, want 3 (every record in the batch)", dropped)
	}
}

// One worker exports, so nothing drains the queue while a batch retries. Past
// the abort depth, retrying to save this batch costs more newer records than it
// rescues — so it must abandon its budget and return to draining.
func TestExportAbandonsRetriesUnderQueuePressure(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 503)
	cfg := retryConfig(url, 5)
	cfg.RetryBackoff = time.Millisecond
	cfg.QueueCapacity = 4
	cfg.RetryAbortQueueRatio = 0.5 // abort depth 2
	o := newTestOTel(t, cfg)

	if o.retryAbortDepth != 2 {
		t.Fatalf("retryAbortDepth = %d, want 2", o.retryAbortDepth)
	}
	// Fill past the abort depth; nothing drains it.
	for i := 0; i < 3; i++ {
		o.queue <- o.buildRecord(restEvent())
	}

	o.exportOne(t)

	// The first attempt happens unconditionally; the depth check runs before the
	// second, so exactly one attempt is made instead of the budgeted six.
	if got := endpoint.attempts(); got != 1 {
		t.Errorf("attempts = %d, want 1 (abandoned before the first retry)", got)
	}
	if dropped := o.droppedCount(); dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
}

// A ratio of 0 disables the check, so every batch gets its full budget.
func TestExportZeroAbortRatioUsesFullBudget(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 503)
	cfg := retryConfig(url, 2)
	cfg.QueueCapacity = 2
	cfg.RetryAbortQueueRatio = 0
	o := newTestOTel(t, cfg)

	if o.retryAbortDepth != 0 {
		t.Fatalf("retryAbortDepth = %d, want 0 (check disabled)", o.retryAbortDepth)
	}
	o.queue <- o.buildRecord(restEvent())
	o.queue <- o.buildRecord(restEvent())

	o.exportOne(t)
	if got := endpoint.attempts(); got != 3 {
		t.Errorf("attempts = %d, want 3 despite a full queue", got)
	}
}

// A 2xx carrying partialSuccess is not a clean export: those records are gone,
// and must be counted rather than silently discarded.
func TestExportCountsPartialSuccessRejections(t *testing.T) {
	cases := map[string]string{
		// Proto3 JSON encodes int64 as a string; some receivers emit a number.
		"int64 as string": `{"partialSuccess":{"rejectedLogRecords":"2","errorMessage":"bad attribute"}}`,
		"bare number":     `{"partialSuccess":{"rejectedLogRecords":2,"errorMessage":"bad attribute"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			o := newTestOTel(t, retryConfig(server.URL+"/v1/logs", 3))
			before := scrapeMetrics(t)
			o.exportN(t, 5)

			if dropped := o.droppedCount(); dropped != 2 {
				t.Errorf("dropped = %d, want 2 from partialSuccess", dropped)
			}
			// The request succeeded, so 3 of the 5 were published — not all 5.
			// Counting the whole batch would report 5 published and 2 dropped for
			// 5 events that existed.
			published := seriesKey("policy_engine_analytics_published_total")
			if got := delta(t, before, scrapeMetrics(t), published); got != 3 {
				t.Errorf("published delta = %v, want 3 (5 sent, 2 rejected)", got)
			}
		})
	}
}

// An endpoint claiming more rejections than were sent is claiming something
// impossible. It has to be clamped rather than trusted: the published tally is
// the batch size minus this count, and a Prometheus counter panics on a negative
// Add — which on the export worker would take the process down.
func TestExportClampsOverReportedRejections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partialSuccess":{"rejectedLogRecords":"99999"}}`))
	}))
	defer server.Close()

	o := newTestOTel(t, retryConfig(server.URL+"/v1/logs", 0))
	before := scrapeMetrics(t)
	o.exportN(t, 2) // must not panic

	if dropped := o.droppedCount(); dropped != 2 {
		t.Errorf("dropped = %d, want 2 clamped to the batch size", dropped)
	}
	published := seriesKey("policy_engine_analytics_published_total")
	if got := delta(t, before, scrapeMetrics(t), published); got != 0 {
		t.Errorf("published delta = %v, want 0 — the whole batch was rejected", got)
	}
}

// The ordinary success shapes must not be read as rejections.
func TestExportCleanSuccessBodiesCountNoDrops(t *testing.T) {
	bodies := map[string]string{
		"empty":                  ``,
		"empty object":           `{}`,
		"empty partial success":  `{"partialSuccess":{}}`,
		"explicit zero rejected": `{"partialSuccess":{"rejectedLogRecords":"0"}}`,
		"not json":               `OK`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			o := newTestOTel(t, retryConfig(server.URL+"/v1/logs", 0))
			o.exportOne(t)
			if dropped := o.droppedCount(); dropped != 0 {
				t.Errorf("dropped = %d, want 0", dropped)
			}
		})
	}
}

// gzip must set Content-Encoding and produce a body the endpoint can inflate
// back into the same OTLP payload.
func TestExportGzipCompression(t *testing.T) {
	type received struct {
		encoding string
		payload  otelExportRequest
	}
	got := make(chan received, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("body is not gzip: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer zr.Close()
		raw, err := io.ReadAll(zr)
		if err != nil {
			t.Errorf("inflate: %v", err)
		}
		var payload otelExportRequest
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Errorf("inflated body is not OTLP JSON: %v", err)
		}
		got <- received{encoding: r.Header.Get("Content-Encoding"), payload: payload}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := retryConfig(server.URL+"/v1/logs", 0)
	cfg.Compression = config.OTelCompressionGzip
	o := newTestOTel(t, cfg)
	o.exportOne(t)

	select {
	case r := <-got:
		if r.encoding != "gzip" {
			t.Errorf("Content-Encoding = %q, want gzip", r.encoding)
		}
		if len(r.payload.ResourceLogs) != 1 || len(r.payload.ResourceLogs[0].ScopeLogs[0].LogRecords) != 1 {
			t.Error("inflated payload did not carry the record")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no export received")
	}
}

// Uncompressed is the default, and must not claim an encoding it did not apply.
func TestExportUncompressedByDefault(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 200)
	o := newTestOTel(t, retryConfig(url, 0))
	o.exportOne(t)

	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if enc := endpoint.requests[0].Header.Get("Content-Encoding"); enc != "" {
		t.Errorf("Content-Encoding = %q, want empty", enc)
	}
	if !json.Valid(endpoint.bodies[0]) {
		t.Error("body is not plain JSON")
	}
}

// Shutdown must not wait out the remaining backoff: a retrying batch has to
// notice the stop signal instead of holding shutdown open.
func TestExportStopsRetryingOnShutdown(t *testing.T) {
	endpoint, url := newScriptedEndpoint(t, 503)
	cfg := retryConfig(url, 100)
	cfg.RetryBackoff = 30 * time.Second // long enough that waiting it out would fail the test
	o := newTestOTel(t, cfg)

	done := make(chan struct{})
	go func() {
		o.exportOne(t)
		close(done)
	}()

	// Wait for the first attempt to fail, then signal shutdown mid-backoff.
	deadline := time.Now().Add(2 * time.Second)
	for endpoint.attempts() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	close(o.stop)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("export kept retrying through shutdown")
	}
	if got := endpoint.attempts(); got != 1 {
		t.Errorf("attempts = %d, want 1 (stopped during the first backoff)", got)
	}
}

// --- self-observability ----------------------------------------------------

// The publishers package shares one process and one registry across tests, so
// counters accumulate. Every assertion below is therefore on a delta.

// scrapeMetrics renders the registry exactly as the policy-engine's /metrics
// endpoint does, so these assertions also prove the series actually reach a
// scrape — a metric that was never registered increments happily and is simply
// absent from the endpoint.
func scrapeMetrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	promhttp.HandlerFor(metrics.Init(), promhttp.HandlerOpts{}).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics returned %d", rec.Code)
	}
	return rec.Body.String()
}

// seriesKey builds the exposition-format identifier for one series. Prometheus
// renders label pairs sorted by label NAME, not in the order the vec declared
// them, so the pairs are sorted here to match: code="..." precedes
// publisher="otel", while reason="..." follows it.
func seriesKey(name string, labels ...string) string {
	pairs := append([]string{`publisher="otel"`}, labels...)
	sort.Strings(pairs)
	return name + "{" + strings.Join(pairs, ",") + "}"
}

// metricValue reads a series out of a scrape, reporting whether it was present
// at all — "absent" and "zero" are different answers and tests need both.
func metricValue(t *testing.T, scrape, key string) (float64, bool) {
	t.Helper()
	for _, line := range strings.Split(scrape, "\n") {
		rest, ok := strings.CutPrefix(line, key+" ")
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			t.Fatalf("unparseable value for %s: %q", key, rest)
		}
		return value, true
	}
	return 0, false
}

// delta reports how much a series moved, requiring it to exist afterwards.
func delta(t *testing.T, before string, after string, key string) float64 {
	t.Helper()
	old, _ := metricValue(t, before, key)
	current, ok := metricValue(t, after, key)
	if !ok {
		t.Fatalf("series %s is absent from the scrape", key)
	}
	return current - old
}

// A successful export must be counted, and its duration observed.
func TestMetricsSuccessfulExport(t *testing.T) {
	_, url := newScriptedEndpoint(t, 200)
	o := newTestOTel(t, retryConfig(url, 0))

	before := scrapeMetrics(t)
	o.export([]*otelLogRecord{o.buildRecord(restEvent()), o.buildRecord(restEvent())})
	after := scrapeMetrics(t)

	published := seriesKey("policy_engine_analytics_published_total")
	if got := delta(t, before, after, published); got != 2 {
		t.Errorf("published delta = %v, want 2", got)
	}
	duration := seriesKey("policy_engine_analytics_export_duration_seconds_count")
	if got := delta(t, before, after, duration); got != 1 {
		t.Errorf("duration observation delta = %v, want 1", got)
	}
}

// Each failure mode must land on its own reason, so an operator can tell them
// apart: a slow endpoint (backpressure) is a different problem from a broken one
// (send_failed) or a full queue.
func TestMetricsDropReasons(t *testing.T) {
	t.Run("queue_full", func(t *testing.T) {
		key := seriesKey("policy_engine_analytics_dropped_total", `reason="`+dropReasonQueueFull+`"`)
		before := scrapeMetrics(t)

		o := newUndrainedOTel(t, 1, config.QueueDropNew)
		for i := 0; i < 3; i++ {
			o.Publish(restEvent())
		}

		if got := delta(t, before, scrapeMetrics(t), key); got != 2 {
			t.Errorf("queue_full delta = %v, want 2", got)
		}
	})

	t.Run("send_failed", func(t *testing.T) {
		key := seriesKey("policy_engine_analytics_dropped_total", `reason="`+dropReasonSendFailed+`"`)
		before := scrapeMetrics(t)

		_, url := newScriptedEndpoint(t, 503)
		o := newTestOTel(t, retryConfig(url, 1))
		o.exportOne(t)

		if got := delta(t, before, scrapeMetrics(t), key); got != 1 {
			t.Errorf("send_failed delta = %v, want 1", got)
		}
	})

	t.Run("backpressure", func(t *testing.T) {
		key := seriesKey("policy_engine_analytics_dropped_total", `reason="`+dropReasonBackpressure+`"`)
		before := scrapeMetrics(t)

		_, url := newScriptedEndpoint(t, 503)
		cfg := retryConfig(url, 5)
		cfg.QueueCapacity = 2
		cfg.RetryAbortQueueRatio = 0.5
		o := newTestOTel(t, cfg)
		o.queue <- o.buildRecord(restEvent())
		o.exportOne(t)

		if got := delta(t, before, scrapeMetrics(t), key); got != 1 {
			t.Errorf("backpressure delta = %v, want 1", got)
		}
	})

	t.Run("rejected", func(t *testing.T) {
		key := seriesKey("policy_engine_analytics_dropped_total", `reason="`+dropReasonRejected+`"`)
		before := scrapeMetrics(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"partialSuccess":{"rejectedLogRecords":"3"}}`))
		}))
		defer server.Close()

		o := newTestOTel(t, retryConfig(server.URL+"/v1/logs", 0))
		o.exportN(t, 5)

		if got := delta(t, before, scrapeMetrics(t), key); got != 3 {
			t.Errorf("rejected delta = %v, want 3", got)
		}
	})
}

// The error code distinguishes an unreachable endpoint from one that answered
// with a status, which are diagnosed differently.
func TestMetricsExportErrorCodes(t *testing.T) {
	t.Run("transport", func(t *testing.T) {
		key := seriesKey("policy_engine_analytics_export_errors_total", `code="`+errCodeTransport+`"`)
		before := scrapeMetrics(t)

		o := newTestOTel(t, retryConfig("http://127.0.0.1:1/v1/logs", 0))
		o.exportOne(t)

		if got := delta(t, before, scrapeMetrics(t), key); got != 1 {
			t.Errorf("transport delta = %v, want 1", got)
		}
	})

	t.Run("http status", func(t *testing.T) {
		key := seriesKey("policy_engine_analytics_export_errors_total", `code="503"`)
		before := scrapeMetrics(t)

		_, url := newScriptedEndpoint(t, 503)
		o := newTestOTel(t, retryConfig(url, 1))
		o.exportOne(t)

		// Both the initial attempt and the retry answered 503.
		if got := delta(t, before, scrapeMetrics(t), key); got != 2 {
			t.Errorf("503 delta = %v, want 2", got)
		}
	})
}

// Depth and capacity are published as a pair so an alert can express "the queue
// is 80% full" rather than an absolute depth that means nothing without it.
func TestMetricsQueueDepthAndCapacity(t *testing.T) {
	_, url := newScriptedEndpoint(t, 200)
	cfg := retryConfig(url, 0)
	cfg.QueueCapacity = 8
	cfg.BatchSize = 8
	cfg.FlushInterval = time.Hour // park the worker so the queue holds

	publisher, err := NewOTel(&cfg)
	if err != nil {
		t.Fatalf("NewOTel: %v", err)
	}
	defer publisher.Close(context.Background())

	capacity, ok := metricValue(t, scrapeMetrics(t), seriesKey("policy_engine_analytics_queue_capacity"))
	if !ok || capacity != 8 {
		t.Errorf("capacity = %v (present=%v), want 8", capacity, ok)
	}

	for i := 0; i < 3; i++ {
		publisher.Publish(restEvent())
	}
	// The worker consumes from the channel as records arrive, so depth is
	// whatever is still buffered — assert it never exceeds capacity and was
	// published at all rather than pinning an inherently racy exact value.
	depth, ok := metricValue(t, scrapeMetrics(t), seriesKey("policy_engine_analytics_queue_depth"))
	if !ok || depth < 0 || depth > 8 {
		t.Errorf("depth = %v (present=%v), want within 0..8", depth, ok)
	}
}

// A labelled counter is absent from a scrape until first incremented, so a
// healthy gateway would show "No data" instead of 0 for the one series that
// makes silent analytics loss visible.
func TestMetricsPreInitializedAtZero(t *testing.T) {
	_, url := newScriptedEndpoint(t, 200)
	cfg := retryConfig(url, 0)
	publisher, err := NewOTel(&cfg)
	if err != nil {
		t.Fatalf("NewOTel: %v", err)
	}
	defer publisher.Close(context.Background())

	scrape := scrapeMetrics(t)
	keys := []string{
		seriesKey("policy_engine_analytics_published_total"),
		seriesKey("policy_engine_analytics_queue_capacity"),
		seriesKey("policy_engine_analytics_queue_depth"),
		seriesKey("policy_engine_analytics_export_errors_total", `code="`+errCodeTransport+`"`),
	}
	for _, reason := range []string{
		dropReasonQueueFull, dropReasonSendFailed, dropReasonBackpressure,
		dropReasonRejected, dropReasonSerializeFailed,
	} {
		keys = append(keys, seriesKey("policy_engine_analytics_dropped_total", `reason="`+reason+`"`))
	}
	for _, key := range keys {
		if _, ok := metricValue(t, scrape, key); !ok {
			t.Errorf("%s is absent from the scrape; a healthy gateway must report a value, not No data", key)
		}
	}
}

// --- header attributes -----------------------------------------------------

// headerEvent returns the REST fixture with the serialized header properties the
// analytics-header-filter policy produces.
func headerEvent(t *testing.T, request, response map[string]string) *dto.Event {
	t.Helper()
	event := restEvent()
	for property, headers := range map[string]map[string]string{
		dto.PropKeyRequestHeaders:  request,
		dto.PropKeyResponseHeaders: response,
	} {
		if headers == nil {
			continue
		}
		serialized, err := json.Marshal(headers)
		if err != nil {
			t.Fatalf("marshal headers: %v", err)
		}
		event.Properties[property] = string(serialized)
	}
	return event
}

// One attribute per header, name lowercased into the key, value a string array —
// the shape the HTTP conventions require.
func TestBuildRecordHeaderAttributes(t *testing.T) {
	event := headerEvent(t,
		map[string]string{"Content-Type": "application/json", "X-Tenant": "acme"},
		map[string]string{"Cache-Control": "no-store"})

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, want := range map[string][]string{
		"http.request.header.content-type":   {"application/json"},
		"http.request.header.x-tenant":       {"acme"},
		"http.response.header.cache-control": {"no-store"},
	} {
		values, ok := got[key].([]string)
		if !ok {
			t.Errorf("%s = %#v, want a string array", key, got[key])
			continue
		}
		if len(values) != len(want) || values[0] != want[0] {
			t.Errorf("%s = %v, want %v", key, values, want)
		}
	}
}

// No header-filter policy attached means no header properties on the event, and
// therefore no header attributes — never an empty or partial set.
func TestBuildRecordNoHeaderAttributesWhenAbsent(t *testing.T) {
	cases := map[string]*dto.Event{
		"property absent":  restEvent(),
		"empty string":     headerEventRaw(""),
		"empty object":     headerEventRaw("{}"),
		"unparseable json": headerEventRaw("not json"),
		"wrong type":       headerEventWrongType(),
	}
	for name, event := range cases {
		t.Run(name, func(t *testing.T) {
			o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
			got := attrMap(t, o.buildRecord(event))
			for key := range got {
				if strings.HasPrefix(key, "http.request.header.") ||
					strings.HasPrefix(key, "http.response.header.") {
					t.Errorf("unexpected header attribute %q", key)
				}
			}
		})
	}
}

func headerEventRaw(serialized string) *dto.Event {
	event := restEvent()
	event.Properties[dto.PropKeyRequestHeaders] = serialized
	return event
}

func headerEventWrongType() *dto.Event {
	event := restEvent()
	event.Properties[dto.PropKeyRequestHeaders] = map[string]string{"not": "a string"}
	return event
}

// HTTP/2 pseudo-headers are not headers. Envoy surfaces them next to the real
// ones, and each duplicates an attribute already mapped from its own event
// field — while ":path" can carry a query string into an attribute key's value.
func TestBuildRecordHeaderAttributesSkipPseudoHeaders(t *testing.T) {
	event := headerEvent(t,
		map[string]string{
			":method": "GET", ":path": "/otele2e/anything?token=secret",
			":scheme": "http", ":authority": "localhost:8080",
			"x-tenant": "acme",
		},
		map[string]string{":status": "200", "content-type": "application/json"})

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key := range got {
		if strings.Contains(key, ".header.:") {
			t.Errorf("pseudo-header emitted as %q", key)
		}
	}
	// The real headers alongside them must still come through.
	for _, want := range []string{"http.request.header.x-tenant", "http.response.header.content-type"} {
		if _, present := got[want]; !present {
			t.Errorf("%s is missing", want)
		}
	}
}

// An over-broad allowlist must not grow a record's schema without bound: header
// names become attribute keys, and SDKs cap a record at 128 attributes.
func TestBuildRecordHeaderAttributesAreCapped(t *testing.T) {
	headers := map[string]string{}
	for i := 0; i < otelMaxHeaderAttributes*2; i++ {
		headers[fmt.Sprintf("x-header-%03d", i)] = "value"
	}
	event := headerEvent(t, headers, nil)

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	emitted := []string{}
	for key := range got {
		if strings.HasPrefix(key, "http.request.header.") {
			emitted = append(emitted, key)
		}
	}
	if len(emitted) != otelMaxHeaderAttributes {
		t.Errorf("emitted %d header attributes, want the cap of %d", len(emitted), otelMaxHeaderAttributes)
	}
	// Sorted selection, so a truncated record keeps the same headers every time
	// rather than an arbitrary subset that changes per request.
	sort.Strings(emitted)
	if emitted[0] != "http.request.header.x-header-000" {
		t.Errorf("first emitted = %s, want the lowest-sorting name", emitted[0])
	}
}

// The cap is per direction, so a wide request allowlist cannot starve the
// response headers.
func TestBuildRecordHeaderCapIsPerDirection(t *testing.T) {
	request := map[string]string{}
	for i := 0; i < otelMaxHeaderAttributes*2; i++ {
		request[fmt.Sprintf("x-req-%03d", i)] = "value"
	}
	event := headerEvent(t, request, map[string]string{"Cache-Control": "no-store"})

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	if _, present := got["http.response.header.cache-control"]; !present {
		t.Error("response header dropped because request headers hit the cap")
	}
}

// A header with no value is skipped rather than emitted as an empty array, and
// must not consume cap budget.
func TestBuildRecordHeaderAttributesSkipEmptyValues(t *testing.T) {
	event := headerEvent(t, map[string]string{"X-Present": "yes", "X-Empty": ""}, nil)

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	if _, present := got["http.request.header.x-empty"]; present {
		t.Error("an empty header value produced an attribute")
	}
	if _, present := got["http.request.header.x-present"]; !present {
		t.Error("x-present is missing")
	}
}

// The array must serialize as OTLP's ArrayValue shape, since that is the wire
// contract a collector parses.
func TestHeaderAttributeWireShape(t *testing.T) {
	event := headerEvent(t, map[string]string{"Accept-Encoding": "gzip, br"}, nil)

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	encoded, err := json.Marshal(o.buildRecord(event))
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	want := `{"key":"http.request.header.accept-encoding","value":{"arrayValue":{"values":[{"stringValue":"gzip, br"}]}}}`
	if !strings.Contains(string(encoded), want) {
		t.Errorf("record does not contain the expected ArrayValue attribute.\nwant substring: %s\ngot: %s", want, encoded)
	}
}

// OTLP log records have traceId/spanId envelope fields. We deliberately do not
// emit them (design doc §2.10), and this pins that: adding the fields "for
// completeness" would send an all-zero id to every destination on every record.
//
// Note that a collector's own re-serialization may still show traceId:"" — its
// internal representation holds those as fixed-size values that are always
// present. That is the collector's output format, not our payload.
func TestRecordOmitsTraceEnvelopeFields(t *testing.T) {
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	encoded, err := json.Marshal(o.buildRecord(restEvent()))
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	for _, field := range []string{"traceId", "spanId"} {
		if strings.Contains(string(encoded), field) {
			t.Errorf("record carries %q; it must be absent, not empty: %s", field, encoded)
		}
	}
}

// A key the analytics policy gains later must appear in the export rather than
// vanish until someone notices. That is the whole point of the sweep: flattening
// to curated names would otherwise mean the publisher and the policy drift
// silently.
func TestMCPUnmappedKeysAreSweptUp(t *testing.T) {
	event := restEvent()
	event.API.APIType = "Mcp"
	event.Properties["mcpAnalytics"] = map[string]interface{}{
		"jsonRpcMethod": "tools/call",
		// Hypothetical future additions, at both levels.
		"toolInvocationCount": float64(3),
		"cacheWasWarm":        true,
		"upstreamLatencyMs":   12.5,
		"clientInfo": map[string]interface{}{
			"name":       "claude-desktop",
			"platformId": "darwin-arm64",
		},
	}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, want := range map[string]interface{}{
		// camelCase becomes snake_case under the custom namespace.
		"wso2.mcp.tool_invocation_count": "3", // integral float -> IntValue, formatted as a string
		"wso2.mcp.cache_was_warm":        true,
		"wso2.mcp.upstream_latency_ms":   12.5,
		"wso2.mcp.client.platform_id":    "darwin-arm64",
		// The curated name still wins for a key that has one.
		"mcp.method.name":      "tools/call",
		"wso2.mcp.client.name": "claude-desktop",
	} {
		if got[key] != want {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
	}

	// A key with a curated name must not also appear under the sweep prefix.
	for _, absent := range []string{
		"wso2.mcp.json_rpc_method", "wso2.mcp.client_info", "wso2.mcp.client.name_",
	} {
		if _, present := got[absent]; present {
			t.Errorf("%s was emitted twice / under the wrong name", absent)
		}
	}
}

// `capability` is read but deliberately not emitted — which attribute is
// populated already says it. It must not reappear via the sweep.
func TestMCPCapabilityIsNotSwept(t *testing.T) {
	event := restEvent()
	event.API.APIType = "Mcp"
	event.Properties["mcpAnalytics"] = map[string]interface{}{
		"jsonRpcMethod":  "tools/call",
		"capability":     "TOOL",
		"capabilityName": "search_docs",
	}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for _, absent := range []string{"wso2.mcp.capability", "wso2.mcp.capability_name"} {
		if _, present := got[absent]; present {
			t.Errorf("%s must not be swept up; it has a curated mapping", absent)
		}
	}
	if got["gen_ai.tool.name"] != "search_docs" {
		t.Errorf("gen_ai.tool.name = %v, want search_docs", got["gen_ai.tool.name"])
	}
}

func TestOTelSnakeCase(t *testing.T) {
	for input, want := range map[string]string{
		"jsonRpcMethod":            "json_rpc_method",
		"resourceUri":              "resource_uri",
		"isError":                  "is_error",
		"requestedProtocolVersion": "requested_protocol_version",
		"sessionId":                "session_id",
		"already_snake":            "already_snake",
		"name":                     "name",
		"":                         "",
	} {
		if got := otelSnakeCase(input); got != want {
			t.Errorf("otelSnakeCase(%q) = %q, want %q", input, got, want)
		}
	}
}

// An integral JSON number must not be reported as a double: a backend types the
// column from the first value it sees, and a consumer summing 3.0 and 3 does not
// reliably get the same answer.
func TestAnyScalarNumberKinds(t *testing.T) {
	attrs := newOTelAttrs()
	attrs.anyScalar("whole", float64(3))
	attrs.anyScalar("fractional", 12.5)
	attrs.anyScalar("text", "hello")
	attrs.anyScalar("flag", true)
	attrs.anyScalar("unsupported", []string{"nope"})

	encoded, err := json.Marshal(attrs.list())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`{"key":"whole","value":{"intValue":"3"}}`,
		`{"key":"fractional","value":{"doubleValue":12.5}}`,
		`{"key":"text","value":{"stringValue":"hello"}}`,
		`{"key":"flag","value":{"boolValue":true}}`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("missing %s in %s", want, encoded)
		}
	}
	if strings.Contains(string(encoded), "unsupported") {
		t.Error("an unsupported type produced an attribute")
	}
}

// --- Zero is a value, not an absence -----------------------------------------
//
// A record has one way to say "this attribute does not apply to this request":
// leave it out. So a measured zero must be emitted, or "the guardrail blocked
// this request so it produced no output tokens" and "this is a REST call with no
// tokens at all" become the same record to a consumer — and every avg() over the
// field drops the zeros from its denominator instead of counting them.

// The wire shape is what actually carries the distinction. omitempty on a
// pointer field tests only for nil, which is why a *string holding "0" and a
// *bool holding false still marshal. Asserted here because a later refactor to
// non-pointer fields would silently restore the bug this test exists to prevent.
func TestZeroValuedAttributesReachTheWire(t *testing.T) {
	attrs := newOTelAttrs()
	attrs.i64("zero.int", 0)
	attrs.f64("zero.double", 0)
	attrs.b("zero.bool", false)

	encoded, err := json.Marshal(attrs.list())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`{"key":"zero.int","value":{"intValue":"0"}}`,
		`{"key":"zero.double","value":{"doubleValue":0}}`,
		`{"key":"zero.bool","value":{"boolValue":false}}`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("missing %s in %s", want, encoded)
		}
	}
}

// A guardrail-blocked completion: the prompt was tokenized, nothing was
// generated, nothing was billed. Reporting no output tokens is the whole point
// of the record.
func TestBuildRecordGenAIZeroUsageIsReported(t *testing.T) {
	event := restEvent()
	event.API.APIType = "LlmProxy"
	event.Operation.APIResourceTemplate = "/ai/chat/completions"
	event.Properties["aiMetadata"] = dto.AIMetadata{
		Model:      "claude-opus-4",
		VendorName: "anthropic",
		LLMCost:    float64(0),
	}
	event.Properties["aiTokenUsage"] = dto.AITokenUsage{PromptToken: 1841, CompletionToken: 0, TotalToken: 1841}
	event.Properties[constants.GuardrailHitMetadataKey] = true

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, want := range map[string]interface{}{
		"gen_ai.usage.input_tokens":      "1841",
		"gen_ai.usage.output_tokens":     "0",
		"wso2.gen_ai.usage.total_tokens": "1841",
		"wso2.gen_ai.cost.total":         float64(0),
		"wso2.guardrail.hit":             true,
	} {
		actual, present := got[key]
		if !present {
			t.Errorf("%s is absent; a measured zero must be emitted, not omitted", key)
			continue
		}
		if actual != want {
			t.Errorf("%s = %v (%T), want %v (%T)", key, actual, actual, want, want)
		}
	}
}

// A GET has no request body and a cache miss is not a cache hit. Both are
// measurements the record must carry: without them "empty body" is
// indistinguishable from "body size not measured", and a cache miss from an API
// with no cache filter at all — which is what makes a hit ratio uncomputable.
func TestBuildRecordZeroSizesAndFalseFlagsAreReported(t *testing.T) {
	event := restEvent()
	event.Properties["requestSize"] = uint64(0)
	event.Properties["responseSize"] = uint64(0)
	event.Target.ResponseCacheHit = false
	event.Latencies = &dto.Latencies{ResponseLatency: 4, BackendLatency: 0, Duration: 4}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for key, want := range map[string]interface{}{
		"http.request.body.size":  "0",
		"http.response.body.size": "0",
		"wso2.cache.hit":          false,
		// Served from cache or sub-millisecond: zero backend time, measured.
		"wso2.latency.backend_ms":            "0",
		"wso2.latency.request_mediation_ms":  "0",
		"wso2.latency.response_mediation_ms": "0",
	} {
		actual, present := got[key]
		if !present {
			t.Errorf("%s is absent; a measured zero must be emitted, not omitted", key)
			continue
		}
		if actual != want {
			t.Errorf("%s = %v (%T), want %v (%T)", key, actual, actual, want, want)
		}
	}
}

// The exceptions. For these four, 0 is a sentinel rather than a measurement —
// there is no HTTP status 0, no TCP port 0, no error code 0 — so they keep
// suppressing it via i64NonZero.
func TestBuildRecordSentinelZerosStayOmitted(t *testing.T) {
	event := restEvent()
	event.ProxyResponseCode = 0
	event.Target = &dto.Target{TargetResponseCode: 0, Destination: "backend:0/pet/1"}
	event.Error = &dto.Error{ErrorCode: 0, ErrorMessage: dto.OtherUnclassified}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	for _, key := range []string{
		"http.response.status_code",
		"server.port",
		"wso2.upstream.response.status_code",
		"wso2.error.code",
	} {
		if actual, present := got[key]; present {
			t.Errorf("%s = %v; 0 is a sentinel for this attribute and must be omitted", key, actual)
		}
	}
}
