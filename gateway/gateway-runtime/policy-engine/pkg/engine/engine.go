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

// Package engine provides a public facade for the policy engine,
// enabling non-Envoy runtimes (e.g., the event gateway) to create
// isolated policy engine instances with explicit lifecycle management.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/common/version"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/executor"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	internalcel "github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pkg/cel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyengine "github.com/wso2/api-platform/sdk/core/policyengine"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Engine is an isolated policy engine instance with its own registry,
// executor, and chain storage. Multiple Engine instances can coexist
// without interfering with each other or the global singleton.
type Engine struct {
	mu       sync.RWMutex
	registry *registry.PolicyRegistry
	executor *executor.ChainExecutor
	chains   map[string]*registry.PolicyChain
	tracer   trace.Tracer
}

// New creates a new Engine instance with its own policy registry and executor.
// The configPath points to a TOML configuration file containing policy_configurations
// entries used to resolve ${config...} references in system parameters.
// If configPath is empty, config resolution is skipped.
func New(config map[string]interface{}) (*Engine, error) {
	// Ensure policy engine metrics are initialized (safe for repeated calls).
	metrics.Init()

	reg := &registry.PolicyRegistry{
		Policies: make(map[string]*registry.PolicyEntry),
	}

	if config != nil {
		if err := reg.SetConfig(config); err != nil {
			return nil, fmt.Errorf("failed to set config: %w", err)
		}
	} else {
		// Create resolver with empty config so it is not nil
		if err := reg.SetConfig(map[string]interface{}{}); err != nil {
			return nil, fmt.Errorf("failed to set empty config: %w", err)
		}
	}

	celEvaluator, err := internalcel.NewCELEvaluator()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL evaluator: %w", err)
	}
	tracer := noop.NewTracerProvider().Tracer("event-gateway")

	exec := executor.NewChainExecutor(reg, celEvaluator, tracer)

	return &Engine{
		registry: reg,
		executor: exec,
		chains:   make(map[string]*registry.PolicyChain),
		tracer:   tracer,
	}, nil
}

// RegisterPolicy registers a policy definition and factory in this engine's
// private registry. Must be called before LoadChainsFromFile.
func (e *Engine) RegisterPolicy(def *policy.PolicyDefinition, factory policy.PolicyFactory) error {
	return e.registry.Register(def, factory)
}

// LoadChainsFromFile loads policy chain configurations from a YAML file.
// The file format matches sdk/core/policyengine.PolicyChain.
func (e *Engine) LoadChainsFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read chains file %s: %w", path, err)
	}

	var configs []policyengine.PolicyChain
	if err := yaml.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("failed to parse chains file %s: %w", path, err)
	}

	chains := make(map[string]*registry.PolicyChain)
	for _, config := range configs {
		if config.RouteKey == "" {
			return fmt.Errorf("route_key is required in chain config")
		}
		for i, pc := range config.Policies {
			if pc.Name == "" {
				return fmt.Errorf("policy[%d]: name is required in chain %s", i, config.RouteKey)
			}
			if pc.Version == "" {
				return fmt.Errorf("policy[%d]: version is required in chain %s", i, config.RouteKey)
			}
			if err := e.registry.PolicyExists(pc.Name, version.MajorVersion(pc.Version)); err != nil {
				return fmt.Errorf("policy[%d] in chain %s: %w", i, config.RouteKey, err)
			}
		}

		chain, err := e.buildPolicyChain(config.RouteKey, &config)
		if err != nil {
			return fmt.Errorf("failed to build chain for route %s: %w", config.RouteKey, err)
		}
		chains[config.RouteKey] = chain
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range chains {
		e.chains[k] = v
		slog.Info("Loaded policy chain", "route", k, "policies", len(v.Policies))
	}
	return nil
}

// RegisterChain registers a pre-built policy chain for a given route key.
func (e *Engine) RegisterChain(routeKey string, chain *registry.PolicyChain) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.chains[routeKey] = chain
}

// UnregisterChain removes the policy chain for the given route key.
func (e *Engine) UnregisterChain(routeKey string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.chains, routeKey)
}

// GetChain returns the policy chain for the given route key, or nil if not found.
func (e *Engine) GetChain(routeKey string) *registry.PolicyChain {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.chains[routeKey]
}

// ExecuteRequestHeaderPolicies executes the request header phase of the policy
// chain identified by chainKey.
func (e *Engine) ExecuteRequestHeaderPolicies(
	ctx context.Context,
	chainKey string,
	sharedCtx *policy.SharedContext,
	reqHeaderCtx *policy.RequestHeaderContext,
) (*RequestHeaderResult, error) {
	chain := e.GetChain(chainKey)
	if chain == nil {
		return nil, fmt.Errorf("no policy chain found for key: %s", chainKey)
	}

	result, err := e.executor.ExecuteRequestHeaderPolicies(
		ctx, chain.Policies, reqHeaderCtx, chain.PolicySpecs,
		"", chainKey, chain.HasExecutionConditions,
	)
	if err != nil {
		return nil, err
	}
	return mapRequestHeaderResult(result), nil
}

// ExecuteRequestBodyPolicies executes the request body phase of the policy
// chain identified by chainKey.
func (e *Engine) ExecuteRequestBodyPolicies(
	ctx context.Context,
	chainKey string,
	sharedCtx *policy.SharedContext,
	reqCtx *policy.RequestContext,
) (*RequestBodyResult, error) {
	chain := e.GetChain(chainKey)
	if chain == nil {
		return nil, fmt.Errorf("no policy chain found for key: %s", chainKey)
	}

	result, err := e.executor.ExecuteRequestPolicies(
		ctx, chain.Policies, reqCtx, chain.PolicySpecs,
		"", chainKey, chain.HasExecutionConditions,
	)
	if err != nil {
		return nil, err
	}
	return mapRequestBodyResult(result), nil
}

