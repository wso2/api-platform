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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	majorOnlyPkgVersionPattern = regexp.MustCompile(`^\d+\.0$`)
	minorOnlyPkgVersionPattern = regexp.MustCompile(`^\d+\.\d+\.0$`)
	majorOnlyRefPattern        = regexp.MustCompile(`v(\d+)$`)
	minorOnlyRefPattern        = regexp.MustCompile(`v(\d+)\.(\d+)$`)
	exactRefPattern            = regexp.MustCompile(`v\d+\.\d+\.\d+$`)
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
		return spec[:idx+3] + sanitizeDirectPipTarget(strings.TrimSpace(spec[idx+3:]))
	}

	return sanitizeDirectPipTarget(spec)
}

func sanitizeDirectPipTarget(target string) string {
	if target == "" {
		return target
	}

	fragment := ""
	if idx := strings.Index(target, "#"); idx >= 0 {
		fragment = target[idx:]
		target = target[:idx]
	}

	if strings.HasPrefix(target, "git+") {
		if atIdx := strings.LastIndex(target, "@"); atIdx > len("git+") {
			repoURL := target[:atIdx]
			gitRef := target[atIdx:]
			return sanitizeURL(repoURL) + gitRef + fragment
		}
	}

	return sanitizeURL(target) + fragment
}

func isDirectPipSpec(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return false
	}

	if strings.HasPrefix(trimmed, "git+") {
		return true
	}

	if idx := strings.Index(trimmed, " @ "); idx > 0 {
		target := strings.TrimSpace(trimmed[idx+3:])
		return strings.HasPrefix(target, "git+")
	}

	return false
}

// resolvePipExecutable returns the pip executable name and any required prefix
// arguments. It probes the PATH in order: pip3 -> pip -> python3 -m pip.
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

// PipPackageInfo contains resolved pip package information.
type PipPackageInfo struct {
	Package        string // PyPI package name (e.g., "my-gateway-policy")
	Version        string // Version (e.g., "1.0.0")
	IndexURL       string // Optional custom index URL
	PipSpec        string // Full pip specifier (e.g., "my-gateway-policy==1.0.0")
	TopLevelModule string // Python module name from top_level.txt (e.g., "prompt_compression_policy")
	Dir            string // Temp directory with extracted policy source (for build-time validation)
}

// PipPackageRef holds parsed indexed pip package reference information.
type PipPackageRef struct {
	PackageName    string
	Version        string
	IndexURL       string
	IsVersionRange bool
}

// VCSPipSpec holds parsed VCS pip spec components.
type VCSPipSpec struct {
	RepoURL      string
	GitRef       string
	Subdirectory string
	FullSpec     string
}

type vcsVersionType int

const (
	vcsVersionExact vcsVersionType = iota
	vcsVersionMinorOnly
	vcsVersionMajorOnly
	vcsVersionNone
)

func (t vcsVersionType) String() string {
	switch t {
	case vcsVersionExact:
		return "exact"
	case vcsVersionMinorOnly:
		return "minor-only"
	case vcsVersionMajorOnly:
		return "major-only"
	default:
		return "none"
	}
}

// FetchPipPackage downloads a Python policy package and extracts metadata.
// Supports indexed packages (PyPI/private), full VCS specs, and Go-style short
// URLs, each with exact or ranged version specifiers.
func FetchPipPackage(pipPackage string) (*PipPackageInfo, error) {
	if isDirectPipSpec(pipPackage) {
		return fetchVCSPipPackage(pipPackage)
	}

	if strings.Contains(pipPackage, " @ ") {
		return nil, fmt.Errorf(
			"unsupported pip direct reference: %q; only git+ VCS specs are supported for direct references",
			sanitizePipSpec(pipPackage),
		)
	}

	if strings.Contains(pipPackage, "==") || strings.Contains(pipPackage, "~=") {
		return fetchIndexedPipPackage(pipPackage)
	}

	if strings.Contains(pipPackage, "@") {
		expanded, err := expandShortURL(pipPackage)
		if err != nil {
			return nil, fmt.Errorf("failed to expand short URL %q: %w", pipPackage, err)
		}

		slog.Info("Expanded short URL to VCS spec",
			"input", pipPackage,
			"expanded", sanitizePipSpec(expanded),
			"phase", "discovery")

		return fetchVCSPipPackage(expanded)
	}

	return nil, fmt.Errorf("unrecognized pipPackage format: %q", pipPackage)
}

