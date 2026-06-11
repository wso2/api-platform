/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package controller

import "testing"

// TestIntersectHostname covers the Gateway-API hostname intersection rules exercised by the
// HTTPRouteHostnameIntersection conformance test (listeners very.specific.com, *.wildcard.io,
// *.anotherwildcard.io).
func TestIntersectHostname(t *testing.T) {
	cases := []struct {
		route, listener string
		want            string
		ok              bool
	}{
		// exact == exact
		{"very.specific.com", "very.specific.com", "very.specific.com", true},
		// specific route under wildcard listener -> keep specific (route may be multi-label)
		{"foo.wildcard.io", "*.wildcard.io", "foo.wildcard.io", true},
		{"foo.bar.wildcard.io", "*.wildcard.io", "foo.bar.wildcard.io", true},
		// bare apex does NOT match the wildcard
		{"wildcard.io", "*.wildcard.io", "", false},
		// wildcard route narrowed by specific listener -> keep the specific listener host
		{"*.specific.com", "very.specific.com", "very.specific.com", true},
		// wildcard route that does NOT cover the specific listener host
		{"*.specific.com", "other.example.com", "", false},
		// both wildcards equal
		{"*.anotherwildcard.io", "*.anotherwildcard.io", "*.anotherwildcard.io", true},
		// decoy hostnames that match no listener
		{"non.matching.com", "very.specific.com", "", false},
		{"non.matching.com", "*.wildcard.io", "", false},
		{"*.nonmatchingwildcard.io", "*.wildcard.io", "", false},
		// empty listener hostname (unspecified listener) accepts the route hostname as-is
		{"first.com", "", "first.com", true},
		{"*.anotherwildcard.io", "", "*.anotherwildcard.io", true},
	}
	for _, c := range cases {
		got, ok := intersectHostname(c.route, c.listener)
		if got != c.want || ok != c.ok {
			t.Errorf("intersectHostname(%q, %q) = (%q, %v); want (%q, %v)",
				c.route, c.listener, got, ok, c.want, c.ok)
		}
	}
}

func TestHostnameMatchesWildcard(t *testing.T) {
	cases := []struct {
		host, wildcard string
		want           bool
	}{
		{"foo.wildcard.io", "*.wildcard.io", true},
		{"foo.bar.wildcard.io", "*.wildcard.io", true},
		{"wildcard.io", "*.wildcard.io", false},
		{"foo.other.io", "*.wildcard.io", false},
		{"very.specific.com", "*.specific.com", true},
	}
	for _, c := range cases {
		if got := hostnameMatchesWildcard(c.host, c.wildcard); got != c.want {
			t.Errorf("hostnameMatchesWildcard(%q, %q) = %v; want %v", c.host, c.wildcard, got, c.want)
		}
	}
}
