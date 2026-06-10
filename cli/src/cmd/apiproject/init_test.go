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
package apiproject

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/utils"
)

func TestBuildDirectoryStructureCreatesExpectedFiles(t *testing.T) {
	tempDir := t.TempDir()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("failed to restore working directory: %v", chdirErr)
		}
	})

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}

	if err := buildDirectoryStructure("FooAPI", utils.APITypeREST, "1.0.0", "/petstore"); err != nil {
		t.Fatalf("buildDirectoryStructure returned an error: %v", err)
	}

	projectRoot := filepath.Join(tempDir, "FooAPI")
	expectedPaths := []string{
		filepath.Join(projectRoot, ".api-platform", "config.yaml"),
		filepath.Join(projectRoot, "api.yaml"),
		filepath.Join(projectRoot, "gateway.yaml"),
		filepath.Join(projectRoot, "definition.yaml"),
		filepath.Join(projectRoot, "docs"),
		filepath.Join(projectRoot, "tests"),
	}

	for _, expectedPath := range expectedPaths {
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected path to exist: %s (%v)", expectedPath, err)
		}
	}

	gatewayYAML, err := os.ReadFile(filepath.Join(projectRoot, "gateway.yaml"))
	if err != nil {
		t.Fatalf("failed to read gateway.yaml: %v", err)
	}
	if !strings.Contains(string(gatewayYAML), `displayName: "FooAPI"`) {
		t.Fatalf("gateway.yaml does not contain the expected display name")
	}
	if !strings.Contains(string(gatewayYAML), `context: "/petstore"`) {
		t.Fatalf("gateway.yaml does not contain the expected context")
	}

	definitionYAML, err := os.ReadFile(filepath.Join(projectRoot, "definition.yaml"))
	if err != nil {
		t.Fatalf("failed to read definition.yaml: %v", err)
	}
	if !strings.Contains(string(definitionYAML), `"/*":`) {
		t.Fatalf("definition.yaml does not contain the wildcard path")
	}
	if !strings.Contains(string(definitionYAML), "options:") {
		t.Fatalf("definition.yaml does not contain the OPTIONS operation")
	}
}
