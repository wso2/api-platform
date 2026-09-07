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
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
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
		QueueSize:     100,
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
			"responseSize":        uint64(463),
			"responseContentType": "application/json",
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
		"event.name":                         "wso2.api.transaction",
		"http.request.method":                "GET",
		"http.route":                         "/petstore/pet/{petId}",
		"url.path":                           "/petstore/pet/{petId}",
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
	if got["url.path"] != "/petstore/pet/{petId}" {
		t.Errorf("url.path = %v, want /petstore/pet/{petId}", got["url.path"])
	}
}

// With no resource template the API context is the fallback path.
func TestBuildRecordFallsBackToContext(t *testing.T) {
	event := restEvent()
	event.Operation.APIResourceTemplate = ""
	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))
	if got["url.path"] != "/petstore" {
		t.Errorf("url.path = %v, want /petstore", got["url.path"])
	}
	if _, present := got["http.route"]; present {
		t.Error("http.route should be absent when there is no resource template")
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
		"wso2.error.sub_category":   "AUTHENTICATION_FAILURE",
		"http.response.status_code": "401",
	} {
		if got[key] != expected {
			t.Errorf("%s = %v, want %v", key, got[key], expected)
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

// An unrecognised provider leaves the enum attribute unset rather than carrying
// a non-member value, and keeps its identity in the wso2.* attribute.
func TestBuildRecordUnknownGenAIProvider(t *testing.T) {
	event := restEvent()
	event.Properties["aiMetadata"] = dto.AIMetadata{VendorName: "some-private-llm", Model: "m1"}

	o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
	got := attrMap(t, o.buildRecord(event))

	if _, present := got["gen_ai.provider.name"]; present {
		t.Errorf("gen_ai.provider.name should be unset for an unknown provider, got %v", got["gen_ai.provider.name"])
	}
	if got["wso2.gen_ai.provider.template_name"] != "some-private-llm" {
		t.Errorf("template_name = %v", got["wso2.gen_ai.provider.template_name"])
	}
}

func TestGenAIOperationName(t *testing.T) {
	for route, want := range map[string]string{
		"/ai/chat/completions": "chat",
		"/anthropic/messages":  "chat",
		"/ai/embeddings":       "embeddings",
		"/ai/completions":      "text_completion",
		"/petstore/pet/{id}":   "",
	} {
		if got := otelGenAIOperationName(route); got != want {
			t.Errorf("otelGenAIOperationName(%q) = %q, want %q", route, got, want)
		}
	}
}

func TestBuildRecordMCP(t *testing.T) {
	event := restEvent()
	event.API.APIType = "Mcp"
	event.Properties["mcpAnalytics"] = map[string]interface{}{
		"jsonRpcMethod":            "tools/call",
		"jsonRpcId":                "7",
		"sessionId":                "sess-1",
		"capability":               "TOOL",
		"capabilityName":           "search_docs",
		"errorCode":                -32602,
		"isError":                  true,
		"protocolVersion":          "2025-06-18",
		"requestedProtocolVersion": "2025-03-26",
		"name":                     "claude-desktop",
		"version":                  "1.2.0",
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

func TestMCPCapabilityRouting(t *testing.T) {
	for capability, wantKey := range map[string]string{
		"RESOURCE": "mcp.resource.uri",
		"PROMPT":   "gen_ai.prompt.name",
		"TOOL":     "gen_ai.tool.name",
	} {
		event := restEvent()
		event.Properties["mcpAnalytics"] = map[string]interface{}{
			"capability":     capability,
			"capabilityName": "target-1",
		}
		o := &OTel{cfg: testOTelConfig("http://collector/v1/logs")}
		got := attrMap(t, o.buildRecord(event))
		if got[wantKey] != "target-1" {
			t.Errorf("capability %s: %s = %v, want target-1", capability, wantKey, got[wantKey])
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

// A full queue must drop rather than block the ALS ingest path.
func TestPublishDropsWhenQueueFull(t *testing.T) {
	cfg := testOTelConfig("http://127.0.0.1:1/v1/logs")
	cfg.QueueSize = 1
	cfg.BatchSize = 1
	cfg.FlushInterval = time.Hour // keep the worker parked so the queue fills

	o := &OTel{
		cfg:        cfg,
		client:     &http.Client{Timeout: cfg.Timeout},
		queue:      make(chan *otelLogRecord, cfg.QueueSize),
		stop:       make(chan struct{}),
		workerDone: make(chan struct{}),
	}
	// No worker started: nothing drains the queue.
	for i := 0; i < 10; i++ {
		o.Publish(restEvent())
	}

	o.droppedMu.Lock()
	dropped := o.dropped
	o.droppedMu.Unlock()
	if dropped != 9 {
		t.Errorf("dropped = %d, want 9 (queue holds 1)", dropped)
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
