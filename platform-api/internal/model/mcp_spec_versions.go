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

// An MCP proxy declares the specification versions it serves either with the deprecated
// single-valued specVersion or with the specVersions list. The control plane stores only the
// list; the helper below is the one place that folds whichever form was supplied, and both the
// service layer and the gateway-push importer call it so the two write paths cannot diverge.

// EffectiveSpecVersions returns the MCP specification versions this configuration declares.
// SpecVersions is authoritative; SpecVersion is read only when it is absent. Returns nil when
// neither is set, leaving the choice of default to the caller.
func (c MCPProxyConfiguration) EffectiveSpecVersions() []string {
	return FoldSpecVersions(c.SpecVersions, c.SpecVersion)
}

// FoldSpecVersions folds the two declared forms into the canonical list, for callers holding
// the values before a MCPProxyConfiguration exists.
func FoldSpecVersions(versions []string, deprecated string) []string {
	if len(versions) > 0 {
		return versions
	}
	if deprecated != "" {
		return []string{deprecated}
	}
	return nil
}
