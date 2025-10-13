package handler

import (
	"errors"
	"net/http"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/utils"

	"github.com/gin-gonic/gin"
	"platform-api/src/internal/model"
	"platform-api/src/internal/service"
)

type ProjectHandler struct {
	projectService *service.ProjectService
}

func NewProjectHandler(projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
	}
}

func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var req dto.Project

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	// Validate required fields
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project name is required"))
		return
	}
	if req.OrganizationID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Organization ID is required"))
		return
	}

	project, err := h.projectService.CreateProject(req.Name, req.OrganizationID, req.IsDefault)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNameExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Project name already exists in organization"))
			return
		}
		if errors.Is(err, constants.ErrDefaultProjectAlreadyExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Default project already exists in organization"))
			return
		}
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Organization not found"))
			return
		}
		if errors.Is(err, constants.ErrInvalidProjectName) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project name is required"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to create project"))
		return
	}

	c.JSON(http.StatusCreated, project)
}

func (h *ProjectHandler) GetProject(c *gin.Context) {
	uuid := c.Param("project_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project UUID is required"))
		return
	}

	project, err := h.projectService.GetProjectByID(uuid)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get project"))
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) GetProjectsByOrganization(c *gin.Context) {
	orgID := c.Param("org_uuid")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Organization UUID is required"))
		return
	}

	projects, err := h.projectService.GetProjectsByOrganization(orgID)
	if err != nil {
		if errors.Is(err, constants.ErrOrganizationNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Organization not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to get projects"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	uuid := c.Param("project_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project UUID is required"))
		return
	}

	var req model.Project
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", err.Error()))
		return
	}

	project, err := h.projectService.UpdateProject(uuid, req.Name)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		}
		if errors.Is(err, constants.ErrProjectNameExists) {
			c.JSON(http.StatusConflict, utils.NewErrorResponse(409, "Conflict", "Project name already exists in organization"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to update project"))
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	uuid := c.Param("project_uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Project UUID is required"))
		return
	}

	err := h.projectService.DeleteProject(uuid)
	if err != nil {
		if errors.Is(err, constants.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, utils.NewErrorResponse(404, "Not Found", "Project not found"))
			return
		}
		if errors.Is(err, constants.ErrCannotDeleteDefaultProject) {
			c.JSON(http.StatusBadRequest, utils.NewErrorResponse(400, "Bad Request", "Cannot delete the default project of an organization"))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.NewErrorResponse(500, "Internal Server Error", "Failed to delete project"))
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *ProjectHandler) RegisterRoutes(r *gin.Engine) {
	projectGroup := r.Group("/api/v1/projects")
	{
		projectGroup.POST("", h.CreateProject)
		projectGroup.GET("/:project_uuid", h.GetProject)
		projectGroup.PUT("/:project_uuid", h.UpdateProject)
		projectGroup.DELETE("/:project_uuid", h.DeleteProject)
	}

	// Organization-specific project routes
	orgProjectGroup := r.Group("/api/v1/organizations/:org_uuid/projects")
	{
		orgProjectGroup.GET("", h.GetProjectsByOrganization)
	}
}
