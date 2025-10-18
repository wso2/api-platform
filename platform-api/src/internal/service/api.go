/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"errors"
	"fmt"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"platform-api/src/internal/constants"
)

// APIService handles business logic for API operations
type APIService struct {
	apiRepo     repository.APIRepository
	projectRepo repository.ProjectRepository
}

// NewAPIService creates a new API service
func NewAPIService(apiRepo repository.APIRepository, projectRepo repository.ProjectRepository) *APIService {
	return &APIService{
		apiRepo:     apiRepo,
		projectRepo: projectRepo,
	}
}

// CreateAPI creates a new API with validation and business logic
func (s *APIService) CreateAPI(req *CreateAPIRequest) (*dto.API, error) {
	// Validate request
	if err := s.validateCreateAPIRequest(req); err != nil {
		return nil, err
	}

	// Check if project exists
	project, err := s.projectRepo.GetProjectByUUID(req.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}

	// Check if API context already exists in the project
	existingAPIs, err := s.apiRepo.GetAPIsByProjectID(req.ProjectID)
	if err != nil {
		return nil, err
	}

	for _, api := range existingAPIs {
		if api.Name == req.Name && api.Context == req.Context && api.Version == req.Version {
			return nil, constants.ErrAPIAlreadyExists
		}
	}

	// Generate UUID for the API
	apiUUID := uuid.New().String()

	// Set default values if not provided
	if req.Provider == "" {
		req.Provider = "admin" // Default provider
	}
	if req.Type == "" {
		req.Type = "HTTP"
	}
	if len(req.Transport) == 0 {
		req.Transport = []string{"http", "https"}
	}
	if req.LifeCycleStatus == "" {
		req.LifeCycleStatus = "CREATED"
	}
	if req.Operations == nil || len(req.Operations) == 0 {
		// generate default get, post, patch and delete operations with path /*
		defaultOperations := s.generateDefaultOperations()
		req.Operations = defaultOperations
	}

	// Create API DTO
	api := &dto.API{
		ID:               apiUUID,
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		Context:          req.Context,
		Version:          req.Version,
		Provider:         req.Provider,
		ProjectID:        req.ProjectID,
		LifeCycleStatus:  req.LifeCycleStatus,
		HasThumbnail:     req.HasThumbnail,
		IsDefaultVersion: req.IsDefaultVersion,
		IsRevision:       req.IsRevision,
		RevisionedAPIID:  req.RevisionedAPIID,
		RevisionID:       req.RevisionID,
		Type:             req.Type,
		Transport:        req.Transport,
		MTLS:             req.MTLS,
		Security:         req.Security,
		CORS:             req.CORS,
		BackendServices:  req.BackendServices,
		APIRateLimiting:  req.APIRateLimiting,
		Operations:       req.Operations,
	}

	apiModel := s.dtoToModel(api)
	// Create API in repository
	if err := s.apiRepo.CreateAPI(apiModel); err != nil {
		return nil, fmt.Errorf("failed to create api: %w", err)
	}

	return api, nil
}

// GetAPIByUUID retrieves an API by its UUID
func (s *APIService) GetAPIByUUID(uuid string) (*dto.API, error) {
	if uuid == "" {
		return nil, errors.New("uuid is required")
	}

	apiModel, err := s.apiRepo.GetAPIByUUID(uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to get api: %w", err)
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}

	api := s.modelToDTO(apiModel)
	return api, nil
}

// GetAPIsByProjectID retrieves all APIs for a project
func (s *APIService) GetAPIsByProjectID(projectID string) ([]*dto.API, error) {
	if projectID == "" {
		return nil, errors.New("project id is required")
	}

	// Check if project exists
	project, err := s.projectRepo.GetProjectByUUID(projectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, constants.ErrProjectNotFound
	}

	apiModels, err := s.apiRepo.GetAPIsByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get apis: %w", err)
	}

	apis := make([]*dto.API, 0)
	for _, apiModel := range apiModels {
		api := s.modelToDTO(apiModel)
		apis = append(apis, api)
	}
	return apis, nil
}

