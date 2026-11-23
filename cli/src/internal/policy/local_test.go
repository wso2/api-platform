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

package policy

import "testing"

func TestCountLabel(t *testing.T) {
	tests := []struct {
		n         int
		qualifier string
		want      string
	}{
		{0, "", "0 policies"},
		{1, "", "1 policy"},
		{2, "", "2 policies"},
		{1, "local", "1 local policy"},
		{3, "local", "3 local policies"},
	}
	for _, tt := range tests {
		if got := CountLabel(tt.n, tt.qualifier); got != tt.want {
			t.Errorf("CountLabel(%d, %q) = %q, want %q", tt.n, tt.qualifier, got, tt.want)
		}
	}
}
