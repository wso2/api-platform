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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"platform-api/src/internal/dto"
	"regexp"
	"sort"
	"strings"
	"time"
)

type GitHubClient struct {
	httpClient *http.Client
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient() GitProviderClient {
	return &GitHubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GitTreeEntry represents a single entry in GitHub's Git Trees API response
type GitTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size,omitempty"`
	URL  string `json:"url"`
}

// GitTreeResponse represents the complete GitHub Git Trees API response
type GitTreeResponse struct {
	SHA       string         `json:"sha"`
	URL       string         `json:"url"`
	Tree      []GitTreeEntry `json:"tree"`
	Truncated bool           `json:"truncated"`
}

// GetProvider returns the provider name
func (c *GitHubClient) GetProvider() GitProvider {
	return GitProviderGitHub
}

// FetchRepoBranches fetches the branches of a GitHub repository
func (c *GitHubClient) FetchRepoBranches(owner, repo string) (*dto.GitRepoBranchesResponse, error) {
	// Start with initial request URL without hardcoded page
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", owner, repo)

	var allBranches []struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}

	// Loop through all pages using GitHub pagination
	for apiURL != "" {
		// Make API request
		resp, err := c.httpClient.Get(apiURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch repository branches: %w", err)
		}

		// Handle different HTTP status codes
		switch resp.StatusCode {
		case http.StatusOK:
			// Continue processing
		case http.StatusNotFound:
			resp.Body.Close()
			return nil, fmt.Errorf("repository not found or is private")
		case http.StatusForbidden:
			resp.Body.Close()
			return nil, fmt.Errorf("access forbidden - repository may be private or rate limit exceeded")
		case http.StatusUnauthorized:
			resp.Body.Close()
			return nil, fmt.Errorf("unauthorized access - repository may be private")
		default:
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
		}

		// Parse current page's branches
		var pageBranches []struct {
			Name      string `json:"name"`
			Protected bool   `json:"protected"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&pageBranches); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse repository branches: %w", err)
		}

		// Append current page's branches to the complete list
		allBranches = append(allBranches, pageBranches...)

		// Extract next page URL from Link header
		linkHeader := resp.Header.Get("link")
		nextURL := c.extractNextLink(linkHeader)

		// Close response body before next iteration
		resp.Body.Close()

		// Set next URL or empty to stop the loop
		apiURL = nextURL
	}

	// Get default branch info
	defaultBranch, err := c.getDefaultBranch(owner, repo)
	if err != nil {
		// If we can't get default branch, assume "main" or first branch
		defaultBranch = "main"
		if len(allBranches) > 0 {
			defaultBranch = allBranches[0].Name
		}
	}

	// Convert GitHub API response to our DTO format
	branches := make([]dto.GitRepoBranch, 0, len(allBranches))
	for _, branch := range allBranches {
		isDefault := "false"
		if branch.Name == defaultBranch {
			isDefault = "true"
		}

		branches = append(branches, dto.GitRepoBranch{
			Name:      branch.Name,
			IsDefault: isDefault,
		})
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	response := &dto.GitRepoBranchesResponse{
		RepoURL:  repoURL,
		Branches: branches,
	}

	return response, nil
}

// FetchRepoContent fetches the contents of a GitHub repository branch using Git Trees API
func (c *GitHubClient) FetchRepoContent(owner, repo, branch string) (*dto.GitRepoContentResponse, error) {
	// Use GitHub Git Trees API to get all content in one request
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", owner, repo, branch)

	// Make API request
	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository content: %w", err)
	}
	defer resp.Body.Close()

	// Handle different HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Continue processing
	case http.StatusNotFound:
		return nil, fmt.Errorf("repository not found, branch not found, or repository is private")
	case http.StatusForbidden:
		return nil, fmt.Errorf("access forbidden - repository may be private or rate limit exceeded")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("unauthorized access - repository may be private")
	default:
		return nil, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	// Parse GitHub Git Trees API response
	var treeResponse GitTreeResponse

	if err := json.NewDecoder(resp.Body).Decode(&treeResponse); err != nil {
		return nil, fmt.Errorf("failed to parse repository content: %w", err)
	}

	if treeResponse.Truncated {
		return nil, fmt.Errorf("GitHub returned a truncated tree for %s/%s@%s; fetch the tree non-recursively"+
			" to avoid data loss", owner, repo, branch)
	}

	// Build tree structure from the flat list of paths
	rootItems, totalItems, maxDepth := c.buildTreeFromPaths(treeResponse.Tree)

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
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

// FetchFileContent fetches the content of a specific file from a GitHub repository
func (c *GitHubClient) FetchFileContent(owner, repo, branch, path string) ([]byte, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)

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

	var fileResponse struct {
		Content     string `json:"content"`
		Encoding    string `json:"encoding"`
		DownloadURL string `json:"download_url"`
		Size        int    `json:"size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fileResponse); err != nil {
		return nil, fmt.Errorf("failed to parse file content response: %w", err)
	}

	// If content is available and base64 encoded, decode it
	if fileResponse.Content != "" && fileResponse.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(fileResponse.Content, "\n", ""))
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %w", err)
		}
		return decoded, nil
	}

	// If content is not available (large files), use download_url
	if fileResponse.DownloadURL != "" {
		downloadResp, err := c.httpClient.Get(fileResponse.DownloadURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download file from raw URL: %w", err)
		}
		defer downloadResp.Body.Close()

		if downloadResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download file, status: %d", downloadResp.StatusCode)
		}

		content, err := io.ReadAll(downloadResp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read downloaded content: %w", err)
		}
		return content, nil
	}

	// If neither content nor download_url is available
	return nil, fmt.Errorf("file content not available in GitHub API response")
}

