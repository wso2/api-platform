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
package devportal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRunGenCommand_RequiresProjectDirectory(t *testing.T) {
	genProjectDir = t.TempDir()

	err := runGenCommand()
	if err == nil || err.Error() != "unable to find project directory, please execute this command inside a project" {
		t.Fatalf("expected project directory error, got %v", err)
	}
}

func TestRunGenCommand_GeneratesDefaultArtifact(t *testing.T) {
	projectRoot := createProjectFixture(t)
	// The shared fixture's metadata.yaml omits displayName/version, which gen
	// sources from metadata.yaml, so provide them here.
	metadata := "apiVersion: management.api-platform.wso2.com/v1\nkind: RestApi\nmetadata:\n  name: booking-api-v1.0\nspec:\n  displayName: Booking API\n  version: v1.0\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "metadata.yaml"), []byte(metadata), 0644); err != nil {
		t.Fatalf("failed to write metadata fixture: %v", err)
	}
	genProjectDir = projectRoot

	if err := runGenCommand(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, path := range []string{
		filepath.Join(projectRoot, "devportal"),
		filepath.Join(projectRoot, "devportal", "devportal.yaml"),
		filepath.Join(projectRoot, "devportal", "definition.yaml"),
		filepath.Join(projectRoot, "devportal", "docs"),
		filepath.Join(projectRoot, "devportal", "content"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected path to exist: %s (%v)", path, err)
		}
	}

	manifestData, err := os.ReadFile(filepath.Join(projectRoot, "devportal", "devportal.yaml"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	manifest := string(manifestData)
	for _, want := range []string{
		"name: booking-api-v1.0",
		"type: REST",
		"displayName: Booking API",
		"version: v1.0",
		"provider: WSO2",
		"gatewayType: wso2/api-platform",
		"visibility: PUBLIC",
		"- default",
	} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("expected manifest to contain %q, got:\n%s", want, manifest)
		}
	}

	// definition.yaml must be copied from the project home definition.
	definitionData, err := os.ReadFile(filepath.Join(projectRoot, "devportal", "definition.yaml"))
	if err != nil {
		t.Fatalf("failed to read copied definition: %v", err)
	}
	if !strings.Contains(string(definitionData), "title: Petstore API") {
		t.Fatalf("expected copied definition to match project home definition, got %q", string(definitionData))
	}
}

func TestRunGenCommand_StopsWhenDevPortalExists(t *testing.T) {
	projectRoot := createProjectFixture(t)
	if err := os.MkdirAll(filepath.Join(projectRoot, "devportal"), 0755); err != nil {
		t.Fatalf("failed to pre-create devportal dir: %v", err)
	}
	genProjectDir = projectRoot

	err := runGenCommand()
	if err == nil || !strings.Contains(err.Error(), "devportal directory already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
}

func TestRenderGeneratedDevPortalManifest_EscapesSpecialCharacters(t *testing.T) {
	// Values with YAML-significant characters must not break the generated
	// manifest: it must stay valid YAML and round-trip to the same values.
	kind := "RestApi"
	name := "foo: bar #1"
	displayName := "*My \"API\": v2"
	version := "1.0"

	manifest := renderGeneratedDevPortalManifest(kind, name, displayName, version)

	var parsed struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			DisplayName string `yaml:"displayName"`
			Version     string `yaml:"version"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(manifest), &parsed); err != nil {
		t.Fatalf("generated manifest is not valid YAML: %v\n%s", err, manifest)
	}

	if parsed.Kind != kind {
		t.Errorf("kind: got %q, want %q", parsed.Kind, kind)
	}
	if parsed.Metadata.Name != name {
		t.Errorf("metadata.name: got %q, want %q", parsed.Metadata.Name, name)
	}
	if parsed.Spec.DisplayName != displayName {
		t.Errorf("spec.displayName: got %q, want %q", parsed.Spec.DisplayName, displayName)
	}
	if parsed.Spec.Version != version {
		t.Errorf("spec.version: got %q, want %q", parsed.Spec.Version, version)
	}
}

func TestRunGenCommand_LeavesNoPartialDirWhenDefinitionMissing(t *testing.T) {
	projectRoot := createProjectFixture(t)
	// Remove the project's home definition so generation fails after inputs are
	// verified but before (previously) the directory would have been created.
	if err := os.Remove(filepath.Join(projectRoot, "definition.yaml")); err != nil {
		t.Fatalf("failed to remove project definition: %v", err)
	}
	genProjectDir = projectRoot

	err := runGenCommand()
	if err == nil || !strings.Contains(err.Error(), "unable to find project definition file") {
		t.Fatalf("expected missing-definition error, got %v", err)
	}

	// The failed run must not leave a devportal directory behind (which would
	// otherwise trip the already-exists guard on the next run).
	if _, statErr := os.Stat(filepath.Join(projectRoot, "devportal")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no devportal directory after a failed gen, got err=%v", statErr)
	}
}
