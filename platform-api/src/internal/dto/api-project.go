/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// ImportAPIProjectRequest represents the request payload for importing an API project from Git
type ImportAPIProjectRequest struct {
	RepoURL  string `json:"repoUrl" binding:"required"`
	Provider string `json:"provider,omitempty"` // Optional: "github", "gitlab", "bitbucket", etc.
	Branch   string `json:"branch" binding:"required"`
	Path     string `json:"path" binding:"required"`
	API      API    `json:"api" binding:"required"`
}

// APIProjectConfig represents the structure of config.yaml file in .api-platform directory
type APIProjectConfig struct {
	Version          string            `yaml:"version"`
	APIs             []APIConfigEntry  `yaml:"apis"`
	SpectralRulesets []SpectralRuleset `yaml:"spectralRulesets,omitempty"`
}

// APIConfigEntry represents an API entry in the config.yaml
type APIConfigEntry struct {
	OpenAPI       string `yaml:"openapi"`
	WSO2Artifact  string `yaml:"wso2Artifact"`
	Documentation string `yaml:"documentation,omitempty"`
	Tests         string `yaml:"tests,omitempty"`
}

// SpectralRuleset represents a spectral ruleset configuration
type SpectralRuleset struct {
	Name               string `yaml:"name"`
	SourceFolder       string `yaml:"sourceFolder"`
	FileName           string `yaml:"fileName"`
	RulesetContentPath string `yaml:"rulesetContentPath"`
}
