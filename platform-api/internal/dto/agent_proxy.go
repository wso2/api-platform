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

package dto

import (
	"encoding/json"
	"sort"

	"github.com/wso2/api-platform/platform-api/api"
)

// The Agent proxy wire types come from api/generated.go — api.A2AAgentProxy,
// api.AgentProxyListItem, api.PublicAgentCard and the rest — so there is exactly
// one definition of each and it is generated from resources/openapi.yaml.
//
// The one exception is below: the Agent Card fetch request needs to tell an
// omitted key from one explicitly set to null, which no generated Go type can
// express. Everything else in this package is mapping, not redefinition.

// AgentProxyKind is the control-plane resource kind. The gateway's own artifact
// kind stays Agent and is mapped at the deployment boundary.
const AgentProxyKind = string(api.A2AAgentProxyKindAgentProxy)

// FetchAgentCardRequest is the Agent Card preview request.
//
// Exactly one of two forms is valid: a direct Url with optional Auth, or an
// AgentProxyId alone, which uses that Agent proxy's stored endpoint and stored
// credentials. The contract requires url and auth to be *rejected* when they
// accompany agentProxyId, including when their supplied value is null — so the
// decoded value cannot be the thing that is checked. A pointer left nil by
// `"url": null` is indistinguishable from one left nil by an absent key, which
// would turn a payload the contract forbids into the valid stored-fetch form.
//
// The schema gets this right on its own (JSON Schema `required` is about key
// presence, so an explicitly null url is still a supplied url), but no runtime
// OpenAPI validation is in play, so the Go type has to carry the distinction.
// UnmarshalJSON records which keys the caller actually sent and validation asks
// the Has* accessors rather than the fields.
type FetchAgentCardRequest struct {
	Url          *string           `json:"url,omitempty"`
	Auth         *api.UpstreamAuth `json:"auth,omitempty"`
	AgentProxyId *string           `json:"agentProxyId,omitempty"`

	// present holds the keys seen in the request body; unknown holds any key that
	// is not part of either branch, so a caller cannot smuggle one past a schema
	// that closes both branches with additionalProperties: false.
	present map[string]struct{}
	unknown []string
}

// fetchAgentCardKnownFields is the union of both branches' permitted properties.
var fetchAgentCardKnownFields = map[string]struct{}{
	"url":          {},
	"auth":         {},
	"agentProxyId": {},
}

// UnmarshalJSON decodes the request while recording key presence separately from
// decoded values.
func (r *FetchAgentCardRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// The alias sheds this method, so the nested call cannot recurse.
	type plain FetchAgentCardRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*r = FetchAgentCardRequest(decoded)
	r.present = make(map[string]struct{}, len(raw))
	r.unknown = nil
	for key := range raw {
		r.present[key] = struct{}{}
		if _, ok := fetchAgentCardKnownFields[key]; !ok {
			r.unknown = append(r.unknown, key)
		}
	}
	sort.Strings(r.unknown)
	return nil
}

// HasURL reports whether the caller sent a "url" key, whatever its value.
func (r *FetchAgentCardRequest) HasURL() bool { return r.hasField("url") }

// HasAuth reports whether the caller sent an "auth" key, whatever its value.
func (r *FetchAgentCardRequest) HasAuth() bool { return r.hasField("auth") }

// HasAgentProxyID reports whether the caller sent an "agentProxyId" key, whatever
// its value.
func (r *FetchAgentCardRequest) HasAgentProxyID() bool { return r.hasField("agentProxyId") }

// UnknownFields returns the sorted names of properties belonging to neither
// branch. Empty when the body is clean.
func (r *FetchAgentCardRequest) UnknownFields() []string {
	if r == nil || len(r.unknown) == 0 {
		return nil
	}
	out := make([]string, len(r.unknown))
	copy(out, r.unknown)
	return out
}

func (r *FetchAgentCardRequest) hasField(name string) bool {
	if r == nil {
		return false
	}
	_, ok := r.present[name]
	return ok
}
