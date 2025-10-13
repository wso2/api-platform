package handler

import (
	"errors"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
	"platform-api/src/internal/service"
)

type OrganizationHandler struct {
	orgService *service.OrganizationService
}

func NewOrganizationHandler(orgService *service.OrganizationService) *OrganizationHandler {
	return &OrganizationHandler{
		orgService: orgService,
	}
}

func (h *OrganizationHandler) CreateOrganization(c *gin.Context) {
	var req dto.Organization

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.Handle == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Handle is required"))
		return
	}

	org, err := h.orgService.CreateOrganization(req.Handle, req.Name)
	if err != nil {
		if errors.Is(err, constants.ErrHandleExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Organization handle already exists"))
			return
		}
		if errors.Is(err, constants.ErrInvalidHandle) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Organization handle must be URL friendly"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create organization"))
		return
	}

	c.JSON(http.StatusCreated, org)
}

func (h *OrganizationHandler) GetOrganization(c *gin.Context) {
	uuid := c.Param("org_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Organization UUID is required"))
		return
	}

	org, err := h.orgService.GetOrganizationByUUID(uuid)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrMultipleOrganizations) {
			c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Data integrity error: multiple organizations found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get organization"))
		return
	}

	c.JSON(http.StatusOK, org)
}

func (h *OrganizationHandler) RegisterRoutes(r *gin.Engine) {
	orgGroup := r.Group("/api/v1/organizations")
	{
		orgGroup.POST("", h.CreateOrganization)
		orgGroup.GET("/:org_uuid", h.GetOrganization)
	}
}
