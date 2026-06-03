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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"platform-api/src/api"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/middleware"
	"platform-api/src/internal/service"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type OrganizationHandler struct {
	orgService *service.OrganizationService
	slogger    *slog.Logger
}

func NewOrganizationHandler(orgService *service.OrganizationService, slogger *slog.Logger) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
		slogger:    slogger,
	}
}

// RegisterOrganization handles POST /api/v1/organizations
func (h *OrganizationHandler) RegisterOrganization(c *gin.Context) {
	var req api.Organization

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Name is required"))
		return
	}
	if req.Region == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request",
			"Region is required"))
		return
	}

	// Auto-generate ID if not provided
	var id string
	if req.Id != nil && *req.Id != (openapi_types.UUID{}) {
		id = utils.OpenAPIUUIDToString(*req.Id)
	} else {
		generated, genErr := utils.GenerateUUID()
		if genErr != nil {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to generate organization ID"))
			return
		}
		id = generated
	}

	// When a bearer token is present, route through RegisterOrganizationForUser so
	// the IDP org claim (organization / organizations) is synced after creation.
	// Without a token this is a system/admin call — IDP sync is skipped.
	authHeader := c.GetHeader("Authorization")
	bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
	userID := subFromBearerToken(authHeader)

	var org *api.Organization
	var err error
	if userID != "" && bearerToken != authHeader {
		currentOrgIDs := orgsFromBearerToken(authHeader)
		org, err = h.orgService.RegisterOrganizationForUser(userID, bearerToken, id, req.Handle, req.Name, req.Region, currentOrgIDs)
	} else {
		org, err = h.orgService.RegisterOrganization(id, req.Handle, req.Name, req.Region, "")
	}
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Organization already exists"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict",
				"Organization with the given ID already exists"))
			return
		}
		h.slogger.Error("Failed to create organization", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to create organization"))
		return
	}

	c.JSON(http.StatusCreated, org)
}

// HeadOrganizationByUuid handles HEAD /api/v1/organizations/{organizationId}
func (h *OrganizationHandler) HeadOrganizationByUuid(c *gin.Context) {
	organizationIdFromContext, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.Status(http.StatusUnauthorized)
		return
	}
	orgID := c.Param("organizationId")

	h.slogger.Debug("Organization from token", "organizationId", organizationIdFromContext)
	// to do: enable this check after finalizing authentication method

	// if orgID != organizationIdFromContext {
	// 	c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
	// 		"Organization ID in token does not match the requested organization ID"))
	// 	return
	// }

	_, err := h.orgService.GetOrganizationByUUID(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

// GetOrganization handles GET /api/v1/organizations
func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	orgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	org, err := h.orgService.GetOrganizationByUUID(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrMultipleOrganizations) {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Data integrity error: multiple organizations found"))
			return
		}
		h.slogger.Error("Failed to get organization", "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get organization"))
		return
	}

	c.JSON(http.StatusOK, org)
}

// GetOrganizationSubscription handles GET /api/v1/organizations/:organizationId/subscription
func (h *OrganizationHandler) GetOrganizationSubscription(c *gin.Context) {
	organizationIdFromContext, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized",
			"Organization claim not found in token"))
		return
	}

	orgID := c.Param("organizationId")
	if orgID != organizationIdFromContext {
		c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden",
			"Organization ID in token does not match the requested organization ID"))
		return
	}

	subscription, err := h.orgService.GetOrganizationSubscription(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found",
				"Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrMultipleOrganizations) {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
				"Data integrity error: multiple organizations found"))
			return
		}
		h.slogger.Error("Failed to get organization subscription", "organizationId", orgID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error",
			"Failed to get organization subscription"))
		return
	}

	c.JSON(http.StatusOK, subscription)
}

