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
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
)

// Agent Card fetch request decoding.
//
// FetchAgentCardRequest is a `oneOf` of two disjoint branches, and the
// generated union type cannot enforce that: it decodes every declared property
// into a pointer on one flat struct, so a body carrying both `agentProxyId` and
// `url` arrives looking exactly like one carrying either. Worse, a *null* value
// decodes into the same nil pointer an omitted key does — and the contract
// rejects `{"agentProxyId": "x", "url": null}` precisely because `required` is
// about key presence, not about whether the value is useful.
//
// Both distinctions only exist while the raw bytes are still in hand, so the
// decode happens here, against the raw object, before anything typed is built.

// Length bounds on agentProxyId, mirroring FetchAgentCardByAgentProxy's
// minLength/maxLength. A handle failing them cannot name a stored Agent proxy,
// so it is a 400 rather than a lookup that is certain to 404.
const (
	fetchAgentCardHandleMinLen = 3
	fetchAgentCardHandleMaxLen = 40
)

// fetchAgentCardDeclaredFields is the closed set of properties the request
// carries, across both branches. Anything else is rejected: both branches
// declare additionalProperties: false.
var fetchAgentCardDeclaredFields = map[string]struct{}{
	"url":          {},
	"auth":         {},
	"agentProxyId": {},
}

// AgentCardFetchRequest is the decoded, disjoint form of an Agent Card fetch
// request. Exactly one of the two forms is populated, decided by which keys the
// body carried — never by which values happen to be non-empty.
type AgentCardFetchRequest struct {
	// AgentProxyID is set for the stored form and empty for the direct-URL form.
	AgentProxyID string
	// URL and Auth belong to the direct-URL form only; the stored form takes its
	// endpoint and credentials from the Agent proxy row.
	URL  string
	Auth *api.UpstreamAuth
}

// IsStored reports whether this request names a saved Agent proxy rather than
// supplying its own endpoint.
func (r *AgentCardFetchRequest) IsStored() bool {
	return r != nil && r.AgentProxyID != ""
}

// DecodeAgentCardFetchRequest turns a fetch request body into exactly one of the
// two forms the contract allows.
//
// Every error it returns is phrased for the caller — it describes the caller's
// own payload and never a Go type or decoder internal — so a service can hand
// it straight to apperror.ValidationFailed. Nothing here resolves a credential
// or issues an outbound request; that is the point of rejecting an invalid
// combination at this stage.
func DecodeAgentCardFetchRequest(data []byte) (*AgentCardFetchRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.New("The request body must be a JSON object.")
	}

	if err := assertFetchAgentCardFieldsDeclared(raw); err != nil {
		return nil, err
	}

	_, hasHandle := raw["agentProxyId"]
	_, hasURL := raw["url"]
	_, hasAuth := raw["auth"]

	switch {
	case hasHandle && (hasURL || hasAuth):
		// Presence, not value: a null url or auth alongside a handle is still a
		// supplied url or auth, and the stored form permits neither. Rejected
		// here, before any credential is resolved or any request is made.
		return nil, errors.New(
			"A request that supplies agentProxyId must not also supply url or auth. " +
				"Use agentProxyId alone to fetch with the Agent proxy's stored endpoint and credentials, " +
				"or url (optionally with auth) to fetch from a supplied endpoint.")
	case hasHandle:
		handle, err := fetchAgentCardString(raw["agentProxyId"], "agentProxyId")
		if err != nil {
			return nil, err
		}
		if len(handle) < fetchAgentCardHandleMinLen || len(handle) > fetchAgentCardHandleMaxLen {
			return nil, fmt.Errorf("The agentProxyId field must be between %d and %d characters.",
				fetchAgentCardHandleMinLen, fetchAgentCardHandleMaxLen)
		}
		return &AgentCardFetchRequest{AgentProxyID: handle}, nil
	case hasURL:
		url, err := fetchAgentCardString(raw["url"], "url")
		if err != nil {
			return nil, err
		}
		auth, err := decodeFetchAgentCardAuth(raw, hasAuth)
		if err != nil {
			return nil, err
		}
		return &AgentCardFetchRequest{URL: url, Auth: auth}, nil
	default:
		// Covers both the empty object and an auth-only body: auth is a modifier
		// on a direct fetch, never a fetch target of its own.
		return nil, errors.New("The request must supply either url or agentProxyId.")
	}
}

// assertFetchAgentCardFieldsDeclared rejects any property outside the closed set
// both branches declare.
func assertFetchAgentCardFieldsDeclared(raw map[string]json.RawMessage) error {
	var unknown []string
	for key := range raw {
		if _, ok := fetchAgentCardDeclaredFields[key]; !ok {
			unknown = append(unknown, strconv.Quote(key))
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("The request body contains unsupported fields: %s.", strings.Join(unknown, ", "))
	}
	return nil
}

// fetchAgentCardString reads a supplied identifier, rejecting null and empty
// alike: the key was supplied, so it names a value the caller meant to use.
func fetchAgentCardString(raw json.RawMessage, field string) (string, error) {
	if isJSONNull(raw) {
		return "", fmt.Errorf("The field %s must not be null. Omit it instead.", strconv.Quote(field))
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("The field %s must be a string.", strconv.Quote(field))
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("The field %s must not be empty.", strconv.Quote(field))
	}
	return value, nil
}

// decodeFetchAgentCardAuth decodes the optional direct-fetch credentials. The
// same declared-fields walk the Agent proxy body gets applies here, so a
// misspelled auth property is a 400 rather than a fetch that silently sends no
// credential at all.
func decodeFetchAgentCardAuth(raw map[string]json.RawMessage, hasAuth bool) (*api.UpstreamAuth, error) {
	if !hasAuth {
		return nil, nil
	}
	body := raw["auth"]
	if isJSONNull(body) {
		return nil, errors.New("The field \"auth\" must not be null. Omit it instead.")
	}
	if err := assertDeclaredFields(body, reflect.TypeOf(api.UpstreamAuth{}), "auth"); err != nil {
		return nil, err
	}
	var auth api.UpstreamAuth
	if err := json.Unmarshal(body, &auth); err != nil {
		return nil, clientDecodeError(err)
	}
	if auth.Type == nil {
		return nil, errors.New("The auth type field is required when auth is supplied.")
	}
	if !auth.Type.Valid() {
		return nil, fmt.Errorf("The auth type %q is not supported.", string(*auth.Type))
	}
	return &auth, nil
}
