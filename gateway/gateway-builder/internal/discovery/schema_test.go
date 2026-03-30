/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package discovery

import (
	"reflect"
	"testing"

	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestExtractDefaultValues_Empty(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name:   "nil schema",
			schema: nil,
			want:   map[string]interface{}{},
		},
		{
			name:   "empty schema",
			schema: map[string]interface{}{},
			want:   map[string]interface{}{},
		},
		{
			name: "no properties",
			schema: map[string]interface{}{
				"type": "object",
			},
			want: map[string]interface{}{},
		},
		{
			name: "empty properties",
			schema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			want: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDefaultValues(tt.schema)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractDefaultValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractDefaultValues_Precedence(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name: "wso2/defaultValue takes precedence over default",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prop1": map[string]interface{}{
						"type":              "string",
						"default":           "default-value",
						"wso2/defaultValue": "${config.Prop1}",
					},
				},
			},
			want: map[string]interface{}{
				"prop1": map[string]interface{}{
					policyv1alpha.SystemParamConfigRefKey:    "${config.Prop1}",
					policyv1alpha.SystemParamDefaultValueKey: "default-value",
					systemParamRequiredKey:                   false,
				},
			},
		},
		{
			name: "only default value",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prop1": map[string]interface{}{
						"type":    "string",
						"default": "default-value",
					},
				},
			},
			want: map[string]interface{}{
				"prop1": "default-value",
			},
		},
		{
			name: "only wso2/defaultValue",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prop1": map[string]interface{}{
						"type":              "string",
						"wso2/defaultValue": "${config.Prop1}",
					},
				},
			},
			want: map[string]interface{}{
				"prop1": map[string]interface{}{
					policyv1alpha.SystemParamConfigRefKey: "${config.Prop1}",
					systemParamRequiredKey:                false,
				},
			},
		},
		{
			name: "property without any default",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prop1": map[string]interface{}{
						"type": "string",
					},
				},
			},
			want: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDefaultValues(tt.schema)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractDefaultValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractDefaultValues_Types(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"stringProp": map[string]interface{}{
				"type":    "string",
				"default": "hello",
			},
			"intProp": map[string]interface{}{
				"type":    "integer",
				"default": 401,
			},
			"boolProp": map[string]interface{}{
				"type":    "boolean",
				"default": false,
			},
			"arrayProp": map[string]interface{}{
				"type":    "array",
				"default": []interface{}{"a", "b"},
			},
		},
	}

	got := ExtractDefaultValues(schema)
	want := map[string]interface{}{
		"stringProp": "hello",
		"intProp":    401,
		"boolProp":   false,
		"arrayProp":  []interface{}{"a", "b"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDefaultValues() = %v, want %v", got, want)
	}
}

func TestExtractDefaultValues_JWTAuthRealWorld(t *testing.T) {
	// Real-world schema from jwt-auth policy
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"authHeaderScheme": map[string]interface{}{
				"type":              "string",
				"default":           "Bearer",
				"wso2/defaultValue": "${config.JWTAuth.AuthHeaderScheme}",
			},
			"headerName": map[string]interface{}{
				"type":              "string",
				"default":           "Authorization",
				"wso2/defaultValue": "${config.JWTAuth.HeaderName}",
			},
			"onFailureStatusCode": map[string]interface{}{
				"type":              "integer",
				"default":           401,
				"wso2/defaultValue": "${config.JWTAuth.OnFailureStatusCode}",
			},
			"jwksCacheTtl": map[string]interface{}{
				"type":              "string",
				"wso2/defaultValue": "${config.JWTAuth.JwksCacheTtl}",
			},
			"keyManagers": map[string]interface{}{
				"type":              "array",
				"wso2/defaultValue": "${config.JWTAuth.KeyManagers}",
			},
		},
	}

	got := ExtractDefaultValues(schema)
	want := map[string]interface{}{
		"authHeaderScheme": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey:    "${config.JWTAuth.AuthHeaderScheme}",
			policyv1alpha.SystemParamDefaultValueKey: "Bearer",
			systemParamRequiredKey:                   false,
		},
		"headerName": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey:    "${config.JWTAuth.HeaderName}",
			policyv1alpha.SystemParamDefaultValueKey: "Authorization",
			systemParamRequiredKey:                   false,
		},
		"onFailureStatusCode": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey:    "${config.JWTAuth.OnFailureStatusCode}",
			policyv1alpha.SystemParamDefaultValueKey: 401,
			systemParamRequiredKey:                   false,
		},
		"jwksCacheTtl": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey: "${config.JWTAuth.JwksCacheTtl}",
			systemParamRequiredKey:                false,
		},
		"keyManagers": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey: "${config.JWTAuth.KeyManagers}",
			systemParamRequiredKey:                false,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDefaultValues() =\n%v\nwant\n%v", got, want)
	}
}

