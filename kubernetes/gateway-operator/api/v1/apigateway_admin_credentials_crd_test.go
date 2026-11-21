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
package v1

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// apiGatewayAdmissionValidators returns, per served version of the generated APIGateway
// CRD, a function running the structural schema validation and the CEL
// x-kubernetes-validations rules the API server applies on admission.
func apiGatewayAdmissionValidators(t *testing.T) map[string]func(obj map[string]interface{}) field.ErrorList {
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

	out := map[string]func(map[string]interface{}) field.ErrorList{}
	for _, v := range crd.Spec.Versions {
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
		out[v.Name] = func(obj map[string]interface{}) field.ErrorList {
			errs := validation.ValidateCustomResource(nil, obj, schemaValidator)
			celErrs, _ := celValidator.Validate(t.Context(), nil, structural, obj, nil, celconfig.RuntimeCELCostBudget)
			return append(errs, celErrs...)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s serves no versions", crd.Name)
	}
	return out
}

func apiGatewayWithAdminCredentials(creds map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "gateway.api-platform.wso2.com/v1",
		"kind":       "APIGateway",
		"metadata":   map[string]interface{}{"name": "gateway-1", "namespace": "gw1"},
		"spec": map[string]interface{}{
			"apiSelector":      map[string]interface{}{"scope": "Cluster"},
			"adminCredentials": creds,
		},
	}
}

// spec.adminCredentials values that can never work are rejected on admission, on every
// served version, instead of failing later at reconcile or rendering invalid TOML.
func TestAPIGatewayCRDValidatesAdminCredentials(t *testing.T) {
	ref := func(name, key string) map[string]interface{} {
		r := map[string]interface{}{"key": key}
		if name != "" {
			r["name"] = name
		}
		return r
	}
	accepted := map[string]map[string]interface{}{
		"username and file source": {"username": "gw1-admin", "source": "file", "passwordSecretRef": ref("gateway-1-admin", "password")},
		"username omitted":         {"passwordSecretRef": ref("gateway-1-admin", "password")},
		"dotted key":               {"username": "ops.admin@example.com", "passwordSecretRef": ref("gateway-1-admin", "admin.password_1")},
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
		"username with colon":   {creds: map[string]interface{}{"username": "gw1:admin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username with newline": {creds: map[string]interface{}{"username": "gw1\nadmin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username with NUL":     {creds: map[string]interface{}{"username": "gw1\x00admin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username with DEL":     {creds: map[string]interface{}{"username": "gw1\x7fadmin", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"username empty":        {creds: map[string]interface{}{"username": "", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.username"},
		"unknown source":        {creds: map[string]interface{}{"source": "vault", "passwordSecretRef": ref("gateway-1-admin", "password")}, wantErr: "spec.adminCredentials.source"},
	}

	for version, validate := range apiGatewayAdmissionValidators(t) {
		for name, creds := range accepted {
			t.Run(version+"/accepts "+name, func(t *testing.T) {
				if errs := validate(apiGatewayWithAdminCredentials(creds)); len(errs) > 0 {
					t.Fatalf("rejected a valid adminCredentials: %v", errs.ToAggregate())
				}
			})
		}
		for name, tc := range rejected {
			t.Run(version+"/rejects "+name, func(t *testing.T) {
				errs := validate(apiGatewayWithAdminCredentials(tc.creds))
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
