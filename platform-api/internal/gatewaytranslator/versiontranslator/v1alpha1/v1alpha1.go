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

// Package v1alpha1 produces deployment artifacts for gateways older than
// gatewaytranslator.MinGatewayV1Version — gateways whose CRD apiVersion is
// "gateway.api-platform.wso2.com/v1alpha1". It is self-contained: the
// apiVersion swap, the deploymentArtifact interface it uses, and any
// kind-specific data-shape transform all live here.
package v1alpha1

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// DataVersion is the gateway data version this package produces artifacts
// for. Matched against gatewaytranslator.GatewayDataVersion by the parent
// versiontranslator router.
const DataVersion = "v1alpha1"

// deploymentArtifact is satisfied by every *DeploymentYAML type. Declared
// here — not in the parent versiontranslator package — because setting the
// v1alpha1 apiVersion string is itself a v1alpha1-specific concern: it is
// kind-agnostic, but tied to producing a v1alpha1 artifact.
type deploymentArtifact interface {
	GetApiVersion() string
	SetApiVersion(string)
}

// shapeHandler down-converts artifact (a pointer to one of the *DeploymentYAML
// structs) from gateway-latest shape to what a v1alpha1 gateway understands,
// in place. It does NOT set apiVersion — DownConvert applies that
// unconditionally afterwards, for every kind.
type shapeHandler func(artifact any) error

// shapeHandlers maps kind -> its shape handler. Kinds absent here need no
// shape change beyond the apiVersion swap (REST, MCP, WebSub, WebBroker, and
// any future/unlisted kind) and fall through to the swap-only default.
// Populated by each kind file's init() — today only llm.go.
var shapeHandlers = map[string]shapeHandler{}

// DownConvert produces a v1alpha1 artifact for kind: it runs the per-kind
// shape handler if one is registered, then unconditionally applies the
// apiVersion swap via the deploymentArtifact interface. A kind with no shape
// handler still gets the swap — this is the safety net that keeps any
// REST/MCP/WebSub/WebBroker/future kind from shipping a v1 artifact to a
// gateway that only understands v1alpha1.
func DownConvert(kind string, payload any) error {
	if handler, ok := shapeHandlers[kind]; ok {
		if err := handler(payload); err != nil {
			return err
		}
	}
	artifact, ok := payload.(deploymentArtifact)
	if !ok {
		return fmt.Errorf("v1alpha1: %T does not implement deploymentArtifact", payload)
	}
	artifact.SetApiVersion(constants.GatewayApiVersionV1Alpha1)
	return nil
}
