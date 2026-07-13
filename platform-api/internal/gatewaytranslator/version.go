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

package gatewaytranslator

import (
	"strconv"
	"strings"
)

// Version is a parsed, comparable gateway semver — the "gateway target
// version" the deploy orchestration layer resolves from a gateway record.
// Empty or unparseable version strings are treated as 1.0.0 — the implicit
// version for gateways that predate version reporting.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion parses a gateway version string into a Version. Pre-release
// suffixes (e.g. "-SNAPSHOT", "-RC1") and a leading "v" are stripped before
// parsing. An empty or unparseable string returns Version{1, 0, 0}.
//
// Callers that must distinguish "genuinely old" from "not a semver at all"
// (e.g. dev/e2e builds versioned "it-e2e") should use parseVersion directly —
// see GatewayDataVersionForGateway.
func ParseVersion(s string) Version {
	v, ok := parseVersion(s)
	if !ok {
		return Version{1, 0, 0} // blank/unparseable → treat as oldest known
	}
	return v
}

// parseVersion parses a gateway version string into a Version, reporting
// whether the string carried a parseable semver. Pre-release suffixes and a
// leading "v" are stripped before parsing. Blank or unparseable strings return
// ok=false.
func parseVersion(s string) (Version, bool) {
	v := strings.TrimSpace(s)
	if v == "" {
		return Version{}, false
	}
	// Strip pre-release suffix
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v = v[:i]
	}
	// Strip leading "v"
	v = strings.TrimPrefix(strings.ToLower(v), "v")
	parts := strings.SplitN(v, ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return Version{}, false
	}
	return Version{major, minor, patch}, true
}

// AtLeast reports whether v is greater than or equal to o.
func (v Version) AtLeast(o Version) bool {
	if v.Major != o.Major {
		return v.Major > o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor > o.Minor
	}
	return v.Patch >= o.Patch
}

// Below reports whether v is strictly older than o.
func (v Version) Below(o Version) bool {
	return !v.AtLeast(o)
}
