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

package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	tccompose "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// stagingDirName is the directory used for compose files and bind-mount sources.
const stagingDirName = ".wso2-apip-it-compose"

// EnvComposeNetwork carries the block's docker network name into a compose file, so it can
// declare that network external and join it instead of creating a private one.
const EnvComposeNetwork = "PG_NETWORK"

// ComposeStack is a running compose-backed component.
type ComposeStack struct {
	Instance *components.Instance

	stack    tccompose.ComposeStack
	def      *components.Definition
	stageDir string
	block    string
}

// LaunchCompose brings up a compose-backed component and presents it as one instance.
//
// The stack's services are an implementation detail: only the primary service's ports are
// resolved into endpoints, because that is the surface a test addresses.
func LaunchCompose(
	ctx context.Context, def *components.Definition, spec *components.ComposeSpec, opts Options,
) (*ComposeStack, error) {
	if ctx == nil {
		return nil, fmt.Errorf("runtime: compose launch requires a context")
	}
	if def == nil || spec == nil {
		return nil, fmt.Errorf("runtime: a compose component needs a definition and a spec")
	}
	if opts.Network == nil {
		return nil, fmt.Errorf("runtime: %s needs a block label for its compose stack", def)
	}
	if opts.Replicas > 1 {
		return nil, fmt.Errorf("runtime: compose component %s does not support replicas", def)
	}
	// Values substituted into the staged compose file. The network is the important one: a
	// compose stack that creates its own network cannot reach anything the framework
	// launched separately.
	substitutions := map[string]string{EnvComposeNetwork: opts.Network.Name()}
	for k, v := range spec.Env {
		substitutions[k] = v
	}
	for k, v := range opts.Env {
		substitutions[k] = v
	}

	stageDir, err := stageComposeFiles(def, spec, opts.RepoRoot, substitutions)
	if err != nil {
		return nil, err
	}

	composePath := filepath.Join(stageDir, spec.StagingName())

	// The stack identifier must be unique per block, or two blocks running the same
	// component would be treated by compose as the SAME stack and the second would adopt
	// the first's containers instead of creating its own.
	identifier, err := uniqueStackID(opts.Network.Block(), def.Name)
	if err != nil {
		return nil, err
	}

	created, err := tccompose.NewDockerComposeWith(
		tccompose.WithStackFiles(composePath),
		tccompose.StackIdentifier(identifier),
	)
	if err != nil {
		return nil, fmt.Errorf("runtime: preparing the compose stack for %s: %w", def, err)
	}
	// The fluent methods return the ComposeStack INTERFACE rather than the concrete type,
	// so the variable has to be the interface for chaining to typecheck.
	var stack tccompose.ComposeStack = created

	env := map[string]string{}
	for k, v := range spec.Env {
		env[k] = v
	}
	for k, v := range opts.Env {
		env[k] = v
	}
	// The block's network, so a compose file can join it rather than creating its own. A
	// compose component on a private network cannot reach anything the framework provisioned
	// separately — starting with its own runtime.
	env[EnvComposeNetwork] = opts.Network.Name()
	stack = stack.WithEnv(env)

	// Application-level readiness on the primary service, gated on the component's own
	// health declaration. Compose reporting the services as up says only that the
	// processes started.
	if def.Health != nil {
		strategy, err := composeWaitStrategy(def)
		if err != nil {
			return nil, err
		}
		// The health check may name a different service from the one tests address; see
		// HealthCheck.Service for why those are not always the same.
		target := def.Health.Service
		if target == "" {
			target = spec.PrimaryService
		}
		if !containsService(spec.Services, target) {
			return nil, fmt.Errorf("runtime: %s health targets service %q, which is not in %v",
				def, target, spec.Services)
		}
		stack = stack.WaitForService(target, strategy)
	}

	result := &ComposeStack{stack: stack, def: def, stageDir: stageDir, block: opts.Network.Block()}

	if err := stack.Up(ctx, tccompose.Wait(true)); err != nil {
		// Keep the handle so the caller can still read logs and tear down; a boot failure
		// with no diagnostics is the hardest kind to fix.
		return result, fmt.Errorf("runtime: bringing up %s: %w", def, err)
	}

	inst, err := composeInstance(ctx, def, spec, stack)
	if err != nil {
		return result, err
	}
	result.Instance = inst

	return result, nil
}

