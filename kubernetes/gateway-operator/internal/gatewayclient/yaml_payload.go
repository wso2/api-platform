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

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	"gopkg.in/yaml.v2"
)

// BuildRestAPIYAML builds the gateway-controller REST payload (application/y aml)
// matching the shape produced for RestApi CRs (api.yaml-compatible).
func BuildRestAPIYAML(apiVersion, kind, name string, spec apiv1.APIConfigData) ([]byte, error) {
	cleanPayload := struct {
		APIVersion string              `yaml:"apiVersion" json:"apiVersion"`
		Kind       string              `yaml:"kind" json:"kind"`
		Metadata   map[string]string   `yaml:"metadata" json:"metadata"`
		Spec       apiv1.APIConfigData `yaml:"spec" json:"spec"`
	}{
		APIVersion: apiVersion,
		Kind:       kind,
		Metadata: map[string]string{
			"name": name,
		},
		Spec: spec,
	}

	jsonBytes, err := json.Marshal(cleanPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal API spec to JSON: %w", err)
	}

	var genericMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &genericMap); err != nil {
		return nil, fmt.Errorf("unmarshal JSON to map: %w", err)
	}

	apiYAML, err := yaml.Marshal(genericMap)
	if err != nil {
		return nil, fmt.Errorf("marshal API spec to YAML: %w", err)
	}

	return apiYAML, nil
}
