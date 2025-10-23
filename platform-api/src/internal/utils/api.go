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

package utils

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"time"
)

type APIUtil struct{}

// Mapping functions
func (u *APIUtil) DTOToModel(dto *dto.API) *model.API {
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
		OrganizationID:   dto.OrganizationID,
		LifeCycleStatus:  dto.LifeCycleStatus,
		HasThumbnail:     dto.HasThumbnail,
		IsDefaultVersion: dto.IsDefaultVersion,
		IsRevision:       dto.IsRevision,
		RevisionedAPIID:  dto.RevisionedAPIID,
		RevisionID:       dto.RevisionID,
		Type:             dto.Type,
		Transport:        dto.Transport,
		MTLS:             u.mtlsDTOToModel(dto.MTLS),
		Security:         u.securityDTOToModel(dto.Security),
		CORS:             u.corsDTOToModel(dto.CORS),
		BackendServices:  u.backendServicesDTOToModel(dto.BackendServices),
		APIRateLimiting:  u.rateLimitingDTOToModel(dto.APIRateLimiting),
		Operations:       u.operationsDTOToModel(dto.Operations),
	}
}

func (u *APIUtil) ModelToDTO(model *model.API) *dto.API {
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
		OrganizationID:   model.OrganizationID,
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
		MTLS:             u.mtlsModelToDTO(model.MTLS),
		Security:         u.securityModelToDTO(model.Security),
		CORS:             u.corsModelToDTO(model.CORS),
		BackendServices:  u.backendServicesModelToDTO(model.BackendServices),
		APIRateLimiting:  u.rateLimitingModelToDTO(model.APIRateLimiting),
		Operations:       u.operationsModelToDTO(model.Operations),
	}
}

// Helper DTO to Model conversion methods

