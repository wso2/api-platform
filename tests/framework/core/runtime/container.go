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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	dockernet "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

const (
	labelBlock          = "io.wso2.apip.it.block"
	labelComponent      = "io.wso2.apip.it.component"
	labelFramework      = "io.wso2.apip.it"
	frameworkLabelValue = "integration-v2"

	// defaultStartTimeout bounds container start when a definition sets nothing.
	// Generous, because a cold image pull on a loaded runner is slow and a spurious
	// timeout here reads as a product failure.
	defaultStartTimeout = 5 * time.Minute
)

// Options contains the resolved values used to start one component instance.
type Options struct {
	// Network is the block's private network. Required.
	Network *Network

	// Ordinal and Replicas position this instance among its component's replicas.
	Ordinal  int
	Replicas int

	// Env is merged over the definition's static Env. This is where a resolved DSN
	// and any wiring-derived values arrive.
	Env map[string]string

	// ConfigContent is the assembled configuration file. When non-nil it is copied to
	// the definition's ConfigInjection.ContainerPath before start, because the
	// products read configuration once at boot.
	ConfigContent []byte

	// Files are extra host files to copy in, resolved to absolute paths already.
	Files []components.FileMount

	// RepoRoot resolves the relative paths in the definition's Files.
	RepoRoot string

	// StartTimeout overrides defaultStartTimeout.
	StartTimeout time.Duration

	// Arch overrides the detected architecture, for testing image substitution.
	Arch string

	// StableHostPorts publishes endpoints on explicitly selected host ports. It is required
	// for components that may be attached to multiple networks during their lifetime.
	StableHostPorts bool
}

// Container is a started component: its Instance for addressing, plus the handle needed
// to stop it and read its logs.
type Container struct {
	Instance  *components.Instance
	container testcontainers.Container
	def       *components.Definition
}

// Launch starts one instance of def and returns it once docker reports the container
// running and its wait strategy satisfied.
//
// This is CONTAINER-level readiness only. Application readiness — the health gate that
// distinguishes a listening socket from a server actually serving — is a separate phase
// the caller applies next. A container that accepts connections is not a component
// that works.
func Launch(ctx context.Context, def *components.Definition, opts Options) (*Container, error) {
	if ctx == nil {
		return nil, fmt.Errorf("runtime: launch requires a context")
	}
	if def == nil {
		return nil, fmt.Errorf("runtime: definition is required")
	}
	if opts.Network == nil {
		return nil, fmt.Errorf("runtime: %s needs a network", def)
	}
	replicas := opts.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	alias, err := components.AliasFor(def, opts.Ordinal, replicas)
	if err != nil {
		return nil, err
	}

	arch := opts.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}

	req, err := buildRequest(def, opts, alias, arch, replicas)
	if err != nil {
		return nil, err
	}

	started, err := startWithPortRetry(ctx, def, opts, alias, arch, replicas, req)
	if err != nil {
		// Return the container handle when we have one so the caller can still read
		// logs: a boot failure with no diagnostics is the hardest kind to fix.
		if started != nil {
			return &Container{container: started, def: def}, fmt.Errorf("runtime: starting %s: %w", def, err)
		}
		return nil, fmt.Errorf("runtime: starting %s: %w", def, err)
	}

	inst, err := buildInstance(ctx, def, started, opts.Ordinal, replicas)
	if err != nil {
		return &Container{container: started, def: def}, err
	}

	return &Container{Instance: inst, container: started, def: def}, nil
}

