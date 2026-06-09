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

func TestGetReleaseName_shortNameUnchanged(t *testing.T) {
	got := GetReleaseName("platform-gw")
	want := "platform-gw-gw"
	if got != want {
		t.Fatalf("GetReleaseName() = %q, want %q", got, want)
	}
	if len(got) > maxHelmReleaseNameLen {
		t.Fatalf("release name length %d exceeds max %d", len(got), maxHelmReleaseNameLen)
	}
}

func TestGetReleaseName_longNameUsesStableHash(t *testing.T) {
	longName := "unresolved-gateway-with-one-attached-unresolved-route"
	got := GetReleaseName(longName)
	if len(got) > maxHelmReleaseNameLen {
		t.Fatalf("release name length %d exceeds max %d: %q", len(got), maxHelmReleaseNameLen, got)
	}
	if !strings.HasPrefix(got, helmReleaseHashPrefix) {
		t.Fatalf("expected hashed release prefix %q, got %q", helmReleaseHashPrefix, got)
	}
	if got2 := GetReleaseName(longName); got2 != got {
		t.Fatalf("expected stable mapping, got %q and %q", got, got2)
	}
}

func TestGetReleaseName_boundaryFitsSuffix(t *testing.T) {
	// 50 chars + "-gw" (3) = 53 (max allowed)
	name50 := strings.Repeat("a", 50)
	got := GetReleaseName(name50)
	want := name50 + helmReleaseNameSuffix
	if got != want {
		t.Fatalf("GetReleaseName() = %q, want %q", got, want)
	}
	if len(got) != maxHelmReleaseNameLen {
		t.Fatalf("len = %d, want %d", len(got), maxHelmReleaseNameLen)
	}
}

func TestGetReleaseName_boundaryExceedsSuffix(t *testing.T) {
	name51 := strings.Repeat("b", 51)
	got := GetReleaseName(name51)
	if len(got) > maxHelmReleaseNameLen {
		t.Fatalf("release name length %d exceeds max %d", len(got), maxHelmReleaseNameLen)
	}
	if strings.HasSuffix(got, helmReleaseNameSuffix) {
		t.Fatalf("expected hashed release name, got suffix form %q", got)
	}
}
