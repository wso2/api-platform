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

package configinterpolate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// buildFuncMap assembles the template functions available in config interpolation.
// Both funcs are fail-closed: a missing required value returns an error, which
// aborts rendering and therefore aborts startup. Neither func ever logs or embeds
// the resolved value in an error.
func buildFuncMap(opts *Options, stats *Stats) template.FuncMap {
	return template.FuncMap{
		"env":  envFunc(stats),
		"file": fileFunc(opts, stats),
	}
}

// envFunc implements {{ env "KEY" ["default"] }}.
//
//   - {{ env "KEY" }}            -> value of KEY; error if KEY is unset or empty.
//   - {{ env "KEY" "fallback" }} -> value of KEY; "fallback" if KEY is unset or empty.
//
// A set-but-empty variable (KEY="") is treated as unset, mirroring bash
// ${KEY:?} / ${KEY:-default} semantics. Fails closed when no fallback is given.
func envFunc(stats *Stats) func(string, ...string) (string, error) {
	return func(key string, fallback ...string) (string, error) {
		stats.EnvRefs++
		if v, ok := os.LookupEnv(key); ok && v != "" {
			return v, nil
		}
		if len(fallback) > 0 {
			return fallback[0], nil
		}
		return "", fmt.Errorf("required env var %q is not found", key)
	}
}

// fileFunc implements {{ file "/path" }}. It reads the file contents (trailing
// whitespace trimmed, since secret files commonly end in a newline) and is always
// required — a missing, unreadable, disallowed, or oversize file aborts startup.
func fileFunc(opts *Options, stats *Stats) func(string) (string, error) {
	return func(path string) (string, error) {
		stats.FileRefs++
		data, err := readAllowedFile(path, opts)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(data), " \t\r\n"), nil
	}
}

// readAllowedFile enforces the file-access security rules before reading:
// null-byte/traversal rejection, allowlist containment on the input path, a
// symlink re-check against the allowlist roots (k8s Secret mounts symlink through
// ..data/… but stay under the root), and a size cap. Error messages are sterile:
// they name the operator-supplied path but never the file contents, the allowlist,
// or the byte limit.
func readAllowedFile(input string, opts *Options) ([]byte, error) {
	if strings.ContainsRune(input, '\x00') {
		return nil, fmt.Errorf("file %q is not in an allowed source directory", input)
	}
	cleaned := filepath.Clean(input)
	if strings.Contains(cleaned, "..") {
		return nil, fmt.Errorf("file %q is not in an allowed source directory", input)
	}
	if len(opts.FileAllowlist) == 0 {
		return nil, fmt.Errorf("file interpolation not permitted: no allowlist configured")
	}
	if !isAllowed(cleaned, opts.FileAllowlist) {
		return nil, fmt.Errorf("file %q is not in an allowed source directory", input)
	}

	f, err := os.Open(cleaned)
	if err != nil {
		return nil, fmt.Errorf("required file %q is not found", input)
	}
	defer f.Close()

	// Re-check the symlink-resolved target against the allowlist roots. The input
	// path already passed containment; here we ensure it does not symlink out of
	// the allowlist. k8s Secret mounts resolve to <root>/..data/<key>, still under
	// the root, so they pass; a symlink escaping the root is rejected.
	if resolved, err := filepath.EvalSymlinks(cleaned); err != nil {
		return nil, fmt.Errorf("required file %q is not found", input)
	} else if !isAllowedResolved(resolved, opts.FileAllowlist) {
		return nil, fmt.Errorf("file %q is not in an allowed source directory", input)
	}

	limited := io.LimitReader(f, opts.MaxFileBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("required file %q is not found", input)
	}
	if int64(len(data)) > opts.MaxFileBytes {
		return nil, fmt.Errorf("file %q exceeds the maximum allowed size", input)
	}
	return data, nil
}

// isAllowed reports whether path is contained in one of the allowlist directories.
// The separator suffix prevents a partial-prefix match (e.g. "/secrets/gw" must not
// match "/secrets/gw-other").
func isAllowed(path string, allow []string) bool {
	for _, d := range allow {
		root := filepath.Clean(d) + string(os.PathSeparator)
		if strings.HasPrefix(path, root) {
			return true
		}
	}
	return false
}

// isAllowedResolved is isAllowed for a fully symlink-resolved target path: it also
// resolves symlinks in each allowlist root before comparing, so that both sides are
// in their real-path form (e.g. macOS /var -> /private/var, or a symlinked mount
// root). A root that cannot be resolved (e.g. does not exist) falls back to its
// cleaned form — harmless, since no readable file could live under a missing root.
func isAllowedResolved(path string, allow []string) bool {
	for _, d := range allow {
		root := filepath.Clean(d)
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			root = resolved
		}
		if strings.HasPrefix(path, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}
