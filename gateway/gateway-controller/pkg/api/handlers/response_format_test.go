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

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// JSON stays the default for every Accept header that does not strictly
// prefer YAML, because the operator sends no Accept at all and the CLI sends
// application/json.
func TestWriteResource_ContentNegotiation(t *testing.T) {
	tests := []struct {
		name     string
		accept   []string // nil: header absent
		wantYAML bool
	}{
		{name: "no Accept header"},
		{name: "wildcard", accept: []string{"*/*"}},
		{name: "application wildcard ties", accept: []string{"application/*"}},
		{name: "application/json", accept: []string{"application/json"}},
		{name: "application/yaml", accept: []string{"application/yaml"}, wantYAML: true},
		{name: "text/yaml with q", accept: []string{"text/yaml;q=0.9"}, wantYAML: true},
		{name: "application/x-yaml", accept: []string{"application/x-yaml"}, wantYAML: true},
		{name: "text/x-yaml, case-insensitive", accept: []string{"Text/X-YAML"}, wantYAML: true},
		{name: "yaml excluded by q=0", accept: []string{"application/yaml;q=0"}},
		{name: "yaml q=0 beats wildcard", accept: []string{"application/yaml;q=0, */*"}},
		{name: "tie goes to JSON", accept: []string{"application/json, application/yaml"}},
		{name: "JSON preferred by q", accept: []string{"application/yaml;q=0.5, application/json"}},
		{name: "YAML preferred by q", accept: []string{"application/json;q=0.5, application/yaml"}, wantYAML: true},
		{name: "YAML over wildcard fallback", accept: []string{"application/yaml, */*;q=0.1"}, wantYAML: true},
		{name: "YAML in a second header line", accept: []string{"text/html", "application/yaml"}, wantYAML: true},
		{name: "garbage header", accept: []string{"garbage"}},
		{name: "unparseable q", accept: []string{"application/yaml;q=abc"}},
		{name: "out-of-range q", accept: []string{"application/yaml;q=2"}},
		{name: "unsupported type falls back to JSON, no 406", accept: []string{"text/html"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/rest-apis/x", nil)
			for _, v := range tt.accept {
				r.Header.Add("Accept", v)
			}
			assert.Equal(t, tt.wantYAML, wantsYAML(r))

			w := httptest.NewRecorder()
			writeResource(w, r, http.StatusOK, map[string]any{"kind": "RestApi"})
			require.Equal(t, http.StatusOK, w.Code)
			if tt.wantYAML {
				assert.Equal(t, "application/yaml", w.Header().Get("Content-Type"))
				assert.Equal(t, "Accept", w.Header().Get("Vary"))
				assert.Equal(t, "kind: RestApi\n", w.Body.String())
				return
			}
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.JSONEq(t, `{"kind":"RestApi"}`, w.Body.String())
		})
	}
}

// The YAML body is block style, keeps the JSON key order, and quotes every
// string that would otherwise read back as a different type.
func TestJSONToYAML_BlockStyleAndQuoting(t *testing.T) {
	out, err := jsonToYAML(map[string]any{
		"spec": map[string]any{
			"version": "1.0",
			"enabled": "true",
			"empty":   "",
			"nested":  []any{map[string]any{"name": "a"}},
			"count":   3,
		},
	})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "{", "flow-style mapping leaked into the YAML body:\n%s", out)

	var back map[string]any
	require.NoError(t, yaml.Unmarshal(out, &back))
	spec := back["spec"].(map[string]any)
	assert.Equal(t, "1.0", spec["version"])
	assert.Equal(t, "true", spec["enabled"])
	assert.Equal(t, "", spec["empty"])
	assert.Equal(t, 3, spec["count"])
	assert.Equal(t, []any{map[string]any{"name": "a"}}, spec["nested"])
}

// A value JSON cannot encode gets the same 500 WriteJSON gives.
func TestWriteResource_EncodeFailure(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/rest-apis/x", nil)
	r.Header.Set("Accept", "application/yaml")
	w := httptest.NewRecorder()

	writeResource(w, r, http.StatusOK, map[string]any{"bad": make(chan int)})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotEqual(t, "application/yaml", w.Header().Get("Content-Type"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body["code"])
}
