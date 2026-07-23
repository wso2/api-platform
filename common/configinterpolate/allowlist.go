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

package configinterpolate

import (
	"os"
	"strings"
)

// EnvFileSourceAllowlist is the environment variable, shared across all components,
// that overrides the {{ file }} source-directory allowlist. It is read directly
// (not through any koanf prefix mapping) because the allowlist gates config
// interpolation itself and so cannot live in config.toml (chicken-and-egg).
const EnvFileSourceAllowlist = "APIP_CONFIG_FILE_SOURCE_ALLOWLIST"

// ResolveAllowlist returns the effective file-source allowlist for a component.
// When EnvFileSourceAllowlist is set to a non-empty value, its comma-separated
// entries replace (not extend) the component defaults; otherwise the defaults are
// returned unchanged. Blank entries and surrounding whitespace are ignored.
//
// The env var overrides rather than extends so an operator can tightly constrain
// (or, deliberately, widen) the allowlist without inheriting a broader default.
func ResolveAllowlist(defaults []string) []string {
	raw, ok := os.LookupEnv(EnvFileSourceAllowlist)
	if !ok || strings.TrimSpace(raw) == "" {
		return defaults
	}
	var dirs []string
	for d := range strings.SplitSeq(raw, ",") {
		if d = strings.TrimSpace(d); d != "" {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return defaults
	}
	return dirs
}
