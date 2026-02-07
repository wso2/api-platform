/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package policyengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"golang.org/x/mod/modfile"
)

// UpdateGoMod updates go.mod for discovered policies:
//   - gomodule entries: runs 'go get' to pin the real module at its resolved version
//   - filePath entries: adds a replace directive pointing to the local path
func UpdateGoMod(srcDir string, policies []*types.DiscoveredPolicy) error {
	// Ensure srcDir is absolute
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute srcDir: %w", err)
	}
	srcDir = absSrcDir

	goModPath := filepath.Join(srcDir, "go.mod")

	// First pass: run 'go get' for all gomodule (remote) entries
	for _, policy := range policies {
		if policy.IsFilePathEntry {
			continue
		}

		target := fmt.Sprintf("%s@%s", policy.GoModulePath, policy.GoModuleVersion)
		slog.Debug("running go get for remote policy",
			"policy", policy.Name,
			"target", target)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, "go", "get", target)
		cmd.Dir = srcDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			cancel()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("go get timed out after 5min for %s in %s: %w; stderr: %s", target, srcDir, err, stderr.String())
			}
			return fmt.Errorf("go get failed for %s in %s: %w; stderr: %s", target, srcDir, err, stderr.String())
		}
		cancel()
	}

	// Second pass: add replace directives for filePath (local) entries
	// Re-read go.mod after 'go get' may have modified it
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}

	for _, policy := range policies {
		if !policy.IsFilePathEntry {
			continue
		}

		relativePath, err := filepath.Rel(srcDir, policy.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", policy.Name, err)
		}

		slog.Debug("adding replace directive for local policy",
			"policy", policy.Name,
			"modulePath", policy.GoModulePath,
			"relativePath", relativePath)

		if err := modFile.AddReplace(policy.GoModulePath, "", relativePath, ""); err != nil {
			if !strings.Contains(err.Error(), "already") {
				return fmt.Errorf("failed to add replace directive for %s: %w", policy.Name, err)
			}
		}
	}

	// Format and write back
	formattedData, err := modFile.Format()
	if err != nil {
		return fmt.Errorf("failed to format go.mod: %w", err)
	}

	if err := os.WriteFile(goModPath, formattedData, 0644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	return nil
}
