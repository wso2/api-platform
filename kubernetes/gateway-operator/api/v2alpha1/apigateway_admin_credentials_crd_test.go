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
package v2alpha1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// apiGatewayServedVersion is one served version of the generated APIGateway CRD.
type apiGatewayServedVersion struct {
	// validate runs the structural schema validation and the CEL
	// x-kubernetes-validations rules the API server applies on admission.
	validate func(obj map[string]interface{}) field.ErrorList
	// structural is the version's structural schema, which the API server prunes
	// unknown fields against.
	structural *structuralschema.Structural
	// hasAdminCredentials reports whether the version's spec declares adminCredentials.
	hasAdminCredentials bool
}

// apiGatewayServedVersions returns every served version of the generated APIGateway CRD.
func apiGatewayServedVersions(t *testing.T) map[string]apiGatewayServedVersion {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "config", "crd", "bases",
		"gateway.api-platform.wso2.com_apigateways.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run `make manifests`?): %v", path, err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}

	out := map[string]apiGatewayServedVersion{}
	for _, v := range crd.Spec.Versions {
		if !v.Served {
			continue
		}
		internal := &apiextensions.JSONSchemaProps{}
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
			v.Schema.OpenAPIV3Schema, internal, nil); err != nil {
			t.Fatalf("convert %q schema: %v", v.Name, err)
		}
		schemaValidator, _, err := validation.NewSchemaValidator(internal)
		if err != nil {
			t.Fatalf("build validator for %q: %v", v.Name, err)
		}
		structural, err := structuralschema.NewStructural(internal)
		if err != nil {
			t.Fatalf("build structural schema for %q: %v", v.Name, err)
		}
		celValidator := cel.NewValidator(structural, true, celconfig.PerCallLimit)
		_, hasAdminCredentials := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["adminCredentials"]
		out[v.Name] = apiGatewayServedVersion{
			validate: func(obj map[string]interface{}) field.ErrorList {
				errs := validation.ValidateCustomResource(nil, obj, schemaValidator)
				celErrs, _ := celValidator.Validate(t.Context(), nil, structural, obj, nil, celconfig.RuntimeCELCostBudget)
				return append(errs, celErrs...)
			},
			structural:          structural,
			hasAdminCredentials: hasAdminCredentials,
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s serves no versions", crd.Name)
	}
	return out
}

func apiGatewayWithAdminCredentials(version string, creds map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "gateway.api-platform.wso2.com/" + version,
		"kind":       "APIGateway",
		"metadata":   map[string]interface{}{"name": "gateway-1", "namespace": "gw1"},
		"spec": map[string]interface{}{
			"apiSelector":      map[string]interface{}{"scope": "Cluster"},
			"adminCredentials": creds,
		},
	}
}

// spec.adminCredentials is declared by v2alpha1 only. v1 (storage) and v1alpha1 keep
// their released schemas, so the API server prunes the field from a v1 or v1alpha1
// object; through v2alpha1 the conversion webhook keeps it in the conversion-data
// annotation on the stored v1 object (see conversion.go).
func TestAPIGatewayCRDServesAdminCredentialsInV2alpha1Only(t *testing.T) {
	versions := apiGatewayServedVersions(t)
	want := map[string]bool{"v1": false, "v1alpha1": false, "v2alpha1": true}
	if len(versions) != len(want) {
		t.Fatalf("APIGateway CRD serves %d versions, want %d (v1, v1alpha1, v2alpha1)", len(versions), len(want))
	}
	for version, wantField := range want {
		v, ok := versions[version]
		if !ok {
			t.Fatalf("APIGateway CRD does not serve %s", version)
		}
		if v.hasAdminCredentials != wantField {
			t.Errorf("%s: spec.adminCredentials declared = %v, want %v", version, v.hasAdminCredentials, wantField)
		}
	}

	// The admin password is always delivered as a file: there is no source field, and
	// one sent by a client is pruned.
	t.Run("v2alpha1/no source field", func(t *testing.T) {
		obj := apiGatewayWithAdminCredentials("v2alpha1", map[string]interface{}{"source": "env",
			"passwordSecretRef": map[string]interface{}{"name": "gateway-1-admin", "key": "password"}})
		pruning.Prune(obj, versions["v2alpha1"].structural, true)
		if _, kept := obj["spec"].(map[string]interface{})["adminCredentials"].(map[string]interface{})["source"]; kept {
			t.Error("spec.adminCredentials.source survived pruning")
		}
	})

	creds := map[string]interface{}{"username": "gw1-admin", "passwordSecretRef": map[string]interface{}{"name": "gateway-1-admin", "key": "password"}}
	for version, wantField := range want {
		t.Run(version+"/pruning", func(t *testing.T) {
			obj := apiGatewayWithAdminCredentials(version, creds)
			pruning.Prune(obj, versions[version].structural, true)
			_, kept := obj["spec"].(map[string]interface{})["adminCredentials"]
			if kept != wantField {
				t.Errorf("spec.adminCredentials kept after pruning = %v, want %v", kept, wantField)
			}
		})
	}
}

