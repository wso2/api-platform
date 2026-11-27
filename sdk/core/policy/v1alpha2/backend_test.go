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

// A record names the cluster of the revision it describes. A clusterName that is not one of that
// Backend's own clusters makes the record unusable, so it can never steer a request elsewhere.
func TestLookupBackend_UsesTheRevisionClusterTheRecordNames(t *testing.T) {
	cases := map[string]struct {
		clusterName string
		want        string
		found       bool
	}{
		"revision cluster":        {"backend_tenant-a_3f9c2a1b0d4e", "backend_tenant-a_3f9c2a1b0d4e", true},
		"unrevisioned cluster":    {"backend_tenant-a", "backend_tenant-a", true},
		"no clusterName":          {"", "backend_tenant-a", true},
		"another Backend's":       {"backend_tenant-b_3f9c2a1b0d4e", "", false},
		"a prefix of another":     {"backend_tenant-a-x_3f9c", "", false},
		"an API cluster":          {"upstream_RestApi_x_main", "", false},
		"non-hex revision":        {"backend_tenant-a_zz", "", false},
		"empty revision":          {"backend_tenant-a_", "", false},
		"revision with separator": {"backend_tenant-a_3f9c_1", "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := NewLazyResourceStore()
			resource := backendResource("tenant-a", "/v1", nil)
			resource.Resource[BackendResourceKeyClusterName] = tc.clusterName
			if err := store.StoreResource(resource); err != nil {
				t.Fatal(err)
			}
			info, ok := store.LookupBackend("tenant-a")
			if ok != tc.found || info.ClusterName != tc.want {
				t.Fatalf("LookupBackend = (%q, %v), want (%q, %v)", info.ClusterName, ok, tc.want, tc.found)
			}
		})
	}
}

func TestIsValidBackendKey(t *testing.T) {
	valid := []string{"a", "dev", "gold-plus", "a1", "1", strings.Repeat("a", 63)}
	invalid := []string{"", "Dev", "-dev", "dev-", "dev.internal", "dev:80", "dev/x", "x@y", "dev_1",
		"dev internal", strings.Repeat("a", 64), "169.254.169.254"}
	for _, key := range valid {
		if !IsValidBackendKey(key) {
			t.Errorf("IsValidBackendKey(%q) = false, want true", key)
		}
	}
	for _, key := range invalid {
		if IsValidBackendKey(key) {
			t.Errorf("IsValidBackendKey(%q) = true, want false", key)
		}
	}
}

func TestParseBackendURLTemplate(t *testing.T) {
	valid := map[string]string{
		"https://url-{key}.url.fr":               "https://url-{key}.url.fr",
		"http://env-{key}.testbench:3000":        "http://env-{key}.testbench:3000",
		"https://{key}.internal":                 "https://{key}.internal",
		"https://{key}-api.example.com:8443/":    "https://{key}-api.example.com:8443",
		"http://a{key}b.svc.cluster.local":       "http://a{key}b.svc.cluster.local",
		"https://backend-{key}.internal.example": "https://backend-{key}.internal.example",
	}
	for raw, want := range valid {
		tmpl, ok := ParseBackendURLTemplate(raw)
		if !ok {
			t.Errorf("ParseBackendURLTemplate(%q) rejected it", raw)
			continue
		}
		if tmpl.String() != want {
			t.Errorf("ParseBackendURLTemplate(%q).String() = %q, want %q", raw, tmpl.String(), want)
		}
	}
	invalid := []string{
		"",
		"url-{key}.url.fr",                                    // no scheme
		"ftp://url-{key}.url.fr",                              // scheme
		"https://url.url.fr",                                  // no placeholder
		"https://url-{key}-{key}.url.fr",                      // two placeholders
		"https://url-{env}.url.fr",                            // another placeholder
		"https://url.{key}.fr",                                // not in the first label
		"https://{key}",                                       // no fixed label after it
		"https://{key}:8080",                                  // no fixed label after it
		"https://{key}.0.0.1",                                 // could become an IP address
		"https://{key}.10",                                    // numeric last label
		"https://url-{key}.url.fr/api",                        // path
		"https://url-{key}.url.fr?x=1",                        // query
		"https://url-{key}.url.fr#x",                          // fragment
		"https://user@url-{key}.url.fr",                       // userinfo
		"https://url-{key}.url.fr:0",                          // port out of range
		"https://url-{key}.url.fr:65536",                      // port out of range
		"https://url-{key}.url.fr:08",                         // port not canonical
		"https://url-{key}.url.fr:",                           // empty port
		"https://URL-{key}.url.fr",                            // upper case
		"https://-{key}.url.fr",                               // label starts with a hyphen
		"https://{key}-.url.fr",                               // label ends with a hyphen
		"https://url_{key}.url.fr",                            // underscore
		"https://url-{key}.url..fr",                           // empty label
		"https://url-{key}.-url.fr",                           // bad fixed label
		"https://[::1]-{key}.url.fr",                          // brackets
		"https://url-{key}.url.fr/{key}",                      // path, and a second placeholder
		"https://url-{key}}.url.fr",                           // stray brace
		"https://url-{{key}.url.fr",                           // stray brace
		"https://url-{key}.url.fr:80:80",                      // two ports
		"https://url-{key}.url.fr:http",                       // port not numeric
		"https://url-{key}.url.fr /x",                         // space
		"https:// url-{key}.url.fr",                           // space
		"https://url-{key}.url.fr\\@evil.io",                  // backslash
		"https://" + strings.Repeat("a", 63) + "{key}.url.fr", // no room for a key in the label
		"https://{key}." + strings.Repeat(strings.Repeat("a", 60)+".", 5) + "fr", // no room in the host
	}
	for _, raw := range invalid {
		if _, ok := ParseBackendURLTemplate(raw); ok {
			t.Errorf("ParseBackendURLTemplate(%q) accepted it", raw)
		}
	}
}

