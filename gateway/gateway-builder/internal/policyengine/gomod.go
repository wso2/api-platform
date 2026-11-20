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

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/goenv"
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
	cmd.Env = goenv.WithToolchain(cmd.Environ())
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

// VerifyPinnedGoModules checks that the policy engine module in srcDir builds
// exactly the remote policy versions pinned in the build file: go.mod must
// require the pinned version of each remote policy module (so neither 'go get'
// nor 'go mod tidy' moved it), and go.sum must record the pinned h1: hash for
// it. Policies without a recorded GoModuleSum (local filePath entries) are skipped.
func VerifyPinnedGoModules(srcDir string, policies []*types.DiscoveredPolicy) error {
	goModPath := filepath.Join(srcDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}
	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}
	required := make(map[string]string, len(modFile.Require))
	for _, r := range modFile.Require {
		required[r.Mod.Path] = r.Mod.Version
	}

	sums, err := readGoSum(filepath.Join(srcDir, "go.sum"))
	if err != nil {
		return err
	}

	for _, p := range policies {
		if p.IsFilePathEntry || p.GoModuleSum == "" {
			continue
		}
		got, ok := required[p.GoModulePath]
		if !ok {
			return fmt.Errorf("policy %s: go.mod does not require %s", p.Name, p.GoModulePath)
		}
		if got != p.GoModuleVersion {
			return fmt.Errorf("policy %s: go.mod selects %s@%s but the build file pins %s",
				p.Name, p.GoModulePath, got, p.GoModuleVersion)
		}
		key := p.GoModulePath + " " + p.GoModuleVersion
		sum, ok := sums[key]
		if !ok {
			return fmt.Errorf("policy %s: go.sum has no entry for %s", p.Name, key)
		}
		if sum != p.GoModuleSum {
			return fmt.Errorf("policy %s: go.sum records %s for %s but the build file pins %s",
				p.Name, sum, key, p.GoModuleSum)
		}
	}
	return nil
}

// readGoSum returns the module zip hashes in a go.sum file keyed by
// "<path> <version>". The "/go.mod" hash lines are ignored.
func readGoSum(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.sum: %w", err)
	}
	sums := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || strings.HasSuffix(fields[1], "/go.mod") {
			continue
		}
		sums[fields[0]+" "+fields[1]] = fields[2]
	}
	return sums, nil
}