func fetchIndexedPipPackage(pipPackage string) (*PipPackageInfo, error) {
	ref, err := ParsePipPackageRef(pipPackage)
	if err != nil {
		return nil, err
	}

	var downloadSpec string
	if ref.IsVersionRange {
		downloadSpec = fmt.Sprintf("%s~=%s", ref.PackageName, ref.Version)
	} else {
		downloadSpec = fmt.Sprintf("%s==%s", ref.PackageName, ref.Version)
	}

	slog.Info("Fetching indexed pip package",
		"package", ref.PackageName,
		"spec", downloadSpec,
		"versionRange", ref.IsVersionRange,
		"indexURL", sanitizeURL(ref.IndexURL),
		"phase", "discovery")

	downloadDir, err := os.MkdirTemp("", "pip-download-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(downloadDir)

	args := []string{"download", "--no-deps", downloadSpec, "-d", downloadDir}
	if ref.IndexURL != "" {
		args = append(args, "--index-url", ref.IndexURL)
	}

	if err := runPipCommand(args, 5*time.Minute, downloadSpec, ref.IndexURL, "download"); err != nil {
		return nil, err
	}

	whlPath, err := findWheelFile(downloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find wheel for %s: %w", downloadSpec, err)
	}

	resolvedVersion, err := readVersionFromWheel(whlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wheel version for %s: %w", downloadSpec, err)
	}

	topLevelModule, err := readTopLevelFromWheel(whlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read top_level.txt from wheel for %s: %w", downloadSpec, err)
	}

	extractDir, err := extractModuleFromWheel(whlPath, topLevelModule)
	if err != nil {
		return nil, fmt.Errorf("failed to extract policy source from wheel for %s: %w", downloadSpec, err)
	}

	exactPipSpec := buildExactIndexedPipSpec(ref, resolvedVersion)
	loggedExactPipSpec := exactPipSpec
	if ref.IndexURL != "" {
		loggedExactPipSpec = fmt.Sprintf("%s==%s@%s", ref.PackageName, resolvedVersion, sanitizeURL(ref.IndexURL))
	}

	slog.Info("Successfully fetched indexed pip package",
		"package", ref.PackageName,
		"resolvedVersion", resolvedVersion,
		"exactPipSpec", loggedExactPipSpec,
		"topLevelModule", topLevelModule,
		"extractDir", extractDir,
		"phase", "discovery")

	return &PipPackageInfo{
		Package:        ref.PackageName,
		Version:        resolvedVersion,
		IndexURL:       ref.IndexURL,
		PipSpec:        exactPipSpec,
		TopLevelModule: topLevelModule,
		Dir:            extractDir,
	}, nil
}

func buildExactIndexedPipSpec(ref *PipPackageRef, resolvedVersion string) string {
	exactPipSpec := fmt.Sprintf("%s==%s", ref.PackageName, resolvedVersion)
	if ref.IndexURL != "" {
		exactPipSpec += "@" + ref.IndexURL
	}
	return exactPipSpec
}

func fetchVCSPipPackage(pipPackage string) (*PipPackageInfo, error) {
	vcs, err := parseVCSPipSpec(pipPackage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse VCS pip spec: %w", err)
	}

	resolvedSpec := vcs.FullSpec
	versionType := classifyVCSRef(vcs.GitRef)
	if versionType == vcsVersionMajorOnly || versionType == vcsVersionMinorOnly {
		exactRef, err := resolveVCSVersion(vcs.RepoURL, vcs.GitRef, versionType)
		if err != nil {
			return nil, err
		}

		resolvedSpec = rebuildVCSPipSpec(vcs, exactRef)
		slog.Info("Resolved VCS ranged pip package",
			"original", sanitizePipSpec(vcs.FullSpec),
			"resolved", sanitizePipSpec(resolvedSpec),
			"phase", "discovery")
	}

	sanitizedSpec := sanitizePipSpec(resolvedSpec)
	slog.Info("Fetching VCS pip package",
		"pipSpec", sanitizedSpec,
		"phase", "discovery")

	wheelDir, err := os.MkdirTemp("", "pip-wheel-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(wheelDir)

	args := []string{"wheel", "--no-deps", resolvedSpec, "-w", wheelDir}
	if err := runPipCommand(args, 5*time.Minute, resolvedSpec, "", "wheel"); err != nil {
		return nil, err
	}

	whlPath, err := findWheelFile(wheelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find wheel for %s: %w", sanitizedSpec, err)
	}

	resolvedVersion, err := readVersionFromWheel(whlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wheel version for %s: %w", sanitizedSpec, err)
	}

	topLevelModule, err := readTopLevelFromWheel(whlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read top_level.txt from wheel for %s: %w", sanitizedSpec, err)
	}

	extractDir, err := extractModuleFromWheel(whlPath, topLevelModule)
	if err != nil {
		return nil, fmt.Errorf("failed to extract policy source from wheel for %s: %w", sanitizedSpec, err)
	}

	slog.Info("Successfully fetched VCS pip package",
		"pipSpec", sanitizedSpec,
		"version", resolvedVersion,
		"topLevelModule", topLevelModule,
		"extractDir", extractDir,
		"phase", "discovery")

	return &PipPackageInfo{
		Package:        vcs.RepoURL,
		Version:        resolvedVersion,
		PipSpec:        resolvedSpec,
		TopLevelModule: topLevelModule,
		Dir:            extractDir,
	}, nil
}

func runPipCommand(args []string, timeout time.Duration, spec string, indexURL string, action string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pipExe, prefixArgs := resolvePipExecutable()
	if pipExe == "" {
		return fmt.Errorf("no pip executable found (tried pip3, pip, python3 -m pip): ensure pip or python3 is installed")
	}

	fullArgs := append(prefixArgs, args...)
	cmd := exec.CommandContext(ctx, pipExe, fullArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		sanitizedSpec := sanitizePipSpec(spec)
		sanitizedStderr := stderr.String()
		if spec != sanitizedSpec {
			sanitizedStderr = strings.ReplaceAll(sanitizedStderr, spec, sanitizedSpec)
		}
		if indexURL != "" {
			sanitizedStderr = strings.ReplaceAll(sanitizedStderr, indexURL, sanitizeURL(indexURL))
		}
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("pip %s timed out for %s", action, sanitizedSpec)
		}
		return fmt.Errorf("pip %s failed for %s: %w; stderr: %s", action, sanitizedSpec, err, sanitizedStderr)
	}

	return nil
}

func expandShortURL(spec string) (string, error) {
	spec = strings.TrimSpace(spec)

	atIdx := strings.LastIndex(spec, "@")
	if atIdx < 0 || atIdx == len(spec)-1 {
		return "", fmt.Errorf("short URL must contain '@version': %s", spec)
	}

	modulePath := spec[:atIdx]
	version := spec[atIdx+1:]

	segments := strings.Split(modulePath, "/")
	if len(segments) < 4 {
		return "", fmt.Errorf(
			"short URL must have at least 4 path segments (host/org/repo/subdir), got %d: %s",
			len(segments),
			spec,
		)
	}

	host := segments[0]
	org := segments[1]
	repo := segments[2]
	subdirectory := strings.Join(segments[3:], "/")

	repoURL := fmt.Sprintf("https://%s/%s/%s.git", host, org, repo)
	gitRef := subdirectory + "/" + version

	return fmt.Sprintf("git+%s@%s#subdirectory=%s", repoURL, gitRef, subdirectory), nil
}

// ParsePipPackageRef parses an indexed pip package reference.
// Supported formats:
//   - "<package>==<version>"
//   - "<package>==<version>@<url>"
//   - "<package>~=<major>.0"
//   - "<package>~=<major>.0@<url>"
//   - "<package>~=<major>.<minor>.0"
//   - "<package>~=<major>.<minor>.0@<url>"
func ParsePipPackageRef(ref string) (*PipPackageRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf(
			"invalid pip spec: expected '<pkg>==<ver>', '<pkg>~=<major>.0', or '<pkg>~=<major>.<minor>.0', got %q",
			sanitizePipSpec(ref),
		)
	}

	if idx := strings.Index(ref, "~="); idx > 0 {
		pkgName := strings.TrimSpace(ref[:idx])
		versionPart := strings.TrimSpace(ref[idx+2:])
		version, indexURL := parseIndexURL(versionPart)
		isMajorOnly := majorOnlyPkgVersionPattern.MatchString(version)
		isMinorOnly := minorOnlyPkgVersionPattern.MatchString(version)
		if pkgName == "" || version == "" || (!isMajorOnly && !isMinorOnly) {
			return nil, fmt.Errorf(
				"invalid pip spec: expected '<pkg>==<ver>', '<pkg>~=<major>.0', or '<pkg>~=<major>.<minor>.0', got %q",
				sanitizePipSpec(ref),
			)
		}

		return &PipPackageRef{
			PackageName:    pkgName,
			Version:        version,
			IndexURL:       indexURL,
			IsVersionRange: true,
		}, nil
	}

	parts := strings.SplitN(ref, "==", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf(
			"invalid pip spec: expected '<pkg>==<ver>', '<pkg>~=<major>.0', or '<pkg>~=<major>.<minor>.0', got %q",
			sanitizePipSpec(ref),
		)
	}

	pkgName := strings.TrimSpace(parts[0])
	version, indexURL := parseIndexURL(strings.TrimSpace(parts[1]))
	if pkgName == "" || version == "" {
		return nil, fmt.Errorf(
			"invalid pip spec: expected '<pkg>==<ver>', '<pkg>~=<major>.0', or '<pkg>~=<major>.<minor>.0', got %q",
			sanitizePipSpec(ref),
		)
	}

	return &PipPackageRef{
		PackageName:    pkgName,
		Version:        version,
		IndexURL:       indexURL,
		IsVersionRange: false,
	}, nil
}

