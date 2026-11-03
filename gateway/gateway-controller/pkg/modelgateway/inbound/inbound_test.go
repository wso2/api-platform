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

package inbound

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfiles(t *testing.T) {
	assert.Equal(t, []string{"anthropic", "gemini", "openai"}, Templates())

	openai, ok := Lookup("openai")
	require.True(t, ok)
	assert.Equal(t, []Operation{{Method: "POST", Path: "/chat/completions"}}, openai.Dispatch)
	assert.Equal(t, []string{"GET /models"}, openai.CatalogScopes())
	assert.Equal(t, FormatOpenAI, openai.Format)
	assert.False(t, openai.ModelInPath())

	anthropic, ok := Lookup("anthropic")
	require.True(t, ok)
	assert.Equal(t, []Operation{{Method: "POST", Path: "/v1/messages"}}, anthropic.Dispatch)
	assert.Equal(t, []string{"GET /v1/models"}, anthropic.CatalogScopes())
	assert.Equal(t, Location{Location: LocationPayload, Identifier: "$.stream"}, anthropic.Stream)

	gemini, ok := Lookup("gemini")
	require.True(t, ok)
	assert.True(t, gemini.ModelInPath())
	assert.ElementsMatch(t, []string{
		"POST /v1/models/{model}:generateContent", "POST /v1/models/{model}:streamGenerateContent",
		"POST /v1beta/models/{model}:generateContent", "POST /v1beta/models/{model}:streamGenerateContent",
	}, operationStrings(gemini.Dispatch))
	assert.ElementsMatch(t, []string{"GET /v1/models", "GET /v1beta/models"}, gemini.CatalogScopes())
	assert.Equal(t, LocationPathParam, gemini.Stream.Location)

	for _, unsupported := range []string{"", "awsbedrock", "azure-openai", "my-vendor"} {
		_, ok := Lookup(unsupported)
		assert.False(t, ok, unsupported)
	}
}

func operationStrings(ops []Operation) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, op.String())
	}
	return out
}

func TestValidateModelLocation(t *testing.T) {
	openai, _ := Lookup("openai")
	gemini, _ := Lookup("gemini")

	cases := []struct {
		name    string
		profile Profile
		loc     Location
		want    Location
		wantErr string
	}{
		{"top-level member", openai, Location{"payload", "$.model"}, Location{"payload", "$.model"}, ""},
		{"nested member refused", openai, Location{"payload", "$.request.model"}, Location{}, "top-level"},
		{"array refused", openai, Location{"payload", "$.models[0]"}, Location{}, "top-level"},
		{"path on a body format refused", openai, Location{"pathParam", "models/([a-z]+)"}, Location{}, "does not carry the model in its path"},
		{"path with one group", gemini, Location{"pathParam", `models/([a-zA-Z0-9.\-]+)`}, Location{"pathParam", `models/([a-zA-Z0-9.\-]+)`}, ""},
		{"path without a group", gemini, Location{"pathParam", `models/[a-z]+`}, Location{}, "exactly one capture group"},
		{"path with two groups", gemini, Location{"pathParam", `(models)/([a-z]+)`}, Location{}, "exactly one capture group"},
		{"path that does not compile", gemini, Location{"pathParam", `models/(`}, Location{}, "does not compile"},
		{"header lower-cased", openai, Location{"header", "X-Model"}, Location{"header", "x-model"}, ""},
		{"bad header", openai, Location{"header", "x model"}, Location{}, "not a valid header name"},
		{"query", openai, Location{"queryParam", "model"}, Location{"queryParam", "model"}, ""},
		{"bad query", openai, Location{"queryParam", "a&b"}, Location{}, "not a valid name"},
		{"unknown location", openai, Location{"cookie", "m"}, Location{}, "not one of"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.profile.ValidateModelLocation(tc.loc)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLocationFromParams(t *testing.T) {
	type named string
	loc, ok := LocationFromParams(map[string]interface{}{
		"requestModel": map[string]interface{}{"location": named("pathParam"), "identifier": "models/(x)"},
	}, "requestModel")
	require.True(t, ok)
	assert.Equal(t, Location{Location: "pathParam", Identifier: "models/(x)"}, loc)

	_, ok = LocationFromParams(map[string]interface{}{}, "requestModel")
	assert.False(t, ok)
	_, ok = LocationFromParams(map[string]interface{}{"requestModel": map[string]interface{}{"location": "payload"}}, "requestModel")
	assert.False(t, ok, "an identifier is required")
}
