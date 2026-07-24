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
	Gateways Gateways
	// add more capability groups as external plugins need them
	// (APIs, Subscriptions, Applications, Projects, Organizations, LLM, MCP, …)

	Config *config.Server
	Logger *slog.Logger
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