func buildRequest(
	def *components.Definition, opts Options, alias, arch string, replicas int,
) (*testcontainers.ContainerRequest, error) {
	exposed := make([]string, 0, len(def.Endpoints))
	var awaited []wait.Strategy
	bindings := dockernet.PortMap{}
	for _, e := range def.Endpoints {
		spec := strconv.Itoa(e.Port) + "/tcp"
		exposed = append(exposed, spec)
		if opts.StableHostPorts {
			hostPort, perr := freeHostPort()
			if perr != nil {
				return nil, fmt.Errorf("runtime: choosing a stable host port for %s/%s: %w",
					def.Name, e.Name, perr)
			}
			port, perr := dockernet.ParsePort(spec)
			if perr != nil {
				return nil, fmt.Errorf("runtime: parsing port %q for %s: %w", spec, def.Name, perr)
			}
			// A DECLARED binding is what docker re-publishes unchanged across network
			// churn. Left to pick an ephemeral port itself, it picks a NEW one each time.
			bindings[port] = []dockernet.PortBinding{{
				HostIP:   netip.AddrFrom4([4]byte{0, 0, 0, 0}),
				HostPort: strconv.Itoa(hostPort),
			}}
		}
		if e.AwaitListening {
			// Only endpoints explicitly marked as always-bound join the wait set.
			// Every port here must come up or start fails, so an optional listener
			// would hang the full timeout against a perfectly healthy container.
			awaited = append(awaited, wait.ForListeningPort(spec))
		}
	}

	env := make(map[string]string, len(def.Env)+len(opts.Env))
	for k, v := range def.Env {
		env[k] = v
	}
	for k, v := range opts.Env {
		env[k] = v // block-resolved values win over static definition values
	}

	files, err := resolveFiles(def, opts)
	if err != nil {
		return nil, err
	}

	timeout := opts.StartTimeout
	if timeout <= 0 {
		timeout = defaultStartTimeout
	}

	var strategy wait.Strategy
	if len(awaited) > 0 {
		strategy = wait.ForAll(awaited...).WithStartupTimeout(timeout)
	}

	req := &testcontainers.ContainerRequest{
		Image:          def.Image.Resolve(arch),
		ExposedPorts:   exposed,
		Env:            env,
		Networks:       []string{opts.Network.Name()},
		NetworkAliases: map[string][]string{opts.Network.Name(): {alias}},
		Files:          files,
		WaitingFor:     strategy,
		Labels: map[string]string{
			labelFramework: frameworkLabelValue,
			labelBlock:     opts.Network.Block(),
			labelComponent: def.Name,
		},
	}
	if len(bindings) > 0 {
		chainHostConfig(req, func(hc *container.HostConfig) {
			hc.PortBindings = bindings
		})
	}
	if len(def.Cmd) > 0 {
		req.Cmd = def.Cmd
	}
	if replicas > 1 {
		req.Labels[labelComponent] = fmt.Sprintf("%s#%d", def.Name, opts.Ordinal+1)
	}

	applyLimits(req, def.Limits)
	return req, nil
}

// resolveFiles collects the definition's static file mounts plus the assembled config.
func resolveFiles(def *components.Definition, opts Options) ([]testcontainers.ContainerFile, error) {
	var files []testcontainers.ContainerFile

	appendMount := func(m components.FileMount) error {
		if m.HostPath == "" || m.ContainerPath == "" {
			return fmt.Errorf("runtime: %s has a file mount missing a path", def)
		}
		mode := m.Mode
		if mode == 0 {
			// A copied file is owned by root while the process may run as another
			// user. Too-restrictive here means the component silently cannot read
			// it — or, for a file it must update in place, silently cannot write.
			mode = 0o644
		}
		files = append(files, testcontainers.ContainerFile{
			HostFilePath:      absPath(opts.RepoRoot, m.HostPath),
			ContainerFilePath: m.ContainerPath,
			FileMode:          mode,
		})
		return nil
	}

	for _, m := range def.Files {
		if err := appendMount(m); err != nil {
			return nil, err
		}
	}
	for _, m := range opts.Files {
		if err := appendMount(m); err != nil {
			return nil, err
		}
	}

	if opts.ConfigContent != nil {
		if def.Config == nil {
			return nil, fmt.Errorf("runtime: %s was given config content but declares no config injection", def)
		}
		files = append(files, testcontainers.ContainerFile{
			Reader:            bytesReader(opts.ConfigContent),
			ContainerFilePath: def.Config.ContainerPath,
			FileMode:          0o644,
		})
	}

	return files, nil
}

