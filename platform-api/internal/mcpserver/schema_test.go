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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAPISchema_EveryType(t *testing.T) {
	// Exact and ordered: the schema's own list first, then each allOf part's, without repeats.
	wantRequired := map[string][]string{
		"rest-api":     {"displayName", "context", "version", "projectId", "upstream"},
		"llm-provider": {"displayName", "version", "template", "upstream", "accessControl"},
		"llm-proxy":    {"displayName", "version", "projectId"},
		"mcp-proxy":    {"displayName", "version", "upstream"},
		"agent-proxy":  {"displayName", "version", "projectId", "upstream", "protocol", "a2a"},
	}
	require.Len(t, apiTypes, len(wantRequired))
	s, _ := newTestServer(t)
	cs := connect(t, s, who(testOrg)) // no scopes: the tool needs none
	for _, at := range apiTypes {
		res, text := call(t, cs, "get_api_schema", map[string]any{"apiType": at.name})
		require.False(t, res.IsError, "%s: %s", at.name, text)
		var out apiSchemaOut
		require.NoError(t, json.Unmarshal([]byte(text), &out))
		assert.Equal(t, at.name, out.APIType)
		assert.Equal(t, wantRequired[at.name], out.Required, at.name)
		assert.NotContains(t, text, "$ref", "%s: every reference is inlined", at.name)
	}
}

func TestInline_KeepsKeysNextToARef(t *testing.T) {
	all := map[string]any{
		"Target": map[string]any{"type": "object", "description": "generic"},
		"Bare":   map[string]any{"$ref": refPrefix + "Missing"},
	}
	field := map[string]any{"$ref": refPrefix + "Target", "description": "specific"}
	got := inline(all, field, map[string]bool{})
	assert.Equal(t, map[string]any{"type": "object", "description": "specific"}, got)

	// A target that is itself an unresolved $ref is the spec's own map: it must not be written to.
	bare := map[string]any{"$ref": refPrefix + "Bare", "description": "specific"}
	got = inline(all, bare, map[string]bool{})
	assert.Equal(t, map[string]any{"$ref": refPrefix + "Missing", "description": "specific"}, got)

	assert.Equal(t, map[string]any{"type": "object", "description": "generic"}, all["Target"])
	assert.Equal(t, map[string]any{"$ref": refPrefix + "Missing"}, all["Bare"])
}

func TestGetAPISchema_AgentProxyKeepsItsResilienceText(t *testing.T) {
	s, _ := newTestServer(t)
	cs := connect(t, s, who(testOrg))
	_, text := call(t, cs, "get_api_schema", map[string]any{"apiType": "agent-proxy"})
	assert.Contains(t, text, "Per-operation resilience values, overriding the Agent-wide defaults.")
}

func TestLoadAPISchemas_MissingSchemaIsRefused(t *testing.T) {
	spec := filepath.Join(t.TempDir(), "openapi.yaml")
	require.NoError(t, os.WriteFile(spec, []byte("openapi: 3.0.3\ncomponents:\n  schemas: {}\n"), 0o600))
	_, err := loadAPISchemas(spec)
	assert.ErrorContains(t, err, "schema CreateRESTAPIRequest for rest-api is not in the OpenAPI spec")
}
