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
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied.  See the License for the specific
 * language governing permissions and limitations under the License.
 */

package platformgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

func (g *Gateway) registerResourceTemplateSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I create (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) from "([^"]*)" with values:$`,
		g.createResourceFromTemplate)
	sc.Step(`^I update (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)" from "([^"]*)" with values:$`,
		g.updateResourceFromTemplate)
	sc.Step(`^the first attached LLM provider policy should be "([^"]*)" version "([^"]*)"$`,
		g.firstLLMProviderPolicyIs)
}

func (g *Gateway) updateResourceFromTemplate(
	ctx context.Context, kind, resourceName, templateName string, table *godog.Table,
) error {
	path, err := g.templatePath(templateName)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read resource template %q: %w", templateName, err)
	}
	definition, err := g.renderResourceTemplate(ctx, kind, templateName, content, table)
	if err != nil {
		return fmt.Errorf("resource template %q: %w", templateName, err)
	}
	name, err := stepscommon.Expand(ctx, resourceName)
	if err != nil {
		return err
	}
	return g.updateResource(ctx, kind, name, &godog.DocString{Content: definition})
}

func (g *Gateway) createResourceFromTemplate(
	ctx context.Context, kind, templateName string, table *godog.Table,
) error {
	path, err := g.templatePath(templateName)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read resource template %q: %w", templateName, err)
	}
	definition, err := g.renderResourceTemplate(ctx, kind, templateName, content, table)
	if err != nil {
		return fmt.Errorf("resource template %q: %w", templateName, err)
	}
	return g.createResource(ctx, kind, &godog.DocString{Content: definition})
}

func (g *Gateway) renderResourceTemplate(
	ctx context.Context, kind, templateName string, content []byte, table *godog.Table,
) (string, error) {
	definition, err := stepscommon.RenderResourceTemplate(ctx, templateName, content, table)
	if err != nil {
		return "", err
	}
	if kind == "API" && usesLegacyUpstreamPath(gatewayVersion(g.topo)) {
		definition, err = legacyRestAPIUpstreamDefinition(definition)
		if err != nil {
			return "", err
		}
	}
	if kind == "API" && usesLegacyGatewayContract(gatewayVersion(g.topo)) {
		definition, err = legacySemanticCacheDefinition(definition)
		if err != nil {
			return "", err
		}
	}
	if !usesLegacyLLMResource(kind, gatewayVersion(g.topo)) {
		return definition, nil
	}
	return legacyLLMPolicyDefinition(definition)
}

// usesLegacyUpstreamPath identifies Gateway releases that take the upstream
// path from each upstream URL instead of upstreamDefinitions.basePath.
func usesLegacyUpstreamPath(version string) bool {
	return usesLegacyGatewayContract(version)
}

func legacyRestAPIUpstreamDefinition(definition string) (string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(definition), &document); err != nil {
		return "", fmt.Errorf("parse API definition: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("API definition must contain one mapping document")
	}

	spec, found := yamlMappingValue(document.Content[0], "spec")
	if !found || spec.Kind != yaml.MappingNode {
		return "", fmt.Errorf("API definition is missing a mapping spec")
	}
	definitions, found := yamlMappingValue(spec, "upstreamDefinitions")
	if !found {
		return definition, nil
	}
	if definitions.Kind != yaml.SequenceNode {
		return "", fmt.Errorf("spec.upstreamDefinitions must be a sequence")
	}

	for _, upstreamDefinition := range definitions.Content {
		if upstreamDefinition.Kind != yaml.MappingNode {
			return "", fmt.Errorf("spec.upstreamDefinitions entries must be mappings")
		}
		basePathNode, hasBasePath := yamlMappingValue(upstreamDefinition, "basePath")
		if !hasBasePath {
			continue
		}
		basePath := strings.TrimSpace(basePathNode.Value)
		upstreams, hasUpstreams := yamlMappingValue(upstreamDefinition, "upstreams")
		if !hasUpstreams || upstreams.Kind != yaml.SequenceNode {
			return "", fmt.Errorf("spec.upstreamDefinitions.upstreams must be a sequence")
		}
		for _, upstream := range upstreams.Content {
			if upstream.Kind != yaml.MappingNode {
				return "", fmt.Errorf("spec.upstreamDefinitions.upstreams entries must be mappings")
			}
			urlNode, hasURL := yamlMappingValue(upstream, "url")
			if !hasURL {
				return "", fmt.Errorf("spec.upstreamDefinitions.upstreams entry is missing url")
			}
			parsed, err := url.Parse(urlNode.Value)
			if err != nil {
				return "", fmt.Errorf("parse upstream URL %q: %w", urlNode.Value, err)
			}
			parsed.Path = joinUpstreamPath(basePath, parsed.Path)
			urlNode.Value = parsed.String()
		}
		yamlDeleteMappingKey(upstreamDefinition, "basePath")
	}

	var rendered bytes.Buffer
	encoder := yaml.NewEncoder(&rendered)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return "", fmt.Errorf("encode legacy API definition: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close legacy API definition encoder: %w", err)
	}
	return rendered.String(), nil
}

