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

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const registryKey = "__resourceCleanup"

// Scope determines how long a registered resource is retained.
type Scope uint8

const (
	// ScenarioScope retains a resource until the current scenario is swept.
	ScenarioScope Scope = iota
	// RunnerScope retains a resource until the owning runner is swept.
	RunnerScope
)

// Kind identifies a resource type and its cleanup order.
type Kind struct {
	// Name identifies the kind in diagnostics.
	Name string
	// Order is ascending: lower is deleted FIRST. A resource that references another must
	// have a lower order than the thing it references.
	Order int
}

// Resource kinds ordered from earliest to latest cleanup.
var (
	KindSubscription = Kind{Name: "subscription", Order: 10}
	KindApplication  = Kind{Name: "application", Order: 20}
	KindAPIKey       = Kind{Name: "api-key", Order: 30}
	KindAPIProduct   = Kind{Name: "api-product", Order: 40}
	KindAPI          = Kind{Name: "api", Order: 50}
	KindMCPServer    = Kind{Name: "mcp-server", Order: 55}
	KindPolicy       = Kind{Name: "policy", Order: 60}
	KindSharedScope  = Kind{Name: "shared-scope", Order: 70}
	KindCertificate  = Kind{Name: "certificate", Order: 80}
	// LLMProxy and LLMProvider both delete before Secret: either can hold a credential as a
	// {{ secret "handle" }} placeholder, and platform-api refuses to delete a secret still
	// referenced by one (409 SECRET_IN_USE). LLMProxy also deletes before LLMProvider, since
	// a proxy references its own provider by id.
	KindLLMProxy    = Kind{Name: "llm-proxy", Order: 82}
	KindLLMProvider = Kind{Name: "llm-provider", Order: 84}
	KindSecret      = Kind{Name: "secret", Order: 85}
	// A provider references its template version by id, so the template version deletes
	// after every provider that could reference it.
	KindLLMProviderTemplate = Kind{Name: "llm-provider-template", Order: 86}
	// A provider or proxy deployment references its gateway by id, so gateway deletes after
	// both.
	KindGateway     = Kind{Name: "gateway", Order: 88}
	KindEnvironment = Kind{Name: "environment", Order: 95}
	KindProject     = Kind{Name: "project", Order: 100}
)

// Deleter removes one resource as the actor that created it.
type Deleter func(ctx context.Context, res Resource) error

// Resource describes a created resource and its creator.
type Resource struct {
	Kind Kind
	ID   string

	// Actor is the reference of the principal that created this. Required.
	Actor string

	// Description is optional context for a leak warning — a name, a tenant.
	Description string

	cleanupFailures int
}

func (r Resource) String() string {
	if r.Description != "" {
		return fmt.Sprintf("%s %q (%s, created by %s)", r.Kind.Name, r.ID, r.Description, r.Actor)
	}
	return fmt.Sprintf("%s %q (created by %s)", r.Kind.Name, r.ID, r.Actor)
}

// Registry tracks resources created by one test runner.
type Registry struct {
	mu                sync.RWMutex
	scenarioResources []Resource
	runnerResources   []Resource
	deleters          map[string]Deleter
	log               *slog.Logger
	maxAttempts       int
}

// NewRegistry returns an empty registry.
// maxAttempts is optional; zero disables retries.
func NewRegistry(log *slog.Logger, maxAttempts ...int) *Registry {
	if log == nil {
		log = slog.Default()
	}
	configured := 0
	if len(maxAttempts) > 0 && maxAttempts[0] > 0 {
		configured = maxAttempts[0]
	}
	return &Registry{deleters: map[string]Deleter{}, log: log, maxAttempts: configured}
}

