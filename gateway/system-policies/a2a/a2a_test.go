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

package a2a

import (
	"context"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	testCard = `{"name":"weather","protocolVersion":"1.0"}`
	testETag = `"c0ffee"`
)

// run calls the policy with the given request headers and the standard card
// parameters, and asserts the chain was terminated.
func run(t *testing.T, headers map[string][]string, params map[string]any) policy.ImmediateResponse {
	t.Helper()
	action := ins.OnRequestHeaders(context.Background(), &policy.RequestHeaderContext{
		SharedContext: &policy.SharedContext{APIId: "agent-uuid-1"},
		Headers:       policy.NewHeaders(headers),
		Method:        "GET",
		Path:          "/weather/.well-known/agent-card.json",
	}, params)

	immediate, ok := action.(policy.ImmediateResponse)
	if !ok {
		t.Fatalf("expected the card policy to terminate the chain, got %T", action)
	}
	return immediate
}

// cardBlockParams wraps an agentCard block in the policy's top-level parameter
// map, which is the shape the controller writes.
func cardBlockParams(block map[string]any) map[string]any {
	return map[string]any{ParamAgentCard: block}
}

func cardParams() map[string]any {
	return cardBlockParams(map[string]any{ParamContent: testCard, ParamETag: testETag})
}

// The card policy must never let a card request through to the upstream: it is
// attached only to a managed card route, where forwarding would serve the
// upstream's own unvalidated card in place of the one the gateway owns.
func TestCardRequestAlwaysTerminatesTheChain(t *testing.T) {
	if !(policy.ImmediateResponse{}).StopExecution() {
		t.Fatal("ImmediateResponse must stop chain execution")
	}
	response := run(t, nil, cardParams())
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
}

func TestServesTheConfiguredCardBytesVerbatim(t *testing.T) {
	response := run(t, nil, cardParams())

	if got := string(response.Body); got != testCard {
		t.Errorf("body = %q, want %q", got, testCard)
	}
	if got := response.Headers["content-type"]; got != contentTypeJSON {
		t.Errorf("content-type = %q, want %q", got, contentTypeJSON)
	}
	if got := response.Headers["etag"]; got != testETag {
		t.Errorf("etag = %q, want %q", got, testETag)
	}
	if got := response.Headers["cache-control"]; got != cacheControlValue {
		t.Errorf("cache-control = %q, want %q", got, cacheControlValue)
	}
}

// A 304 carries the validator and the caching directives but no body — a client
// refreshing a stored copy must see the same metadata the full response carried.
func TestMatchingIfNoneMatchReturns304WithNoBody(t *testing.T) {
	response := run(t, map[string][]string{"If-None-Match": {testETag}}, cardParams())

	if response.StatusCode != 304 {
		t.Fatalf("status = %d, want 304", response.StatusCode)
	}
	if len(response.Body) != 0 {
		t.Errorf("304 carried a %d-byte body; it must carry none", len(response.Body))
	}
	if got := response.Headers["etag"]; got != testETag {
		t.Errorf("etag = %q, want %q", got, testETag)
	}
	if got := response.Headers["cache-control"]; got != cacheControlValue {
		t.Errorf("cache-control = %q, want %q", got, cacheControlValue)
	}
}

// If-None-Match uses weak comparison whatever the tags involved, may repeat, and
// may carry a list per occurrence. A tag for some other representation must not
// produce a 304 — that would serve the client a card it does not have.
func TestIfNoneMatchComparison(t *testing.T) {
	cases := []struct {
		name    string
		values  []string
		matches bool
	}{
		{"exact", []string{testETag}, true},
		{"weak candidate against strong tag", []string{`W/"c0ffee"`}, true},
		{"wildcard", []string{"*"}, true},
		{"one of a list", []string{`"other", "c0ffee"`}, true},
		{"second occurrence", []string{`"other"`, testETag}, true},
		{"list with spaces and empty entries", []string{`  ,  "c0ffee"  `}, true},
		{"different tag", []string{`"decaf"`}, false},
		{"unquoted tag", []string{"c0ffee"}, false},
		{"substring of the tag", []string{`"c0ff"`}, false},
		{"empty header", []string{""}, false},
		{"absent header", nil, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ifNoneMatchSatisfied(testCase.values, testETag); got != testCase.matches {
				t.Errorf("ifNoneMatchSatisfied(%q, %q) = %v, want %v",
					testCase.values, testETag, got, testCase.matches)
			}
		})
	}
}

