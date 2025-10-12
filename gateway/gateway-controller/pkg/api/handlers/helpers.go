package handlers

import (
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Helper functions to convert values to pointers

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func uuidToOpenAPIUUID(id string) (*openapi_types.UUID, error) {
	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	openapiUUID := openapi_types.UUID(parsedUUID)
	return &openapiUUID, nil
}

func statusPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
