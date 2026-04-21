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

package discovery

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// sanitizeURL removes credentials from a URL string for safe logging.
func sanitizeURL(u string) string {
	if u == "" {
		return u
	}
	if idx := strings.Index(u, "://"); idx > 0 {
		scheme := u[:idx+3]
		rest := u[idx+3:]
		if at := strings.LastIndex(rest, "@"); at > 0 {
			return scheme + "<redacted-credentials>@" + rest[at+1:]
		}
	}
	return u
}

// sanitizePipSpec removes credentials from any URL-like part of a pip spec.
func sanitizePipSpec(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return spec
	}
	if idx := strings.Index(spec, " @ "); idx > 0 {
		return spec[:idx+3] + sanitizeURL(strings.TrimSpace(spec[idx+3:]))
	}
	return sanitizeURL(spec)
}

func isDirectPipSpec(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return false
	}

	for _, prefix := range []string{"git+", "hg+", "svn+", "bzr+"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	if idx := strings.Index(trimmed, " @ "); idx > 0 {
		target := strings.TrimSpace(trimmed[idx+3:])
		for _, prefix := range []string{"git+", "hg+", "svn+", "bzr+", "https://", "http://", "file://"} {
			if strings.HasPrefix(target, prefix) {
				return true
			}
		}
	}

	return false
}

// resolvePipExecutable returns the pip executable name and any required prefix
// arguments. It probes the PATH in order: pip3 → pip → python3 -m pip.
func resolvePipExecutable() (exe string, prefixArgs []string) {
	for _, candidate := range []string{"pip3", "pip"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3", []string{"-m", "pip"}
	}
	return "", nil
}

// PipPackageInfo contains resolved pip package information
type PipPackageInfo struct {
	Package        string // PyPI package name (e.g., "my-gateway-policy")
	Version        string // Version (e.g., "1.0.0")
	IndexURL       string // Optional custom index URL
	PipSpec        string // Full pip specifier (e.g., "my-gateway-policy==1.0.0")
	TopLevelModule string // Python module name from top_level.txt (e.g., "prompt_compression_policy")
	Dir            string // Temp directory with extracted policy source (for build-time validation)
}

// FetchPipPackage downloads a Python policy wheel and extracts metadata.
//
// Supported formats:
//   - "<package>==<version>[@<index-url>]"
//   - Direct pip specs such as:
//     "git+https://github.com/org/repo.git@v0.1.0#subdirectory=policy"
//     "policy-name @ git+https://github.com/org/repo.git@v0.1.0"
//
// It uses "pip download --no-deps" to fetch only the wheel file, then reads
// the top-level module name and policy source directly from the wheel (ZIP).
// No compilation or dependency installation happens here — that is deferred
// to the python-deps Docker stage where the target platform is correct.
// The returned PipPackageInfo.Dir is a temporary directory created by this
// function. Callers are responsible for removing it when done (os.RemoveAll).
func FetchPipPackage(pipPackage string) (*PipPackageInfo, error) {
	if isDirectPipSpec(pipPackage) {
		return fetchDirectPipPackage(pipPackage)
	}

	pkgName, version, indexURL, err := ParsePipPackageRef(pipPackage)
	if err != nil {
		return nil, err
	}
	sanitizedIndexURL := sanitizeURL(indexURL)

	slog.Info("Fetching pip package",
		"package", pkgName,
		"version", version,
		"indexURL", sanitizedIndexURL,
		"phase", "discovery")

	downloadDir, err := os.MkdirTemp("", "pip-download-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(downloadDir)

	pipSpec := fmt.Sprintf("%s==%s", pkgName, version)
	args := []string{"download", "--no-deps", pipSpec, "-d", downloadDir}
	if indexURL != "" {
		args = append(args, "--index-url", indexURL)
	}

	slog.Debug("Running pip download",
		"package", pipSpec,
		"target", downloadDir,
		"phase", "discovery")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pipExe, prefixArgs := resolvePipExecutable()
	if pipExe == "" {
		return nil, fmt.Errorf("no pip executable found (tried pip3, pip, python3 -m pip): ensure pip or python3 is installed")
	}

	fullArgs := append(prefixArgs, args...)

	cmd := exec.CommandContext(ctx, pipExe, fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		sanitizedStderr := stderr.String()
		if indexURL != "" {
			sanitizedStderr = strings.ReplaceAll(sanitizedStderr, indexURL, sanitizedIndexURL)
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("pip download timed out for %s", pipSpec)
		}
		return nil, fmt.Errorf("pip download failed for %s: %w; stderr: %s", pipSpec, err, sanitizedStderr)
	}

	whlPath, err := findWheelFile(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find wheel for %s: %w", pipSpec, err)
	}

	topLevelModule, err := readTopLevelFromWheel(whlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read top_level.txt from wheel for %s: %w", pipSpec, err)
	}

	extractDir, err := extractModuleFromWheel(whlPath, topLevelModule)
	if err != nil {
		return nil, fmt.Errorf("failed to extract policy source from wheel for %s: %w", pipSpec, err)
	}

	slog.Info("Successfully fetched pip package",
		"package", pkgName,
		"version", version,
		"topLevelModule", topLevelModule,
		"extractDir", extractDir,
		"phase", "discovery")

	return &PipPackageInfo{
		Package:        pkgName,
		Version:        version,
		IndexURL:       indexURL,
		PipSpec:        pipSpec,
		TopLevelModule: topLevelModule,
		Dir:            extractDir,
	}, nil
}

func fetchDirectPipPackage(pipPackage string) (*PipPackageInfo, error) {
	pipSpec := strings.TrimSpace(pipPackage)
	sanitizedSpec := sanitizePipSpec(pipSpec)

	slog.Info("Fetching direct pip package",
		"pipSpec", sanitizedSpec,
		"phase", "discovery")

	wheelDir, err := os.MkdirTemp("", "pip-wheel-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(wheelDir)

	args := []string{"wheel", "--no-deps", "--no-build-isolation", pipSpec, "-w", wheelDir}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pipExe, prefixArgs := resolvePipExecutable()
	if pipExe == "" {
		return nil, fmt.Errorf("no pip executable found (tried pip3, pip, python3 -m pip): ensure pip or python3 is installed")
	}

	fullArgs := append(prefixArgs, args...)
	cmd := exec.CommandContext(ctx, pipExe, fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		sanitizedStderr := strings.ReplaceAll(stderr.String(), pipSpec, sanitizedSpec)
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("pip wheel timed out for %s", sanitizedSpec)
		}
		return nil, fmt.Errorf("pip wheel failed for %s: %w; stderr: %s", sanitizedSpec, err, sanitizedStderr)
	}

	whlPath, err := findWheelFile(wheelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find wheel for %s: %w", sanitizedSpec, err)
	}

	topLevelModule, err := readTopLevelFromWheel(whlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read top_level.txt from wheel for %s: %w", sanitizedSpec, err)
	}

	extractDir, err := extractModuleFromWheel(whlPath, topLevelModule)
	if err != nil {
		return nil, fmt.Errorf("failed to extract policy source from wheel for %s: %w", sanitizedSpec, err)
	}

	slog.Info("Successfully fetched direct pip package",
		"pipSpec", sanitizedSpec,
		"topLevelModule", topLevelModule,
		"extractDir", extractDir,
		"phase", "discovery")

	return &PipPackageInfo{
		Package:        pipSpec,
		PipSpec:        pipSpec,
		TopLevelModule: topLevelModule,
		Dir:            extractDir,
	}, nil
}

// ParsePipPackageRef parses a pip package reference.
// Format: "<package>==<version>[@<index-url>]"
func ParsePipPackageRef(ref string) (pkgName, version, indexURL string, err error) {
	parts := strings.SplitN(ref, "==", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("invalid pipPackage format, expected '<package>==<version>'")
	}

	pkgName = parts[0]
	versionPart := parts[1]

	if idx := strings.Index(versionPart, "@"); idx > 0 {
		candidate := versionPart[idx+1:]
		if strings.Contains(candidate, "://") {
			indexURL = candidate
			versionPart = versionPart[:idx]
		}
	}

	pkgName = strings.TrimSpace(pkgName)
	version = strings.TrimSpace(versionPart)

	if pkgName == "" || version == "" {
		return "", "", "", fmt.Errorf("invalid pipPackage format, expected '<package>==<version>'")
	}

	return pkgName, version, indexURL, nil
}

// findWheelFile finds the .whl file in the download directory.
func findWheelFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read download directory: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".whl") {
			return filepath.Join(dir, e.Name()), nil
		}
	}

	return "", fmt.Errorf("no .whl file found in %s", dir)
}

