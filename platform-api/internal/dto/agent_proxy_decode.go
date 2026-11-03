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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// The public Agent proxy schema is a discriminated union: `protocol` selects the
// complete variant, and only that variant's named configuration block is
// permitted alongside it. api.AgentProxy is the generated union wrapper, which
// holds the raw bytes and offers no validation of its own — so the decode step
// that turns a request body into a typed variant lives here, next to the mapping
// that turns that variant into the persisted model.

// DecodeAgentProxyRequest turns a create/replace request body into the typed
// Agent proxy variant its `protocol` discriminator selects.
//
// Every error it returns is phrased for the caller — it describes the caller's
// own payload and never a Go type, field path or decoder internal — so a service
// can hand it straight to apperror.ValidationFailed.
func DecodeAgentProxyRequest(data []byte) (*api.A2AAgentProxy, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.New("The request body must be a JSON object.")
	}

	protocol, err := agentProxyRequestProtocol(raw)
	if err != nil {
		return nil, err
	}
	if err := assertSingleProtocolBlock(raw, protocol, model.SupportedAgentProxyProtocols()); err != nil {
		return nil, err
	}

	switch protocol {
	case model.AgentProxyProtocolA2A:
		// Unknown fields are reported first, so a body still using the old
		// a2a.transports layout is told that field is unsupported rather than
		// only that operationConfigs is missing.
		out, err := decodeAgentProxyVariant[api.A2AAgentProxy](data)
		if err != nil {
			return nil, err
		}
		if err := assertA2AOperationConfigsPresent(raw["a2a"]); err != nil {
			return nil, err
		}
		return out, nil
	default:
		// Registered in the model but with no request variant wired up here. A
		// caller cannot tell that apart from an unknown protocol, and neither
		// answer would be more actionable, so both read the same.
		return nil, unsupportedAgentProxyProtocolError(string(protocol))
	}
}

// assertSingleProtocolBlock checks that the body carries the named configuration
// block the discriminator selects, and no other registered protocol's block.
//
// registered is passed in rather than read from the model so the second half can
// be exercised with more than one protocol in play — today the registry holds
// only A2A, which is exactly when this rule is easiest to get wrong and hardest
// to notice.
func assertSingleProtocolBlock(raw map[string]json.RawMessage, protocol model.AgentProxyProtocol, registered []string) error {
	if _, ok := raw[string(protocol)]; !ok {
		return fmt.Errorf("The %q configuration block is required when protocol is %q.", string(protocol), string(protocol))
	}
	for _, other := range registered {
		if model.AgentProxyProtocol(other) == protocol {
			continue
		}
		if _, ok := raw[other]; ok {
			return fmt.Errorf("The request carries a %q configuration block, but protocol is %q. Exactly one protocol configuration block is permitted.", other, string(protocol))
		}
	}
	return nil
}

// assertA2AOperationConfigsPresent checks that the a2a block carries the
// required operationConfigs key.
//
// The generated type holds operationConfigs as a value, so once decoded an
// omitted block is indistinguishable from one with an empty transport list, and
// the caller would be told about transports inside a block they never sent.
// Checked against the raw body, where key presence is still visible. A block
// that is not an object has already been reported by the decoder.
func assertA2AOperationConfigsPresent(block json.RawMessage) error {
	var a2a map[string]json.RawMessage
	if err := json.Unmarshal(block, &a2a); err != nil || a2a == nil {
		return nil
	}
	if _, ok := a2a["operationConfigs"]; !ok {
		return errors.New("The a2a.operationConfigs block is required; it carries the transports the Agent proxy is served on.")
	}
	return nil
}

// agentProxyRequestProtocol reads the discriminator. It is required, must be a
// non-empty string, and must name a registered protocol — there is no default,
// so a body without it is rejected rather than assumed to be A2A.
func agentProxyRequestProtocol(raw map[string]json.RawMessage) (model.AgentProxyProtocol, error) {
	value, ok := raw["protocol"]
	if !ok {
		return "", errors.New("The protocol field is required.")
	}
	var protocol string
	if err := json.Unmarshal(value, &protocol); err != nil || protocol == "" {
		return "", errors.New("The protocol field must be a non-empty string.")
	}
	if !model.IsSupportedAgentProxyProtocol(model.AgentProxyProtocol(protocol)) {
		return "", unsupportedAgentProxyProtocolError(protocol)
	}
	return model.AgentProxyProtocol(protocol), nil
}

