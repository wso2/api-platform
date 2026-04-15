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

package templateengine

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_SimpleSubstitution(t *testing.T) {
	fm := template.FuncMap{
		"greet": func() string { return "hello" },
	}
	result, err := render([]byte(`value: {{ greet }}`), fm)
	require.NoError(t, err)
	assert.Equal(t, "value: hello", string(result))
}

func TestRender_NoTemplateExpressions(t *testing.T) {
	result, err := render([]byte(`plain text`), template.FuncMap{})
	require.NoError(t, err)
	assert.Equal(t, "plain text", string(result))
}

func TestRender_ParseError(t *testing.T) {
	_, err := render([]byte(`{{ invalid`), template.FuncMap{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template parse error")
}

func TestRender_ExecutionError(t *testing.T) {
	fm := template.FuncMap{
		"fail": func() (string, error) { return "", assert.AnError },
	}
	_, err := render([]byte(`{{ fail }}`), fm)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template execution error")
}

func TestRender_JSONWithTemplates(t *testing.T) {
	fm := template.FuncMap{
		"val": func() string { return "resolved" },
	}
	input := `{"key": "{{ val }}", "other": "static"}`
	result, err := render([]byte(input), fm)
	require.NoError(t, err)
	assert.Equal(t, `{"key": "resolved", "other": "static"}`, string(result))
}