// composeWaitStrategy builds the readiness probe for the primary service.
func composeWaitStrategy(def *components.Definition) (wait.Strategy, error) {
	hc := def.Health
	endpoint, ok := def.Endpoint(hc.Endpoint)
	if !ok {
		return nil, fmt.Errorf("runtime: %s health references unknown endpoint %q", def, hc.Endpoint)
	}

	strategy := wait.ForHTTP(hc.Path).
		WithPort(strconv.Itoa(endpoint.Port) + "/tcp").
		WithStatusCodeMatcher(func(status int) bool { return status == hc.ExpectStatus }).
		WithStartupTimeout(hc.Timeout).
		WithPollInterval(hc.Interval)

	if endpoint.Scheme == "https" {
		// The component presents a generated certificate. Asserting the harness trusts
		// it proves nothing about the product; certificate behaviour is a product
		// assertion made by a test.
		strategy = strategy.WithTLS(true).WithAllowInsecure(true)
	}

	return strategy, nil
}

// composeInstance reads back the primary service's mapped ports.
func composeInstance(
	ctx context.Context, def *components.Definition, spec *components.ComposeSpec, stack tccompose.ComposeStack,
) (*components.Instance, error) {
	// Endpoints may live on DIFFERENT services — the gateway publishes its data plane on
	// the runtime and its management API on the controller — so each endpoint is resolved
	// against the service that actually publishes it. Reading only the primary service's
	// ports would silently drop every endpoint belonging to another service, and the
	// symptom is an unresolvable accessor much later rather than a boot failure.
	containers := map[string]*testcontainers.DockerContainer{}
	serviceOf := func(e components.Endpoint) string {
		if e.Service != "" {
			return e.Service
		}
		return spec.PrimaryService
	}

	getContainer := func(svc string) (*testcontainers.DockerContainer, error) {
		if c, ok := containers[svc]; ok {
			return c, nil
		}
		c, err := stack.ServiceContainer(ctx, svc)
		if err != nil {
			return nil, fmt.Errorf("runtime: locating service %q of %s: %w", svc, def, err)
		}
		containers[svc] = c
		return c, nil
	}

	primary, err := getContainer(spec.PrimaryService)
	if err != nil {
		return nil, err
	}
	host, err := primary.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("runtime: resolving host for %s: %w", def, err)
	}

	mapped := make(map[int]int, len(def.Endpoints))
	for _, e := range def.Endpoints {
		container, err := getContainer(serviceOf(e))
		if err != nil {
			return nil, err
		}
		p, err := container.MappedPort(ctx, strconv.Itoa(e.Port)+"/tcp")
		if err != nil {
			// An endpoint the compose file does not publish. Instance.URL reports the
			// specific missing endpoint far better than a boot failure could.
			continue
		}
		mapped[e.Port] = int(p.Num())
	}

	return components.NewInstance(def, 0, 1, host, mapped)
}

