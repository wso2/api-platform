package storage

import (
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// Storage is the interface for persisting API configurations
type Storage interface {
	// SaveConfig persists a new API configuration
	SaveConfig(cfg *models.StoredAPIConfig) error

	// UpdateConfig updates an existing API configuration
	UpdateConfig(cfg *models.StoredAPIConfig) error

	// DeleteConfig removes an API configuration by ID
	DeleteConfig(id string) error

	// GetConfig retrieves an API configuration by ID
	GetConfig(id string) (*models.StoredAPIConfig, error)

	// GetConfigByNameVersion retrieves an API configuration by name and version
	GetConfigByNameVersion(name, version string) (*models.StoredAPIConfig, error)

	// GetAllConfigs retrieves all API configurations
	GetAllConfigs() ([]*models.StoredAPIConfig, error)

	// Close closes the storage connection
	Close() error
}

// AuditLogger is the interface for logging audit events
type AuditLogger interface {
	// LogEvent logs an audit event
	LogEvent(event *AuditEvent) error

	// GetEvents retrieves audit events
	GetEvents(limit int) ([]*AuditEvent, error)
}

// AuditEvent represents a configuration change event
type AuditEvent struct {
	ID            string                 `json:"id"`
	Timestamp     string                 `json:"timestamp"`
	Operation     AuditOperation         `json:"operation"`
	ConfigID      string                 `json:"config_id"`
	ConfigName    string                 `json:"config_name"`
	ConfigVersion string                 `json:"config_version"`
	Status        string                 `json:"status"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// AuditOperation represents the type of change
type AuditOperation string

const (
	AuditCreate AuditOperation = "CREATE"
	AuditUpdate AuditOperation = "UPDATE"
	AuditDelete AuditOperation = "DELETE"
	AuditQuery  AuditOperation = "QUERY"
)