// UpdateAPI updates an existing API
func (s *APIService) UpdateAPI(uuid string, req *UpdateAPIRequest) (*dto.API, error) {
	if uuid == "" {
		return nil, errors.New("uuid is required")
	}

	// Get existing API
	existingAPIModel, err := s.apiRepo.GetAPIByUUID(uuid)
	if err != nil {
		return nil, err
	}
	if existingAPIModel == nil {
		return nil, constants.ErrAPINotFound
	}

	existingAPI := s.modelToDTO(existingAPIModel)

	// Validate update request
	if err := s.validateUpdateAPIRequest(req); err != nil {
		return nil, err
	}

	// Update fields (only allow certain fields to be updated)
	if req.DisplayName != nil {
		existingAPI.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		existingAPI.Description = *req.Description
	}
	if req.Provider != nil {
		existingAPI.Provider = *req.Provider
	}
	if req.LifeCycleStatus != nil {
		existingAPI.LifeCycleStatus = *req.LifeCycleStatus
	}
	if req.HasThumbnail != nil {
		existingAPI.HasThumbnail = *req.HasThumbnail
	}
	if req.IsDefaultVersion != nil {
		existingAPI.IsDefaultVersion = *req.IsDefaultVersion
	}
	if req.IsRevision != nil {
		existingAPI.IsRevision = *req.IsRevision
	}
	if req.RevisionedAPIID != nil {
		existingAPI.RevisionedAPIID = *req.RevisionedAPIID
	}
	if req.RevisionID != nil {
		existingAPI.RevisionID = *req.RevisionID
	}
	if req.Type != nil {
		existingAPI.Type = *req.Type
	}
	if req.Transport != nil {
		existingAPI.Transport = *req.Transport
	}
	if req.MTLS != nil {
		existingAPI.MTLS = req.MTLS
	}
	if req.Security != nil {
		existingAPI.Security = req.Security
	}
	if req.CORS != nil {
		existingAPI.CORS = req.CORS
	}
	if req.BackendServices != nil {
		existingAPI.BackendServices = *req.BackendServices
	}
	if req.APIRateLimiting != nil {
		existingAPI.APIRateLimiting = req.APIRateLimiting
	}
	if req.Operations != nil {
		existingAPI.Operations = *req.Operations
	}

	// Update API in repository
	updatedAPIModel := s.dtoToModel(existingAPI)
	if err := s.apiRepo.UpdateAPI(updatedAPIModel); err != nil {
		return nil, fmt.Errorf("failed to update api: %w", err)
	}

	return existingAPI, nil
}

// DeleteAPI deletes an API
func (s *APIService) DeleteAPI(uuid string) error {
	if uuid == "" {
		return errors.New("uuid is required")
	}

	// Check if API exists
	api, err := s.apiRepo.GetAPIByUUID(uuid)
	if err != nil {
		return err
	}
	if api == nil {
		return constants.ErrAPINotFound
	}

	// Delete API from repository
	if err := s.apiRepo.DeleteAPI(uuid); err != nil {
		return fmt.Errorf("failed to delete api: %w", err)
	}

	return nil
}

// UpdateAPILifecycleStatus updates only the lifecycle status of an API
func (s *APIService) UpdateAPILifecycleStatus(uuid string, status string) (*dto.API, error) {
	if uuid == "" {
		return nil, errors.New("uuid is required")
	}
	if status == "" {
		return nil, errors.New("status is required")
	}

	// Validate lifecycle status
	if !constants.ValidLifecycleStates[status] {
		return nil, constants.ErrInvalidLifecycleState
	}

	// Get existing API
	apiModel, err := s.apiRepo.GetAPIByUUID(uuid)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}

	// Update lifecycle status
	apiModel.LifeCycleStatus = status
	apiModel.UpdatedAt = time.Now()

	// Update API in repository
	if err := s.apiRepo.UpdateAPI(apiModel); err != nil {
		return nil, fmt.Errorf("failed to update api lifecycle status: %w", err)
	}

	api := s.modelToDTO(apiModel)
	return api, nil
}

// DeployAPIRevision deploys an API revision and generates deployment YAML
func (s *APIService) DeployAPIRevision(apiUUID string, revisionID string,
	deploymentRequests []dto.APIRevisionDeployment) ([]*dto.APIRevisionDeployment, error) {
	if apiUUID == "" {
		return nil, errors.New("api uuid is required")
	}

	// Get the API from database
	apiModel, err := s.apiRepo.GetAPIByUUID(apiUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, constants.ErrAPINotFound
	}

	// Convert to DTO for easier manipulation
	api := s.modelToDTO(apiModel)

	// Generate API deployment YAML
	apiYAML, err := s.generateAPIDeploymentYAML(api)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API deployment YAML: %w", err)
	}

	// Process deployment requests and create deployment responses
	var deployments []*dto.APIRevisionDeployment
	currentTime := time.Now().Format(time.RFC3339)

	for _, deploymentReq := range deploymentRequests {
		// Validate deployment request
		if err := s.validateDeploymentRequest(&deploymentReq); err != nil {
			return nil, err
		}

		deployment := &dto.APIRevisionDeployment{
			RevisionUUID:        revisionID, // Optional, can be empty
			Name:                deploymentReq.Name,
			Status:              "CREATED", // Default status for new deployments
			VHost:               deploymentReq.VHost,
			DisplayOnDevportal:  deploymentReq.DisplayOnDevportal,
			DeployedTime:        &currentTime,
			SuccessDeployedTime: &currentTime,
		}

		deployments = append(deployments, deployment)
	}

	// Log the generated YAML for debugging/monitoring purposes
	// TODO - send the deployment requests to the gateway via websocket
	fmt.Printf("Generated API Deployment YAML for API %s:\n%s\n", apiUUID, apiYAML)

	return deployments, nil
}

