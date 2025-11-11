package devportal_client

import (
	"log"
	dto "platform-api/src/internal/client/devportal_client/dto"
)

// OrganizationsService defines organization-related operations supported by the DevPortal DevPortalClient.
type OrganizationsService interface {
	Create(req dto.OrganizationCreateRequest) (*dto.OrganizationResponse, error)
	Get(orgID string) (*dto.OrganizationResponse, error)
	List() (dto.OrganizationListResponse, error)
	Update(orgID string, req dto.OrganizationUpdateRequest) (*dto.OrganizationResponse, error)
	Delete(orgID string) error
}

// organizationsService is the concrete implementation of OrganizationsService.
type organizationsService struct {
	DevPortalClient *DevPortalClient
}

// Create posts a new organization to the DevPortal.
// Assumes endpoint POST {baseURL}/organizations
func (s *organizationsService) Create(req dto.OrganizationCreateRequest) (*dto.OrganizationResponse, error) {
	url := s.DevPortalClient.buildURL("organizations")
	httpReq, err := s.DevPortalClient.newJSONRequest("POST", url, req)
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200, 201}, &out); err != nil {
		if de, ok := err.(*DevPortalError); ok {
			if de.Code == 409 {
				return nil, ErrOrganizationAlreadyExists
			}
		}
		return nil, err
	}
	return &out, nil
}

// Get retrieves an organization by ID. Assumes GET {baseURL}/organizations/{orgID}
func (s *organizationsService) Get(orgID string) (*dto.OrganizationResponse, error) {
	url := s.DevPortalClient.buildURL("organizations", orgID)
	httpReq, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		// convert not-found into friendly message like before
		if de, ok := err.(*DevPortalError); ok && de.Code == 404 {
			return nil, ErrOrganizationNotFound
		}
		return nil, err
	}
	return &out, nil
}

// List returns all organizations. Assumes GET {baseURL}/organizations
func (s *organizationsService) List() (dto.OrganizationListResponse, error) {
	url := s.DevPortalClient.buildURL("organizations")
	httpReq, err := s.DevPortalClient.newJSONRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationListResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Update sends an update for an organization. Assumes PUT {baseURL}/organizations/{orgID}
func (s *organizationsService) Update(orgID string, req dto.OrganizationUpdateRequest) (*dto.OrganizationResponse, error) {
	url := s.DevPortalClient.buildURL("organizations", orgID)
	log.Printf("Updating organization %s with data %+v", orgID, req)
	httpReq, err := s.DevPortalClient.newJSONRequest("PUT", url, req)
	if err != nil {
		return nil, err
	}
	var out dto.OrganizationResponse
	if err := s.DevPortalClient.doAndDecode(httpReq, []int{200}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes an organization. Assumes DELETE {baseURL}/organizations/{orgID}
func (s *organizationsService) Delete(orgID string) error {
	url := s.DevPortalClient.buildURL("organizations", orgID)
	httpReq, err := s.DevPortalClient.newJSONRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	return s.DevPortalClient.doNoContent(httpReq, []int{200, 204})
}

// Organizations returns a service for organization-related operations.
func (c *DevPortalClient) Organizations() OrganizationsService {
	return &organizationsService{DevPortalClient: c}
}
