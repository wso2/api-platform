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

package service

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"
)

// GetAPIInvokeTarget returns where a test request for the GraphQL API apiHandle
// goes on gatewayHandle: the gateway's endpoints and the context of the API's
// deployment on it. The API must belong to orgID, and a gateway it is only
// associated with is refused, as for a REST API. Vhost is always empty: a GraphQL
// API is served on the gateway's default vhost.
func (s *GraphQLAPIService) GetAPIInvokeTarget(apiHandle, gatewayHandle, orgID string) (*APIInvokeTarget, error) {
	apiModel, err := s.repo.GetByHandle(apiHandle, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get GraphQL API: %w", err)
	}
	if apiModel == nil || apiModel.OrganizationID != orgID {
		return nil, apperror.GraphQLAPINotFound.New()
	}

	gateways, err := s.repo.GetAPIGatewaysWithDetails(apiModel.ID, orgID)
	if err != nil {
		return nil, err
	}
	for _, gw := range gateways {
		if gw == nil || gw.Handle != gatewayHandle {
			continue
		}
		if !gw.IsDeployed {
			break
		}
		deployed, err := s.deploymentRepo.GetCurrentByGateway(apiModel.ID, gw.ID, orgID)
		if err != nil {
			return nil, err
		}
		if deployed == nil {
			break
		}
		var artifact dto.GraphQLAPIDeploymentYAML
		if err := yaml.Unmarshal(deployed.Content, &artifact); err != nil {
			return nil, fmt.Errorf("failed to read deployment %s of GraphQL API %s: %w", deployed.DeploymentID, apiModel.ID, err)
		}
		apiContext := artifact.Spec.Context
		if apiContext == "" && apiModel.Configuration.Context != nil {
			apiContext = *apiModel.Configuration.Context
		}
		version := artifact.Spec.Version
		if version == "" {
			version = apiModel.Version
		}
		return &APIInvokeTarget{
			Context:   strings.ReplaceAll(apiContext, "$version", version),
			Endpoints: gw.Endpoints,
		}, nil
	}
	return nil, apperror.GraphQLAPIInvokeGatewayNotDeployed.New()
}
