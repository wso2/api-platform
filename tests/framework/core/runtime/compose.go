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
	"io"
	"log/slog"
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
	"github.com/wso2/api-platform/tests/framework/core/logcapture"
)

// stagingDirName is the directory used for compose files and bind-mount sources.
const stagingDirName = ".wso2-apip-it-compose"

// EnvComposeNetwork names the block network used by a compose stack.
const EnvComposeNetwork = "PG_NETWORK"

// EnvComposeCPULimit and EnvComposeMemoryLimitMB carry a compose component's declared
// resource limits into the stack's substitution env, for a service's own compose file to
// reference under its "deploy.resources.limits" (honoured by `docker compose up` without
// swarm mode). Set only when the definition declares a limit, so a compose file that does
// not reference them sees no behavior change.
const (
	EnvComposeCPULimit      = "APIP_CPU_LIMIT"
	EnvComposeMemoryLimitMB = "APIP_MEMORY_LIMIT_MB"
)

// ComposeStack is a running compose-backed component.
type ComposeStack struct {
	Instance *components.Instance

	stack    tccompose.ComposeStack
	def      *components.Definition
	stageDir string
	block    string

	// stopLogProducers stops each service's attached log producer, populated only when
	// the block is capturing container output.
	stopLogProducers []func() error
}

// LaunchCompose starts a compose-backed component and presents it as one instance.
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
	// Values substituted into the staged compose file.
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
	keepStageDir := true
	defer func() {
		if keepStageDir {
			_ = os.RemoveAll(stageDir)
		}
	}()

	composePath := filepath.Join(stageDir, spec.StagingName())

	// Keep stack identifiers unique per block.
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
	var stack tccompose.ComposeStack = created

	env := map[string]string{}
	for k, v := range spec.Env {
		env[k] = v
	}
	for k, v := range opts.Env {
		env[k] = v
	}
	// Join the block network so the stack can reach other components.
	env[EnvComposeNetwork] = opts.Network.Name()
	// A compose-backed component's Limits have no effect unless its own compose file opts
	// in by referencing these under "deploy.resources.limits" - unlike a raw container
	// (applyLimits, container.go), there is no host-config hook this runtime can apply on
	// the component's behalf here.
	if def.Limits.CPUs > 0 {
		env[EnvComposeCPULimit] = strconv.FormatFloat(def.Limits.CPUs, 'f', -1, 64)
	}
	if def.Limits.MemoryMB > 0 {
		env[EnvComposeMemoryLimitMB] = strconv.FormatInt(def.Limits.MemoryMB, 10)
	}
	stack = stack.WithEnv(env)

	// Wait for application-level readiness on the primary service.
	if def.Health != nil {
		strategy, err := composeWaitStrategy(def)
		if err != nil {
			return nil, err
		}
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
		cleanupErr := result.Stop(context.Background())
		return nil, fmt.Errorf("runtime: bringing up %s: %w", def, errors.Join(err, cleanupErr))
	}

	if opts.LogWriter != nil {
		stops, err := attachComposeLogProducers(ctx, stack, spec.Services, opts.LogWriter)
		result.stopLogProducers = stops
		if err != nil {
			cleanupErr := result.Stop(context.Background())
			return nil, fmt.Errorf("runtime: attaching log capture for %s: %w", def, errors.Join(err, cleanupErr))
		}
	}

	inst, err := composeInstance(ctx, def, spec, stack)
	if err != nil {
		cleanupErr := result.Stop(context.Background())
		return nil, errors.Join(err, cleanupErr)
	}
	result.Instance = inst
	keepStageDir = false

	return result, nil
}

