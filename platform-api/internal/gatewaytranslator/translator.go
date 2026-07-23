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

// Package gatewaytranslator adapts a deployment artifact between two version
// axes:
//
//   - the platform data version the artifact was generated from (the shape
//     platform-api stored it as — see PlatformDataVersion), and
//   - the gateway data version the target gateway accepts (its CRD apiVersion,
//     derived from its semver — see GatewayDataVersion).
//
// Generators always produce the canonical, gateway-latest artifact shape.
// Translate first normalizes the artifact up to that shape (a no-op unless
// the stored source lags behind), then — only if the target gateway is older
// than latest — down-converts to whatever shape that gateway understands.
// This is invoked in the deploy orchestration layer, before the artifact is
// marshalled and stored; the deploy services call only Translate.
package gatewaytranslator

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/normalizer"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/versiontranslator"
)

// Translate adapts artifact (a pointer to one of the *DeploymentYAML structs,
// mutated in place) so the target gateway can consume it.
//
//   - kind is the artifact kind (e.g. constants.LLMProvider).
//   - sourceDataVersion is the platform data version the artifact was
//     generated from.
//   - targetDataVersion is the gateway data version the target gateway
//     accepts (see TargetGatewayDataVersion).
func Translate(kind string, sourceDataVersion PlatformDataVersion, targetDataVersion GatewayDataVersion, artifact any) error {
	if err := normalizer.Normalize(kind, string(sourceDataVersion), artifact); err != nil {
		return fmt.Errorf("gatewaytranslator: normalize %q: %w", kind, err)
	}
	if targetDataVersion == GatewayDataVersionV1 {
		return nil
	}
	if err := versiontranslator.DownConvert(kind, string(targetDataVersion), artifact); err != nil {
		return fmt.Errorf("gatewaytranslator: down-convert %q to %s: %w", kind, targetDataVersion, err)
	}
	return nil
}
