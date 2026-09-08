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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/actor"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/coverage"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// Shared-scope keys published by the runtime.
const (
	KeyBlockName     = "blockName"
	KeyAdminUser     = "adminUsername"
	KeyAdminPass     = "adminPassword"
	KeyConsumerUser  = "consumerUsername"
	KeyConsumerPass  = "consumerPassword"
	KeyDeveloperUser = "developerUsername"
	KeyDeveloperPass = "developerPassword"
	KeyInstanceSet   = "instances"
)

// Topology is one block's running infrastructure.
type Topology struct {
	// Block is the resolved declaration this came from.
	Block *topology.ResolvedBlock

	// Instances are the running components, addressable by name.
	Instances *components.Set

	// Admin are the fixed administrative credentials supplied by the test overlays.
	Admin actor.Credentials

	// Shared is the block's shared scope, already populated.
	Shared *tcontext.Shared

	// Storage is what the database layer provisioned.
	Storage *Provisioned

	network *Network
	stops   []func(context.Context) error

	// provisioned holds values produced by running components.
	provisionMu sync.Mutex
	provisioned map[string]map[string]string

	// stacks contains compose-backed components addressable by service name.
	stacks map[string]*ComposeStack
}

// ServiceControl resolves a compose service or component to its controlling stack.
func (t *Topology) ServiceControl(name string) (*ComposeStack, string, error) {
	if t == nil {
		return nil, "", fmt.Errorf("runtime: topology is required")
	}
	var found *ComposeStack
	var service string
	matches := 0

	for component, stack := range t.stacks {
		if component == name {
			found, service, matches = stack, stack.PrimaryService(), matches+1
			continue
		}
		for _, svc := range stack.Services() {
			if svc == name {
				found, service, matches = stack, svc, matches+1
			}
		}
	}

	switch {
	case matches == 0:
		return nil, "", fmt.Errorf(
			"no service or compose component named %q in block %q; this block runs %v",
			name, blockName(t.Block), t.serviceNames())
	case matches > 1:
		return nil, "", fmt.Errorf("service %q is ambiguous in block %q: several components expose it",
			name, blockName(t.Block))
	}
	return found, service, nil
}

func (t *Topology) serviceNames() []string {
	var out []string
	for component, stack := range t.stacks {
		out = append(out, component)
		out = append(out, stack.Services()...)
	}
	sort.Strings(out)
	return out
}

// Component returns a running instance by component name.
func (t *Topology) Component(name string) (*components.Instance, error) {
	if t == nil {
		return nil, fmt.Errorf("runtime: topology is required")
	}
	if t.Instances == nil {
		return nil, fmt.Errorf("runtime: block %q has no instances", blockName(t.Block))
	}
	return t.Instances.Get(name)
}

// ComponentVersion returns the effective version resolved for a component in this block.
func (t *Topology) ComponentVersion(name string) (string, error) {
	if t == nil {
		return "", fmt.Errorf("runtime: topology is required")
	}
	if t.Block == nil {
		return "", fmt.Errorf("runtime: block is required")
	}
	for _, component := range t.Block.Components {
		if component.Def != nil && component.Def.Name == name {
			if strings.TrimSpace(component.Version) == "" {
				return "", fmt.Errorf("runtime: component %q has no resolved version", name)
			}
			return component.Version, nil
		}
	}
	return "", fmt.Errorf("runtime: component %q is not declared in block %q", name, t.Block.Name)
}

// URL returns a named endpoint address on a component.
func (t *Topology) URL(component, endpoint string) (string, error) {
	inst, err := t.Component(component)
	if err != nil {
		return "", err
	}
	return inst.URL(endpoint)
}