// stageComposeFiles materializes the compose file and everything it mounts into one
// directory, so compose's relative bind mounts resolve.
func stageComposeFiles(
	def *components.Definition, spec *components.ComposeSpec, repoRoot string, substitutions map[string]string,
) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("runtime: locating the home directory for compose staging: %w", err)
	}
	base := filepath.Join(home, stagingDirName)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("runtime: creating compose staging root %q: %w", base, err)
	}

	dir, err := os.MkdirTemp(base, strings.ReplaceAll(def.Name, "/", "-")+"-")
	if err != nil {
		return "", fmt.Errorf("runtime: creating a compose staging directory: %w", err)
	}

	copyIn := func(name, source string) error {
		if err := validateStagePath(name); err != nil {
			return err
		}
		target := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("runtime: creating %q: %w", filepath.Dir(target), err)
		}
		src := source
		if !filepath.IsAbs(src) && repoRoot != "" {
			src = filepath.Join(repoRoot, source)
		}
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("runtime: staging %s for %s: %w", source, def, err)
		}
		if info.IsDir() {
			return copyTree(src, target)
		}
		content, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("runtime: reading %q: %w", src, err)
		}
		return os.WriteFile(target, content, 0o644)
	}

	if err := copyIn(spec.StagingName(), spec.ComposeFile); err != nil {
		return "", err
	}
	// Substitute ${VAR} in the staged compose file ourselves rather than relying on the
	// library to pass environment through to compose's own interpolation.
	//
	// It does not, at least not for values supplied after the stack is constructed: the file
	// is parsed at construction, so a later WithEnv arrives too late. The failure is silent
	// and badly misleading — ${PG_NETWORK} resolved to an empty string, compose quietly
	// created its own default network instead of joining the block's, and the symptom was a
	// component unable to resolve a sibling by name. Image tags appeared to work only
	// because they carry `:-default` fallbacks.
	if err := interpolateStagedFile(filepath.Join(dir, spec.StagingName()), substitutions); err != nil {
		return "", err
	}
	for name, source := range spec.StagedFiles {
		if err := copyIn(name, source); err != nil {
			return "", err
		}
	}
	for name, content := range spec.GeneratedFiles {
		if err := validateStagePath(name); err != nil {
			return "", err
		}
		target := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", fmt.Errorf("runtime: creating %q: %w", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return "", fmt.Errorf("runtime: writing generated file %q: %w", target, err)
		}
	}

	return dir, nil
}

func validateStagePath(path string) error {
	if path == "" || filepath.IsAbs(path) {
		return fmt.Errorf("runtime: staged path %q must be relative and non-empty", path)
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("runtime: staged path %q escapes the staging directory", path)
	}
	return nil
}

// interpolateStagedFile replaces ${KEY} and ${KEY:-default} with supplied values.
//
// A key with no supplied value is left ALONE rather than blanked, so its `:-default` still
// applies and an unknown variable stays visible in the file instead of silently vanishing.
func interpolateStagedFile(path string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("runtime: reading staged compose file %q: %w", path, err)
	}

	text := string(content)
	for key, value := range values {
		if value == "" {
			continue
		}
		// The defaulted form first, so ${K:-x} is not left with a dangling suffix.
		text = replaceDefaulted(text, key, value)
		text = strings.ReplaceAll(text, "${"+key+"}", value)
	}

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return fmt.Errorf("runtime: writing staged compose file %q: %w", path, err)
	}
	return nil
}

// replaceDefaulted rewrites every ${KEY:-anything} occurrence to value.
func replaceDefaulted(text, key, value string) string {
	prefix := "${" + key + ":-"
	for {
		start := strings.Index(text, prefix)
		if start < 0 {
			return text
		}
		end := strings.Index(text[start:], "}")
		if end < 0 {
			return text // unterminated; leave it for compose to complain about
		}
		text = text[:start] + value + text[start+end+1:]
	}
}

func copyTree(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("runtime: reading %q: %w", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(s, d); err != nil {
				return err
			}
			continue
		}
		content, err := os.ReadFile(s)
		if err != nil {
			return fmt.Errorf("runtime: reading %q: %w", s, err)
		}
		if err := os.WriteFile(d, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// uniqueStackID keeps two blocks' stacks distinct.
func uniqueStackID(block, component string) (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("runtime: generating a compose stack id: %w", err)
	}
	sanitize := func(s string) string {
		return strings.ToLower(strings.NewReplacer("/", "-", "_", "-", " ", "-").Replace(s))
	}
	return fmt.Sprintf("%s-%s-%s", sanitize(block), sanitize(component), hex.EncodeToString(raw)), nil
}

// Stop tears the stack down and removes its staging directory.
//
// Idempotent: teardown runs on both the success and failure paths.
func (c *ComposeStack) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	var err error
	if c.stack != nil {
		stack := c.stack
		// Remove volumes too — a compose stack's anonymous volumes otherwise accumulate
		// across every block that ever ran.
		err = stack.Down(ctx, tccompose.RemoveOrphans(true), tccompose.RemoveVolumes(true))
		if err == nil {
			c.stack = nil
		}
	}
	if err == nil && c.stageDir != "" {
		if removeErr := os.RemoveAll(c.stageDir); removeErr != nil {
			err = fmt.Errorf("runtime: removing compose staging directory %q: %w", c.stageDir, removeErr)
		} else {
			c.stageDir = ""
		}
	}
	if err != nil {
		return fmt.Errorf("runtime: tearing down %s: %w", c.def, err)
	}
	return nil
}

