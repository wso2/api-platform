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

package secretinline

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const artifactWithPlaceholders = `apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: demo
spec:
  displayName: Demo
  upstream:
    main:
      url: https://backend.example/v1
  policies:
    - name: set-headers
      version: v1
      params:
        value: '{{ secret "api-key" }}'
        pair: '{{ secret "a" }}:{{ secret "b" }}'
        escaped: "{{ secret \"api-key\" }}"
        plain: untouched
  '{{ secret "not-a-value" }}': keys are never rewritten
`

func fixedResolver(values map[string]string, calls *map[string]int) Resolver {
	return func(handle string) (string, error) {
		if calls != nil {
			(*calls)[handle]++
		}
		v, ok := values[handle]
		if !ok {
			return "", errors.New("no such secret")
		}
		return v, nil
	}
}

// decode returns the rendered document as nested maps for assertions.
func decode(t *testing.T, content []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(content, &doc))
	return doc
}

func params(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	spec := doc["spec"].(map[string]any)
	policies := spec["policies"].([]any)
	return policies[0].(map[string]any)["params"].(map[string]any)
}

func TestRender_ReplacesEveryPlaceholderInValues(t *testing.T) {
	calls := map[string]int{}
	out, err := Render([]byte(artifactWithPlaceholders), fixedResolver(map[string]string{
		"api-key": "sk-live-123",
		"a":       "left",
		"b":       "right",
	}, &calls))
	require.NoError(t, err)

	doc := decode(t, out)
	p := params(t, doc)
	assert.Equal(t, "sk-live-123", p["value"])
	assert.Equal(t, "left:right", p["pair"], "several placeholders in one scalar")
	assert.Equal(t, "sk-live-123", p["escaped"], "the escaped-quote spelling is a placeholder too")
	assert.Equal(t, "untouched", p["plain"])

	spec := doc["spec"].(map[string]any)
	_, keyKept := spec[`{{ secret "not-a-value" }}`]
	assert.True(t, keyKept, "a placeholder in a mapping key is not rewritten")
	assert.Equal(t, 1, calls["api-key"], "each handle is resolved once per render")

	assert.Equal(t, "demo", doc["metadata"].(map[string]any)["name"])
	assert.Equal(t, "gateway.api-platform.wso2.com/v1alpha1", doc["apiVersion"])
}

// A secret value must come back byte-for-byte, whatever YAML-significant
// characters it holds — and must not be able to add keys to the document.
func TestRender_ValuesWithYAMLSyntaxAreInert(t *testing.T) {
	for name, secret := range map[string]string{
		"double quotes": `say "hi"`,
		"single quotes": `it's`,
		"newline":       "line1\nline2",
		"hash":          "# not a comment",
		"colon space":   "key: value",
		"leading star":  "*alias",
		"leading amp":   "&anchor",
		"leading dash":  "- item",
		"leading bang":  "!tag",
		"leading brace": "{a: b}",
		"brackets":      "[1, 2]",
		"template":      "{{ not a secret }}",
		"empty":         "",
		"unicode":       "pässwörd ✓",
	} {
		t.Run(name, func(t *testing.T) {
			out, err := Render([]byte(artifactWithPlaceholders), fixedResolver(map[string]string{
				"api-key": secret, "a": "x", "b": "y",
			}, nil))
			require.NoError(t, err)
			doc := decode(t, out)
			p := params(t, doc)
			assert.Equal(t, secret, p["value"])
			assert.Len(t, p, 4, "no key was injected into params")
			assert.Len(t, doc, 4, "no top-level key was injected")
		})
	}
}

func TestRender_PreservesKeyOrder(t *testing.T) {
	out, err := Render([]byte(artifactWithPlaceholders), fixedResolver(map[string]string{"api-key": "v", "a": "x", "b": "y"}, nil))
	require.NoError(t, err)

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(out, &node))
	root := node.Content[0]
	var keys []string
	for i := 0; i < len(root.Content); i += 2 {
		keys = append(keys, root.Content[i].Value)
	}
	assert.Equal(t, []string{"apiVersion", "kind", "metadata", "spec"}, keys)
}

func TestRender_NoPlaceholderIsByteIdentical(t *testing.T) {
	in := []byte("apiVersion: v1\nkind: RestApi\nspec:\n  policies: []\n")
	out, err := Render(in, func(string) (string, error) {
		t.Fatal("resolver must not be called")
		return "", nil
	})
	require.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestRender_UnknownHandleFailsTheWholeRender(t *testing.T) {
	out, err := Render([]byte(artifactWithPlaceholders), fixedResolver(map[string]string{"api-key": "v"}, nil))
	require.Error(t, err)
	assert.Nil(t, out, "nothing partially rendered is returned")
	assert.Contains(t, err.Error(), `"a"`, "the failing handle is named")
}

func TestRender_InvalidYAML(t *testing.T) {
	_, err := Render([]byte("spec: [unclosed\nvalue: '{{ secret \"x\" }}'"), fixedResolver(map[string]string{"x": "v"}, nil))
	assert.Error(t, err)
}
