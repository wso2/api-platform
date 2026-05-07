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

// EnvelopeMetadata mirrors RestAPIPayloadMetadata; aliased for readability
// when used by non-RestApi envelope kinds.
type EnvelopeMetadata = RestAPIPayloadMetadata

// BuildEnvelopeYAML builds a generic management-API envelope payload
// ({apiVersion, kind, metadata, spec}) as YAML. This is the same shape as
// BuildRestAPIYAML but generalised so new CRDs can reuse it without
// duplicating the JSON->YAML round-trip.
//
// spec must be a JSON-marshalable value (typically the CRD's Spec struct);
// the conversion via json.Marshal preserves omitempty/json tags so the
// emitted YAML matches the management-API schema.
func BuildEnvelopeYAML(apiVersion, kind string, metadata EnvelopeMetadata, spec interface{}) ([]byte, error) {
	if strings.TrimSpace(apiVersion) == "" {
		return nil, fmt.Errorf("missing apiVersion")
	}
	if strings.TrimSpace(kind) == "" {
		return nil, fmt.Errorf("missing kind")
	}

	cleanPayload := struct {
		APIVersion string           `yaml:"apiVersion" json:"apiVersion"`
		Kind       string           `yaml:"kind" json:"kind"`
		Metadata   EnvelopeMetadata `yaml:"metadata" json:"metadata"`
		Spec       interface{}      `yaml:"spec" json:"spec"`
	}{
		APIVersion: apiVersion,
		Kind:       kind,
		Metadata:   metadata,
		Spec:       spec,
	}

	jsonBytes, err := json.Marshal(cleanPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope to JSON: %w", err)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &generic); err != nil {
		return nil, fmt.Errorf("unmarshal JSON to map: %w", err)
	}

	out, err := yaml.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope to YAML: %w", err)
	}
	return out, nil
}