// parseIndexURL extracts an optional @<index-url> suffix from a version string.
func parseIndexURL(versionPart string) (version, indexURL string) {
	if idx := strings.Index(versionPart, "@"); idx > 0 {
		candidate := strings.TrimSpace(versionPart[idx+1:])
		if strings.Contains(candidate, "://") {
			return strings.TrimSpace(versionPart[:idx]), candidate
		}
	}
	return strings.TrimSpace(versionPart), ""
}

// parseVCSPipSpec parses a VCS pip spec into its components.
func parseVCSPipSpec(spec string) (*VCSPipSpec, error) {
	s := strings.TrimSpace(spec)
	if idx := strings.Index(s, " @ "); idx > 0 {
		s = strings.TrimSpace(s[idx+3:])
	}

	subdirectory := ""
	if hashIdx := strings.Index(s, "#"); hashIdx > 0 {
		fragment := s[hashIdx+1:]
		s = s[:hashIdx]

		for _, part := range strings.Split(fragment, "&") {
			if strings.HasPrefix(part, "subdirectory=") {
				subdirectory = strings.TrimPrefix(part, "subdirectory=")
				break
			}
		}
	}

	if !strings.HasPrefix(s, "git+") {
		return nil, fmt.Errorf("only git+ VCS specs are supported, got %q", sanitizePipSpec(spec))
	}

	atIdx := strings.LastIndex(s, "@")
	if atIdx < 0 || atIdx == len(s)-1 {
		return nil, fmt.Errorf("no ref separator '@' found in VCS spec")
	}

	return &VCSPipSpec{
		RepoURL:      strings.TrimPrefix(s[:atIdx], "git+"),
		GitRef:       s[atIdx+1:],
		Subdirectory: subdirectory,
		FullSpec:     spec,
	}, nil
}

