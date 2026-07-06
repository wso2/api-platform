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

package deploymenttransform

import (
	"strconv"
	"strings"
)

// MinSplitPoliciesVersion is the first gateway release that understands
// globalPolicies/operationPolicies. Use this constant in tests and call sites
// instead of a raw string literal so a future version-boundary change is a
// one-place edit.
const MinSplitPoliciesVersion = "1.2.0"

// Version is a parsed, comparable gateway semver. Empty or unparseable version
// strings are treated as 1.0.0 — the implicit version for gateways that predate
// version reporting.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion parses a gateway version string into a Version. Pre-release
// suffixes (e.g. "-SNAPSHOT", "-RC1") and a leading "v" are stripped before
// parsing. An empty or unparseable string returns Version{1, 0, 0}.
func ParseVersion(s string) Version {
	v := strings.TrimSpace(s)
	if v == "" {
		return Version{1, 0, 0}
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
		return Version{1, 0, 0} // unparseable → treat as oldest known
	}
	return Version{major, minor, patch}
}

// GTE reports whether v is greater than or equal to o.
func (v Version) GTE(o Version) bool {
	if v.Major != o.Major {
		return v.Major > o.Major
	}
	if v.Minor != o.Minor {
		return v.Minor > o.Minor
	}
	return v.Patch >= o.Patch
}
