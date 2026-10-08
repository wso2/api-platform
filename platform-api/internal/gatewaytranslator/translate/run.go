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

package translate

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
)

// apiVersionAccessor is satisfied by every *DeploymentYAML type. It is what the
// apiVersion swap depends on, and it is the only shape Run requires of an
// artifact; everything kind-specific is behind Kind.Normalize and Kind.Steps.
type apiVersionAccessor interface {
	GetApiVersion() string
	SetApiVersion(string)
}

// Run adapts artifact (a *DeploymentYAML pointer, mutated in place) for the
// gateway that reported gatewayVersion:
//
//  1. k.Normalize brings the artifact from sourceDataVersion up to the latest
//     shape (no-op when nil).
//  2. Each of k.Steps runs, in order, when gwversion.Below(gatewayVersion,
//     step.Below) holds.
//  3. The CRD apiVersion is swapped to v1alpha1 when the gateway is below
//     gwversion.MinGatewayV1Version. This step belongs to Run, not to any kind,
//     so a kind definition cannot omit it.
//
// A blank or non-semver gatewayVersion is a current build: step 1 runs, steps
// 2 and 3 do nothing. The returned Report lists every lossy decision; the
// error is reserved for programming mistakes such as a payload of the wrong
// type, and leaves the artifact in an undefined state.
func Run(k Kind, sourceDataVersion, gatewayVersion string, artifact any) (Report, error) {
	var r Report
	if k.Normalize != nil {
		if err := k.Normalize(sourceDataVersion, artifact); err != nil {
			return r, fmt.Errorf("translate %s: normalize: %w", k.GatewayKind, err)
		}
	}
	for _, step := range k.Steps {
		if !gwversion.Below(gatewayVersion, step.Below) {
			continue
		}
		if err := step.Apply(artifact, &r); err != nil {
			return r, fmt.Errorf("translate %s: %s: %w", k.GatewayKind, step.Name, err)
		}
	}
	if gwversion.Below(gatewayVersion, gwversion.MinGatewayV1Version) {
		accessor, ok := artifact.(apiVersionAccessor)
		if !ok {
			return r, fmt.Errorf("translate %s: %T has no apiVersion accessors", k.GatewayKind, artifact)
		}
		accessor.SetApiVersion(constants.GatewayApiVersionV1Alpha1)
	}
	return r, nil
}
