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

// Package gwversion knows gateway release versions and nothing else: how to
// parse and compare the version string a gateway reports, and the release at
// which each gateway capability the control plane depends on first appeared.
//
// It is a leaf package with no project imports, so every other translator
// package can depend on it. It is also the only place in platform-api where a
// gateway version literal may be written — kind files and services refer to
// the Min* constants by name.
package gwversion

import (
	"strconv"
	"strings"
)

// Version is a parsed, comparable gateway version.
type Version struct {
	Major, Minor, Patch int
}

// stsMinMajor separates the two gateway release channels. LTS releases use
// semver ("1.2.0"); STS releases are named after their release date
// ("YYYY.MM.DD"), so their first field is a year. The channels do not share a
// version line: a date says nothing about which LTS capabilities a build has.
const stsMinMajor = 2000

// IsLTS reports whether v is an LTS (semver) release rather than a
// date-named STS release. Only LTS versions are compared against capability
// minimums; resolving STS capabilities is tracked in
// https://github.com/wso2/api-platform/issues/3681.
func (v Version) IsLTS() bool {
	return v.Major < stsMinMajor
}

// Parse parses a gateway version string, reporting whether it carried a
// parseable version at all. Pre-release suffixes ("-SNAPSHOT", "-rc1") and a
// leading "v" are stripped first. Blank or non-numeric strings (dev/e2e build
// tags such as "it-e2e") return ok=false; callers treat those as current builds.
func Parse(s string) (v Version, ok bool) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Version{}, false
	}
	if i := strings.IndexByte(raw, '-'); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimPrefix(strings.ToLower(raw), "v")
	parts := strings.SplitN(raw, ".", 3)
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

// ParseVersion is the lenient form of Parse: a blank or unparseable string is
// treated as 1.0.0, the oldest release, which predates version reporting. Use
// it for known-good inputs such as the Min* constants; use Parse where
// "unknown" must stay distinguishable from "old".
func ParseVersion(s string) Version {
	v, ok := Parse(s)
	if !ok {
		return Version{1, 0, 0}
	}
	return v
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

// String renders the version as "major.minor.patch".
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}
