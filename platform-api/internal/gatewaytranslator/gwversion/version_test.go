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

package gwversion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	minVer := ParseVersion(MinGatewayV1Version)

	tests := []struct {
		version   string
		want      Version
		wantAtGTE bool // whether ParseVersion(version).AtLeast(minVer)
	}{
		{"1.2.0", Version{1, 2, 0}, true},
		{"v1.2.0", Version{1, 2, 0}, true},
		{"1.2.0-SNAPSHOT", Version{1, 2, 0}, true},
		{"1.3.0", Version{1, 3, 0}, true},
		{"2.0.0", Version{2, 0, 0}, true},
		{"1.1.9", Version{1, 1, 9}, false},
		{"1.1.0", Version{1, 1, 0}, false},
		{"1.0.0", Version{1, 0, 0}, false},
		{"", Version{1, 0, 0}, false},
		{"not-a-version", Version{1, 0, 0}, false},
		{"1.2", Version{1, 2, 0}, true},
		{"1", Version{1, 0, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := ParseVersion(tt.version)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantAtGTE, got.AtLeast(minVer))
			assert.Equal(t, !tt.wantAtGTE, got.Below(minVer))
		})
	}
}

// Parse must keep "unknown" apart from "old": the deploy path treats an
// unparseable version as a current build, never as 1.0.0.
func TestParse_DistinguishesUnknownFromOld(t *testing.T) {
	for _, raw := range []string{"", "   ", "it-e2e", "not-a-version", "v"} {
		_, ok := Parse(raw)
		assert.Falsef(t, ok, "%q must not parse", raw)
	}
	for _, raw := range []string{"1.0.0", "1.1", "v1.2.0-rc", "1"} {
		_, ok := Parse(raw)
		assert.Truef(t, ok, "%q must parse", raw)
	}
}

func TestVersion_AtLeast(t *testing.T) {
	assert.True(t, Version{1, 2, 0}.AtLeast(Version{1, 2, 0}))
	assert.True(t, Version{1, 3, 0}.AtLeast(Version{1, 2, 0}))
	assert.True(t, Version{2, 0, 0}.AtLeast(Version{1, 9, 9}))
	assert.False(t, Version{1, 1, 9}.AtLeast(Version{1, 2, 0}))
	assert.False(t, Version{1, 2, 0}.AtLeast(Version{1, 2, 1}))
}

func TestVersion_Below(t *testing.T) {
	assert.True(t, Version{1, 1, 9}.Below(Version{1, 2, 0}))
	assert.False(t, Version{1, 2, 0}.Below(Version{1, 2, 0}))
	assert.False(t, Version{1, 3, 0}.Below(Version{1, 2, 0}))
}

func TestVersion_String(t *testing.T) {
	assert.Equal(t, "2026.9.24", Version{2026, 9, 24}.String())
	assert.Equal(t, "1.2.0", ParseVersion("v1.2.0-rc1").String())
}

// LTS releases are semver; a first field that is a year names a date-based STS
// release, which the translator does not compare (issue #3681).
func TestVersion_IsLTS(t *testing.T) {
	for _, v := range []Version{{1, 0, 0}, {1, 2, 0}, {2, 0, 0}} {
		assert.Truef(t, v.IsLTS(), "%s is LTS", v)
	}
	assert.False(t, Version{2026, 5, 13}.IsLTS())
}
