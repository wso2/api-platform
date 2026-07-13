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

// Package clusterkey produces deterministic Envoy cluster names for the
// gateway-controller, shared by the RDC transformer and the xDS translator so
// they name clusters identically.
package clusterkey

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// Hash returns the full SHA-256 hash in hex representation.
func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// HashedName joins a prefix string and the full SHA-256 hash of value with an underscore.
func HashedName(prefix, value string) string {
	return prefix + "_" + Hash(value)
}

// DefinitionName returns the full Envoy cluster name for an upstream definition,
// formatted as "upstream_<kind>_<apiID>_<sanitized name>". Dots and colons in the
// definition name are replaced so the result is a valid Envoy cluster name. The
// RDC transformer and the xDS translator use this so they name definition
// clusters identically.
func DefinitionName(kind, apiID, defName string) string {
	return constants.UpstreamDefinitionClusterPrefix + kind + "_" + apiID + "_" + sanitizeDefName(defName)
}

// sanitizeDefName replaces dots and colons, which are not allowed in Envoy cluster names.
func sanitizeDefName(name string) string {
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return name
}
