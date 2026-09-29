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

package kernel

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// The sterile-response rule survives the addition of a code: the body must reveal no more
// than the status line beside it already does. So codes are allocated per STATUS, and the
// four kinds that all mean "400" share one — a code per kind would let a caller tell
// FailureParse from FailureInvalidRequest, which is resolver internals.
func TestResolutionFailureAccount_GroupedByStatusNotByFailureKind(t *testing.T) {
	cases := []struct {
		kind    resolver.FailureKind
		status  int
		code    string
		errType string
	}{
		{resolver.FailureParse, http.StatusBadRequest, codeResolutionBadRequest, policy.FaultTypeValidation},
		{resolver.FailureInvalidRequest, http.StatusBadRequest, codeResolutionBadRequest, policy.FaultTypeValidation},
		{resolver.FailureMultiOperation, http.StatusBadRequest, codeResolutionBadRequest, policy.FaultTypeValidation},
		{resolver.FailureUndecodableBody, http.StatusBadRequest, codeResolutionBadRequest, policy.FaultTypeValidation},
		{resolver.FailureUnknownOperation, http.StatusNotFound, codeNoRoute, policy.FaultTypeRouting},
		{resolver.FailurePayloadTooLarge, http.StatusRequestEntityTooLarge, codePayloadTooLarge, policy.FaultTypeRequestSize},
		{resolver.FailureUnsupportedEncoding, http.StatusUnsupportedMediaType, codeResolutionUnsupportedEncoding, policy.FaultTypeValidation},
		{resolver.FailureUnknownResolver, http.StatusInternalServerError, codeEngineInternal, policy.FaultTypeInternal},
		{resolver.FailureChainMissing, http.StatusInternalServerError, codeEngineInternal, policy.FaultTypeInternal},
		{resolver.FailureInternal, http.StatusInternalServerError, codeEngineInternal, policy.FaultTypeInternal},
	}

	byStatus := map[int]string{}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			status, account := resolutionFailureAccount(tc.kind)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, account.Code)
			assert.Equal(t, tc.errType, account.Type)
			assert.NotEmpty(t, account.Message, "a renderer needs something to put in message")
			assert.Empty(t, account.Policy, "no policy runs before a chain is bound")
			assert.Equal(t, policy.DirectionRequest, account.Direction)
		})
		if seen, ok := byStatus[tc.status]; ok {
			assert.Equal(t, seen, tc.code,
				"two kinds sharing a status must share a code, or the body discloses the kind")
		}
		byStatus[tc.status] = tc.code
	}

	// And distinct statuses must NOT share a code, or the code adds nothing.
	seen := map[string]int{}
	for status, code := range byStatus {
		if prev, ok := seen[code]; ok {
			t.Errorf("code %s used for both status %d and %d", code, prev, status)
		}
		seen[code] = status
	}
}

// An unclassified kind must land on the 500 default rather than a zero status, which Envoy
// would reject as an invalid immediate response.
func TestResolutionFailureAccount_UnknownKindFallsBackToInternal(t *testing.T) {
	status, account := resolutionFailureAccount(resolver.FailureKind("something-new"))
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, codeEngineInternal, account.Code)
	assert.Equal(t, policy.FaultTypeInternal, account.Type)
}

func TestShapeSignalsFromHeaders(t *testing.T) {
	hdrs := &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
		{Key: ":method", RawValue: []byte("post")},
		{Key: "content-type", RawValue: []byte("text/event-stream")},
		{Key: "accept", RawValue: []byte("application/xml")},
	}}}

	sig := shapeSignalsFromHeaders(policy.APIKindMCP, hdrs)

	assert.Equal(t, policy.APIKindMCP, sig.APIKind)
	assert.Equal(t, "POST", sig.Method, "the method is upper-cased at extraction (GO-AUTH-006)")
	assert.Equal(t, "text/event-stream", sig.ContentType)
	assert.Equal(t, "application/xml", sig.Accept)

	// A request with no headers at all must not panic — the no-route paths reach here with
	// whatever Envoy sent, which in a malformed case may be nothing.
	empty := shapeSignalsFromHeaders(policy.APIKindRestApi, nil)
	assert.Equal(t, policy.APIKindRestApi, empty.APIKind)
	assert.Empty(t, empty.Method)
}

// ─── Resolution failures ─────────────────────────────────────────────────────

func resolutionFailureBody(t *testing.T, kind policy.APIKind, enable bool,
	failure resolver.FailureKind, sig requestShapeSignals) *extprocv3.ImmediateResponse {
	t.Helper()
	f := newResolutionFixture(t)
	if enable {
		enableFaultFormatterOn(t, f.server, kind)
	}
	sig.APIKind = kind
	// nil analytics: these cases assert the shape of the sterile body, and the payload is
	// attached only when one is supplied.
	resp, _ := f.server.renderResolutionFailure(context.Background(), "res", "route", "req-1",
		sig, &resolver.ResolutionError{Kind: failure}, nil)
	imm := resp.GetImmediateResponse()
	require.NotNil(t, imm)
	return imm
}

// The compatibility guarantee for these paths: a kind the operator did not enable receives
// the bytes it has always received.
func TestResolutionFailure_DisabledKindKeepsTheSterileBody(t *testing.T) {
	imm := resolutionFailureBody(t, policy.APIKindMCP, false, resolver.FailureParse,
		requestShapeSignals{Method: "POST", ContentType: "application/json"})

	assert.Equal(t, uint32(400), uint32(imm.GetStatus().GetCode()))
	var body map[string]any
	require.NoError(t, json.Unmarshal(imm.GetBody(), &body))
	assert.Equal(t, "Bad Request", body["error"], "the pre-existing sterile shape")
	assert.NotEmpty(t, body["error_id"])
	assert.NotContains(t, body, "code", "an unenabled kind gains no rendered envelope")
}

