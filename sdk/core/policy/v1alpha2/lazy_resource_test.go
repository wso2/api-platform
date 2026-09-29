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

import "testing"

func TestLazyResourceStore_VersionIncreasesOnEveryChange(t *testing.T) {
	store := NewLazyResourceStore()
	if got := store.Version(); got != 0 {
		t.Fatalf("new store Version() = %d, want 0", got)
	}

	resource := &LazyResource{ID: "a", ResourceType: "T", Resource: map[string]interface{}{}}
	last := store.Version()
	step := func(name string, change func() error) {
		t.Helper()
		if err := change(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := store.Version()
		if got <= last {
			t.Fatalf("%s: Version() = %d, want greater than %d", name, got, last)
		}
		last = got
	}

	step("ReplaceAll", func() error { return store.ReplaceAll([]*LazyResource{resource}) })
	step("ReplaceAll again with the same set", func() error { return store.ReplaceAll([]*LazyResource{resource}) })
	step("ReplaceAll with nothing", func() error { return store.ReplaceAll(nil) })
	step("StoreResource", func() error { return store.StoreResource(resource) })
	step("RemoveResourceByIDAndType", func() error { return store.RemoveResourceByIDAndType("a", "T") })
	step("StoreResource before RemoveResource", func() error { return store.StoreResource(resource) })
	step("RemoveResource", func() error { return store.RemoveResource("a") })
	step("StoreResource before RemoveResourcesByType", func() error { return store.StoreResource(resource) })
	step("RemoveResourcesByType", func() error { return store.RemoveResourcesByType("T") })
	step("ClearAll", store.ClearAll)
}

func TestLazyResourceStore_VersionUnchangedByReadsAndFailedRemovals(t *testing.T) {
	store := NewLazyResourceStore()
	if err := store.ReplaceAll([]*LazyResource{{ID: "a", ResourceType: "T"}}); err != nil {
		t.Fatal(err)
	}
	before := store.Version()

	_, _ = store.GetResourceByIDAndType("a", "T")
	_, _ = store.GetResourcesByType("T")
	_ = store.GetAllResources()
	if err := store.RemoveResourceByIDAndType("missing", "T"); err != ErrLazyResourceNotFound {
		t.Fatalf("RemoveResourceByIDAndType(missing) = %v, want ErrLazyResourceNotFound", err)
	}

	if got := store.Version(); got != before {
		t.Fatalf("Version() = %d after reads and a failed removal, want %d", got, before)
	}
}