func classifyVCSRef(ref string) vcsVersionType {
	if exactRefPattern.MatchString(ref) {
		return vcsVersionExact
	}
	if minorOnlyRefPattern.MatchString(ref) {
		return vcsVersionMinorOnly
	}
	if majorOnlyRefPattern.MatchString(ref) {
		return vcsVersionMajorOnly
	}
	return vcsVersionNone
}

func resolveVCSVersion(repoURL string, partialRef string, versionType vcsVersionType) (string, error) {
	sanitizedURL := sanitizeURL(repoURL)

	slog.Info("Resolving VCS partial version ref",
		"repo", sanitizedURL,
		"ref", partialRef,
		"type", versionType.String(),
		"phase", "discovery")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", repoURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		sanitizedStderr := strings.ReplaceAll(stderr.String(), repoURL, sanitizedURL)
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git ls-remote timed out for %s", sanitizedURL)
		}
		return "", fmt.Errorf("git ls-remote failed for %s: %w; stderr: %s", sanitizedURL, err, sanitizedStderr)
	}

	tagPrefix := partialRef + "."
	type candidate struct {
		tag   string
		minor int
		patch int
	}

	var candidates []candidate
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		refPath := parts[1]
		if !strings.HasPrefix(refPath, "refs/tags/") || strings.HasSuffix(refPath, "^{}") {
			continue
		}

		tagName := strings.TrimPrefix(refPath, "refs/tags/")
		if !strings.HasPrefix(tagName, tagPrefix) {
			continue
		}

		suffix := strings.TrimPrefix(tagName, tagPrefix)
		switch versionType {
		case vcsVersionMajorOnly:
			segments := strings.Split(suffix, ".")
			if len(segments) != 2 {
				continue
			}

			minor, err := strconv.Atoi(segments[0])
			if err != nil {
				continue
			}

			patch, err := strconv.Atoi(segments[1])
			if err != nil {
				continue
			}

			candidates = append(candidates, candidate{
				tag:   tagName,
				minor: minor,
				patch: patch,
			})
		case vcsVersionMinorOnly:
			patch, err := strconv.Atoi(suffix)
			if err != nil {
				continue
			}

			candidates = append(candidates, candidate{
				tag:   tagName,
				patch: patch,
			})
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no tags found matching %q in %s", partialRef, sanitizedURL)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].minor != candidates[j].minor {
			return candidates[i].minor > candidates[j].minor
		}
		return candidates[i].patch > candidates[j].patch
	})

	best := candidates[0].tag
	slog.Info("Resolved VCS partial version ref",
		"repo", sanitizedURL,
		"ref", partialRef,
		"resolvedTag", best,
		"candidateCount", len(candidates),
		"phase", "discovery")

	return best, nil
}

