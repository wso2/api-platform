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

package pdk

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// The pdk re-declares the artifact kinds because plugins live outside this module
// and cannot import internal packages. That makes them two copies of one value, so
// this pins them together: a kind renamed internally must be renamed here too, or
// every plugin silently starts naming a kind the platform no longer knows.
func TestKindConstantsMatchThePlatform(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exported string
		internal string
	}{
		{"REST API", KindRestAPI, constants.RestApi},
		{"MCP proxy", KindMCPProxy, constants.MCPProxy},
		{"LLM proxy", KindLLMProxy, constants.LLMProxy},
		{"LLM provider", KindLLMProvider, constants.LLMProvider},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.exported != tc.internal {
				t.Errorf("pdk has %q but the platform uses %q", tc.exported, tc.internal)
			}
		})
	}
}
