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

package logcapture

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// EnvOut overrides the default log capture output directory.
const EnvOut = "IT_LOG_OUT"

// Sink resolves where each block's combined container log file is written.
type Sink struct {
	root string
}

// NewSink creates a unique directory for a new log capture run beneath root, isolated
// beneath its own ".run-*" directory so a stale local run's log files are never mixed
// with a fresh run's.
func NewSink(root string) (*Sink, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("logcapture: the sink needs a root directory")
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("logcapture: resolving sink root %q: %w", root, err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("logcapture: creating sink base %q: %w", base, err)
	}
	runDir, err := os.MkdirTemp(base, ".run-")
	if err != nil {
		return nil, fmt.Errorf("logcapture: creating sink run directory beneath %q: %w", base, err)
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

// FileFor returns the combined-log file path for the named block, creating its
// containing directory. Safe to call concurrently for different blocks.
func (s *Sink) FileFor(block string) (string, error) {
	return s.fileIn(blocksDir, "block", block)
}

// SharedFileFor returns the log file path for a component shared across blocks, creating
// its containing directory. Shared components are started once and outlive every block, so
// their output belongs to the run rather than to whichever block happened to start them.
// Safe to call concurrently for different components.
func (s *Sink) SharedFileFor(component string) (string, error) {
	return s.fileIn(sharedDir, "component", component)
}

// fileIn resolves one log file beneath a subdirectory of the sink root, rejecting any name
// that would not land strictly inside it.
func (s *Sink) fileIn(sub, kind, name string) (string, error) {
	if s == nil || strings.TrimSpace(s.root) == "" {
		return "", fmt.Errorf("logcapture: the sink is not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("logcapture: a log file needs a %s name", kind)
	}
	safe, err := sanitize(name)
	if err != nil {
		return "", fmt.Errorf("logcapture: invalid %s name %q: %w", kind, name, err)
	}
	dir, err := logDir(s.root, sub)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("logcapture: creating %q: %w", dir, err)
	}
	path := filepath.Join(dir, safe+".log")
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("logcapture: log file %q is not strictly below %q", path, dir)
	}
	return path, nil
}

// sanitize makes a name a safe, collision-resistant path element. Mirrors
// coverage.sanitize (core/coverage/sink.go) — kept as its own small copy rather than
// a shared dependency between the two packages, since each owns its own directory
// layout beneath the sink root.
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

// Log files live in one of two directories beneath the run root: per-block combined output,
// and output from components shared across blocks.
const (
	blocksDir = "blocks"
	sharedDir = "shared"
)

func logDir(root, sub string) (string, error) {
	dir, err := filepath.Abs(filepath.Join(root, sub))
	if err != nil {
		return "", fmt.Errorf("logcapture: resolving %s log directory: %w", sub, err)
	}
	return dir, nil
}