func TestExtractDefaultValues_MixedProperties(t *testing.T) {
	// Test with some properties having defaults and some not
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"withDefault": map[string]interface{}{
				"type":    "string",
				"default": "value1",
			},
			"withWso2Default": map[string]interface{}{
				"type":              "string",
				"wso2/defaultValue": "${config.Value2}",
			},
			"noDefault": map[string]interface{}{
				"type": "string",
			},
			"withBothDefaults": map[string]interface{}{
				"type":              "integer",
				"default":           100,
				"wso2/defaultValue": "${config.Value3}",
			},
		},
	}

	got := ExtractDefaultValues(schema)
	want := map[string]interface{}{
		"withDefault": "value1",
		"withWso2Default": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey: "${config.Value2}",
			systemParamRequiredKey:                false,
		},
		"withBothDefaults": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey:    "${config.Value3}",
			policyv1alpha.SystemParamDefaultValueKey: 100,
			systemParamRequiredKey:                   false,
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDefaultValues() = %v, want %v", got, want)
	}
}

func TestExtractDefaultValues_NestedObjectProperties(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"algorithm": map[string]interface{}{
				"type":              "string",
				"default":           "gcra",
				"wso2/defaultValue": "${config.policy.algorithm}",
			},
			"redis": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"host": map[string]interface{}{
						"type":              "string",
						"default":           "localhost",
						"wso2/defaultValue": "${config.policy.redis.host}",
					},
					"port": map[string]interface{}{
						"type":    "integer",
						"default": 6379,
					},
					"password": map[string]interface{}{
						"type":              "string",
						"wso2/defaultValue": "${config.policy.redis.password}",
					},
				},
			},
		},
	}

	got := ExtractDefaultValues(schema)
	want := map[string]interface{}{
		"algorithm": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey:    "${config.policy.algorithm}",
			policyv1alpha.SystemParamDefaultValueKey: "gcra",
			systemParamRequiredKey:                   false,
		},
		"redis": map[string]interface{}{
			"host": map[string]interface{}{
				policyv1alpha.SystemParamConfigRefKey:    "${config.policy.redis.host}",
				policyv1alpha.SystemParamDefaultValueKey: "localhost",
				systemParamRequiredKey:                   false,
			},
			"port": 6379,
			"password": map[string]interface{}{
				policyv1alpha.SystemParamConfigRefKey: "${config.policy.redis.password}",
				systemParamRequiredKey:                false,
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDefaultValues() = %v, want %v", got, want)
	}
}

func TestExtractDefaultValues_InvalidPropertyDef(t *testing.T) {
	// Test with invalid property definition (not a map)
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"validProp": map[string]interface{}{
				"type":    "string",
				"default": "value1",
			},
			"invalidProp": "not-a-map", // Invalid: should be skipped
			"anotherValidProp": map[string]interface{}{
				"type":    "string",
				"default": "value2",
			},
		},
	}

	got := ExtractDefaultValues(schema)
	want := map[string]interface{}{
		"validProp":        "value1",
		"anotherValidProp": "value2",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDefaultValues() = %v, want %v", got, want)
	}
}

func TestExtractDefaultValues_RequiredMetadata(t *testing.T) {
	schema := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"requiredProp", "nested"},
		"properties": map[string]interface{}{
			"requiredProp": map[string]interface{}{
				"type":              "string",
				"wso2/defaultValue": "${config.required}",
			},
			"optionalProp": map[string]interface{}{
				"type":              "string",
				"wso2/defaultValue": "${config.optional}",
			},
			"nested": map[string]interface{}{
				"type":     "object",
				"required": []interface{}{"nestedRequired"},
				"properties": map[string]interface{}{
					"nestedRequired": map[string]interface{}{
						"type":              "string",
						"wso2/defaultValue": "${config.nested.required}",
					},
					"nestedOptional": map[string]interface{}{
						"type":              "string",
						"wso2/defaultValue": "${config.nested.optional}",
					},
				},
			},
		},
	}

	got := ExtractDefaultValues(schema)
	want := map[string]interface{}{
		"requiredProp": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey: "${config.required}",
			systemParamRequiredKey:                true,
		},
		"optionalProp": map[string]interface{}{
			policyv1alpha.SystemParamConfigRefKey: "${config.optional}",
			systemParamRequiredKey:                false,
		},
		"nested": map[string]interface{}{
			"nestedRequired": map[string]interface{}{
				policyv1alpha.SystemParamConfigRefKey: "${config.nested.required}",
				systemParamRequiredKey:                true,
			},
			"nestedOptional": map[string]interface{}{
				policyv1alpha.SystemParamConfigRefKey: "${config.nested.optional}",
				systemParamRequiredKey:                false,
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDefaultValues() = %v, want %v", got, want)
	}
}
