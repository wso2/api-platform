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

package xds

import "testing"

// parseDurationAllowZero must accept exactly what the CRD admission controller accepts
// (constants.ResilienceDurationPattern): single-unit durations including "0s" to disable, while
// rejecting compound, negative, and unitless values.
func TestParseDurationAllowZero_MatchesCRDPattern(t *testing.T) {
	ptr := func(s string) *string { return &s }

	t.Run("accepts single-unit and zero", func(t *testing.T) {
		for _, in := range []string{"30s", "500ms", "1m", "2h", "1.5s", "0s", "0ms"} {
			d, err := parseDurationAllowZero(ptr(in))
			if err != nil {
				t.Errorf("expected %q to be accepted, got error: %v", in, err)
				continue
			}
			if d == nil {
				t.Errorf("expected %q to yield a non-nil duration", in)
			}
		}
	})

	t.Run("nil and empty yield nil without error", func(t *testing.T) {
		for _, in := range []*string{nil, ptr(""), ptr("  ")} {
			d, err := parseDurationAllowZero(in)
			if err != nil || d != nil {
				t.Errorf("expected nil,nil for empty input, got %v,%v", d, err)
			}
		}
	})

	t.Run("rejects compound, negative, and unitless", func(t *testing.T) {
		for _, in := range []string{"1h30m", "1m30s", "-30s", "-5s", "30", "0", "15seconds", "abc"} {
			if _, err := parseDurationAllowZero(ptr(in)); err == nil {
				t.Errorf("expected %q to be rejected, but it was accepted", in)
			}
		}
	})
}
