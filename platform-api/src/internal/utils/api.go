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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"platform-api/src/internal/client/devportal_client"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"

	"github.com/pb33f/libopenapi"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"gopkg.in/yaml.v3"
)

type APIUtil struct{}

// Mapping functions
// DTOToModel converts a DTO API to a Model API
// Note: DTO.ID maps to Model.Handle (user-facing identifier)
// The internal Model.ID (UUID) should be set separately by the caller
func (u *APIUtil) DTOToModel(dto *dto.API) *model.API {
	if dto == nil {
		return nil
	}

	return &model.API{
		Handle:           dto.ID, // DTO.ID is the handle (user-facing identifier)
		Name:             dto.Name,
		Description:      dto.Description,
		Context:          dto.Context,
		Version:          dto.Version,
		Provider:         dto.Provider,
		ProjectID:        dto.ProjectID,
		OrganizationID:   dto.OrganizationID,
		LifeCycleStatus:  dto.LifeCycleStatus,
		HasThumbnail:     dto.HasThumbnail,
		IsDefaultVersion: dto.IsDefaultVersion,
		Type:             dto.Type,
		Transport:        dto.Transport,
		MTLS:             u.MTLSDTOToModel(dto.MTLS),
		Security:         u.SecurityDTOToModel(dto.Security),
		CORS:             u.CORSDTOToModel(dto.CORS),
		BackendServices:  u.BackendServicesDTOToModel(dto.BackendServices),
		APIRateLimiting:  u.RateLimitingDTOToModel(dto.APIRateLimiting),
		Policies:         u.PoliciesDTOToModel(dto.Policies),
		Operations:       u.OperationsDTOToModel(dto.Operations),
		Channels:         u.ChannelsDTOToModel(dto.Channels),
	}
}

// ModelToDTO converts a Model API to a DTO API
// Note: Model.Handle maps to DTO.ID (user-facing identifier)
// The internal Model.ID (UUID) is not exposed in the DTO
func (u *APIUtil) ModelToDTO(model *model.API) *dto.API {
	if model == nil {
		return nil
	}

	return &dto.API{
		ID:               model.Handle, // Model.Handle is exposed as DTO.ID
		Name:             model.Name,
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
		Type:             model.Type,
		Transport:        model.Transport,
		MTLS:             u.MTLSModelToDTO(model.MTLS),
		Security:         u.SecurityModelToDTO(model.Security),
		CORS:             u.CORSModelToDTO(model.CORS),
		BackendServices:  u.BackendServicesModelToDTO(model.BackendServices),
		APIRateLimiting:  u.RateLimitingModelToDTO(model.APIRateLimiting),
		Policies:         u.PoliciesModelToDTO(model.Policies),
		Operations:       u.OperationsModelToDTO(model.Operations),
		Channels:         u.ChannelsModelToDTO(model.Channels),
	}
}

// Helper DTO to Model conversion methods

