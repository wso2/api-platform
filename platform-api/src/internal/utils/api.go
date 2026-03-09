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

	"platform-api/src/api"
	"platform-api/src/internal/client/devportal_client"
	"platform-api/src/internal/constants"
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/pb33f/libopenapi"
	v2high "github.com/pb33f/libopenapi/datamodel/high/v2"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"gopkg.in/yaml.v3"
)

type APIUtil struct{}

// Mapping functions
// RESTAPIToModel converts a REST API model to internal model representation.
// Note: RESTAPI.Id maps to Model.Handle (user-facing identifier)
// Organization ID must be provided by the caller.
func (u *APIUtil) RESTAPIToModel(restAPI *api.RESTAPI, orgID string) *model.API {
	if restAPI == nil {
		return nil
	}

	handle := ""
	if restAPI.Id != nil {
		handle = *restAPI.Id
	}

	kind := constants.RestApi
	if restAPI.Kind != nil {
		kind = *restAPI.Kind
	}

	description := ""
	if restAPI.Description != nil {
		description = *restAPI.Description
	}

	createdBy := ""
	if restAPI.CreatedBy != nil {
		createdBy = *restAPI.CreatedBy
	}

	lifeCycleStatus := ""
	if restAPI.LifeCycleStatus != nil {
		lifeCycleStatus = string(*restAPI.LifeCycleStatus)
	}

	transport := []string{}
	if restAPI.Transport != nil {
		transport = *restAPI.Transport
	}

	projectID := OpenAPIUUIDToString(restAPI.ProjectId)

	apiModel := &model.API{
		Handle:          handle,
		Name:            restAPI.Name,
		Kind:            kind,
		Description:     description,
		Version:         restAPI.Version,
		CreatedBy:       createdBy,
		ProjectID:       projectID,
		OrganizationID:  orgID,
		LifeCycleStatus: lifeCycleStatus,
		Transport:       transport,
		Channels:        u.ChannelsAPIToModel(restAPI.Channels),
		Configuration: model.RestAPIConfig{
			Name:       restAPI.Name,
			Version:    restAPI.Version,
			Context:    &restAPI.Context,
			Upstream:   *u.UpstreamConfigAPIToModel(&restAPI.Upstream),
			Policies:   u.PoliciesAPIToModel(restAPI.Policies),
			Operations: u.OperationsAPIToModel(restAPI.Operations),
		},
	}

	if restAPI.CreatedAt != nil {
		apiModel.CreatedAt = *restAPI.CreatedAt
	}
	if restAPI.UpdatedAt != nil {
		apiModel.UpdatedAt = *restAPI.UpdatedAt
	}

	return apiModel
}

