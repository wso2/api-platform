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
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// MinGatewayV1Version is the first gateway release whose CRD apiVersion is
// "gateway.api-platform.wso2.com/v1". Every artifact kind flips apiVersion
// together at this boundary — it is a gateway-wide fact, not a per-kind one —
// so it lives here, in exactly one place. Kind-specific data-shape transforms
// (see versiontranslator/v1alpha1) never hardcode this constant; they are
// reached only because GatewayDataVersion already resolved the target using it.
const MinGatewayV1Version = "1.2.0"

// GatewayDataVersion is the CRD apiVersion a target gateway accepts, in
// normalized form.
type GatewayDataVersion string

const (
	// GatewayDataVersionV1 is the latest gateway artifact shape
	// (gateways >= MinGatewayV1Version).
	GatewayDataVersionV1 GatewayDataVersion = "v1"
	// GatewayDataVersionV1Alpha1 is the legacy shape understood by gateways
	// older than MinGatewayV1Version.
	GatewayDataVersionV1Alpha1 GatewayDataVersion = "v1alpha1"
)

// TargetGatewayDataVersion derives the gateway data version a target gateway
// accepts from its semver.
func TargetGatewayDataVersion(gatewayTargetVersion Version) GatewayDataVersion {
	if gatewayTargetVersion.AtLeast(ParseVersion(MinGatewayV1Version)) {
		return GatewayDataVersionV1
	}
	return GatewayDataVersionV1Alpha1
}

// GatewayDataVersionForGateway resolves the gateway data version a target
// gateway accepts from its raw reported version string (model.Gateway.Version).
//
// An empty/blank version is treated as the LATEST (v1): down-conversion is the
// lossy operation, so we only down-convert when a gateway POSITIVELY reports a
// version older than MinGatewayV1Version. A gateway may legitimately be
// registered without a version (its reported semver is not persisted onto the
// record), and mis-down-converting such a current gateway to v1alpha1 produces
// an artifact it will not route. Assuming latest for an unknown version matches
// the pre-translation behaviour (REST/MCP/WebSub always shipped v1) and keeps
// the down-convert path scoped to gateways that explicitly report < 1.2.0.
func GatewayDataVersionForGateway(rawVersion string) GatewayDataVersion {
	if strings.TrimSpace(rawVersion) == "" {
		return GatewayDataVersionV1
	}
	return TargetGatewayDataVersion(ParseVersion(rawVersion))
}

// PlatformDataVersion is the shape platform-api stored an entity as, recorded
// in the DB data_version column as "<major>.<minor>" (e.g. "1.0", "1.1").
type PlatformDataVersion string

// defaultPlatformDataVersion is used when the artifact's gateway apiVersion is
// missing or unparseable.
const defaultPlatformDataVersion PlatformDataVersion = "1.0"

// platformDataMinorVersions holds the current data-shape minor version for
// each artifact kind. BUMP the value here when a kind's stored data shape
// changes without a gateway apiVersion bump. Ported from
// gateway/gateway-controller/pkg/models/data_version.go — keep the two in
// sync so platform-api and the gateway controller compute the same
// data_version for the same inputs.
var platformDataMinorVersions = map[string]int{
	constants.RestApi:      0,
	constants.WebSubApi:    0,
	constants.WebBrokerApi: 0,
	constants.MCPProxy:     0,
	constants.LLMProxy:     1,
	constants.LLMProvider:  1,
}

// majorFromApiVersion parses ".../v<N>..." and returns "N" (the leading
// numeric run after "/v"). Any alpha/beta qualifier is stripped, so
// "gateway.api-platform.wso2.com/v1alpha1" -> "1". Returns "" if unparseable.
func majorFromApiVersion(apiVersion string) string {
	idx := strings.LastIndex(apiVersion, "/v")
	if idx == -1 {
		return ""
	}
	suffix := apiVersion[idx+2:] // text after "/v"
	major := suffix
	for i, r := range suffix {
		if r < '0' || r > '9' {
			major = suffix[:i]
			break
		}
	}
	if major == "" {
		return ""
	}
	return major
}

// ComputeDataVersion derives the stored data_version "<major>.<minor>" for
// kind from its gateway apiVersion (major) and the per-kind data-shape minor
// constant. Falls back to defaultPlatformDataVersion ("1.0") when apiVersion
// is empty or unparseable, or kind is unrecognized.
//
// Ported from gateway-controller's ComputeDataVersion
// (gateway/gateway-controller/pkg/models/data_version.go) so both sides
// compute the same value for the same inputs.
func ComputeDataVersion(kind string, apiVersion string) PlatformDataVersion {
	minor, known := platformDataMinorVersions[kind]
	if !known {
		return defaultPlatformDataVersion
	}
	major := majorFromApiVersion(apiVersion)
	if major == "" {
		return defaultPlatformDataVersion
	}
	return PlatformDataVersion(fmt.Sprintf("%s.%d", major, minor))
}
