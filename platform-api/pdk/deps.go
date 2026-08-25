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

package pdk

import (
	"log/slog"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/config"
)

// Deps gives an external plugin the platform's capabilities as interfaces grouped
// by area, using only public types. It never hands over repositories, DB handles,
// or concrete internal service types — the type system keeps model.* / repository.*
// from leaking out.
//
// Capabilities are added here as external plugins need them. Each interface is
// satisfied by shape by the concrete internal service — the methods listed are
// exactly existing service methods that already speak public types — so exposing
// one is a plain assignment in the server (see StartPlatformAPIServer), with no
// adapter code. The assignment itself is the compile-time contract check: if a
// signature drifts, the server stops building.
type Deps struct {
	Gateways      Gateways
	Projects      Projects
	RestAPIs      RestAPIs
	Organizations Organizations
	Projects      Projects
	// add more capability groups as external plugins need them
	// (APIs, Subscriptions, Applications, Organizations, LLM, MCP, …)

	Config *config.Server
	Logger *slog.Logger
}

// Projects exposes the minimum project identity capability external plugins
// need to validate and query project-scoped resources. The internal UUID is
// returned only after the handle has been resolved inside the authenticated
// organization.
type Projects interface {
	GetProjectInternalID(handle, orgID string) (string, error)
}

// Organizations exposes the minimum read-only organization identity capability
// needed by external plugins. The returned external ID is the identity-provider
// organization reference stored when the platform organization is registered;
// callers must continue to use the internal orgID for platform authorization.
type Organizations interface {
	GetOrganizationExternalID(orgID string) (string, error)
}

// RestAPIs exposes the minimum read-only API capability external plugins need
// to validate that a resource belongs to the authenticated organization. It
// intentionally does not expose repositories or mutation methods.
type RestAPIs interface {
	// GetAPIByHandle returns an API only when handle belongs to orgID.
	GetAPIByHandle(handle, orgID string) (*api.RESTAPI, error)
}

// Gateways exposes CRUD access to the platform's gateways, scoped by organization.
// Every method mirrors an existing GatewayService method verbatim and takes the
// organization id explicitly — handlers MUST pass the org resolved from the
// request context, never one from request input (GO-AUTH-005).
type Gateways interface {
	// RegisterGateway creates a gateway in an organization (Create).
	RegisterGateway(orgID string, id *string, displayName, description string, endpoints []string,
		isCritical bool, functionalityType, version, createdBy string, properties map[string]any) (*api.GatewayResponse, error)

	// GetGateway returns a single gateway by id within an organization (Read).
	GetGateway(gatewayID, orgID string) (*api.GatewayResponse, error)

	// UpdateGateway updates a gateway within an organization (Update).
	UpdateGateway(gatewayID, orgID, updatedBy string, req *api.GatewayResponse) (*api.GatewayResponse, error)

	// DeleteGateway removes a gateway within an organization (Delete).
	DeleteGateway(gatewayID, orgID, deletedBy string) error
}

// Projects exposes create/read/delete access to the platform's projects, scoped
// by organization. Every method mirrors an existing ProjectService method verbatim
// and takes the organization id explicitly — handlers MUST pass the org resolved
// from the request context, never one from request input (GO-AUTH-005).
type Projects interface {
	// CreateProject creates a project in an organization (Create).
	CreateProject(req *api.CreateProjectRequest, organizationID, actor string) (*api.Project, error)

	// GetProjectByHandle returns a single project by its handle within an
	// organization (Read).
	GetProjectByHandle(handle, orgID string) (*api.Project, error)

	// DeleteProject removes a project within an organization (Delete).
	DeleteProject(handle, orgID, actor string) error
}
