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
	products, err := sourceProducts(resolved)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return nil
	}
	return builder.BuildProducts(ctx, products, root, runner, coverage)
}

func sourceProducts(resolved *topology.Resolved) ([]builder.Product, error) {
	seen := map[string]bool{}
	products := make([]builder.Product, 0)
	for blockIndex := range resolved.Blocks {
		for componentIndex := range resolved.Blocks[blockIndex].Components {
			component := &resolved.Blocks[blockIndex].Components[componentIndex]
			if component.Def == nil || component.Version != "" {
				continue
			}
			version, ok := shared.SourceVersion(component.Def.Name)
			if !ok {
				continue
			}
			component.Version = version
			if seen[component.Def.Name] {
				continue
			}
			spec, err := BuildSpec(component.Def.Name, version)
			if err != nil {
				return nil, fmt.Errorf("catalog: preparing source build for %s: %w", component.Def.Name, err)
			}
			products = append(products, builder.Product{Spec: spec, Version: version})
			seen[component.Def.Name] = true
		}
	}
	return products, nil
}

// All returns every component definition in the catalog.
func All() []*components.Definition {
	return []*components.Definition{
		platformgateway.PlatformGateway(),
		platformapi.PlatformAPI(),
		apiportal.APIPortal(),
		aiworkspace.AIWorkspace(),
		browser.Browser(),
		testbench.Testbench(),
		infrastructure.Redis(),
	}
}

// Registry builds and validates the catalog registry.
func Registry() (*components.Registry, error) {
	r := components.NewRegistry()
	for _, d := range All() {
		if err := r.Register(d); err != nil {
			return nil, fmt.Errorf("catalog: %w", err)
		}
	}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	return r, nil
}

// MustRegistry returns the catalog registry or panics if validation fails.
func MustRegistry() *components.Registry {
	r, err := Registry()
	if err != nil {
		panic(err)
	}
	return r
}
