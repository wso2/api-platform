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

package goenv

import (
	"strings"
	"testing"
)

// gotoolchainValues returns every GOTOOLCHAIN= value present in env, in order.
func gotoolchainValues(env []string) []string {
	var vals []string
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, gotoolchainKey+"="); ok {
			vals = append(vals, v)
		}
	}
	return vals
}

func TestWithToolchain(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{name: "unset defaults to auto", in: []string{"HOME=/root"}, want: "auto"},
		{name: "local is overridden to auto", in: []string{"GOTOOLCHAIN=local"}, want: "auto"},
		{name: "empty is overridden to auto", in: []string{"GOTOOLCHAIN="}, want: "auto"},
		{name: "explicit auto is kept", in: []string{"GOTOOLCHAIN=auto"}, want: "auto"},
		{name: "pinned version is respected", in: []string{"GOTOOLCHAIN=go1.26.6"}, want: "go1.26.6"},
		{name: "path+auto is respected", in: []string{"GOTOOLCHAIN=go1.26.6+auto"}, want: "go1.26.6+auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gotoolchainValues(WithToolchain(tt.in))
			if len(got) != 1 {
				t.Fatalf("expected exactly one GOTOOLCHAIN entry, got %v", got)
			}
			if got[0] != tt.want {
				t.Errorf("GOTOOLCHAIN = %q, want %q", got[0], tt.want)
			}
		})
	}
}

// TestWithToolchainPreservesOtherVars ensures the helper only touches
// GOTOOLCHAIN and leaves the rest of the environment (e.g. the PWD entry
// os/exec derives from Cmd.Dir) intact.
func TestWithToolchainPreservesOtherVars(t *testing.T) {
	in := []string{"PWD=/some/dir", "GOTOOLCHAIN=local", "HOME=/root"}
	got := WithToolchain(in)

	found := map[string]bool{}
	for _, e := range got {
		found[e] = true
	}
	for _, want := range []string{"PWD=/some/dir", "HOME=/root"} {
		if !found[want] {
			t.Errorf("expected %q to be preserved, got %v", want, got)
		}
	}
}
