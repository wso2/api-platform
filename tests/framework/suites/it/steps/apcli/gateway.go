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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
	"github.com/wso2/api-platform/tests/framework/suites/it/steps/platformgateway"
)

const (
	// platformGatewayTarget names the topology's platform gateway in the gateway step.
	platformGatewayTarget = "the platform gateway"

	// resourcePrefix marks an argument that names a file under the suite's feature root.
	resourcePrefix = "resources/"

	envGatewayUsername = "WSO2AP_GW_USERNAME"
	envGatewayPassword = "WSO2AP_GW_PASSWORD"
)

// gatewayAtTarget extracts the server URL from a `a gateway at "<url>"` target.
var gatewayAtTarget = regexp.MustCompile(`^a gateway at "([^"]*)"$`)

// cliGroups maps the resource kinds the apply step deploys to the ap gateway command group
// that reads and deletes them.
var cliGroups = map[string]string{
	"API":       "rest-api",
	"MCP proxy": "mcp",
}

// Steps holds one scenario's CLI session. Register builds a fresh instance for every
// scenario, so nothing here is shared between scenarios or runners.
type Steps struct {
	topo        *runtime.Topology
	featureRoot string

	session *Session
	// env carries the platform gateway's credentials to every command once the scenario has
	// added that gateway, so they are never written to the CLI configuration.
	env  []string
	last Result
	ran  bool
	// deployed maps "<group>/<id>" to the cleanup kind of each resource the scenario applied.
	deployed map[string]cleanup.Kind
}

// Register binds the ap CLI steps.
func Register(sc *godog.ScenarioContext, topo *runtime.Topology, featureRoot string) {
	s := &Steps{topo: topo, featureRoot: featureRoot, deployed: map[string]cleanup.Kind{}}
	sc.Step(`^the "ap" CLI is available$`, s.cliAvailable)
	sc.Step(`^the CLI uses (the platform gateway|a gateway at "[^"]*") as "([^"]*)"$`, s.useGateway)
	sc.Step(`^I run ap with arguments "([^"]*)"$`, s.runWithArguments)
	sc.Step(`^I apply (API|MCP proxy) from "([^"]*)" through the CLI with values:$`, s.applyFromTemplate)
	sc.Step(`^the CLI command should (succeed|fail)$`, s.commandOutcome)
	sc.Step(`^the CLI (output|error output) should (contain|not contain) "([^"]*)"$`, s.outputContains)
}

// execute runs one CLI command and publishes its result for the assertions that follow. The
// previous result is cleared first so a failure to run cannot leave a stale one to assert on.
func (s *Steps) execute(ctx context.Context, args ...string) (Result, error) {
	s.last, s.ran = Result{}, false
	session, err := s.ensureSession(ctx)
	if err != nil {
		return Result{}, err
	}
	res, err := session.Run(ctx, s.env, args...)
	if err != nil {
		return Result{}, err
	}
	s.last, s.ran = res, true
	return res, nil
}

// ensureSession returns the scenario's CLI session, creating it on first use.
func (s *Steps) ensureSession(ctx context.Context) (*Session, error) {
	if s.session != nil {
		return s.session, nil
	}
	bin, err := ResolveBinary()
	if err != nil {
		return nil, err
	}
	session, err := NewSession(ctx, bin)
	if err != nil {
		return nil, err
	}
	s.session = session
	return session, nil
}

// mustSucceed runs a command a step depends on, failing the step when ap exits non-zero.
func (s *Steps) mustSucceed(ctx context.Context, args ...string) error {
	res, err := s.execute(ctx, args...)
	if err != nil {
		return err
	}
	if res.Exit != 0 {
		return fmt.Errorf("ap %s failed (exit %d):\n%s", strings.Join(args, " "), res.Exit, res.Combined())
	}
	return nil
}