// A template whose one-character expansion just fits both DNS limits is valid.
func TestParseBackendURLTemplate_AtTheLimits(t *testing.T) {
	label := "https://" + strings.Repeat("a", 62) + "{key}.url.fr"
	if tmpl, ok := ParseBackendURLTemplate(label); !ok {
		t.Fatal("a 62-character prefix leaves room for a one-character key")
	} else if _, _, ok := tmpl.Expand("k"); !ok {
		t.Fatal("the one-character key expands")
	}
	// 1 + 1 + 4*61 + 7 = 253 characters with a one-character key.
	host := "https://{key}." + strings.Repeat(strings.Repeat("a", 60)+".", 4) + "abcdefg"
	if _, ok := ParseBackendURLTemplate(host); !ok {
		t.Fatal("a 253-character expansion is valid")
	}
}

func TestBackendURLTemplate_Expand(t *testing.T) {
	tmpl, ok := ParseBackendURLTemplate("https://url-{key}.url.fr")
	if !ok {
		t.Fatal("template rejected")
	}
	url, authority, ok := tmpl.Expand("dev")
	if !ok || url != "https://url-dev.url.fr" || authority != "url-dev.url.fr" {
		t.Fatalf("Expand(dev) = (%q, %q, %v)", url, authority, ok)
	}

	withPort, _ := ParseBackendURLTemplate("http://env-{key}.testbench:3000")
	if url, authority, ok := withPort.Expand("prod"); !ok || url != "http://env-prod.testbench:3000" ||
		authority != "env-prod.testbench:3000" {
		t.Fatalf("Expand(prod) = (%q, %q, %v)", url, authority, ok)
	}

	for _, key := range []string{"", "Dev", "dev.evil.io", "dev:1", "x@169.254.169.254", "dev/x"} {
		if _, _, ok := tmpl.Expand(key); ok {
			t.Errorf("Expand(%q) accepted an invalid key", key)
		}
	}
	// The key is a valid label alone, but the expanded label exceeds 63 characters.
	if _, _, ok := tmpl.Expand(strings.Repeat("a", 60)); ok {
		t.Error("Expand accepted a key whose label exceeds 63 characters")
	}
	if _, _, ok := (BackendURLTemplate{}).Expand("dev"); ok {
		t.Error("the zero template expanded")
	}
}

func TestLookupBackend_ParsesATemplatedRecord(t *testing.T) {
	store := NewLazyResourceStore()
	resource := backendResource("env-router", "", nil)
	resource.Resource[BackendResourceKeyURLTemplate] = "https://url-{key}.url.fr"
	if err := store.StoreResource(resource); err != nil {
		t.Fatal(err)
	}
	plain := backendResource("tenant-a", "/v1", nil)
	if err := store.StoreResource(plain); err != nil {
		t.Fatal(err)
	}

	info, ok := store.LookupBackend("env-router")
	if !ok || !info.Templated() || info.URLTemplate.String() != "https://url-{key}.url.fr" {
		t.Fatalf("LookupBackend(env-router) = (%+v, %v)", info, ok)
	}
	if info, ok := store.LookupBackend("tenant-a"); !ok || info.Templated() {
		t.Fatalf("a plain Backend is not templated: (%+v, %v)", info, ok)
	}
}

// A record whose urlTemplate does not parse is skipped rather than served as a plain Backend,
// so a request to it fails closed.
func TestLookupBackend_SkipsAMalformedTemplate(t *testing.T) {
	for name, value := range map[string]interface{}{
		"no placeholder": "https://url.url.fr",
		"bare host":      "https://{key}",
		"not a string":   42,
		"empty":          "",
	} {
		t.Run(name, func(t *testing.T) {
			store := NewLazyResourceStore()
			resource := backendResource("env-router", "", nil)
			resource.Resource[BackendResourceKeyURLTemplate] = value
			if err := store.StoreResource(resource); err != nil {
				t.Fatal(err)
			}
			if info, ok := store.LookupBackend("env-router"); ok {
				t.Fatalf("a malformed template resolved: %+v", info)
			}
		})
	}
}