// ExecuteResponseHeaderPolicies executes the response header phase of the policy
// chain identified by chainKey.
func (e *Engine) ExecuteResponseHeaderPolicies(
	ctx context.Context,
	chainKey string,
	sharedCtx *policy.SharedContext,
	respHeaderCtx *policy.ResponseHeaderContext,
) (*ResponseHeaderResult, error) {
	chain := e.GetChain(chainKey)
	if chain == nil {
		return nil, fmt.Errorf("no policy chain found for key: %s", chainKey)
	}

	result, err := e.executor.ExecuteResponseHeaderPolicies(
		ctx, chain.Policies, respHeaderCtx, chain.PolicySpecs,
		"", chainKey, chain.HasExecutionConditions,
	)
	if err != nil {
		return nil, err
	}
	return mapResponseHeaderResult(result), nil
}

// ExecuteResponseBodyPolicies executes the response body phase of the policy
// chain identified by chainKey.
func (e *Engine) ExecuteResponseBodyPolicies(
	ctx context.Context,
	chainKey string,
	sharedCtx *policy.SharedContext,
	respCtx *policy.ResponseContext,
) (*ResponseBodyResult, error) {
	chain := e.GetChain(chainKey)
	if chain == nil {
		return nil, fmt.Errorf("no policy chain found for key: %s", chainKey)
	}

	result, err := e.executor.ExecuteResponsePolicies(
		ctx, chain.Policies, respCtx, chain.PolicySpecs,
		"", chainKey, chain.HasExecutionConditions,
	)
	if err != nil {
		return nil, err
	}
	return mapResponseBodyResult(result), nil
}

// buildPolicyChain builds a PolicyChain from config, mirroring the internal
// kernel's buildPolicyChain logic.
func (e *Engine) buildPolicyChain(routeKey string, config *policyengine.PolicyChain) (*registry.PolicyChain, error) {
	var policyList []policy.Policy
	var policySpecs []policy.PolicySpec

	requiresRequestBody := false
	requiresResponseBody := false
	hasExecutionConditions := false
	requiresRequestHeader := false
	requiresResponseHeader := false
	supportsRequestStreaming := true
	supportsResponseStreaming := true
	hasRequestBodyPolicy := false
	hasResponseBodyPolicy := false

	for _, pc := range config.Policies {
		metadata := policy.PolicyMetadata{
			RouteName: routeKey,
		}

		impl, mergedParams, err := e.registry.GetInstance(pc.Name, pc.Version, metadata, pc.Parameters)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy instance %s:%s: %w", pc.Name, pc.Version, err)
		}

		spec := policy.PolicySpec{
			Name:               pc.Name,
			Version:            pc.Version,
			Enabled:            pc.Enabled,
			ExecutionCondition: pc.ExecutionCondition,
			Parameters: policy.PolicyParameters{
				Raw: mergedParams,
			},
		}

		if pc.ExecutionCondition != nil && *pc.ExecutionCondition != "" {
			hasExecutionConditions = true
		}

		policyList = append(policyList, impl)
		policySpecs = append(policySpecs, spec)

		mode := impl.Mode()
		if mode.RequestBodyMode == policy.BodyModeBuffer || mode.RequestBodyMode == policy.BodyModeStream {
			requiresRequestBody = true
			hasRequestBodyPolicy = true
			if _, streaming := impl.(policy.StreamingRequestPolicy); !streaming {
				supportsRequestStreaming = false
			}
		}
		if mode.ResponseBodyMode == policy.BodyModeBuffer || mode.ResponseBodyMode == policy.BodyModeStream {
			requiresResponseBody = true
			hasResponseBodyPolicy = true
			if _, streaming := impl.(policy.StreamingResponsePolicy); !streaming {
				supportsResponseStreaming = false
			}
		}
		if _, ok := impl.(policy.RequestHeaderPolicy); ok {
			requiresRequestHeader = true
		}
		if _, ok := impl.(policy.ResponseHeaderPolicy); ok {
			requiresResponseHeader = true
		}
	}

	if !hasRequestBodyPolicy {
		supportsRequestStreaming = false
	}
	if !hasResponseBodyPolicy {
		supportsResponseStreaming = false
	}

	return &registry.PolicyChain{
		Policies:                  policyList,
		PolicySpecs:               policySpecs,
		RequiresRequestBody:       requiresRequestBody,
		RequiresResponseBody:      requiresResponseBody,
		HasExecutionConditions:    hasExecutionConditions,
		RequiresRequestHeader:     requiresRequestHeader,
		RequiresResponseHeader:    requiresResponseHeader,
		SupportsRequestStreaming:  supportsRequestStreaming,
		SupportsResponseStreaming: supportsResponseStreaming,
	}, nil
}

// BuildChain creates an executable policy chain from specs.
// This is useful for programmatic chain construction (e.g., from channel bindings).
func (e *Engine) BuildChain(routeKey string, specs []PolicySpec) (*registry.PolicyChain, error) {
	instances := make([]policyengine.PolicyInstance, len(specs))
	for i, s := range specs {
		instances[i] = policyengine.PolicyInstance{
			Name:               s.Name,
			Version:            s.Version,
			Enabled:            s.Enabled,
			ExecutionCondition: s.ExecutionCondition,
			Parameters:         s.Parameters,
		}
	}
	config := &policyengine.PolicyChain{
		RouteKey: routeKey,
		Policies: instances,
	}
	return e.buildPolicyChain(routeKey, config)
}

// PolicySpec describes a policy to include in a chain (public facade type).
type PolicySpec struct {
	Name               string
	Version            string
	Enabled            bool
	ExecutionCondition *string
	Parameters         map[string]interface{}
}
