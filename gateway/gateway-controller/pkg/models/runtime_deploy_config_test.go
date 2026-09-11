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

package models

import "testing"

func TestMetadataAnalyticsProjectRef(t *testing.T) {
	t.Run("prefers project handle for analytics", func(t *testing.T) {
		md := Metadata{
			ProjectID:     "019feb20-bd8f-74f1-9489-8814a129cd80",
			ProjectHandle: "new-project",
		}
		if got := md.AnalyticsProjectRef(); got != "new-project" {
			t.Fatalf("AnalyticsProjectRef() = %q, want handle", got)
		}
	})

	t.Run("falls back to project id when handle unset", func(t *testing.T) {
		md := Metadata{ProjectID: "019feb20-bd8f-74f1-9489-8814a129cd80"}
		if got := md.AnalyticsProjectRef(); got != md.ProjectID {
			t.Fatalf("AnalyticsProjectRef() = %q, want project id", got)
		}
	})

	t.Run("trims padded handle", func(t *testing.T) {
		md := Metadata{
			ProjectID:     "019feb20-bd8f-74f1-9489-8814a129cd80",
			ProjectHandle: "  new-project  ",
		}
		if got := md.AnalyticsProjectRef(); got != "new-project" {
			t.Fatalf("AnalyticsProjectRef() = %q, want trimmed handle", got)
		}
	})

	t.Run("whitespace-only handle falls back to project id", func(t *testing.T) {
		md := Metadata{
			ProjectID:     "019feb20-bd8f-74f1-9489-8814a129cd80",
			ProjectHandle: " \t ",
		}
		if got := md.AnalyticsProjectRef(); got != md.ProjectID {
			t.Fatalf("AnalyticsProjectRef() = %q, want project id fallback", got)
		}
	})
}