// StageDir is where the stack's files were materialized, for diagnosing a config problem.
func (c *ComposeStack) StageDir() string {
	if c == nil {
		return ""
	}
	return c.stageDir
}

// Exec runs a command inside one of the stack's services and returns its output.
func (c *ComposeStack) Exec(ctx context.Context, service string, cmd []string) (string, error) {
	if c == nil || c.stack == nil {
		return "", fmt.Errorf("runtime: no compose stack to exec in")
	}
	container, err := c.stack.ServiceContainer(ctx, service)
	if err != nil {
		return "", fmt.Errorf("runtime: locating service %q: %w", service, err)
	}
	// See Container.Exec: without Multiplexed() the docker stream's framing bytes appear in
	// the output and break exact comparisons.
	code, reader, err := container.Exec(ctx, cmd, tcexec.Multiplexed())
	out := ""
	var readErr error
	if reader != nil {
		out, readErr = readAllString(reader)
	}
	if err != nil {
		if readErr != nil {
			return out, errors.Join(fmt.Errorf("runtime: exec in service %q: %w", service, err),
				fmt.Errorf("runtime: reading exec output: %w", readErr))
		}
		return out, fmt.Errorf("runtime: exec in service %q: %w", service, err)
	}
	if readErr != nil {
		return out, fmt.Errorf("runtime: reading exec output: %w", readErr)
	}
	if code != 0 {
		return out, fmt.Errorf("runtime: exec in service %q exited %d: %s", service, code, strings.TrimSpace(out))
	}
	return out, nil
}