// ModelToRESTAPI converts internal model representation to REST API model.
// Note: Model.Handle maps to RESTAPI.Id (user-facing identifier)
func (u *APIUtil) ModelToRESTAPI(modelAPI *model.API) (*api.RESTAPI, error) {
	if modelAPI == nil {
		return nil, nil
	}

	projectID, err := ParseOpenAPIUUID(modelAPI.ProjectID)
	if err != nil {
		return nil, err
	}

	var status *api.RESTAPILifeCycleStatus
	if modelAPI.LifeCycleStatus != "" {
		value := api.RESTAPILifeCycleStatus(modelAPI.LifeCycleStatus)
		status = &value
	}

	return &api.RESTAPI{
		Channels:        u.ChannelsModelToAPI(modelAPI.Channels),
		Context:         defaultStringPtr(modelAPI.Configuration.Context),
		CreatedAt:       TimePtrIfNotZero(modelAPI.CreatedAt),
		CreatedBy:       StringPtrIfNotEmpty(modelAPI.CreatedBy),
		Description:     StringPtrIfNotEmpty(modelAPI.Description),
		Id:              StringPtrIfNotEmpty(modelAPI.Handle),
		Kind:            StringPtrIfNotEmpty(modelAPI.Kind),
		LifeCycleStatus: status,
		Name:            modelAPI.Name,
		Operations:      u.OperationsModelToAPI(modelAPI.Configuration.Operations),
		Policies:        u.PoliciesModelToAPI(modelAPI.Configuration.Policies),
		ProjectId:       *projectID,
		Transport:       stringSlicePtr(modelAPI.Transport),
		UpdatedAt:       TimePtrIfNotZero(modelAPI.UpdatedAt),
		Upstream:        u.UpstreamConfigModelToAPI(&modelAPI.Configuration.Upstream),
		Version:         modelAPI.Version,
	}, nil
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

// API to Model conversion helpers

func (u *APIUtil) OperationsAPIToModel(operations *[]api.Operation) []model.Operation {
	if operations == nil {
		return nil
	}
	models := make([]model.Operation, 0, len(*operations))
	for _, op := range *operations {
		models = append(models, *u.OperationAPIToModel(&op))
	}
	return models
}

func (u *APIUtil) ChannelsAPIToModel(channels *[]api.Channel) []model.Channel {
	if channels == nil {
		return nil
	}
	models := make([]model.Channel, 0, len(*channels))
	for _, ch := range *channels {
		models = append(models, *u.ChannelAPIToModel(&ch))
	}
	return models
}

func (u *APIUtil) OperationAPIToModel(operation *api.Operation) *model.Operation {
	if operation == nil {
		return nil
	}
	return &model.Operation{
		Name:        defaultStringPtr(operation.Name),
		Description: defaultStringPtr(operation.Description),
		Request:     u.OperationRequestAPIToModel(&operation.Request),
	}
}

func (u *APIUtil) ChannelAPIToModel(channel *api.Channel) *model.Channel {
	if channel == nil {
		return nil
	}
	return &model.Channel{
		Name:        defaultStringPtr(channel.Name),
		Description: defaultStringPtr(channel.Description),
		Request:     u.ChannelRequestAPIToModel(&channel.Request),
	}
}

func (u *APIUtil) OperationRequestAPIToModel(req *api.OperationRequest) *model.OperationRequest {
	if req == nil {
		return nil
	}
	return &model.OperationRequest{
		Method:   string(req.Method),
		Path:     req.Path,
		Policies: u.PoliciesAPIToModel(req.Policies),
	}
}

func (u *APIUtil) ChannelRequestAPIToModel(req *api.ChannelRequest) *model.ChannelRequest {
	if req == nil {
		return nil
	}
	return &model.ChannelRequest{
		Method:   string(req.Method),
		Name:     req.Name,
		Policies: u.PoliciesAPIToModel(req.Policies),
	}
}

func (u *APIUtil) PoliciesAPIToModel(policies *[]api.Policy) []model.Policy {
	if policies == nil {
		return nil
	}
	models := make([]model.Policy, 0, len(*policies))
	for _, policy := range *policies {
		models = append(models, *u.PolicyAPIToModel(&policy))
	}
	return models
}

func (u *APIUtil) PolicyAPIToModel(policy *api.Policy) *model.Policy {
	if policy == nil {
		return nil
	}
	return &model.Policy{
		ExecutionCondition: policy.ExecutionCondition,
		Name:               policy.Name,
		Params:             policy.Params,
		Version:            policy.Version,
	}
}

func (u *APIUtil) UpstreamConfigAPIToModel(upstream *api.Upstream) *model.UpstreamConfig {
	if upstream == nil {
		return &model.UpstreamConfig{}
	}
	return &model.UpstreamConfig{
		Main:    u.upstreamDefinitionToModel(&upstream.Main),
		Sandbox: u.upstreamDefinitionToModel(upstream.Sandbox),
	}
}

func (u *APIUtil) upstreamDefinitionToModel(definition *api.UpstreamDefinition) *model.UpstreamEndpoint {
	if definition == nil {
		return nil
	}
	if definition.Url == nil && definition.Ref == nil && definition.Auth == nil {
		return nil
	}
	endpoint := &model.UpstreamEndpoint{
		URL: defaultStringPtr(definition.Url),
		Ref: defaultStringPtr(definition.Ref),
	}
	if definition.Auth != nil {
		endpoint.Auth = u.upstreamAuthToModel(definition.Auth)
	}
	return endpoint
}

func (u *APIUtil) upstreamAuthToModel(auth *api.UpstreamAuth) *model.UpstreamAuth {
	if auth == nil {
		return nil
	}
	modelAuth := &model.UpstreamAuth{}
	if auth.Type != nil {
		modelAuth.Type = string(*auth.Type)
	}
	modelAuth.Header = defaultStringPtr(auth.Header)
	modelAuth.Value = defaultStringPtr(auth.Value)
	return modelAuth
}

// Model to API conversion helpers

func (u *APIUtil) OperationsModelToAPI(models []model.Operation) *[]api.Operation {
	if models == nil {
		return nil
	}
	operations := make([]api.Operation, 0, len(models))
	for _, op := range models {
		operations = append(operations, *u.OperationModelToAPI(&op))
	}
	return &operations
}

func (u *APIUtil) ChannelsModelToAPI(models []model.Channel) *[]api.Channel {
	if models == nil {
		return nil
	}
	channels := make([]api.Channel, 0, len(models))
	for _, ch := range models {
		channels = append(channels, *u.ChannelModelToAPI(&ch))
	}
	return &channels
}

func (u *APIUtil) OperationModelToAPI(modelOp *model.Operation) *api.Operation {
	if modelOp == nil {
		return nil
	}

	request := api.OperationRequest{}
	if modelOp.Request != nil {
		request = *u.OperationRequestModelToAPI(modelOp.Request)
	}
	return &api.Operation{
		Name:        StringPtrIfNotEmpty(modelOp.Name),
		Description: StringPtrIfNotEmpty(modelOp.Description),
		Request:     request,
	}
}

func (u *APIUtil) ChannelModelToAPI(modelCh *model.Channel) *api.Channel {
	if modelCh == nil {
		return nil
	}

	request := api.ChannelRequest{}
	if modelCh.Request != nil {
		request = *u.ChannelRequestModelToAPI(modelCh.Request)
	}
	return &api.Channel{
		Name:        StringPtrIfNotEmpty(modelCh.Name),
		Description: StringPtrIfNotEmpty(modelCh.Description),
		Request:     request,
	}
}

func (u *APIUtil) OperationRequestModelToAPI(modelReq *model.OperationRequest) *api.OperationRequest {
	if modelReq == nil {
		return nil
	}
	return &api.OperationRequest{
		Method:   api.OperationRequestMethod(modelReq.Method),
		Path:     modelReq.Path,
		Policies: u.PoliciesModelToAPI(modelReq.Policies),
	}
}

func (u *APIUtil) ChannelRequestModelToAPI(modelReq *model.ChannelRequest) *api.ChannelRequest {
	if modelReq == nil {
		return nil
	}
	return &api.ChannelRequest{
		Method:   api.ChannelRequestMethod(modelReq.Method),
		Name:     modelReq.Name,
		Policies: u.PoliciesModelToAPI(modelReq.Policies),
	}
}

func (u *APIUtil) PoliciesModelToAPI(models []model.Policy) *[]api.Policy {
	if models == nil {
		return nil
	}
	policies := make([]api.Policy, 0, len(models))
	for _, policy := range models {
		policies = append(policies, *u.PolicyModelToAPI(policy))
	}
	return &policies
}

func (u *APIUtil) PolicyModelToAPI(modelPolicy model.Policy) *api.Policy {
	return &api.Policy{
		ExecutionCondition: modelPolicy.ExecutionCondition,
		Name:               modelPolicy.Name,
		Params:             modelPolicy.Params,
		Version:            modelPolicy.Version,
	}
}

func (u *APIUtil) UpstreamConfigModelToAPI(modelUpstream *model.UpstreamConfig) api.Upstream {
	if modelUpstream == nil {
		return api.Upstream{Main: api.UpstreamDefinition{}}
	}
	return api.Upstream{
		Main:    u.upstreamEndpointToAPI(modelUpstream.Main),
		Sandbox: u.upstreamEndpointPtrToAPI(modelUpstream.Sandbox),
	}
}

func (u *APIUtil) upstreamEndpointPtrToAPI(endpoint *model.UpstreamEndpoint) *api.UpstreamDefinition {
	if endpoint == nil {
		return nil
	}
	def := u.upstreamEndpointToAPI(endpoint)
	return &def
}

func (u *APIUtil) upstreamEndpointToAPI(endpoint *model.UpstreamEndpoint) api.UpstreamDefinition {
	if endpoint == nil {
		return api.UpstreamDefinition{}
	}
	def := api.UpstreamDefinition{}
	if endpoint.URL != "" {
		def.Url = StringPtrIfNotEmpty(endpoint.URL)
	}
	if endpoint.Ref != "" {
		def.Ref = StringPtrIfNotEmpty(endpoint.Ref)
	}
	if endpoint.Auth != nil {
		def.Auth = u.upstreamAuthToAPI(endpoint.Auth)
	}
	return def
}

func (u *APIUtil) upstreamAuthToAPI(auth *model.UpstreamAuth) *api.UpstreamAuth {
	if auth == nil {
		return nil
	}
	apiAuth := &api.UpstreamAuth{}
	if auth.Type != "" {
		value := api.UpstreamAuthType(auth.Type)
		apiAuth.Type = &value
	}
	apiAuth.Header = StringPtrIfNotEmpty(auth.Header)
	apiAuth.Value = StringPtrIfNotEmpty(auth.Value)
	return apiAuth
}

// GenerateAPIDeploymentYAML creates the deployment YAML from API model
func (u *APIUtil) GenerateAPIDeploymentYAML(apiModel *model.API) (string, error) {
	operationList := make([]api.OperationRequest, 0)
	for _, op := range apiModel.Configuration.Operations {
		operationList = append(operationList, api.OperationRequest{
			Method:   api.OperationRequestMethod(op.Request.Method),
			Path:     op.Request.Path,
			Policies: u.PoliciesModelToAPI(op.Request.Policies),
		})
	}
	channelList := make([]api.ChannelRequest, 0)
	for _, ch := range apiModel.Channels {
		channelList = append(channelList, api.ChannelRequest{
			Method:   api.ChannelRequestMethod(ch.Request.Method),
			Name:     ch.Request.Name,
			Policies: u.PoliciesModelToAPI(ch.Request.Policies),
		})
	}

	// Convert upstream config to YAML format
	var upstreamYAML *dto.UpstreamYAML
	if apiModel.Configuration.Upstream.Main != nil || apiModel.Configuration.Upstream.Sandbox != nil {
		upstreamYAML = &dto.UpstreamYAML{}
		if apiModel.Configuration.Upstream.Main != nil {
			upstreamYAML.Main = &dto.UpstreamTarget{}
			if apiModel.Configuration.Upstream.Main.URL != "" {
				upstreamYAML.Main.URL = apiModel.Configuration.Upstream.Main.URL
			}
			if apiModel.Configuration.Upstream.Main.Ref != "" {
				upstreamYAML.Main.Ref = apiModel.Configuration.Upstream.Main.Ref
			}
		}
		if apiModel.Configuration.Upstream.Sandbox != nil {
			upstreamYAML.Sandbox = &dto.UpstreamTarget{}
			if apiModel.Configuration.Upstream.Sandbox.URL != "" {
				upstreamYAML.Sandbox.URL = apiModel.Configuration.Upstream.Sandbox.URL
			}
			if apiModel.Configuration.Upstream.Sandbox.Ref != "" {
				upstreamYAML.Sandbox.Ref = apiModel.Configuration.Upstream.Sandbox.Ref
			}
		}
	}

	apiYAMLData := dto.APIYAMLData{}
	apiYAMLData.DisplayName = apiModel.Name
	apiYAMLData.Version = apiModel.Version
	apiYAMLData.Context = defaultStringPtr(apiModel.Configuration.Context)
	apiYAMLData.Policies = u.PoliciesModelToDTO(apiModel.Configuration.Policies)

	// Only set upstream and operations for HTTP APIs
	switch apiModel.Kind {
	case constants.RestApi:
		apiYAMLData.Upstream = upstreamYAML
		apiYAMLData.Operations = operationList
	case constants.WebSubApi:
		apiYAMLData.Channels = channelList
	}

	apiType := ""
	switch apiModel.Kind {
	case constants.RestApi:
		apiType = constants.RestApi
	case constants.WebSubApi:
		apiType = constants.WebSubApi
	}

	apiDeployment := dto.APIDeploymentYAML{
		ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
		Kind:       apiType,
		Metadata: dto.DeploymentMetadata{
			Name: apiModel.Handle,
			Labels: map[string]string{
				"project-id": apiModel.ProjectID,
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
// GenerateOpenAPIDefinitionFromRESTAPI generates an OpenAPI 3.0 definition from a generated api.RESTAPI model.
func (u *APIUtil) GenerateOpenAPIDefinitionFromRESTAPI(restAPI *api.RESTAPI, req *devportal_client.APIMetadataRequest) ([]byte, error) {
	if restAPI == nil {
		return nil, fmt.Errorf("api model is required")
	}
	if req == nil {
		return nil, fmt.Errorf("metadata request is required")
	}

	openAPISpec := dto.OpenAPI{
		OpenAPI: "3.0.3",
		Info:    u.buildInfoSectionFromRESTAPI(restAPI),
		Servers: u.buildServersSectionFromRESTAPI(restAPI, &req.EndPoints),
		Paths:   u.buildPathsSectionFromRESTAPI(restAPI),
	}

	apiDefinition, err := json.Marshal(openAPISpec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAPI definition: %w", err)
	}

	return apiDefinition, nil
}

func (u *APIUtil) buildInfoSectionFromRESTAPI(restAPI *api.RESTAPI) dto.Info {
	info := dto.Info{}
	info.Title = restAPI.Name
	info.Version = restAPI.Version

	if restAPI.Description != nil {
		info.Description = *restAPI.Description
	}
	if restAPI.CreatedBy != nil && *restAPI.CreatedBy != "" {
		info.Contact = &dto.Contact{Name: *restAPI.CreatedBy}
	}

	return info
}

func (u *APIUtil) buildServersSectionFromRESTAPI(restAPI *api.RESTAPI, endpoints *devportal_client.EndPoints) []dto.Server {
	var servers []dto.Server
	if endpoints == nil {
		return servers
	}

	context := restAPI.Context
	joinBaseAndContext := func(baseURL, ctx string) string {
		if ctx == "" {
			return baseURL
		}

		normalizedBase := strings.TrimRight(baseURL, "/")
		normalizedContext := strings.TrimLeft(ctx, "/")
		if normalizedContext == "" {
			return normalizedBase
		}

		if strings.HasSuffix(normalizedBase, "/"+normalizedContext) {
			return normalizedBase
		}

		return normalizedBase + "/" + normalizedContext
	}

	if endpoints.ProductionURL != "" {
		prodURL := joinBaseAndContext(endpoints.ProductionURL, context)
		servers = append(servers, dto.Server{URL: prodURL, Description: "Production server"})
	}

	if endpoints.SandboxURL != "" {
		sandboxURL := joinBaseAndContext(endpoints.SandboxURL, context)
		servers = append(servers, dto.Server{URL: sandboxURL, Description: "Sandbox server"})
	}

	return servers
}

func (u *APIUtil) buildPathsSectionFromRESTAPI(restAPI *api.RESTAPI) map[string]dto.PathItem {
	paths := make(map[string]dto.PathItem)
	if restAPI.Operations == nil {
		return paths
	}

	for _, operation := range *restAPI.Operations {
		path := operation.Request.Path
		method := strings.ToLower(string(operation.Request.Method))

		pathItem, exists := paths[path]
		if !exists {
			pathItem = dto.PathItem{}
		}

		summary := ""
		if operation.Name != nil {
			summary = *operation.Name
		}
		description := ""
		if operation.Description != nil {
			description = *operation.Description
		}

		operationSpec := &dto.OpenAPIOperation{Summary: summary, Description: description}

		if parameters := u.buildParameters(path); len(parameters) > 0 {
			operationSpec.Parameters = parameters
		}

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
func (u *APIUtil) buildParameters(path string) []dto.Parameter {
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

	return parameters
}

// APIYAMLDataToRESTAPI converts APIYAMLData to generated RESTAPI model.
//
// This function maps the fields from APIYAMLData
// to the RESTAPI structure. Fields that don't exist in APIYAMLData
// are left with their zero values and should be populated by the caller.
//
// Parameters:
//   - yamlData: The APIYAMLData source data
//
// Returns:
//   - *api.RESTAPI: Converted generated RESTAPI model with mapped fields
func (u *APIUtil) APIYAMLDataToRESTAPI(yamlData *dto.APIYAMLData) *api.RESTAPI {
	if yamlData == nil {
		return nil
	}

	// Convert operations if present (always initialize to empty slice to avoid null JSON)
	operations := make([]api.Operation, 0)
	if len(yamlData.Operations) > 0 {
		operations = make([]api.Operation, len(yamlData.Operations))
		for i, op := range yamlData.Operations {
			policies := op.Policies
			if policies == nil {
				policies = &[]api.Policy{}
			}
			operations[i] = api.Operation{
				Name:        StringPtrIfNotEmpty(fmt.Sprintf("Operation-%d", i+1)),
				Description: StringPtrIfNotEmpty(fmt.Sprintf("Operation for %s %s", op.Method, op.Path)),
				Request: api.OperationRequest{
					Method:   op.Method,
					Path:     op.Path,
					Policies: policies,
				},
			}
		}
	}

	// Convert channels if present (always initialize to empty slice to avoid null JSON)
	channels := make([]api.Channel, 0)
	if len(yamlData.Channels) > 0 {
		channels = make([]api.Channel, len(yamlData.Channels))
		for i, ch := range yamlData.Channels {
			policies := ch.Policies
			if policies == nil {
				policies = &[]api.Policy{}
			}
			channels[i] = api.Channel{
				Name:        StringPtrIfNotEmpty(fmt.Sprintf("Channel-%d", i+1)),
				Description: StringPtrIfNotEmpty(fmt.Sprintf("Channel for %s %s", ch.Method, ch.Name)),
				Request: api.ChannelRequest{
					Method:   ch.Method,
					Name:     ch.Name,
					Policies: policies,
				},
			}
		}
	}

	// Map upstream from YAML to DTO
	upstream := api.Upstream{Main: api.UpstreamDefinition{}}
	if yamlData.Upstream != nil {
		if yamlData.Upstream.Main != nil {
			upstream.Main = api.UpstreamDefinition{
				Url: StringPtrIfNotEmpty(yamlData.Upstream.Main.URL),
				Ref: StringPtrIfNotEmpty(yamlData.Upstream.Main.Ref),
			}
		}
		if yamlData.Upstream.Sandbox != nil {
			upstream.Sandbox = &api.UpstreamDefinition{
				Url: StringPtrIfNotEmpty(yamlData.Upstream.Sandbox.URL),
				Ref: StringPtrIfNotEmpty(yamlData.Upstream.Sandbox.Ref),
			}
		}
	}

	kind := constants.RestApi
	if len(channels) > 0 && len(operations) == 0 {
		kind = constants.WebSubApi
	}

	lifeCycleStatus := api.RESTAPILifeCycleStatus("CREATED")

	// Create and populate generated RESTAPI model with available fields
	restAPI := &api.RESTAPI{
		Name:            yamlData.DisplayName,
		Context:         yamlData.Context,
		Version:         yamlData.Version,
		Operations:      &operations,
		Channels:        &channels,
		Policies:        u.PoliciesModelToAPI(u.PoliciesDTOToModel(yamlData.Policies)),
		Upstream:        upstream,
		LifeCycleStatus: &lifeCycleStatus,
		Kind:            StringPtrIfNotEmpty(kind),
		Transport:       stringSlicePtr([]string{"http", "https"}),
		ProjectId:       openapi_types.UUID{},

		// Fields that may be set by caller:
		// - Id
		// - Description
		// - CreatedBy
		// - CreatedAt, UpdatedAt
	}

	return restAPI
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

// ParseAPIDefinitionToRESTAPI parses OpenAPI 3.x or Swagger 2.0 content and extracts metadata into a generated api.RESTAPI.
//
// Notes:
//   - The returned RESTAPI is *partial*: fields like Context and ProjectId are not present in OpenAPI and will be empty.
//   - ProjectId is set to the zero UUID to satisfy generated model requirements.
func (u *APIUtil) ParseAPIDefinitionToRESTAPI(content []byte) (*api.RESTAPI, error) {
	document, err := libopenapi.NewDocument(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API definition: %w", err)
	}

	specInfo := document.GetSpecInfo()
	if specInfo == nil {
		return nil, fmt.Errorf("unable to determine API specification version")
	}

	switch {
	case specInfo.Version != "" && strings.HasPrefix(specInfo.Version, "3."):
		return u.parseOpenAPI3DocumentToRESTAPI(document)
	case specInfo.Version != "" && strings.HasPrefix(specInfo.Version, "2."):
		return u.parseSwagger2DocumentToRESTAPI(document)
	default:
		return u.parseDocumentByStructureToRESTAPI(document)
	}
}

func (u *APIUtil) parseOpenAPI3DocumentToRESTAPI(document libopenapi.Document) (*api.RESTAPI, error) {
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

	rest := &api.RESTAPI{
		Name:      doc.Info.Title,
		Context:   "",
		Version:   doc.Info.Version,
		ProjectId: openapi_types.UUID{},
		Kind:      StringPtrIfNotEmpty(constants.RestApi),
		Upstream:  api.Upstream{},
		Transport: stringSlicePtr([]string{"http", "https"}),
	}
	if doc.Info.Description != "" {
		rest.Description = StringPtrIfNotEmpty(doc.Info.Description)
	}

	ops := u.extractOperationsFromV3PathsAPI(doc.Paths)
	if len(ops) > 0 {
		rest.Operations = &ops
	}

	// Extract upstream from servers
	if len(doc.Servers) > 0 {
		rest.Upstream = api.Upstream{
			Main: api.UpstreamDefinition{
				Url: StringPtrIfNotEmpty(doc.Servers[0].URL),
			},
		}
	}

	return rest, nil
}

func (u *APIUtil) parseSwagger2DocumentToRESTAPI(document libopenapi.Document) (*api.RESTAPI, error) {
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

	rest := &api.RESTAPI{
		Name:      doc.Info.Title,
		Context:   "",
		Version:   doc.Info.Version,
		ProjectId: openapi_types.UUID{},
		Kind:      StringPtrIfNotEmpty(constants.RestApi),
		Upstream:  api.Upstream{},
		Transport: stringSlicePtr([]string{"http", "https"}),
	}
	if doc.Info.Description != "" {
		rest.Description = StringPtrIfNotEmpty(doc.Info.Description)
	}

	ops := u.extractOperationsFromV2PathsAPI(doc.Paths)
	if len(ops) > 0 {
		rest.Operations = &ops
	}

	// Convert Swagger 2.0 host/basePath/schemes to upstream
	rest.Upstream = u.convertSwagger2ToUpstreamAPI(doc.Host, doc.BasePath, doc.Schemes)

	return rest, nil
}

func (u *APIUtil) parseDocumentByStructureToRESTAPI(document libopenapi.Document) (*api.RESTAPI, error) {
	v3Model, v3Errs := document.BuildV3Model()
	if v3Errs == nil && v3Model != nil {
		return u.parseOpenAPI3DocumentToRESTAPI(document)
	}

	v2Model, v2Errs := document.BuildV2Model()
	if v2Errs == nil && v2Model != nil {
		return u.parseSwagger2DocumentToRESTAPI(document)
	}

	var errorMessages []string
	if v3Errs != nil {
		errorMessages = append(errorMessages, "OpenAPI 3.x: "+v3Errs.Error())
	}
	if v2Errs != nil {
		errorMessages = append(errorMessages, "Swagger 2.0: "+v2Errs.Error())
	}

	return nil, fmt.Errorf("document parsing failed: %s", strings.Join(errorMessages, "; "))
}

func (u *APIUtil) extractOperationsFromV3PathsAPI(paths *v3high.Paths) []api.Operation {
	operations := make([]api.Operation, 0)
	if paths == nil || paths.PathItems == nil {
		return operations
	}

	for pair := paths.PathItems.First(); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()
		if pathItem == nil {
			continue
		}

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

			op := api.Operation{
				Name:        StringPtrIfNotEmpty(operation.Summary),
				Description: StringPtrIfNotEmpty(operation.Description),
				Request: api.OperationRequest{
					Method:   api.OperationRequestMethod(method),
					Path:     path,
					Policies: &[]api.Policy{},
				},
			}

			operations = append(operations, op)
		}
	}

	return operations
}

func (u *APIUtil) extractOperationsFromV2PathsAPI(paths *v2high.Paths) []api.Operation {
	operations := make([]api.Operation, 0)
	if paths == nil || paths.PathItems == nil {
		return operations
	}

	for pair := paths.PathItems.First(); pair != nil; pair = pair.Next() {
		path := pair.Key()
		pathItem := pair.Value()
		if pathItem == nil {
			continue
		}

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

			op := api.Operation{
				Name:        StringPtrIfNotEmpty(operation.Summary),
				Description: StringPtrIfNotEmpty(operation.Description),
				Request: api.OperationRequest{
					Method:   api.OperationRequestMethod(method),
					Path:     path,
					Policies: &[]api.Policy{},
				},
			}

			operations = append(operations, op)
		}
	}

	return operations
}

func (u *APIUtil) convertSwagger2ToUpstreamAPI(host, basePath string, schemes []string) api.Upstream {
	if host == "" {
		return api.Upstream{}
	}
	if len(schemes) == 0 {
		schemes = []string{"https"}
	}
	if basePath == "" {
		basePath = "/"
	}
	url := fmt.Sprintf("%s://%s%s", schemes[0], host, basePath)
	return api.Upstream{
		Main: api.UpstreamDefinition{Url: StringPtrIfNotEmpty(url)},
	}
}

// ValidateAndParseOpenAPIToRESTAPI validates and parses OpenAPI definition content into a partial RESTAPI model.
func (u *APIUtil) ValidateAndParseOpenAPIToRESTAPI(content []byte) (*api.RESTAPI, error) {
	if err := u.ValidateOpenAPIDefinition(content); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI definition: %w", err)
	}
	apiModel, err := u.ParseAPIDefinitionToRESTAPI(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI definition: %w", err)
	}
	return apiModel, nil
}

// MergeRESTAPIDetails merges user-provided REST API details with extracted OpenAPI details.
// User-provided details take precedence for scalar metadata; extracted operations are used when available.
func (u *APIUtil) MergeRESTAPIDetails(userAPI *api.RESTAPI, extractedAPI *api.RESTAPI) *api.RESTAPI {
	if userAPI == nil || extractedAPI == nil {
		return nil
	}

	merged := &api.RESTAPI{
		Name:      userAPI.Name,
		Context:   userAPI.Context,
		Version:   userAPI.Version,
		ProjectId: userAPI.ProjectId,
		Upstream:  api.Upstream{},
	}

	// Handle / Id
	if userAPI.Id != nil && *userAPI.Id != "" {
		merged.Id = userAPI.Id
	} else {
		merged.Id = extractedAPI.Id
	}

	// Description
	if userAPI.Description != nil && *userAPI.Description != "" {
		merged.Description = userAPI.Description
	} else {
		merged.Description = extractedAPI.Description
	}

	// CreatedBy
	if userAPI.CreatedBy != nil && *userAPI.CreatedBy != "" {
		merged.CreatedBy = userAPI.CreatedBy
	} else {
		merged.CreatedBy = extractedAPI.CreatedBy
	}

	// Kind
	if userAPI.Kind != nil && *userAPI.Kind != "" {
		merged.Kind = userAPI.Kind
	} else {
		merged.Kind = extractedAPI.Kind
	}

	// Transport
	if userAPI.Transport != nil && len(*userAPI.Transport) > 0 {
		merged.Transport = userAPI.Transport
	} else {
		merged.Transport = extractedAPI.Transport
	}

	// Lifecycle
	if userAPI.LifeCycleStatus != nil && string(*userAPI.LifeCycleStatus) != "" {
		merged.LifeCycleStatus = userAPI.LifeCycleStatus
	} else {
		merged.LifeCycleStatus = extractedAPI.LifeCycleStatus
	}

	// Upstream
	if !isEmptyUpstreamAPI(userAPI.Upstream) {
		merged.Upstream = userAPI.Upstream
	} else {
		merged.Upstream = extractedAPI.Upstream
	}

	// Policies/channels are only from user input
	merged.Policies = userAPI.Policies
	merged.Channels = userAPI.Channels

	// Operations: prefer extracted ops when available
	if extractedAPI.Operations != nil && len(*extractedAPI.Operations) > 0 {
		merged.Operations = extractedAPI.Operations
	} else {
		merged.Operations = userAPI.Operations
	}

	return merged
}

func isEmptyUpstreamAPI(upstream api.Upstream) bool {
	if !isEmptyUpstreamDefinitionAPI(upstream.Main) {
		return false
	}
	if upstream.Sandbox != nil && !isEmptyUpstreamDefinitionAPI(*upstream.Sandbox) {
		return false
	}
	return true
}

func isEmptyUpstreamDefinitionAPI(definition api.UpstreamDefinition) bool {
	if definition.Url != nil && *definition.Url != "" {
		return false
	}
	if definition.Ref != nil && *definition.Ref != "" {
		return false
	}
	return true
}
