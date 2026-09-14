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

// An MCP proxy declares the specification versions it serves either with the deprecated
// single-valued `specVersion` or with the `specVersions` list. Every consumer should read
// them through the helper below so it does not need to know which form was authored.

// EffectiveSpecVersions returns the MCP specification versions this proxy declares.
// specVersions is authoritative; specVersion is read only when it is absent. Returns nil
// when neither is set, leaving the choice of default to the caller.
func (s MCPProxyConfigData) EffectiveSpecVersions() []string {
	if s.SpecVersions != nil && len(*s.SpecVersions) > 0 {
		return *s.SpecVersions
	}
	if s.SpecVersion != nil && *s.SpecVersion != "" {
		return []string{*s.SpecVersion}
	}
	return nil
}
