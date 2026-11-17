/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestParseEndpointTemplate(t *testing.T) {
	valid := map[string]struct{ published, sample, domain string }{
		"https://url-{key}.url.fr":            {"https://url-{key}.url.fr", "https://url-key.url.fr", "url.fr"},
		"http://env-{key}.testbench:3000":     {"http://env-{key}.testbench:3000", "http://env-key.testbench:3000", "testbench"},
		"https://{key}-api.example.com:8443/": {"https://{key}-api.example.com:8443", "https://key-api.example.com:8443", "example.com"},
		"https://{key}.internal":              {"https://{key}.internal", "https://key.internal", "internal"},
	}
	for raw, want := range valid {
		tmpl, err := ParseEndpointTemplate(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want.published, tmpl.String(), raw)
		assert.Equal(t, want.sample, tmpl.Sample(), raw)
		assert.Equal(t, want.domain, tmpl.Domain(), raw)
	}

	invalid := map[string]string{
		"url-{key}.url.fr":                                    "http:// or https://",
		"ftp://url-{key}.url.fr":                              "http:// or https://",
		"https://url-{key}-{key}.url.fr":                      "exactly once",
		"https://url-{env}.url.fr":                            "exactly once",
		"https://url-{key}}.url.fr":                           "exactly once",
		"https://url-{key}.url.fr/api":                        "no path",
		"https://url-{key}.url.fr?x=1":                        "no path",
		"https://url-{key}.url.fr#x":                          "no path",
		"https://user@url-{key}.url.fr":                       "no path",
		"https://url-{key}.url.fr:0":                          "port",
		"https://url-{key}.url.fr:08":                         "port",
		"https://url-{key}.url.fr:http":                       "port",
		"https://url.{key}.fr":                                "first label",
		"https://{key}":                                       "first label",
		"https://{key}:8080":                                  "first label",
		"https://URL-{key}.url.fr":                            "text around",
		"https://-{key}.url.fr":                               "text around",
		"https://{key}-.url.fr":                               "text around",
		"https://url_{key}.url.fr":                            "text around",
		"https://url-{key}.url..fr":                           "host labels",
		"https://url-{key}.URL.fr":                            "host labels",
		"https://{key}.0.0.1":                                 "contain a letter",
		"https://{key}.10":                                    "contain a letter",
		"https://" + strings.Repeat("a", 63) + "{key}.url.fr": "no room for a key",
		"https://{key}." + strings.Repeat(strings.Repeat("a", 60)+".", 5) + "fr": "no room for a key",
	}
	for raw, wantMessage := range invalid {
		_, err := ParseEndpointTemplate(raw)
		require.Error(t, err, raw)
		assert.Contains(t, err.Error(), wantMessage, raw)
	}
}

func backendWithEndpoints(urls ...string) api.BackendRequest {
	var backend api.BackendRequest
	for _, u := range urls {
		backend.Spec.Endpoints = append(backend.Spec.Endpoints, struct {
			Url    string `json:"url" yaml:"url"`
			Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
		}{Url: u})
	}
	return backend
}

func TestBackendEndpointTemplate(t *testing.T) {
	_, ok, err := BackendEndpointTemplate(backendWithEndpoints("https://a.example.com", "https://b.example.com"))
	assert.False(t, ok)
	assert.NoError(t, err)

	tmpl, ok, err := BackendEndpointTemplate(backendWithEndpoints("https://url-{key}.url.fr"))
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, "https://url-{key}.url.fr", tmpl.String())

	_, ok, err = BackendEndpointTemplate(backendWithEndpoints("https://url-{key}.url.fr", "https://b.example.com"))
	assert.True(t, ok)
	assert.ErrorContains(t, err, "only endpoint")

	_, ok, err = BackendEndpointTemplate(backendWithEndpoints("https://url.{key}.fr"))
	assert.True(t, ok)
	assert.Error(t, err)
}

// The sample copy never changes the stored Backend, whose template is what the revision hashes
// and what is published.
func TestWithSampleEndpoint_LeavesTheOriginalAlone(t *testing.T) {
	original := backendWithEndpoints("https://url-{key}.url.fr")
	sample := WithSampleEndpoint(original)
	assert.Equal(t, "https://url-key.url.fr", sample.Spec.Endpoints[0].Url)
	assert.Equal(t, "https://url-{key}.url.fr", original.Spec.Endpoints[0].Url)

	plain := backendWithEndpoints("https://a.example.com")
	assert.Equal(t, plain, WithSampleEndpoint(plain))
}
