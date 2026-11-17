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

// Package configcompat keeps renamed configuration keys working: a loader
// passes its raw, nested configuration map through it before unmarshalling,
// and a key set under its old name is moved to the new one.
package configcompat

import (
	"fmt"
	"strings"
)

// DestinationGuardKey is the http_client table that configures the outbound
// destination guard (an httpclient.DestinationGuardConfig).
const DestinationGuardKey = "destination_guard"

// LegacyDestinationGuardKey is the name that table shipped under before it was
// renamed. It is read only to move it to DestinationGuardKey, and exported so
// loader tests can prove a configuration that still uses it keeps loading.
const LegacyDestinationGuardKey = "ssrf"

// MigrateDestinationGuard moves <parent>.http_client.<legacy name> to
// <parent>.http_client.destination_guard in raw, a nested map as koanf's
// Raw() returns it. parent is the path to the table holding http_client
// (empty when http_client is at the top level). It reports the deprecated
// path when it moved one, so the caller can log it, and fails when both names
// are set, rather than silently picking one.
func MigrateDestinationGuard(raw map[string]any, parent ...string) (deprecatedPath string, err error) {
	path := append(append([]string{}, parent...), "http_client")
	moved, err := RenameKey(raw, path, LegacyDestinationGuardKey, DestinationGuardKey)
	if err != nil || !moved {
		return "", err
	}
	return strings.Join(append(path, LegacyDestinationGuardKey), "."), nil
}

// AliasLegacyDestinationGuard makes <parent>.http_client.<legacy name> name the
// same table as <parent>.http_client.destination_guard in raw, so a path written
// against the old name (a policy's ${config...} reference, for example) still
// resolves after MigrateDestinationGuard has moved the table. Apply it only to a
// map that paths are looked up in, never to one that is unmarshalled or migrated
// again: there both names would be set. It does nothing when there is no
// destination_guard table or the legacy name is already taken.
func AliasLegacyDestinationGuard(raw map[string]any, parent ...string) {
	table := raw
	for _, seg := range append(append([]string{}, parent...), "http_client") {
		next, ok := table[seg].(map[string]any)
		if !ok {
			return
		}
		table = next
	}
	guard, ok := table[DestinationGuardKey]
	if !ok {
		return
	}
	if _, taken := table[LegacyDestinationGuardKey]; taken {
		return
	}
	table[LegacyDestinationGuardKey] = guard
}

// RenameKey moves raw[path...][oldKey] to raw[path...][newKey]. It is a no-op
// when the old key is absent, and an error when both keys are set.
func RenameKey(raw map[string]any, path []string, oldKey, newKey string) (bool, error) {
	table := raw
	for _, seg := range path {
		next, ok := table[seg].(map[string]any)
		if !ok {
			return false, nil
		}
		table = next
	}
	value, ok := table[oldKey]
	if !ok {
		return false, nil
	}
	prefix := strings.Join(path, ".")
	if prefix != "" {
		prefix += "."
	}
	if _, exists := table[newKey]; exists {
		return false, fmt.Errorf("%s%s and %s%s are both set: %s%s is the deprecated name of %s%s; remove it",
			prefix, oldKey, prefix, newKey, prefix, oldKey, prefix, newKey)
	}
	table[newKey] = value
	delete(table, oldKey)
	return true, nil
}
