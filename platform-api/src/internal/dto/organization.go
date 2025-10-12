package dto

import (
	"time"
)

// Organization represents an organization entity in the API management platform
type Organization struct {
	UUID      string    `json:"uuid"`
	Handle    string    `json:"handle"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
