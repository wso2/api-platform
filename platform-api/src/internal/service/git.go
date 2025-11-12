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
	"platform-api/src/internal/dto"
)

type GitService interface {
	FetchRepoBranches(repoURL string) (*dto.GitRepoBranchesResponse, error)
	FetchRepoContent(repoURL, branch string) (*dto.GitRepoContentResponse, error)
	GetSupportedProviders() []string
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
