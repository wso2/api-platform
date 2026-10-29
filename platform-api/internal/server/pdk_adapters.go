/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package server

// pdk_adapters.go bridges the internal service layer and the stable PDK interfaces
import (
	"context"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/service"
)

// gatewaysPDKAdapter satisfies pdk.Gateways.
type gatewaysPDKAdapter struct{ *service.GatewayService }

func (a *gatewaysPDKAdapter) RegisterGateway(orgID string, id *string, displayName, description string,
	endpoints []string, isCritical bool, functionalityType, version, createdBy string,
	properties map[string]any) (*api.GatewayResponse, error) {
	return a.GatewayService.RegisterGateway(context.Background(), orgID, id, displayName, description,
		endpoints, isCritical, functionalityType, version, createdBy, properties)
}

func (a *gatewaysPDKAdapter) UpdateGateway(gatewayID, orgID, updatedBy string, req *api.GatewayResponse) (*api.GatewayResponse, error) {
	return a.GatewayService.UpdateGateway(context.Background(), gatewayID, orgID, updatedBy, req)
}

func (a *gatewaysPDKAdapter) DeleteGateway(gatewayID, orgID, deletedBy string) error {
	return a.GatewayService.DeleteGateway(context.Background(), gatewayID, orgID, deletedBy)
}

// projectsPDKAdapter satisfies pdk.Projects.
type projectsPDKAdapter struct{ *service.ProjectService }

func (a *projectsPDKAdapter) CreateProject(req *api.CreateProjectRequest, organizationID, actor string) (*api.Project, error) {
	return a.ProjectService.CreateProject(context.Background(), req, organizationID, actor)
}

func (a *projectsPDKAdapter) DeleteProject(handle, orgID, actor string) error {
	return a.ProjectService.DeleteProject(context.Background(), handle, orgID, actor)
}

// apiPortalsPDKAdapter satisfies pdk.APIPortals.
type apiPortalsPDKAdapter struct{ *service.APIPortalService }

func (a *apiPortalsPDKAdapter) CreateAPIPortal(req *api.CreateApiPortalRequest, orgID, createdBy string) (*api.ApiPortalResponse, error) {
	return a.APIPortalService.CreateAPIPortal(context.Background(), req, orgID, createdBy)
}

func (a *apiPortalsPDKAdapter) UpdateAPIPortal(handle string, req *api.UpdateApiPortalRequest, orgID, updatedBy string) (*api.ApiPortalResponse, error) {
	return a.APIPortalService.UpdateAPIPortal(context.Background(), handle, req, orgID, updatedBy)
}

func (a *apiPortalsPDKAdapter) DeleteAPIPortal(handle, orgID, actor string) error {
	return a.APIPortalService.DeleteAPIPortal(context.Background(), handle, orgID, actor)
}

func (a *apiPortalsPDKAdapter) CreateAPIPortalWithStatus(req *api.CreateApiPortalRequest, orgID, createdBy, status string) (*api.ApiPortalResponse, error) {
	return a.APIPortalService.CreateAPIPortalWithStatus(context.Background(), req, orgID, createdBy, status)
}
