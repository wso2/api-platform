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

package gatewayclient

import (
	"encoding/json"
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// BudgetPayloadMetadata is the metadata of a Budget sent to the gateway-controller.
// Unlike the other kinds, a budget's labels are a list of words, not a map.
type BudgetPayloadMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	Labels      []string          `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// BuildBudgetYAML builds the Budget payload ({apiVersion, kind, metadata, spec}) as YAML
// for POST/PUT /budgets. spec must be JSON-marshalable; its json tags give the field names.
func BuildBudgetYAML(apiVersion string, metadata BudgetPayloadMetadata, spec interface{}) ([]byte, error) {
	if strings.TrimSpace(apiVersion) == "" {
		return nil, fmt.Errorf("missing apiVersion")
	}
	if strings.TrimSpace(metadata.Name) == "" {
		return nil, fmt.Errorf("missing budget name")
	}

	jsonBytes, err := json.Marshal(struct {
		APIVersion string                `json:"apiVersion"`
		Kind       string                `json:"kind"`
		Metadata   BudgetPayloadMetadata `json:"metadata"`
		Spec       interface{}           `json:"spec"`
	}{APIVersion: apiVersion, Kind: "Budget", Metadata: metadata, Spec: spec})
	if err != nil {
		return nil, fmt.Errorf("marshal budget to JSON: %w", err)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &generic); err != nil {
		return nil, fmt.Errorf("unmarshal JSON to map: %w", err)
	}

	out, err := yaml.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("marshal budget to YAML: %w", err)
	}
	return out, nil
}
