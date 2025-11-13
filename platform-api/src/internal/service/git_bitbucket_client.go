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
	"io"
	"net/http"
	"platform-api/src/internal/dto"
	"regexp"
	"time"
)

type BitbucketClient struct {
	httpClient *http.Client
}

// NewBitbucketClient creates a new Bitbucket client
func NewBitbucketClient() GitProviderClient {
	return &BitbucketClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProvider returns the provider name
func (c *BitbucketClient) GetProvider() GitProvider {
	return GitProviderBitbucket
}

// FetchRepoBranches fetches the branches of a Bitbucket repository
func (c *BitbucketClient) FetchRepoBranches(owner, repo string) (*dto.GitRepoBranchesResponse, error) {
	// Use Bitbucket API v2 to fetch repository branches
	apiURL := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s/refs/branches", owner, repo)

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

	// Parse Bitbucket API response
	var bitbucketResponse struct {
		Values []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&bitbucketResponse); err != nil {
		return nil, fmt.Errorf("failed to parse repository branches: %w", err)
	}

	// Get default branch info
	defaultBranch, err := c.getDefaultBranch(owner, repo)
	if err != nil {
		// If we can't get default branch, assume "main" or first branch
		defaultBranch = "main"
		if len(bitbucketResponse.Values) > 0 {
			defaultBranch = bitbucketResponse.Values[0].Name
		}
	}

	// Convert Bitbucket API response to our DTO format
	branches := make([]dto.GitRepoBranch, 0, len(bitbucketResponse.Values))
	for _, branch := range bitbucketResponse.Values {
		isDefault := "false"
		if branch.Name == defaultBranch {
			isDefault = "true"
		}

		branches = append(branches, dto.GitRepoBranch{
			Name:      branch.Name,
			IsDefault: isDefault,
		})
	}

	repoURL := fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
	response := &dto.GitRepoBranchesResponse{
		RepoURL:  repoURL,
		Branches: branches,
	}

	return response, nil
}

// FetchFileContent fetches the content of a specific file from a Bitbucket repository
func (c *BitbucketClient) FetchFileContent(owner, repo, branch, path string) ([]byte, error) {
	apiURL := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s/src/%s/%s", owner, repo, branch, path)

	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file content: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Continue processing
	case http.StatusNotFound:
		return nil, fmt.Errorf("file not found: %s", path)
	case http.StatusForbidden:
		return nil, fmt.Errorf("access forbidden - repository may be private or rate limit exceeded")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized access - repository may be private")
	default:
		return nil, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return content, nil
}

// FetchRepoContent fetches the contents of a Bitbucket repository branch
func (c *BitbucketClient) FetchRepoContent(owner, repo, branch string) (*dto.GitRepoContentResponse, error) {
	// Build tree structure recursively
	rootItems, totalItems, maxDepth, err := c.buildTree(owner, repo, branch, "", 0)
	if err != nil {
		return nil, err
	}

	repoURL := fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
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

// buildTree recursively builds the tree structure for Bitbucket repository content
func (c *BitbucketClient) buildTree(owner, repo, branch, path string, currentDepth int) ([]dto.GitRepoItem, int, int, error) {
	// Build API URL for the current path
	var apiURL string
	if path == "" {
		apiURL = fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s/src/%s", owner, repo, branch)
	} else {
		apiURL = fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s/src/%s/%s", owner, repo, branch, path)
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

	// Parse Bitbucket API response
	var bitbucketResponse struct {
		Values []struct {
			Type string `json:"type"`
			Path string `json:"path"`
			Name string `json:"name"`
			Size int64  `json:"size,omitempty"`
		} `json:"values"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&bitbucketResponse); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to parse repository content: %w", err)
	}

	var items []dto.GitRepoItem
	totalItems := 0
	maxDepth := currentDepth

	// Process items at current level
	for _, item := range bitbucketResponse.Values {
		var children []*dto.GitRepoItem
		var itemType string

		switch item.Type {
		case "commit_file":
			itemType = "blob"
			totalItems++
		case "commit_directory":
			itemType = "tree"
			totalItems++

			// Recursively fetch directory contents
			childItems, childCount, childMaxDepth, err := c.buildTree(owner, repo, branch, item.Path, currentDepth+1)
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

// getDefaultBranch gets the default branch of a Bitbucket repository
func (c *BitbucketClient) getDefaultBranch(owner, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s/%s", owner, repo)

	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get repository info")
	}

	var repoInfo struct {
		Mainbranch struct {
			Name string `json:"name"`
		} `json:"mainbranch"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return "", err
	}

	return repoInfo.Mainbranch.Name, nil
}

// ParseRepoURL parses a Bitbucket repository URL and extracts owner and repository name
func (c *BitbucketClient) ParseRepoURL(repoURL string) (owner, repo string, err error) {
	repoInfo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return "", "", err
	}

	if repoInfo.Provider != GitProviderBitbucket {
		return "", "", fmt.Errorf("not a Bitbucket repository URL")
	}

	// Additional Bitbucket-specific validation
	if !c.ValidateName(repoInfo.Owner) || !c.ValidateName(repoInfo.Repo) {
		return "", "", fmt.Errorf("invalid Bitbucket repository owner or name")
	}

	return repoInfo.Owner, repoInfo.Repo, nil
}

// ValidateName validates Bitbucket username/repository name format
func (c *BitbucketClient) ValidateName(name string) bool {
	// Bitbucket usernames and repo names can contain alphanumeric characters, hyphens, underscores
	// They cannot start or end with special characters
	if len(name) == 0 || len(name) > 62 {
		return false
	}

	// Bitbucket naming rules
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)
	return validPattern.MatchString(name)
}
