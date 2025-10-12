package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	"go.uber.org/zap"
)

// APIServer implements the generated ServerInterface
type APIServer struct {
	store           *storage.ConfigStore
	db              storage.Storage
	snapshotManager *xds.SnapshotManager
	parser          *config.Parser
	validator       *config.Validator
	logger          *zap.Logger
}

// NewAPIServer creates a new API server with dependencies
func NewAPIServer(
	store *storage.ConfigStore,
	db storage.Storage,
	snapshotManager *xds.SnapshotManager,
	logger *zap.Logger,
) *APIServer {
	server := &APIServer{
		store:           store,
		db:              db,
		snapshotManager: snapshotManager,
		parser:          config.NewParser(),
		validator:       config.NewValidator(),
		logger:          logger,
	}

	// Register status update callback
	snapshotManager.SetStatusCallback(server.handleStatusUpdate)

	return server
}

// handleStatusUpdate is called by SnapshotManager after xDS deployment
func (s *APIServer) handleStatusUpdate(configID string, success bool, version int64) {
	cfg, err := s.store.Get(configID)
	if err != nil {
		s.logger.Warn("Config not found for status update", zap.String("id", configID))
		return
	}

	now := time.Now()
	if success {
		cfg.Status = models.StatusDeployed
		cfg.DeployedAt = &now
		cfg.DeployedVersion = version
		s.logger.Info("API configuration deployed successfully",
			zap.String("id", configID),
			zap.String("name", cfg.Configuration.Data.Name),
			zap.Int64("version", version))
	} else {
		cfg.Status = models.StatusFailed
		cfg.DeployedAt = nil
		cfg.DeployedVersion = 0
		s.logger.Error("API configuration deployment failed",
			zap.String("id", configID),
			zap.String("name", cfg.Configuration.Data.Name))
	}

	cfg.UpdatedAt = now

	// Update database (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(cfg); err != nil {
			s.logger.Error("Failed to update config status in database", zap.Error(err), zap.String("id", configID))
		}
	}

	// Update in-memory store
	if err := s.store.Update(cfg); err != nil {
		s.logger.Error("Failed to update config status in memory", zap.Error(err), zap.String("id", configID))
	}
}

