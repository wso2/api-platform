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

package common

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"
)

const templateValuePrefix = "${VALUE:"

// RenderResourceTemplate renders a canonical or legacy suite resource template.
func RenderResourceTemplate(ctx context.Context, name string, content []byte, table *godog.Table) (string, error) {
	if isCanonicalResourceTemplate(name) {
		return renderCanonicalTemplate(ctx, content, table)
	}
	definition, err := substituteTemplateValues(string(content), table)
	if err != nil {
		return "", err
	}
	return Expand(ctx, definition)
}

func isCanonicalResourceTemplate(name string) bool {
	clean := filepath.ToSlash(filepath.Clean(name))
	return strings.HasPrefix(clean, "resources/templates/")
}

func renderCanonicalTemplate(ctx context.Context, content []byte, table *godog.Table) (string, error) {
	values, err := templateValues(table)
	if err != nil {
		return "", err
	}

	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return "", fmt.Errorf("parse canonical template: %w", err)
	}
	if len(document.Content) != 1 {
		return "", fmt.Errorf("canonical template must contain one YAML document")
	}

	used := make(map[string]bool)
	if err := replaceCanonicalPlaceholders(ctx, document.Content[0], values, used); err != nil {
		return "", err
	}
	for key, value := range values {
		if used[key] {
			continue
		}
		if err := setCanonicalField(ctx, document.Content[0], key, value); err != nil {
			return "", err
		}
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return "", fmt.Errorf("encode canonical template: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("close canonical template encoder: %w", err)
	}
	return out.String(), nil
}

func replaceCanonicalPlaceholders(
	ctx context.Context, node *yaml.Node, values map[string]string, used map[string]bool,
) error {
	if node.Kind == yaml.ScalarNode && strings.HasPrefix(node.Value, templateValuePrefix) && strings.HasSuffix(node.Value, "}") {
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(node.Value, templateValuePrefix), "}"))
		if key == "" {
			return fmt.Errorf("template placeholder has an empty key")
		}
		value, ok := values[key]
		if !ok {
			return fmt.Errorf("no value supplied for %q", key)
		}
		expanded, err := Expand(ctx, value)
		if err != nil {
			return err
		}
		replacement, err := yamlValueNode(expanded)
		if err != nil {
			return fmt.Errorf("value %q: %w", key, err)
		}
		*node = *replacement
		used[key] = true
		return nil
	}
	for _, child := range node.Content {
		if err := replaceCanonicalPlaceholders(ctx, child, values, used); err != nil {
			return err
		}
	}
	return nil
}

func yamlValueNode(value string) (*yaml.Node, error) {
	if strings.TrimSpace(value) == "" {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""}, nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(value), &document); err != nil {
		return nil, fmt.Errorf("parse value as YAML: %w", err)
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("value must contain one YAML node")
	}
	return document.Content[0], nil
}

func setCanonicalField(ctx context.Context, root *yaml.Node, path, value string) error {
	parts := strings.Split(path, ".")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			return fmt.Errorf("field path %q contains an empty segment", path)
		}
	}
	expanded, err := Expand(ctx, value)
	if err != nil {
		return err
	}
	replacement, err := yamlValueNode(expanded)
	if err != nil {
		return fmt.Errorf("field %q: %w", path, err)
	}

	current := root
	if current.Kind == yaml.DocumentNode {
		if len(current.Content) != 1 {
			return fmt.Errorf("field %q: document has no root node", path)
		}
		current = current.Content[0]
	}
	for i, part := range parts {
		if current.Kind != yaml.MappingNode {
			return fmt.Errorf("field %q: %q is not a mapping", path, strings.Join(parts[:i], "."))
		}
		valueIndex := mappingValueIndex(current, part)
		if i == len(parts)-1 {
			if valueIndex >= 0 {
				current.Content[valueIndex] = replacement
			} else {
				current.Content = append(current.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: part}, replacement)
			}
			return nil
		}
		if valueIndex < 0 {
			key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: part}
			child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			current.Content = append(current.Content, key, child)
			current = child
			continue
		}
		current = current.Content[valueIndex]
	}
	return nil
}

func mappingValueIndex(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i + 1
		}
	}
	return -1
}

func substituteTemplateValues(template string, table *godog.Table) (string, error) {
	values, err := templateValues(table)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	from := 0
	for from < len(template) {
		relStart := strings.Index(template[from:], templateValuePrefix)
		if relStart < 0 {
			b.WriteString(template[from:])
			break
		}
		start := from + relStart
		b.WriteString(template[from:start])
		relEnd := strings.IndexByte(template[start+len(templateValuePrefix):], '}')
		if relEnd < 0 {
			return "", fmt.Errorf("unterminated template placeholder at byte %d", start)
		}
		end := start + len(templateValuePrefix) + relEnd
		key := strings.TrimSpace(template[start+len(templateValuePrefix) : end])
		if key == "" {
			return "", fmt.Errorf("template placeholder has an empty key")
		}
		value, ok := values[key]
		if !ok {
			return "", fmt.Errorf("no value supplied for %q", key)
		}
		if strings.Contains(value, templateValuePrefix) {
			return "", fmt.Errorf("value for %q contains a recursive template placeholder", key)
		}
		b.WriteString(value)
		from = end + 1
	}
	return b.String(), nil
}

func templateValues(table *godog.Table) (map[string]string, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("values table is required")
	}
	values := make(map[string]string, len(table.Rows))
	for i, row := range table.Rows {
		if row == nil || len(row.Cells) != 2 {
			return nil, fmt.Errorf("values row %d must contain exactly two cells", i+1)
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		if key == "" {
			return nil, fmt.Errorf("values row %d has an empty key", i+1)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate value key %q", key)
		}
		values[key] = row.Cells[1].Value
	}
	return values, nil
}
