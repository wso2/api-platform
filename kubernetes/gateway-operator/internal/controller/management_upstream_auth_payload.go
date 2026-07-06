/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package controller

import (
	"encoding/json"
	"fmt"
)

// specToJSONMap marshals a spec struct to a generic map for gateway-controller
// payloads. JSON tags must align with the management API field names.
func specToJSONMap(spec interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("spec to map: %w", err)
	}
	return m, nil
}

// flattenUpstreamAuthCredentialValue replaces spec[parentKey].auth.value with a
// plain string. Gateway-controller unmarshals upstream auth into
// LLMUpstreamAuth.Value *string — not the Kubernetes CRD SecretValueSource shape
// ({ valueFrom } | { value: nested ... }).
func flattenUpstreamAuthCredentialValue(specMap map[string]interface{}, parentKey string, plaintext string) error {
	parent, ok := specMap[parentKey].(map[string]interface{})
	if !ok {
		return fmt.Errorf("spec.%s must be an object", parentKey)
	}
	auth, ok := parent["auth"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("spec.%s.auth must be an object", parentKey)
	}
	auth["value"] = plaintext
	return nil
}

func flattenAdditionalProviderAuthCredentialValue(specMap map[string]interface{}, index int, plaintext string) error {
	additionalProviders, ok := specMap["additionalProviders"].([]interface{})
	if !ok {
		return fmt.Errorf("spec.additionalProviders must be an array")
	}
	if index < 0 || index >= len(additionalProviders) {
		return fmt.Errorf("spec.additionalProviders[%d] is out of range", index)
	}
	parent, ok := additionalProviders[index].(map[string]interface{})
	if !ok {
		return fmt.Errorf("spec.additionalProviders[%d] must be an object", index)
	}
	auth, ok := parent["auth"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("spec.additionalProviders[%d].auth must be an object", index)
	}
	auth["value"] = plaintext
	return nil
}
