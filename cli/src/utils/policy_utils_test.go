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

package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupTempGatewayWorkspaceCreatesSharedTempDirectories(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	buildFilePath := filepath.Join(t.TempDir(), "build.yaml")
	if err := os.WriteFile(buildFilePath, []byte("version: v1\npolicies: []\n"), 0644); err != nil {
		t.Fatalf("failed to write build file: %v", err)
	}

	workspaceDir, err := SetupTempGatewayWorkspace(buildFilePath)
	if err != nil {
		t.Fatalf("SetupTempGatewayWorkspace() failed: %v", err)
	}
	defer os.RemoveAll(workspaceDir)

	for _, dir := range []string{
		filepath.Join(homeDir, ".wso2ap", ".tmp"),
		workspaceDir,
		filepath.Join(workspaceDir, "output"),
		filepath.Join(workspaceDir, "policies"),
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", dir, err)
		}
		if got := info.Mode().Perm(); got != 0755 {
			t.Fatalf("expected %s permissions to be 0755, got %03o", dir, got)
		}
	}
}

func TestCopyPolicyToWorkspaceReturnsSlashSeparatedBuildPath(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "my-policy")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source policy dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "policy-definition.yaml"), []byte("name: my-policy\n"), 0644); err != nil {
		t.Fatalf("failed to write policy definition: %v", err)
	}

	got, err := CopyPolicyToWorkspace("my-policy", "v1.0.0", sourceDir, true, t.TempDir())
	if err != nil {
		t.Fatalf("CopyPolicyToWorkspace() failed: %v", err)
	}

	// The path is written into the build file read by the Linux gateway-builder container,
	// so it must use forward slashes on every host OS, including Windows.
	if want := "policies/my-policy"; got != want {
		t.Fatalf("CopyPolicyToWorkspace() = %q, want %q", got, want)
	}
}
