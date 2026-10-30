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

package apcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
)

const (
	// EnvBinary overrides the location of the ap binary under test.
	EnvBinary = "AP_CLI_BINARY"

	// commandTimeout bounds one ap invocation.
	commandTimeout = 60 * time.Second

	// buildHint is appended to a missing-binary error so the fix is obvious.
	buildHint = "build it with 'make -C tests/framework ap-cli'"
)

// scratchKind removes the per-scenario HOME and workspace directories after every resource
// the CLI created has been deleted.
var scratchKind = cleanup.Kind{Name: "ap-cli-scratch", Order: 200}

// Result captures one ap invocation.
type Result struct {
	Stdout string
	Stderr string
	Exit   int
}

// Combined returns stdout followed by stderr.
func (r Result) Combined() string { return r.Stdout + r.Stderr }

// Session runs the ap binary with an isolated HOME, so its configuration never touches the
// developer's real ~/.wso2ap, and from a scratch workspace that relative paths resolve against.
type Session struct {
	bin       string
	home      string
	workspace string
}

// Workspace returns the directory ap runs from.
func (s *Session) Workspace() string { return s.workspace }

// Home returns the HOME directory ap keeps its configuration under.
func (s *Session) Home() string { return s.home }

// ResolveBinary locates the ap binary: AP_CLI_BINARY when set, otherwise the build output
// of 'make -C cli/src build-skip-tests' in this checkout.
func ResolveBinary() (string, error) {
	bin := os.Getenv(EnvBinary)
	if bin == "" {
		root, ok := shared.RepoRootFromCallerFile()
		if !ok {
			return "", fmt.Errorf("cannot locate the repository root; set %s or %s", EnvBinary, buildHint)
		}
		bin = filepath.Join(root, "cli", "src", "build", "ap")
	}
	bin, err := filepath.Abs(bin)
	if err != nil {
		return "", fmt.Errorf("resolving the ap binary path: %w", err)
	}
	info, err := os.Stat(bin)
	if err != nil {
		return "", fmt.Errorf("ap binary not found at %s (%s): %w", bin, buildHint, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("ap binary path %s is a directory", bin)
	}
	return bin, nil
}

// NewSession creates the isolated HOME and workspace and registers them for cleanup, so a
// failing scenario still removes them.
func NewSession(ctx context.Context, bin string) (*Session, error) {
	home, err := os.MkdirTemp("", "apcli-home-*")
	if err != nil {
		return nil, fmt.Errorf("creating the CLI home: %w", err)
	}
	workspace, err := os.MkdirTemp("", "apcli-work-*")
	if err != nil {
		_ = os.RemoveAll(home)
		return nil, fmt.Errorf("creating the CLI workspace: %w", err)
	}
	s := &Session{bin: bin, home: home, workspace: workspace}
	for _, dir := range []string{home, workspace} {
		if err := registerScratch(ctx, dir); err != nil {
			_ = os.RemoveAll(home)
			_ = os.RemoveAll(workspace)
			return nil, err
		}
	}
	return s, nil
}

func registerScratch(ctx context.Context, dir string) error {
	reg, err := cleanup.Of(ctx)
	if err != nil {
		return err
	}
	if err := reg.RegisterDeleter(scratchKind, func(_ context.Context, res cleanup.Resource) error {
		return os.RemoveAll(res.ID)
	}); err != nil {
		return err
	}
	return reg.Register(cleanup.Resource{
		Kind: scratchKind, ID: dir, Actor: "ap-cli", Description: "ap CLI scratch directory",
	})
}

// Run executes ap with extraEnv appended to the process environment. A non-zero exit is
// returned in the result; an error means the process could not run at all.
func (s *Session) Run(ctx context.Context, extraEnv []string, args ...string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.bin, args...)
	cmd.Dir = s.workspace
	cmd.Env = append(os.Environ(), "AP_NO_COLOR=true", "HOME="+s.home)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.Exit = exitErr.ExitCode()
		return res, nil
	}
	if err != nil {
		res.Exit = -1
		return res, fmt.Errorf("running ap %s: %w", strings.Join(args, " "), err)
	}
	return res, nil
}
