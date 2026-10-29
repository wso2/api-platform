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

package mcpserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIOutput_LeavesOutTheOpenAPIDocument(t *testing.T) {
	s, f := newTestServer(t)
	f.api = object{"id": "openai", "description": "old", "openapi": "openapi: 3.0.3 ..."}
	cs := connect(t, s, who(testOrg, "ap:llm_provider:manage"))

	res, text := call(t, cs, "get_api", map[string]any{"apiType": "llm-provider", "apiId": "openai"})
	require.False(t, res.IsError, text)
	assert.NotContains(t, text, "openapi")
	assert.Contains(t, f.api, "openapi", "the stored object is not changed")

	res, text = call(t, cs, "update_api", map[string]any{"apiType": "llm-provider", "apiId": "openai",
		"payload": map[string]any{"description": "new"}})
	require.False(t, res.IsError, text)
	assert.NotContains(t, text, "openapi")
	assert.Equal(t, "openapi: 3.0.3 ...", f.updated["openapi"], "the update still sends the whole object")
}

func TestUpdateAPI_NullRemovesAField(t *testing.T) {
	s, f := newTestServer(t)
	f.api = object{"id": "a1", "vhost": "a.example.com", "description": "old",
		"upstream": map[string]any{"main": map[string]any{"url": "http://x", "auth": map[string]any{"type": "api-key", "value": "{{ secret \"k\" }}"}}}}
	cs := connect(t, s, who(testOrg, "ap:agent_proxy:manage"))

	res, text := call(t, cs, "update_api", map[string]any{"apiType": "agent-proxy", "apiId": "a1",
		"payload": map[string]any{"vhost": nil, "upstream": map[string]any{"main": map[string]any{"auth": map[string]any{"value": nil}}}}})
	require.False(t, res.IsError, text)
	assert.NotContains(t, f.updated, "vhost")
	auth := f.updated["upstream"].(map[string]any)["main"].(map[string]any)["auth"].(map[string]any)
	assert.NotContains(t, auth, "value")
	assert.Equal(t, "api-key", auth["type"])
	assert.Equal(t, "old", f.updated["description"])
}

func TestUpdateAPI_RESTAPIDescriptionMatchesTheRESTAPI(t *testing.T) {
	s, f := newTestServer(t)
	current := func() object { return object{"id": "r1", "description": "old", "displayName": "R"} }
	update := func(apiType, scope string, description any) {
		t.Helper()
		f.api = current()
		cs := connect(t, s, who(testOrg, scope))
		res, text := call(t, cs, "update_api", map[string]any{"apiType": apiType, "apiId": "r1",
			"payload": map[string]any{"description": description}})
		require.False(t, res.IsError, text)
	}

	update("rest-api", "ap:rest_api:manage", nil)
	assert.NotContains(t, f.updated, "description", "null leaves it out, so the service keeps the stored one")

	update("rest-api", "ap:rest_api:manage", "")
	assert.Equal(t, "", f.updated["description"])

	update("mcp-proxy", "ap:mcp_proxy:manage", nil)
	assert.NotContains(t, f.updated, "description")

	update("rest-api", "ap:rest_api:manage", "new")
	assert.Equal(t, "new", f.updated["description"])
}

func TestUpdateAPI_RefusesAnIDChange(t *testing.T) {
	s, f := newTestServer(t)
	f.api = object{"id": "a1", "projectId": "default"}
	for _, tc := range []struct{ apiType, scope string }{
		{"rest-api", "ap:rest_api:manage"}, {"llm-provider", "ap:llm_provider:manage"}, {"llm-proxy", "ap:llm_proxy:manage"},
		{"mcp-proxy", "ap:mcp_proxy:manage"}, {"agent-proxy", "ap:agent_proxy:manage"},
	} {
		cs := connect(t, s, who(testOrg, tc.scope))
		for _, id := range []any{"other", ""} {
			for _, key := range []string{"id", "ID"} {
				res, text := call(t, cs, "update_api", map[string]any{"apiType": tc.apiType, "apiId": "a1", "payload": map[string]any{key: id}})
				assert.True(t, res.IsError, "%s %s %v", tc.apiType, key, id)
				assert.Contains(t, text, "id cannot change")
			}
		}
		res, text := call(t, cs, "update_api", map[string]any{"apiType": tc.apiType, "apiId": "a1", "payload": map[string]any{"id": "a1"}})
		assert.False(t, res.IsError, "the same id is fine: %s", text)
		for _, key := range []string{"id", "ID"} {
			f.updated = nil
			res, text = call(t, cs, "update_api", map[string]any{"apiType": tc.apiType, "apiId": "a1", "payload": map[string]any{key: nil}})
			assert.False(t, res.IsError, "%s %s null removes the id from the body: %s", tc.apiType, key, text)
			assert.NotContains(t, f.updated, "id")
			assert.NotContains(t, f.updated, "ID")
		}
	}
}

func TestMerge(t *testing.T) {
	current := map[string]any{"description": "old", "a": map[string]any{"b": 1, "c": 2}, "list": []any{1}}
	got := merge(current, map[string]any{
		"Description": "new",
		"A":           map[string]any{"B": nil, "d": map[string]any{"e": nil, "f": 3}},
		"list":        []any{2},
		"extra":       nil,
	})
	assert.Equal(t, map[string]any{
		"description": "new",
		"a":           map[string]any{"c": 2, "d": map[string]any{"f": 3}},
		"list":        []any{2},
	}, got)
	assert.Equal(t, "old", current["description"], "the current object is not changed")
}

func TestListAPIs_ShowsReadOnly(t *testing.T) {
	s, f := newTestServer(t)
	f.listed = []object{{"id": "g1", "displayName": "G", "readOnly": true}, {"id": "c1", "displayName": "C", "readOnly": false}}
	_, text := call(t, connect(t, s, who(testOrg, "ap:agent_proxy:read")), "list_apis", map[string]any{"apiType": "agent-proxy"})
	assert.Contains(t, text, `{"displayName":"G","id":"g1","readOnly":true}`)
	assert.NotContains(t, text, `"readOnly":false`)
}
