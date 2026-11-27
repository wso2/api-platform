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
// version of each CRD shares an identical OpenAPI schema, apart from the fields
// listed in fieldsAddedInVersion.
//
// The operator ships its CRDs (kubernetes/helm/operator-helm-chart/crds/) serving
// v1 (storage) and v1alpha1 (served). Kinds introduced after the v1 promotion (Agent)
// are served as v1 only and are skipped below, since a single-version CRD has nothing
// to compare.
//
// Most CRDs use conversion strategy None: the API server only relabels apiVersion
// on read/write. That is safe ONLY while the served schemas are identical; any
// divergence would silently drop or corrupt fields at the storage boundary.
//
// The one exception is the APIGateway CRD, which also serves v2alpha1. v2alpha1 is
// v1 plus spec.adminCredentials, and v1 and v1alpha1 keep their released schemas.
// That CRD uses strategy Webhook: the operator's conversion webhook
// (internal/conversionwebhook, api/v2alpha1/conversion.go) keeps the v2alpha1-only
// fields in an annotation on the stored v1 object. fieldsAddedInVersion records the
// fields v2alpha1 adds, nothing else may differ, and TestChartCRDs checks that every
// CRD whose versions differ ships with the webhook strategy.
//
// This guard fails the moment versions drift unintentionally, so an edit to one
// version's Go types (without mirroring it in the others) cannot ship.

// fieldsAddedInVersion lists, per CRD and served version, the fields that version
// declares beyond the storage version. Each entry is a path of property names from
// the schema root. A CRD listed here must convert with a webhook that preserves them.
var fieldsAddedInVersion = map[string]map[string][][]string{
	"apigateways.gateway.api-platform.wso2.com": {
		"v2alpha1": {{"spec", "adminCredentials"}},
	},
}

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
			crd := readCRD(t, path)
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
				got := v.Schema.OpenAPIV3Schema
				if added := fieldsAddedInVersion[crd.Name][v.Name]; len(added) > 0 {
					got = got.DeepCopy()
					for _, path := range added {
						if !removeSchemaProperty(got, path) {
							t.Fatalf("version %q of %s has no %s; remove the stale fieldsAddedInVersion entry",
								v.Name, crd.Name, strings.Join(path, "."))
						}
					}
				}
				if !reflect.DeepEqual(ref.Schema.OpenAPIV3Schema, got) {
					t.Errorf("schema for version %q differs from storage version %q of %s.\n"+
						"mirror the change in api/v1 and every other served version (api/v1alpha1, "+
						"and api/v2alpha1 for APIGateway) and re-run `make manifests`; a field only a "+
						"newer version declares must be listed in fieldsAddedInVersion and preserved "+
						"by its conversion webhook.\n%s",
						v.Name, ref.Name, crd.Name,
						schemaDiff(ref.Schema.OpenAPIV3Schema, got))
				}
			}
		})
	}
}

// TestChartCRDs checks that the operator chart ships exactly the generated CRDs
// (`make helm-crds`), and that a CRD whose versions differ converts through the
// operator's webhook while every other CRD keeps strategy None.
func TestChartCRDs(t *testing.T) {
	bases := crdBasesDir(t)
	chart := filepath.Join(bases, "..", "..", "..", "..", "helm", "operator-helm-chart", "crds")
	paths, err := filepath.Glob(filepath.Join(bases, "*.yaml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no CRDs in %s: %v", bases, err)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			generated := readCRD(t, path)
			shipped := readCRD(t, filepath.Join(chart, name))

			if generated.Spec.Conversion != nil {
				t.Fatalf("config/crd/bases/%s has spec.conversion; controller-gen output must not", name)
			}
			_, differs := fieldsAddedInVersion[generated.Name]
			conv := shipped.Spec.Conversion
			if differs {
				if conv == nil || conv.Strategy != apiextensionsv1.WebhookConverter || conv.Webhook == nil ||
					conv.Webhook.ClientConfig == nil || conv.Webhook.ClientConfig.Service == nil ||
					conv.Webhook.ClientConfig.Service.Path == nil || *conv.Webhook.ClientConfig.Service.Path != "/convert" ||
					!reflect.DeepEqual(conv.Webhook.ConversionReviewVersions, []string{"v1"}) {
					t.Fatalf("chart CRD %s serves versions that differ but does not convert through the "+
						"operator's /convert webhook: %+v (run `make helm-crds`)", generated.Name, conv)
				}
			} else if conv != nil && conv.Strategy != apiextensionsv1.NoneConverter {
				t.Fatalf("chart CRD %s has identical versions but conversion %q", generated.Name, conv.Strategy)
			}

			shipped.Spec.Conversion = nil
			if !reflect.DeepEqual(generated, shipped) {
				t.Fatalf("chart CRD %s differs from config/crd/bases (run `make helm-crds`)", name)
			}
		})
	}
}

func readCRD(t *testing.T, path string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(data, crd); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return crd
}

// removeSchemaProperty deletes the property at path (property names from the
// schema root) and reports whether it was there.
func removeSchemaProperty(s *apiextensionsv1.JSONSchemaProps, path []string) bool {
	if len(path) == 0 {
		return false
	}
	child, ok := s.Properties[path[0]]
	if !ok {
		return false
	}
	if len(path) == 1 {
		delete(s.Properties, path[0])
		return true
	}
	if !removeSchemaProperty(&child, path[1:]) {
		return false
	}
	s.Properties[path[0]] = child
	return true
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