// BootBlock starts the block's storage and components in dependency order.
func BootBlock(
	ctx context.Context, block *topology.ResolvedBlock, repoRoot string,
) (*Topology, error) {
	if block == nil {
		return nil, fmt.Errorf("runtime: block is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("runtime: context is required")
	}
	t := &Topology{Block: block, Instances: components.NewSet()}

	nw, err := NewNetwork(ctx, block.Name)
	if err != nil {
		return nil, err
	}
	t.network = nw

	fail := func(err error) (*Topology, error) {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		_ = t.Teardown(cleanupCtx)
		return nil, err
	}

	t.Admin = actor.Administrator()

	// Provision storage before starting components.
	storage, err := Provision(ctx, DatabaseOptions{
		Network:  nw,
		RepoRoot: repoRoot,
		Requests: storageRequests(block),
	})
	if err != nil {
		return fail(fmt.Errorf("runtime: block %q: %w", block.Name, err))
	}
	t.Storage = storage
	t.stops = append(t.stops, storage.Stop)

	ordered, err := bootOrder(block)
	if err != nil {
		return fail(fmt.Errorf("runtime: block %q: %w", block.Name, err))
	}

	for _, rc := range ordered {
		if err := t.startComponent(ctx, rc, repoRoot); err != nil {
			return fail(fmt.Errorf("runtime: block %q: %w", block.Name, err))
		}
	}

	t.Shared = tcontext.NewShared(block.Name)
	t.publishAccessors()

	return t, nil
}

func storageRequests(block *topology.ResolvedBlock) []Request {
	requests := make([]Request, 0, len(block.Components))
	for _, component := range block.Components {
		if component.Def.DB == nil {
			continue
		}
		requests = append(requests, Request{
			Def: component.Def, Type: component.DB, Image: component.Image, Replicas: component.Replicas,
		})
	}
	return requests
}

// startComponent starts one component and waits for application readiness.
func (t *Topology) startComponent(
	ctx context.Context, rc topology.ResolvedComponent, repoRoot string,
) error {
	def := rc.Def

	env := map[string]string{}
	for k, v := range t.Storage.Env[KeyFor(def.Name, 0)] {
		env[k] = v
	}
	// Values produced by dependencies.
	for _, dep := range rc.AllDependencies() {
		for k, v := range t.provisionedBy(dep) {
			env[k] = v
		}
	}

	var configContent []byte
	if def.Config != nil {
		// Inject the block identity into overlays at the runtime boundary.
		vars := components.Vars{components.VarBlock: t.Block.PartitionKey()}
		content, err := components.Assemble(def.Config, repoRoot, rc.Overlay, vars)
		if err != nil {
			return fmt.Errorf("assembling config for %s: %w", def.Name, err)
		}
		configContent = content
	}

	opts := Options{
		Network:  t.network,
		RepoRoot: repoRoot,
		Env:      env,
		Replicas: rc.Replicas,
	}

	if def.IsCompose() {
		// Compose owns the generated configuration mounts.
		generated := map[string][]byte{
			"api-platform.env": components.EnvFileContent(env),
		}
		if configContent != nil {
			generated["config.toml"] = configContent
		}
		spec := def.Compose.WithGenerated(generated)

		stack, err := LaunchCompose(ctx, def, spec, opts)
		if stack != nil {
			t.stops = append(t.stops, stack.Stop)
		}
		if err != nil {
			if stack != nil {
				return fmt.Errorf("starting %s: %w\nservice logs:\n%s",
					def.Name, err, stack.Logs(ctx))
			}
			return fmt.Errorf("starting %s: %w", def.Name, err)
		}
		if err := t.Instances.Add(stack.Instance); err != nil {
			return err
		}
		// Keep compose stacks available for service lifecycle operations.
		if t.stacks == nil {
			t.stacks = map[string]*ComposeStack{}
		}
		t.stacks[def.Name] = stack
		return t.runProvisioner(ctx, def, stack.Instance)
	}

	opts.ConfigContent = configContent

	if def.Shared {
		return t.startShared(ctx, def, opts)
	}

	for ordinal := range rc.Replicas {
		instOpts := opts
		instOpts.Ordinal = ordinal
		if dsnEnv, ok := t.Storage.Env[KeyFor(def.Name, ordinal)]; ok {
			merged := map[string]string{}
			for k, v := range env {
				merged[k] = v
			}
			for k, v := range dsnEnv {
				merged[k] = v
			}
			instOpts.Env = merged
		}

		container, err := Launch(ctx, def, instOpts)
		if container != nil {
			t.stops = append(t.stops, container.Stop)
		}
		if err != nil {
			if container != nil {
				if logs, logErr := container.Logs(ctx); logErr == nil {
					return fmt.Errorf("starting %s: %w\nlogs:\n%s", def.Name, err, logs)
				}
			}
			return fmt.Errorf("starting %s: %w", def.Name, err)
		}

		if err := AwaitHealthy(ctx, container.Instance, nil); err != nil {
			logs, _ := container.Logs(ctx)
			return fmt.Errorf("%w\nlogs:\n%s", err, logs)
		}

		if err := t.Instances.Add(container.Instance); err != nil {
			return err
		}

		if err := t.runProvisioner(ctx, def, container.Instance); err != nil {
			logs, _ := container.Logs(ctx)
			return fmt.Errorf("%w\nlogs:\n%s", err, logs)
		}
	}

	return nil
}

// runProvisioner records values produced for dependent components.
func (t *Topology) runProvisioner(
	ctx context.Context, def *components.Definition, inst *components.Instance,
) error {
	if def.Provisions == nil {
		return nil
	}
	values, err := def.Provisions(ctx, inst)
	if err != nil {
		return fmt.Errorf("provisioning from %s: %w", def.Name, err)
	}
	t.provisionMu.Lock()
	defer t.provisionMu.Unlock()
	if t.provisioned == nil {
		t.provisioned = map[string]map[string]string{}
	}
	t.provisioned[def.Name] = values
	return nil
}

// provisionedBy returns what the named component provisioned, or nil.
func (t *Topology) provisionedBy(name string) map[string]string {
	t.provisionMu.Lock()
	defer t.provisionMu.Unlock()
	return t.provisioned[name]
}

// startShared attaches a shared component to the block's network.
func (t *Topology) startShared(
	ctx context.Context, def *components.Definition, opts Options,
) error {
	container, err := LaunchShared(ctx, def, opts)
	if err != nil {
		return fmt.Errorf("starting shared %s: %w", def.Name, err)
	}

	alias, err := components.AliasFor(def, 0, 1)
	if err != nil {
		return err
	}
	if err := container.AttachTo(ctx, t.network, alias); err != nil {
		return err
	}

	// Detach the shared container before removing the block network.
	t.stops = append(t.stops, func(ctx context.Context) error {
		return container.DetachFrom(ctx, t.network)
	})

	if err := AwaitHealthy(ctx, container.Instance, nil); err != nil {
		return err
	}
	if err := t.Instances.Add(container.Instance); err != nil {
		return err
	}
	return t.runProvisioner(ctx, def, container.Instance)
}

// publishAccessors puts everything a step needs into shared scope.
func (t *Topology) publishAccessors() {
	t.Shared.Set(KeyBlockName, t.Block.Name)
	t.Shared.Set(KeyAdminUser, t.Admin.Username)
	t.Shared.Set(KeyAdminPass, t.Admin.Password)
	consumer := actor.Consumer()
	t.Shared.Set(KeyConsumerUser, consumer.Username)
	t.Shared.Set(KeyConsumerPass, consumer.Password)
	developer := actor.Developer()
	t.Shared.Set(KeyDeveloperUser, developer.Username)
	t.Shared.Set(KeyDeveloperPass, developer.Password)
	t.Shared.Set(KeyInstanceSet, t.Instances)

	// Publish ready-to-use endpoint URLs for every component.
	for _, name := range t.Instances.Names() {
		for _, inst := range t.Instances.All(name) {
			for _, e := range inst.Definition().Endpoints {
				url, err := inst.URL(e.Name)
				if err != nil {
					continue // unpublished endpoint; Instance.URL reports it better on demand
				}
				t.Shared.Set(accessorKey(inst.Name(), e.Name), url)
			}
		}
	}
}

// accessorKey is the shared-scope key for one component endpoint.
func accessorKey(component, endpoint string) string {
	return "url:" + component + ":" + endpoint
}

// AccessorKey exposes the key format so a step helper can read an accessor.
func AccessorKey(component, endpoint string) string { return accessorKey(component, endpoint) }

// CollectCoverage stops coverage-enabled services and copies their artifacts to the sink.
func (t *Topology) CollectCoverage(ctx context.Context, sink *coverage.Sink) error {
	if t == nil {
		return fmt.Errorf("runtime: topology is required")
	}
	if sink == nil {
		return fmt.Errorf("runtime: coverage sink is required")
	}
	if ctx == nil {
		return fmt.Errorf("runtime: context is required")
	}
	var errs []error
	for component, stack := range t.stacks {
		for _, coverageService := range stack.CoverageServices() {
			svc := coverageService.Name
			coverageType := strings.Join(coverageService.Types, ",")
			// The container ID must be resolved while the service still runs — the
			// lookup behind it only sees running containers.
			id, err := stack.ServiceContainerID(ctx, svc)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s/%s (%s): %w", component, svc, coverageType, err))
				continue
			}
			if err := stack.StopService(ctx, svc); err != nil {
				errs = append(errs, fmt.Errorf("%s/%s (%s): %w", component, svc, coverageType, err))
				continue
			}
			dst, err := sink.Dir(t.Block.Name, svc)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s/%s (%s): %w", component, svc, coverageType, err))
				continue
			}
			if err := coverage.CopyDir(ctx, id, coverage.GuestDir, dst); err != nil {
				errs = append(errs, fmt.Errorf("%s/%s (%s): %w", component, svc, coverageType, err))
			}
		}
	}
	return errors.Join(errs...)
}

