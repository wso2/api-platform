package devportal_client

import dto "platform-api/src/internal/client/devportal_client/dto"

// Re-export commonly used DTO types so callers can import only this package.
// These are type aliases and incur no runtime cost.
type (
	// Organizations
	OrganizationCreateRequest = dto.OrganizationCreateRequest
	OrganizationResponse      = dto.OrganizationResponse
	OrganizationUpdateRequest = dto.OrganizationUpdateRequest
	OrganizationListResponse  = dto.OrganizationListResponse

	// Views
	ViewRequest  = dto.ViewRequest
	ViewResponse = dto.ViewResponse

	// Labels
	Label     = dto.Label
	LabelList = dto.LabelList

	// Subscription policies
	SubscriptionPolicy = dto.SubscriptionPolicy

	// APIs
	Owners             = dto.Owners
	APIInfo            = dto.APIInfo
	EndPoints          = dto.EndPoints
	APIMetadataRequest = dto.APIMetadataRequest
	APIResponse        = dto.APIResponse

	// Re-export enum types from dto so callers only import devportal_client.
	SubscriptionType = dto.SubscriptionType
	Visibility       = dto.APIVisibility
	APIStatus        = dto.APIStatus
	APIType          = dto.APIType
)
