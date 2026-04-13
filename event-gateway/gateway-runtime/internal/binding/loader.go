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

package binding

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// rawEntry is used for initial YAML parsing to discriminate by kind.
type rawEntry struct {
	Kind string `yaml:"kind"`
}

// rawChannelsConfig allows mixed-kind parsing.
type rawChannelsConfig struct {
	Channels []yaml.Node `yaml:"channels"`
}

// ParseResult holds the parsed bindings from a channels YAML file.
type ParseResult struct {
	Bindings          []Binding
	WebSubApiBindings []WebSubApiBinding
}

// ParseChannels reads and parses the channels YAML file.
// It discriminates entries by the "kind" field:
//   - "WebSubApi" entries are parsed as WebSubApiBinding (multi-channel per API)
//   - All other entries are parsed as Binding (legacy flat format)
func ParseChannels(filePath string) (*ParseResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read channels file %s: %w", filePath, err)
	}

	var raw rawChannelsConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse channels file %s: %w", filePath, err)
	}

	result := &ParseResult{}
	for i, node := range raw.Channels {
		var probe rawEntry
		if err := node.Decode(&probe); err != nil {
			return nil, fmt.Errorf("failed to probe kind of channel entry %d: %w", i, err)
		}

		switch probe.Kind {
		case "WebSubApi":
			var wsb WebSubApiBinding
			if err := node.Decode(&wsb); err != nil {
				return nil, fmt.Errorf("failed to parse WebSubApi entry %d: %w", i, err)
			}
			result.WebSubApiBindings = append(result.WebSubApiBindings, wsb)
		default:
			var b Binding
			if err := node.Decode(&b); err != nil {
				return nil, fmt.Errorf("failed to parse binding entry %d: %w", i, err)
			}
			result.Bindings = append(result.Bindings, b)
		}
	}

	return result, nil
}

// GenerateRouteKey creates a route key in the Method|Path|Vhost format
// used by the policy engine.
func GenerateRouteKey(method, fullPath, vhost string) string {
	return fmt.Sprintf("%s|%s|%s", method, fullPath, vhost)
}
