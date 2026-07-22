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
	"os"
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

func TestEnv(t *testing.T) {
	tests := []struct {
		name  string
		set   bool
		value string
		want  string
	}{
		{name: "unset defaults to auto", set: false, want: "auto"},
		{name: "local is overridden to auto", set: true, value: "local", want: "auto"},
		{name: "empty is overridden to auto", set: true, value: "", want: "auto"},
		{name: "explicit auto is kept", set: true, value: "auto", want: "auto"},
		{name: "pinned version is respected", set: true, value: "go1.26.6", want: "go1.26.6"},
		{name: "path+auto is respected", set: true, value: "go1.26.6+auto", want: "go1.26.6+auto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv registers cleanup that restores the original value, so
			// mutating the var (including unsetting it) below is safe.
			t.Setenv(gotoolchainKey, tt.value)
			if !tt.set {
				os.Unsetenv(gotoolchainKey)
			}

			got := gotoolchainValues(Env())
			if len(got) != 1 {
				t.Fatalf("expected exactly one GOTOOLCHAIN entry, got %v", got)
			}
			if got[0] != tt.want {
				t.Errorf("GOTOOLCHAIN = %q, want %q", got[0], tt.want)
			}
		})
	}
}
