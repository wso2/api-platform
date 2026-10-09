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

package aiworkspacecli

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
	// envCLIBinary overrides the location of the ap binary under test.
	envCLIBinary = "AP_CLI_BINARY"

	// commandTimeout bounds one ap invocation.
	commandTimeout = 60 * time.Second

	// buildHint is appended to a missing-binary error so the fix is obvious.
	buildHint = "build it with 'make -C tests/framework ap-cli'"
)

// scratchKind removes the per-scenario HOME and workspace directories after every resource
// the CLI created has been deleted.
var scratchKind = cleanup.Kind{Name: "ap-cli-scratch", Order: 200}

// cliResult captures one ap invocation.
type cliResult struct {
	stdout string
	stderr string
	exit   int
}

func (r cliResult) combined() string { return r.stdout + r.stderr }

// cli runs the ap binary with an isolated HOME, so its configuration never touches the
// developer's real ~/.wso2ap, and a scratch workspace where projects are scaffolded.
type cli struct {
	bin       string
	home      string
	workspace string
}

// resolveBinary locates the ap binary: AP_CLI_BINARY when set, otherwise the build output
// of 'make -C cli/src build-skip-tests' in this checkout.
func resolveBinary() (string, error) {
	bin := os.Getenv(envCLIBinary)
	if bin == "" {
		root, ok := shared.RepoRootFromCallerFile()
		if !ok {
			return "", fmt.Errorf("cannot locate the repository root; set %s or %s", envCLIBinary, buildHint)
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

// newCLI creates the isolated HOME and workspace and registers them for cleanup, so a failing
// scenario still removes them.
func newCLI(ctx context.Context, bin string) (*cli, error) {
	home, err := os.MkdirTemp("", "aiwscli-home-*")
	if err != nil {
		return nil, fmt.Errorf("creating the CLI home: %w", err)
	}
	workspace, err := os.MkdirTemp("", "aiwscli-work-*")
	if err != nil {
		_ = os.RemoveAll(home)
		return nil, fmt.Errorf("creating the CLI workspace: %w", err)
	}
	c := &cli{bin: bin, home: home, workspace: workspace}
	for _, dir := range []string{home, workspace} {
		if err := registerScratch(ctx, dir); err != nil {
			_ = os.RemoveAll(home)
			_ = os.RemoveAll(workspace)
			return nil, err
		}
	}
	return c, nil
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

// run executes ap with extraEnv appended to the process environment. A non-zero exit is
// returned in the result; an error means the process could not run at all.
func (c *cli) run(ctx context.Context, extraEnv []string, args ...string) (cliResult, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Dir = c.workspace
	cmd.Env = append(os.Environ(), "AP_NO_COLOR=true", "HOME="+c.home)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	res := cliResult{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.exit = exitErr.ExitCode()
		return res, nil
	}
	if err != nil {
		res.exit = -1
		return res, fmt.Errorf("running ap %s: %w", strings.Join(args, " "), err)
	}
	return res, nil
}
