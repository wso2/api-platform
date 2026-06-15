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

// Package clusterkey produces deterministic, hex-encoded cluster-key fragments
// used by the gateway-controller to name Envoy clusters. It is a leaf package
// (stdlib imports only) so both xDS builders (pkg/xds and pkg/transform) can
// share one naming source without import cycles.
package clusterkey

import (
	"crypto/sha256"
	"encoding/hex"
)

// APILevel returns a deterministic, hex-encoded cluster-key fragment for an
// API-level upstream cluster. The key is derived from SHA-256 of the apiID
// alone, so an API's main and sandbox clusters share the same fragment and an
// operator can pair them at a glance; the env prefix the caller prepends
// ("main_"/"sandbox_") distinguishes them. The backend URL is deliberately
// excluded from the input so the cluster NAME stays stable across URL edits:
// host, port, or scheme changes reach Envoy as an update to the same named
// cluster (warmed and swapped) instead of removing one cluster name and
// adding another, and path-only changes touch just the route rewrite, leaving
// the cluster untouched. Routes and name-keyed stats stay continuous either
// way. Cross-API uniqueness rests on the 12-byte (96-bit) truncation, which
// makes collisions between distinct apiIDs cryptographically unlikely, not
// impossible.
func APILevel(apiID string) string {
	sum := sha256.Sum256([]byte(apiID))
	return hex.EncodeToString(sum[:12])
}

// APILevelName joins the env prefix ("main"/"sandbox") to the APILevel fragment
// to form the full Envoy cluster name, so both xDS builders name clusters identically.
func APILevelName(env, apiID string) string {
	return env + "_" + APILevel(apiID)
}