// Logs returns each service's log output, concatenated and labelled.
//
// Essential on the boot-failure path: a compose stack that fails its readiness gate
// otherwise reports only "context deadline exceeded", which says nothing about WHICH
// service failed or why. Best-effort per service — a container that never started has no
// logs, and that should not mask the logs of one that did.
func (c *ComposeStack) Logs(ctx context.Context) string {
	if c == nil || c.stack == nil {
		return "(no stack to read logs from)"
	}
	var b strings.Builder
	for _, svc := range c.services() {
		fmt.Fprintf(&b, "───── %s ─────\n", svc)
		container, err := c.stack.ServiceContainer(ctx, svc)
		if err != nil {
			fmt.Fprintf(&b, "(could not locate service: %v)\n", err)
			continue
		}
		rc, err := container.Logs(ctx)
		if err != nil {
			fmt.Fprintf(&b, "(could not read logs: %v)\n", err)
			continue
		}
		text, readErr := readAllString(rc)
		_ = rc.Close()
		if readErr != nil {
			fmt.Fprintf(&b, "(log read failed: %v)\n", readErr)
			continue
		}
		b.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func containsService(services []string, name string) bool {
	for _, s := range services {
		if s == name {
			return true
		}
	}
	return false
}

func (c *ComposeStack) services() []string {
	if c.def != nil && c.def.Compose != nil {
		return c.def.Compose.Services
	}
	return nil
}

// StopService stops ONE service in the stack, leaving the rest running.
//
// The topology stays otherwise intact, which is the point: a test that stops the control
// plane is asking what the data plane does while its control plane is gone, not what a
// half-torn-down block does.
//
// The container is stopped, NOT removed, so StartService can bring the same one back with its
// state and its published ports unchanged. Removing and recreating would hand the service new
// ports mid-scenario and invalidate every URL already published to the block.
func (c *ComposeStack) StopService(ctx context.Context, service string) error {
	container, err := c.serviceContainer(ctx, service)
	if err != nil {
		return err
	}
	timeout := 30 * time.Second
	if err := container.Stop(ctx, &timeout); err != nil {
		return fmt.Errorf("runtime: stopping service %q: %w", service, err)
	}
	return nil
}

// StartService starts a service previously stopped by StopService, then re-resolves ports.
//
// The refresh matters as much here as in RestartService: docker assigns NEW published ports
// when a stopped container starts, so a component brought back up is reachable at an address
// nobody holds. Without this the caller sees connection-refused against the old port and
// concludes the service failed to start, when it started perfectly well somewhere else.
func (c *ComposeStack) StartService(ctx context.Context, service string) error {
	container, err := c.serviceContainer(ctx, service)
	if err != nil {
		return err
	}
	if err := container.Start(ctx); err != nil {
		return fmt.Errorf("runtime: starting service %q: %w", service, err)
	}
	return c.RefreshPorts(ctx)
}

// RestartService stops and starts one service, then RE-RESOLVES the component's host ports.
//
// The refresh is not optional. Docker frees a container's published ports when it stops and
// assigns new ones when it starts — measured on this host as 33246 -> 33247 for a plain
// stop/start. Without re-resolving, every accessor built at boot points at a port nothing is
// listening on, and the next request fails with a bare "connection refused" that names neither
// the restart nor the port.
//
// Readiness remains the CALLER's problem. "Running" and "ready" are different facts, and a
// test of restart recovery usually wants to watch readiness return rather than have the
// framework paper over the gap — see the `wait for the ... component to be healthy` step.
func (c *ComposeStack) RestartService(ctx context.Context, service string) error {
	if err := c.StopService(ctx, service); err != nil {
		return err
	}
	// StartService already refreshes.
	return c.StartService(ctx, service)
}

// RefreshPorts re-reads every endpoint's host port and updates the component's Instance.
//
// Applies to the WHOLE component, not just the restarted service: a compose component's
// endpoints may span services, and re-reading all of them is both cheap and impossible to get
// selectively wrong.
func (c *ComposeStack) RefreshPorts(ctx context.Context) error {
	if c == nil || c.stack == nil || c.Instance == nil || c.def == nil || c.def.Compose == nil {
		return fmt.Errorf("runtime: no compose stack to refresh")
	}

	mapped := make(map[int]int, len(c.def.Endpoints))
	for _, e := range c.def.Endpoints {
		service := c.def.Compose.PrimaryService
		if e.Service != "" {
			service = e.Service
		}
		container, err := c.stack.ServiceContainer(ctx, service)
		if err != nil {
			return fmt.Errorf("runtime: locating service %q while refreshing ports: %w", service, err)
		}
		p, err := container.MappedPort(ctx, strconv.Itoa(e.Port)+"/tcp")
		if err != nil {
			// Same tolerance as the initial mapping: an endpoint the compose file does not
			// publish is reported by Instance.URL, which names it, rather than failing here.
			continue
		}
		mapped[e.Port] = int(p.Num())
	}

	c.Instance.RefreshPorts(mapped)
	return nil
}

func (c *ComposeStack) serviceContainer(ctx context.Context, service string) (testcontainers.Container, error) {
	if c == nil || c.stack == nil {
		return nil, fmt.Errorf("runtime: no compose stack to control")
	}
	container, err := c.stack.ServiceContainer(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("runtime: locating service %q: %w", service, err)
	}
	return container, nil
}

// Services lists the compose services this stack runs.
func (c *ComposeStack) Services() []string {
	if c == nil || c.def == nil || c.def.Compose == nil {
		return nil
	}
	return append([]string(nil), c.def.Compose.Services...)
}

// PrimaryService is the service whose ports back the component's endpoints.
func (c *ComposeStack) PrimaryService() string {
	if c == nil || c.def == nil || c.def.Compose == nil {
		return ""
	}
	return c.def.Compose.PrimaryService
}

// CoverageServices lists the services whose coverage artifacts a coverage run collects.
func (c *ComposeStack) CoverageServices() []components.CoverageService {
	if c == nil || c.def == nil || c.def.Compose == nil {
		return nil
	}
	return append([]components.CoverageService(nil), c.def.Compose.CoverageServices...)
}

// ServiceContainerID resolves one service's docker container ID. Resolve BEFORE stopping
// the service — the underlying lookup only sees running containers.
func (c *ComposeStack) ServiceContainerID(ctx context.Context, service string) (string, error) {
	container, err := c.serviceContainer(ctx, service)
	if err != nil {
		return "", err
	}
	return container.GetContainerID(), nil
}
