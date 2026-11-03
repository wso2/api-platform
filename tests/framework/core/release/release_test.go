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
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in     string
		want   Version
		wantOK bool
	}{
		{"0.4.2", Version{0, 4, 2}, true},
		{"v0.4.2", Version{0, 4, 2}, true},
		{"10.20.30", Version{10, 20, 30}, true},
		{"latest", Version{}, false},
		{"0.4", Version{}, false},
		{"0", Version{}, false},
		{"", Version{}, false},
		{"0.4.2-rc1", Version{}, false},
		{"v0.4.2.1", Version{}, false},
		{"0.0.0", Version{0, 0, 0}, true},
		{"00.4.2", Version{}, false},
		{"0.04.2", Version{}, false},
		{"1.2.03", Version{}, false},
	}
	for _, c := range cases {
		got, ok := ParseVersion(c.in)
		require.Equal(t, c.wantOK, ok, c.in)
		if c.wantOK {
			require.Equal(t, c.want, got, c.in)
		}
	}
}

func TestParseVersionsSkipsMalformedEntries(t *testing.T) {
	got := ParseVersions([]string{"0.1.0", "latest", "0.4", "v0.4.2", "", "not-a-version"})
	require.Equal(t, []Version{{0, 1, 0}, {0, 4, 2}}, got)
}

func TestHighest(t *testing.T) {
	_, ok := Highest(nil)
	require.False(t, ok)

	got, ok := Highest([]Version{{0, 1, 0}, {0, 4, 2}, {0, 3, 9}, {1, 0, 0}, {0, 4, 9}})
	require.True(t, ok)
	require.Equal(t, Version{1, 0, 0}, got)
}

func TestNextVersionBootstrapsWhenNoReleaseExists(t *testing.T) {
	got, err := NextVersion(nil)
	require.NoError(t, err)
	require.Equal(t, Version{0, 0, 1}, got)

	got, err = NextVersion([]string{"latest", "0.4"})
	require.NoError(t, err)
	require.Equal(t, Version{0, 0, 1}, got)
}

func TestNextVersionBumpsThePatchOfTheHighestExistingRelease(t *testing.T) {
	got, err := NextVersion([]string{"v0.1.0", "v0.4.0", "v0.4.2", "v0.3.9", "latest", "0.4"})
	require.NoError(t, err)
	require.Equal(t, Version{0, 4, 3}, got)
}

func TestNextVersionRejectsAPatchThatCannotBeIncremented(t *testing.T) {
	_, err := NextVersion([]string{"0.4." + strconv.Itoa(math.MaxInt)})
	require.Error(t, err)
}

func TestFloatingTagsForTheHighestOverallRelease(t *testing.T) {
	got := FloatingTags([]string{"0.4.0"}, Version{0, 4, 1})
	require.ElementsMatch(t, []string{"0.4", "0", "latest"}, got)
}

func TestFloatingTagsIgnoresItsOwnTagInTheInputSet(t *testing.T) {
	// released already present in tags (e.g. the version was tagged before this call) — same result.
	got := FloatingTags([]string{"0.4.0", "0.4.1"}, Version{0, 4, 1})
	require.ElementsMatch(t, []string{"0.4", "0", "latest"}, got)
}

func TestFloatingTagsForANewMajorHighestOverallButNotYetTheOnlyMajor(t *testing.T) {
	// 0.9.0 exists; releasing 1.0.0 is the new overall highest and the first (so highest) 1.x.
	got := FloatingTags([]string{"0.9.0"}, Version{1, 0, 0})
	require.ElementsMatch(t, []string{"1.0", "1", "latest"}, got)
}

func TestFloatingTagsForAHigherMajorAlreadyPublishedSkipsLatest(t *testing.T) {
	// Releasing 0.5.0 while 1.0.0 already exists: highest within 0.x, but not overall.
	got := FloatingTags([]string{"1.0.0", "0.4.2"}, Version{0, 5, 0})
	require.ElementsMatch(t, []string{"0.5", "0"}, got)
}

func TestFloatingTagsForAnOlderMinorLineGetsOnlyItsOwnMinorLine(t *testing.T) {
	// A version in an older minor line (0.3.x) while 0.4.x exists.
	got := FloatingTags([]string{"0.4.2"}, Version{0, 3, 5})
	require.ElementsMatch(t, []string{"0.3"}, got)
}

func TestFloatingTagsForAnOutOfOrderPatchBackfillGetsNoFloatingTag(t *testing.T) {
	// 0.4.2 already exists; backfilling the skipped 0.4.1 must not move :0.4 backward.
	got := FloatingTags([]string{"0.4.0", "0.4.2"}, Version{0, 4, 1})
	require.Empty(t, got)
}

func TestFloatingTagsWhenNothingElseExists(t *testing.T) {
	got := FloatingTags(nil, Version{0, 0, 1})
	require.ElementsMatch(t, []string{"0.0", "0", "latest"}, got)
}