func unsupportedAgentProxyProtocolError(protocol string) error {
	return fmt.Errorf("The protocol %q is not supported. Supported protocols: %s.",
		protocol, strings.Join(model.SupportedAgentProxyProtocols(), ", "))
}

// decodeAgentProxyVariant checks the body against the variant's declared shape
// and then decodes it.
//
// The unknown-property check is a separate pass rather than the decoder's own
// DisallowUnknownFields, because that option cannot see inside a type with a
// custom UnmarshalJSON — and oapi-codegen generates one for every schema that
// pairs `properties` with `oneOf`, which is exactly the Agent Card
// configuration. Relying on the decoder would silently accept `signing` on a
// card, the one property Phase 1 has to reject.
func decodeAgentProxyVariant[T any](data []byte) (*T, error) {
	var out T
	if err := assertDeclaredFields(data, reflect.TypeOf(out), ""); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, clientDecodeError(err)
	}
	return &out, nil
}

// assertDeclaredFields walks a raw JSON value against the Go type it will be
// decoded into and rejects any object key the type does not declare, and any
// explicitly null value.
//
// Null is rejected here because nothing downstream can see it: no property in
// the Agent proxy contract is nullable, and `"context": null` decodes to the
// same nil pointer as an omitted key — so a caller asking to *clear* a field by
// sending null would silently get the field's default instead. The distinction
// only exists while the raw bytes are still in hand.
//
// The walk stops wherever the contract stops being typed: a map-typed field is
// free-form by design (Agent Card content, policy parameters, gateway
// configuration overrides), so nothing inside it is inspected — neither an
// extension key nor a null value — and extension data passes through untouched.
// It also stops at any value whose JSON shape does not match the Go kind — a
// mismatch is the decoder's to report, with a message about the field rather
// than about an unexpected key inside it.
func assertDeclaredFields(raw json.RawMessage, t reflect.Type, path string) error {
	// The root is never reached with a null: a body that is not a JSON object
	// fails before this, so every null seen here is a field or an array element
	// and has a path to name.
	if path != "" && isJSONNull(raw) {
		return fmt.Errorf("The field %s must not be null. Omit it instead.", strconv.Quote(path))
	}

	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return nil
	}

	switch t.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil
		}
		declared := declaredJSONFields(t)

		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		var unknown []string
		for _, key := range keys {
			if _, ok := declared[key]; !ok {
				unknown = append(unknown, strconv.Quote(joinFieldPath(path, key)))
			}
		}
		if len(unknown) > 0 {
			return fmt.Errorf("The request body contains unsupported fields: %s.", strings.Join(unknown, ", "))
		}

		for _, key := range keys {
			if err := assertDeclaredFields(object[key], declared[key], joinFieldPath(path, key)); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil
		}
		for i, item := range items {
			if err := assertDeclaredFields(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// declaredJSONFields maps a struct's wire property names to their field types.
// An unexported field carries no json tag and is skipped, which is what keeps
// oapi-codegen's internal `union` holder out of the permitted set.
func declaredJSONFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		fields[name] = field.Type
	}
	return fields
}

// isJSONNull reports whether a raw value is the JSON literal null, ignoring the
// insignificant whitespace a caller may have sent around it.
func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

func joinFieldPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// clientDecodeError restates a decoder failure in terms of the caller's own
// payload. encoding/json's messages name Go types and struct fields
// ("...Go struct field A2AAgentProxy.upstream.main.url of type string"), which
// error-handling.md keeps out of a response body.
func clientDecodeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return fmt.Errorf("The field %s has the wrong type.", strconv.Quote(typeErr.Field))
	}
	return errors.New("The request body could not be parsed as JSON.")
}