// cliAvailable verifies the binary runs. A missing binary fails the scenario: a skipped test
// would leave the build green.
func (s *Steps) cliAvailable(ctx context.Context) error {
	return s.mustSucceed(ctx, "version")
}

// useGateway adds a gateway to the CLI configuration under a display name and makes it the
// current gateway. The platform gateway is added with its management and admin addresses and
// basic authentication; any other target is added by URL alone, without authentication.
func (s *Steps) useGateway(ctx context.Context, target, name string) error {
	name, err := stepscommon.Expand(ctx, name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("a CLI gateway needs a display name")
	}
	args, env, err := s.gatewayAddArguments(ctx, target, name)
	if err != nil {
		return err
	}
	if env != nil {
		s.env = env
	}
	if err := s.mustSucceed(ctx, args...); err != nil {
		return err
	}
	return s.mustSucceed(ctx, "gateway", "use", "--display-name", name)
}

// gatewayAddArguments builds the `ap gateway add` arguments for a target, and the environment
// carrying its credentials when it needs them.
func (s *Steps) gatewayAddArguments(ctx context.Context, target, name string) ([]string, []string, error) {
	if target == platformGatewayTarget {
		if s.topo == nil {
			return nil, nil, errors.New("the platform gateway is not part of this topology")
		}
		management, admin, err := platformgateway.ControllerEndpoints(s.topo)
		if err != nil {
			return nil, nil, err
		}
		args := []string{"gateway", "add", "--display-name", name, "--server", management,
			"--admin-server", admin, "--auth", "basic", "--no-interactive"}
		env := []string{
			envGatewayUsername + "=" + s.topo.Admin.Username,
			envGatewayPassword + "=" + s.topo.Admin.Password,
		}
		return args, env, nil
	}
	match := gatewayAtTarget.FindStringSubmatch(target)
	if match == nil {
		return nil, nil, fmt.Errorf("unknown CLI gateway target %q", target)
	}
	server, err := stepscommon.Expand(ctx, match[1])
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(server) == "" {
		return nil, nil, errors.New("a CLI gateway needs a server URL")
	}
	return []string{"gateway", "add", "--display-name", name, "--server", server}, nil, nil
}

// runWithArguments runs ap with whitespace-separated arguments. Scenario values are expanded,
// and an argument naming a file under resources/ resolves against the suite's feature root.
// A successful delete of a resource the scenario applied removes it from cleanup.
func (s *Steps) runWithArguments(ctx context.Context, arguments string) error {
	args, err := s.arguments(ctx, arguments)
	if err != nil {
		return err
	}
	res, err := s.execute(ctx, args...)
	if err != nil {
		return err
	}
	if res.Exit == 0 {
		s.forgetDeleted(ctx, args)
	}
	return nil
}

func (s *Steps) arguments(ctx context.Context, arguments string) ([]string, error) {
	expanded, err := stepscommon.Expand(ctx, arguments)
	if err != nil {
		return nil, err
	}
	args := strings.Fields(expanded)
	if len(args) == 0 {
		return nil, errors.New("ap needs at least one argument")
	}
	for i, arg := range args {
		if !strings.HasPrefix(arg, resourcePrefix) {
			continue
		}
		path, err := stepscommon.ResourceTemplatePath(s.featureRoot, arg)
		if err != nil {
			return nil, err
		}
		args[i] = path
	}
	return args, nil
}

// forgetDeleted deregisters a resource the scenario applied once `ap gateway <group> delete
// --id <id>` has removed it.
func (s *Steps) forgetDeleted(ctx context.Context, args []string) {
	if len(args) < 3 || args[0] != "gateway" || args[2] != "delete" {
		return
	}
	id := flagValue(args, "--id")
	kind, ok := s.deployed[args[1]+"/"+id]
	if !ok {
		return
	}
	if reg, err := cleanup.Of(ctx); err == nil {
		reg.Deregister(kind, id)
	}
	delete(s.deployed, args[1]+"/"+id)
}

