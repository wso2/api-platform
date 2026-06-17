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

package project

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// manifestMeta is the metadata block shared by every generated manifest.
type manifestMeta struct {
	Name string `yaml:"name"`
}

// BusinessInformation captures the ownership metadata carried by management
// (REST) artifacts.
type BusinessInformation struct {
	BusinessOwner       string `yaml:"businessOwner"`
	BusinessOwnerEmail  string `yaml:"businessOwnerEmail"`
	TechnicalOwner      string `yaml:"technicalOwner"`
	TechnicalOwnerEmail string `yaml:"technicalOwnerEmail"`
}

// Endpoints captures the backend endpoints carried by management artifacts.
type Endpoints struct {
	SandboxURL    string `yaml:"sandboxUrl"`
	ProductionURL string `yaml:"productionUrl"`
}

type manifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   manifestMeta `yaml:"metadata"`
	Spec       interface{}  `yaml:"spec"`
}

// managementMetadataSpec is the metadata.yaml spec for management-plane (REST)
// artifacts.
type managementMetadataSpec struct {
	DisplayName         string              `yaml:"displayName"`
	Version             string              `yaml:"version"`
	Description         string              `yaml:"description"`
	GatewayType         string              `yaml:"gatewayType"`
	Status              string              `yaml:"status"`
	ReferenceID         string              `yaml:"referenceID"`
	Tags                []string            `yaml:"tags"`
	Labels              []string            `yaml:"labels"`
	BusinessInformation BusinessInformation `yaml:"businessInformation"`
	Endpoints           Endpoints           `yaml:"endpoints"`
}

// aiWorkspaceMetadataSpec is the slimmer metadata.yaml spec for ai-workspace
// artifacts (LLM proxies/providers, MCP proxies).
type aiWorkspaceMetadataSpec struct {
	DisplayName string `yaml:"displayName"`
	Version     string `yaml:"version"`
}

type runtimeSpec struct {
	DisplayName string             `yaml:"displayName"`
	Version     string             `yaml:"version"`
	Context     string             `yaml:"context"`
	Upstream    runtimeUpstream    `yaml:"upstream"`
	Operations  []runtimeOperation `yaml:"operations"`
}

type runtimeUpstream struct {
	Main runtimeUpstreamTarget `yaml:"main"`
}

type runtimeUpstreamTarget struct {
	URL string `yaml:"url"`
}

type runtimeOperation struct {
	Path   string `yaml:"path"`
	Method string `yaml:"method"`
}

// BuildConfigYAML renders the default .api-platform/config.yaml for a new
// project, sourcing the file-path values from the shared FilePaths struct so
// the scaffold and the loader can never drift apart.
func BuildConfigYAML() (string, error) {
	config := Config{
		Version:            DefaultConfigVersion,
		FilePaths:          DefaultFilePaths(),
		GovernanceRulesets: []string{},
		AutoSync: map[string]interface{}{
			"gatewayArtifactFromDefinition": true,
		},
	}

	return renderYAML(config, map[string]string{
		"filePaths":          "Default file paths (can be customized)",
		"governanceRulesets": "Governance rulesets for design-time validation",
		"autoSync":           "Auto-sync configuration for vscode plugin",
	}, map[string]string{
		"autoSync.gatewayArtifactFromDefinition": "Auto-generate runtime.yaml when definition.yaml changes",
	})
}

// BuildMetadataYAML renders the default metadata.yaml for the given artifact
// type. Management (REST) artifacts get the full business/ownership metadata;
// ai-workspace artifacts get the slim displayName/version form.
func BuildMetadataYAML(artifactType, resourceName, displayName string) (string, error) {
	m := manifest{
		APIVersion: MetadataAPIVersion(artifactType),
		Kind:       ArtifactKind(artifactType),
		Metadata:   manifestMeta{Name: resourceName},
	}

	if IsAIWorkspaceType(artifactType) {
		m.Spec = aiWorkspaceMetadataSpec{
			DisplayName: displayName,
			Version:     "v1.0",
		}
	} else {
		m.Spec = managementMetadataSpec{
			DisplayName: displayName,
			Version:     "v1.0",
			GatewayType: DefaultGatewayType,
			Status:      "PUBLISHED",
			Tags:        []string{},
			Labels:      []string{},
		}
	}

	return renderYAML(m, nil, nil)
}

// BuildRuntimeYAML renders the default runtime.yaml (the gateway deployment
// artifact) for the given artifact type.
func BuildRuntimeYAML(artifactType, resourceName, displayName string) (string, error) {
	m := manifest{
		APIVersion: GatewayAPIVersion,
		Kind:       ArtifactKind(artifactType),
		Metadata:   manifestMeta{Name: resourceName},
		Spec: runtimeSpec{
			DisplayName: displayName,
			Upstream: runtimeUpstream{
				Main: runtimeUpstreamTarget{URL: "http://sample-backend.org:9080"},
			},
			Operations: []runtimeOperation{
				{Path: "/*", Method: "GET"},
				{Path: "/*", Method: "POST"},
				{Path: "/*", Method: "PUT"},
				{Path: "/*", Method: "DELETE"},
				{Path: "/*", Method: "OPTIONS"},
			},
		},
	}

	return renderYAML(m, nil, map[string]string{
		"spec.upstream.main.url": "Change this to your backend URL",
	})
}

// BuildDefinitionYAML renders the default OpenAPI definition.yaml.
func BuildDefinitionYAML(displayName string) string {
	return fmt.Sprintf(`openapi: 3.0.3
info:
  title: %q
  version: v1.0
servers:
  - url: https://example.com
paths:
  "/*":
    get:
      responses:
        "200":
          description: OK
    post:
      responses:
        "200":
          description: OK
    put:
      responses:
        "200":
          description: OK
    delete:
      responses:
        "200":
          description: OK
    options:
      responses:
        "200":
          description: OK
`, displayName)
}

// renderYAML marshals v to YAML and attaches the supplied head/line comments to
// the addressed keys. Comment keys are dotted paths into the document
// (e.g. "spec.upstream.main.url").
func renderYAML(v interface{}, headComments, lineComments map[string]string) (string, error) {
	var doc yaml.Node
	if err := doc.Encode(v); err != nil {
		return "", fmt.Errorf("failed to encode manifest: %w", err)
	}

	for path, comment := range headComments {
		applyComment(&doc, splitPath(path), comment, true)
	}
	for path, comment := range lineComments {
		applyComment(&doc, splitPath(path), comment, false)
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		return "", fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to flush manifest: %w", err)
	}
	return buf.String(), nil
}

func splitPath(path string) []string {
	segments := make([]string, 0)
	for _, segment := range splitOnDot(path) {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func splitOnDot(path string) []string {
	var segments []string
	current := ""
	for _, r := range path {
		if r == '.' {
			segments = append(segments, current)
			current = ""
			continue
		}
		current += string(r)
	}
	return append(segments, current)
}

// applyComment walks the mapping nodes of doc following path and sets a head or
// line comment on the addressed key. Missing keys are ignored so callers can
// describe optional fields without guarding each one.
func applyComment(doc *yaml.Node, path []string, comment string, head bool) {
	if len(path) == 0 {
		return
	}

	node := doc
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	for depth, key := range path {
		if node.Kind != yaml.MappingNode {
			return
		}
		matched := false
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			if keyNode.Value != key {
				continue
			}
			if depth == len(path)-1 {
				if head {
					keyNode.HeadComment = comment
				} else {
					keyNode.LineComment = comment
				}
				return
			}
			node = node.Content[i+1]
			matched = true
			break
		}
		if !matched {
			return
		}
	}
}
