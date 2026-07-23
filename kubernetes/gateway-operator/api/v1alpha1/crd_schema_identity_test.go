/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// TestCRDVersionSchemasIdentical enforces the invariant that every served
// version of each CRD shares an identical OpenAPI schema.
//
// The operator ships its CRDs (kubernetes/helm/operator-helm-chart/crds/) serving
// v1 (storage) and v1alpha1 (served) with conversion strategy None — a multi-version
// CRD with no explicit spec.conversion block defaults to None at the API server.
// "None" performs NO field transformation between versions — the API server only
// relabels apiVersion on read/write. That is safe ONLY while the v1 (storage)
// and v1alpha1 (served) schemas are equivalent; any divergence would silently
// drop or corrupt fields at the storage boundary, with no conversion webhook to
// catch it (see conversion.go — the Hub()/ConvertTo/ConvertFrom scaffolding is
// deliberately not wired to a live webhook while the schemas match).
//
// This guard fails the moment the two versions drift, so an accidental edit to
// one version's Go types (without mirroring it in the other) cannot ship.
//
// When the versions are INTENTIONALLY diverged in the future, this test is the
// signal to switch models: delete it, flip strategy: None -> Webhook, wire
// SetupWebhookWithManager in cmd/main.go, replace the JSON round-trip in
// conversion.go with explicit per-field mapping, and rely on round-trip
// conversion tests instead.
func TestCRDVersionSchemasIdentical(t *testing.T) {
	dir := crdBasesDir(t)
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatalf("globbing CRD dir %s: %v", dir, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no CRD YAML files found in %s (run `make manifests`?)", dir)
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			var crd apiextensionsv1.CustomResourceDefinition
			if err := yaml.Unmarshal(data, &crd); err != nil {
				t.Fatalf("unmarshal %s: %v", path, err)
			}

			versions := crd.Spec.Versions
			if len(versions) < 2 {
				t.Skipf("%s serves a single version; nothing to compare", crd.Name)
			}

			ref := storageVersion(versions)
			if ref.Schema == nil || ref.Schema.OpenAPIV3Schema == nil {
				t.Fatalf("storage version %q of %s has no schema", ref.Name, crd.Name)
			}

			for _, v := range versions {
				if v.Name == ref.Name {
					continue
				}
				if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
					t.Fatalf("version %q of %s has no schema", v.Name, crd.Name)
				}
				if !reflect.DeepEqual(ref.Schema.OpenAPIV3Schema, v.Schema.OpenAPIV3Schema) {
					t.Errorf("schema for version %q differs from storage version %q of %s.\n"+
						"conversion strategy None requires identical schemas — mirror the change "+
						"in both api/v1 and api/v1alpha1 (and re-run `make manifests`), or move to a "+
						"real conversion webhook (see this test's doc comment).\n%s",
						v.Name, ref.Name, crd.Name,
						schemaDiff(ref.Schema.OpenAPIV3Schema, v.Schema.OpenAPIV3Schema))
				}
			}
		})
	}
}

// storageVersion returns the version flagged storage:true, falling back to the
// first entry (which should never happen for a well-formed multi-version CRD).
func storageVersion(vs []apiextensionsv1.CustomResourceDefinitionVersion) apiextensionsv1.CustomResourceDefinitionVersion {
	for _, v := range vs {
		if v.Storage {
			return v
		}
	}
	return vs[0]
}

// crdBasesDir resolves config/crd/bases relative to this test file, so the test
// passes regardless of the working directory `go test` is invoked from.
func crdBasesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	// api/v1alpha1/<thisfile> -> ../../config/crd/bases
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "crd", "bases")
}

// schemaDiff renders the first divergence between two schemas as indented JSON,
// so a failure points at the offending field instead of dumping the whole tree.
func schemaDiff(a, b any) string {
	aj, _ := json.MarshalIndent(a, "", "  ")
	bj, _ := json.MarshalIndent(b, "", "  ")
	al := strings.Split(string(aj), "\n")
	bl := strings.Split(string(bj), "\n")

	n := min(len(al), len(bl))
	for i := 0; i < n; i++ {
		if al[i] != bl[i] {
			return renderContext(al, bl, i)
		}
	}
	if len(al) != len(bl) {
		return renderContext(al, bl, n)
	}
	// Equal as JSON but not under reflect.DeepEqual (e.g. nil vs empty slice).
	// Both serialize identically, so the drift is not field-content — flag it plainly.
	return "(schemas differ only in nil-vs-empty representation; check the Go types)"
}

func renderContext(al, bl []string, i int) string {
	const ctx = 4
	start := i - ctx
	if start < 0 {
		start = 0
	}
	var sb strings.Builder
	sb.WriteString("first divergence near line " + strconv.Itoa(i+1) + ":\n")
	sb.WriteString("--- storage version\n")
	for j := start; j <= i && j < len(al); j++ {
		sb.WriteString("  " + al[j] + "\n")
	}
	sb.WriteString("+++ other version\n")
	for j := start; j <= i && j < len(bl); j++ {
		sb.WriteString("  " + bl[j] + "\n")
	}
	return sb.String()
}
