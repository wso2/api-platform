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

// goGetMaxAttempts and goGetRetryBackoff control retries around the 'go get'
// invocation below. Module proxy fetches occasionally fail with transient
// network errors (e.g. an HTTP/2 stream INTERNAL_ERROR from proxy.golang.org)
// that succeed on a bare retry; a timeout or a genuinely missing/invalid
// module will simply fail again on retry at negligible extra cost.
var (
	goGetMaxAttempts  = 3
	goGetRetryBackoff = 2 * time.Second
)

// runGoGet is a seam over exec.CommandContext so tests can stub out the real
// 'go get' invocation.
var runGoGet = func(ctx context.Context, dir, target string) (stderr []byte, err error) {
	cmd := exec.CommandContext(ctx, "go", "get", target)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stderr = &buf
	err = cmd.Run()
	return buf.Bytes(), err
}

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

		var lastErr error
		var lastStderr []byte
		for attempt := 1; attempt <= goGetMaxAttempts; attempt++ {
			slog.Debug("running go get for remote policy",
				"policy", policy.Name,
				"target", target,
				"attempt", attempt)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			stderr, runErr := runGoGet(ctx, srcDir, target)
			timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
			cancel()

			if runErr == nil {
				lastErr = nil
				break
			}
			if timedOut {
				return fmt.Errorf("go get timed out after 5min for %s in %s: %w; stderr: %s", target, srcDir, runErr, stderr)
			}

			lastErr, lastStderr = runErr, stderr
			if attempt < goGetMaxAttempts {
				slog.Warn("go get failed, retrying",
					"policy", policy.Name,
					"target", target,
					"attempt", attempt,
					"maxAttempts", goGetMaxAttempts,
					"err", runErr)
				time.Sleep(goGetRetryBackoff * time.Duration(attempt))
			}
		}
		if lastErr != nil {
			return fmt.Errorf("go get failed for %s in %s after %d attempts: %w; stderr: %s", target, srcDir, goGetMaxAttempts, lastErr, lastStderr)
		}
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

	if err := os.WriteFile(goModPath, formattedData, 0600); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	return nil
}
