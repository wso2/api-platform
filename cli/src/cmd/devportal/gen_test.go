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