// GetUserOrganizations handles GET /api/v1/users/me/organizations.
// Returns all organizations the authenticated user belongs to.
// On first call it auto-seeds a membership from the JWT org claim (migration path).
func (h *OrganizationHandler) GetUserOrganizations(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists || userID == "" {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "User ID not found in token"))
		return
	}

	jwtOrgID, _ := middleware.GetOrganizationFromContext(c)
	jwtOrgIDs, _ := middleware.GetOrganizationsFromContext(c)

	orgs, err := h.orgService.GetUserOrganizations(userID, jwtOrgIDs, jwtOrgID)
	if err != nil {
		h.slogger.Error("Failed to get user organizations", "userID", userID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get organizations"))
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// CreateOrganizationForUser handles POST /api/v1/users/me/organizations.
// Creates a new organization and records the calling user as owner.
func (h *OrganizationHandler) CreateOrganizationForUser(c *gin.Context) {
	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists || userID == "" {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "User ID not found in token"))
		return
	}

	var req api.Organization
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Name is required"))
		return
	}
	if req.Region == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Region is required"))
		return
	}

	// Generate org ID server-side when not supplied by caller.
	var id string
	if req.Id != nil && *req.Id != (openapi_types.UUID{}) {
		id = utils.OpenAPIUUIDToString(*req.Id)
	} else {
		generated, genErr := utils.GenerateUUID()
		if genErr != nil {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to generate organization ID"))
			return
		}
		id = generated
	}

	// Forward the bearer token and current org list for IDP sync.
	bearerToken := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	currentOrgIDs, _ := middleware.GetOrganizationsFromContext(c)

	org, err := h.orgService.RegisterOrganizationForUser(userID, bearerToken, id, req.Handle, req.Name, req.Region, currentOrgIDs)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Organization handle already exists"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Organization with the given ID already exists"))
			return
		}
		h.slogger.Error("Failed to create organization for user", "userID", userID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create organization"))
		return
	}

	c.JSON(http.StatusCreated, org)
}

type addOrgMemberRequest struct {
	UserID string `json:"userId" binding:"required"`
	Role   string `json:"role"`
}

// AddOrgMember handles POST /api/v1/organizations/:organizationId/members.
// Grants an existing user membership in the specified org and syncs their
// IDP org claim so the change is reflected on their next login.
func (h *OrganizationHandler) AddOrgMember(c *gin.Context) {
	callerOrgID, exists := middleware.GetOrganizationFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.NewErrorResponse(401, "Unauthorized", "Organization claim not found in token"))
		return
	}

	orgID := c.Param("organizationId")
	if orgID != callerOrgID {
		c.JSON(http.StatusForbidden, utils.NewErrorResponse(403, "Forbidden", "Organization ID in token does not match the requested organization ID"))
		return
	}

	var req addOrgMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "userId is required"))
		return
	}

	role := req.Role
	if role == "" {
		role = "member"
	}

	adminBearerToken := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if err := h.orgService.AddUserToOrganization(adminBearerToken, req.UserID, orgID, role); err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Organization not found"))
			return
		}
		h.slogger.Error("Failed to add member to organization", "orgID", orgID, "userID", req.UserID, "error", err)
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to add member to organization"))
		return
	}

	c.Status(http.StatusNoContent)
}

// subFromBearerToken parses the `sub` claim from a bearer token without
// signature validation. Returns an empty string on any error.
func subFromBearerToken(authHeader string) string {
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader || tokenStr == "" {
		return ""
	}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}
	if _, _, err := parser.ParseUnverified(tokenStr, claims); err != nil {
		return ""
	}
	sub, _ := claims["sub"].(string)
	return sub
}

// orgsFromBearerToken parses the `organizations` claim from a bearer token
// without signature validation. Handles both space-separated string and
// string-array encodings. Returns nil on any error.
func orgsFromBearerToken(authHeader string) []string {
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader || tokenStr == "" {
		return nil
	}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}
	if _, _, err := parser.ParseUnverified(tokenStr, claims); err != nil {
		return nil
	}
	raw := claims["organizations"]
	switch v := raw.(type) {
	case string:
		return strings.Fields(v)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// RegisterPublicRoutes registers routes that do not require authentication.
// Must be called before auth middleware is applied to the engine.
func (h *OrganizationHandler) RegisterPublicRoutes(r *gin.Engine) {
	r.POST("/api/v1/organizations", h.RegisterOrganization)
}

func (h *OrganizationHandler) RegisterRoutes(r *gin.Engine) {
	orgGroup := r.Group("/api/v1/organizations")
	{
		orgGroup.GET("", h.GetOrganization)
		orgGroup.HEAD("/:organizationId", h.HeadOrganizationByUuid)
		orgGroup.GET("/:organizationId/subscription", h.GetOrganizationSubscription)
		orgGroup.POST("/:organizationId/members", h.AddOrgMember)
	}

	// Multi-org: user-scoped org management
	userGroup := r.Group("/api/v1/users/me")
	{
		userGroup.GET("/organizations", h.GetUserOrganizations)
		userGroup.POST("/organizations", h.CreateOrganizationForUser)
	}
}
