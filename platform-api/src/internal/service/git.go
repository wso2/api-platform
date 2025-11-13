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

package service

import (
	"fmt"
	"gopkg.in/yaml.v3"
	pathpkg "path"
	"platform-api/src/internal/dto"
	"strings"
)

type GitService interface {
	FetchRepoBranches(repoURL string) (*dto.GitRepoBranchesResponse, error)
	FetchRepoContent(repoURL, branch string) (*dto.GitRepoContentResponse, error)
	GetSupportedProviders() []string
	FetchFileContent(repoURL, branch, path string) ([]byte, error)
	ValidateAPIProject(repoURL, branch, path string) (*dto.APIProjectConfig, error)
	FetchWSO2Artifact(repoURL, branch, path string) (*dto.APIDeploymentYAML, error)
}

type gitService struct {
	providers map[GitProvider]GitProviderClient
}

// NewGitService creates a new Git service instance with support for multiple providers
func NewGitService() GitService {
	return &gitService{
		providers: map[GitProvider]GitProviderClient{
			GitProviderGitHub:    NewGitHubClient(),
			GitProviderGitLab:    NewGitLabClient(),
			GitProviderBitbucket: NewBitbucketClient(),
		},
	}
}

// FetchRepoBranches fetches the branches of a public Git repository from any supported provider
func (s *gitService) FetchRepoBranches(repoURL string) (*dto.GitRepoBranchesResponse, error) {
	// Parse repository URL to determine provider and extract owner/repo
	repoInfo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Get the appropriate provider client
	providerClient, exists := s.providers[repoInfo.Provider]
	if !exists {
		return nil, fmt.Errorf("unsupported Git provider: %s", repoInfo.Provider)
	}

	// Use the provider-specific client to fetch branches
	response, err := providerClient.FetchRepoBranches(repoInfo.Owner, repoInfo.Repo)
	if err != nil {
		return nil, err
	}

	// Ensure the response has the original URL
	response.RepoURL = repoURL

	return response, nil
}

// FetchRepoContent fetches the contents of a public Git repository branch from any supported provider
func (s *gitService) FetchRepoContent(repoURL, branch string) (*dto.GitRepoContentResponse, error) {
	// Parse repository URL to determine provider and extract owner/repo
	repoInfo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Get the appropriate provider client
	providerClient, exists := s.providers[repoInfo.Provider]
	if !exists {
		return nil, fmt.Errorf("unsupported Git provider: %s", repoInfo.Provider)
	}

	// Use the provider-specific client to fetch content
	response, err := providerClient.FetchRepoContent(repoInfo.Owner, repoInfo.Repo, branch)
	if err != nil {
		return nil, err
	}

	// Ensure the response has the original URL and branch
	response.RepoURL = repoURL
	response.Branch = branch

	return response, nil
}

// GetSupportedProviders returns a list of supported Git providers
func (s *gitService) GetSupportedProviders() []string {
	providers := make([]string, 0, len(s.providers))
	for provider := range s.providers {
		providers = append(providers, string(provider))
	}
	return providers
}

// FetchFileContent fetches the content of a specific file from a Git repository
func (s *gitService) FetchFileContent(repoURL, branch, path string) ([]byte, error) {
	// Parse repository URL to determine provider and extract owner/repo
	repoInfo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Get the appropriate provider client
	providerClient, exists := s.providers[repoInfo.Provider]
	if !exists {
		return nil, fmt.Errorf("unsupported Git provider: %s", repoInfo.Provider)
	}

	normalizedPath := strings.TrimSpace(path)
	normalizedPath = strings.TrimPrefix(normalizedPath, "/")
	normalizedPath = strings.TrimPrefix(pathpkg.Clean("/"+normalizedPath), "/")

	// Use the provider-specific client to fetch file content
	content, err := providerClient.FetchFileContent(repoInfo.Owner, repoInfo.Repo, branch, normalizedPath)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// ValidateAPIProject validates an API project structure in a Git repository
func (s *gitService) ValidateAPIProject(repoURL, branch, path string) (*dto.APIProjectConfig, error) {
	// 1. Check if .api-platform directory exists
	apiPlatformPath := path + "/.api-platform"
	configContent, err := s.FetchFileContent(repoURL, branch, apiPlatformPath+"/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("api project not found: .api-platform directory or config.yaml not found")
	}

	// 2. Parse config.yaml
	var config dto.APIProjectConfig
	if err := yaml.Unmarshal(configContent, &config); err != nil {
		return nil, fmt.Errorf("malformed api project: invalid config.yaml format")
	}

	// 3. Validate config structure
	if len(config.APIs) == 0 {
		return nil, fmt.Errorf("malformed api project: no APIs defined in config.yaml")
	}

	for _, api := range config.APIs {
		if api.OpenAPI == "" || api.WSO2Artifact == "" {
			return nil, fmt.Errorf("malformed api project: apis.openapi and apis.wso2Artifact fields are required")
		}

		// 4. Check if the referenced files exist in the project path
		openAPIPath := path + "/" + api.OpenAPI
		_, err := s.FetchFileContent(repoURL, branch, openAPIPath)
		if err != nil {
			return nil, fmt.Errorf("invalid api project: openapi file not found: %s", api.OpenAPI)
		}

		wso2ArtifactPath := path + "/" + api.WSO2Artifact
		_, err = s.FetchFileContent(repoURL, branch, wso2ArtifactPath)
		if err != nil {
			return nil, fmt.Errorf("invalid api project: wso2 artifact file not found: %s", api.WSO2Artifact)
		}
	}

	return &config, nil
}

// FetchWSO2Artifact fetches and parses the WSO2 artifact file from a Git repository
func (s *gitService) FetchWSO2Artifact(repoURL, branch, path string) (*dto.APIDeploymentYAML, error) {
	content, err := s.FetchFileContent(repoURL, branch, path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch WSO2 artifact file: %w", err)
	}

	var artifact dto.APIDeploymentYAML
	if err := yaml.Unmarshal(content, &artifact); err != nil {
		return nil, fmt.Errorf("failed to parse WSO2 artifact file: %w", err)
	}

	return &artifact, nil
}
