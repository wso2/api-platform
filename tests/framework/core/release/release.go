/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package release

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

// Version is a parsed MAJOR.MINOR.PATCH release version.
type Version struct {
	Major, Minor, Patch int
}

// String renders v as "MAJOR.MINOR.PATCH" — the form used both for the testbench's Docker image
// tags and for the suffix of this module's "tests/framework/vX.Y.Z" git tags.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Less reports whether v sorts before other under semver precedence: major, then minor, then patch.
func (v Version) Less(other Version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// ParseVersion parses a MAJOR.MINOR.PATCH string, with or without a leading "v". It reports
// false for anything else — a floating tag ("latest", "0", "0.4"), a malformed string, or an
// empty one — so a caller can filter a raw tag list without pre-validating it.
func ParseVersion(s string) (Version, bool) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	major, err1 := strconv.Atoi(m[1])
	minor, err2 := strconv.Atoi(m[2])
	patch, err3 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return Version{}, false
	}
	return Version{Major: major, Minor: minor, Patch: patch}, true
}

// ParseVersions parses every tag that matches ParseVersion, silently skipping the rest.
func ParseVersions(tags []string) []Version {
	var out []Version
	for _, tag := range tags {
		if v, ok := ParseVersion(tag); ok {
			out = append(out, v)
		}
	}
	return out
}

// Highest returns the highest of versions, and false if versions is empty.
func Highest(versions []Version) (Version, bool) {
	if len(versions) == 0 {
		return Version{}, false
	}
	highest := versions[0]
	for _, v := range versions[1:] {
		if highest.Less(v) {
			highest = v
		}
	}
	return highest, true
}

// NextVersion returns the version a release should cut when none is given explicitly: the
// highest existing release's major.minor with patch+1, or 0.0.1 when tags holds no release yet.
func NextVersion(tags []string) Version {
	highest, ok := Highest(ParseVersions(tags))
	if !ok {
		return Version{Major: 0, Minor: 0, Patch: 1}
	}
	return Version{Major: highest.Major, Minor: highest.Minor, Patch: highest.Patch + 1}
}

// FloatingTags decides which floating tags — "latest", the bare major, and "major.minor" — a
// release should receive, given every other known release tag. A floating tag moves only when
// released is the highest version within its scope: "major.minor" requires it to be the highest
// patch in its own minor line, so a deliberately backfilled, out-of-order patch (a backport)
// receives none of these; the bare major additionally requires the highest minor line within
// that major; "latest" requires the highest release overall. tags may or may not already
// contain released — either way gives the same result.
func FloatingTags(tags []string, released Version) []string {
	highestInMinor, highestInMajor, highestOverall := true, true, true
	for _, v := range ParseVersions(tags) {
		if !released.Less(v) {
			continue // v <= released (including v == released): can't disqualify released
		}
		highestOverall = false
		if v.Major == released.Major {
			highestInMajor = false
			if v.Minor == released.Minor {
				highestInMinor = false
			}
		}
	}
	var result []string
	if highestInMinor {
		result = append(result, fmt.Sprintf("%d.%d", released.Major, released.Minor))
	}
	if highestInMajor {
		result = append(result, strconv.Itoa(released.Major))
	}
	if highestOverall {
		result = append(result, "latest")
	}
	return result
}

// TaggedVersion pairs a parsed release version with an opaque identifier the caller acts on —
// a GHCR package-version ID in production, an arbitrary string in tests.
type TaggedVersion struct {
	Version Version
	ID      string
}

// FilterMinor returns the entries of all whose version is major.minor.
func FilterMinor(all []TaggedVersion, major, minor int) []TaggedVersion {
	var out []TaggedVersion
	for _, tv := range all {
		if tv.Version.Major == major && tv.Version.Minor == minor {
			out = append(out, tv)
		}
	}
	return out
}

// Retain returns which entries of line — one minor line's existing patches, including the one
// just released — to delete so at most keep survive: the newest keep are retained, and the
// rest are returned oldest-first for deletion. A non-positive keep retains nothing.
func Retain(line []TaggedVersion, keep int) []TaggedVersion {
	if keep < 0 {
		keep = 0
	}
	if len(line) <= keep {
		return nil
	}
	sorted := append([]TaggedVersion(nil), line...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version.Less(sorted[j].Version) })
	return sorted[:len(sorted)-keep]
}
