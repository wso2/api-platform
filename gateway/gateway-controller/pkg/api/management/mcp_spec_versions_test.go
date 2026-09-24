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

package management

import (
	"reflect"
	"testing"
)

func TestEffectiveSpecVersions(t *testing.T) {
	tests := []struct {
		name         string
		specVersion  *string
		specVersions *[]string
		want         []string
	}{
		{
			name: "neither form declared",
			want: nil,
		},
		{
			name:        "deprecated single version",
			specVersion: Ptr("2025-06-18"),
			want:        []string{"2025-06-18"},
		},
		{
			name:        "deprecated single version, empty",
			specVersion: Ptr(""),
			want:        nil,
		},
		{
			name:         "version list",
			specVersions: &[]string{"2025-06-18", "2026-07-28"},
			want:         []string{"2025-06-18", "2026-07-28"},
		},
		{
			name:         "empty version list falls through to the deprecated field",
			specVersion:  Ptr("2025-11-25"),
			specVersions: &[]string{},
			want:         []string{"2025-11-25"},
		},
		{
			name:         "the list wins over the deprecated field",
			specVersion:  Ptr("2025-06-18"),
			specVersions: &[]string{"2026-07-28"},
			want:         []string{"2026-07-28"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := MCPProxyConfigData{SpecVersion: tt.specVersion, SpecVersions: tt.specVersions}
			if got := spec.EffectiveSpecVersions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EffectiveSpecVersions() = %v, want %v", got, tt.want)
			}
		})
	}
}