func (u *APIUtil) mtlsDTOToModel(dto *dto.MTLSConfig) *model.MTLSConfig {
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

func (u *APIUtil) securityDTOToModel(dto *dto.SecurityConfig) *model.SecurityConfig {
	if dto == nil {
		return nil
	}
	return &model.SecurityConfig{
		Enabled: dto.Enabled,
		APIKey:  u.apiKeyDTOToModel(dto.APIKey),
		OAuth2:  u.oauth2DTOToModel(dto.OAuth2),
	}
}

func (u *APIUtil) apiKeyDTOToModel(dto *dto.APIKeySecurity) *model.APIKeySecurity {
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

func (u *APIUtil) oauth2DTOToModel(dto *dto.OAuth2Security) *model.OAuth2Security {
	if dto == nil {
		return nil
	}
	return &model.OAuth2Security{
		GrantTypes: u.oauth2GrantTypesDTOToModel(dto.GrantTypes),
		Scopes:     dto.Scopes,
	}
}

func (u *APIUtil) oauth2GrantTypesDTOToModel(dto *dto.OAuth2GrantTypes) *model.OAuth2GrantTypes {
	if dto == nil {
		return nil
	}
	return &model.OAuth2GrantTypes{
		AuthorizationCode: u.authCodeGrantDTOToModel(dto.AuthorizationCode),
		Implicit:          u.implicitGrantDTOToModel(dto.Implicit),
		Password:          u.passwordGrantDTOToModel(dto.Password),
		ClientCredentials: u.clientCredGrantDTOToModel(dto.ClientCredentials),
	}
}

func (u *APIUtil) authCodeGrantDTOToModel(dto *dto.AuthorizationCodeGrant) *model.AuthorizationCodeGrant {
	if dto == nil {
		return nil
	}
	return &model.AuthorizationCodeGrant{
		Enabled:     dto.Enabled,
		CallbackURL: dto.CallbackURL,
	}
}

func (u *APIUtil) implicitGrantDTOToModel(dto *dto.ImplicitGrant) *model.ImplicitGrant {
	if dto == nil {
		return nil
	}
	return &model.ImplicitGrant{
		Enabled:     dto.Enabled,
		CallbackURL: dto.CallbackURL,
	}
}

func (u *APIUtil) passwordGrantDTOToModel(dto *dto.PasswordGrant) *model.PasswordGrant {
	if dto == nil {
		return nil
	}
	return &model.PasswordGrant{
		Enabled: dto.Enabled,
	}
}

func (u *APIUtil) clientCredGrantDTOToModel(dto *dto.ClientCredentialsGrant) *model.ClientCredentialsGrant {
	if dto == nil {
		return nil
	}
	return &model.ClientCredentialsGrant{
		Enabled: dto.Enabled,
	}
}

func (u *APIUtil) corsDTOToModel(dto *dto.CORSConfig) *model.CORSConfig {
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

func (u *APIUtil) backendServicesDTOToModel(dtos []dto.BackendService) []model.BackendService {
	if dtos == nil {
		return nil
	}
	backendServiceModels := make([]model.BackendService, 0)
	for _, backendServiceDTO := range dtos {
		backendServiceModels = append(backendServiceModels, *u.backendServiceDTOToModel(&backendServiceDTO))
	}
	return backendServiceModels
}

func (u *APIUtil) backendServiceDTOToModel(dto *dto.BackendService) *model.BackendService {
	if dto == nil {
		return nil
	}
	return &model.BackendService{
		Name:           dto.Name,
		IsDefault:      dto.IsDefault,
		Endpoints:      u.backendEndpointsDTOToModel(dto.Endpoints),
		Timeout:        u.timeoutDTOToModel(dto.Timeout),
		Retries:        dto.Retries,
		LoadBalance:    u.loadBalanceDTOToModel(dto.LoadBalance),
		CircuitBreaker: u.circuitBreakerDTOToModel(dto.CircuitBreaker),
	}
}

func (u *APIUtil) backendEndpointsDTOToModel(dtos []dto.BackendEndpoint) []model.BackendEndpoint {
	if dtos == nil {
		return nil
	}
	backendEndpointModels := make([]model.BackendEndpoint, 0)
	for _, backendEndpointDTO := range dtos {
		backendEndpointModels = append(backendEndpointModels, *u.backendEndpointDTOToModel(&backendEndpointDTO))
	}
	return backendEndpointModels
}

func (u *APIUtil) backendEndpointDTOToModel(dto *dto.BackendEndpoint) *model.BackendEndpoint {
	if dto == nil {
		return nil
	}
	return &model.BackendEndpoint{
		URL:         dto.URL,
		Description: dto.Description,
		HealthCheck: u.healthCheckDTOToModel(dto.HealthCheck),
		Weight:      dto.Weight,
		MTLS:        u.mtlsDTOToModel(dto.MTLS),
	}
}

func (u *APIUtil) healthCheckDTOToModel(dto *dto.HealthCheckConfig) *model.HealthCheckConfig {
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

func (u *APIUtil) timeoutDTOToModel(dto *dto.TimeoutConfig) *model.TimeoutConfig {
	if dto == nil {
		return nil
	}
	return &model.TimeoutConfig{
		Connect: dto.Connect,
		Read:    dto.Read,
		Write:   dto.Write,
	}
}

func (u *APIUtil) loadBalanceDTOToModel(dto *dto.LoadBalanceConfig) *model.LoadBalanceConfig {
	if dto == nil {
		return nil
	}
	return &model.LoadBalanceConfig{
		Algorithm: dto.Algorithm,
		Failover:  dto.Failover,
	}
}

func (u *APIUtil) circuitBreakerDTOToModel(dto *dto.CircuitBreakerConfig) *model.CircuitBreakerConfig {
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

func (u *APIUtil) rateLimitingDTOToModel(dto *dto.RateLimitingConfig) *model.RateLimitingConfig {
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

func (u *APIUtil) operationsDTOToModel(dtos []dto.Operation) []model.Operation {
	if dtos == nil {
		return nil
	}
	operationsModels := make([]model.Operation, 0)
	for _, operationsDTO := range dtos {
		operationsModels = append(operationsModels, *u.operationDTOToModel(&operationsDTO))
	}
	return operationsModels
}

func (u *APIUtil) operationDTOToModel(dto *dto.Operation) *model.Operation {
	if dto == nil {
		return nil
	}
	return &model.Operation{
		Name:        dto.Name,
		Description: dto.Description,
		Request:     u.operationRequestDTOToModel(dto.Request),
	}
}

func (u *APIUtil) operationRequestDTOToModel(dto *dto.OperationRequest) *model.OperationRequest {
	if dto == nil {
		return nil
	}
	return &model.OperationRequest{
		Method:           dto.Method,
		Path:             dto.Path,
		BackendServices:  u.backendRoutingDTOsToModel(dto.BackendServices),
		Authentication:   u.authConfigDTOToModel(dto.Authentication),
		RequestPolicies:  u.policiesDTOToModel(dto.RequestPolicies),
		ResponsePolicies: u.policiesDTOToModel(dto.ResponsePolicies),
	}
}

func (u *APIUtil) backendRoutingDTOsToModel(dtos []dto.BackendRouting) []model.BackendRouting {
	if dtos == nil {
		return nil
	}
	backendRoutingModels := make([]model.BackendRouting, 0)
	for _, operationsDTO := range dtos {
		backendRoutingModels = append(backendRoutingModels, *u.backendRoutingDTOToModel(&operationsDTO))
	}
	return backendRoutingModels
}

func (u *APIUtil) backendRoutingDTOToModel(dto *dto.BackendRouting) *model.BackendRouting {
	if dto == nil {
		return nil
	}
	return &model.BackendRouting{
		Name:   dto.Name,
		Weight: dto.Weight,
	}
}

func (u *APIUtil) authConfigDTOToModel(dto *dto.AuthenticationConfig) *model.AuthenticationConfig {
	if dto == nil {
		return nil
	}
	return &model.AuthenticationConfig{
		Required: dto.Required,
		Scopes:   dto.Scopes,
	}
}

func (u *APIUtil) policiesDTOToModel(dtos []dto.Policy) []model.Policy {
	if dtos == nil {
		return nil
	}
	policyModels := make([]model.Policy, 0)
	for _, policyDTO := range dtos {
		policyModels = append(policyModels, *u.policyDTOToModel(&policyDTO))
	}
	return policyModels
}

func (u *APIUtil) policyDTOToModel(dto *dto.Policy) *model.Policy {
	if dto == nil {
		return nil
	}
	return &model.Policy{
		Name:   dto.Name,
		Params: dto.Params,
	}
}

// Helper Model to DTO conversion methods

func (u *APIUtil) mtlsModelToDTO(model *model.MTLSConfig) *dto.MTLSConfig {
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

func (u *APIUtil) securityModelToDTO(model *model.SecurityConfig) *dto.SecurityConfig {
	if model == nil {
		return nil
	}
	return &dto.SecurityConfig{
		Enabled: model.Enabled,
		APIKey:  u.apiKeyModelToDTO(model.APIKey),
		OAuth2:  u.oauth2ModelToDTO(model.OAuth2),
	}
}

func (u *APIUtil) apiKeyModelToDTO(model *model.APIKeySecurity) *dto.APIKeySecurity {
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

func (u *APIUtil) oauth2ModelToDTO(model *model.OAuth2Security) *dto.OAuth2Security {
	if model == nil {
		return nil
	}
	return &dto.OAuth2Security{
		GrantTypes: u.oauth2GrantTypesModelToDTO(model.GrantTypes),
		Scopes:     model.Scopes,
	}
}

func (u *APIUtil) oauth2GrantTypesModelToDTO(model *model.OAuth2GrantTypes) *dto.OAuth2GrantTypes {
	if model == nil {
		return nil
	}
	return &dto.OAuth2GrantTypes{
		AuthorizationCode: u.authCodeGrantModelToDTO(model.AuthorizationCode),
		Implicit:          u.implicitGrantModelToDTO(model.Implicit),
		Password:          u.passwordGrantModelToDTO(model.Password),
		ClientCredentials: u.clientCredGrantModelToDTO(model.ClientCredentials),
	}
}

func (u *APIUtil) authCodeGrantModelToDTO(model *model.AuthorizationCodeGrant) *dto.AuthorizationCodeGrant {
	if model == nil {
		return nil
	}
	return &dto.AuthorizationCodeGrant{
		Enabled:     model.Enabled,
		CallbackURL: model.CallbackURL,
	}
}

func (u *APIUtil) implicitGrantModelToDTO(model *model.ImplicitGrant) *dto.ImplicitGrant {
	if model == nil {
		return nil
	}
	return &dto.ImplicitGrant{
		Enabled:     model.Enabled,
		CallbackURL: model.CallbackURL,
	}
}

func (u *APIUtil) passwordGrantModelToDTO(model *model.PasswordGrant) *dto.PasswordGrant {
	if model == nil {
		return nil
	}
	return &dto.PasswordGrant{
		Enabled: model.Enabled,
	}
}

func (u *APIUtil) clientCredGrantModelToDTO(model *model.ClientCredentialsGrant) *dto.ClientCredentialsGrant {
	if model == nil {
		return nil
	}
	return &dto.ClientCredentialsGrant{
		Enabled: model.Enabled,
	}
}

func (u *APIUtil) corsModelToDTO(model *model.CORSConfig) *dto.CORSConfig {
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

func (u *APIUtil) backendServicesModelToDTO(models []model.BackendService) []dto.BackendService {
	if models == nil {
		return nil
	}
	backendServiceDTOs := make([]dto.BackendService, 0)
	for _, backendServiceModel := range models {
		backendServiceDTOs = append(backendServiceDTOs, *u.backendServiceModelToDTO(&backendServiceModel))
	}
	return backendServiceDTOs
}

func (u *APIUtil) backendServiceModelToDTO(model *model.BackendService) *dto.BackendService {
	if model == nil {
		return nil
	}
	return &dto.BackendService{
		Name:           model.Name,
		IsDefault:      model.IsDefault,
		Endpoints:      u.backendEndpointsModelToDTO(model.Endpoints),
		Timeout:        u.timeoutModelToDTO(model.Timeout),
		Retries:        model.Retries,
		LoadBalance:    u.loadBalanceModelToDTO(model.LoadBalance),
		CircuitBreaker: u.circuitBreakerModelToDTO(model.CircuitBreaker),
	}
}

func (u *APIUtil) backendEndpointsModelToDTO(models []model.BackendEndpoint) []dto.BackendEndpoint {
	if models == nil {
		return nil
	}
	backendEndpointDTOs := make([]dto.BackendEndpoint, 0)
	for _, backendServiceModel := range models {
		backendEndpointDTOs = append(backendEndpointDTOs, *u.backendEndpointModelToDTO(&backendServiceModel))
	}
	return backendEndpointDTOs
}

func (u *APIUtil) backendEndpointModelToDTO(model *model.BackendEndpoint) *dto.BackendEndpoint {
	if model == nil {
		return nil
	}
	return &dto.BackendEndpoint{
		URL:         model.URL,
		Description: model.Description,
		HealthCheck: u.healthCheckModelToDTO(model.HealthCheck),
		Weight:      model.Weight,
		MTLS:        u.mtlsModelToDTO(model.MTLS),
	}
}

func (u *APIUtil) healthCheckModelToDTO(model *model.HealthCheckConfig) *dto.HealthCheckConfig {
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

func (u *APIUtil) timeoutModelToDTO(model *model.TimeoutConfig) *dto.TimeoutConfig {
	if model == nil {
		return nil
	}
	return &dto.TimeoutConfig{
		Connect: model.Connect,
		Read:    model.Read,
		Write:   model.Write,
	}
}

func (u *APIUtil) loadBalanceModelToDTO(model *model.LoadBalanceConfig) *dto.LoadBalanceConfig {
	if model == nil {
		return nil
	}
	return &dto.LoadBalanceConfig{
		Algorithm: model.Algorithm,
		Failover:  model.Failover,
	}
}

func (u *APIUtil) circuitBreakerModelToDTO(model *model.CircuitBreakerConfig) *dto.CircuitBreakerConfig {
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

func (u *APIUtil) rateLimitingModelToDTO(model *model.RateLimitingConfig) *dto.RateLimitingConfig {
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

func (u *APIUtil) operationsModelToDTO(models []model.Operation) []dto.Operation {
	if models == nil {
		return nil
	}
	operationsDTOs := make([]dto.Operation, 0)
	for _, operationsModel := range models {
		operationsDTOs = append(operationsDTOs, *u.operationModelToDTO(&operationsModel))
	}
	return operationsDTOs
}

func (u *APIUtil) operationModelToDTO(model *model.Operation) *dto.Operation {
	if model == nil {
		return nil
	}
	return &dto.Operation{
		Name:        model.Name,
		Description: model.Description,
		Request:     u.operationRequestModelToDTO(model.Request),
	}
}

func (u *APIUtil) operationRequestModelToDTO(model *model.OperationRequest) *dto.OperationRequest {
	if model == nil {
		return nil
	}
	return &dto.OperationRequest{
		Method:           model.Method,
		Path:             model.Path,
		BackendServices:  u.backendRoutingModelsToDTO(model.BackendServices),
		Authentication:   u.authConfigModelToDTO(model.Authentication),
		RequestPolicies:  u.policiesModelToDTO(model.RequestPolicies),
		ResponsePolicies: u.policiesModelToDTO(model.ResponsePolicies),
	}
}

func (u *APIUtil) backendRoutingModelsToDTO(models []model.BackendRouting) []dto.BackendRouting {
	if models == nil {
		return nil
	}
	backendRoutingDTOs := make([]dto.BackendRouting, 0)
	for _, backendRoutingModel := range models {
		backendRoutingDTOs = append(backendRoutingDTOs, *u.backendRoutingModelToDTO(&backendRoutingModel))
	}
	return backendRoutingDTOs
}

func (u *APIUtil) backendRoutingModelToDTO(model *model.BackendRouting) *dto.BackendRouting {
	if model == nil {
		return nil
	}
	return &dto.BackendRouting{
		Name:   model.Name,
		Weight: model.Weight,
	}
}

func (u *APIUtil) authConfigModelToDTO(model *model.AuthenticationConfig) *dto.AuthenticationConfig {
	if model == nil {
		return nil
	}
	return &dto.AuthenticationConfig{
		Required: model.Required,
		Scopes:   model.Scopes,
	}
}

func (u *APIUtil) policiesModelToDTO(models []model.Policy) []dto.Policy {
	if models == nil {
		return nil
	}
	policyDTOs := make([]dto.Policy, 0)
	for _, policyModel := range models {
		policyDTOs = append(policyDTOs, *u.policyModelToDTO(&policyModel))
	}
	return policyDTOs
}

func (u *APIUtil) policyModelToDTO(model *model.Policy) *dto.Policy {
	if model == nil {
		return nil
	}
	return &dto.Policy{
		Name:   model.Name,
		Params: model.Params,
	}
}

// GenerateAPIDeploymentYAML creates the deployment YAML from API data
func (u *APIUtil) GenerateAPIDeploymentYAML(api *dto.API) (string, error) {
	// Create API deployment YAML structure
	apiYAMLData := dto.APIYAMLData{
		Id:              api.ID,
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
		Kind:    "http/rest",
		Version: "api-platform.wso2.com/v1",
		Data:    apiYAMLData,
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(apiDeployment)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API to YAML: %w", err)
	}

	return string(yamlBytes), nil
}
