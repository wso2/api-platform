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

package image

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGetDockerBuildLogPathUsesPersistentLogsDirectory(t *testing.T) {
	tmpRoot := t.TempDir()
	workspaceDir := filepath.Join(tmpRoot, "gateway-image-build-123456")
	if err := os.Mkdir(workspaceDir, 0755); err != nil {
		t.Fatalf("failed to create workspace directory: %v", err)
	}

	logPath, err := getDockerBuildLogPath(workspaceDir)
	if err != nil {
		t.Fatalf("getDockerBuildLogPath() failed: %v", err)
	}

	expectedLogPath := filepath.Join(tmpRoot, "logs", "gateway-image-build-123456-docker.log")
	if logPath != expectedLogPath {
		t.Fatalf("expected log path %s, got %s", expectedLogPath, logPath)
	}

	logsInfo, err := os.Stat(filepath.Join(tmpRoot, "logs"))
	if err != nil {
		t.Fatalf("failed to stat logs directory: %v", err)
	}
	if !logsInfo.IsDir() {
		t.Fatalf("expected logs path to be a directory")
	}

	if err := os.RemoveAll(workspaceDir); err != nil {
		t.Fatalf("failed to remove workspace directory: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(logPath)); err != nil {
		t.Fatalf("expected logs directory to survive workspace cleanup: %v", err)
	}
}

// resetPrivateModuleFlags clears the package-level flag state the tests below set.
func resetPrivateModuleFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, name := range []string{"netrc", "git-credential", "go-env"} {
			f := buildCmd.Flags().Lookup(name)
			if sv, ok := f.Value.(interface{ Replace([]string) error }); ok {
				_ = sv.Replace(nil)
			} else {
				_ = f.Value.Set("")
			}
			f.Changed = false
		}
	})
}

func TestGoEnvFlagKeepsCommasInValues(t *testing.T) {
	resetPrivateModuleFlags(t)

	err := buildCmd.ParseFlags([]string{
		"--go-env", "GOPRIVATE=github.com/acme/*,gitlab.acme.com",
		"--go-env", "GOPROXY=https://goproxy.acme.com,direct",
	})
	if err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}

	want := []string{"GOPRIVATE=github.com/acme/*,gitlab.acme.com", "GOPROXY=https://goproxy.acme.com,direct"}
	if !slices.Equal(goEnvFlags, want) {
		t.Fatalf("goEnvFlags = %q, want %q", goEnvFlags, want)
	}
}

func TestNetrcAndGitCredentialAreMutuallyExclusive(t *testing.T) {
	resetPrivateModuleFlags(t)

	err := buildCmd.ParseFlags([]string{"--netrc", "/tmp/netrc", "--git-credential", "github.com=x:TOKEN"})
	if err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	if err := buildCmd.ValidateFlagGroups(); err == nil || !strings.Contains(err.Error(), "none of the others can be") {
		t.Fatalf("ValidateFlagGroups() error = %v, want a mutually exclusive flags error", err)
	}
}
