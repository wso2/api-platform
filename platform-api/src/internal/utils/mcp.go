/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package utils

import (
	"fmt"
	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"strings"

	"gopkg.in/yaml.v3"
)

type MCPUtils struct{}

func (u *MCPUtils) GenerateMCPDeploymentYAML(proxy *model.MCPProxy) (string, error) {

	contextValue := "/"
	if proxy.Configuration.Context != nil && *proxy.Configuration.Context != "" {
		contextValue = *proxy.Configuration.Context
	}
	vhostValue := ""
	if proxy.Configuration.Vhost != nil {
		vhostValue = *proxy.Configuration.Vhost
	}

	mcpDeploymentYaml := model.MCPProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.MCPProxy,
		Metadata: model.DeploymentMetadata{
			Name: proxy.Handle,
		},
		Spec: model.MCPProxyDeploymentSpec{
			DisplayName: proxy.Name,
			Version:     proxy.Version,
			Context:     contextValue,
			Vhost:       vhostValue,
			Upstream:    proxy.Configuration.Upstream,
			SpecVersion: proxy.Configuration.SpecVersion,
			Policies:    []model.Policy{},
		},
	}

	if proxy.ProjectUUID != nil {
		mcpDeploymentYaml.Metadata.Labels = map[string]string{
			"projectId": *proxy.ProjectUUID,
		}
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(mcpDeploymentYaml)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API to YAML: %w", err)
	}

	return string(yamlBytes), nil

}

func mapUpstreamAuthModelToAPI(in *model.UpstreamAuth) *api.UpstreamAuth {
	if in == nil {
		return nil
	}
	var authType *api.UpstreamAuthType
	if normalized := normalizeUpstreamAuthType(in.Type); normalized != "" {
		t := api.UpstreamAuthType(normalized)
		authType = &t
	}
	return &api.UpstreamAuth{
		Type:   authType,
		Header: StringPtrIfNotEmpty(in.Header),
		Value:  StringPtrIfNotEmpty(in.Value),
	}
}

func normalizeUpstreamAuthType(authType string) string {
	normalized := strings.TrimSpace(authType)
	if normalized == "" {
		return ""
	}

	canonical := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(normalized, "-", ""), "_", ""))
	switch canonical {
	case "apikey":
		return string(api.ApiKey)
	case "basic":
		return string(api.Basic)
	case "bearer":
		return string(api.Bearer)
	default:
		return normalized
	}
}
