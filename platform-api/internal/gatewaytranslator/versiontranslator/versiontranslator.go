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

// Package versiontranslator routes a gateway-latest-shaped deployment artifact
// to the sub-package that knows how to produce the shape a given target
// gateway data version expects. It holds no transform logic itself — that
// lives in the target-specific sub-package (e.g. v1alpha1), which is
// self-contained: its own apiVersion swap, its own per-kind shape handlers.
//
// Adding support for a new target data version is a new sibling package plus
// one case in DownConvert — existing kinds and their shape handlers are
// untouched.
package versiontranslator

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/versiontranslator/v1alpha1"
)

// knownKinds documents the artifact kinds this translator is aware of. It
// exists for coverage/visibility only: DownConvert works for any kind whose
// payload implements the target package's deploymentArtifact interface,
// listed here or not — see v1alpha1.DownConvert's default apiVersion swap.
var knownKinds = []string{
	"RestApi", "WebSubApi", "WebBrokerApi", "Mcp", "LlmProvider", "LlmProxy",
}

// KnownKinds returns the artifact kinds this translator is aware of.
func KnownKinds() []string {
	out := make([]string, len(knownKinds))
	copy(out, knownKinds)
	return out
}

// DownConvert produces, in place, the shape targetDataVersion expects for
// artifact (already in gateway-latest shape).
func DownConvert(kind string, targetDataVersion string, artifact any) error {
	switch targetDataVersion {
	case v1alpha1.DataVersion:
		return v1alpha1.DownConvert(kind, artifact)
	default:
		return fmt.Errorf("versiontranslator: unsupported target data version %q", targetDataVersion)
	}
}
