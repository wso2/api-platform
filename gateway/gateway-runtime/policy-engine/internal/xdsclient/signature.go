/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package xdsclient

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// routeSignatureView is the exact set of inputs buildPolicyChain consumes to
// construct a route's policy chain. Its purpose is change detection for
// per-route reconciliation: two snapshots that produce the SAME view must yield
// byte-identical JSON, and any change that would alter the built chain must
// alter the view.
//
// Deliberately EXCLUDED: the volatile Metadata fields (CreatedAt, UpdatedAt,
// ResourceVersion) which the controller bumps on every push, and Context which
// buildPolicyChain does not read — including any of them would make every route
// look changed on every snapshot and silently defeat the optimization.
//
// KEEP IN SYNC WITH buildPolicyChain (handler.go). New keys inside Parameters and
// new PolicyInstance fields are covered automatically (both are hashed wholesale).
// But Metadata is PROJECTED here, not hashed wholesale — so if buildPolicyChain
// starts consuming another apiMetadata.* field, it MUST be added to this view too,
// or reconciliation will reuse a stale chain. The completeness test does NOT catch
// that omission; it only proves the fields already listed here affect the hash.
//
// If a NEW field is added here, extend TestRouteSignatureView_Completeness with a
// mutator so a change to it is proven to alter the signature.
type routeSignatureView struct {
	RouteKey string `json:"route_key"`
	// Whole PolicyInstance list, in order. PolicyInstance carries only behavioral
	// fields (Name, Version, Enabled, ExecutionCondition, Parameters), so it is
	// marshaled wholesale — new behavioral fields are covered automatically.
	// Order is significant (execution order) and preserved by JSON arrays.
	Policies   []policyenginev1.PolicyInstance `json:"policies"`
	APIId      string                          `json:"api_id"`
	APIName    string                          `json:"api_name"`
	APIVersion string                          `json:"api_version"`
}

// routeSignature returns a stable content hash of the behavioral configuration
// that produces a route's policy chain. Identical config always yields the same
// signature; any change to a build input yields a different one.
//
// Canonicalization: encoding/json emits struct fields in declaration order and
// sorts map keys, so the output is deterministic for a given value. Parameters
// arrive as JSON-native types (float64/string/…) from the original unmarshal, so
// re-marshaling here round-trips stably. ${config} references are not resolved
// here on purpose: resolution is process-constant (from baked-in SystemParameters
// against PE-local config) and changes only on a restart, which rebuilds every
// chain from scratch anyway.
func routeSignature(config *policyenginev1.PolicyChain, md policyenginev1.Metadata) (string, error) {
	return signatureOf(routeSignatureView{
		RouteKey:   config.RouteKey,
		Policies:   config.Policies,
		APIId:      md.APIId,
		APIName:    md.APIName,
		APIVersion: md.Version,
	})
}

// signatureOf hashes a projection view. Kept separate so tests can exercise
// field sensitivity directly.
func signatureOf(view routeSignatureView) (string, error) {
	b, err := json.Marshal(view)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// shortSig returns the first 8 hex chars of a signature for compact logging.
func shortSig(sig string) string {
	if len(sig) <= 8 {
		return sig
	}
	return sig[:8]
}
