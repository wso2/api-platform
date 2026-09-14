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

package components

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// WiringSpec decodes component-specific block configuration.
type WiringSpec interface {
	// Decode parses a wiring node into the component's wiring type.
	Decode(node *yaml.Node) (any, error)
}

type wiringFunc func(node *yaml.Node) (any, error)

func (f wiringFunc) Decode(node *yaml.Node) (any, error) { return f(node) }

// TypedWiring creates a strict WiringSpec for type T.
func TypedWiring[T any]() WiringSpec {
	return wiringFunc(func(node *yaml.Node) (any, error) {
		var target T
		if err := decodeStrict(node, &target); err != nil {
			return nil, err
		}
		if v, ok := any(&target).(validator); ok {
			if err := v.Validate(); err != nil {
				return nil, err
			}
		}
		return &target, nil
	})
}

type validator interface {
	Validate() error
}

func decodeStrict(node *yaml.Node, target any) error {
	if node == nil {
		return nil
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		return fmt.Errorf("re-encoding wiring: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decoding wiring: %w", err)
	}
	return nil
}
