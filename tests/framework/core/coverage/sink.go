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

// Package coverage collects coverage artifacts from a block's containers.
package coverage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// WriteIstanbulReport writes one browser coverage snapshot atomically.
func WriteIstanbulReport(filename string, report any) error {
	if strings.TrimSpace(filename) == "" {
		return fmt.Errorf("coverage: Istanbul report path is required")
	}
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("coverage: encoding Istanbul report: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("coverage: creating Istanbul report directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(filename), ".istanbul-*")
	if err != nil {
		return fmt.Errorf("coverage: creating Istanbul report temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("coverage: writing Istanbul report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("coverage: closing Istanbul report: %w", err)
	}
	if err := os.Rename(tmpName, filename); err != nil {
		return fmt.Errorf("coverage: publishing Istanbul report: %w", err)
	}
	return nil
}

// EnvOut overrides the default coverage output directory.
const EnvOut = "IT_COVERAGE_OUT"

// Sink stores counters in separate directories for each block and service.
type Sink struct {
	root string
}

var browserArtifactID atomic.Uint64

// NewSink creates a unique directory for a new coverage run beneath root.
func NewSink(root string) (*Sink, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("coverage: the sink needs a root directory")
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("coverage: resolving sink root %q: %w", root, err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("coverage: creating sink base %q: %w", base, err)
	}
	runDir, err := os.MkdirTemp(base, ".run-")
	if err != nil {
		return nil, fmt.Errorf("coverage: creating sink run directory beneath %q: %w", base, err)
	}
	return &Sink{root: runDir}, nil
}

// Root returns the sink's absolute root directory.
func (s *Sink) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

// Dir returns, creating it if needed, the directory for one container's counters.
// Safe to call concurrently — parallel blocks collect at the same time.
func (s *Sink) Dir(block, service string) (string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return "", fmt.Errorf("coverage: the sink is not initialized")
	}
	if strings.TrimSpace(block) == "" || strings.TrimSpace(service) == "" {
		return "", fmt.Errorf("coverage: a counter directory needs a block and a service, got %q/%q", block, service)
	}
	blockName, err := sanitize(block)
	if err != nil {
		return "", fmt.Errorf("coverage: invalid block name %q: %w", block, err)
	}
	serviceName, err := sanitize(service)
	if err != nil {
		return "", fmt.Errorf("coverage: invalid service name %q: %w", service, err)
	}
	dir, err := coveragePath(s.root, blockName, serviceName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("coverage: creating %q: %w", dir, err)
	}
	return dir, nil
}

// BrowserDir returns a unique directory for one browser scenario.
func (s *Sink) BrowserDir(block, scenario string) (string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return "", fmt.Errorf("coverage: the sink is not initialized")
	}
	if strings.TrimSpace(block) == "" || strings.TrimSpace(scenario) == "" {
		return "", fmt.Errorf("coverage: a browser directory needs a block and scenario")
	}
	blockName, err := sanitize(block)
	if err != nil {
		return "", fmt.Errorf("coverage: invalid block name %q: %w", block, err)
	}
	scenarioName, err := sanitize(scenario)
	if err != nil {
		return "", fmt.Errorf("coverage: invalid scenario name %q: %w", scenario, err)
	}
	id := browserArtifactID.Add(1)
	dir, err := coveragePath(s.root, "blocks", blockName, "browser", fmt.Sprintf("%s-%d", scenarioName, id))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("coverage: creating browser directory %q: %w", dir, err)
	}
	return dir, nil
}

// sanitize makes a name a safe, collision-resistant path element.
func sanitize(name string) (string, error) {
	decoded := name
	for {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return "", fmt.Errorf("invalid URL encoding")
		}
		if next == decoded {
			break
		}
		decoded = next
	}
	if strings.Contains(decoded, "\x00") {
		return "", fmt.Errorf("name contains a null byte")
	}
	for _, part := range strings.FieldsFunc(decoded, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == "." || part == ".." {
			return "", fmt.Errorf("name contains a traversal segment")
		}
	}
	replaced := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-").Replace(name)
	if replaced == name {
		return replaced, nil
	}
	digest := sha256.Sum256([]byte(name))
	return replaced + "-" + hex.EncodeToString(digest[:4]), nil
}

func coveragePath(root string, elements ...string) (string, error) {
	raw, err := filepath.Abs(filepath.Join(root, "raw"))
	if err != nil {
		return "", fmt.Errorf("coverage: resolving raw coverage directory: %w", err)
	}
	path, err := filepath.Abs(filepath.Join(append([]string{raw}, elements...)...))
	if err != nil {
		return "", fmt.Errorf("coverage: resolving artifact directory: %w", err)
	}
	rel, err := filepath.Rel(raw, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("coverage: artifact directory %q is not strictly below raw coverage directory", path)
	}
	return path, nil
}
