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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"platform-api/src/internal/dto"
	"regexp"
	"time"
)

type GitLabClient struct {
	httpClient *http.Client
}

// NewGitLabClient creates a new GitLab client
func NewGitLabClient() GitProviderClient {
	return &GitLabClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProvider returns the provider name
func (c *GitLabClient) GetProvider() GitProvider {
	return GitProviderGitLab
}

// FetchRepoBranches fetches the branches of a GitLab repository
func (c *GitLabClient) FetchRepoBranches(owner, repo string) (*dto.GitRepoBranchesResponse, error) {
	// URL encode the project path for GitLab API
	projectPath := url.QueryEscape(fmt.Sprintf("%s/%s", owner, repo))

	// Use GitLab API to fetch repository branches
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/branches", projectPath)

	// Make API request
	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository branches: %w", err)
	}
	defer resp.Body.Close()

	// Handle different HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Continue processing
	case http.StatusNotFound:
		return nil, fmt.Errorf("repository not found or is private")
	case http.StatusForbidden:
		return nil, fmt.Errorf("access forbidden - repository may be private or rate limit exceeded")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized access - repository may be private")
	default:
		return nil, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	// Parse GitLab API response
	var gitlabBranches []struct {
		Name      string `json:"name"`
		Default   bool   `json:"default"`
		Protected bool   `json:"protected"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gitlabBranches); err != nil {
		return nil, fmt.Errorf("failed to parse repository branches: %w", err)
	}

	// Convert GitLab API response to our DTO format
	branches := make([]dto.GitRepoBranch, 0, len(gitlabBranches))
	for _, branch := range gitlabBranches {
		isDefault := "false"
		if branch.Default {
			isDefault = "true"
		}

		branches = append(branches, dto.GitRepoBranch{
			Name:      branch.Name,
			IsDefault: isDefault,
		})
	}

	repoURL := fmt.Sprintf("https://gitlab.com/%s/%s", owner, repo)
	response := &dto.GitRepoBranchesResponse{
		RepoURL:  repoURL,
		Branches: branches,
	}

	return response, nil
}

// FetchRepoContent fetches the contents of a GitLab repository branch
func (c *GitLabClient) FetchRepoContent(owner, repo, branch string) (*dto.GitRepoContentResponse, error) {
	// URL encode the project path for GitLab API
	projectPath := url.QueryEscape(fmt.Sprintf("%s/%s", owner, repo))

	// Build tree structure recursively
	rootItems, totalItems, maxDepth, err := c.buildTree(projectPath, branch, "", 0)
	if err != nil {
		return nil, err
	}

	repoURL := fmt.Sprintf("https://gitlab.com/%s/%s", owner, repo)
	response := &dto.GitRepoContentResponse{
		RepoURL:        repoURL,
		Branch:         branch,
		Items:          rootItems,
		TotalItems:     totalItems,
		MaxDepth:       maxDepth,
		RequestedDepth: 0, // No depth limit for this implementation
	}

	return response, nil
}

// buildTree recursively builds the tree structure for GitLab repository content
func (c *GitLabClient) buildTree(projectPath, branch, path string, currentDepth int) ([]dto.GitRepoItem, int, int, error) {
	// Build API URL for the current path
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/tree?ref=%s&recursive=false", projectPath, branch)
	if path != "" {
		apiURL += fmt.Sprintf("&path=%s", url.QueryEscape(path))
	}

	// Make API request
	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch repository content: %w", err)
	}
	defer resp.Body.Close()

	// Handle different HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Continue processing
	case http.StatusNotFound:
		return nil, 0, 0, fmt.Errorf("repository not found or is private")
	case http.StatusForbidden:
		return nil, 0, 0, fmt.Errorf("access forbidden - repository may be private or rate limit exceeded")
	case http.StatusUnauthorized:
		return nil, 0, 0, fmt.Errorf("unauthorized access - repository may be private")
	default:
		return nil, 0, 0, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	// Parse GitLab API response
	var gitlabItems []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
		Mode string `json:"mode"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gitlabItems); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to parse repository content: %w", err)
	}

	var items []dto.GitRepoItem
	totalItems := 0
	maxDepth := currentDepth

	// Process items at current level
	for _, item := range gitlabItems {
		var children []*dto.GitRepoItem
		var itemType string

		switch item.Type {
		case "blob":
			itemType = "blob"
			totalItems++
		case "tree":
			itemType = "tree"
			totalItems++

			// Recursively fetch directory contents
			childItems, childCount, childMaxDepth, err := c.buildTree(projectPath, branch, item.Path, currentDepth+1)
			if err != nil {
				// Log error but continue processing
				fmt.Printf("Warning: failed to fetch directory %s: %v\n", item.Path, err)
				children = []*dto.GitRepoItem{} // Empty children array
			} else {
				// Convert child items to pointers
				children = make([]*dto.GitRepoItem, len(childItems))
				for i := range childItems {
					children[i] = &childItems[i]
				}
				totalItems += childCount
				if childMaxDepth > maxDepth {
					maxDepth = childMaxDepth
				}
			}
		default:
			// Skip unsupported types
			continue
		}

		gitItem := dto.GitRepoItem{
			Path:     item.Path,
			SubPath:  item.Name,
			Children: children,
			Type:     itemType,
		}

		items = append(items, gitItem)
	}

	return items, totalItems, maxDepth, nil
}

// ParseRepoURL parses a GitLab repository URL and extracts owner and repository name
func (c *GitLabClient) ParseRepoURL(repoURL string) (owner, repo string, err error) {
	repoInfo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return "", "", err
	}

	if repoInfo.Provider != GitProviderGitLab {
		return "", "", fmt.Errorf("not a GitLab repository URL")
	}

	// Additional GitLab-specific validation
	if !c.ValidateName(repoInfo.Owner) || !c.ValidateName(repoInfo.Repo) {
		return "", "", fmt.Errorf("invalid GitLab repository owner or name")
	}

	return repoInfo.Owner, repoInfo.Repo, nil
}

// ValidateName validates GitLab username/repository name format
func (c *GitLabClient) ValidateName(name string) bool {
	// GitLab usernames and repo names can contain alphanumeric characters, hyphens, underscores, and dots
	// They cannot start or end with special characters
	if len(name) == 0 || len(name) > 255 {
		return false
	}

	// GitLab allows more characters than GitHub
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
	return validPattern.MatchString(name)
}

// FetchFileContent fetches the content of a specific file from a GitLab repository
// TODO: Implement proper GitLab file content fetching
func (c *GitLabClient) FetchFileContent(owner, repo, branch, path string) ([]byte, error) {
	return nil, fmt.Errorf("FetchFileContent not implemented for GitLab client - only GitHub is currently supported")
}
