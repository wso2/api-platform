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

package catalog

import (
	"context"
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog/aiworkspace"
	"github.com/wso2/api-platform/tests/framework/core/catalog/apiportal"
	"github.com/wso2/api-platform/tests/framework/core/catalog/browser"
	"github.com/wso2/api-platform/tests/framework/core/catalog/infrastructure"
	"github.com/wso2/api-platform/tests/framework/core/catalog/platformapi"
	"github.com/wso2/api-platform/tests/framework/core/catalog/platformgateway"
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/catalog/testbench"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/topology"
)

// BuildSpec returns the source-build metadata for a catalog product.
func BuildSpec(component, version string) (builder.Spec, error) {
	switch component {
	case "platform-gateway":
		return platformgateway.BuildSpec(version)
	case "platform-api":
		return platformapi.BuildSpec(version)
	case "api-portal":
		return apiportal.BuildSpec(version)
	case "ai-workspace":
		return aiworkspace.BuildSpec(version)
	default:
		return builder.Spec{}, fmt.Errorf("catalog: no source builder for %q", component)
	}
}

// BuildSources builds each unversioned source product used by a resolved suite once.
func BuildSources(ctx context.Context, resolved *topology.Resolved, root string, runner builder.Runner, coverage bool) error {
	if resolved == nil {
		return fmt.Errorf("catalog: resolved suite is required")
	}
	seen := map[string]bool{}
	products := make([]builder.Product, 0)
	for _, block := range resolved.Blocks {
		for _, component := range block.Components {
			if component.Def == nil || component.Version != "" || seen[component.Def.Name] {
				continue
			}
			version, ok := shared.SourceVersion(component.Def.Name)
			if !ok {
				continue
			}
			spec, err := BuildSpec(component.Def.Name, version)
			if err != nil {
				return fmt.Errorf("catalog: preparing source build for %s: %w", component.Def.Name, err)
			}
			products = append(products, builder.Product{Spec: spec, Version: version})
			seen[component.Def.Name] = true
		}
	}
	if len(products) == 0 {
		return nil
	}
	return builder.BuildProducts(ctx, products, root, runner, coverage)
}

// All returns every component definition the catalog knows about.
//
// Order is irrelevant — the registry indexes by name and validates cross-references
// afterwards — but grouping mirrors how the suites use them.
func All() []*components.Definition {
	return []*components.Definition{
		// The gateway, as ONE component. Its controller and runtime are compose services
		// inside it and are deliberately not separately referenceable: a suite file has no
		// reason to know the gateway is two containers, and could otherwise compose a
		// runtime with no controller, which is not a gateway.
		platformgateway.PlatformGateway(),

		// The control plane — the REAL product, not a mock. The gateway's cross-plane
		// contract is only meaningfully tested against it; see PlatformAPI's doc comment
		// and the mock rule in docs/migration-policy.md.
		platformapi.PlatformAPI(),

		// The developer portal. Present because it is the only issuer of subscription tokens —
		// see APIPortal's doc comment. A block needs it only for subscription scenarios.
		apiportal.APIPortal(),

		// The AI portal — the UI suite's system under test. See AIWorkspace's doc comment.
		aiworkspace.AIWorkspace(),

		// The Playwright server the UI suite's scenarios drive, on the block's network.
		browser.Browser(),

		// Backends APIs are routed to.
		testbench.Testbench(),

		// Infrastructure.
		infrastructure.Redis(),
	}
}

// Registry builds and validates a registry of every catalog component.
//
// Returned as a value rather than exposed as a package-level singleton: a global would
// make registration order significant and is the kind of shared mutable state this
// framework bans. A suite calls this once and hands the result to the topology loader.
func Registry() (*components.Registry, error) {
	r := components.NewRegistry()
	for _, d := range All() {
		if err := r.Register(d); err != nil {
			return nil, fmt.Errorf("catalog: %w", err)
		}
	}
	// Cross-definition checks: store sharing resolves to a real owner, dependencies
	// exist, no dependency cycles. Run before any container starts.
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	return r, nil
}

// MustRegistry is Registry for a suite's own initialisation, where a broken catalog is a
// programming error that should stop immediately rather than resurface later as a
// confusing topology-load failure.
func MustRegistry() *components.Registry {
	r, err := Registry()
	if err != nil {
		panic(err)
	}
	return r
}