// HealthCheck implements ServerInterface.HealthCheck
// (GET /health)
func (s *APIServer) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// CreateAPI implements ServerInterface.CreateAPI
// (POST /apis)
func (s *APIServer) CreateAPI(c *gin.Context) {
	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Parse configuration
	contentType := c.GetHeader("Content-Type")
	apiConfig, err := s.parser.Parse(body, contentType)
	if err != nil {
		s.logger.Error("Failed to parse configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to parse configuration",
		})
		return
	}

	// Validate configuration
	validationErrors := s.validator.Validate(apiConfig)
	if len(validationErrors) > 0 {
		s.logger.Warn("Configuration validation failed",
			zap.String("name", apiConfig.Data.Name),
			zap.Int("num_errors", len(validationErrors)))

		errors := make([]api.ValidationError, len(validationErrors))
		for i, e := range validationErrors {
			errors[i] = api.ValidationError{
				Field:   stringPtr(e.Field),
				Message: stringPtr(e.Message),
			}
		}

		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Configuration validation failed",
			Errors:  &errors,
		})
		return
	}

	// Create stored configuration
	now := time.Now()
	storedCfg := &models.StoredAPIConfig{
		ID:              uuid.New().String(),
		Configuration:   *apiConfig,
		Status:          models.StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
		DeployedAt:      nil,
		DeployedVersion: 0,
	}

	// Atomic dual-write: database + in-memory
	// Save to database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.SaveConfig(storedCfg); err != nil {
			s.logger.Error("Failed to save config to database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to persist configuration",
			})
			return
		}
	}

	if err := s.store.Add(storedCfg); err != nil {
		// Rollback database write (only if persistent mode)
		if s.db != nil {
			_ = s.db.DeleteConfig(storedCfg.ID)
		}
		s.logger.Error("Failed to add config to memory store", zap.Error(err))
		c.JSON(http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// Log audit event
	auditEvent := storage.CreateAuditEvent(
		storage.AuditCreate,
		storedCfg.ID,
		apiConfig.Data.Name,
		apiConfig.Data.Version,
		"SUCCESS",
		"",
	)
	if bboltDB, ok := s.db.(*storage.BBoltStorage); ok {
		_ = bboltDB.LogEvent(auditEvent)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx); err != nil {
			s.logger.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	s.logger.Info("API configuration created",
		zap.String("id", storedCfg.ID),
		zap.String("name", apiConfig.Data.Name),
		zap.String("version", apiConfig.Data.Version))

	// Return success response
	id, _ := uuidToOpenAPIUUID(storedCfg.ID)
	c.JSON(http.StatusCreated, api.APICreateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("API configuration created successfully"),
		Id:        id,
		CreatedAt: timePtr(storedCfg.CreatedAt),
	})
}

// ListAPIs implements ServerInterface.ListAPIs
// (GET /apis)
func (s *APIServer) ListAPIs(c *gin.Context) {
	configs := s.store.GetAll()

	items := make([]api.APIListItem, len(configs))
	for i, cfg := range configs {
		id, _ := uuidToOpenAPIUUID(cfg.ID)
		status := string(cfg.Status)
		items[i] = api.APIListItem{
			Id:        id,
			Name:      stringPtr(cfg.Configuration.Data.Name),
			Version:   stringPtr(cfg.Configuration.Data.Version),
			Context:   stringPtr(cfg.Configuration.Data.Context),
			Status:    (*api.APIListItemStatus)(&status),
			CreatedAt: timePtr(cfg.CreatedAt),
			UpdatedAt: timePtr(cfg.UpdatedAt),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(items),
		"apis":   items,
	})
}

// GetAPIByID implements ServerInterface.GetAPIByID
// (GET /apis/{id})
func (s *APIServer) GetAPIByID(c *gin.Context, id string) {
	cfg, err := s.store.Get(id)
	if err != nil {
		s.logger.Warn("API configuration not found", zap.String("id", id))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with ID '%s' not found", id),
		})
		return
	}

	apiDetail := gin.H{
		"id":            cfg.ID,
		"configuration": cfg.Configuration,
		"metadata": gin.H{
			"status":     string(cfg.Status),
			"created_at": cfg.CreatedAt.Format(time.RFC3339),
			"updated_at": cfg.UpdatedAt.Format(time.RFC3339),
		},
	}

	if cfg.DeployedAt != nil {
		apiDetail["metadata"].(gin.H)["deployed_at"] = cfg.DeployedAt.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"api":    apiDetail,
	})
}

