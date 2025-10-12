package model

import (
	"time"
)

// Project represents a project entity in the API management platform
type Project struct {
	UUID           string    `json:"uuid" db:"uuid"`
	Name           string    `json:"name" db:"name"`
	OrganizationID string    `json:"organization_id" db:"organization_id"` // FK to Organization.UUID
	IsDefault      bool      `json:"is_default" db:"is_default"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the table name for the Organization model
func (Project) TableName() string {
	return "projects"
}
