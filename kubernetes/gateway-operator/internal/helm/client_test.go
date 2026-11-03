/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"strings"
	"testing"
)

// chartServiceName mirrors the gateway chart's fullname helper and the longest
// suffix it adds to name a Service, so these tests measure the name the API
// server actually sees rather than the release name alone. Getting this wrong is
// what let an over-long Service through: the helper inserts the chart's own name
// unless the release name already contains it.
func chartServiceName(releaseName string) string {
	fullname := releaseName
	if !strings.Contains(releaseName, helmChartName) {
		fullname = releaseName + "-" + helmChartName
	}
	return fullname + runtimeServiceSuffix
}

func TestGetReleaseName_shortNameUnchanged(t *testing.T) {
	got := GetReleaseName("platform-gw")
	want := "platform-gw-gw"
	if got != want {
		t.Fatalf("GetReleaseName() = %q, want %q", got, want)
	}
}

func TestGetReleaseName_longNameUsesStableHash(t *testing.T) {
	longName := "unresolved-gateway-with-one-attached-unresolved-route"
	got := GetReleaseName(longName)
	if !strings.HasPrefix(got, helmReleaseHashPrefix) {
		t.Fatalf("expected hashed release prefix %q, got %q", helmReleaseHashPrefix, got)
	}
	if got2 := GetReleaseName(longName); got2 != got {
		t.Fatalf("expected stable mapping, got %q and %q", got, got2)
	}
}

// TestGetReleaseName_boundaryDependsOnChartName pins the part that was wrong: a
// name carrying the chart's own name keeps the readable form for longer, because
// the chart does not have to add it back.
func TestGetReleaseName_boundaryDependsOnChartName(t *testing.T) {
	for _, tc := range []struct {
		label      string
		filler     string
		wantBudget int
	}{
		{"without the chart name", "a", 63 - len(runtimeServiceSuffix) - 1 - len(helmChartName)},
		{"with the chart name", helmChartName, 63 - len(runtimeServiceSuffix)},
	} {
		t.Run(tc.label, func(t *testing.T) {
			atLimit := (strings.Repeat(tc.filler, 64))[:tc.wantBudget-len(helmReleaseNameSuffix)]
			if got := GetReleaseName(atLimit); got != atLimit+helmReleaseNameSuffix {
				t.Fatalf("a name at the budget should keep the readable form, got %q", got)
			}
			over := (strings.Repeat(tc.filler, 64))[:tc.wantBudget-len(helmReleaseNameSuffix)+1]
			if got := GetReleaseName(over); strings.HasSuffix(got, helmReleaseNameSuffix) {
				t.Fatalf("a name past the budget should hash, got %q", got)
			}
		})
	}
}

// TestGetReleaseName_derivedServiceNamesFit is the invariant the bound exists for:
// whatever GetReleaseName returns, the Service the chart derives from it must stay
// within 63. An overflow is not rejected by Helm — the Service is rejected by the
// API server later, and the gateway runs without one.
func TestGetReleaseName_derivedServiceNamesFit(t *testing.T) {
	// Both shapes matter: a name containing the chart name takes the short path
	// through the fullname helper, a name without it takes the long one.
	for _, filler := range []string{"a", helmChartName, "my-gateway-"} {
		for n := 1; n <= 120; n++ {
			gatewayName := (strings.Repeat(filler, 128))[:n]
			service := chartServiceName(GetReleaseName(gatewayName))
			if len(service) > maxDNS1123Label {
				t.Fatalf("gateway name %q (%d chars) yields Service %q (%d chars), over the %d limit",
					gatewayName, n, service, len(service), maxDNS1123Label)
			}
		}
	}
}

// TestLegacyReleaseName_onlyWhenTheReadableNameMoved covers the three shapes a
// gateway can be in, because cleaning up the wrong one would uninstall a release
// that is still serving traffic.
func TestLegacyReleaseName_onlyWhenTheReadableNameMoved(t *testing.T) {
	t.Run("a name still on its readable form has no legacy release", func(t *testing.T) {
		if got := LegacyReleaseName("platform"); got != "" {
			t.Fatalf("LegacyReleaseName() = %q, want %q", got, "")
		}
	})

	t.Run("a name that moved to the hash reports the readable form it left", func(t *testing.T) {
		// Past the 39 budget a name without the chart name gets, inside Helm's 53.
		gatewayName := strings.Repeat("a", 45)
		got := LegacyReleaseName(gatewayName)
		if want := gatewayName + helmReleaseNameSuffix; got != want {
			t.Fatalf("LegacyReleaseName() = %q, want %q", got, want)
		}
		if GetReleaseName(gatewayName) == got {
			t.Fatal("the legacy name must not be the name in use")
		}
	})

	t.Run("a name Helm never accepted has no legacy release", func(t *testing.T) {
		// Too long for Helm at any bound, so it was always on the hash.
		if got := LegacyReleaseName(strings.Repeat("a", 60)); got != "" {
			t.Fatalf("LegacyReleaseName() = %q, want %q", got, "")
		}
	})
}

// TestLegacyReleaseName_neverNamesTheReleaseInUse is the safety property: whatever
// the gateway name, cleanup must never target the release the gateway is running.
func TestLegacyReleaseName_neverNamesTheReleaseInUse(t *testing.T) {
	for _, filler := range []string{"a", helmChartName, "my-gateway-"} {
		for n := 1; n <= 120; n++ {
			gatewayName := (strings.Repeat(filler, 128))[:n]
			if legacy := LegacyReleaseName(gatewayName); legacy != "" && legacy == GetReleaseName(gatewayName) {
				t.Fatalf("gateway name %q (%d chars): legacy name equals the release in use (%q)",
					gatewayName, n, legacy)
			}
		}
	}
}
