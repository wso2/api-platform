package model

import (
	"time"
)

// Organization represents an organization entity in the API management platform
type Organization struct {
	UUID      string    `json:"uuid" db:"uuid"`
	Handle    string    `json:"handle" db:"handle"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TableName returns the table name for the Organization model
func (Organization) TableName() string {
	return "organizations"
}