// extractNextLink parses the GitHub Link header to find the next page URL
func (c *GitHubClient) extractNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	// GitHub Link header format: <https://api.github.com/repos/.../branches?page=2>; rel="next", <https://...>; rel="last"
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		parts := strings.Split(strings.TrimSpace(link), ";")
		if len(parts) != 2 {
			continue
		}

		url := strings.Trim(strings.TrimSpace(parts[0]), "<>")
		rel := strings.TrimSpace(parts[1])

		if strings.Contains(rel, `rel="next"`) {
			return url
		}
	}

	return ""
}

// buildTreeFromPaths builds a hierarchical tree structure from a flat list of Git tree entries
func (c *GitHubClient) buildTreeFromPaths(treeEntries []GitTreeEntry) ([]dto.GitRepoItem, int, int) {
	// Create a map to store all items by their path for easy lookup
	itemMap := make(map[string]*dto.GitRepoItem)
	totalItems := len(treeEntries)
	maxDepth := 0

	// First pass: create all items and store in map
	for _, entry := range treeEntries {
		// Calculate depth from path separators
		depth := len(strings.Split(entry.Path, "/")) - 1
		if depth > maxDepth {
			maxDepth = depth
		}

		pathParts := strings.Split(entry.Path, "/")
		itemName := pathParts[len(pathParts)-1]

		// Create the item
		item := &dto.GitRepoItem{
			Path:     entry.Path,
			SubPath:  itemName,
			Type:     entry.Type, // "blob" or "tree"
			Children: []*dto.GitRepoItem{},
		}

		itemMap[entry.Path] = item
	}

	// Second pass: build parent-child relationships
	for path, item := range itemMap {
		pathParts := strings.Split(path, "/")

		// If this is not a root-level item, find its parent and add it as a child
		if len(pathParts) > 1 {
			parentPath := strings.Join(pathParts[:len(pathParts)-1], "/")
			if parentItem, exists := itemMap[parentPath]; exists {
				parentItem.Children = append(parentItem.Children, item)
			} else {
				// Parent not found - this should not happen in a well-formed tree
				log.Println("Warning: parent item not found for path:", path)
			}
		}
	}

	// Third pass: collect only root-level items (they will contain all their children)
	var rootItems []dto.GitRepoItem
	for path, item := range itemMap {
		pathParts := strings.Split(path, "/")
		if len(pathParts) == 1 {
			// This is a root-level item
			c.sortChildrenRecursively(item)
			rootItems = append(rootItems, *item)
		}
	}

	// Sort root items
	c.sortTreeItems(rootItems)

	return rootItems, totalItems, maxDepth
}

// sortChildrenRecursively sorts children at all levels of the tree
func (c *GitHubClient) sortChildrenRecursively(item *dto.GitRepoItem) {
	// Sort children of current item
	c.sortTreeItemsPointers(item.Children)

	// Recursively sort children of each child
	for _, child := range item.Children {
		c.sortChildrenRecursively(child)
	}
}

// getDefaultBranch gets the default branch of a GitHub repository
func (c *GitHubClient) getDefaultBranch(owner, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get repository info")
	}

	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return "", err
	}

	return repoInfo.DefaultBranch, nil
}

// ParseRepoURL parses a GitHub repository URL and extracts owner and repository name
func (c *GitHubClient) ParseRepoURL(repoURL string) (owner, repo string, err error) {
	repoInfo, err := ParseRepositoryURL(repoURL)
	if err != nil {
		return "", "", err
	}

	if repoInfo.Provider != GitProviderGitHub {
		return "", "", fmt.Errorf("not a GitHub repository URL")
	}

	// Additional GitHub-specific validation
	if !c.ValidateName(repoInfo.Owner) || !c.ValidateName(repoInfo.Repo) {
		return "", "", fmt.Errorf("invalid GitHub repository owner or name")
	}

	return repoInfo.Owner, repoInfo.Repo, nil
}

// ValidateName validates GitHub username/repository name format
func (c *GitHubClient) ValidateName(name string) bool {
	// GitHub username validation rules:
	// - May only contain alphanumeric characters or hyphens
	// - Cannot have multiple consecutive hyphens
	// - Cannot begin or end with a hyphen
	// - Maximum is 39 characters
	if len(name) == 0 || len(name) > 39 {
		return false
	}

	// Use the official GitHub username regex pattern
	// ^[a-z\d](?:[a-z\d]|-(?=[a-z\d])){0,38}$/i
	validPattern := regexp.MustCompile(`^[a-z\d](?:[a-z\d]|-(?=[a-z\d])){0,38}$`)
	return validPattern.MatchString(strings.ToLower(name))
}

// sortTreeItems sorts items with directories first, then files, both alphabetically
func (c *GitHubClient) sortTreeItems(items []dto.GitRepoItem) {
	sort.Slice(items, func(i, j int) bool {
		// Directories come before files
		if items[i].Type == "tree" && items[j].Type == "blob" {
			return true
		}
		if items[i].Type == "blob" && items[j].Type == "tree" {
			return false
		}
		// Within same type, sort alphabetically by name
		return items[i].SubPath < items[j].SubPath
	})
}

// sortTreeItemsPointers sorts pointer items with directories first, then files, both alphabetically
func (c *GitHubClient) sortTreeItemsPointers(items []*dto.GitRepoItem) {
	sort.Slice(items, func(i, j int) bool {
		// Directories come before files
		if items[i].Type == "tree" && items[j].Type == "blob" {
			return true
		}
		if items[i].Type == "blob" && items[j].Type == "tree" {
			return false
		}
		// Within same type, sort alphabetically by name
		return items[i].SubPath < items[j].SubPath
	})
}
