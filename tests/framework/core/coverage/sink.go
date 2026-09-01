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
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvOut overrides the default coverage output directory.
const EnvOut = "IT_COVERAGE_OUT"

// Sink stores counters in separate directories for each block and service.
type Sink struct {
	root string
}

// NewSink clears and recreates root for a new coverage run.
func NewSink(root string) (*Sink, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("coverage: the sink needs a root directory")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("coverage: resolving sink root %q: %w", root, err)
	}
	// A wipe target this close to the filesystem root is a config error, not a request.
	if home, _ := os.UserHomeDir(); abs == string(filepath.Separator) || (home != "" && abs == home) {
		return nil, fmt.Errorf("coverage: refusing to wipe %q as a sink root", abs)
	}
	if err := os.RemoveAll(abs); err != nil {
		return nil, fmt.Errorf("coverage: clearing sink root %q: %w", abs, err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("coverage: creating sink root %q: %w", abs, err)
	}
	return &Sink{root: abs}, nil
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
	dir := filepath.Join(s.root, sanitize(block), sanitize(service))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("coverage: creating %q: %w", dir, err)
	}
	return dir, nil
}

// sanitize makes a name a safe, collision-resistant path element.
func sanitize(name string) string {
	replaced := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-").Replace(name)
	if replaced == name {
		return replaced
	}
	digest := sha256.Sum256([]byte(name))
	return replaced + "-" + hex.EncodeToString(digest[:4])
}
