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

package policyv1alpha2

import "testing"

// Basic and API key auth leave Scopes and Audience nil; a policy reading them must not panic.
func TestAuthContextNilScopesAndAudienceReadAsEmpty(t *testing.T) {
	for _, ac := range []*AuthContext{
		{Authenticated: true, AuthType: "basic", Subject: "alice"},
		{Authenticated: true, AuthType: "apikey"},
	} {
		if ac.Scopes["read"] {
			t.Errorf("%s: scope lookup on nil Scopes = true, want false", ac.AuthType)
		}
		for _, aud := range ac.Audience {
			t.Errorf("%s: ranging nil Audience yielded %q, want nothing", ac.AuthType, aud)
		}
		if len(ac.Scopes) != 0 || len(ac.Audience) != 0 {
			t.Errorf("%s: len(Scopes)=%d len(Audience)=%d, want 0 and 0", ac.AuthType, len(ac.Scopes), len(ac.Audience))
		}
	}
}
