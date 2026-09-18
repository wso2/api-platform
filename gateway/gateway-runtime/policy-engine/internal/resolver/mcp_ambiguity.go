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

package resolver

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// Members of the JSON-RPC envelope this resolver reads. A request that names any of
// them more than once, or under a spelling encoding/json would fold onto them, is
// ambiguous — see hasAmbiguousMembers.
var (
	mcpEnvelopeMembers = []string{"method", "id", "params"}
	mcpParamsMembers   = []string{"name", "uri", "clientInfo", "protocolVersion", "requestState", "_meta"}
	mcpMetaMembers     = []string{mcpMetaProtocolVersionKey, mcpMetaClientInfoKey}
)

// hasAmbiguousMembers reports whether the body names any member this resolver reads more
// than once. It must run before the unmarshal, which is where the duplicate is lost.
//
// encoding/json folds member names for case and keeps the last match, so a backend that
// matches exactly, or keeps the first, reads a different value from the same bytes:
//
//	{"method":"tools/list","method":"tools/call"}
//
// The gateway would govern tools/list while the server executes tools/call. Folding makes
// {"method":…,"Method":…} ambiguous too.
//
// An ambiguous body publishes no facts rather than failing the request: a resolution error
// replaces MCP's own JSON-RPC error shapes, and every consumer already fails closed on a
// missing fact.
func hasAmbiguousMembers(body []byte) bool {
	members, ok := jsonObjectMembers(body)
	if !ok {
		// Not an object, or unparseable. Resolve treats that as "publish nothing"
		// anyway, so there is no ambiguity question to answer.
		return false
	}
	if foldsOntoOneName(members, mcpEnvelopeMembers) {
		return true
	}

	for _, member := range members {
		if member.name != "params" || !isJSONObject(member.value) {
			continue
		}
		params, ok := jsonObjectMembers(member.value)
		if !ok {
			return false
		}
		if foldsOntoOneName(params, mcpParamsMembers) {
			return true
		}
		if hasAmbiguousMetaMembers(params) {
			return true
		}
	}
	return false
}

// hasAmbiguousMetaMembers applies the same rule one level down, to the reserved members of
// params._meta. A vendor member is left alone: only what this resolver reads can diverge.
func hasAmbiguousMetaMembers(params []jsonMember) bool {
	for _, member := range params {
		if member.name != "_meta" || !isJSONObject(member.value) {
			continue
		}
		meta, ok := jsonObjectMembers(member.value)
		if !ok {
			return false
		}
		if foldsOntoOneName(meta, mcpMetaMembers) {
			return true
		}
	}
	return false
}

// jsonMember is one object member, kept in document order so repeated names stay
// visible — which a map would discard.
type jsonMember struct {
	name  string
	value json.RawMessage
}

// jsonObjectMembers returns a JSON object's members in document order, duplicates
// included. ok is false for anything that is not a well-formed object.
func jsonObjectMembers(raw []byte) ([]jsonMember, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	if delim, isDelim := tok.(json.Delim); !isDelim || delim != '{' {
		return nil, false
	}

	var members []jsonMember
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		name, isString := tok.(string)
		if !isString {
			return nil, false
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, false
		}
		members = append(members, jsonMember{name: name, value: value})
	}

	// The closing brace, then end of input. Without this, bytes after a complete object are
	// unread and a malformed body reports the wrong reason: {"method":"a","method":"b"}x is a
	// syntax error, not two readings of one member, and the unmarshal below classifies it so.
	if _, err := dec.Token(); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return members, true
}

// foldsOntoOneName reports whether two members collide on one canonical name, or a
// member uses a non-canonical spelling of one.
//
// A non-canonical spelling is rejected even when it appears alone: encoding/json would
// accept "Method" as "method" while a strict backend would not see a method at all, so
// the two ends would again disagree about what was requested.
func foldsOntoOneName(members []jsonMember, canonical []string) bool {
	seen := make(map[string]bool, len(canonical))
	for _, member := range members {
		match := ""
		for _, name := range canonical {
			// EqualFold mirrors encoding/json's own matching, including Unicode
			// simple-fold equivalents such as "s" and "ſ".
			if strings.EqualFold(member.name, name) {
				match = name
				break
			}
		}
		if match == "" {
			continue
		}
		if seen[match] || member.name != match {
			return true
		}
		seen[match] = true
	}
	return false
}

// isJSONObject reports whether raw is a JSON object rather than some other value.
func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '{'
}
