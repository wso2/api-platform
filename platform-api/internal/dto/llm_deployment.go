/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package dto

import "github.com/wso2/api-platform/platform-api/api"

// LLMProviderDeploymentYAML represents the LLM provider deployment YAML structure
// This format aligns with the gateway controller LLMProviderConfiguration schema.
type LLMProviderDeploymentYAML struct {
	ApiVersion string                    `yaml:"apiVersion"`
	Kind       string                    `yaml:"kind"`
	Metadata   DeploymentMetadata        `yaml:"metadata"`
	Spec       LLMProviderDeploymentSpec `yaml:"spec"`
}

// LLMProviderDeploymentSpec represents the spec section for LLM provider deployments
type LLMProviderDeploymentSpec struct {
	DisplayName       string                `yaml:"displayName"`
	Version           string                `yaml:"version"`
	Context           string                `yaml:"context,omitempty"`
	VHost             string                `yaml:"vhost,omitempty"`
	Template          string                `yaml:"template"`
	Upstream          LLMUpstreamYAML       `yaml:"upstream"`
	AccessControl     api.LLMAccessControl  `yaml:"accessControl"`
	GlobalPolicies    []api.Policy          `yaml:"globalPolicies,omitempty"`
	OperationPolicies []api.OperationPolicy `yaml:"operationPolicies,omitempty"`
	Policies          []api.LLMPolicy       `yaml:"policies,omitempty"`
}

// LLMUpstreamYAML represents the upstream configuration for LLM provider deployments
type LLMUpstreamYAML struct {
	URL         string            `yaml:"url,omitempty"`
	Ref         string            `yaml:"ref,omitempty"`
	HostRewrite *string           `yaml:"hostRewrite,omitempty"`
	Auth        *api.UpstreamAuth `yaml:"auth,omitempty"`
}

// LLMProxyDeploymentYAML represents the LLM proxy deployment YAML structure
// This format aligns with the gateway controller LLMProxyConfiguration schema.
type LLMProxyDeploymentYAML struct {
	ApiVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   DeploymentMetadata     `yaml:"metadata"`
	Spec       LLMProxyDeploymentSpec `yaml:"spec"`
}

// LLMProxyDeploymentSpec represents the spec section for LLM proxy deployments
type LLMProxyDeploymentSpec struct {
	DisplayName         string                                 `yaml:"displayName"`
	Version             string                                 `yaml:"version"`
	Context             string                                 `yaml:"context,omitempty"`
	VHost               string                                 `yaml:"vhost,omitempty"`
	Provider            LLMProxyDeploymentProvider             `yaml:"provider"`
	AdditionalProviders []LLMProxyDeploymentAdditionalProvider `yaml:"additionalProviders,omitempty"`
	GlobalPolicies      []api.Policy                           `yaml:"globalPolicies,omitempty"`
	OperationPolicies   []api.OperationPolicy                  `yaml:"operationPolicies,omitempty"`
	Policies            []api.LLMPolicy                        `yaml:"policies,omitempty"`
}

type LLMProxyDeploymentProvider struct {
	ID   string            `yaml:"id"`
	Auth *api.UpstreamAuth `yaml:"auth,omitempty"`
}

type LLMProxyDeploymentAdditionalProvider struct {
	ID string `yaml:"id"`
	As string `yaml:"as,omitempty"`
}