// legacySemanticCacheDefinition removes semantic-cache parameters that were
// introduced after Gateway 1.1.0. The feature describes the current policy
// contract once; this adapter keeps the same semantic-cache scenarios usable
// against the older policy schema without weakening the assertions.
func legacySemanticCacheDefinition(definition string) (string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(definition), &document); err != nil {
		return "", fmt.Errorf("parse API definition: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("API definition must contain one mapping document")
	}

	spec, found := yamlMappingValue(document.Content[0], "spec")
	if !found || spec.Kind != yaml.MappingNode {
		return definition, nil
	}
	operations, found := yamlMappingValue(spec, "operations")
	if !found || operations.Kind != yaml.SequenceNode {
		return definition, nil
	}

	changed := false
	for _, operation := range operations.Content {
		if operation.Kind != yaml.MappingNode {
			continue
		}
		policies, found := yamlMappingValue(operation, "policies")
		if !found || policies.Kind != yaml.SequenceNode {
			continue
		}
		for _, policy := range policies.Content {
			if policy.Kind != yaml.MappingNode {
				continue
			}
			name, found := yamlMappingValue(policy, "name")
			if !found || name.Value != "semantic-cache" {
				continue
			}
			params, found := yamlMappingValue(policy, "params")
			if !found || params.Kind != yaml.MappingNode {
				continue
			}
			changed = yamlDeleteMappingKey(params, "cacheUnauthenticated") || changed
		}
	}
	if !changed {
		return definition, nil
	}

	var rendered bytes.Buffer
	encoder := yaml.NewEncoder(&rendered)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return "", fmt.Errorf("encode legacy semantic-cache API definition: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close legacy semantic-cache API definition encoder: %w", err)
	}
	return rendered.String(), nil
}

func joinUpstreamPath(basePath, upstreamPath string) string {
	basePath = strings.TrimRight(basePath, "/")
	upstreamPath = strings.TrimLeft(upstreamPath, "/")
	if basePath == "" {
		if upstreamPath == "" {
			return ""
		}
		return "/" + upstreamPath
	}
	if upstreamPath == "" {
		return basePath
	}
	return basePath + "/" + upstreamPath
}

func usesLegacyLLMResource(kind, version string) bool {
	return (kind == "LLM provider" || kind == "LLM proxy") && usesLegacyLLMContract(version)
}

// usesLegacyLLMContract identifies Gateway releases that use the legacy LLM
// policy and response contracts. Gateway 1.2 introduced the newer contracts.
func usesLegacyLLMContract(version string) bool {
	return usesLegacyGatewayContract(version)
}

// usesLegacyGatewayContract identifies Gateway releases that use the pre-1.2
// resource and routing contracts.
func usesLegacyGatewayContract(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	version, _, _ = strings.Cut(version, "-")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	patch, patchErr := strconv.Atoi(parts[2])
	if majorErr != nil || minorErr != nil || patchErr != nil || major < 0 || minor < 0 || patch < 0 {
		return false
	}
	return major < 1 || (major == 1 && (minor < 1 || (minor == 1 && patch == 0)))
}

