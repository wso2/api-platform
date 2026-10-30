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

package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// The loader must take groupId from the YAML's spec.groupId, independent of the
// handle (metadata.name).
func TestLoadLLMProviderTemplates_UsesGroupIdFromYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: foo-handle
spec:
  groupId: foo-group
  displayName: Foo
`
	if err := os.WriteFile(filepath.Join(dir, "foo.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	got, err := LoadLLMProviderTemplatesFromDirectory(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 template, got %d", len(got))
	}
	if got[0].ID != "foo-handle" {
		t.Errorf("handle: got %q, want foo-handle", got[0].ID)
	}
	if got[0].GroupID != "foo-group" {
		t.Errorf("groupId: got %q, want foo-group (must come from spec.groupId, not the handle)", got[0].GroupID)
	}
}

// When spec.groupId is omitted, the loader falls back to the handle so older
// templates keep working.
func TestLoadLLMProviderTemplates_GroupIdFallsBackToHandle(t *testing.T) {
	dir := t.TempDir()
	yaml := `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: bar
spec:
  displayName: Bar
`
	if err := os.WriteFile(filepath.Join(dir, "bar.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	got, err := LoadLLMProviderTemplatesFromDirectory(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].GroupID != "bar" {
		t.Fatalf("expected groupId fallback to handle 'bar', got %+v", got)
	}
}

// The shipped built-in templates must all declare a groupId.
func TestLoadLLMProviderTemplates_BuiltInsHaveGroupId(t *testing.T) {
	dir := filepath.Join("..", "..", "resources", "default-llm-provider-templates")
	got, err := LoadLLMProviderTemplatesFromDirectory(dir)
	if err != nil {
		t.Fatalf("load built-ins: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected built-in templates to load")
	}
	for _, tpl := range got {
		if tpl.GroupID == "" {
			t.Errorf("built-in %q has empty groupId", tpl.ID)
		}
	}
}

// writeTemplateFile writes one template definition into dir.
func writeTemplateFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
}

// spec.metadata.deploymentType is a metadata field: a definition that sets it keeps
// it, alongside the rest of its metadata.
func TestLoadLLMProviderTemplates_KeepsDeploymentType(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "vllm.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: vllm
spec:
  displayName: vLLM
  metadata:
    endpointUrl: https://vllm.example.com/v1
    deploymentType: self-managed
`)

	got, err := LoadLLMProviderTemplatesFromDirectory(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Metadata == nil {
		t.Fatalf("expected 1 template with metadata, got %+v", got)
	}
	if got[0].Metadata.DeploymentType != "self-managed" {
		t.Errorf("deploymentType: got %q, want self-managed", got[0].Metadata.DeploymentType)
	}
	if got[0].Metadata.EndpointURL != "https://vllm.example.com/v1" {
		t.Errorf("endpointUrl: got %q, want it kept alongside deploymentType", got[0].Metadata.EndpointURL)
	}
}

// A metadata block that holds only deploymentType is not empty and must not be dropped.
func TestLoadLLMProviderTemplates_KeepsMetadataWithOnlyDeploymentType(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "local.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: local
spec:
  displayName: Local
  metadata:
    deploymentType: self-managed
`)

	got, err := LoadLLMProviderTemplatesFromDirectory(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 || got[0].Metadata == nil || got[0].Metadata.DeploymentType != "self-managed" {
		t.Fatalf("expected metadata with deploymentType self-managed, got %+v", got)
	}
}

// The REST API accepts only managed and self-managed, so a definition with any other
// value fails to load instead of storing a value the API would reject.
func TestLoadLLMProviderTemplates_RejectsUnknownDeploymentType(t *testing.T) {
	dir := t.TempDir()
	writeTemplateFile(t, dir, "typo.yaml", `apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProviderTemplate
metadata:
  name: typo
spec:
  displayName: Typo
  metadata:
    deploymentType: self-hosted
`)

	if _, err := LoadLLMProviderTemplatesFromDirectory(dir); err == nil {
		t.Fatal("expected an error for deploymentType self-hosted, got none")
	}
}

// The shipped built-in templates are all vendor-run services, and each declares
// deploymentType managed, so operators who copy one see the field.
func TestLoadLLMProviderTemplates_BuiltInsAreManaged(t *testing.T) {
	dir := filepath.Join("..", "..", "resources", "default-llm-provider-templates")
	got, err := LoadLLMProviderTemplatesFromDirectory(dir)
	if err != nil {
		t.Fatalf("load built-ins: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected built-in templates to load")
	}
	for _, tpl := range got {
		if tpl.Metadata == nil || tpl.Metadata.DeploymentType != "managed" {
			t.Errorf("built-in %q: metadata %+v, want deploymentType managed", tpl.ID, tpl.Metadata)
		}
	}
}
