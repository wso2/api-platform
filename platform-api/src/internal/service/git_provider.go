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
	"net/url"
	"platform-api/src/internal/dto"
	"regexp"
	"strings"
)

// GitProvider represents supported Git providers
type GitProvider string

const (
	GitProviderGitHub    GitProvider = "github"
	GitProviderGitLab    GitProvider = "gitlab"
	GitProviderBitbucket GitProvider = "bitbucket"
)

// GitProviderClient interface that all Git provider clients must implement
type GitProviderClient interface {
	FetchRepoBranches(owner, repo string) (*dto.GitRepoBranchesResponse, error)
	FetchRepoContent(owner, repo, branch string) (*dto.GitRepoContentResponse, error)
	FetchFileContent(owner, repo, branch, path string) ([]byte, error)
	ParseRepoURL(repoURL string) (owner, repo string, err error)
	ValidateName(name string) bool
	GetProvider() GitProvider
}

// GitRepositoryInfo holds parsed repository information
type GitRepositoryInfo struct {
	Provider GitProvider
	Owner    string
	Repo     string
	URL      string
}

// DetectGitProvider detects the Git provider from a repository URL
func DetectGitProvider(repoURL string) (GitProvider, error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format")
	}

	host := strings.ToLower(parsedURL.Host)
	switch host {
	case "github.com", "www.github.com":
		return GitProviderGitHub, nil
	case "gitlab.com", "www.gitlab.com":
		return GitProviderGitLab, nil
	case "bitbucket.org", "www.bitbucket.org":
		return GitProviderBitbucket, nil
	default:
		// Check for self-hosted GitLab instances (common pattern)
		if strings.Contains(host, "gitlab") {
			return GitProviderGitLab, nil
		}
		return "", fmt.Errorf("unsupported Git provider: %s", host)
	}
}

// ParseRepositoryURL parses a Git repository URL and extracts provider, owner, and repo
func ParseRepositoryURL(repoURL string) (*GitRepositoryInfo, error) {
	provider, err := DetectGitProvider(repoURL)
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format")
	}

	// Extract path components
	path := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repository URL format")
	}

	owner := parts[0]
	repo := parts[1]

	// Remove .git suffix if present
	repo = strings.TrimSuffix(repo, ".git")

	// Basic validation
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid repository owner or name")
	}

	return &GitRepositoryInfo{
		Provider: provider,
		Owner:    owner,
		Repo:     repo,
		URL:      repoURL,
	}, nil
}

// isValidName validates repository/username format for most Git providers
func isValidName(name string) bool {
	if len(name) == 0 || len(name) > 100 {
		return false
	}

	// Most Git providers allow alphanumeric characters, hyphens, underscores, and dots
	// Cannot start or end with special characters
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
	return validPattern.MatchString(name)
}
