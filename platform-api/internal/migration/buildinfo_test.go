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

package migration

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
)

func TestBuildInfo_StampedValuesWin(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = oldV, oldC, oldD })

	Version, Commit, BuildDate = "v1.2.3", "0123456789abcdef0123", "2026-10-03T00:00:00Z"
	got := BuildInfo()
	want := "v1.2.3 (commit 0123456789ab, built 2026-10-03T00:00:00Z, " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH + ")"
	if got != want {
		t.Fatalf("BuildInfo() = %q, want %q", got, want)
	}
}

func TestBuildInfo_UnstampedNeverEmpty(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = oldV, oldC, oldD })

	Version, Commit, BuildDate = "", "", ""
	got := BuildInfo()
	for _, must := range []string{"(commit ", ", built ", runtime.GOOS + "/" + runtime.GOARCH} {
		if !strings.Contains(got, must) {
			t.Fatalf("BuildInfo() = %q, missing %q", got, must)
		}
	}
}

func TestParseFlags_VersionFlag(t *testing.T) {
	_, err := ParseFlags([]string{"--version"}, io.Discard)
	if !errors.Is(err, ErrVersionRequested) {
		t.Fatalf("ParseFlags(--version) err = %v, want ErrVersionRequested", err)
	}
}
