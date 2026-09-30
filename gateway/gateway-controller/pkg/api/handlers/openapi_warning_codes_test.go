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

package handlers

import (
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/gatewayidentity"
)

// codeEnum reads schemaName's "code" property enum from the embedded OpenAPI
// spec and returns it as a set. It fails the test if there is no enum.
func codeEnum(t *testing.T, schemaName string) map[string]bool {
	t.Helper()

	swagger, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("failed to load embedded OpenAPI spec: %v", err)
	}

	schemaRef, ok := swagger.Components.Schemas[schemaName]
	if !ok || schemaRef.Value == nil {
		t.Fatalf("spec has no schema %q", schemaName)
	}
	codeProp, ok := schemaRef.Value.Properties["code"]
	if !ok || codeProp.Value == nil {
		t.Fatalf("schema %q has no 'code' property", schemaName)
	}
	if len(codeProp.Value.Enum) == 0 {
		t.Fatalf("schema %q's code property has no enum — it is still only documented via `example`", schemaName)
	}

	enum := make(map[string]bool, len(codeProp.Value.Enum))
	for _, v := range codeProp.Value.Enum {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("schema %q's code enum contains a non-string value: %v", schemaName, v)
		}
		enum[s] = true
	}
	return enum
}

// Every certificate warning code is declared in CertificateWarning.code.
func TestOpenAPI_CertificateWarningCodeEnum_CoversEveryEmittedCode(t *testing.T) {
	enum := codeEnum(t, "CertificateWarning")

	emitted := []string{
		clientca.CodeClientCAIsLeaf,
		clientca.CodeClientCANotYetValid,
		clientca.CodeCertExpiresSoon,
		gatewayidentity.CodeNoClientAuthEKU,
	}
	for _, code := range emitted {
		if !enum[code] {
			t.Errorf("CertificateWarning.code enum is missing %q", code)
		}
	}
}

// Every deploy warning code is declared in Warning.code.
func TestOpenAPI_WarningCodeEnum_CoversEveryEmittedCode(t *testing.T) {
	enum := codeEnum(t, "Warning")

	emitted := []string{
		config.WarningCodeMTLSAcceptInheritsPool,
		config.WarningCodeMTLSAcceptUnnarrowed,
		config.WarningCodeMTLSAuthNotFirst,
		config.WarningCodeMTLSAcceptNamesRelayAuthority,
		config.WarningCodeMTLSThumbprintNormalised,
		config.WarningCodeHeaderCertBypassActive,
		config.WarningCodeTLSVerifyHostNameDisabled,
		config.WarningCodeTLSIdentityExpired,
	}
	for _, code := range emitted {
		if !enum[code] {
			t.Errorf("Warning.code enum is missing %q", code)
		}
	}
}