// readTopLevelFromWheel reads the top-level module name from top_level.txt inside a wheel.
func readTopLevelFromWheel(whlPath string) (string, error) {
	r, err := zip.OpenReader(whlPath)
	if err != nil {
		return "", fmt.Errorf("failed to open wheel: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".dist-info/top_level.txt") {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open top_level.txt: %w", err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return "", fmt.Errorf("failed to read top_level.txt: %w", err)
			}

			module := strings.TrimSpace(string(data))
			if module == "" {
				return "", fmt.Errorf("top_level.txt is empty")
			}
			// Take only the first line (some packages list multiple modules)
			if idx := strings.IndexByte(module, '\n'); idx >= 0 {
				module = strings.TrimSpace(module[:idx])
			}
			return module, nil
		}
	}

	return "", fmt.Errorf("top_level.txt not found in wheel")
}

// extractModuleFromWheel extracts the policy module directory from a wheel into a temp directory.
// Returns the path to the extracted module directory (for build-time validation).
func extractModuleFromWheel(whlPath string, topLevelModule string) (string, error) {
	r, err := zip.OpenReader(whlPath)
	if err != nil {
		return "", fmt.Errorf("failed to open wheel: %w", err)
	}
	defer r.Close()

	extractDir, err := os.MkdirTemp("", "pip-policy-*")
	if err != nil {
		return "", fmt.Errorf("failed to create extraction directory: %w", err)
	}

	prefix := topLevelModule + "/"
	extracted := false

	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}

		destPath := filepath.Join(extractDir, f.Name)

		// Validate against zip-slip
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(extractDir)+string(os.PathSeparator)) {
			os.RemoveAll(extractDir)
			return "", fmt.Errorf("zip-slip detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				os.RemoveAll(extractDir)
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			os.RemoveAll(extractDir)
			return "", fmt.Errorf("failed to create parent directory: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			os.RemoveAll(extractDir)
			return "", fmt.Errorf("failed to open file in wheel: %w", err)
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			os.RemoveAll(extractDir)
			return "", fmt.Errorf("failed to create extracted file: %w", err)
		}

		if _, err := io.Copy(outFile, rc); err != nil {
			outFile.Close()
			rc.Close()
			os.RemoveAll(extractDir)
			return "", fmt.Errorf("failed to write extracted file: %w", err)
		}

		outFile.Close()
		rc.Close()
		extracted = true
	}

	if !extracted {
		os.RemoveAll(extractDir)
		return "", fmt.Errorf("no files found for module %s in wheel", topLevelModule)
	}

	return filepath.Join(extractDir, topLevelModule), nil
}
