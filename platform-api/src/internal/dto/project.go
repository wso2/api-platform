package dto

import (
	"time"
)

// Project represents a project entity in the API management platform
type Project struct {
	UUID           string    `json:"uuid"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"organization_id"`
	IsDefault      bool      `json:"is_default"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
