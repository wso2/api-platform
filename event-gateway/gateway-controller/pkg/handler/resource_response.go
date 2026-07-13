/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handler

import (
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"

	eventgateway "github.com/wso2/api-platform/event-gateway/gateway-controller/pkg/api/eventgateway"
)

// buildResourceStatus builds the server-managed status block for a stored config.
func buildResourceStatus(cfg *models.StoredConfig) eventgateway.ResourceStatus {
	id := cfg.Handle
	state := eventgateway.ResourceStatusState(cfg.DesiredState)
	createdAt := cfg.CreatedAt
	updatedAt := cfg.UpdatedAt

	status := eventgateway.ResourceStatus{
		Id:        &id,
		State:     &state,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if cfg.DeployedAt != nil {
		deployedAt := *cfg.DeployedAt
		status.DeployedAt = &deployedAt
	}
	return status
}

// buildResourceResponse merges a WebSubAPI/WebBrokerApi configuration value
// with the server-managed status block. Any user-provided Status in the input
// is replaced with the authoritative server value. Unknown types are returned
// unchanged so callers can fall through to a generic JSON response if required.
func buildResourceResponse(cfg any, status eventgateway.ResourceStatus) any {
	switch v := cfg.(type) {
	case eventgateway.WebSubAPI:
		v.Status = &status
		return v
	case *eventgateway.WebSubAPI:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	case eventgateway.WebBrokerApi:
		v.Status = &status
		return v
	case *eventgateway.WebBrokerApi:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	default:
		return cfg
	}
}

// buildResourceResponseFromStored is a convenience wrapper combining
// buildResourceStatus and buildResourceResponse.
func buildResourceResponseFromStored(cfg any, stored *models.StoredConfig) any {
	return buildResourceResponse(cfg, buildResourceStatus(stored))
}
