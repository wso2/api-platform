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

package policyv1alpha2

import (
	"strings"
	"testing"
)

func endpointResource(name string, labels map[string]interface{}) *LazyResource {
	return &LazyResource{
		ID:           name,
		ResourceType: LazyResourceTypeEndpoint,
		Resource: map[string]interface{}{
			"name":        name,
			"clusterName": EndpointClusterPrefix + name,
			"basePath":    "/v1",
			"labels":      labels,
		},
	}
}

func TestIsValidEndpointName(t *testing.T) {
	for name, want := range map[string]bool{
		"tenant-a":              true,
		"abc":                   true,
		"a1-b2-c3":              true,
		strings.Repeat("a", 40): true,
		"":                      false,
		"ab":                    false,
		strings.Repeat("a", 41): false,
		"Tenant-A":              false,
		"tenant_a":              false,
		"-tenant":               false,
		"tenant-":               false,
		"tenant--a":             false,
		"tenant a":              false,
		"upstream_main_host_80": false,
		"tenant-a\n":            false,
		"tenant-a,tenant-b":     false,
	} {
		if got := IsValidEndpointName(name); got != want {
			t.Errorf("IsValidEndpointName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestLookupEndpoint(t *testing.T) {
	store := GetLazyResourceStoreInstance()
	t.Cleanup(func() { _ = store.RemoveResourcesByType(LazyResourceTypeEndpoint) })

	if _, ok := LookupEndpoint("tenant-a"); ok {
		t.Fatal("LookupEndpoint found an Endpoint in an empty store")
	}

	if err := store.StoreResource(endpointResource("tenant-a", map[string]interface{}{"team": "orders"})); err != nil {
		t.Fatal(err)
	}
	info, ok := LookupEndpoint("tenant-a")
	if !ok {
		t.Fatal("LookupEndpoint did not find a stored Endpoint")
	}
	if info.Name != "tenant-a" || info.ClusterName != "endpoint_tenant-a" || info.BasePath != "/v1" {
		t.Errorf("unexpected EndpointInfo: %+v", info)
	}
	if info.Labels["team"] != "orders" {
		t.Errorf("labels = %v, want team=orders", info.Labels)
	}

	// The index follows the store without being told.
	if err := store.StoreResource(endpointResource("tenant-b", nil)); err != nil {
		t.Fatal(err)
	}
	if _, ok := LookupEndpoint("tenant-b"); !ok {
		t.Error("LookupEndpoint did not see an Endpoint added after the first lookup")
	}
	if err := store.RemoveResourceByIDAndType("tenant-a", LazyResourceTypeEndpoint); err != nil {
		t.Fatal(err)
	}
	if _, ok := LookupEndpoint("tenant-a"); ok {
		t.Error("LookupEndpoint still finds a removed Endpoint")
	}
}

func TestLookupEndpoint_IgnoresResourcesThatCouldMisroute(t *testing.T) {
	store := GetLazyResourceStoreInstance()
	t.Cleanup(func() { _ = store.RemoveResourcesByType(LazyResourceTypeEndpoint) })

	// A resource whose cluster is not the Endpoint's own reserved cluster.
	foreign := endpointResource("tenant-a", nil)
	foreign.Resource["clusterName"] = "upstream_main_internal_8080"
	// A resource of another type under a valid Endpoint name.
	other := endpointResource("tenant-c", nil)
	other.ResourceType = "LlmProviderTemplate"
	// A resource whose ID is not a well-formed Endpoint name.
	malformed := endpointResource("Tenant_B", nil)

	for _, resource := range []*LazyResource{foreign, other, malformed} {
		if err := store.StoreResource(resource); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = store.RemoveResourceByIDAndType("tenant-c", "LlmProviderTemplate") })

	for _, name := range []string{"tenant-a", "tenant-c", "Tenant_B"} {
		if info, ok := LookupEndpoint(name); ok {
			t.Errorf("LookupEndpoint(%q) = %+v, want not found", name, info)
		}
	}
}

func TestEndpointInfo_MatchesLabels(t *testing.T) {
	info := EndpointInfo{Labels: map[string]string{"team": "orders", "region": "eu"}}
	for _, tc := range []struct {
		name     string
		selector map[string]string
		want     bool
	}{
		{"empty selector matches any Endpoint", map[string]string{}, true},
		{"nil selector matches any Endpoint", nil, true},
		{"one matching label", map[string]string{"team": "orders"}, true},
		{"all matching labels", map[string]string{"team": "orders", "region": "eu"}, true},
		{"different value", map[string]string{"team": "billing"}, false},
		{"missing key", map[string]string{"tier": "gold"}, false},
		{"one of two differs", map[string]string{"team": "orders", "region": "us"}, false},
	} {
		if got := info.MatchesLabels(tc.selector); got != tc.want {
			t.Errorf("%s: MatchesLabels = %v, want %v", tc.name, got, tc.want)
		}
	}
	if (EndpointInfo{}).MatchesLabels(map[string]string{"team": "orders"}) {
		t.Error("an Endpoint with no labels matched a non-empty selector")
	}
}

// 905008 is on the wire in error bodies and analytics; a rename must be deliberate.
func TestFaultCodeEndpointUnavailableWireValue(t *testing.T) {
	if FaultCodeEndpointUnavailable != "905008" {
		t.Errorf("FaultCodeEndpointUnavailable = %q, want 905008", FaultCodeEndpointUnavailable)
	}
}