// Teardown stops all block resources in reverse order.
func (t *Topology) Teardown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	var errs []error
	remaining := make([]func(context.Context) error, 0)
	for i := len(t.stops) - 1; i >= 0; i-- {
		if err := t.stops[i](ctx); err != nil {
			errs = append(errs, err)
			remaining = append(remaining, t.stops[i])
		}
	}
	for i, j := 0, len(remaining)-1; i < j; i, j = i+1, j-1 {
		remaining[i], remaining[j] = remaining[j], remaining[i]
	}
	t.stops = remaining

	if t.network != nil {
		if err := t.network.Remove(ctx); err != nil {
			errs = append(errs, err)
		} else {
			t.network = nil
		}
	}
	return errors.Join(errs...)
}

func blockName(block *topology.ResolvedBlock) string {
	if block == nil {
		return "<unknown>"
	}
	return block.Name
}

// bootOrder sorts components so dependencies start first.
func bootOrder(block *topology.ResolvedBlock) ([]topology.ResolvedComponent, error) {
	byName := make(map[string]topology.ResolvedComponent, len(block.Components))
	for _, c := range block.Components {
		byName[c.Def.Name] = c
	}

	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic, so a boot problem reproduces

	const (
		unvisited = 0
		active    = 1
		done      = 2
	)
	state := map[string]int{}
	var out []topology.ResolvedComponent

	var visit func(string, []string) error
	visit = func(name string, path []string) error {
		switch state[name] {
		case active:
			return fmt.Errorf("dependency cycle at %q (path: %v)", name, path)
		case done:
			return nil
		}
		state[name] = active

		c, present := byName[name]
		if !present {
			// Validated at load; treated as satisfied here rather than failing the boot.
			state[name] = done
			return nil
		}
		deps := c.AllDependencies()
		sort.Strings(deps)
		for _, dep := range deps {
			if err := visit(dep, append(path, name)); err != nil {
				return err
			}
		}

		state[name] = done
		out = append(out, c)
		return nil
	}

	for _, n := range names {
		if err := visit(n, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}
