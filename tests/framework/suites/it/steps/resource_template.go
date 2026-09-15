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

package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"

	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

func (g *Gateway) registerResourceTemplateSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I create (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) from "([^"]*)" with values:$`,
		g.createResourceFromTemplate)
	sc.Step(`^I update (API|LLM provider|LLM provider template|MCP proxy|LLM proxy) "([^"]*)" from "([^"]*)" with values:$`,
		g.updateResourceFromTemplate)
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
	definition, err := stepscommon.RenderResourceTemplate(ctx, templateName, content, table)
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
	definition, err := stepscommon.RenderResourceTemplate(ctx, templateName, content, table)
	if err != nil {
		return fmt.Errorf("resource template %q: %w", templateName, err)
	}
	return g.createResource(ctx, kind, &godog.DocString{Content: definition})
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