// rebuildVCSPipSpec reconstructs a VCS pip spec with a new exact git ref.
func rebuildVCSPipSpec(parsed *VCSPipSpec, exactRef string) string {
	result := "git+" + parsed.RepoURL
	if exactRef != "" {
		result += "@" + exactRef
	}
	if parsed.Subdirectory != "" {
		result += "#subdirectory=" + parsed.Subdirectory
	}
	return result
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

// readVersionFromWheel reads the package version from a wheel's METADATA file.
func readVersionFromWheel(whlPath string) (string, error) {
	r, err := zip.OpenReader(whlPath)
	if err != nil {
		return "", fmt.Errorf("failed to open wheel: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if !strings.HasSuffix(f.Name, ".dist-info/METADATA") {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open METADATA: %w", err)
		}

		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Version: ") {
				if closeErr := rc.Close(); closeErr != nil {
					return "", fmt.Errorf("failed to close METADATA: %w", closeErr)
				}
				return strings.TrimSpace(strings.TrimPrefix(line, "Version: ")), nil
			}
		}

		if err := scanner.Err(); err != nil {
			rc.Close()
			return "", fmt.Errorf("failed to scan METADATA: %w", err)
		}
		if err := rc.Close(); err != nil {
			return "", fmt.Errorf("failed to close METADATA: %w", err)
		}
	}

	return "", fmt.Errorf("Version header not found in wheel METADATA")
}

// readTopLevelFromWheel reads the top-level module name from top_level.txt inside a wheel.
// If top_level.txt is not present (e.g. hatchling builds), it infers the module name from the wheel contents.
func readTopLevelFromWheel(whlPath string) (string, error) {
	r, err := zip.OpenReader(whlPath)
	if err != nil {
		return "", fmt.Errorf("failed to open wheel: %w", err)
	}
	defer r.Close()

	// First pass: look for top_level.txt (setuptools behavior)
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".dist-info/top_level.txt") {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open top_level.txt: %w", err)
			}

			data, err := io.ReadAll(rc)
			closeErr := rc.Close()
			if err != nil {
				return "", fmt.Errorf("failed to read top_level.txt: %w", err)
			}
			if closeErr != nil {
				return "", fmt.Errorf("failed to close top_level.txt: %w", closeErr)
			}

			module := strings.TrimSpace(string(data))
			if module == "" {
				return "", fmt.Errorf("top_level.txt is empty")
			}
			if idx := strings.IndexByte(module, '\n'); idx >= 0 {
				module = strings.TrimSpace(module[:idx])
			}
			return module, nil
		}
	}

	// Second pass: infer from wheel contents (hatchling behavior)
	for _, f := range r.File {
		if !strings.Contains(f.Name, ".dist-info/") && !strings.Contains(f.Name, ".data/") {
			parts := strings.Split(f.Name, "/")
			if len(parts) >= 2 {
				topDir := parts[0]
				fileName := parts[1]
				if fileName == "policy.py" || fileName == "__init__.py" || fileName == "policy-definition.yaml" {
					return topDir, nil
				}
			}
		}
	}

	return "", fmt.Errorf("top_level.txt not found and could not infer top-level module from wheel contents")
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
