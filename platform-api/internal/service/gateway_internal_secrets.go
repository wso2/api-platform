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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/secretinline"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Delivery-time secret rendering.
//
// Stored deployment content always keeps its {{ secret "handle" }}
// placeholders: artifact_secret_refs is rebuilt from that content on
// acknowledgement, which is what protects a referenced secret from deletion.
// Gateways from gwversion.MinSecretSyncGatewayVersion on pull the values
// themselves. Older gateways cannot, so when one of them fetches an artifact
// the placeholders are replaced with plaintext here, in the response only.

// SetSecretService injects the secret store used to render placeholders for
// gateways that cannot resolve them. Wired by the server after construction,
// like the other services' SetSecretService. Without it such a gateway's
// fetch fails rather than receiving an artifact it cannot use.
func (s *GatewayInternalAPIService) SetSecretService(svc *SecretService) {
	s.secretService = svc
}

// gatewayForDelivery loads the gateway an artifact is being delivered to and
// confirms it belongs to the calling organization.
func (s *GatewayInternalAPIService) gatewayForDelivery(orgID, gatewayID string) (*model.Gateway, error) {
	gateway, err := s.gatewayRepo.GetByUUID(gatewayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway: %w", err)
	}
	if gateway == nil || gateway.OrganizationID != orgID {
		return nil, apperror.GatewayNotFound.New()
	}
	return gateway, nil
}

// renderContentForGateway returns content as the gateway must receive it.
// A gateway that syncs secrets gets the stored bytes unchanged. An older
// gateway gets every placeholder replaced with the decrypted value. A secret
// that cannot be resolved (missing, deprecated or not decryptable) fails the
// render with apperror.DeploymentSecretResolutionFailed, and a missing secret
// store fails it too: a placeholder shipped to such a gateway can never
// resolve.
func (s *GatewayInternalAPIService) renderContentForGateway(orgID string, gateway *model.Gateway, content []byte) ([]byte, error) {
	if !gatewaytranslator.RequiresInlineSecrets(gateway.Version) {
		return content, nil
	}
	if !constants.SecretPlaceholderRe.Match(content) {
		return content, nil
	}
	if s.secretService == nil {
		return nil, fmt.Errorf("secret service not configured: cannot inline secrets for gateway %s (version %q)", gateway.ID, gateway.Version)
	}
	return secretinline.Render(content, func(handle string) (string, error) {
		value, err := s.secretService.Decrypt(orgID, handle)
		if err != nil {
			return "", apperror.DeploymentSecretResolutionFailed.Wrap(err)
		}
		return value, nil
	})
}

// deliverDeployment is gatewayForDelivery followed by renderContentForGateway,
// for the single-artifact fetch paths. A render failure is recorded on the
// deployment before the error is returned, so the operator sees why the
// gateway was refused it: the gateway's own failed ack only says it could not
// process the deployment.
func (s *GatewayInternalAPIService) deliverDeployment(orgID, gatewayID string, deployment *model.Deployment) ([]byte, error) {
	gateway, err := s.gatewayForDelivery(orgID, gatewayID)
	if err != nil {
		return nil, err
	}
	content, err := s.renderContentForGateway(orgID, gateway, deployment.Content)
	if err != nil {
		s.recordSecretResolutionFailure(orgID, gatewayID, deployment.ArtifactID, deployment.DeploymentID)
		return nil, err
	}
	return content, nil
}

// recordSecretResolutionFailure sets a deployment the gateway was refused to
// FAILED with reason SECRET_RESOLUTION_FAILED. Its desired state stays
// DEPLOYED so the next startup sync asks for it again. performed_at is
// re-stamped with now on purpose: the gateway acks the deploy event it was
// refused as failed with its generic GATEWAY_PROCESSING_ERROR, and that ack
// is guarded by the event's performed_at, so it is discarded instead of
// replacing the reason recorded here.
func (s *GatewayInternalAPIService) recordSecretResolutionFailure(orgID, gatewayID, artifactID, deploymentID string) {
	if _, err := s.deploymentRepo.SetCurrentWithDetails(artifactID, orgID, gatewayID, deploymentID,
		model.DeploymentStatusFailed, string(model.DeploymentStatusDeployed), nil,
		model.DeploymentErrorSecretResolutionFailed); err != nil {
		s.slogger.Error("Failed to record secret resolution failure on deployment status",
			"deploymentID", deploymentID, "gatewayID", gatewayID, "error", err)
	}
}
