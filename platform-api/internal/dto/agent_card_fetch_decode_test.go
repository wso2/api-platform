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

package dto

import (
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/api"
)

// The three bodies the spec publishes as named examples. Each must decode into
// exactly the form its keys select.
func TestDecodeAgentCardFetchRequest_AcceptsBothForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantHandle string
		wantURL    string
		wantAuth   bool
	}{
		{
			name:    "direct url",
			body:    `{"url":"http://weather-agent:9000"}`,
			wantURL: "http://weather-agent:9000",
		},
		{
			name:     "direct url with auth",
			body:     `{"url":"http://weather-agent:9000","auth":{"type":"api-key","header":"X-API-Key","value":"example-preview-key"}}`,
			wantURL:  "http://weather-agent:9000",
			wantAuth: true,
		},
		{
			name:       "stored handle",
			body:       `{"agentProxyId":"weather-agent"}`,
			wantHandle: "weather-agent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeAgentCardFetchRequest([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeAgentCardFetchRequest() error = %v, want nil", err)
			}
			if got.AgentProxyID != tc.wantHandle {
				t.Errorf("AgentProxyID = %q, want %q", got.AgentProxyID, tc.wantHandle)
			}
			if got.URL != tc.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tc.wantURL)
			}
			if (got.Auth != nil) != tc.wantAuth {
				t.Errorf("Auth present = %v, want %v", got.Auth != nil, tc.wantAuth)
			}
			if got.IsStored() != (tc.wantHandle != "") {
				t.Errorf("IsStored() = %v, want %v", got.IsStored(), tc.wantHandle != "")
			}
		})
	}
}

// The stored form forbids both url and auth, and forbids them on key presence
// rather than on value — an explicitly null url is still a supplied url. Every
// one of these has to be rejected here, before any credential is resolved or any
// outbound request is made.
func TestDecodeAgentCardFetchRequest_RejectsInvalidCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "empty object", body: `{}`},
		{name: "auth alone", body: `{"auth":{"type":"api-key","header":"X-API-Key","value":"k"}}`},
		{name: "handle plus url", body: `{"agentProxyId":"weather-agent","url":"http://weather-agent:9000"}`},
		{name: "handle plus auth", body: `{"agentProxyId":"weather-agent","auth":{"type":"none"}}`},
		{name: "handle plus null url", body: `{"agentProxyId":"weather-agent","url":null}`},
		{name: "handle plus null auth", body: `{"agentProxyId":"weather-agent","auth":null}`},
		{name: "all three fields", body: `{"agentProxyId":"weather-agent","url":"http://a:1","auth":{"type":"none"}}`},
		{name: "all three with nulls", body: `{"agentProxyId":"weather-agent","url":null,"auth":null}`},
		{name: "null handle", body: `{"agentProxyId":null}`},
		{name: "null url", body: `{"url":null}`},
		{name: "empty handle", body: `{"agentProxyId":""}`},
		{name: "blank handle", body: `{"agentProxyId":"   "}`},
		{name: "empty url", body: `{"url":""}`},
		{name: "short handle", body: `{"agentProxyId":"ab"}`},
		{name: "long handle", body: `{"agentProxyId":"` + strings.Repeat("a", 41) + `"}`},
		{name: "handle not a string", body: `{"agentProxyId":42}`},
		{name: "url not a string", body: `{"url":["http://a:1"]}`},
		{name: "unknown field", body: `{"url":"http://a:1","refresh":true}`},
		{name: "unknown field with handle", body: `{"agentProxyId":"weather-agent","cache":false}`},
		{name: "unknown auth field", body: `{"url":"http://a:1","auth":{"type":"api-key","header":"X","value":"k","scheme":"x"}}`},
		{name: "auth without type", body: `{"url":"http://a:1","auth":{"header":"X","value":"k"}}`},
		{name: "auth with unknown type", body: `{"url":"http://a:1","auth":{"type":"mutual-tls","header":"X","value":"k"}}`},
		{name: "auth with null type", body: `{"url":"http://a:1","auth":{"type":null}}`},
		{name: "body is an array", body: `[{"url":"http://a:1"}]`},
		{name: "body is a scalar", body: `"http://a:1"`},
		{name: "body is null", body: `null`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeAgentCardFetchRequest([]byte(tc.body))
			if err == nil {
				t.Fatalf("DecodeAgentCardFetchRequest() error = nil, want an error (decoded %+v)", got)
			}
			if got != nil {
				t.Errorf("DecodeAgentCardFetchRequest() returned %+v alongside an error, want nil", got)
			}
			// The message is handed straight to the client, so it must describe the
			// caller's own payload rather than a Go type or decoder internal.
			for _, leak := range []string{"json:", "struct", "reflect", "api.", "dto."} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("error message leaks an internal detail %q: %q", leak, err.Error())
				}
			}
		})
	}
}

// A handle at each end of the declared length range is accepted: the bound is
// inclusive, matching minLength/maxLength in the schema.
func TestDecodeAgentCardFetchRequest_HandleLengthBoundsAreInclusive(t *testing.T) {
	t.Parallel()

	for _, length := range []int{fetchAgentCardHandleMinLen, fetchAgentCardHandleMaxLen} {
		handle := strings.Repeat("a", length)
		got, err := DecodeAgentCardFetchRequest([]byte(`{"agentProxyId":"` + handle + `"}`))
		if err != nil {
			t.Fatalf("handle of length %d: error = %v, want nil", length, err)
		}
		if got.AgentProxyID != handle {
			t.Errorf("handle of length %d: AgentProxyID = %q, want %q", length, got.AgentProxyID, handle)
		}
	}
}

// auth type "none" is a complete, valid auth block: it is how a caller says the
// upstream needs no credential, and the fetch sends no header for it.
func TestDecodeAgentCardFetchRequest_AcceptsNoneAuth(t *testing.T) {
	t.Parallel()

	got, err := DecodeAgentCardFetchRequest([]byte(`{"url":"http://a:1","auth":{"type":"none"}}`))
	if err != nil {
		t.Fatalf("DecodeAgentCardFetchRequest() error = %v, want nil", err)
	}
	if got.Auth == nil || got.Auth.Type == nil || *got.Auth.Type != api.None {
		t.Fatalf("Auth = %+v, want type none", got.Auth)
	}
}
