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
// gateway gets every placeholder replaced with the decrypted value; a
// deprecated or missing secret, or a missing secret store, fails the render,
// because a placeholder shipped to such a gateway can never resolve.
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
		return s.secretService.Decrypt(orgID, handle)
	})
}

// deliverContent is gatewayForDelivery followed by renderContentForGateway,
// for the single-artifact fetch paths.
func (s *GatewayInternalAPIService) deliverContent(orgID, gatewayID string, content []byte) ([]byte, error) {
	gateway, err := s.gatewayForDelivery(orgID, gatewayID)
	if err != nil {
		return nil, err
	}
	return s.renderContentForGateway(orgID, gateway, content)
}