// A card with no entity tag still serves. The ETag header is omitted rather than
// sent empty, and no If-None-Match can then produce a 304 — including "*", which
// would otherwise 304 a client that holds nothing.
func TestCardWithoutAnETagServesWithoutOne(t *testing.T) {
	params := cardBlockParams(map[string]any{ParamContent: testCard})

	response := run(t, map[string][]string{"If-None-Match": {"*"}}, params)
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if _, present := response.Headers["etag"]; present {
		t.Error("an ETag header was sent for a card that has no entity tag")
	}
}

// A chain built without the card this policy exists to serve is a controller
// defect. It must fail closed rather than fall through to the upstream, and the
// body must not explain why (error-handling.md directive 1).
func TestMissingOrUnusableContentFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
	}{
		{"nil params", nil},
		{"no agentCard block", map[string]any{}},
		{"agentCard block is not an object", map[string]any{ParamAgentCard: testCard}},
		{"no content parameter", cardBlockParams(map[string]any{ParamETag: testETag})},
		{"empty content", cardBlockParams(map[string]any{ParamContent: ""})},
		{"content is not a string", cardBlockParams(map[string]any{
			ParamContent: map[string]any{"name": "weather"},
		})},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := run(t, nil, testCase.params)
			if response.StatusCode != 500 {
				t.Fatalf("status = %d, want 500", response.StatusCode)
			}
			if body := string(response.Body); body != `{"error":"unavailable","message":"The Agent Card could not be served."}` {
				t.Errorf("failure body is not the sterile payload: %q", body)
			}
		})
	}
}

// A non-string etag is ignored rather than coerced: an ETag header built from a
// stringified map would be a validator no client could ever match, and 304
// decisions would be made against it.
func TestNonStringETagIsIgnored(t *testing.T) {
	params := cardBlockParams(map[string]any{ParamContent: testCard, ParamETag: 42})

	response := run(t, nil, params)
	if response.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if _, present := response.Headers["etag"]; present {
		t.Error("an ETag header was built from a non-string parameter")
	}
}

// The card route answers immediately, so buffering either body would make the
// kernel wait for bytes nothing reads.
func TestModeParticipatesInTheRequestHeaderPhaseOnly(t *testing.T) {
	mode := ins.Mode()

	if mode.RequestHeaderMode != policy.HeaderModeProcess {
		t.Errorf("RequestHeaderMode = %q, want %q", mode.RequestHeaderMode, policy.HeaderModeProcess)
	}
	if mode.RequestBodyMode != policy.BodyModeSkip {
		t.Errorf("RequestBodyMode = %q, want %q", mode.RequestBodyMode, policy.BodyModeSkip)
	}
	if mode.ResponseHeaderMode != policy.HeaderModeSkip {
		t.Errorf("ResponseHeaderMode = %q, want %q", mode.ResponseHeaderMode, policy.HeaderModeSkip)
	}
	if mode.ResponseBodyMode != policy.BodyModeSkip {
		t.Errorf("ResponseBodyMode = %q, want %q", mode.ResponseBodyMode, policy.BodyModeSkip)
	}
}

// The parameter names are the contract with the gateway controller's Agent
// transformer, which writes them from a separate Go module and so cannot share
// these constants. Renaming one here without renaming it there would leave a
// managed card route serving a 500.
func TestParameterNamesMatchTheControllerContract(t *testing.T) {
	if ParamAgentCard != "agentCard" {
		t.Errorf("ParamAgentCard = %q, want \"agentCard\"", ParamAgentCard)
	}
	if ParamContent != "content" {
		t.Errorf("ParamContent = %q, want \"content\"", ParamContent)
	}
	if ParamETag != "etag" {
		t.Errorf("ParamETag = %q, want \"etag\"", ParamETag)
	}
}

func TestGetPolicyReturnsAUsableInstance(t *testing.T) {
	for name, factory := range map[string]policy.PolicyFactory{
		"GetPolicy":   GetPolicy,
		"GetPolicyV2": GetPolicyV2,
	} {
		t.Run(name, func(t *testing.T) {
			instance, err := factory(policy.PolicyMetadata{}, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, ok := instance.(policy.RequestHeaderPolicy); !ok {
				t.Fatalf("instance %T does not implement RequestHeaderPolicy", instance)
			}
		})
	}
}
