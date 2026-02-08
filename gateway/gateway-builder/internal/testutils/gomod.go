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

// Package testutils provides common test helper functions for gateway-builder tests.
package testutils

import (
	"fmt"
	"path/filepath"
	"testing"
)

// DefaultGoVersion is the default Go version used in generated go.mod files.
const DefaultGoVersion = "1.25.1"

// WriteGoMod creates a go.mod file in the specified directory with the given module name.
// Uses DefaultGoVersion for the Go version.
// Parent directories are created if they don't exist.
func WriteGoMod(t *testing.T, dir, moduleName string) {
	t.Helper()
	WriteGoModWithVersion(t, dir, moduleName, DefaultGoVersion)
}

// WriteGoModWithVersion creates a go.mod file with a specific Go version.
// Parent directories are created if they don't exist.
func WriteGoModWithVersion(t *testing.T, dir, moduleName, goVersion string) {
	t.Helper()
	content := fmt.Sprintf("module %s\n\ngo %s\n", moduleName, goVersion)
	path := filepath.Join(dir, "go.mod")
	WriteFile(t, path, content)
}

// WritePolicyEngineGoMod creates a go.mod file for the policy-engine module.
// This is the standard module used in most gateway-builder tests.
// Parent directories are created if they don't exist.
func WritePolicyEngineGoMod(t *testing.T, dir string) {
	t.Helper()
	WriteGoMod(t, dir, "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine")
}

// WriteGoModWithRequire creates a go.mod file with a require directive.
// Parent directories are created if they don't exist.
func WriteGoModWithRequire(t *testing.T, dir, moduleName, goVersion, requireModule, requireVersion string) {
	t.Helper()
	content := fmt.Sprintf(`module %s

go %s

require %s %s
`, moduleName, goVersion, requireModule, requireVersion)
	path := filepath.Join(dir, "go.mod")
	WriteFile(t, path, content)
}

// WriteGoModWithReplace creates a go.mod file with a replace directive.
// Parent directories are created if they don't exist.
func WriteGoModWithReplace(t *testing.T, dir, moduleName, goVersion, replaceModule, replacePath string) {
	t.Helper()
	content := fmt.Sprintf(`module %s

go %s

replace %s => %s
`, moduleName, goVersion, replaceModule, replacePath)
	path := filepath.Join(dir, "go.mod")
	WriteFile(t, path, content)
}
