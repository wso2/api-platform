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

package version

import (
	"fmt"
	"strconv"
	"strings"
)

// CompareSemver compares two semver strings (e.g. "v1.2.3").
// Returns 1 if a > b, -1 if a < b, 0 if equal.
// Strings that are not valid semver are compared lexicographically.
func CompareSemver(a, b string) int {
	aMajor, aMinor, aPatch, okA := parseSemver(a)
	bMajor, bMinor, bPatch, okB := parseSemver(b)
	if !okA || !okB {
		switch {
		case a > b:
			return 1
		case a < b:
			return -1
		default:
			return 0
		}
	}
	for _, pair := range [][2]int{{aMajor, bMajor}, {aMinor, bMinor}, {aPatch, bPatch}} {
		if pair[0] != pair[1] {
			if pair[0] > pair[1] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func parseSemver(v string) (int, int, int, bool) {
	raw := strings.TrimPrefix(v, "v")
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

// MajorVersion extracts the major version from a semver string.
// "v1.0.0" → "v1", "v0.3.1" → "v0", "v1" → "v1", "" → ""
func MajorVersion(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return trimmed
	}

	raw := strings.TrimPrefix(trimmed, "v")
	parts := strings.Split(raw, ".")
	if len(parts) == 0 {
		return trimmed
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return trimmed
	}

	return fmt.Sprintf("v%d", major)
}
