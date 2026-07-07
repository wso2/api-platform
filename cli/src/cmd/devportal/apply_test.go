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
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internaldevportal "github.com/wso2/api-platform/cli/internal/devportal"
)

func TestResolveApplyTarget(t *testing.T) {
	cases := []struct {
		kind           string
		wantField      string
		wantOrgScoped  bool
		wantSupportsUp bool
		wantEndpoint   string
	}{
		{kindOrganization, "organization", false, true, "/organizations"},
		{kindSubscriptionPolicy, "subscriptionPolicy", true, false, internaldevportal.OrgScopedPath("org-1", "subscription-policies")},
		{kindSubscriptionPolicyList, "subscriptionPolicy", true, false, internaldevportal.OrgScopedPath("org-1", "subscription-policies")},
		{kindRestAPI, "artifact", true, true, internaldevportal.OrgScopedPath("org-1", "apis")},
	}
	for _, tc := range cases {
		target, err := resolveApplyTarget(tc.kind)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.kind, err)
		}
		if target.multipartField != tc.wantField {
			t.Errorf("%s: field = %q, want %q", tc.kind, target.multipartField, tc.wantField)
		}
		if target.orgScoped != tc.wantOrgScoped {
			t.Errorf("%s: orgScoped = %v, want %v", tc.kind, target.orgScoped, tc.wantOrgScoped)
		}
		if target.supportsUpdate != tc.wantSupportsUp {
			t.Errorf("%s: supportsUpdate = %v, want %v", tc.kind, target.supportsUpdate, tc.wantSupportsUp)
		}
		if got := target.collection("org-1"); got != tc.wantEndpoint {
			t.Errorf("%s: collection = %q, want %q", tc.kind, got, tc.wantEndpoint)
		}
	}
}

func TestResolveApplyTarget_UnsupportedKind(t *testing.T) {
	if _, err := resolveApplyTarget("Banana"); err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("expected unsupported kind error, got %v", err)
	}
}

func TestDetectApplyResource_YAMLCRKinds(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name       string
		content    string
		wantKind   string
		wantHandle string
	}{
		{"org.yaml", "kind: Organization\nmetadata:\n  name: acme\n", kindOrganization, "acme"},
		{"plan.yaml", "kind: SubscriptionPolicy\nmetadata:\n  name: Gold\n", kindSubscriptionPolicy, "Gold"},
		{"plans.yaml", "kind: SubscriptionPolicyList\nitems:\n  - metadata:\n      name: Gold\n", kindSubscriptionPolicyList, ""},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.name)
		if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
			t.Fatalf("write %s: %v", tc.name, err)
		}
		kind, handle, err := detectApplyResource(path)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if kind != tc.wantKind || handle != tc.wantHandle {
			t.Fatalf("%s: got (kind=%q, handle=%q), want (kind=%q, handle=%q)", tc.name, kind, handle, tc.wantKind, tc.wantHandle)
		}
	}
}

func TestDetectApplyResource_RejectsRestApiAsYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.yaml")
	if err := os.WriteFile(path, []byte("kind: RestApi\nmetadata:\n  name: foo\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := detectApplyResource(path); err == nil || !strings.Contains(err.Error(), "must be provided as a built .zip") {
		t.Fatalf("expected RestApi-as-YAML rejection, got %v", err)
	}
}

func TestDetectApplyResource_UnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "org.txt")
	if err := os.WriteFile(path, []byte("kind: Organization\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := detectApplyResource(path); err == nil || !strings.Contains(err.Error(), "unsupported file type") {
		t.Fatalf("expected unsupported file type error, got %v", err)
	}
}

func TestDetectApplyResource_YAMLValidation(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"no-kind.yaml", "metadata:\n  name: x\n", "'kind' field is required"},
		{"no-name.yaml", "kind: SubscriptionPolicy\nspec:\n  displayName: Gold\n", "metadata.name is required"},
		{"empty-list.yaml", "kind: SubscriptionPolicyList\nitems: []\n", "at least one entry"},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, tc.name)
		if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
			t.Fatalf("write %s: %v", tc.name, err)
		}
		_, _, err := detectApplyResource(path)
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Fatalf("%s: expected error containing %q, got %v", tc.name, tc.wantErr, err)
		}
	}
}

func TestDetectApplyResource_ZipArtifact(t *testing.T) {
	// A REST API artifact whose devportal.yaml declares kind: RestApi resolves to
	// the RestApi kind and its handle (metadata.name), both read from the zip.
	zipPath := writeArtifactZip(t, "kind: RestApi\nmetadata:\n  name: foo-1.0.0\n")
	kind, handle, err := detectApplyResource(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != kindRestAPI || handle != "foo-1.0.0" {
		t.Fatalf("got (kind=%q, handle=%q), want (kind=%q, handle=%q)", kind, handle, kindRestAPI, "foo-1.0.0")
	}
}

func TestDetectApplyResource_ZipWrongKindRejected(t *testing.T) {
	zipPath := writeArtifactZip(t, "kind: Organization\nmetadata:\n  name: acme\n")
	if _, _, err := detectApplyResource(zipPath); err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("expected unsupported kind error for non-RestApi zip, got %v", err)
	}
}

func TestDetectApplyResource_ZipMissingManifest(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "empty.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)
	if w, err := zw.Create("definition.yaml"); err == nil {
		_, _ = w.Write([]byte("openapi: 3.0.0\n"))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	_ = f.Close()

	if _, _, err := detectApplyResource(zipPath); err == nil || !strings.Contains(err.Error(), "does not contain "+archiveMetadataFileName) {
		t.Fatalf("expected missing-manifest error, got %v", err)
	}
}

// writeArtifactZip creates a zip in a temp dir containing a devportal.yaml with
// the given content, mirroring what `ap devportal build` produces.
func writeArtifactZip(t *testing.T, manifest string) string {
	t.Helper()
	zipPath := filepath.Join(t.TempDir(), "devportal.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	w, err := zw.Create(archiveMetadataFileName)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return zipPath
}
