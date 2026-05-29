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

package handler

import (
	"log/slog"
	"net/http"
	"platform-api/src/api"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"strings"

	"github.com/gin-gonic/gin"
)

type GitHandler struct {
	gitService service.GitService
	slogger    *slog.Logger
}

// NewGitHandler creates a new Git handler instance
func NewGitHandler(gitService service.GitService, slogger *slog.Logger) *GitHandler {
	return &GitHandler{
		gitService: gitService,
		slogger:    slogger,
	}
}

// FetchRepoBranches handles POST /git/repo/fetch-branches
func (h *GitHandler) FetchRepoBranches(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		errorResponse := api.GitRepoError{
			Error:   "UNAUTHORIZED",
			Code:    "GIT_401",
			Message: "Organization claim not found in token",
		}
		c.JSON(http.StatusUnauthorized, errorResponse)
		return
	}

	var request api.GitRepoBranchesRequest

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&request); err != nil {
		errorResponse := api.GitRepoError{
			Error:   "INVALID_REQUEST",
			Code:    "GIT_001",
			Message: "Invalid request payload: " + err.Error(),
		}
		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// Additional validation for empty repository URL
	if strings.TrimSpace(request.RepoUrl) == "" {
		errorResponse := api.GitRepoError{
			Error:   "INVALID_REPO_URL",
			Code:    "GIT_002",
			Message: "Repository URL cannot be empty",
		}
		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// Log the request for debugging
	if request.Provider != nil {
		h.slogger.Info("Fetching repository branches",
			"organization", orgId,
			"provider", *request.Provider,
			"repoUrl", request.RepoUrl)
	} else {
		h.slogger.Info("Fetching repository branches (auto-detect provider)",
			"organization", orgId,
			"repoUrl", request.RepoUrl)
	}

	// Fetch repository branches
	repoBranches, err := h.gitService.FetchRepoBranches(request.RepoUrl)
	if err != nil {
		h.slogger.Error("Failed to fetch repository branches", "error", err)

		// Determine appropriate status code and error response based on error type
		var statusCode int
		var errorCode string
		errorMessage := err.Error()

		switch {
		case strings.Contains(errorMessage, "invalid repository URL") ||
			strings.Contains(errorMessage, "invalid URL format") ||
			strings.Contains(errorMessage, "unsupported Git provider") ||
			strings.Contains(errorMessage, "invalid repository URL format") ||
			strings.Contains(errorMessage, "invalid repository owner or name"):
			statusCode = http.StatusBadRequest
			errorCode = "GIT_002"
		case strings.Contains(errorMessage, "repository not found") ||
			strings.Contains(errorMessage, "access forbidden") ||
			strings.Contains(errorMessage, "unauthorized access"):
			statusCode = http.StatusNotFound
			errorCode = "GIT_003"
		case strings.Contains(errorMessage, "rate limit exceeded"):
			statusCode = http.StatusTooManyRequests
			errorCode = "GIT_004"
		case strings.Contains(errorMessage, "failed to fetch repository branches"):
			statusCode = http.StatusBadGateway
			errorCode = "GIT_005"
		default:
			statusCode = http.StatusInternalServerError
			errorCode = "GIT_999"
		}

		errorResponse := api.GitRepoError{
			Error:   "FETCH_FAILED",
			Code:    errorCode,
			Message: errorMessage,
		}
		c.JSON(statusCode, errorResponse)
		return
	}

	// Return successful response
	c.JSON(http.StatusOK, repoBranches)
}

// FetchRepoContent handles POST /git/repo/branch/fetch-content
func (h *GitHandler) FetchRepoContent(c *gin.Context) {
	// Extract organization from JWT token
	orgId, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		errorResponse := api.GitRepoError{
			Error:   "UNAUTHORIZED",
			Code:    "GIT_401",
			Message: "Organization claim not found in token",
		}
		c.JSON(http.StatusUnauthorized, errorResponse)
		return
	}

	var request api.GitRepoContentRequest

	// Bind and validate request payload
	if err := c.ShouldBindJSON(&request); err != nil {
		errorResponse := api.GitRepoError{
			Error:   "INVALID_REQUEST",
			Code:    "GIT_001",
			Message: "Invalid request payload: " + err.Error(),
		}
		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// Additional validation for empty repository URL
	if strings.TrimSpace(request.RepoUrl) == "" {
		errorResponse := api.GitRepoError{
			Error:   "INVALID_REPO_URL",
			Code:    "GIT_002",
			Message: "Repository URL cannot be empty",
		}
		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// Additional validation for empty branch
	if strings.TrimSpace(request.Branch) == "" {
		errorResponse := api.GitRepoError{
			Error:   "INVALID_BRANCH",
			Code:    "GIT_007",
			Message: "Branch name cannot be empty",
		}
		c.JSON(http.StatusBadRequest, errorResponse)
		return
	}

	// Log the request for debugging
	if request.Provider != nil {
		h.slogger.Info("Fetching repository content",
			"organization", orgId,
			"provider", *request.Provider,
			"repoUrl", request.RepoUrl,
			"branch", request.Branch)
	} else {
		h.slogger.Info("Fetching repository content (auto-detect provider)",
			"organization", orgId,
			"repoUrl", request.RepoUrl,
			"branch", request.Branch)
	}

	// Fetch repository content
	repoContent, err := h.gitService.FetchRepoContent(request.RepoUrl, request.Branch)
	if err != nil {
		h.slogger.Error("Failed to fetch repository content", "error", err)

		// Determine appropriate status code and error response based on error type
		var statusCode int
		var errorCode string
		errorMessage := err.Error()

		switch {
		case strings.Contains(errorMessage, "invalid repository URL") ||
			strings.Contains(errorMessage, "invalid URL format") ||
			strings.Contains(errorMessage, "unsupported Git provider") ||
			strings.Contains(errorMessage, "invalid repository URL format") ||
			strings.Contains(errorMessage, "invalid repository owner or name"):
			statusCode = http.StatusBadRequest
			errorCode = "GIT_002"
		case strings.Contains(errorMessage, "repository not found") ||
			strings.Contains(errorMessage, "access forbidden") ||
			strings.Contains(errorMessage, "unauthorized access"):
			statusCode = http.StatusNotFound
			errorCode = "GIT_003"
		case strings.Contains(errorMessage, "rate limit exceeded"):
			statusCode = http.StatusTooManyRequests
			errorCode = "GIT_004"
		case strings.Contains(errorMessage, "failed to fetch repository content"):
			statusCode = http.StatusBadGateway
			errorCode = "GIT_005"
		default:
			statusCode = http.StatusInternalServerError
			errorCode = "GIT_999"
		}

		errorResponse := api.GitRepoError{
			Error:   "FETCH_FAILED",
			Code:    errorCode,
			Message: errorMessage,
		}
		c.JSON(statusCode, errorResponse)
		return
	}

	// Return successful response
	c.JSON(http.StatusOK, repoContent)
}

// RegisterRoutes registers Git-related routes
func (h *GitHandler) RegisterRoutes(router *gin.Engine) {
	gitRoutes := router.Group("/api/v1/git")
	{
		gitRoutes.POST("/repo/fetch-branches", h.FetchRepoBranches)
		gitRoutes.POST("/repo/branch/fetch-content", h.FetchRepoContent)
	}
}
