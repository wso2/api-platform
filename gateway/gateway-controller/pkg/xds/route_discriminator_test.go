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

package xds

import (
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func TestHeaderMatchDiscriminator(t *testing.T) {
	// No headers -> empty discriminator (route key stays byte-identical to legacy format).
	if got := HeaderMatchDiscriminator(nil); got != "" {
		t.Errorf("nil headers: got %q, want empty", got)
	}
	if got := HeaderMatchDiscriminator([]models.RouteHeaderMatch{}); got != "" {
		t.Errorf("empty headers: got %q, want empty", got)
	}

	base := []models.RouteHeaderMatch{
		{Name: "Version", Value: "two", Type: "Exact"},
		{Name: "Color", Value: "orange", Type: "Exact"},
	}

	// Stable across calls.
	if HeaderMatchDiscriminator(base) != HeaderMatchDiscriminator(base) {
		t.Error("discriminator is not stable across calls for the same input")
	}

	// Order-independent (canonical sort).
	reordered := []models.RouteHeaderMatch{
		{Name: "Color", Value: "orange", Type: "Exact"},
		{Name: "Version", Value: "two", Type: "Exact"},
	}
	if HeaderMatchDiscriminator(base) != HeaderMatchDiscriminator(reordered) {
		t.Error("discriminator changed when header order changed; should be order-independent")
	}

	// Header-name case does not matter (Envoy matches case-insensitively).
	caseVariant := []models.RouteHeaderMatch{
		{Name: "version", Value: "two", Type: "Exact"},
		{Name: "COLOR", Value: "orange", Type: "Exact"},
	}
	if HeaderMatchDiscriminator(base) != HeaderMatchDiscriminator(caseVariant) {
		t.Error("discriminator changed with header-name case; should be case-insensitive on names")
	}

	// Empty Type defaults to Exact -> same as explicit Exact.
	defaultType := []models.RouteHeaderMatch{
		{Name: "Version", Value: "two", Type: ""},
		{Name: "Color", Value: "orange", Type: ""},
	}
	if HeaderMatchDiscriminator(base) != HeaderMatchDiscriminator(defaultType) {
		t.Error("empty Type should be treated as Exact")
	}

	// Distinct sets produce distinct discriminators.
	cases := map[string][]models.RouteHeaderMatch{
		"different value": {{Name: "Version", Value: "one", Type: "Exact"}, {Name: "Color", Value: "orange", Type: "Exact"}},
		"different name":  {{Name: "Version", Value: "two", Type: "Exact"}, {Name: "Shade", Value: "orange", Type: "Exact"}},
		"different type":  {{Name: "Version", Value: "two", Type: "RegularExpression"}, {Name: "Color", Value: "orange", Type: "Exact"}},
		"subset":          {{Name: "Version", Value: "two", Type: "Exact"}},
	}
	baseDisc := HeaderMatchDiscriminator(base)
	for name, hdrs := range cases {
		if HeaderMatchDiscriminator(hdrs) == baseDisc {
			t.Errorf("%s: discriminator collided with base set", name)
		}
	}
}

func TestGenerateRouteNameWithDiscriminator(t *testing.T) {
	method, ctx, ver, path, vhost := "GET", "/", "v1.0", "/", "example.com"

	// Empty discriminator must be byte-identical to the legacy 3-segment key.
	legacy := GenerateRouteName(method, ctx, ver, path, vhost)
	withEmpty := GenerateRouteNameWithDiscriminator(method, ctx, ver, path, vhost, "")
	if legacy != withEmpty {
		t.Errorf("empty discriminator changed the key: %q != %q", withEmpty, legacy)
	}

	// Non-empty discriminator appends a 4th segment; vhost stays at index 2.
	withDisc := GenerateRouteNameWithDiscriminator(method, ctx, ver, path, vhost, "abc123")
	want := legacy + "|abc123"
	if withDisc != want {
		t.Errorf("got %q, want %q", withDisc, want)
	}
}