// RegisterDeleter registers the function used to remove a resource kind.
func (r *Registry) RegisterDeleter(kind Kind, deleter Deleter) error {
	if kind.Name == "" {
		return errors.New("cleanup: cannot register a deleter with no kind")
	}
	if deleter == nil {
		return fmt.Errorf("cleanup: cannot register a nil deleter for kind %q", kind.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleters == nil {
		r.deleters = make(map[string]Deleter)
	}
	r.deleters[kind.Name] = deleter
	if r.log == nil {
		r.log = slog.Default()
	}
	return nil
}

// Register records a resource for cleanup.
func (r *Registry) Register(res Resource) error {
	return r.RegisterScoped(ScenarioScope, res)
}

// RegisterRunner records a resource until the runner completes.
func (r *Registry) RegisterRunner(res Resource) error {
	return r.RegisterScoped(RunnerScope, res)
}

// RegisterScoped records a resource for the selected lifecycle scope.
func (r *Registry) RegisterScoped(scope Scope, res Resource) error {
	if res.ID == "" {
		return fmt.Errorf("cleanup: cannot register a %s with no id", res.Kind.Name)
	}
	if res.Actor == "" {
		return fmt.Errorf("cleanup: cannot register %s %q with no actor — teardown deletes each "+
			"resource as its creator, so an unattributed resource cannot be cleaned up",
			res.Kind.Name, res.ID)
	}
	if res.Kind.Name == "" {
		return fmt.Errorf("cleanup: cannot register resource %q with no kind", res.ID)
	}
	if scope != ScenarioScope && scope != RunnerScope {
		return fmt.Errorf("cleanup: invalid resource scope %d", scope)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	resources := r.resourcesForScopeLocked(scope)
	for _, registered := range *resources {
		if registered.Kind.Name == res.Kind.Name && registered.ID == res.ID {
			r.logger().Warn("duplicate cleanup registration ignored",
				"kind", res.Kind.Name, "id", res.ID,
				"existing_failures", registered.cleanupFailures)
			return nil
		}
	}
	otherScope := r.resourcesForScopeLocked(otherScope(scope))
	for i, registered := range *otherScope {
		if registered.Kind.Name != res.Kind.Name || registered.ID != res.ID {
			continue
		}
		if scope == RunnerScope {
			// A setup fixture may be discovered after an earlier scenario registered it.
			// Promote it so scenario cleanup cannot remove a resource the runner still owns.
			*otherScope = append((*otherScope)[:i], (*otherScope)[i+1:]...)
			*resources = append(*resources, registered)
			r.logger().Info("cleanup resource scope promoted",
				"kind", res.Kind.Name, "id", res.ID, "from", "scenario", "to", "runner")
		}
		return nil
	}
	*resources = append(*resources, res)
	return nil
}

func otherScope(scope Scope) Scope {
	if scope == RunnerScope {
		return ScenarioScope
	}
	return RunnerScope
}

// Deregister removes a resource that was deleted by the test.
func (r *Registry) Deregister(kind Kind, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenarioResources = deregister(r.scenarioResources, kind, id)
	r.runnerResources = deregister(r.runnerResources, kind, id)
}

func deregister(resources []Resource, kind Kind, id string) []Resource {
	kept := resources[:0]
	for _, res := range resources {
		if res.Kind.Name != kind.Name || res.ID != id {
			kept = append(kept, res)
		}
	}
	return kept
}

// Pending returns registered resources in cleanup order.
func (r *Registry) Pending() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resources := make([]Resource, 0, len(r.scenarioResources)+len(r.runnerResources))
	resources = append(resources, r.scenarioResources...)
	resources = append(resources, r.runnerResources...)
	return ordered(resources)
}

func ordered(resources []Resource) []Resource {
	type entry struct {
		resource Resource
		index    int
	}
	entries := make([]entry, len(resources))
	for i, resource := range resources {
		entries[i] = entry{resource: resource, index: i}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.resource.Kind.Order != right.resource.Kind.Order {
			return left.resource.Kind.Order < right.resource.Kind.Order
		}
		if left.resource.Kind.Name != right.resource.Kind.Name {
			return left.resource.Kind.Name < right.resource.Kind.Name
		}
		return left.index > right.index
	})

	out := make([]Resource, len(entries))
	for i, entry := range entries {
		out[i] = entry.resource
	}
	return out
}

// Count returns the number of registered resources.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.scenarioResources) + len(r.runnerResources)
}

// Sweep deletes scenario-scoped resources and clears their registrations.
func (r *Registry) Sweep(ctx context.Context) error {
	return r.sweep(ctx, ScenarioScope)
}

// SweepRunner deletes runner-scoped resources and clears their registrations.
func (r *Registry) SweepRunner(ctx context.Context) error {
	return r.sweep(ctx, RunnerScope)
}