// buildInstance reads back the ephemeral host ports docker assigned. These are not
// knowable before start, which is the whole reason addressing goes through Instance
// rather than through constants.
func buildInstance(
	ctx context.Context, def *components.Definition, c testcontainers.Container, ordinal, replicas int,
) (*components.Instance, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("runtime: resolving host for %s: %w", def, err)
	}

	mapped := make(map[int]int, len(def.Endpoints))
	for _, e := range def.Endpoints {
		p, err := c.MappedPort(ctx, strconv.Itoa(e.Port)+"/tcp")
		if err != nil {
			// Not fatal: an endpoint may legitimately be unpublished in some
			// configurations, and Instance.URL reports the specific missing endpoint
			// with a better message than a boot failure could.
			continue
		}
		mapped[e.Port] = int(p.Num())
	}

	return components.NewInstance(def, ordinal, replicas, host, mapped)
}

// startWithPortRetry starts the container, re-picking declared host ports on a bind conflict.
//
// Only StableHostPorts components can hit this. freeHostPort closes its probe listener before
// docker binds the port, so something else can take it in between — and with several shared
// services claiming a port each, plus containers a previous run has not finished releasing,
// "rare" turned out to mean "several times an hour".
//
// Retried rather than merely reported because the failure is transient BY CONSTRUCTION: a new
// draw is overwhelmingly likely to succeed, and surfacing it as a boot failure sends the
// reader to look for a broken component when the port was simply taken. A conflict that
// survives every attempt is still a hard failure — that one is not transient.
func startWithPortRetry(
	ctx context.Context, def *components.Definition, opts Options,
	alias, arch string, replicas int, req *testcontainers.ContainerRequest,
) (testcontainers.Container, error) {
	const attempts = 4

	var started testcontainers.Container
	var err error
	for attempt := range attempts {
		started, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: *req,
			Started:          true,
		})
		if err == nil || !opts.StableHostPorts || !isPortConflict(err) || attempt == attempts-1 {
			return started, err
		}
		// The half-started container holds nothing useful and would leak; drop it before
		// drawing fresh ports.
		if started != nil {
			_ = started.Terminate(ctx)
			started = nil
		}
		req, err = buildRequest(def, opts, alias, arch, replicas)
		if err != nil {
			return nil, err
		}
	}
	return started, err
}

// isPortConflict recognises the daemon's message for "that host port is taken".
//
// Matched on the message because the docker API returns it as an opaque 500 with no code a
// caller can switch on. Kept narrow: a broader match would retry genuine start failures and
// turn a clear error into four slow ones.
func isPortConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "port is already allocated") ||
		strings.Contains(msg, "failed programming external connectivity")
}

// freeHostPort asks the OS for a port nobody is listening on.
//
// Binding port 0 and reading back what the kernel chose is the only way to pick one without
// guessing; there is an unavoidable gap between closing this listener and docker binding the
// port. That race is accepted rather than papered over: if something takes the port in between,
// the container fails to START, loudly, naming the port — which is a far better failure than
// the silent address drift this exists to prevent.
//
// On a docker host that is a VM (colima, docker-machine) the port must be free INSIDE the VM,
// which this cannot observe. The same loud start failure covers it.
func freeHostPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Stop terminates the container.
//
// Idempotent, because teardown runs on both the success and failure paths and a second
// stop must not mask the original error.
func (c *Container) Stop(ctx context.Context) error {
	if c == nil || c.container == nil {
		return nil
	}
	inner := c.container
	if err := inner.Terminate(ctx); err != nil {
		return fmt.Errorf("runtime: terminating %s: %w", c.def, err)
	}
	c.container = nil
	return nil
}

