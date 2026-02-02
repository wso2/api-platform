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

package utils

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLLMTemplateLoader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	assert.NotNil(t, loader)
	assert.NotNil(t, loader.logger)
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_NonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	templates, err := loader.LoadTemplatesFromDirectory("/non/existent/path")
	assert.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_EmptyDir(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_SkipsNonTemplateFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create non-template files
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("Not a template"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "config.xml"), []byte("<config></config>"), 0644)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_JSONFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a valid JSON template
	jsonTemplate := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {
			"name": "test-template"
		},
		"spec": {
			"displayName": "Test Template"
		}
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "test-template.json"), []byte(jsonTemplate), 0644)
	require.NoError(t, err)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Contains(t, templates, "test-template")
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_YAMLFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a valid YAML template
	yamlTemplate := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: yaml-template
spec:
  displayName: YAML Template
`
	err = os.WriteFile(filepath.Join(tmpDir, "yaml-template.yaml"), []byte(yamlTemplate), 0644)
	require.NoError(t, err)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Contains(t, templates, "yaml-template")
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_YMLFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a valid YML template
	ymlTemplate := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: yml-template
spec:
  displayName: YML Template
`
	err = os.WriteFile(filepath.Join(tmpDir, "yml-template.yml"), []byte(ymlTemplate), 0644)
	require.NoError(t, err)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Contains(t, templates, "yml-template")
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_MultipleFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create multiple templates
	jsonTemplate := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "template-1"},
		"spec": {"displayName": "Template 1"}
	}`
	yamlTemplate := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: template-2
spec:
  displayName: Template 2
`
	err = os.WriteFile(filepath.Join(tmpDir, "template-1.json"), []byte(jsonTemplate), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "template-2.yaml"), []byte(yamlTemplate), 0644)
	require.NoError(t, err)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, templates, 2)
	assert.Contains(t, templates, "template-1")
	assert.Contains(t, templates, "template-2")
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_DuplicateHandle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create two templates with the same handle
	template1 := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "duplicate-template"},
		"spec": {"displayName": "Template 1"}
	}`
	template2 := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "duplicate-template"},
		"spec": {"displayName": "Template 2"}
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "template-1.json"), []byte(template1), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "template-2.json"), []byte(template2), 0644)
	require.NoError(t, err)

	_, err = loader.LoadTemplatesFromDirectory(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate template handle")
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_InvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create an invalid JSON file
	err = os.WriteFile(filepath.Join(tmpDir, "invalid.json"), []byte("not valid json {"), 0644)
	require.NoError(t, err)

	_, err = loader.LoadTemplatesFromDirectory(tmpDir)
	assert.Error(t, err)
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_InvalidYAML(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create an invalid YAML file
	err = os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte("not: valid: yaml: ["), 0644)
	require.NoError(t, err)

	_, err = loader.LoadTemplatesFromDirectory(tmpDir)
	assert.Error(t, err)
}

func TestLLMTemplateLoader_LoadTemplatesFromDirectory_Subdirectories(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary directory with subdirectory
	tmpDir, err := os.MkdirTemp("", "llm-templates-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create templates in root and subdirectory
	rootTemplate := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "root-template"},
		"spec": {"displayName": "Root Template"}
	}`
	subTemplate := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "sub-template"},
		"spec": {"displayName": "Sub Template"}
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "root-template.json"), []byte(rootTemplate), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "sub-template.json"), []byte(subTemplate), 0644)
	require.NoError(t, err)

	templates, err := loader.LoadTemplatesFromDirectory(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, templates, 2)
	assert.Contains(t, templates, "root-template")
	assert.Contains(t, templates, "sub-template")
}

func TestLLMTemplateLoader_loadTemplateFile_NonExistent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	_, err := loader.loadTemplateFile("/non/existent/file.json")
	assert.Error(t, err)
}

func TestLLMTemplateLoader_loadTemplateFile_ValidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "template-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	jsonTemplate := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "test-template"},
		"spec": {"displayName": "Test Template"}
	}`
	_, err = tmpFile.WriteString(jsonTemplate)
	require.NoError(t, err)
	tmpFile.Close()

	template, err := loader.loadTemplateFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.Equal(t, "test-template", template.Metadata.Name)
}

func TestLLMTemplateLoader_loadTemplateFile_ValidYAML(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "template-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	yamlTemplate := `
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: yaml-template
spec:
  displayName: YAML Template
`
	_, err = tmpFile.WriteString(yamlTemplate)
	require.NoError(t, err)
	tmpFile.Close()

	template, err := loader.loadTemplateFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.Equal(t, "yaml-template", template.Metadata.Name)
}

func TestLLMTemplateLoader_loadTemplateFile_CompleteTemplate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := NewLLMTemplateLoader(logger)

	// Create a temporary file with complete template
	tmpFile, err := os.CreateTemp("", "template-*.json")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	jsonTemplate := `{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind": "LlmProviderTemplate",
		"metadata": {"name": "complete-template"},
		"spec": {
			"displayName": "Complete Template",
			"requestModel": {
				"location": "body",
				"identifier": "model"
			},
			"responseModel": {
				"location": "body",
				"identifier": "model"
			},
			"promptTokens": {
				"location": "body",
				"identifier": "usage.prompt_tokens"
			},
			"completionTokens": {
				"location": "body",
				"identifier": "usage.completion_tokens"
			},
			"totalTokens": {
				"location": "body",
				"identifier": "usage.total_tokens"
			}
		}
	}`
	_, err = tmpFile.WriteString(jsonTemplate)
	require.NoError(t, err)
	tmpFile.Close()

	template, err := loader.loadTemplateFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.Equal(t, "complete-template", template.Metadata.Name)
	assert.Equal(t, "Complete Template", template.Spec.DisplayName)
	assert.NotNil(t, template.Spec.RequestModel)
	assert.NotNil(t, template.Spec.ResponseModel)
	assert.NotNil(t, template.Spec.PromptTokens)
	assert.NotNil(t, template.Spec.CompletionTokens)
	assert.NotNil(t, template.Spec.TotalTokens)
}