// spec.adminCredentials values that can never work are rejected on admission, on every
// version that serves the field, instead of failing later at reconcile or rendering
// invalid TOML.
func TestAPIGatewayCRDValidatesAdminCredentials(t *testing.T) {
	ref := func(name, key string) map[string]interface{} {
		r := map[string]interface{}{"key": key}
		if name != "" {
			r["name"] = name
		}
		return r
	}
	accepted := map[string]map[string]interface{}{
		"username":              {"username": "gw1-admin", "passwordSecretRef": ref("gateway-1-admin", "password")},
		"username omitted":      {"passwordSecretRef": ref("gateway-1-admin", "password")},
		"dotted key":            {"username": "ops.admin@example.com", "passwordSecretRef": ref("gateway-1-admin", "admin.password_1")},
		"dotted name":           {"passwordSecretRef": ref("gateway.admin", "password")},
		"key of 253 characters": {"passwordSecretRef": ref("gateway-1-admin", strings.Repeat("k", 253))},
		"optional false":        {"passwordSecretRef": map[string]interface{}{"name": "gateway-1-admin", "key": "password", "optional": false}},
	}
	rejected := map[string]struct {
		creds   map[string]interface{}
		wantErr string
	}{
		"name missing":          {creds: map[string]interface{}{"passwordSecretRef": ref("", "password")}, wantErr: "passwordSecretRef.name must name the Secret"},
		"name empty":            {creds: map[string]interface{}{"passwordSecretRef": map[string]interface{}{"name": "", "key": "password"}}, wantErr: "passwordSecretRef.name must name the Secret"},
		"key empty":             {creds: map[string]interface{}{"passwordSecretRef": ref("gateway-1-admin", "")}, wantErr: "passwordSecretRef.key must be a Secret key name"},
		"key with slash":        {creds: map[string]interface{}{"passwordSecretRef": ref("gateway-1-admin", "a/b")}, wantErr: "passwordSecretRef.key must be a Secret key name"},
		"key dot-dot":           {creds: map[string]interface{}{"passwordSecretRef": ref("gateway-1-admin", "..")}, wantErr: "passwordSecretRef.key must be a Secret key name"},
		"key starting with ..":  {creds: map[string]interface{}{"passwordSecretRef": ref("gateway-1-admin", "..password")}, wantErr: "passwordSecretRef.key must be a Secret key name"},
		"key over 253":          {creds: map[string]interface{}{"passwordSecretRef": ref("gateway-1-admin", strings.Repeat("k", 254))}, wantErr: "passwordSecretRef.key must be a Secret key name"},
		"key with ..":           {creds: map[string]interface{}{"passwordSecretRef": ref("gateway-1-admin", "admin..password")}, wantErr: "passwordSecretRef.key must be a Secret key name"},
		"name with underscore":  {creds: map[string]interface{}{"passwordSecretRef": ref("Gateway_Admin", "password")}, wantErr: "passwordSecretRef.name must be a Secret name"},
		"name over 253":         {creds: map[string]interface{}{"passwordSecretRef": ref(strings.Repeat("a", 254), "password")}, wantErr: "passwordSecretRef.name must be a Secret name"},
		"optional true":         {creds: map[string]interface{}{"passwordSecretRef": map[string]interface{}{"name": "gateway-1-admin", "key": "password", "optional": true}}, wantErr: "passwordSecretRef.optional must not be true"},
		"username with colon":   {creds: map[string]interface{}{"username": "gw1:admin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username with newline": {creds: map[string]interface{}{"username": "gw1\nadmin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username with NUL":     {creds: map[string]interface{}{"username": "gw1\x00admin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username with DEL":     {creds: map[string]interface{}{"username": "gw1\x7fadmin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username empty":        {creds: map[string]interface{}{"username": "", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
	}

	// The operator re-validates spec.adminCredentials it reads from the conversion-data
	// annotation (AdminCredentialsOf), because a v1 client can write that annotation
	// directly. Its rules must agree with the CRD's.
	for name, creds := range accepted {
		t.Run("Validate/accepts "+name, func(t *testing.T) {
			if err := decodeAndValidate(t, creds); err != nil {
				t.Fatalf("Validate rejected a value the CRD accepts: %v", err)
			}
		})
	}
	for name, tc := range rejected {
		if name == "username empty" {
			// The CRD rejects an explicit "", but an omitted username is defaulted.
			// The annotation never distinguishes the two (omitempty), so the operator
			// defaults it like an omitted one.
			continue
		}
		t.Run("Validate/rejects "+name, func(t *testing.T) {
			if err := decodeAndValidate(t, tc.creds); err == nil {
				t.Fatal("Validate accepted a value the CRD rejects")
			}
		})
	}

	for version, v := range apiGatewayServedVersions(t) {
		if !v.hasAdminCredentials {
			continue
		}
		validate := v.validate
		for name, creds := range accepted {
			t.Run(version+"/accepts "+name, func(t *testing.T) {
				if errs := validate(apiGatewayWithAdminCredentials(version, creds)); len(errs) > 0 {
					t.Fatalf("rejected a valid adminCredentials: %v", errs.ToAggregate())
				}
			})
		}
		for name, tc := range rejected {
			t.Run(version+"/rejects "+name, func(t *testing.T) {
				errs := validate(apiGatewayWithAdminCredentials(version, tc.creds))
				if len(errs) == 0 {
					t.Fatal("accepted an invalid adminCredentials")
				}
				if !strings.Contains(errs.ToAggregate().Error(), tc.wantErr) {
					t.Errorf("rejected, but not for %q: %v", tc.wantErr, errs.ToAggregate())
				}
			})
		}
	}
}

// decodeAndValidate decodes an adminCredentials object as the operator does and runs
// Default and Validate.
func decodeAndValidate(t *testing.T, creds map[string]interface{}) error {
	t.Helper()
	raw, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var c GatewayAdminCredentials
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	c.Default()
	return c.Validate()
}