// Exec runs a command inside the container and returns its combined output.
//
// Used for the narrow set of operations that genuinely have no other interface — most
// notably applying schema to an engine that cannot bootstrap at first boot. It is not a
// general escape hatch: a product operation belongs in a step, performed as an actor
// over the product's own API, so that it is visible to coverage and its resources are
// sweepable by cleanup.
func (c *Container) Exec(ctx context.Context, cmd []string) (string, error) {
	if c == nil || c.container == nil {
		return "", fmt.Errorf("runtime: no container to exec in")
	}
	// Multiplexed() is required, not cosmetic. Docker frames exec output with an 8-byte
	// header per chunk to separate stdout from stderr; without demultiplexing those bytes
	// land in the returned string, so an exact comparison against a query result or a
	// resolved address fails on invisible framing characters.
	code, reader, err := c.container.Exec(ctx, cmd, tcexec.Multiplexed())
	out := ""
	var readErr error
	if reader != nil {
		out, readErr = readAllString(reader)
	}
	if err != nil {
		if readErr != nil {
			return out, errors.Join(fmt.Errorf("runtime: exec in %s: %w", c.def, err),
				fmt.Errorf("runtime: reading exec output: %w", readErr))
		}
		return out, fmt.Errorf("runtime: exec in %s: %w", c.def, err)
	}
	if readErr != nil {
		return out, fmt.Errorf("runtime: reading exec output: %w", readErr)
	}
	if code != 0 {
		return out, fmt.Errorf("runtime: exec in %s exited %d", c.def, code)
	}
	return out, nil
}