// An MCP client cannot parse `{"error":"Bad Request"}` — it is not a JSON-RPC error object.
// This is the case these paths were missing.
func TestResolutionFailure_EnabledMcpKindGetsJSONRPC(t *testing.T) {
	imm := resolutionFailureBody(t, policy.APIKindMCP, true, resolver.FailureParse,
		requestShapeSignals{Method: "POST", ContentType: "application/json"})

	assert.Equal(t, uint32(400), uint32(imm.GetStatus().GetCode()),
		"formatting must never change the status")

	var body map[string]any
	require.NoError(t, json.Unmarshal(imm.GetBody(), &body), "body: %s", imm.GetBody())
	assert.Equal(t, "2.0", body["jsonrpc"])
	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "error must be a JSON-RPC error OBJECT: %s", imm.GetBody())
	assert.Equal(t, "Bad Request", errObj["message"])

	data, ok := errObj["data"].(map[string]any)
	require.True(t, ok, "the gateway code travels in data: %s", imm.GetBody())
	assert.Equal(t, codeResolutionBadRequest, data["code"])
	assert.Equal(t, policy.FaultTypeValidation, data["type"])
	assert.NotEmpty(t, data["error_id"], "the correlation id must survive rendering")
}

// The content type has to travel with the body, or a client sees JSON-RPC labelled as
// whatever the sterile response happened to declare.
func TestResolutionFailure_EnabledKindSetsTheRenderedContentType(t *testing.T) {
	imm := resolutionFailureBody(t, policy.APIKindRestApi, true, resolver.FailureUnknownOperation,
		requestShapeSignals{Method: "GET", Accept: "application/xml"})

	assert.Equal(t, uint32(404), uint32(imm.GetStatus().GetCode()))
	assert.Contains(t, string(imm.GetBody()), "<error>", "an XML caller gets an XML document")

	var contentType string
	for _, h := range imm.GetHeaders().GetSetHeaders() {
		if h.GetHeader().GetKey() == "content-type" {
			contentType = string(h.GetHeader().GetRawValue())
		}
	}
	assert.Equal(t, "application/xml", contentType)
}

// ─── A route with no policy chain ────────────────────────────────────────────

func noChainResponse(t *testing.T, kind policy.APIKind, enable bool,
	headers map[string]string) *extprocv3.ImmediateResponse {
	t.Helper()
	const routeKey = "GET|/pets|example.com"
	f := newResolutionFixture(t)
	rc := f.route(routeKey, resolver.RouteResolution{}) // identity route, and no chain registered
	rc.Metadata.APIKind = string(kind)
	if enable {
		enableFaultFormatterOn(t, f.server, kind)
	}

	stream := newMockStream([]*extprocv3.ProcessingRequest{headersRequest(routeKey, true, headers)})
	require.NoError(t, f.server.Process(stream))
	require.Len(t, stream.responses, 1)
	imm := stream.responses[0].GetImmediateResponse()
	require.NotNil(t, imm)
	return imm
}

func TestNoPolicyChain_DisabledKindKeepsTheSterileBody(t *testing.T) {
	imm := noChainResponse(t, policy.APIKindMCP, false, map[string]string{":method": "GET"})

	assert.Equal(t, uint32(500), uint32(imm.GetStatus().GetCode()))
	assert.JSONEq(t, `{"error":"Internal Server Error"}`, string(imm.GetBody()),
		"byte-for-byte the body this path has always returned")
}

func TestNoPolicyChain_EnabledMcpKindGetsJSONRPC(t *testing.T) {
	imm := noChainResponse(t, policy.APIKindMCP, true, map[string]string{":method": "GET"})

	assert.Equal(t, uint32(500), uint32(imm.GetStatus().GetCode()))
	var body map[string]any
	require.NoError(t, json.Unmarshal(imm.GetBody(), &body), "body: %s", imm.GetBody())
	assert.Equal(t, "2.0", body["jsonrpc"])

	errObj, ok := body["error"].(map[string]any)
	require.True(t, ok, "body: %s", imm.GetBody())
	data, ok := errObj["data"].(map[string]any)
	require.True(t, ok, "body: %s", imm.GetBody())
	assert.Equal(t, codeNoPolicyChain, data["code"],
		"a missing chain is its own condition, not the generic engine-internal code")
	assert.Equal(t, policy.FaultTypeInternal, data["type"])
	assert.NotEmpty(t, data["error_id"],
		"this path had no correlation id at all before; the rendered body carries one")
}

// A HEAD response carries no body whatever the config says — Envoy recalculates
// content-length from the mutation, so a body here contradicts the method.
func TestNoPolicyChain_HeadRequestGetsNoSynthesizedBody(t *testing.T) {
	imm := noChainResponse(t, policy.APIKindMCP, true, map[string]string{":method": "HEAD"})

	assert.Equal(t, uint32(500), uint32(imm.GetStatus().GetCode()))
	assert.JSONEq(t, `{"error":"Internal Server Error"}`, string(imm.GetBody()),
		"HEAD outranks an enabled kind")
}

// Enabling one kind must not format another's failures.
func TestNoPolicyChain_EnablingAnotherKindChangesNothing(t *testing.T) {
	imm := noChainResponse(t, policy.APIKindRestApi, false, map[string]string{":method": "GET"})
	assert.JSONEq(t, `{"error":"Internal Server Error"}`, string(imm.GetBody()))
}