// UpdateAPI implements ServerInterface.UpdateAPI
// (PUT /apis/{id})
func (s *APIServer) UpdateAPI(c *gin.Context, id string) {
	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to read request body",
		})
		return
	}

	// Parse configuration
	contentType := c.GetHeader("Content-Type")
	apiConfig, err := s.parser.Parse(body, contentType)
	if err != nil {
		s.logger.Error("Failed to parse configuration", zap.Error(err))
		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to parse configuration",
		})
		return
	}

	// Validate configuration
	validationErrors := s.validator.Validate(apiConfig)
	if len(validationErrors) > 0 {
		s.logger.Warn("Configuration validation failed",
			zap.String("name", apiConfig.Data.Name),
			zap.Int("num_errors", len(validationErrors)))

		errors := make([]api.ValidationError, len(validationErrors))
		for i, e := range validationErrors {
			errors[i] = api.ValidationError{
				Field:   stringPtr(e.Field),
				Message: stringPtr(e.Message),
			}
		}

		c.JSON(http.StatusBadRequest, api.ErrorResponse{
			Status:  "error",
			Message: "Configuration validation failed",
			Errors:  &errors,
		})
		return
	}

	// Check if config exists
	existing, err := s.store.Get(id)
	if err != nil {
		s.logger.Warn("API configuration not found", zap.String("id", id))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with ID '%s' not found", id),
		})
		return
	}

	// Update stored configuration
	now := time.Now()
	existing.Configuration = *apiConfig
	existing.Status = models.StatusPending
	existing.UpdatedAt = now
	existing.DeployedAt = nil
	existing.DeployedVersion = 0

	// Atomic dual-write: database + in-memory
	// Update database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.UpdateConfig(existing); err != nil {
			s.logger.Error("Failed to update config in database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to persist configuration update",
			})
			return
		}
	}

	if err := s.store.Update(existing); err != nil {
		s.logger.Error("Failed to update config in memory store", zap.Error(err))
		c.JSON(http.StatusConflict, api.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// Log audit event
	auditEvent := storage.CreateAuditEvent(
		storage.AuditUpdate,
		existing.ID,
		apiConfig.Data.Name,
		apiConfig.Data.Version,
		"SUCCESS",
		"",
	)
	if bboltDB, ok := s.db.(*storage.BBoltStorage); ok {
		_ = bboltDB.LogEvent(auditEvent)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx); err != nil {
			s.logger.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	s.logger.Info("API configuration updated",
		zap.String("id", existing.ID),
		zap.String("name", apiConfig.Data.Name),
		zap.String("version", apiConfig.Data.Version))

	// Return success response
	updateId, _ := uuidToOpenAPIUUID(existing.ID)
	c.JSON(http.StatusOK, api.APIUpdateResponse{
		Status:    stringPtr("success"),
		Message:   stringPtr("API configuration updated successfully"),
		Id:        updateId,
		UpdatedAt: timePtr(existing.UpdatedAt),
	})
}

// DeleteAPI implements ServerInterface.DeleteAPI
// (DELETE /apis/{id})
func (s *APIServer) DeleteAPI(c *gin.Context, id string) {
	// Check if config exists
	cfg, err := s.store.Get(id)
	if err != nil {
		s.logger.Warn("API configuration not found", zap.String("id", id))
		c.JSON(http.StatusNotFound, api.ErrorResponse{
			Status:  "error",
			Message: fmt.Sprintf("API configuration with ID '%s' not found", id),
		})
		return
	}

	// Delete from database first (only if persistent mode)
	if s.db != nil {
		if err := s.db.DeleteConfig(id); err != nil {
			s.logger.Error("Failed to delete config from database", zap.Error(err))
			c.JSON(http.StatusInternalServerError, api.ErrorResponse{
				Status:  "error",
				Message: "Failed to delete configuration",
			})
			return
		}
	}

	// Delete from in-memory store
	if err := s.store.Delete(id); err != nil {
		s.logger.Error("Failed to delete config from memory store", zap.Error(err))
		c.JSON(http.StatusInternalServerError, api.ErrorResponse{
			Status:  "error",
			Message: "Failed to delete configuration",
		})
		return
	}

	// Log audit event
	auditEvent := storage.CreateAuditEvent(
		storage.AuditDelete,
		cfg.ID,
		cfg.Configuration.Data.Name,
		cfg.Configuration.Data.Version,
		"SUCCESS",
		"",
	)
	if bboltDB, ok := s.db.(*storage.BBoltStorage); ok {
		_ = bboltDB.LogEvent(auditEvent)
	}

	// Update xDS snapshot asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.snapshotManager.UpdateSnapshot(ctx); err != nil {
			s.logger.Error("Failed to update xDS snapshot", zap.Error(err))
		}
	}()

	s.logger.Info("API configuration deleted",
		zap.String("id", cfg.ID),
		zap.String("name", cfg.Configuration.Data.Name),
		zap.String("version", cfg.Configuration.Data.Version))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "API configuration deleted successfully",
		"id":      id,
	})
}