func (u *APIUtil) MTLSDTOToModel(dto *dto.MTLSConfig) *model.MTLSConfig {
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

func (u *APIUtil) SecurityDTOToModel(dto *dto.SecurityConfig) *model.SecurityConfig {
	if dto == nil {
		return nil
	}
	return &model.SecurityConfig{
		Enabled:       dto.Enabled,
		APIKey:        u.APIKeyDTOToModel(dto.APIKey),
		OAuth2:        u.OAuth2DTOToModel(dto.OAuth2),
		XHubSignature: u.XHubSignatureDTOToModel(dto.XHubSignature),
	}
}

func (u *APIUtil) XHubSignatureDTOToModel(dto *dto.XHubSignatureSecurity) *model.XHubSignatureSecurity {
	if dto == nil {
		return nil
	}
	return &model.XHubSignatureSecurity{
		Enabled:   dto.Enabled,
		Header:    dto.Header,
		Secret:    dto.Secret,
		Algorithm: dto.Algorithm,
	}
}

func (u *APIUtil) APIKeyDTOToModel(dto *dto.APIKeySecurity) *model.APIKeySecurity {
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

func (u *APIUtil) OAuth2DTOToModel(dto *dto.OAuth2Security) *model.OAuth2Security {
	if dto == nil {
		return nil
	}
	return &model.OAuth2Security{
		GrantTypes: u.OAuth2GrantTypesDTOToModel(dto.GrantTypes),
		Scopes:     dto.Scopes,
	}
}

func (u *APIUtil) OAuth2GrantTypesDTOToModel(dto *dto.OAuth2GrantTypes) *model.OAuth2GrantTypes {
	if dto == nil {
		return nil
	}
	return &model.OAuth2GrantTypes{
		AuthorizationCode: u.AuthCodeGrantDTOToModel(dto.AuthorizationCode),
		Implicit:          u.ImplicitGrantDTOToModel(dto.Implicit),
		Password:          u.PasswordGrantDTOToModel(dto.Password),
		ClientCredentials: u.ClientCredGrantDTOToModel(dto.ClientCredentials),
	}
}

func (u *APIUtil) AuthCodeGrantDTOToModel(dto *dto.AuthorizationCodeGrant) *model.AuthorizationCodeGrant {
	if dto == nil {
		return nil
	}
	return &model.AuthorizationCodeGrant{
		Enabled:     dto.Enabled,
		CallbackURL: dto.CallbackURL,
	}
}

func (u *APIUtil) ImplicitGrantDTOToModel(dto *dto.ImplicitGrant) *model.ImplicitGrant {
	if dto == nil {
		return nil
	}
	return &model.ImplicitGrant{
		Enabled:     dto.Enabled,
		CallbackURL: dto.CallbackURL,
	}
}

func (u *APIUtil) PasswordGrantDTOToModel(dto *dto.PasswordGrant) *model.PasswordGrant {
	if dto == nil {
		return nil
	}
	return &model.PasswordGrant{
		Enabled: dto.Enabled,
	}
}

func (u *APIUtil) ClientCredGrantDTOToModel(dto *dto.ClientCredentialsGrant) *model.ClientCredentialsGrant {
	if dto == nil {
		return nil
	}
	return &model.ClientCredentialsGrant{
		Enabled: dto.Enabled,
	}
}

func (u *APIUtil) CORSDTOToModel(dto *dto.CORSConfig) *model.CORSConfig {
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

func (u *APIUtil) BackendServicesDTOToModel(dtos []dto.BackendService) []model.BackendService {
	if dtos == nil {
		return nil
	}
	backendServiceModels := make([]model.BackendService, 0)
	for _, backendServiceDTO := range dtos {
		backendServiceModels = append(backendServiceModels, *u.BackendServiceDTOToModel(&backendServiceDTO))
	}
	return backendServiceModels
}

func (u *APIUtil) BackendServiceDTOToModel(dto *dto.BackendService) *model.BackendService {
	if dto == nil {
		return nil
	}
	return &model.BackendService{
		Name:           dto.Name,
		Endpoints:      u.BackendEndpointsDTOToModel(dto.Endpoints),
		Timeout:        u.TimeoutDTOToModel(dto.Timeout),
		Retries:        dto.Retries,
		LoadBalance:    u.LoadBalanceDTOToModel(dto.LoadBalance),
		CircuitBreaker: u.CircuitBreakerDTOToModel(dto.CircuitBreaker),
	}
}

func (u *APIUtil) BackendEndpointsDTOToModel(dtos []dto.BackendEndpoint) []model.BackendEndpoint {
	if dtos == nil {
		return nil
	}
	backendEndpointModels := make([]model.BackendEndpoint, 0)
	for _, backendEndpointDTO := range dtos {
		backendEndpointModels = append(backendEndpointModels, *u.BackendEndpointDTOToModel(&backendEndpointDTO))
	}
	return backendEndpointModels
}

func (u *APIUtil) BackendEndpointDTOToModel(dto *dto.BackendEndpoint) *model.BackendEndpoint {
	if dto == nil {
		return nil
	}
	return &model.BackendEndpoint{
		URL:         dto.URL,
		Description: dto.Description,
		HealthCheck: u.HealthCheckDTOToModel(dto.HealthCheck),
		Weight:      dto.Weight,
		MTLS:        u.MTLSDTOToModel(dto.MTLS),
	}
}

func (u *APIUtil) HealthCheckDTOToModel(dto *dto.HealthCheckConfig) *model.HealthCheckConfig {
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

func (u *APIUtil) TimeoutDTOToModel(dto *dto.TimeoutConfig) *model.TimeoutConfig {
	if dto == nil {
		return nil
	}
	return &model.TimeoutConfig{
		Connect: dto.Connect,
		Read:    dto.Read,
		Write:   dto.Write,
	}
}

func (u *APIUtil) LoadBalanceDTOToModel(dto *dto.LoadBalanceConfig) *model.LoadBalanceConfig {
	if dto == nil {
		return nil
	}
	return &model.LoadBalanceConfig{
		Algorithm: dto.Algorithm,
		Failover:  dto.Failover,
	}
}

func (u *APIUtil) CircuitBreakerDTOToModel(dto *dto.CircuitBreakerConfig) *model.CircuitBreakerConfig {
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

func (u *APIUtil) RateLimitingDTOToModel(dto *dto.RateLimitingConfig) *model.RateLimitingConfig {
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

func (u *APIUtil) OperationsDTOToModel(dtos []dto.Operation) []model.Operation {
	if dtos == nil {
		return nil
	}
	operationsModels := make([]model.Operation, 0)
	for _, operationsDTO := range dtos {
		operationsModels = append(operationsModels, *u.OperationDTOToModel(&operationsDTO))
	}
	return operationsModels
}

func (u *APIUtil) ChannelsDTOToModel(dtos []dto.Channel) []model.Channel {
	if dtos == nil {
		return nil
	}
	channelsModels := make([]model.Channel, 0)
	for _, channelDTO := range dtos {
		channelsModels = append(channelsModels, *u.ChannelDTOToModel(&channelDTO))
	}
	return channelsModels
}

func (u *APIUtil) ChannelDTOToModel(dto *dto.Channel) *model.Channel {
	if dto == nil {
		return nil
	}
	return &model.Channel{
		Name:        dto.Name,
		Description: dto.Description,
		Request:     u.ChannelRequestDTOToModel(dto.Request),
	}
}

func (u *APIUtil) OperationDTOToModel(dto *dto.Operation) *model.Operation {
	if dto == nil {
		return nil
	}
	return &model.Operation{
		Name:        dto.Name,
		Description: dto.Description,
		Request:     u.OperationRequestDTOToModel(dto.Request),
	}
}

func (u *APIUtil) OperationRequestDTOToModel(dto *dto.OperationRequest) *model.OperationRequest {
	if dto == nil {
		return nil
	}
	return &model.OperationRequest{
		Method:          dto.Method,
		Path:            dto.Path,
		BackendServices: u.BackendRoutingDTOsToModel(dto.BackendServices),
		Authentication:  u.AuthConfigDTOToModel(dto.Authentication),
		Policies:        u.PoliciesDTOToModel(dto.Policies),
	}
}

func (u *APIUtil) ChannelRequestDTOToModel(dto *dto.ChannelRequest) *model.ChannelRequest {
	if dto == nil {
		return nil
	}
	return &model.ChannelRequest{
		Method:          dto.Method,
		Name:            dto.Name,
		BackendServices: u.BackendRoutingDTOsToModel(dto.BackendServices),
		Authentication:  u.AuthConfigDTOToModel(dto.Authentication),
		Policies:        u.PoliciesDTOToModel(dto.Policies),
	}
}

func (u *APIUtil) BackendRoutingDTOsToModel(dtos []dto.BackendRouting) []model.BackendRouting {
	if dtos == nil {
		return nil
	}
	backendRoutingModels := make([]model.BackendRouting, 0)
	for _, operationsDTO := range dtos {
		backendRoutingModels = append(backendRoutingModels, *u.BackendRoutingDTOToModel(&operationsDTO))
	}
	return backendRoutingModels
}

func (u *APIUtil) BackendRoutingDTOToModel(dto *dto.BackendRouting) *model.BackendRouting {
	if dto == nil {
		return nil
	}
	return &model.BackendRouting{
		Name:   dto.Name,
		Weight: dto.Weight,
	}
}

func (u *APIUtil) AuthConfigDTOToModel(dto *dto.AuthenticationConfig) *model.AuthenticationConfig {
	if dto == nil {
		return nil
	}
	return &model.AuthenticationConfig{
		Required: dto.Required,
		Scopes:   dto.Scopes,
	}
}

func (u *APIUtil) PoliciesDTOToModel(dtos []dto.Policy) []model.Policy {
	if dtos == nil {
		return nil
	}
	policyModels := make([]model.Policy, 0)
	for _, policyDTO := range dtos {
		policyModels = append(policyModels, *u.PolicyDTOToModel(&policyDTO))
	}
	return policyModels
}

func (u *APIUtil) PolicyDTOToModel(dto *dto.Policy) *model.Policy {
	if dto == nil {
		return nil
	}
	return &model.Policy{
		ExecutionCondition: dto.ExecutionCondition,
		Name:               dto.Name,
		Params:             dto.Params,
		Version:            dto.Version,
	}
}

// Helper Model to DTO conversion methods

func (u *APIUtil) MTLSModelToDTO(model *model.MTLSConfig) *dto.MTLSConfig {
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

func (u *APIUtil) SecurityModelToDTO(model *model.SecurityConfig) *dto.SecurityConfig {
	if model == nil {
		return nil
	}
	return &dto.SecurityConfig{
		Enabled:       model.Enabled,
		APIKey:        u.APIKeyModelToDTO(model.APIKey),
		OAuth2:        u.OAuth2ModelToDTO(model.OAuth2),
		XHubSignature: u.XHubSignatureModelToDTO(model.XHubSignature),
	}
}

func (u *APIUtil) XHubSignatureModelToDTO(model *model.XHubSignatureSecurity) *dto.XHubSignatureSecurity {
	if model == nil {
		return nil
	}
	return &dto.XHubSignatureSecurity{
		Enabled:   model.Enabled,
		Header:    model.Header,
		Secret:    model.Secret,
		Algorithm: model.Algorithm,
	}
}

func (u *APIUtil) APIKeyModelToDTO(model *model.APIKeySecurity) *dto.APIKeySecurity {
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

func (u *APIUtil) OAuth2ModelToDTO(model *model.OAuth2Security) *dto.OAuth2Security {
	if model == nil {
		return nil
	}
	return &dto.OAuth2Security{
		GrantTypes: u.OAuth2GrantTypesModelToDTO(model.GrantTypes),
		Scopes:     model.Scopes,
	}
}

func (u *APIUtil) OAuth2GrantTypesModelToDTO(model *model.OAuth2GrantTypes) *dto.OAuth2GrantTypes {
	if model == nil {
		return nil
	}
	return &dto.OAuth2GrantTypes{
		AuthorizationCode: u.AuthCodeGrantModelToDTO(model.AuthorizationCode),
		Implicit:          u.ImplicitGrantModelToDTO(model.Implicit),
		Password:          u.PasswordGrantModelToDTO(model.Password),
		ClientCredentials: u.ClientCredGrantModelToDTO(model.ClientCredentials),
	}
}

func (u *APIUtil) AuthCodeGrantModelToDTO(model *model.AuthorizationCodeGrant) *dto.AuthorizationCodeGrant {
	if model == nil {
		return nil
	}
	return &dto.AuthorizationCodeGrant{
		Enabled:     model.Enabled,
		CallbackURL: model.CallbackURL,
	}
}

func (u *APIUtil) ImplicitGrantModelToDTO(model *model.ImplicitGrant) *dto.ImplicitGrant {
	if model == nil {
		return nil
	}
	return &dto.ImplicitGrant{
		Enabled:     model.Enabled,
		CallbackURL: model.CallbackURL,
	}
}

func (u *APIUtil) PasswordGrantModelToDTO(model *model.PasswordGrant) *dto.PasswordGrant {
	if model == nil {
		return nil
	}
	return &dto.PasswordGrant{
		Enabled: model.Enabled,
	}
}

func (u *APIUtil) ClientCredGrantModelToDTO(model *model.ClientCredentialsGrant) *dto.ClientCredentialsGrant {
	if model == nil {
		return nil
	}
	return &dto.ClientCredentialsGrant{
		Enabled: model.Enabled,
	}
}

func (u *APIUtil) CORSModelToDTO(model *model.CORSConfig) *dto.CORSConfig {
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

func (u *APIUtil) BackendServicesModelToDTO(models []model.BackendService) []dto.BackendService {
	if models == nil {
		return nil
	}
	backendServiceDTOs := make([]dto.BackendService, 0)
	for _, backendServiceModel := range models {
		backendServiceDTOs = append(backendServiceDTOs, *u.BackendServiceModelToDTO(&backendServiceModel))
	}
	return backendServiceDTOs
}

func (u *APIUtil) BackendServiceModelToDTO(model *model.BackendService) *dto.BackendService {
	if model == nil {
		return nil
	}
	return &dto.BackendService{
		Name:           model.Name,
		Endpoints:      u.BackendEndpointsModelToDTO(model.Endpoints),
		Timeout:        u.TimeoutModelToDTO(model.Timeout),
		Retries:        model.Retries,
		LoadBalance:    u.LoadBalanceModelToDTO(model.LoadBalance),
		CircuitBreaker: u.CircuitBreakerModelToDTO(model.CircuitBreaker),
	}
}

func (u *APIUtil) BackendEndpointsModelToDTO(models []model.BackendEndpoint) []dto.BackendEndpoint {
	if models == nil {
		return nil
	}
	backendEndpointDTOs := make([]dto.BackendEndpoint, 0)
	for _, backendServiceModel := range models {
		backendEndpointDTOs = append(backendEndpointDTOs, *u.BackendEndpointModelToDTO(&backendServiceModel))
	}
	return backendEndpointDTOs
}

func (u *APIUtil) BackendEndpointModelToDTO(model *model.BackendEndpoint) *dto.BackendEndpoint {
	if model == nil {
		return nil
	}
	return &dto.BackendEndpoint{
		URL:         model.URL,
		Description: model.Description,
		HealthCheck: u.HealthCheckModelToDTO(model.HealthCheck),
		Weight:      model.Weight,
		MTLS:        u.MTLSModelToDTO(model.MTLS),
	}
}

func (u *APIUtil) HealthCheckModelToDTO(model *model.HealthCheckConfig) *dto.HealthCheckConfig {
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

func (u *APIUtil) TimeoutModelToDTO(model *model.TimeoutConfig) *dto.TimeoutConfig {
	if model == nil {
		return nil
	}
	return &dto.TimeoutConfig{
		Connect: model.Connect,
		Read:    model.Read,
		Write:   model.Write,
	}
}

func (u *APIUtil) LoadBalanceModelToDTO(model *model.LoadBalanceConfig) *dto.LoadBalanceConfig {
	if model == nil {
		return nil
	}
	return &dto.LoadBalanceConfig{
		Algorithm: model.Algorithm,
		Failover:  model.Failover,
	}
}

func (u *APIUtil) CircuitBreakerModelToDTO(model *model.CircuitBreakerConfig) *dto.CircuitBreakerConfig {
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

func (u *APIUtil) RateLimitingModelToDTO(model *model.RateLimitingConfig) *dto.RateLimitingConfig {
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

func (u *APIUtil) OperationsModelToDTO(models []model.Operation) []dto.Operation {
	if models == nil {
		return nil
	}
	operationsDTOs := make([]dto.Operation, 0)
	for _, operationsModel := range models {
		operationsDTOs = append(operationsDTOs, *u.OperationModelToDTO(&operationsModel))
	}
	return operationsDTOs
}

func (u *APIUtil) ChannelsModelToDTO(models []model.Channel) []dto.Channel {
	if models == nil {
		return nil
	}
	channelsDTOs := make([]dto.Channel, 0)
	for _, channelModel := range models {
		channelsDTOs = append(channelsDTOs, *u.ChannelModelToDTO(&channelModel))
	}
	return channelsDTOs
}

func (u *APIUtil) OperationModelToDTO(model *model.Operation) *dto.Operation {
	if model == nil {
		return nil
	}
	return &dto.Operation{
		Name:        model.Name,
		Description: model.Description,
		Request:     u.OperationRequestModelToDTO(model.Request),
	}
}

func (u *APIUtil) ChannelModelToDTO(model *model.Channel) *dto.Channel {
	if model == nil {
		return nil
	}
	return &dto.Channel{
		Name:        model.Name,
		Description: model.Description,
		Request:     u.ChannelRequestModelToDTO(model.Request),
	}
}

func (u *APIUtil) ChannelRequestModelToDTO(model *model.ChannelRequest) *dto.ChannelRequest {
	if model == nil {
		return nil
	}
	return &dto.ChannelRequest{
		Method:          model.Method,
		Name:            model.Name,
		BackendServices: u.BackendRoutingModelsToDTO(model.BackendServices),
		Authentication:  u.AuthConfigModelToDTO(model.Authentication),
		Policies:        u.PoliciesModelToDTO(model.Policies),
	}
}

func (u *APIUtil) OperationRequestModelToDTO(model *model.OperationRequest) *dto.OperationRequest {
	if model == nil {
		return nil
	}
	return &dto.OperationRequest{
		Method:          model.Method,
		Path:            model.Path,
		BackendServices: u.BackendRoutingModelsToDTO(model.BackendServices),
		Authentication:  u.AuthConfigModelToDTO(model.Authentication),
		Policies:        u.PoliciesModelToDTO(model.Policies),
	}
}

func (u *APIUtil) BackendRoutingModelsToDTO(models []model.BackendRouting) []dto.BackendRouting {
	if models == nil {
		return nil
	}
	backendRoutingDTOs := make([]dto.BackendRouting, 0)
	for _, backendRoutingModel := range models {
		backendRoutingDTOs = append(backendRoutingDTOs, *u.BackendRoutingModelToDTO(&backendRoutingModel))
	}
	return backendRoutingDTOs
}

func (u *APIUtil) BackendRoutingModelToDTO(model *model.BackendRouting) *dto.BackendRouting {
	if model == nil {
		return nil
	}
	return &dto.BackendRouting{
		Name:   model.Name,
		Weight: model.Weight,
	}
}

func (u *APIUtil) AuthConfigModelToDTO(model *model.AuthenticationConfig) *dto.AuthenticationConfig {
	if model == nil {
		return nil
	}
	return &dto.AuthenticationConfig{
		Required: model.Required,
		Scopes:   model.Scopes,
	}
}

func (u *APIUtil) PoliciesModelToDTO(models []model.Policy) []dto.Policy {
	if models == nil {
		return nil
	}
	policyDTOs := make([]dto.Policy, 0)
	for _, policyModel := range models {
		policyDTOs = append(policyDTOs, *u.PolicyModelToDTO(&policyModel))
	}
	return policyDTOs
}

func (u *APIUtil) PolicyModelToDTO(model *model.Policy) *dto.Policy {
	if model == nil {
		return nil
	}
	return &dto.Policy{
		ExecutionCondition: model.ExecutionCondition,
		Name:               model.Name,
		Params:             model.Params,
		Version:            model.Version,
	}
}

// GetAPISubType determines the API subtype based on the API type using constants
func (u *APIUtil) GetAPISubType(apiType string) string {
	switch apiType {
	case constants.APITypeHTTP:
		return constants.APISubTypeHTTP
	case constants.APITypeGraphQL:
		return constants.APISubTypeGraphQL
	case constants.APITypeAsync, constants.APITypeWebSub, constants.APITypeSSE, constants.APITypeWebhook:
		return constants.APISubTypeAsync
	case constants.APITypeWS:
		return constants.APISubTypeWebSocket
	case constants.APITypeSOAP, constants.APITypeSOAPToREST:
		return constants.APISubTypeSOAP
	default:
		return constants.APISubTypeHTTP // Default to HTTP for unknown types
	}
}

// GenerateAPIDeploymentYAML creates the deployment YAML from API
func (u *APIUtil) GenerateAPIDeploymentYAML(api *dto.API) (string, error) {
	operationList := make([]dto.OperationRequest, 0)
	for _, op := range api.Operations {
		operationList = append(operationList, *op.Request)
	}
	channelList := make([]dto.ChannelRequest, 0)
	for _, ch := range api.Channels {
		channelList = append(channelList, *ch.Request)
	}

	// Get the main upstream URL from the first backend service endpoint
	var upstreamYAML *dto.UpstreamYAML
	for _, backendService := range api.BackendServices {
		for _, endpoint := range backendService.Endpoints {
			if endpoint.URL != "" {
				upstreamYAML = &dto.UpstreamYAML{
					Main: &dto.UpstreamTarget{
						URL: endpoint.URL,
					},
				}
				break
			}
		}
		if upstreamYAML != nil {
			break
		}
	}

	apiYAMLData := dto.APIYAMLData{}
	apiYAMLData.DisplayName = api.Name
	apiYAMLData.Version = api.Version
	apiYAMLData.Context = api.Context
	apiYAMLData.Policies = api.Policies

	// Only set upstream and operations for HTTP APIs
	switch api.Type {
	case constants.APITypeHTTP:
		apiYAMLData.Upstream = upstreamYAML
		apiYAMLData.Operations = operationList
	case constants.APITypeWebSub:
		apiYAMLData.Channels = channelList
	}

	// // Create API deployment YAML structure
	// apiYAMLData = dto.APIYAMLData{
	// 	DisplayName: api.Name,
	// 	Version:     api.Version,
	// 	Context:     api.Context,
	// 	Upstream:    upstreamYAML,
	// 	Operations:  operationList,
	// 	Channels:    channelList,
	// }

	apiType := ""
	switch api.Type {
	case constants.APITypeHTTP:
		apiType = "RestApi"
	case constants.APITypeWebSub:
		apiType = "WebSubApi"
	}

	apiDeployment := dto.APIDeploymentYAML{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       apiType,
		Metadata: dto.APIDeploymentMetadata{
			Name: api.ID,
			Labels: map[string]string{
				"project-id": api.ProjectID,
			},
		},
		Spec: apiYAMLData,
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(apiDeployment)
	if err != nil {
		return "", fmt.Errorf("failed to marshal API to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// TODO: Enhance GenerateOpenAPIDefinition to include request/response schemas, examples,
// detailed parameters, and complete security configurations from original OpenAPI sources
// to make the spec more useful for API consumers. Currently generates minimal spec
// with only available DTO data to avoid inventing information.
// GenerateOpenAPIDefinition generates an OpenAPI 3.0 definition from the API struct
func (u *APIUtil) GenerateOpenAPIDefinition(api *dto.API, req *devportal_client.APIMetadataRequest) ([]byte, error) {
	// Build the OpenAPI specification
	openAPISpec := dto.OpenAPI{
		OpenAPI: "3.0.3",
		Info:    u.buildInfoSection(api),
		Servers: u.buildServersSection(api, &req.EndPoints),
		Paths:   u.buildPathsSection(api),
	}

	// Add security schemes if configured
	if securitySchemes := u.buildSecuritySchemes(api); len(securitySchemes) > 0 {
		openAPISpec.Components = &dto.Components{
			SecuritySchemes: securitySchemes,
		}
	}

	// Add global security if configured
	if globalSecurity := u.buildGlobalSecurity(api); len(globalSecurity) > 0 {
		openAPISpec.Security = globalSecurity
	}

	// Marshal to JSON
	apiDefinition, err := json.Marshal(openAPISpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAPI definition: %w", err)
	}

	return apiDefinition, nil
}

// buildInfoSection creates the info section of the OpenAPI spec
func (u *APIUtil) buildInfoSection(api *dto.API) dto.Info {
	info := dto.Info{}

	if api.Name != "" {
		info.Title = api.Name
	}
	if api.Version != "" {
		info.Version = api.Version
	}
	if api.Description != "" {
		info.Description = api.Description
	}

	// Add contact info only if available
	if api.Provider != "" {
		info.Contact = &dto.Contact{
			Name: api.Provider,
		}
	}

	return info
}

// buildServersSection creates the servers section
func (u *APIUtil) buildServersSection(api *dto.API, endpoints *devportal_client.EndPoints) []dto.Server {
	var servers []dto.Server

	// Add production server if available
	if endpoints.ProductionURL != "" {
		prodURL := endpoints.ProductionURL
		if !strings.HasSuffix(prodURL, api.Context) {
			prodURL += api.Context
		}
		servers = append(servers, dto.Server{
			URL:         prodURL,
			Description: "Production server",
		})
	}

	// Add sandbox server if available
	if endpoints.SandboxURL != "" {
		sandboxURL := endpoints.SandboxURL
		if !strings.HasSuffix(sandboxURL, api.Context) {
			sandboxURL += api.Context
		}
		servers = append(servers, dto.Server{
			URL:         sandboxURL,
			Description: "Sandbox server",
		})
	}

	return servers
}

// buildPathsSection creates the paths section with detailed operations
func (u *APIUtil) buildPathsSection(api *dto.API) map[string]dto.PathItem {
	paths := make(map[string]dto.PathItem)

	for _, operation := range api.Operations {
		if operation.Request == nil {
			continue
		}

		path := operation.Request.Path
		method := strings.ToLower(operation.Request.Method)

		// Get or create PathItem
		pathItem, exists := paths[path]
		if !exists {
			pathItem = dto.PathItem{}
		}

		// Build operation details - only include available data
		operationSpec := &dto.OpenAPIOperation{
			Summary:     operation.Name,
			Description: operation.Description,
		}

		// Add parameters inferred from the path or authentication when present
		if parameters := u.buildParameters(path, operation, api); len(parameters) > 0 {
			operationSpec.Parameters = parameters
		}

		// Add security requirements only if operation has authentication configured
		if operationSecurity := u.buildOperationSecurity(operation, api); len(operationSecurity) > 0 {
			operationSpec.Security = operationSecurity
		}

		// Set the operation on the pathItem
		switch method {
		case "get":
			pathItem.Get = operationSpec
		case "post":
			pathItem.Post = operationSpec
		case "put":
			pathItem.Put = operationSpec
		case "delete":
			pathItem.Delete = operationSpec
		case "patch":
			pathItem.Patch = operationSpec
		case "options":
			pathItem.Options = operationSpec
		case "head":
			pathItem.Head = operationSpec
		case "trace":
			pathItem.Trace = operationSpec
		}

		paths[path] = pathItem
	}

	return paths
}

// buildParameters extracts path, query, and header parameters from the path
func (u *APIUtil) buildParameters(path string, operation dto.Operation, api *dto.API) []dto.Parameter {
	var parameters []dto.Parameter

	// Extract path parameters (e.g., {id} -> id)
	pathParamRegex := regexp.MustCompile(`\{([^}]+)\}`)
	matches := pathParamRegex.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		if len(match) > 1 {
			paramName := match[1]
			parameters = append(parameters, dto.Parameter{
				Name:        paramName,
				In:          "path",
				Required:    true,
				Schema:      dto.Schema{Type: "string"},
				Description: fmt.Sprintf("The %s parameter", paramName),
			})
		}
	}

	// Add common query parameters if applicable
	if operation.Request != nil {
		// Add authentication-related parameters
		if operation.Request.Authentication != nil && operation.Request.Authentication.Required {
			if api.Security != nil && api.Security.OAuth2 != nil && len(api.Security.OAuth2.Scopes) > 0 {
				// For OAuth2, add Authorization header
				parameters = append(parameters, dto.Parameter{
					Name:        "Authorization",
					In:          "header",
					Required:    true,
					Schema:      dto.Schema{Type: "string"},
					Description: "Bearer token for authentication",
				})
			} else if api.Security != nil && api.Security.APIKey != nil && api.Security.APIKey.Enabled {
				// For API Key, add the key parameter
				paramName := u.getAPIKeyName(api.Security.APIKey)
				in := u.getAPIKeyLocation(api.Security.APIKey)
				parameters = append(parameters, dto.Parameter{
					Name:        paramName,
					In:          in,
					Required:    true,
					Schema:      dto.Schema{Type: "string"},
					Description: "API key for authentication",
				})
			}
		}
	}

	return parameters
}

// buildSecuritySchemes creates security scheme definitions
func (u *APIUtil) buildSecuritySchemes(api *dto.API) map[string]dto.SecurityScheme {
	schemes := make(map[string]dto.SecurityScheme)

	if api.Security != nil {
		// API Key security
		if api.Security.APIKey != nil && api.Security.APIKey.Enabled {
			schemes["ApiKeyAuth"] = dto.SecurityScheme{
				Type: "apiKey",
				In:   u.getAPIKeyLocation(api.Security.APIKey),
				Name: u.getAPIKeyName(api.Security.APIKey),
			}
		}
	}

	return schemes
}

// getAPIKeyLocation determines where the API key is expected
func (u *APIUtil) getAPIKeyLocation(apiKey *dto.APIKeySecurity) string {
	if apiKey.Header != "" {
		return "header"
	}
	if apiKey.Query != "" {
		return "query"
	}
	if apiKey.Cookie != "" {
		return "cookie"
	}
	return "header" // default
}

// getAPIKeyName gets the API key parameter name
func (u *APIUtil) getAPIKeyName(apiKey *dto.APIKeySecurity) string {
	if apiKey.Header != "" {
		return apiKey.Header
	}
	if apiKey.Query != "" {
		return apiKey.Query
	}
	if apiKey.Cookie != "" {
		return apiKey.Cookie
	}
	return "X-API-Key" // default
}

// buildGlobalSecurity creates global security requirements
func (u *APIUtil) buildGlobalSecurity(api *dto.API) []dto.SecurityRequirement {
	var security []dto.SecurityRequirement

	if api.Security != nil {
		if api.Security.APIKey != nil && api.Security.APIKey.Enabled {
			security = append(security, dto.SecurityRequirement{
				"ApiKeyAuth": []string{},
			})
		}
	}

	return security
}

// buildOperationSecurity creates operation-specific security
func (u *APIUtil) buildOperationSecurity(operation dto.Operation, api *dto.API) []dto.SecurityRequirement {
	var security []dto.SecurityRequirement

	if operation.Request != nil && operation.Request.Authentication != nil && operation.Request.Authentication.Required {
		if api.Security != nil {
			if api.Security.APIKey != nil && api.Security.APIKey.Enabled {
				security = append(security, dto.SecurityRequirement{
					"ApiKeyAuth": []string{},
				})
			}

		}
	}

	return security
}

// ConvertAPIYAMLDataToDTO converts APIDeploymentYAML to API DTO
func (u *APIUtil) ConvertAPIYAMLDataToDTO(artifact *dto.APIDeploymentYAML) (*dto.API, error) {
	if artifact == nil {
		return nil, fmt.Errorf("invalid artifact data")
	}

	return u.APIYAMLDataToDTO(&artifact.Spec), nil
}

// APIYAMLDataToDTO converts APIYAMLData to API DTO
//
// This function maps the fields from APIYAMLData
// to the complete API DTO structure. Fields that don't exist in APIYAMLData
// are left with their zero values and should be populated by the caller.
//
// Parameters:
//   - yamlData: The APIYAMLData source data
//
// Returns:
//   - *dto.API: Converted API DTO with mapped fields
func (u *APIUtil) APIYAMLDataToDTO(yamlData *dto.APIYAMLData) *dto.API {
	if yamlData == nil {
		return nil
	}

	// Convert upstream to backend services if present
	var backendServices []dto.BackendService
	if yamlData.Upstream != nil && yamlData.Upstream.Main != nil && yamlData.Upstream.Main.URL != "" {
		backendServices = []dto.BackendService{
			{
				IsDefault: true,
				Endpoints: []dto.BackendEndpoint{
					{
						URL: yamlData.Upstream.Main.URL,
					},
				},
			},
		}
	}

	// Convert operations if present
	var operations []dto.Operation
	if len(yamlData.Operations) > 0 {
		operations = make([]dto.Operation, len(yamlData.Operations))
		for i, op := range yamlData.Operations {
			operations[i] = dto.Operation{
				Name:        fmt.Sprintf("Operation-%d", i+1),
				Description: fmt.Sprintf("Operation for %s %s", op.Method, op.Path),
				Request: &dto.OperationRequest{
					Method:          op.Method,
					Path:            op.Path,
					BackendServices: op.BackendServices,
					Authentication:  op.Authentication,
					Policies:        op.Policies,
				},
			}
		}
	}

	// Create and populate API DTO with available fields
	api := &dto.API{
		Name:            yamlData.DisplayName,
		Context:         yamlData.Context,
		Version:         yamlData.Version,
		BackendServices: backendServices,
		Operations:      operations,
		Policies:        yamlData.Policies,

		// Set reasonable defaults for required fields that aren't in APIYAMLData
		LifeCycleStatus:  "CREATED",
		Type:             "HTTP",
		Transport:        []string{"http", "https"},
		HasThumbnail:     false,
		IsDefaultVersion: false,

		// Fields that need to be set by caller:
		// - ProjectID (required)
		// - OrganizationID (required)
		// - CreatedAt, UpdatedAt (timestamps)
		// - RevisionedAPIID (if applicable)
		// - MTLS, Security, CORS, APIRateLimiting configs
	}

	return api
}

// Validation functions for OpenAPI specifications and WSO2 artifacts

// ValidateOpenAPIDefinition performs comprehensive validation on OpenAPI content using libopenapi
func (u *APIUtil) ValidateOpenAPIDefinition(content []byte) error {
	// Create a new document from the content
	document, err := libopenapi.NewDocument(content)
	if err != nil {
		return fmt.Errorf("failed to parse document: %s", err.Error())
	}

	// Check the specification version
	specInfo := document.GetSpecInfo()
	if specInfo == nil {
		return fmt.Errorf("unable to determine specification version")
	}

	// Handle different specification versions based on version string
	switch {
	case specInfo.Version != "" && strings.HasPrefix(specInfo.Version, "3."):
		return u.validateOpenAPI3Document(document)
	case specInfo.Version != "" && strings.HasPrefix(specInfo.Version, "2."):
		return u.validateSwagger2Document(document)
	default:
		// Try to determine from the document structure
		return u.validateDocumentByStructure(document)
	}
}

// validateDocumentByStructure tries to validate by attempting to build both models
func (u *APIUtil) validateDocumentByStructure(document libopenapi.Document) error {
	// Try OpenAPI 3.x first
	v3Model, v3Errs := document.BuildV3Model()
	if v3Errs == nil && v3Model != nil {
		return u.validateOpenAPI3Model(v3Model)
	}

	// Try Swagger 2.0
	v2Model, v2Errs := document.BuildV2Model()
	if v2Errs == nil && v2Model != nil {
		return u.validateSwagger2Model(v2Model)
	}

	// Both failed, return error
	var errorMessages []string
	if v3Errs != nil {
		errorMessages = append(errorMessages, "OpenAPI 3.x: "+v3Errs.Error())
	}
	if v2Errs != nil {
		errorMessages = append(errorMessages, "Swagger 2.0: "+v2Errs.Error())
	}

	return fmt.Errorf("document validation failed: %s", strings.Join(errorMessages, "; "))
}

// validateOpenAPI3Document validates OpenAPI 3.x documents using libopenapi
func (u *APIUtil) validateOpenAPI3Document(document libopenapi.Document) error {
	// Build the OpenAPI 3.x model
	docModel, err := document.BuildV3Model()
	if err != nil {
		return fmt.Errorf("OpenAPI 3.x model build error: %s", err.Error())
	}

	return u.validateOpenAPI3Model(docModel)
}

// validateOpenAPI3Model validates an OpenAPI 3.x model
func (u *APIUtil) validateOpenAPI3Model(docModel *libopenapi.DocumentModel[v3high.Document]) error {
	if docModel == nil {
		return fmt.Errorf("invalid OpenAPI 3.x document model")
	}

	// Get the OpenAPI document
	doc := &docModel.Model
	if doc.Info == nil {
		return fmt.Errorf("missing required field: info")
	}

	if doc.Info.Title == "" {
		return fmt.Errorf("missing required field: info.title")
	}

	if doc.Info.Version == "" {
		return fmt.Errorf("missing required field: info.version")
	}

	return nil
}

// validateSwagger2Document validates Swagger 2.0 documents using libopenapi
func (u *APIUtil) validateSwagger2Document(document libopenapi.Document) error {
	// Build the Swagger 2.0 model
	docModel, err := document.BuildV2Model()
	if err != nil {
		return fmt.Errorf("Swagger 2.0 model build error: %s", err.Error())
	}

	return u.validateSwagger2Model(docModel)
}

// validateSwagger2Model validates a Swagger 2.0 model
func (u *APIUtil) validateSwagger2Model(docModel *libopenapi.DocumentModel[v2high.Swagger]) error {
	if docModel == nil {
		return fmt.Errorf("invalid Swagger 2.0 document model")
	}

	// Get the Swagger document
	doc := &docModel.Model
	if doc.Info == nil {
		return fmt.Errorf("missing required field: info")
	}

	if doc.Info.Title == "" {
		return fmt.Errorf("missing required field: info.title")
	}

	if doc.Info.Version == "" {
		return fmt.Errorf("missing required field: info.version")
	}

	if doc.Swagger == "" {
		return fmt.Errorf("missing required field: swagger version")
	}

	// Validate that it's a proper 2.0 version
	if !strings.HasPrefix(doc.Swagger, "2.") {
		return fmt.Errorf("invalid swagger version: %s, expected 2.x", doc.Swagger)
	}

	return nil
}

// ValidateWSO2Artifact validates the structure of WSO2 artifact
func (u *APIUtil) ValidateWSO2Artifact(artifact *dto.APIDeploymentYAML) error {
	if artifact.Kind == "" {
		return fmt.Errorf("invalid artifact: missing kind")
	}

	if artifact.ApiVersion == "" {
		return fmt.Errorf("invalid artifact: missing apiVersion")
	}

	if artifact.Spec.DisplayName == "" {
		return fmt.Errorf("missing API displayName")
	}

	if artifact.Spec.Context == "" {
		return fmt.Errorf("missing API context")
	}

	if artifact.Spec.Version == "" {
		return fmt.Errorf("missing API version")
	}

	return nil
}

// ValidateAPIDefinitionConsistency checks if OpenAPI and WSO2 artifact are consistent
func (u *APIUtil) ValidateAPIDefinitionConsistency(openAPIContent []byte, wso2Artifact *dto.APIDeploymentYAML) error {
	var openAPIDoc map[string]interface{}
	if err := yaml.Unmarshal(openAPIContent, &openAPIDoc); err != nil {
		return fmt.Errorf("failed to parse OpenAPI document")
	}

	// Extract info from OpenAPI
	info, exists := openAPIDoc["info"].(map[string]interface{})
	if !exists {
		return fmt.Errorf("missing info section in OpenAPI")
	}

	// Check version consistency
	if version, exists := info["version"].(string); exists {
		if version != wso2Artifact.Spec.Version {
			return fmt.Errorf("version mismatch between OpenAPI (%s) and WSO2 artifact (%s)",
				version, wso2Artifact.Spec.Version)
		}
	}

	return nil
}

// FetchOpenAPIFromURL fetches OpenAPI content from a URL
func (u *APIUtil) FetchOpenAPIFromURL(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return content, nil
}

// ParseAPIDefinition parses OpenAPI 3.x or Swagger 2.0 content and extracts metadata directly into API DTO
func (u *APIUtil) ParseAPIDefinition(content []byte) (*dto.API, error) {
	// Create a new document from the content using libopenapi
	document, err := libopenapi.NewDocument(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API definition: %w", err)
	}

	// Check the specification version
	specInfo := document.GetSpecInfo()
	if specInfo == nil {
		return nil, fmt.Errorf("unable to determine API specification version")
	}

	// Handle different specification versions
	switch {
	case specInfo.Version != "" && strings.HasPrefix(specInfo.Version, "3."):
		return u.parseOpenAPI3Document(document)
	case specInfo.Version != "" && strings.HasPrefix(specInfo.Version, "2."):
		return u.parseSwagger2Document(document)
	default:
		// Try to determine from document structure if version detection fails
		return u.parseDocumentByStructure(document)
	}
}

// parseOpenAPI3Document parses OpenAPI 3.x documents using libopenapi and returns API DTO directly
func (u *APIUtil) parseOpenAPI3Document(document libopenapi.Document) (*dto.API, error) {
	// Build the OpenAPI 3.x model
	docModel, err := document.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("failed to build OpenAPI 3.x model: %w", err)
	}

	if docModel == nil {
		return nil, fmt.Errorf("invalid OpenAPI 3.x document model")
	}

	doc := &docModel.Model
	if doc.Info == nil {
		return nil, fmt.Errorf("missing required field: info")
	}

	// Create API DTO directly
	api := &dto.API{
		Name:        doc.Info.Title,
		Description: doc.Info.Description,
		Version:     doc.Info.Version,
		Type:        "HTTP",
		Transport:   []string{"http", "https"},
	}

	// Extract operations from paths
	operations := u.extractOperationsFromV3Paths(doc.Paths)
	api.Operations = operations

	// Extract backend services from servers
	var backendServices []dto.BackendService
	if doc.Servers != nil {
		for _, server := range doc.Servers {
			service := dto.BackendService{
				Name:        server.Name,
				Description: server.Description,
				Endpoints: []dto.BackendEndpoint{
					{
						URL:    server.URL,
						Weight: 100,
					},
				},
			}
			backendServices = append(backendServices, service)
		}
	}

	api.BackendServices = backendServices

	return api, nil
}

// parseSwagger2Document parses Swagger 2.0 documents using libopenapi and returns API DTO directly
func (u *APIUtil) parseSwagger2Document(document libopenapi.Document) (*dto.API, error) {
	// Build the Swagger 2.0 model
	docModel, err := document.BuildV2Model()
	if err != nil {
		return nil, fmt.Errorf("failed to build Swagger 2.0 model: %w", err)
	}

	if docModel == nil {
		return nil, fmt.Errorf("invalid Swagger 2.0 document model")
	}

	doc := &docModel.Model
	if doc.Info == nil {
		return nil, fmt.Errorf("missing required field: info")
	}

	// Create API DTO directly
	api := &dto.API{
		Name:        doc.Info.Title,
		Description: doc.Info.Description,
		Version:     doc.Info.Version,
		Type:        "HTTP",
		Transport:   []string{"http", "https"},
	}

	// Extract operations from paths
	operations := u.extractOperationsFromV2Paths(doc.Paths)
	api.Operations = operations

	// Convert Swagger 2.0 host/basePath/schemes to backend services
	backendServices := u.convertSwagger2ToBackendServices(doc.Host, doc.BasePath, doc.Schemes)

	api.BackendServices = backendServices

	return api, nil
}

// parseDocumentByStructure tries to parse by attempting to build both models
func (u *APIUtil) parseDocumentByStructure(document libopenapi.Document) (*dto.API, error) {
	// Try OpenAPI 3.x first
	v3Model, v3Errs := document.BuildV3Model()
	if v3Errs == nil && v3Model != nil {
		return u.parseOpenAPI3Document(document)
	}

	// Try Swagger 2.0
	v2Model, v2Errs := document.BuildV2Model()
	if v2Errs == nil && v2Model != nil {
		return u.parseSwagger2Document(document)
	}

	// Both failed, return error
	var errorMessages []string
	if v3Errs != nil {
		errorMessages = append(errorMessages, "OpenAPI 3.x: "+v3Errs.Error())
	}
	if v2Errs != nil {
		errorMessages = append(errorMessages, "Swagger 2.0: "+v2Errs.Error())
	}

	return nil, fmt.Errorf("document parsing failed: %s", strings.Join(errorMessages, "; "))
}

// extractOperationsFromV3Paths extracts operations from OpenAPI 3.x paths
func (u *APIUtil) extractOperationsFromV3Paths(paths *v3high.Paths) []dto.Operation {
	var operations []dto.Operation

	if paths == nil || paths.PathItems == nil {
		return operations
	}

	for pair := paths.PathItems.First(); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()
		if pathItem == nil {
			continue
		}

		// Extract operations for each HTTP method
		methodOps := map[string]*v3high.Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"PATCH":   pathItem.Patch,
			"DELETE":  pathItem.Delete,
			"OPTIONS": pathItem.Options,
			"HEAD":    pathItem.Head,
			"TRACE":   pathItem.Trace,
		}

		for method, operation := range methodOps {
			if operation == nil {
				continue
			}

			op := dto.Operation{
				Name:        operation.Summary,
				Description: operation.Description,
				Request: &dto.OperationRequest{
					Method: method,
					Path:   path,
					Authentication: &dto.AuthenticationConfig{
						Required: false,
						Scopes:   []string{},
					},
					Policies: []dto.Policy{},
				},
			}

			operations = append(operations, op)
		}
	}

	return operations
}

// extractOperationsFromV2Paths extracts operations from Swagger 2.0 paths
func (u *APIUtil) extractOperationsFromV2Paths(paths *v2high.Paths) []dto.Operation {
	var operations []dto.Operation

	if paths == nil || paths.PathItems == nil {
		return operations
	}

	for pair := paths.PathItems.First(); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()

		if pathItem == nil {
			continue
		}

		// Extract operations for each HTTP method
		methodOps := map[string]*v2high.Operation{
			"GET":     pathItem.Get,
			"POST":    pathItem.Post,
			"PUT":     pathItem.Put,
			"PATCH":   pathItem.Patch,
			"DELETE":  pathItem.Delete,
			"OPTIONS": pathItem.Options,
			"HEAD":    pathItem.Head,
		}

		for method, operation := range methodOps {
			if operation == nil {
				continue
			}

			op := dto.Operation{
				Name:        operation.Summary,
				Description: operation.Description,
				Request: &dto.OperationRequest{
					Method: method,
					Path:   path,
					Authentication: &dto.AuthenticationConfig{
						Required: false,
						Scopes:   []string{},
					},
					Policies: []dto.Policy{},
				},
			}

			operations = append(operations, op)
		}
	}

	return operations
}

// convertSwagger2ToBackendServices converts Swagger 2.0 host/basePath/schemes to backend services
func (u *APIUtil) convertSwagger2ToBackendServices(host, basePath string, schemes []string) []dto.BackendService {
	var backendServices []dto.BackendService

	if host == "" {
		return backendServices // No host specified, cannot create backend services
	}

	if len(schemes) == 0 {
		schemes = []string{"https"} // Default to HTTPS
	}

	if basePath == "" {
		basePath = "/"
	}

	// Create backend services for each scheme
	for _, scheme := range schemes {
		url := fmt.Sprintf("%s://%s%s", scheme, host, basePath)
		service := dto.BackendService{
			Endpoints: []dto.BackendEndpoint{
				{
					URL:    url,
					Weight: 100,
				},
			},
		}
		backendServices = append(backendServices, service)
	}

	return backendServices
}

// ValidateAndParseOpenAPI validates and parses OpenAPI definition content
func (u *APIUtil) ValidateAndParseOpenAPI(content []byte) (*dto.API, error) {
	// Validate the OpenAPI definition
	if err := u.ValidateOpenAPIDefinition(content); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI definition: %w", err)
	}

	// Parse and extract API details
	api, err := u.ParseAPIDefinition(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI definition: %w", err)
	}

	return api, nil
}

// MergeAPIDetails merges user-provided API details with extracted OpenAPI details
// User-provided details take precedence over extracted details
func (u *APIUtil) MergeAPIDetails(userAPI *dto.API, extractedAPI *dto.API) *dto.API {
	if userAPI == nil || extractedAPI == nil {
		return nil
	}

	merged := &dto.API{}

	// Required fields from user input (these must be provided)
	merged.Name = userAPI.Name
	merged.Context = userAPI.Context
	merged.Version = userAPI.Version
	merged.ProjectID = userAPI.ProjectID

	if userAPI.ID != "" {
		merged.ID = userAPI.ID
	} else {
		merged.ID = extractedAPI.ID
	}
	if userAPI.Description != "" {
		merged.Description = userAPI.Description
	} else {
		merged.Description = extractedAPI.Description
	}

	if userAPI.Provider != "" {
		merged.Provider = userAPI.Provider
	} else {
		merged.Provider = extractedAPI.Provider
	}

	if userAPI.Type != "" {
		merged.Type = userAPI.Type
	} else {
		merged.Type = extractedAPI.Type
	}

	if len(userAPI.Transport) > 0 {
		merged.Transport = userAPI.Transport
	} else {
		merged.Transport = extractedAPI.Transport
	}

	if userAPI.LifeCycleStatus != "" {
		merged.LifeCycleStatus = userAPI.LifeCycleStatus
	} else {
		merged.LifeCycleStatus = extractedAPI.LifeCycleStatus
	}

	if len(userAPI.BackendServices) > 0 {
		merged.BackendServices = userAPI.BackendServices
	} else {
		merged.BackendServices = extractedAPI.BackendServices
	}

	// Use extracted operations from OpenAPI
	merged.Operations = extractedAPI.Operations

	// Use user-provided configuration if available
	if userAPI.MTLS != nil {
		merged.MTLS = userAPI.MTLS
	}
	if userAPI.Security != nil {
		merged.Security = userAPI.Security
	}
	if userAPI.CORS != nil {
		merged.CORS = userAPI.CORS
	}
	if userAPI.APIRateLimiting != nil {
		merged.APIRateLimiting = userAPI.APIRateLimiting
	}

	// Copy boolean fields from user input
	merged.HasThumbnail = userAPI.HasThumbnail
	merged.IsDefaultVersion = userAPI.IsDefaultVersion

	return merged
}