// flagValue returns the value following a flag, or "" when the flag is absent or last.
func flagValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

// applyFromTemplate renders the canonical template of a resource kind, writes it to the CLI
// workspace and deploys it to the current gateway with `ap gateway apply`. The resource is
// registered for cleanup as soon as the CLI reports it applied, before any assertion can fail.
func (s *Steps) applyFromTemplate(ctx context.Context, kind, templateName string, table *godog.Table) error {
	group, ok := cliGroups[kind]
	if !ok {
		return fmt.Errorf("the CLI cannot apply resource kind %q", kind)
	}
	cleanupKind, err := platformgateway.CleanupKindForResource(kind)
	if err != nil {
		return err
	}
	definition, err := platformgateway.RenderResourceTemplate(ctx, s.topo, s.featureRoot, kind, templateName, table)
	if err != nil {
		return err
	}
	name, err := resourceName(definition)
	if err != nil {
		return err
	}
	session, err := s.ensureSession(ctx)
	if err != nil {
		return err
	}
	file := filepath.Join(session.Workspace(), name+".yaml")
	if err := os.WriteFile(file, []byte(definition), 0o600); err != nil {
		return fmt.Errorf("writing %s %q for the CLI: %w", kind, name, err)
	}
	if err := s.mustSucceed(ctx, "gateway", "apply", "-f", file); err != nil {
		return err
	}
	applied := s.last
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: cleanupKind, ID: name, Actor: "admin", Description: "applied through the ap CLI",
	}); err != nil {
		if _, deleteErr := s.execute(ctx, "gateway", group, "delete", "--id", name); deleteErr != nil {
			return fmt.Errorf("registering %s %q for cleanup: %w; compensation failed: %v", kind, name, err, deleteErr)
		}
		return fmt.Errorf("registering %s %q for cleanup: %w", kind, name, err)
	}
	s.deployed[group+"/"+name] = cleanupKind
	s.last = applied
	return nil
}

// resourceName returns metadata.name of a rendered resource document.
func resourceName(definition string) (string, error) {
	var document struct {
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(definition), &document); err != nil {
		return "", fmt.Errorf("reading the rendered resource: %w", err)
	}
	if strings.TrimSpace(document.Metadata.Name) == "" {
		return "", errors.New("the rendered resource has no metadata.name")
	}
	if strings.ContainsAny(document.Metadata.Name, `/\`) {
		return "", fmt.Errorf("metadata.name %q is not a valid resource name", document.Metadata.Name)
	}
	return document.Metadata.Name, nil
}

// commandOutcome asserts the exit status of the last command.
func (s *Steps) commandOutcome(_ context.Context, outcome string) error {
	if !s.ran {
		return errors.New("no ap command has run in this scenario")
	}
	succeeded := s.last.Exit == 0
	if outcome == "succeed" && !succeeded {
		return fmt.Errorf("expected ap to succeed, it exited %d:\n%s", s.last.Exit, s.last.Combined())
	}
	if outcome == "fail" && succeeded {
		return fmt.Errorf("expected ap to fail, it exited 0:\n%s", s.last.Combined())
	}
	return nil
}

// outputContains asserts on the last command's output: stdout and stderr together, or stderr
// alone.
func (s *Steps) outputContains(ctx context.Context, stream, mode, text string) error {
	if !s.ran {
		return errors.New("no ap command has run in this scenario")
	}
	want, err := stepscommon.Expand(ctx, text)
	if err != nil {
		return err
	}
	out := s.last.Combined()
	if stream == "error output" {
		out = s.last.Stderr
	}
	contains := strings.Contains(out, want)
	if mode == "contain" && !contains {
		return fmt.Errorf("expected the CLI %s to contain %q, got:\n%s", stream, want, out)
	}
	if mode == "not contain" && contains {
		return fmt.Errorf("expected the CLI %s not to contain %q, got:\n%s", stream, want, out)
	}
	return nil
}
