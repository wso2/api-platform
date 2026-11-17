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

func backendResource(name, basePath string, labels map[string]interface{}) *LazyResource {
	return &LazyResource{
		ID:           name,
		ResourceType: LazyResourceTypeBackend,
		Resource: map[string]interface{}{
			BackendResourceKeyName:        name,
			BackendResourceKeyClusterName: BackendClusterPrefix + name,
			BackendResourceKeyBasePath:    basePath,
			BackendResourceKeyLabels:      labels,
		},
	}
}

func TestIsValidBackendName(t *testing.T) {
	valid := []string{"abc", "tenant-a", "a1-b2-c3", strings.Repeat("a", 40)}
	invalid := []string{"", "ab", strings.Repeat("a", 41), "Tenant-A", "tenant_a", "-tenant", "tenant-",
		"tenant--a", "tenant.a", "tenant a", "backend_x", "../x"}
	for _, name := range valid {
		if !IsValidBackendName(name) {
			t.Errorf("IsValidBackendName(%q) = false, want true", name)
		}
	}
	for _, name := range invalid {
		if IsValidBackendName(name) {
			t.Errorf("IsValidBackendName(%q) = true, want false", name)
		}
	}
}

func TestLookupBackend_ResolvesPublishedRecord(t *testing.T) {
	store := NewLazyResourceStore()
	if err := store.StoreResource(backendResource("tenant-a", "/v1", map[string]interface{}{"team": "orders"})); err != nil {
		t.Fatal(err)
	}

	info, ok := store.LookupBackend("tenant-a")
	if !ok {
		t.Fatal("tenant-a not found")
	}
	if info.Name != "tenant-a" || info.ClusterName != "backend_tenant-a" || info.BasePath != "/v1" {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.Labels["team"] != "orders" {
		t.Fatalf("labels not parsed: %+v", info.Labels)
	}
	if _, ok := store.LookupBackend("tenant-b"); ok {
		t.Fatal("tenant-b must not resolve")
	}
}

func TestLookupBackend_FollowsStoreChanges(t *testing.T) {
	store := NewLazyResourceStore()
	if _, ok := store.LookupBackend("tenant-b"); ok {
		t.Fatal("empty store must not resolve")
	}

	// Added after the index was first built: visible on the next lookup.
	if err := store.StoreResource(backendResource("tenant-b", "/v2", nil)); err != nil {
		t.Fatal(err)
	}
	info, ok := store.LookupBackend("tenant-b")
	if !ok || info.BasePath != "/v2" {
		t.Fatalf("tenant-b after add: ok=%v info=%+v", ok, info)
	}

	// Updated in place.
	if err := store.StoreResource(backendResource("tenant-b", "/v3", nil)); err != nil {
		t.Fatal(err)
	}
	if info, _ := store.LookupBackend("tenant-b"); info.BasePath != "/v3" {
		t.Fatalf("tenant-b after update: %+v", info)
	}

	// Removed.
	if err := store.RemoveResourceByIDAndType("tenant-b", LazyResourceTypeBackend); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.LookupBackend("tenant-b"); ok {
		t.Fatal("tenant-b must not resolve after removal")
	}
}

func TestLookupBackend_IgnoresOtherTypesAndInvalidIDs(t *testing.T) {
	store := NewLazyResourceStore()
	// Both records must reach the store, so the checks below test the lookup's filtering rather
	// than a store that rejected them.
	if err := store.StoreResource(&LazyResource{ID: "tenant-a", ResourceType: "LlmProviderTemplate", Resource: map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreResource(backendResource("Bad_Name", "/x", nil)); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreResource(backendResource("tenant-ok", "/v1", nil)); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.LookupBackend("tenant-ok"); !ok {
		t.Fatal("positive control: a valid Backend in the same store must resolve")
	}

	if _, ok := store.LookupBackend("tenant-a"); ok {
		t.Fatal("a resource of another type must not resolve as a Backend")
	}
	if _, ok := store.LookupBackend("Bad_Name"); ok {
		t.Fatal("an invalid Backend id must never resolve")
	}
}

func TestLookupBackend_AcceptsStringLabelMap(t *testing.T) {
	store := NewLazyResourceStore()
	res := backendResource("tenant-c", "", nil)
	res.Resource[BackendResourceKeyLabels] = map[string]string{"region": "eu"}
	_ = store.StoreResource(res)

	info, ok := store.LookupBackend("tenant-c")
	if !ok || info.Labels["region"] != "eu" || info.BasePath != "" {
		t.Fatalf("ok=%v info=%+v", ok, info)
	}
}

func TestBackendInfo_MatchesLabels(t *testing.T) {
	info := BackendInfo{Labels: map[string]string{"team": "orders", "region": "eu"}}
	cases := []struct {
		selector map[string]string
		want     bool
	}{
		{nil, true},
		{map[string]string{}, true},
		{map[string]string{"team": "orders"}, true},
		{map[string]string{"team": "orders", "region": "eu"}, true},
		{map[string]string{"team": "billing"}, false},
		{map[string]string{"tier": "gold"}, false},
	}
	for _, c := range cases {
		if got := info.MatchesLabels(c.selector); got != c.want {
			t.Errorf("MatchesLabels(%v) = %v, want %v", c.selector, got, c.want)
		}
	}
	if (BackendInfo{}).MatchesLabels(map[string]string{"team": "orders"}) {
		t.Error("a Backend without labels must not match a non-empty selector")
	}
}

func TestLookupBackend_ConcurrentWithUpdates(t *testing.T) {
	store := NewLazyResourceStore()
	_ = store.StoreResource(backendResource("tenant-a", "/v1", nil))
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			_ = store.StoreResource(backendResource("tenant-z", "/z", nil))
			_ = store.RemoveResourceByIDAndType("tenant-z", LazyResourceTypeBackend)
		}
	}()
	for i := 0; i < 500; i++ {
		if _, ok := store.LookupBackend("tenant-a"); !ok {
			t.Fatal("tenant-a must stay resolvable while other Backends change")
		}
	}
	<-done
}
