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

package redact

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedact_SingleValue(t *testing.T) {
	input := `{"apiKey": "sk-super-secret-123", "name": "test"}`
	result := Redact(input, []string{"sk-super-secret-123"})

	assert.Contains(t, result, RedactedPlaceholder)
	assert.NotContains(t, result, "sk-super-secret-123")
	assert.Contains(t, result, "test")
}

func TestRedact_MultipleValues(t *testing.T) {
	input := `{"apiKey": "sk-secret-1", "token": "Bearer eyJhbGciOiJSUzI1NiJ9"}`
	result := Redact(input, []string{"sk-secret-1", "eyJhbGciOiJSUzI1NiJ9"})

	assert.NotContains(t, result, "sk-secret-1")
	assert.NotContains(t, result, "eyJhbGciOiJSUzI1NiJ9")
	assert.Contains(t, result, "Bearer")
}

func TestRedact_MultipleOccurrences(t *testing.T) {
	input := `{"field1": "secret-val", "field2": "prefix-secret-val-suffix"}`
	result := Redact(input, []string{"secret-val"})

	assert.NotContains(t, result, "secret-val")
}

func TestRedact_NoSensitiveValues(t *testing.T) {
	input := `{"name": "test"}`
	result := Redact(input, nil)

	assert.Equal(t, input, result)
}

func TestRedact_EmptyInput(t *testing.T) {
	result := Redact("", []string{"secret"})
	assert.Equal(t, "", result)
}

func TestRedact_EmptyValueInList(t *testing.T) {
	input := `{"key": "value"}`
	result := Redact(input, []string{"", "value"})

	assert.NotContains(t, result, "value")
	assert.Contains(t, result, RedactedPlaceholder)
}

func TestRedact_NoMatch(t *testing.T) {
	input := `{"key": "value"}`
	result := Redact(input, []string{"not-present"})

	assert.Equal(t, input, result)
}