// generateAPIDeploymentYAML creates the deployment YAML from API data
func (s *APIService) generateAPIDeploymentYAML(api *dto.API) (string, error) {
	// Create API deployment YAML structure
	apiYAMLData := dto.APIYAMLData{
		UUID:            api.ID,
		Name:            api.Name,
		DisplayName:     api.DisplayName,
		Version:         api.Version,
		Description:     api.Description,
		Context:         api.Context,
		Provider:        api.Provider,
		CreatedTime:     api.CreatedAt.Format(time.RFC3339),
		LastUpdatedTime: api.UpdatedAt.Format(time.RFC3339),
		LifeCycleStatus: api.LifeCycleStatus,
		Type:            api.Type,
		Transport:       api.Transport,
		MTLS:            api.MTLS,
		Security:        api.Security,
		CORS:            api.CORS,
		BackendServices: api.BackendServices,
		APIRateLimiting: api.APIRateLimiting,
		Operations:      api.Operations,
	}

	apiDeployment := dto.APIDeploymentYAML{
		Kind:    "api",
		Version: "v1",
		Data:    apiYAMLData,
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(apiDeployment)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// validateDeploymentRequest validates the deployment request
func (s *APIService) validateDeploymentRequest(req *dto.APIRevisionDeployment) error {
	if req.Name == "" {
		return errors.New("deployment name is required")
	}
	if req.VHost == "" {
		return errors.New("vhost is required")
	}

	// TODO - vHost validation

	return nil
}

// Validation methods

// validateCreateAPIRequest checks the validity of the create API request
func (s *APIService) validateCreateAPIRequest(req *CreateAPIRequest) error {
	if req.Name == "" {
		return constants.ErrInvalidAPIName
	}
	if req.Context == "" {
		return constants.ErrInvalidAPIContext
	}
	if req.Version == "" {
		return constants.ErrInvalidAPIVersion
	}
	if req.ProjectID == "" {
		return errors.New("project id is required")
	}

	// Validate API name format
	if !s.isValidAPIName(req.Name) {
		return constants.ErrInvalidAPIName
	}

	// Validate context format
	if !s.isValidContext(req.Context) {
		return constants.ErrInvalidAPIContext
	}

	// Validate version format
	if !s.isValidVersion(req.Version) {
		return constants.ErrInvalidAPIVersion
	}

	// Validate lifecycle status if provided
	if req.LifeCycleStatus != "" && !constants.ValidLifecycleStates[req.LifeCycleStatus] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Type != "" && !constants.ValidAPITypes[req.Type] {
		return constants.ErrInvalidAPIType
	}

	// Validate transport protocols if provided
	if len(req.Transport) > 0 {
		for _, transport := range req.Transport {
			if !constants.ValidTransports[strings.ToLower(transport)] {
				return constants.ErrInvalidTransport
			}
		}
	}

	return nil
}

// validateUpdateAPIRequest checks the validity of the update API request
func (s *APIService) validateUpdateAPIRequest(req *UpdateAPIRequest) error {
	// Validate lifecycle status if provided
	if req.LifeCycleStatus != nil && !constants.ValidLifecycleStates[*req.LifeCycleStatus] {
		return constants.ErrInvalidLifecycleState
	}

	// Validate API type if provided
	if req.Type != nil && !constants.ValidAPITypes[*req.Type] {
		return constants.ErrInvalidAPIType
	}

	// Validate transport protocols if provided
	if req.Transport != nil {
		for _, transport := range *req.Transport {
			if !constants.ValidTransports[strings.ToLower(transport)] {
				return constants.ErrInvalidTransport
			}
		}
	}

	return nil
}

// Helper validation methods

func (s *APIService) isValidAPIName(name string) bool {
	// API name should not contain special characters except spaces and hyphens
	pattern := `^[^~!@#;:%^*()+={}|\\<>"'',&$\[\]\/]*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched && len(name) > 0
}

func (s *APIService) isValidContext(context string) bool {
	// Context should be URL-friendly, no spaces or special characters
	pattern := `^\/?[a-zA-Z0-9_/-]+$`
	matched, _ := regexp.MatchString(pattern, context)
	return matched && len(context) > 0 && len(context) <= 232
}

func (s *APIService) isValidVersion(version string) bool {
	// Version should follow semantic versioning or simple version format
	pattern := `^[^~!@#;:%^*()+={}|\\<>"'',&/$\[\]\s+\/]+$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched && len(version) > 0 && len(version) <= 30
}

// isValidVHost validates vhost format
func (s *APIService) isValidVHost(vhost string) bool {
	// Basic hostname validation pattern as per RFC 1123
	pattern := `^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-ZaZ0-9\-]*[A-ZaZ0-9])$`
	matched, _ := regexp.MatchString(pattern, vhost)
	return matched
}

// Request/Response DTOs

// CreateAPIRequest represents the request to create a new API
type CreateAPIRequest struct {
	Name             string                  `json:"name"`
	DisplayName      string                  `json:"displayName,omitempty"`
	Description      string                  `json:"description,omitempty"`
	Context          string                  `json:"context"`
	Version          string                  `json:"version"`
	Provider         string                  `json:"provider,omitempty"`
	ProjectID        string                  `json:"project_id"`
	LifeCycleStatus  string                  `json:"lifeCycleStatus,omitempty"`
	HasThumbnail     bool                    `json:"hasThumbnail,omitempty"`
	IsDefaultVersion bool                    `json:"isDefaultVersion,omitempty"`
	IsRevision       bool                    `json:"isRevision,omitempty"`
	RevisionedAPIID  string                  `json:"revisionedApiId,omitempty"`
	RevisionID       int                     `json:"revisionId,omitempty"`
	Type             string                  `json:"type,omitempty"`
	Transport        []string                `json:"transport,omitempty"`
	MTLS             *dto.MTLSConfig         `json:"mtls,omitempty"`
	Security         *dto.SecurityConfig     `json:"security,omitempty"`
	CORS             *dto.CORSConfig         `json:"cors,omitempty"`
	BackendServices  []dto.BackendService    `json:"backend-services,omitempty"`
	APIRateLimiting  *dto.RateLimitingConfig `json:"api-rate-limiting,omitempty"`
	Operations       []dto.Operation         `json:"operations,omitempty"`
}

// UpdateAPIRequest represents the request to update an API
type UpdateAPIRequest struct {
	DisplayName      *string                 `json:"displayName,omitempty"`
	Description      *string                 `json:"description,omitempty"`
	Provider         *string                 `json:"provider,omitempty"`
	LifeCycleStatus  *string                 `json:"lifeCycleStatus,omitempty"`
	HasThumbnail     *bool                   `json:"hasThumbnail,omitempty"`
	IsDefaultVersion *bool                   `json:"isDefaultVersion,omitempty"`
	IsRevision       *bool                   `json:"isRevision,omitempty"`
	RevisionedAPIID  *string                 `json:"revisionedApiId,omitempty"`
	RevisionID       *int                    `json:"revisionId,omitempty"`
	Type             *string                 `json:"type,omitempty"`
	Transport        *[]string               `json:"transport,omitempty"`
	MTLS             *dto.MTLSConfig         `json:"mtls,omitempty"`
	Security         *dto.SecurityConfig     `json:"security,omitempty"`
	CORS             *dto.CORSConfig         `json:"cors,omitempty"`
	BackendServices  *[]dto.BackendService   `json:"backend-services,omitempty"`
	APIRateLimiting  *dto.RateLimitingConfig `json:"api-rate-limiting,omitempty"`
	Operations       *[]dto.Operation        `json:"operations,omitempty"`
}

// Mapping functions
func (s *APIService) dtoToModel(dto *dto.API) *model.API {
	if dto == nil {
		return nil
	}

	return &model.API{
		ID:               dto.ID,
		Name:             dto.Name,
		DisplayName:      dto.DisplayName,
		Description:      dto.Description,
		Context:          dto.Context,
		Version:          dto.Version,
		Provider:         dto.Provider,
		ProjectID:        dto.ProjectID,
		LifeCycleStatus:  dto.LifeCycleStatus,
		HasThumbnail:     dto.HasThumbnail,
		IsDefaultVersion: dto.IsDefaultVersion,
		IsRevision:       dto.IsRevision,
		RevisionedAPIID:  dto.RevisionedAPIID,
		RevisionID:       dto.RevisionID,
		Type:             dto.Type,
		Transport:        dto.Transport,
		MTLS:             s.mtlsDTOToModel(dto.MTLS),
		Security:         s.securityDTOToModel(dto.Security),
		CORS:             s.corsDTOToModel(dto.CORS),
		BackendServices:  s.backendServicesDTOToModel(dto.BackendServices),
		APIRateLimiting:  s.rateLimitingDTOToModel(dto.APIRateLimiting),
		Operations:       s.operationsDTOToModel(dto.Operations),
	}
}

func (s *APIService) modelToDTO(model *model.API) *dto.API {
	if model == nil {
		return nil
	}

	return &dto.API{
		ID:               model.ID,
		Name:             model.Name,
		DisplayName:      model.DisplayName,
		Description:      model.Description,
		Context:          model.Context,
		Version:          model.Version,
		Provider:         model.Provider,
		ProjectID:        model.ProjectID,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
		LifeCycleStatus:  model.LifeCycleStatus,
		HasThumbnail:     model.HasThumbnail,
		IsDefaultVersion: model.IsDefaultVersion,
		IsRevision:       model.IsRevision,
		RevisionedAPIID:  model.RevisionedAPIID,
		RevisionID:       model.RevisionID,
		Type:             model.Type,
		Transport:        model.Transport,
		MTLS:             s.mtlsModelToDTO(model.MTLS),
		Security:         s.securityModelToDTO(model.Security),
		CORS:             s.corsModelToDTO(model.CORS),
		BackendServices:  s.backendServicesModelToDTO(model.BackendServices),
		APIRateLimiting:  s.rateLimitingModelToDTO(model.APIRateLimiting),
		Operations:       s.operationsModelToDTO(model.Operations),
	}
}

// Helper DTO to Model conversion methods

func (s *APIService) mtlsDTOToModel(dto *dto.MTLSConfig) *model.MTLSConfig {
	if dto == nil {
		return nil
	}
	return &model.MTLSConfig{
		Enabled:                    dto.Enabled,
		EnforceIfClientCertPresent: dto.EnforceIfClientCertPresent,
		VerifyClient:               dto.VerifyClient,
		ClientCert:                 dto.ClientCert,
		ClientKey:                  dto.ClientKey,
		CACert:                     dto.CACert,
	}
}

func (s *APIService) securityDTOToModel(dto *dto.SecurityConfig) *model.SecurityConfig {
	if dto == nil {
		return nil
	}
	return &model.SecurityConfig{
		Enabled: dto.Enabled,
		APIKey:  s.apiKeyDTOToModel(dto.APIKey),
		OAuth2:  s.oauth2DTOToModel(dto.OAuth2),
	}
}

func (s *APIService) apiKeyDTOToModel(dto *dto.APIKeySecurity) *model.APIKeySecurity {
	if dto == nil {
		return nil
	}
	return &model.APIKeySecurity{
		Enabled: dto.Enabled,
		Header:  dto.Header,
		Query:   dto.Query,
		Cookie:  dto.Cookie,
	}
}

func (s *APIService) oauth2DTOToModel(dto *dto.OAuth2Security) *model.OAuth2Security {
	if dto == nil {
		return nil
	}
	return &model.OAuth2Security{
		GrantTypes: s.oauth2GrantTypesDTOToModel(dto.GrantTypes),
		Scopes:     dto.Scopes,
	}
}

func (s *APIService) oauth2GrantTypesDTOToModel(dto *dto.OAuth2GrantTypes) *model.OAuth2GrantTypes {
	if dto == nil {
		return nil
	}
	return &model.OAuth2GrantTypes{
		AuthorizationCode: s.authCodeGrantDTOToModel(dto.AuthorizationCode),
		Implicit:          s.implicitGrantDTOToModel(dto.Implicit),
		Password:          s.passwordGrantDTOToModel(dto.Password),
		ClientCredentials: s.clientCredGrantDTOToModel(dto.ClientCredentials),
	}
}

func (s *APIService) authCodeGrantDTOToModel(dto *dto.AuthorizationCodeGrant) *model.AuthorizationCodeGrant {
	if dto == nil {
		return nil
	}
	return &model.AuthorizationCodeGrant{
		Enabled:     dto.Enabled,
		CallbackURL: dto.CallbackURL,
	}
}

func (s *APIService) implicitGrantDTOToModel(dto *dto.ImplicitGrant) *model.ImplicitGrant {
	if dto == nil {
		return nil
	}
	return &model.ImplicitGrant{
		Enabled:     dto.Enabled,
		CallbackURL: dto.CallbackURL,
	}
}

func (s *APIService) passwordGrantDTOToModel(dto *dto.PasswordGrant) *model.PasswordGrant {
	if dto == nil {
		return nil
	}
	return &model.PasswordGrant{
		Enabled: dto.Enabled,
	}
}

func (s *APIService) clientCredGrantDTOToModel(dto *dto.ClientCredentialsGrant) *model.ClientCredentialsGrant {
	if dto == nil {
		return nil
	}
	return &model.ClientCredentialsGrant{
		Enabled: dto.Enabled,
	}
}

func (s *APIService) corsDTOToModel(dto *dto.CORSConfig) *model.CORSConfig {
	if dto == nil {
		return nil
	}
	return &model.CORSConfig{
		Enabled:          dto.Enabled,
		AllowOrigins:     dto.AllowOrigins,
		AllowMethods:     dto.AllowMethods,
		AllowHeaders:     dto.AllowHeaders,
		ExposeHeaders:    dto.ExposeHeaders,
		MaxAge:           dto.MaxAge,
		AllowCredentials: dto.AllowCredentials,
	}
}

func (s *APIService) backendServicesDTOToModel(dtos []dto.BackendService) []model.BackendService {
	if dtos == nil {
		return nil
	}
	backendServiceModels := make([]model.BackendService, 0)
	for _, backendServiceDTO := range dtos {
		backendServiceModels = append(backendServiceModels, *s.backendServiceDTOToModel(&backendServiceDTO))
	}
	return backendServiceModels
}

func (s *APIService) backendServiceDTOToModel(dto *dto.BackendService) *model.BackendService {
	if dto == nil {
		return nil
	}
	return &model.BackendService{
		Name:           dto.Name,
		IsDefault:      dto.IsDefault,
		Endpoints:      s.backendEndpointsDTOToModel(dto.Endpoints),
		Timeout:        s.timeoutDTOToModel(dto.Timeout),
		Retries:        dto.Retries,
		LoadBalance:    s.loadBalanceDTOToModel(dto.LoadBalance),
		CircuitBreaker: s.circuitBreakerDTOToModel(dto.CircuitBreaker),
	}
}

func (s *APIService) backendEndpointsDTOToModel(dtos []dto.BackendEndpoint) []model.BackendEndpoint {
	if dtos == nil {
		return nil
	}
	backendEndpointModels := make([]model.BackendEndpoint, 0)
	for _, backendEndpointDTO := range dtos {
		backendEndpointModels = append(backendEndpointModels, *s.backendEndpointDTOToModel(&backendEndpointDTO))
	}
	return backendEndpointModels
}

func (s *APIService) backendEndpointDTOToModel(dto *dto.BackendEndpoint) *model.BackendEndpoint {
	if dto == nil {
		return nil
	}
	return &model.BackendEndpoint{
		URL:         dto.URL,
		Description: dto.Description,
		HealthCheck: s.healthCheckDTOToModel(dto.HealthCheck),
		Weight:      dto.Weight,
		MTLS:        s.mtlsDTOToModel(dto.MTLS),
	}
}

func (s *APIService) healthCheckDTOToModel(dto *dto.HealthCheckConfig) *model.HealthCheckConfig {
	if dto == nil {
		return nil
	}
	return &model.HealthCheckConfig{
		Enabled:            dto.Enabled,
		Interval:           dto.Interval,
		Timeout:            dto.Timeout,
		UnhealthyThreshold: dto.UnhealthyThreshold,
		HealthyThreshold:   dto.HealthyThreshold,
	}
}

func (s *APIService) timeoutDTOToModel(dto *dto.TimeoutConfig) *model.TimeoutConfig {
	if dto == nil {
		return nil
	}
	return &model.TimeoutConfig{
		Connect: dto.Connect,
		Read:    dto.Read,
		Write:   dto.Write,
	}
}

func (s *APIService) loadBalanceDTOToModel(dto *dto.LoadBalanceConfig) *model.LoadBalanceConfig {
	if dto == nil {
		return nil
	}
	return &model.LoadBalanceConfig{
		Algorithm: dto.Algorithm,
		Failover:  dto.Failover,
	}
}

func (s *APIService) circuitBreakerDTOToModel(dto *dto.CircuitBreakerConfig) *model.CircuitBreakerConfig {
	if dto == nil {
		return nil
	}
	return &model.CircuitBreakerConfig{
		Enabled:            dto.Enabled,
		MaxConnections:     dto.MaxConnections,
		MaxPendingRequests: dto.MaxPendingRequests,
		MaxRequests:        dto.MaxRequests,
		MaxRetries:         dto.MaxRetries,
	}
}

func (s *APIService) rateLimitingDTOToModel(dto *dto.RateLimitingConfig) *model.RateLimitingConfig {
	if dto == nil {
		return nil
	}
	return &model.RateLimitingConfig{
		Enabled:           dto.Enabled,
		RateLimitCount:    dto.RateLimitCount,
		RateLimitTimeUnit: dto.RateLimitTimeUnit,
		StopOnQuotaReach:  dto.StopOnQuotaReach,
	}
}

func (s *APIService) operationsDTOToModel(dtos []dto.Operation) []model.Operation {
	if dtos == nil {
		return nil
	}
	operationsModels := make([]model.Operation, 0)
	for _, operationsDTO := range dtos {
		operationsModels = append(operationsModels, *s.operationDTOToModel(&operationsDTO))
	}
	return operationsModels
}

func (s *APIService) operationDTOToModel(dto *dto.Operation) *model.Operation {
	if dto == nil {
		return nil
	}
	return &model.Operation{
		Name:        dto.Name,
		Description: dto.Description,
		Request:     s.operationRequestDTOToModel(dto.Request),
	}
}

func (s *APIService) operationRequestDTOToModel(dto *dto.OperationRequest) *model.OperationRequest {
	if dto == nil {
		return nil
	}
	return &model.OperationRequest{
		Method:           dto.Method,
		Path:             dto.Path,
		BackendServices:  s.backendRoutingDTOsToModel(dto.BackendServices),
		Authentication:   s.authConfigDTOToModel(dto.Authentication),
		RequestPolicies:  s.policiesDTOToModel(dto.RequestPolicies),
		ResponsePolicies: s.policiesDTOToModel(dto.ResponsePolicies),
	}
}

func (s *APIService) backendRoutingDTOsToModel(dtos []dto.BackendRouting) []model.BackendRouting {
	if dtos == nil {
		return nil
	}
	backendRoutingModels := make([]model.BackendRouting, 0)
	for _, operationsDTO := range dtos {
		backendRoutingModels = append(backendRoutingModels, *s.backendRoutingDTOToModel(&operationsDTO))
	}
	return backendRoutingModels
}

func (s *APIService) backendRoutingDTOToModel(dto *dto.BackendRouting) *model.BackendRouting {
	if dto == nil {
		return nil
	}
	return &model.BackendRouting{
		Name:   dto.Name,
		Weight: dto.Weight,
	}
}

func (s *APIService) authConfigDTOToModel(dto *dto.AuthenticationConfig) *model.AuthenticationConfig {
	if dto == nil {
		return nil
	}
	return &model.AuthenticationConfig{
		Required: dto.Required,
		Scopes:   dto.Scopes,
	}
}

func (s *APIService) policiesDTOToModel(dtos []dto.Policy) []model.Policy {
	if dtos == nil {
		return nil
	}
	policyModels := make([]model.Policy, 0)
	for _, policyDTO := range dtos {
		policyModels = append(policyModels, *s.policyDTOToModel(&policyDTO))
	}
	return policyModels
}

func (s *APIService) policyDTOToModel(dto *dto.Policy) *model.Policy {
	if dto == nil {
		return nil
	}
	return &model.Policy{
		Name:   dto.Name,
		Params: dto.Params,
	}
}

// Helper Model to DTO conversion methods

func (s *APIService) mtlsModelToDTO(model *model.MTLSConfig) *dto.MTLSConfig {
	if model == nil {
		return nil
	}
	return &dto.MTLSConfig{
		Enabled:                    model.Enabled,
		EnforceIfClientCertPresent: model.EnforceIfClientCertPresent,
		VerifyClient:               model.VerifyClient,
		ClientCert:                 model.ClientCert,
		ClientKey:                  model.ClientKey,
		CACert:                     model.CACert,
	}
}

func (s *APIService) securityModelToDTO(model *model.SecurityConfig) *dto.SecurityConfig {
	if model == nil {
		return nil
	}
	return &dto.SecurityConfig{
		Enabled: model.Enabled,
		APIKey:  s.apiKeyModelToDTO(model.APIKey),
		OAuth2:  s.oauth2ModelToDTO(model.OAuth2),
	}
}

func (s *APIService) apiKeyModelToDTO(model *model.APIKeySecurity) *dto.APIKeySecurity {
	if model == nil {
		return nil
	}
	return &dto.APIKeySecurity{
		Enabled: model.Enabled,
		Header:  model.Header,
		Query:   model.Query,
		Cookie:  model.Cookie,
	}
}

func (s *APIService) oauth2ModelToDTO(model *model.OAuth2Security) *dto.OAuth2Security {
	if model == nil {
		return nil
	}
	return &dto.OAuth2Security{
		GrantTypes: s.oauth2GrantTypesModelToDTO(model.GrantTypes),
		Scopes:     model.Scopes,
	}
}

func (s *APIService) oauth2GrantTypesModelToDTO(model *model.OAuth2GrantTypes) *dto.OAuth2GrantTypes {
	if model == nil {
		return nil
	}
	return &dto.OAuth2GrantTypes{
		AuthorizationCode: s.authCodeGrantModelToDTO(model.AuthorizationCode),
		Implicit:          s.implicitGrantModelToDTO(model.Implicit),
		Password:          s.passwordGrantModelToDTO(model.Password),
		ClientCredentials: s.clientCredGrantModelToDTO(model.ClientCredentials),
	}
}

func (s *APIService) authCodeGrantModelToDTO(model *model.AuthorizationCodeGrant) *dto.AuthorizationCodeGrant {
	if model == nil {
		return nil
	}
	return &dto.AuthorizationCodeGrant{
		Enabled:     model.Enabled,
		CallbackURL: model.CallbackURL,
	}
}

func (s *APIService) implicitGrantModelToDTO(model *model.ImplicitGrant) *dto.ImplicitGrant {
	if model == nil {
		return nil
	}
	return &dto.ImplicitGrant{
		Enabled:     model.Enabled,
		CallbackURL: model.CallbackURL,
	}
}

func (s *APIService) passwordGrantModelToDTO(model *model.PasswordGrant) *dto.PasswordGrant {
	if model == nil {
		return nil
	}
	return &dto.PasswordGrant{
		Enabled: model.Enabled,
	}
}

func (s *APIService) clientCredGrantModelToDTO(model *model.ClientCredentialsGrant) *dto.ClientCredentialsGrant {
	if model == nil {
		return nil
	}
	return &dto.ClientCredentialsGrant{
		Enabled: model.Enabled,
	}
}

func (s *APIService) corsModelToDTO(model *model.CORSConfig) *dto.CORSConfig {
	if model == nil {
		return nil
	}
	return &dto.CORSConfig{
		Enabled:          model.Enabled,
		AllowOrigins:     model.AllowOrigins,
		AllowMethods:     model.AllowMethods,
		AllowHeaders:     model.AllowHeaders,
		ExposeHeaders:    model.ExposeHeaders,
		MaxAge:           model.MaxAge,
		AllowCredentials: model.AllowCredentials,
	}
}

func (s *APIService) backendServicesModelToDTO(models []model.BackendService) []dto.BackendService {
	if models == nil {
		return nil
	}
	backendServiceDTOs := make([]dto.BackendService, 0)
	for _, backendServiceModel := range models {
		backendServiceDTOs = append(backendServiceDTOs, *s.backendServiceModelToDTO(&backendServiceModel))
	}
	return backendServiceDTOs
}

func (s *APIService) backendServiceModelToDTO(model *model.BackendService) *dto.BackendService {
	if model == nil {
		return nil
	}
	return &dto.BackendService{
		Name:           model.Name,
		IsDefault:      model.IsDefault,
		Endpoints:      s.backendEndpointsModelToDTO(model.Endpoints),
		Timeout:        s.timeoutModelToDTO(model.Timeout),
		Retries:        model.Retries,
		LoadBalance:    s.loadBalanceModelToDTO(model.LoadBalance),
		CircuitBreaker: s.circuitBreakerModelToDTO(model.CircuitBreaker),
	}
}

func (s *APIService) backendEndpointsModelToDTO(models []model.BackendEndpoint) []dto.BackendEndpoint {
	if models == nil {
		return nil
	}
	backendEndpointDTOs := make([]dto.BackendEndpoint, 0)
	for _, backendServiceModel := range models {
		backendEndpointDTOs = append(backendEndpointDTOs, *s.backendEndpointModelToDTO(&backendServiceModel))
	}
	return backendEndpointDTOs
}

func (s *APIService) backendEndpointModelToDTO(model *model.BackendEndpoint) *dto.BackendEndpoint {
	if model == nil {
		return nil
	}
	return &dto.BackendEndpoint{
		URL:         model.URL,
		Description: model.Description,
		HealthCheck: s.healthCheckModelToDTO(model.HealthCheck),
		Weight:      model.Weight,
		MTLS:        s.mtlsModelToDTO(model.MTLS),
	}
}

func (s *APIService) healthCheckModelToDTO(model *model.HealthCheckConfig) *dto.HealthCheckConfig {
	if model == nil {
		return nil
	}
	return &dto.HealthCheckConfig{
		Enabled:            model.Enabled,
		Interval:           model.Interval,
		Timeout:            model.Timeout,
		UnhealthyThreshold: model.UnhealthyThreshold,
		HealthyThreshold:   model.HealthyThreshold,
	}
}

func (s *APIService) timeoutModelToDTO(model *model.TimeoutConfig) *dto.TimeoutConfig {
	if model == nil {
		return nil
	}
	return &dto.TimeoutConfig{
		Connect: model.Connect,
		Read:    model.Read,
		Write:   model.Write,
	}
}

func (s *APIService) loadBalanceModelToDTO(model *model.LoadBalanceConfig) *dto.LoadBalanceConfig {
	if model == nil {
		return nil
	}
	return &dto.LoadBalanceConfig{
		Algorithm: model.Algorithm,
		Failover:  model.Failover,
	}
}

func (s *APIService) circuitBreakerModelToDTO(model *model.CircuitBreakerConfig) *dto.CircuitBreakerConfig {
	if model == nil {
		return nil
	}
	return &dto.CircuitBreakerConfig{
		Enabled:            model.Enabled,
		MaxConnections:     model.MaxConnections,
		MaxPendingRequests: model.MaxPendingRequests,
		MaxRequests:        model.MaxRequests,
		MaxRetries:         model.MaxRetries,
	}
}

func (s *APIService) rateLimitingModelToDTO(model *model.RateLimitingConfig) *dto.RateLimitingConfig {
	if model == nil {
		return nil
	}
	return &dto.RateLimitingConfig{
		Enabled:           model.Enabled,
		RateLimitCount:    model.RateLimitCount,
		RateLimitTimeUnit: model.RateLimitTimeUnit,
		StopOnQuotaReach:  model.StopOnQuotaReach,
	}
}

func (s *APIService) operationsModelToDTO(models []model.Operation) []dto.Operation {
	if models == nil {
		return nil
	}
	operationsDTOs := make([]dto.Operation, 0)
	for _, operationsModel := range models {
		operationsDTOs = append(operationsDTOs, *s.operationModelToDTO(&operationsModel))
	}
	return operationsDTOs
}

func (s *APIService) operationModelToDTO(model *model.Operation) *dto.Operation {
	if model == nil {
		return nil
	}
	return &dto.Operation{
		Name:        model.Name,
		Description: model.Description,
		Request:     s.operationRequestModelToDTO(model.Request),
	}
}

func (s *APIService) operationRequestModelToDTO(model *model.OperationRequest) *dto.OperationRequest {
	if model == nil {
		return nil
	}
	return &dto.OperationRequest{
		Method:           model.Method,
		Path:             model.Path,
		BackendServices:  s.backendRoutingModelsToDTO(model.BackendServices),
		Authentication:   s.authConfigModelToDTO(model.Authentication),
		RequestPolicies:  s.policiesModelToDTO(model.RequestPolicies),
		ResponsePolicies: s.policiesModelToDTO(model.ResponsePolicies),
	}
}

func (s *APIService) backendRoutingModelsToDTO(models []model.BackendRouting) []dto.BackendRouting {
	if models == nil {
		return nil
	}
	backendRoutingDTOs := make([]dto.BackendRouting, 0)
	for _, backendRoutingModel := range models {
		backendRoutingDTOs = append(backendRoutingDTOs, *s.backendRoutingModelToDTO(&backendRoutingModel))
	}
	return backendRoutingDTOs
}

func (s *APIService) backendRoutingModelToDTO(model *model.BackendRouting) *dto.BackendRouting {
	if model == nil {
		return nil
	}
	return &dto.BackendRouting{
		Name:   model.Name,
		Weight: model.Weight,
	}
}

func (s *APIService) authConfigModelToDTO(model *model.AuthenticationConfig) *dto.AuthenticationConfig {
	if model == nil {
		return nil
	}
	return &dto.AuthenticationConfig{
		Required: model.Required,
		Scopes:   model.Scopes,
	}
}

func (s *APIService) policiesModelToDTO(models []model.Policy) []dto.Policy {
	if models == nil {
		return nil
	}
	policyDTOs := make([]dto.Policy, 0)
	for _, policyModel := range models {
		policyDTOs = append(policyDTOs, *s.policyModelToDTO(&policyModel))
	}
	return policyDTOs
}

func (s *APIService) policyModelToDTO(model *model.Policy) *dto.Policy {
	if model == nil {
		return nil
	}
	return &dto.Policy{
		Name:   model.Name,
		Params: model.Params,
	}
}

func (s *APIService) generateDefaultOperations() []dto.Operation {
	return []dto.Operation{
		{
			Name:        "Get Resource",
			Description: "Retrieve all resources",
			Request: &dto.OperationRequest{
				Method: "GET",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
		{
			Name:        "POST Resource",
			Description: "Create a new resource",
			Request: &dto.OperationRequest{
				Method: "POST",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
		{
			Name:        "Update Resource",
			Description: "Update an existing resource",
			Request: &dto.OperationRequest{
				Method: "PATCH",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
		{
			Name:        "Delete Resource",
			Description: "Delete an existing resource",
			Request: &dto.OperationRequest{
				Method: "DELETE",
				Path:   "/*",
				Authentication: &dto.AuthenticationConfig{
					Required: false,
					Scopes:   []string{},
				},
				RequestPolicies:  []dto.Policy{},
				ResponsePolicies: []dto.Policy{},
			},
		},
	}
}