// sweep deletes resources in one lifecycle scope. Deletion continues after individual
// failures, which are returned as one combined error.
func (r *Registry) sweep(ctx context.Context, scope Scope) error {
	r.mu.Lock()
	resources := r.resourcesForScopeLocked(scope)
	pending := ordered(*resources)
	if len(pending) == 0 {
		r.mu.Unlock()
		return nil
	}

	deleters := make(map[string]Deleter, len(r.deleters))
	for k, v := range r.deleters {
		deleters[k] = v
	}
	log := r.log
	if log == nil {
		log = slog.Default()
	}
	maxAttempts := r.maxAttempts
	*resources = nil
	r.mu.Unlock()

	var errs []error
	var failed []Resource

	for _, res := range pending {
		deleter, ok := deleters[res.Kind.Name]
		if !ok {
			err := fmt.Errorf("cleanup: no deleter registered for kind %q, so %s leaked",
				res.Kind.Name, res)
			res.cleanupFailures++
			willRetry := maxAttempts > 0 && res.cleanupFailures < maxAttempts
			if willRetry {
				failed = append(failed, res)
			}
			log.Warn("resource cleanup failed: no deleter",
				"kind", res.Kind.Name, "id", res.ID, "resource", res.String(),
				"attempt", res.cleanupFailures, "max_attempts", maxAttempts,
				"will_retry", willRetry)
			if maxAttempts > 0 && !willRetry {
				errs = append(errs, fmt.Errorf("cleanup: retry limit exhausted for %s: %w", res, err))
				continue
			}
			errs = append(errs, err)
			continue
		}

		if err := deleter(ctx, res); err != nil {
			res.cleanupFailures++
			willRetry := maxAttempts > 0 && res.cleanupFailures < maxAttempts
			if willRetry {
				failed = append(failed, res)
			}
			log.Warn("resource cleanup failed: delete error",
				"kind", res.Kind.Name, "id", res.ID, "resource", res.String(),
				"error", err, "attempt", res.cleanupFailures,
				"max_attempts", maxAttempts, "will_retry", willRetry)
			if maxAttempts > 0 && !willRetry {
				errs = append(errs, fmt.Errorf("cleanup: retry limit exhausted for %s: %w", res, err))
				continue
			}
			errs = append(errs, fmt.Errorf("cleanup: deleting %s: %w", res, err))
			continue
		}
	}

	if len(failed) > 0 {
		r.mu.Lock()
		for _, res := range failed {
			resources := r.resourcesForScopeLocked(scope)
			if hasResource(*resources, res.Kind.Name, res.ID) {
				log.Warn("failed cleanup resource already registered; retry retained",
					"kind", res.Kind.Name, "id", res.ID,
					"attempt", res.cleanupFailures, "max_attempts", maxAttempts)
				continue
			}
			*resources = append(*resources, res)
			log.Warn("failed cleanup resource requeued",
				"kind", res.Kind.Name, "id", res.ID,
				"attempt", res.cleanupFailures, "max_attempts", maxAttempts,
				"next_attempt", res.cleanupFailures+1)
		}
		r.mu.Unlock()
	}

	return errors.Join(errs...)
}

func (r *Registry) resourcesForScopeLocked(scope Scope) *[]Resource {
	if scope == RunnerScope {
		return &r.runnerResources
	}
	return &r.scenarioResources
}

func hasResource(resources []Resource, kind, id string) bool {
	for _, resource := range resources {
		if resource.Kind.Name == kind && resource.ID == id {
			return true
		}
	}
	return false
}

func (r *Registry) logger() *slog.Logger {
	if r.log == nil {
		r.log = slog.Default()
	}
	return r.log
}

// ── Context integration ──────────────────────────────────────────────────────────

// Install attaches a registry to a runner context.
func Install(ctx context.Context, r *Registry) error {
	if r == nil {
		return errors.New("cleanup: cannot install a nil registry")
	}
	return tcontext.Set(ctx, registryKey, r)
}

// Of returns the registry attached to a runner context.
func Of(ctx context.Context) (*Registry, error) {
	v, ok := tcontext.Get(ctx, registryKey)
	if !ok {
		return nil, fmt.Errorf("cleanup: no registry in context — resources registered now would " +
			"never be swept, so this is a wiring error rather than something to ignore")
	}
	r, ok := v.(*Registry)
	if !ok || r == nil {
		return nil, fmt.Errorf("cleanup: context key %q does not contain a valid registry", registryKey)
	}
	return r, nil
}

// Register records a resource using the registry in the context.
func Register(ctx context.Context, res Resource) error {
	r, err := Of(ctx)
	if err != nil {
		return err
	}
	return r.Register(res)
}

// RegisterRunner records a runner-scoped resource using the registry in the context.
func RegisterRunner(ctx context.Context, res Resource) error {
	r, err := Of(ctx)
	if err != nil {
		return err
	}
	return r.RegisterRunner(res)
}

// Sweep runs the cleanup sweep for the registry in the context.
func Sweep(ctx context.Context) error {
	r, err := Of(ctx)
	if err != nil {
		return err
	}
	return r.Sweep(ctx)
}

// SweepRunner runs the runner-scoped cleanup sweep for the registry in the context.
func SweepRunner(ctx context.Context) error {
	r, err := Of(ctx)
	if err != nil {
		return err
	}
	return r.SweepRunner(ctx)
}
