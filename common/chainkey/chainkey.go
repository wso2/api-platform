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

// Package chainkey composes the key of a policy chain that belongs to one logical
// operation of a multiplexed API.
//
// It lives in common because both sides of the contract need it and neither can
// import the other: the gateway controller writes policy chains under these keys,
// and the policy engine composes the same key at request time to look one up. They
// are separate Go modules, and the engine's copy is under internal/, so a single
// shared construction is the only way the two cannot drift. A key that the engine
// composes and the controller never emitted is a request-time failure with no
// deploy-time signal, which is exactly the failure mode this package removes.
//
// The composition is a pure function of the operation. That is what makes two
// transports of one logical operation (A2A JSON-RPC and A2A HTTP+JSON) select the
// same chain without either being told about the other.
package chainkey

import "strings"

// Separator joins the key's components. ASCII US (unit separator, 0x1f) is used
// because it cannot appear in an HTTP-safe identifier — an API id, a vhost, or an
// operation name — so the join is unambiguous and, unlike a printable separator,
// needs no escaping rule.
//
// Contrast common/apikey's entity id, which joins on "_" and therefore has to guess
// the split with strings.LastIndex; that is ambiguous the moment a component
// contains the separator.
const Separator = "\x1f"

// For composes the policy chain key for one operation of one routing partition.
//
// vhost alone represents the routing partition, so two routes that differ only by a
// header match would compose the same key. Callers that can produce such a pair must
// reject that configuration rather than let two partitions collide here.
func For(apiID, vhost, operation string) string {
	return apiID + Separator + vhost + Separator + operation
}

// Split decomposes a key produced by For. ok is false for anything that is not one —
// a route-key chain, or a malformed composed key.
//
// It lives here rather than at either call site because a caller that re-implements the
// split is a second place the format is encoded, which is the drift this package exists
// to prevent. The vhost is allowed to be empty (that is the default vhost); the API id
// and the operation are not.
func Split(key string) (apiID, vhost, operation string, ok bool) {
	parts := strings.Split(key, Separator)
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// IsComposed reports whether key was produced by For rather than being a route-key chain.
// A malformed key containing the separator counts as composed, so a caller can tell
// "not one of these" apart from "one of these, built wrong" and report the second.
func IsComposed(key string) bool {
	return strings.Contains(key, Separator)
}

// ValidComponent reports whether s can be used as a key component: non-empty and
// free of the separator.
//
// Operation identifiers are the case that matters. An API id and a vhost are
// server-derived, but an operation identifier can come from user-controlled space (an
// MCP tool name), and one containing the separator could otherwise compose the same
// key as a different (apiID, vhost, operation) triple. A resolver over such a space
// must reject an identifier this returns false for rather than escape it — escaping
// would need the same rule implemented identically on both sides again.
func ValidComponent(s string) bool {
	return s != "" && !strings.Contains(s, Separator)
}
