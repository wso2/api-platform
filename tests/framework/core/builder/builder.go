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

package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Image describes one image produced by a component build.
type Image struct {
	Name       string
	Dockerfile string
	Context    string
}

// Spec describes how a catalog component is built from source.
type Spec struct {
	Component           string
	SourceDir           string
	Images              []Image
	SupportsCoverage    bool
	InstrumentByDefault bool
	Plan                func(root, version string, coverage bool) ([]Command, error)
}

// Command is one product build command. Directory is relative to the repository root.
type Command struct {
	Directory string
	Args      []string
	Env       map[string]string
}

// Runner executes product build commands.
type Runner interface {
	Run(context.Context, Command) error
}

// Request controls one source build.
type Request struct {
	RepoRoot string
	Version  string
	Coverage bool
	Runner   Runner
}

// Product associates a build specification with the version to produce.
type Product struct {
	Spec    Spec
	Version string
}

// Build validates a specification and executes its commands in declaration order.
func Build(ctx context.Context, spec Spec, req Request) error {
	if ctx == nil {
		return fmt.Errorf("build: context is required")
	}
	if err := validate(spec, req); err != nil {
		return err
	}
	commands, err := spec.Plan(req.RepoRoot, req.Version, req.Coverage)
	if err != nil {
		return fmt.Errorf("build %s: planning: %w", spec.Component, err)
	}
	if len(commands) == 0 {
		return fmt.Errorf("build %s: plan is empty", spec.Component)
	}
	for i, command := range commands {
		if err := validateCommand(req.RepoRoot, command); err != nil {
			return fmt.Errorf("build %s: command %d: %w", spec.Component, i+1, err)
		}
		if err := req.Runner.Run(ctx, command); err != nil {
			return fmt.Errorf("build %s: command %d: %w", spec.Component, i+1, err)
		}
	}
	return nil
}

// BuildMany executes independent component build plans in declaration order.
func BuildMany(ctx context.Context, specs []Spec, req Request) error {
	if len(specs) == 0 {
		return fmt.Errorf("build: at least one specification is required")
	}
	for _, spec := range specs {
		if err := Build(ctx, spec, req); err != nil {
			return err
		}
	}
	return nil
}

// BuildProducts builds products with independent versions in declaration order.
func BuildProducts(ctx context.Context, products []Product, repoRoot string, runner Runner) error {
	if len(products) == 0 {
		return fmt.Errorf("build: at least one product is required")
	}
	for _, product := range products {
		if err := Build(ctx, product.Spec, Request{
			RepoRoot: repoRoot,
			Version:  product.Version,
			Coverage: product.Spec.InstrumentByDefault,
			Runner:   runner,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ExecRunner runs commands through the host process environment.
type ExecRunner struct{}

// Run executes one command from the repository root.
func (ExecRunner) Run(ctx context.Context, command Command) error {
	if len(command.Args) == 0 {
		return fmt.Errorf("command arguments are required")
	}
	cmd := exec.CommandContext(ctx, command.Args[0], command.Args[1:]...)
	cmd.Dir = command.Directory
	cmd.Env = commandEnvironment(command.Env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func validate(spec Spec, req Request) error {
	if strings.TrimSpace(spec.Component) == "" {
		return fmt.Errorf("build: component is required")
	}
	if strings.TrimSpace(req.RepoRoot) == "" {
		return fmt.Errorf("build %s: repository root is required", spec.Component)
	}
	if strings.TrimSpace(req.Version) == "" {
		return fmt.Errorf("build %s: version is required", spec.Component)
	}
	if err := validateRelativePath(spec.SourceDir, "source directory"); err != nil {
		return fmt.Errorf("build %s: %w", spec.Component, err)
	}
	if len(spec.Images) == 0 {
		return fmt.Errorf("build %s: at least one image is required", spec.Component)
	}
	for i, image := range spec.Images {
		if strings.TrimSpace(image.Name) == "" {
			return fmt.Errorf("build %s: image %d has no name", spec.Component, i+1)
		}
		if err := validateRelativePath(image.Dockerfile, "Dockerfile"); err != nil {
			return fmt.Errorf("build %s: image %d: %w", spec.Component, i+1, err)
		}
		if err := validateRelativePath(image.Context, "build context"); err != nil {
			return fmt.Errorf("build %s: image %d: %w", spec.Component, i+1, err)
		}
	}
	if req.Runner == nil {
		return fmt.Errorf("build %s: runner is required", spec.Component)
	}
	if spec.Plan == nil {
		return fmt.Errorf("build %s: plan is required", spec.Component)
	}
	if req.Coverage && !spec.SupportsCoverage {
		return fmt.Errorf("build %s: coverage instrumentation is not supported", spec.Component)
	}
	return nil
}

func validateCommand(root string, command Command) error {
	if len(command.Args) == 0 || strings.TrimSpace(command.Args[0]) == "" {
		return fmt.Errorf("command is required")
	}
	if strings.TrimSpace(command.Directory) == "" {
		return fmt.Errorf("command directory is required")
	}
	abs, err := filepath.Abs(command.Directory)
	if err != nil {
		return fmt.Errorf("resolve command directory: %w", err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("command directory %q is outside repository root", command.Directory)
	}
	return nil
}

func commandEnvironment(overrides map[string]string) []string {
	values := append([]string(nil), os.Environ()...)
	for key, value := range overrides {
		values = setEnvironment(values, key, value)
	}
	return values
}

func validateRelativePath(value, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	clean := filepath.Clean(value)
	if filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s %q must be relative to the repository root", label, value)
	}
	return nil
}

func setEnvironment(values []string, key, value string) []string {
	prefix := key + "="
	for i, current := range values {
		if strings.HasPrefix(current, prefix) {
			values[i] = prefix + value
			return values
		}
	}
	return append(values, prefix+value)
}
