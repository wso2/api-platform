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

package discovery

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// resolvePipExecutable returns the pip executable name and any required prefix
// arguments. It probes the PATH in order: pip3 → pip → python3 -m pip.
//
// This is needed because different environments expose pip differently:
//   - Debian/Ubuntu (and their Docker images): pip3 only
//   - macOS system Python: pip3 only (pip is an alias that may differ)
//   - PyPI-installed Python: both pip and pip3 available
//   - Minimal containers: only python3 with -m pip
func resolvePipExecutable() (exe string, prefixArgs []string) {
	for _, candidate := range []string{"pip3", "pip"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	// Fallback: python3 -m pip
	return "python3", []string{"-m", "pip"}
}

// PipPackageInfo contains resolved pip package information
type PipPackageInfo struct {
	Package  string // PyPI package name (e.g., "wso2-gateway-policy-prompt-compression")
	Version  string // Version (e.g., "1.0.0")
	IndexURL string // Optional custom index URL
	Dir      string // Local directory where the package was installed
}

// FetchPipPackage downloads a Python policy package using pip.
//
// Format: "<package>==<version>[@<index-url>]"
//
// Examples:
//   - "wso2-gateway-policy-prompt-compression==1.0.0"
//   - "my-org-auth-policy==2.3.0@https://pypi.my-company.com/simple"
//
// The package is installed into an isolated directory using pip install --target.
func FetchPipPackage(pipPackage string) (*PipPackageInfo, error) {
	slog.Info("Fetching pip package", "reference", pipPackage, "phase", "discovery")

	pkgName, version, indexURL, err := ParsePipPackageRef(pipPackage)
	if err != nil {
		return nil, err
	}

	// Create isolated install directory
	installDir, err := os.MkdirTemp("", "pip-policy-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Build pip install command
	pipSpec := fmt.Sprintf("%s==%s", pkgName, version)
	args := []string{"install", pipSpec, "--target", installDir}
	if indexURL != "" {
		args = append(args, "--index-url", indexURL)
	}

	slog.Debug("Running pip install",
		"package", pipSpec,
		"target", installDir,
		"indexURL", indexURL,
		"phase", "discovery")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pipExe, prefixArgs := resolvePipExecutable()
	slog.Debug("Using pip executable", "exe", pipExe, "prefixArgs", prefixArgs, "phase", "discovery")
	fullArgs := append(prefixArgs, args...)

	cmd := exec.CommandContext(ctx, pipExe, fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(installDir)
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("pip install timed out for %s", pipSpec)
		}
		return nil, fmt.Errorf("pip install failed for %s: %w; stderr: %s", pipSpec, err, stderr.String())
	}

	// Find the policy directory inside the installed package.
	// pip installs the package as a Python module — we look for the directory
	// containing policy-definition.yaml.
	policyDir, err := findPolicyDir(installDir)
	if err != nil {
		os.RemoveAll(installDir)
		return nil, fmt.Errorf("failed to locate policy in installed package %s: %w", pipSpec, err)
	}

	slog.Info("Successfully fetched pip package",
		"package", pkgName,
		"version", version,
		"path", policyDir,
		"phase", "discovery")

	return &PipPackageInfo{
		Package:  pkgName,
		Version:  version,
		IndexURL: indexURL,
		Dir:      policyDir,
	}, nil
}

// ParsePipPackageRef parses a pip package reference.
// Format: "<package>==<version>[@<index-url>]"
func ParsePipPackageRef(ref string) (pkgName, version, indexURL string, err error) {
	// Split off optional @<index-url> suffix
	mainPart := ref
	if idx := strings.LastIndex(ref, "@"); idx > 0 {
		candidate := ref[idx+1:]
		// Only treat as index URL if it looks like a URL (contains "://")
		if strings.Contains(candidate, "://") {
			indexURL = candidate
			mainPart = ref[:idx]
		}
	}

	// Split "package==version"
	parts := strings.SplitN(mainPart, "==", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("invalid pipPackage format, expected '<package>==<version>': %s", ref)
	}

	pkgName = strings.TrimSpace(parts[0])
	version = strings.TrimSpace(parts[1])

	return pkgName, version, indexURL, nil
}

// findPolicyDir searches for a directory containing policy-definition.yaml
// within the pip install target directory.
func findPolicyDir(installDir string) (string, error) {
	var found string
	err := filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "policy-definition.yaml" {
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no policy-definition.yaml found in installed package")
	}
	return found, nil
}
