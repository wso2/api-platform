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

package webhook

import "testing"

// These vectors were produced by the API Portal's secretHash
// (portals/api-portal/src/services/webhooks/envelopeCrypto.js). If either side's algorithm or
// label changes, this test or the portal's envelopeCrypto.test.js fails, so the two cannot drift.
func TestSecretHash_MatchesAPIPortalVectors(t *testing.T) {
	vectors := []struct{ secret, want string }{
		{"whsec_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "32413deb36859f80321545a85378e517fad68ae583f7c3f3a0c6e50a1c96f824"},
		{"change-me-minimum-32-chars", "bc6bb059212d79fd2689e6fe2adf8cc5cf24abe211e0ddf44de1f9c590e1e69f"},
		{"ünïcödé-secret-🔑-0123456789abcdef", "588a57d77853c7938c575e0135da9842fe4ee03010b825c9d6d6d12b56ef4c38"},
	}
	for _, v := range vectors {
		if got := SecretHash(v.secret); got != v.want {
			t.Errorf("SecretHash(%q) = %s, want %s", v.secret, got, v.want)
		}
	}
}