// attachComposeLogProducers streams every service's stdout/stderr into writer, tagged
// with its service name. Compose services have no pre-creation log-consumer hook (unlike
// a raw container's ContainerRequest.LogConsumerCfg — see container.go's buildRequest),
// so this uses testcontainers' older FollowOutput/StartLogProducer API, the only
// mechanism available for a container the compose module already created. Returns the
// stop function for every service successfully attached, even when a later service
// fails, so the caller can still release what succeeded.
func attachComposeLogProducers(
	ctx context.Context, stack tccompose.ComposeStack, services []string, writer *logcapture.Writer,
) ([]func() error, error) {
	stops := make([]func() error, 0, len(services))
	for _, svc := range services {
		container, err := stack.ServiceContainer(ctx, svc)
		if err != nil {
			return stops, fmt.Errorf("runtime: locating service %q for log capture: %w", svc, err)
		}
		container.FollowOutput(writer.Consumer(svc)) //nolint:staticcheck // no LogConsumerCfg hook exists for an already-created compose service container
		if err := container.StartLogProducer(ctx); err != nil {
			return stops, fmt.Errorf("runtime: starting log producer for service %q: %w", svc, err)
		}
		stops = append(stops, container.StopLogProducer)
	}
	return stops, nil
}

// launchComposeWithRetry retries a failed compose boot up to attempts times. Unlike the
// raw-container retry path (launchWithRetry), a failed launch already tears itself down
// and stages a fresh directory and stack identifier on its own next call, so a retry
// needs nothing beyond calling launch again.
func launchComposeWithRetry(
	ctx context.Context, componentName string, attempts int,
	launch func(context.Context) (*ComposeStack, error),
) (*ComposeStack, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		stack, err := launch(ctx)
		if err == nil {
			return stack, nil
		}
		lastErr = err
		if attempt < attempts {
			slog.Warn("compose stack boot failed; retrying with a fresh stack",
				"component", componentName, "attempt", attempt, "of", attempts, "error", err)
		}
	}
	return nil, lastErr
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
		// The readiness probe accepts the component's generated certificate.
		strategy = strategy.WithTLS(true).WithAllowInsecure(true)
	}

	return strategy, nil
}

// composeInstance reads the mapped ports for each published endpoint.
func composeInstance(
	ctx context.Context, def *components.Definition, spec *components.ComposeSpec, stack tccompose.ComposeStack,
) (*components.Instance, error) {
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

// stageComposeFiles materializes the compose file and its bind mounts in one directory.
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
	keepDir := false
	defer func() {
		if !keepDir {
			_ = os.RemoveAll(dir)
		}
	}()

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
	// Resolve framework variables before compose parses the staged file.
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

	keepDir = true
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

// interpolateStagedFile replaces supplied ${KEY} and ${KEY:-default} variables.
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

// Stop tears down the stack and removes its staging directory.
func (c *ComposeStack) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	for _, stop := range c.stopLogProducers {
		_ = stop()
	}
	c.stopLogProducers = nil
	var err error
	if c.stack != nil {
		stack := c.stack
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

// CopyFileFromContainer reads one file out of a service's container.
//
// Used to read a component's own persisted state directly - an embedded SQLite database, for
// instance - when no product API exposes it. This is a snapshot, not a live handle: the file is
// fully read before this call returns, so a caller holds a copy from one instant, not a
// connection to the container's own open file.
func (c *ComposeStack) CopyFileFromContainer(ctx context.Context, service, path string) ([]byte, error) {
	container, err := c.serviceContainer(ctx, service)
	if err != nil {
		return nil, err
	}
	reader, err := container.CopyFileFromContainer(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("runtime: copying %q from service %q: %w", path, service, err)
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("runtime: reading %q from service %q: %w", path, service, err)
	}
	return data, nil
}

// Logs returns each service's log output, concatenated and labelled.
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

// StopService stops one service while leaving the rest of the stack running.
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

// StartService starts a stopped service and refreshes its published ports.
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

// RestartService stops and starts one service, then refreshes its published ports.
func (c *ComposeStack) RestartService(ctx context.Context, service string) error {
	if err := c.StopService(ctx, service); err != nil {
		return err
	}
	// StartService already refreshes.
	return c.StartService(ctx, service)
}

// RefreshPorts re-reads every endpoint's host port and updates the component instance.
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
			// Unpublished endpoints are reported by Instance.URL when accessed.
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

// ServiceContainerID resolves a running service's container ID.
func (c *ComposeStack) ServiceContainerID(ctx context.Context, service string) (string, error) {
	container, err := c.serviceContainer(ctx, service)
	if err != nil {
		return "", err
	}
	return container.GetContainerID(), nil
}
