/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package model

import (
	"reflect"
	"testing"
)

func TestEffectiveSpecVersions(t *testing.T) {
	tests := []struct {
		name string
		cfg  MCPProxyConfiguration
		want []string
	}{
		{name: "neither form declared", cfg: MCPProxyConfiguration{}, want: nil},
		{
			name: "deprecated scalar, as a pre-change row carries it",
			cfg:  MCPProxyConfiguration{SpecVersion: "2025-06-18"},
			want: []string{"2025-06-18"},
		},
		{
			name: "canonical list",
			cfg:  MCPProxyConfiguration{SpecVersions: []string{"2025-06-18", "2026-07-28"}},
			want: []string{"2025-06-18", "2026-07-28"},
		},
		{
			name: "the list wins over a scalar left on an old row",
			cfg:  MCPProxyConfiguration{SpecVersion: "2025-06-18", SpecVersions: []string{"2026-07-28"}},
			want: []string{"2026-07-28"},
		},
		{
			name: "an empty list falls through to the deprecated scalar",
			cfg:  MCPProxyConfiguration{SpecVersion: "2025-11-25", SpecVersions: []string{}},
			want: []string{"2025-11-25"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.EffectiveSpecVersions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EffectiveSpecVersions() = %v, want %v", got, tt.want)
			}
		})
	}
}
