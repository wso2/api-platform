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
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/builder"
	"github.com/wso2/api-platform/tests/framework/core/catalog/aiworkspace"
	"github.com/wso2/api-platform/tests/framework/core/catalog/apiportal"
	"github.com/wso2/api-platform/tests/framework/core/catalog/browser"
	"github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
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

// BuildSources builds each unversioned source product used by a resolved suite once,
// including custom platform-gateway images requested with addPoliciesFrom.
func BuildSources(ctx context.Context, resolved *topology.Resolved, root string, runner builder.Runner, coverage bool) error {
	if resolved == nil {
		return fmt.Errorf("catalog: resolved suite is required")
	}
	products, err := sourceProducts(resolved)
	if err != nil {
		return err
	}
	if len(products) == 0 {
		return buildPolicyProducts(ctx, resolved, root, runner, coverage)
	}
	if err := builder.BuildProducts(ctx, products, root, runner, coverage); err != nil {
		return err
	}
	return buildPolicyProducts(ctx, resolved, root, runner, coverage)
}

func sourceProducts(resolved *topology.Resolved) ([]builder.Product, error) {
	seen := map[string]bool{}
	products := make([]builder.Product, 0)
	for blockIndex := range resolved.Blocks {
		for componentIndex := range resolved.Blocks[blockIndex].Components {
			component := &resolved.Blocks[blockIndex].Components[componentIndex]
			if component.Def == nil || component.Version != "" || component.AddPoliciesFrom != "" {
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

type policyProduct struct {
	component       string
	version         string
	source          string
	buildFromSource bool
}

func buildPolicyProducts(
	ctx context.Context, resolved *topology.Resolved, root string, runner builder.Runner, coverage bool,
) error {
	products, err := policyProducts(resolved)
	if err != nil {
		return err
	}
	for _, product := range products {
		switch {
		case product.buildFromSource:
			images, buildErr := platformgateway.BuildSourceWithPolicies(
				ctx, root, product.version, product.source, runner, coverage,
			)
			if buildErr != nil {
				return fmt.Errorf("catalog: building %s with policies from %q: %w",
					product.component, product.source, buildErr)
			}
			setPlatformGatewayImages(resolved, product, images)
		default:
			baseController, baseRuntime, imageErr := platformGatewayBaseImages(resolved, product)
			if imageErr != nil {
				return imageErr
			}
			images, buildErr := platformgateway.BuildVersionedWithPolicies(
				ctx, root, product.version, product.source, baseController, baseRuntime, runner,
			)
			if buildErr != nil {
				return fmt.Errorf("catalog: extending %s:%s with policies from %q: %w",
					product.component, product.version, product.source, buildErr)
			}
			setPlatformGatewayImages(resolved, product, images)
		}
	}
	return nil
}

func policyProducts(resolved *topology.Resolved) ([]policyProduct, error) {
	seen := map[string]bool{}
	products := make([]policyProduct, 0)
	for blockIndex := range resolved.Blocks {
		for componentIndex := range resolved.Blocks[blockIndex].Components {
			component := &resolved.Blocks[blockIndex].Components[componentIndex]
			if component.Def == nil || strings.TrimSpace(component.AddPoliciesFrom) == "" {
				continue
			}
			version := strings.TrimSpace(component.Version)
			fromSource := version == ""
			if fromSource {
				var ok bool
				version, ok = shared.SourceVersion(component.Def.Name)
				if !ok {
					return nil, fmt.Errorf("catalog: no source version for %q", component.Def.Name)
				}
				component.Version = version
			}
			key := component.Def.Name + "\x00" + version + "\x00" + component.AddPoliciesFrom + "\x00" + fmt.Sprint(fromSource)
			if seen[key] {
				continue
			}
			seen[key] = true
			products = append(products, policyProduct{
				component: component.Def.Name, version: version,
				source: component.AddPoliciesFrom, buildFromSource: fromSource,
			})
		}
	}
	return products, nil
}

func platformGatewayBaseImages(resolved *topology.Resolved, product policyProduct) (string, string, error) {
	for blockIndex := range resolved.Blocks {
		for componentIndex := range resolved.Blocks[blockIndex].Components {
			component := &resolved.Blocks[blockIndex].Components[componentIndex]
			if component.Def == nil || component.Def.Name != product.component ||
				component.Version != product.version || component.BuildFromSource != product.buildFromSource ||
				component.AddPoliciesFrom != product.source {
				continue
			}
			if component.Def.Compose == nil {
				return "", "", fmt.Errorf("catalog: %s is not compose-backed", product.component)
			}
			controller := strings.TrimSpace(component.Def.Compose.Env[platformgateway.EnvImagePGController])
			runtime := strings.TrimSpace(component.Def.Compose.Env[platformgateway.EnvImagePGRuntime])
			if controller == "" || runtime == "" {
				return "", "", fmt.Errorf("catalog: %s has no controller/runtime base images", product.component)
			}
			return controller, runtime, nil
		}
	}
	return "", "", fmt.Errorf("catalog: no resolved %s component for policy build", product.component)
}

func setPlatformGatewayImages(resolved *topology.Resolved, product policyProduct, images platformgateway.DerivedImages) {
	for blockIndex := range resolved.Blocks {
		for componentIndex := range resolved.Blocks[blockIndex].Components {
			component := &resolved.Blocks[blockIndex].Components[componentIndex]
			if component.Def == nil || component.Def.Name != product.component ||
				component.Version != product.version || component.BuildFromSource != product.buildFromSource ||
				component.AddPoliciesFrom != product.source {
				continue
			}
			def := *component.Def
			compose := *component.Def.Compose
			compose.Env = make(map[string]string, len(component.Def.Compose.Env)+2)
			for key, value := range component.Def.Compose.Env {
				compose.Env[key] = value
			}
			compose.Env[platformgateway.EnvImagePGController] = images.Controller
			compose.Env[platformgateway.EnvImagePGRuntime] = images.Runtime
			def.Compose = &compose
			component.Def = &def
		}
	}
}

// All returns every component definition in the catalog.
func All() []*components.Definition {
	return []*components.Definition{
		platformgateway.PlatformGateway(),
		platformapi.PlatformAPI(),
		apiportal.APIPortal(),
		aiworkspace.AIWorkspace(),
		browser.Browser(),
		cloudconsole.CloudConsole(),
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
