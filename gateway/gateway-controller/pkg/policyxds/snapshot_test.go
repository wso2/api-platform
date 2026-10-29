/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package policyxds

import "testing"

// CROSS-MODULE CONTRACT with the policy engine's internal/kernel UpstreamChainKey. The engine
// builds this exact key to look a per-attempt chain up, and the two are separate Go modules that
// cannot share the constant.
//
// A mismatch is SILENT: the engine finds no chain, the attempt runs unmodified, and the request
// still succeeds carrying whatever the previous attempt left in place. Nothing errors, so only
// pinning the literal on both sides catches a rename.
func TestUpstreamChainKey_CrossModuleContract(t *testing.T) {
	if UpstreamChainPrefix != "upstream:" {
		t.Fatalf("prefix = %q, want %q — the engine looks chains up under this exact string",
			UpstreamChainPrefix, "upstream:")
	}
	if got := UpstreamChainKey("upstream_LlmProxy_abc_anthropic"); got != "upstream:upstream_LlmProxy_abc_anthropic" {
		t.Fatalf("key = %q", got)
	}
}
