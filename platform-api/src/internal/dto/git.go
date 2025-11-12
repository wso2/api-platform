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

// GitRepoBranchesRequest represents the request payload for fetching Git repository branches
type GitRepoBranchesRequest struct {
	RepoURL  string `json:"repoUrl" binding:"required"`
	Provider string `json:"provider,omitempty"` // Optional: "github", "gitlab", "bitbucket", etc. Will be auto-detected if not provided
}

// GitRepoBranch represents a branch in a Git repository
type GitRepoBranch struct {
	Name      string `json:"name"`
	IsDefault string `json:"isDefault"` // if this is the default branch
}

// GitRepoBranchesResponse represents the response for Git repository branches
type GitRepoBranchesResponse struct {
	RepoURL  string          `json:"repoUrl"`
	Branches []GitRepoBranch `json:"branches"`
}

// GitRepoContentRequest represents the request payload for fetching Git repository content
type GitRepoContentRequest struct {
	RepoURL  string `json:"repoUrl" binding:"required"`
	Branch   string `json:"branch" binding:"required"`
	Provider string `json:"provider,omitempty"` // Optional: "github", "gitlab", "bitbucket", etc. Will be auto-detected if not provided
}

// GitRepoItem represents a file or directory in a Git repository with hierarchical structure
type GitRepoItem struct {
	Path     string         `json:"path"`     // Full path of the item within the repository
	SubPath  string         `json:"subPath"`  // Name of the item (file or directory)
	Children []*GitRepoItem `json:"children"` // Child items (for directories)
	Type     string         `json:"type"`     // "tree", "blob", or "EOF"
}

// GitRepoContentResponse represents the response for Git repository content
type GitRepoContentResponse struct {
	RepoURL        string        `json:"repoUrl"`
	Branch         string        `json:"branch"`
	Items          []GitRepoItem `json:"items"`
	TotalItems     int           `json:"totalItems"`
	MaxDepth       int           `json:"maxDepth"`
	RequestedDepth int           `json:"requestedDepth"`
}

// GitRepoError represents error responses for Git operations
type GitRepoError struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