func llmPolicyFieldForVersion(version string) string {
	if usesLegacyLLMContract(version) {
		return "spec.policies"
	}
	return "spec.operationPolicies"
}

// legacyLLMPolicyDefinition maps the 1.2+ operationPolicies contract to the 1.1
// policies contract without discarding an unsupported execution condition.
func legacyLLMPolicyDefinition(definition string) (string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(definition), &document); err != nil {
		return "", fmt.Errorf("parse LLM definition: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("LLM definition must contain one mapping document")
	}

	spec, found := yamlMappingValue(document.Content[0], "spec")
	if !found || spec.Kind != yaml.MappingNode {
		return "", fmt.Errorf("LLM definition is missing a mapping spec")
	}
	operationPolicies, found := yamlMappingValue(spec, "operationPolicies")
	if !found {
		return definition, nil
	}
	if _, hasLegacyPolicies := yamlMappingValue(spec, "policies"); hasLegacyPolicies {
		return "", fmt.Errorf("LLM definition cannot set both spec.operationPolicies and spec.policies")
	}
	if operationPolicies.Kind != yaml.SequenceNode {
		return "", fmt.Errorf("spec.operationPolicies must be a sequence")
	}
	for _, policy := range operationPolicies.Content {
		if policy.Kind != yaml.MappingNode {
			return "", fmt.Errorf("spec.operationPolicies entries must be mappings")
		}
		if _, hasExecutionCondition := yamlMappingValue(policy, "executionCondition"); hasExecutionCondition {
			return "", fmt.Errorf("Gateway 1.1.0 does not support spec.operationPolicies[].executionCondition")
		}
	}
	if !yamlRenameMappingKey(spec, "operationPolicies", "policies") {
		return "", fmt.Errorf("rename spec.operationPolicies to spec.policies")
	}

	var rendered bytes.Buffer
	encoder := yaml.NewEncoder(&rendered)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return "", fmt.Errorf("encode legacy LLM definition: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close legacy LLM definition encoder: %w", err)
	}
	return rendered.String(), nil
}

func yamlMappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], true
		}
	}
	return nil, false
}

func yamlRenameMappingKey(mapping *yaml.Node, from, to string) bool {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == from {
			mapping.Content[index].Value = to
			return true
		}
	}
	return false
}

func yamlDeleteMappingKey(mapping *yaml.Node, key string) bool {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
			return true
		}
	}
	return false
}

func (g *Gateway) firstLLMProviderPolicyIs(ctx context.Context, wantName, wantVersion string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var document map[string]any
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return fmt.Errorf("parse LLM provider response: %w", err)
	}
	field := llmPolicyFieldForVersion(gatewayVersion(g.topo))
	name, found := traverseJSON(document, field+"[0].name")
	if !found {
		return fmt.Errorf("first attached LLM provider policy name is absent at %q", field+"[0].name")
	}
	version, found := traverseJSON(document, field+"[0].version")
	if !found {
		return fmt.Errorf("first attached LLM provider policy version is absent at %q", field+"[0].version")
	}
	resolvedName, err := stepscommon.Expand(ctx, wantName)
	if err != nil {
		return err
	}
	resolvedVersion, err := stepscommon.Expand(ctx, wantVersion)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(name); got != resolvedName {
		return fmt.Errorf("first attached LLM provider policy: expected name %q, got %q", resolvedName, got)
	}
	if got := fmt.Sprint(version); got != resolvedVersion {
		return fmt.Errorf("first attached LLM provider policy: expected version %q, got %q", resolvedVersion, got)
	}
	return nil
}

func (g *Gateway) templatePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("resource template path is required")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("resource template path must be relative: %q", name)
	}
	root := g.featureRoot
	if root == "" {
		return "", fmt.Errorf("resource template root is not configured")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve resource template root: %w", err)
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resource template path escapes the suite resource root: %q", name)
	}
	path := filepath.Join(root, clean)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve resource template root: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err == nil && !isWithinPath(resolvedRoot, resolvedPath) {
		return "", fmt.Errorf("resource template path escapes the suite resource root: %q", name)
	}
	return path, nil
}

func isWithinPath(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
