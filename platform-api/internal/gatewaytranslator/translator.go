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

package gatewaytranslator

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/kinds"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
)

// Report lists the lossy decisions one translation took. Deploy services log
// it with the deployment and gateway it belongs to (service.LogTranslationWarnings).
type Report = translate.Report

// Warning is one entry of a Report.
type Warning = translate.Warning

// MinGatewayV1Version is re-exported for callers that only need the apiVersion
// boundary and should not have to import gwversion for it.
const MinGatewayV1Version = gwversion.MinGatewayV1Version

// RequiresInlineSecrets reports whether a gateway that reported gatewayVersion
// needs {{ secret }} placeholders replaced with plaintext before it fetches an
// artifact. It is the delivery path's gate for secretinline.Render.
func RequiresInlineSecrets(gatewayVersion string) bool {
	return gwversion.RequiresInlineSecrets(gatewayVersion)
}

// Translate adapts artifact (a pointer to one of the *DeploymentYAML structs,
// mutated in place) for the gateway that reported gatewayVersion.
//
//   - kind is the artifact kind as the gateway names it (constants.RestApi,
//     constants.MCPProxy, ..., constants.GatewayKindAgent).
//   - sourceDataVersion is the platform data version the artifact was stored
//     at (the build's or artifact row's data_version).
//   - gatewayVersion is model.Gateway.Version as reported; blank or non-semver
//     means a current build and only the up-conversion runs.
//
// It is invoked in the deploy orchestration layer, before the artifact is
// marshalled and stored. A kind without a definition is an error: every kind
// the control plane can deploy is listed in kinds.All, and a test keeps that
// table and the kind-name map in step.
func Translate(kind string, sourceDataVersion PlatformDataVersion, gatewayVersion string, artifact any) (Report, error) {
	k, ok := kinds.Lookup(kind)
	if !ok {
		return Report{}, fmt.Errorf("gatewaytranslator: no translator registered for kind %q", kind)
	}
	return translate.Run(k, string(sourceDataVersion), gatewayVersion, artifact)
}
