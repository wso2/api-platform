/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

// Package brokersecret keeps WebBroker broker and receiver credentials out of
// read responses while leaving the stored value intact.
//
// Broker properties are a free-form map, so there is no per-property schema to
// carry writeOnly. The broker type is the discriminator instead: each driver
// declares which of its property keys hold a credential.
//
// Blank and Preserve are two halves of one contract. Blank replaces a credential
// with an empty value of the same shape rather than dropping the key, because
// the key is what tells a client the credential exists; Preserve reads that same
// empty value back as "unchanged". Dropping the key instead would make a plain
// read-then-write erase the credential.
package brokersecret

import "strings"

// kafkaSensitiveKeys are the Kafka broker property keys whose values are
// credentials or raw key material.
//
// Both spellings are listed deliberately. The gateway's Kafka driver reads
// snake_case keys (event-gateway brokerdriver/kafka/config.go), while the
// Choreo console writes the dotted Kafka client property names. Until those
// converge, blanking either spelling alone leaves the other exposed.
//
// Usernames, mechanisms and file paths are not listed: they are configuration a
// user needs to see to recognise the connection they configured.
var kafkaSensitiveKeys = []string{
	"sasl_password",
	"sasl.password",
	"ssl.key.password",
	"ssl.keystore.key",
	"sasl.kerberos.keytab",
}

// sensitiveKeysByType maps a broker type to its credential-bearing property keys.
// Keys are matched case-insensitively against the broker type, which arrives as
// "kafka" from the Platform API and "KAFKA" from Choreo.
var sensitiveKeysByType = map[string][]string{
	"kafka": kafkaSensitiveKeys,
}

// SensitiveKeys returns the credential-bearing property keys for a broker type,
// or nil when the type declares none.
func SensitiveKeys(brokerType string) []string {
	return sensitiveKeysByType[strings.ToLower(strings.TrimSpace(brokerType))]
}

// Blank returns a copy of properties with every credential value replaced by an
// empty value of the same shape. The input map is never modified: callers hold a
// pointer into the stored model.
//
// Returns the input unchanged when the broker type declares no credential keys.
func Blank(brokerType string, properties map[string]interface{}) map[string]interface{} {
	keys := SensitiveKeys(brokerType)
	if len(keys) == 0 || len(properties) == 0 {
		return properties
	}

	out := make(map[string]interface{}, len(properties))
	for k, v := range properties {
		out[k] = v
	}
	for _, key := range keys {
		if v, ok := out[key]; ok {
			out[key] = emptyLike(v)
		}
	}
	return out
}

// Preserve merges an incoming property map over the stored one for a single
// broker type, resolving the blanked credentials Blank produced:
//
//   - an incoming credential that is blank  -> the stored value is kept
//   - an incoming credential that is null   -> the credential is cleared
//   - any other incoming value              -> the incoming value wins
//
// Only credential keys are merged. Every other property is taken from incoming
// as-is, so removing an ordinary property still removes it.
func Preserve(brokerType string, incoming, stored map[string]interface{}) map[string]interface{} {
	keys := SensitiveKeys(brokerType)
	if len(keys) == 0 || incoming == nil {
		return incoming
	}

	out := make(map[string]interface{}, len(incoming))
	for k, v := range incoming {
		out[k] = v
	}
	for _, key := range keys {
		v, present := out[key]
		if !present {
			continue
		}
		if v == nil {
			delete(out, key)
			continue
		}
		if isBlank(v) {
			if prev, ok := stored[key]; ok {
				out[key] = prev
			} else {
				delete(out, key)
			}
		}
	}
	return out
}

// emptyLike returns an empty value matching v's shape, so a client that reads a
// credential as a string gets "" back and one that reads it as a single-element
// list gets [""] back.
func emptyLike(v interface{}) interface{} {
	switch t := v.(type) {
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			out[i] = ""
		}
		return out
	case []string:
		out := make([]string, len(t))
		return out
	default:
		return ""
	}
}

// isBlank reports whether v carries no credential: an empty string, or a list
// whose entries are all empty strings. A list of empty strings is what Blank
// hands back for a list-shaped credential.
func isBlank(v interface{}) bool {
	switch t := v.(type) {
	case string:
		return t == ""
	case []interface{}:
		for _, item := range t {
			s, ok := item.(string)
			if !ok || s != "" {
				return false
			}
		}
		return true
	case []string:
		for _, item := range t {
			if item != "" {
				return false
			}
		}
		return true
	default:
		return false
	}
}