// Logs returns the container's log output, for diagnosing a boot or health failure.
func (c *Container) Logs(ctx context.Context) (string, error) {
	if c == nil || c.container == nil {
		return "", fmt.Errorf("runtime: no container to read logs from")
	}
	rc, err := c.container.Logs(ctx)
	if err != nil {
		return "", fmt.Errorf("runtime: reading logs for %s: %w", c.def, err)
	}
	defer func() { _ = rc.Close() }()
	return readAllString(rc)
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func readAllString(r io.Reader) (string, error) {
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// absPath resolves a repo-relative path against the repository root. An already
// absolute path is returned untouched, so a caller that has resolved paths itself is
// not second-guessed.
func absPath(repoRoot, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	if repoRoot == "" {
		return p
	}
	return filepath.Join(repoRoot, p)
}

// applyLimits caps the container's host resources.
//
// With parallel blocks these are load-bearing rather than advisory: a few unbounded
// topologies can starve the runner, and the symptom is timeouts in unrelated tests
// rather than anything pointing at the real cause.
func applyLimits(req *testcontainers.ContainerRequest, limits components.ResourceLimits) {
	if limits.CPUs <= 0 && limits.MemoryMB <= 0 {
		return
	}
	chainHostConfig(req, func(hc *container.HostConfig) {
		if limits.MemoryMB > 0 {
			hc.Memory = limits.MemoryMB * 1024 * 1024
		}
		if limits.CPUs > 0 {
			// Docker expresses a fractional CPU as a quota over a fixed period.
			const period = 100_000
			hc.CPUPeriod = period
			hc.CPUQuota = int64(limits.CPUs * period)
		}
	})
}

// chainHostConfig adds one more mutation to the request's host config instead of REPLACING
// whatever was there.
//
// testcontainers exposes a single HostConfigModifier field, so a plain assignment silently
// discards any earlier one. That is not hypothetical: this function exists because
// applyLimits overwrote the shared-component port bindings set moments earlier, and the only
// symptom was a container coming up on a docker-chosen port instead of the declared one —
// nothing errored, nothing logged, and the modifier that lost simply never ran.
//
// Anything setting host config must go through here, so the order of the callers stops
// mattering.
func chainHostConfig(req *testcontainers.ContainerRequest, add func(*container.HostConfig)) {
	prev := req.HostConfigModifier
	req.HostConfigModifier = func(hc *container.HostConfig) {
		if prev != nil {
			prev(hc)
		}
		add(hc)
	}
}

// Shared components exist ONCE per process and are multi-homed onto each block's private
// network, rather than started per block.
//
// Shared components are started once and attached to each block network as needed.
var (
	sharedMu   sync.Mutex
	sharedHome *Network
	sharedRun  = map[string]*Container{}
)

// homeNetwork returns the process-lifetime network shared components are started on.
//
// It exists because a container must be created on SOME network, and a shared component has
// no block to belong to. Never removed explicitly — Ryuk reaps it when the process exits,
// which is the same lifetime the components on it have.
func homeNetwork(ctx context.Context) (*Network, error) {
	if sharedHome != nil {
		return sharedHome, nil
	}
	nw, err := NewNetwork(ctx, "shared")
	if err != nil {
		return nil, fmt.Errorf("runtime: creating the shared home network: %w", err)
	}
	sharedHome = nw
	return sharedHome, nil
}

// LaunchShared starts a shared component once and returns the same container thereafter.
//
// Keyed by component name. A second block asking for the same component gets the running
// one, which is the entire point.
//
// The whole call is under one lock, deliberately: the container start is slow, but two
// blocks booting concurrently must not BOTH start one. Holding the lock across the start is
// what makes "once" true rather than merely likely.
func LaunchShared(
	ctx context.Context, def *components.Definition, opts Options,
) (*Container, error) {
	if def == nil {
		return nil, fmt.Errorf("runtime: definition is required")
	}

	sharedMu.Lock()
	defer sharedMu.Unlock()

	if c, ok := sharedRun[def.Name]; ok {
		return c, nil
	}

	home, err := homeNetwork(ctx)
	if err != nil {
		return nil, err
	}

	opts.Network = home
	// A shared container is re-homed onto every block's network as blocks come and go, and
	// docker reallocates an EPHEMERAL published port on each of those attaches and detaches.
	// See Options.StableHostPorts: without this, an address published to one block is
	// invalidated by the next block booting.
	opts.StableHostPorts = true
	c, err := Launch(ctx, def, opts)
	if err != nil {
		return c, err
	}

	sharedRun[def.Name] = c
	return c, nil
}

// AttachTo connects a shared container to a block's private network under its alias.
//
// The alias MUST be re-declared here. A container connected to a network without one is
// reachable only by IP, so the block's components would resolve nothing and the failure
// would surface far away as a connection error naming no host.
func (c *Container) AttachTo(ctx context.Context, nw *Network, alias string) error {
	if c == nil || c.container == nil {
		return fmt.Errorf("runtime: no container to attach")
	}
	if nw == nil || nw.inner == nil {
		return fmt.Errorf("runtime: no network to attach %s to", c.def)
	}

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return fmt.Errorf("runtime: docker provider for attaching %s: %w", c.def, err)
	}
	defer func() { _ = provider.Close() }()

	_, err = provider.Client().NetworkConnect(ctx, nw.inner.ID, client.NetworkConnectOptions{
		Container:      c.container.GetContainerID(),
		EndpointConfig: &network.EndpointSettings{Aliases: []string{alias}},
	})
	if err != nil {
		return fmt.Errorf("runtime: attaching %s to network %q: %w", c.def, nw.Name(), err)
	}
	return nil
}

// DetachFrom disconnects a shared container from a block's network.
//
// This is NOT tidiness. Docker refuses to remove a network that still has a container
// connected, so without this every block's network removal fails and networks accumulate for
// the rest of the run. It must therefore happen before Network.Remove, which is why the
// engine registers it as a teardown step rather than leaving it to the caller.
func (c *Container) DetachFrom(ctx context.Context, nw *Network) error {
	if c == nil || c.container == nil || nw == nil || nw.inner == nil {
		return nil
	}

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return fmt.Errorf("runtime: docker provider for detaching %s: %w", c.def, err)
	}
	defer func() { _ = provider.Close() }()

	if _, err := provider.Client().NetworkDisconnect(ctx, nw.inner.ID, client.NetworkDisconnectOptions{
		Container: c.container.GetContainerID(),
	}); err != nil {
		return fmt.Errorf("runtime: detaching %s from network %q: %w", c.def, nw.Name(), err)
	}
	return nil
}
