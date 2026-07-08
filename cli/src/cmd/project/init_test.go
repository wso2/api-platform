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
package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/cli/utils"
)

func chdirTemp(t *testing.T) string {
	t.Helper()
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
	return tempDir
}

func TestBuildDirectoryStructureCreatesExpectedFiles(t *testing.T) {
	tempDir := chdirTemp(t)

	if err := buildDirectoryStructure("FooAPI", utils.TypeREST); err != nil {
		t.Fatalf("buildDirectoryStructure returned an error: %v", err)
	}

	projectRoot := filepath.Join(tempDir, "FooAPI")
	expectedPaths := []string{
		filepath.Join(projectRoot, ".api-platform", "config.yaml"),
		filepath.Join(projectRoot, "metadata.yaml"),
		filepath.Join(projectRoot, "runtime.yaml"),
		filepath.Join(projectRoot, "definition.yaml"),
		filepath.Join(projectRoot, "docs"),
		filepath.Join(projectRoot, "tests"),
	}

	for _, expectedPath := range expectedPaths {
		if _, err := os.Stat(expectedPath); err != nil {
			t.Fatalf("expected path to exist: %s (%v)", expectedPath, err)
		}
	}

	runtimeYAML := readFile(t, filepath.Join(projectRoot, "runtime.yaml"))
	if !strings.Contains(runtimeYAML, "kind: RestApi") {
		t.Fatalf("runtime.yaml does not contain the expected kind: %q", runtimeYAML)
	}
	if !strings.Contains(runtimeYAML, "displayName: FooAPI") {
		t.Fatalf("runtime.yaml does not contain the expected display name: %q", runtimeYAML)
	}
	if !strings.Contains(runtimeYAML, "method: OPTIONS") {
		t.Fatalf("runtime.yaml does not contain the OPTIONS operation: %q", runtimeYAML)
	}

	metadataYAML := readFile(t, filepath.Join(projectRoot, "metadata.yaml"))
	if !strings.Contains(metadataYAML, "apiVersion: management.api-platform.wso2.com/v1") {
		t.Fatalf("metadata.yaml does not carry the management apiVersion: %q", metadataYAML)
	}
	if !strings.Contains(metadataYAML, "businessInformation:") {
		t.Fatalf("management metadata.yaml should contain businessInformation: %q", metadataYAML)
	}

	configYAML := readFile(t, filepath.Join(projectRoot, ".api-platform", "config.yaml"))
	for _, want := range []string{"deploymentArtifact: ./runtime.yaml", "metadataFile: ./metadata.yaml", "definition: ./definition.yaml"} {
		if !strings.Contains(configYAML, want) {
			t.Fatalf("config.yaml missing %q: %s", want, configYAML)
		}
	}

	definitionYAML := readFile(t, filepath.Join(projectRoot, "definition.yaml"))
	if !strings.Contains(definitionYAML, `"/*":`) {
		t.Fatalf("definition.yaml does not contain the wildcard path")
	}
	if !strings.Contains(definitionYAML, "options:") {
		t.Fatalf("definition.yaml does not contain the OPTIONS operation")
	}
}

func TestBuildDirectoryStructureAIWorkspaceMetadata(t *testing.T) {
	tempDir := chdirTemp(t)

	if err := buildDirectoryStructure("OpenAI Dev LLM", utils.TypeLLMProxy); err != nil {
		t.Fatalf("buildDirectoryStructure returned an error: %v", err)
	}

	projectRoot := filepath.Join(tempDir, "OpenAI Dev LLM")
	metadataYAML := readFile(t, filepath.Join(projectRoot, "metadata.yaml"))

	for _, want := range []string{
		"apiVersion: ai-workspace.api-platform.wso2.com/v1alpha",
		"kind: LlmProxyMetadata",
		"name: openai-dev-llm",
		"displayName: OpenAI Dev LLM",
	} {
		if !strings.Contains(metadataYAML, want) {
			t.Fatalf("ai-workspace metadata.yaml missing %q: %s", want, metadataYAML)
		}
	}

	// The slim ai-workspace metadata must not carry the management-only blocks.
	if strings.Contains(metadataYAML, "businessInformation:") || strings.Contains(metadataYAML, "endpoints:") {
		t.Fatalf("ai-workspace metadata.yaml should not contain management-only fields: %s", metadataYAML)
	}

	runtimeYAML := readFile(t, filepath.Join(projectRoot, "runtime.yaml"))
	if !strings.Contains(runtimeYAML, "kind: LlmProxy") {
		t.Fatalf("runtime.yaml should carry the artifact kind: %s", runtimeYAML)
	}
}

func TestBuildDirectoryStructureRejectsUnsupportedType(t *testing.T) {
	chdirTemp(t)

	err := buildDirectoryStructure("FooAPI", "graphql")
	if err == nil || !strings.Contains(err.Error(), "unsupported artifact type") {
		t.Fatalf("expected unsupported artifact type error, got %v", err)
	}
	if !strings.Contains(err.Error(), "supported types:") {
		t.Fatalf("expected error to list supported types, got %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}
