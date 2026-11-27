/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package azureauthentication

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// Policies list the types in their messages and docs in this order, and a caller that changes the
// returned slice must not change what the next caller gets.
func TestTypes(t *testing.T) {
	want := []string{"workload-identity", "managed-identity", "container-apps"}
	got := Types()
	if !slices.Equal(got, want) {
		t.Fatalf("Types() = %v, want %v", got, want)
	}
	got[0] = "changed"
	if again := Types(); !slices.Equal(again, want) {
		t.Fatalf("Types() = %v after the caller changed its copy, want %v", again, want)
	}
}

func TestValidate(t *testing.T) {
	valid := []Identity{
		{Type: "workload-identity"},
		{Type: "workload-identity", ClientID: testClientID, TenantID: testTenantID},
		{Type: "workload-identity", TenantID: "contoso.onmicrosoft.com"},
		{Type: "managed-identity"},
		{Type: "managed-identity", ClientID: strings.ToUpper(testClientID)},
		{Type: "container-apps", ClientID: testClientID},
	}
	for _, id := range valid {
		if err := id.Validate(); err != nil {
			t.Errorf("%+v: Validate() = %v, want nil", id, err)
		}
	}

	invalid := []struct {
		name string
		id   Identity
		want string
	}{
		{"no type", Identity{}, "authentication type is required"},
		{"an unsupported type", Identity{Type: "client-secret"}, `"client-secret"`},
		{"a client ID that is not a GUID", Identity{Type: "managed-identity", ClientID: "my-app"}, "client ID"},
		{"a client ID with extra text", Identity{Type: "container-apps", ClientID: testClientID + "x"}, "client ID"},
		{"a tenant with a slash", Identity{Type: "workload-identity", TenantID: "a/b"}, "tenant ID"},
		{"a tenant with a dot dot", Identity{Type: "workload-identity", TenantID: "contoso..com"}, "tenant ID"},
		{"a tenant with a query", Identity{Type: "workload-identity", TenantID: "contoso?x=1"}, "tenant ID"},
		{"a tenant for managed identity", Identity{Type: "managed-identity", TenantID: testTenantID}, `tenant ID is not used with authentication type "managed-identity"`},
		{"a tenant for Container Apps", Identity{Type: "container-apps", TenantID: testTenantID}, `tenant ID is not used with authentication type "container-apps"`},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.id.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want an error with %q", err, tt.want)
			}
		})
	}
}

func TestValidateScope(t *testing.T) {
	for _, scope := range []string{"https://cognitiveservices.azure.com/.default", "https://ai.azure.com/.default", "api://my-app/.default"} {
		if err := ValidateScope(scope); err != nil {
			t.Errorf("ValidateScope(%q) = %v, want nil", scope, err)
		}
	}
	for _, scope := range []string{
		"",
		"https://cognitiveservices.azure.com",
		"https://cognitiveservices.azure.com/.default https://graph.microsoft.com/.default",
		"/.default",
		"https://cognitiveservices.azure.com/.default/extra",
	} {
		if err := ValidateScope(scope); err == nil {
			t.Errorf("ValidateScope(%q) = nil, want an error", scope)
		}
	}
}

// Get checks the identity before building a credential, so a bad client ID never reaches the SDK.
func TestGetRejectsAnInvalidIdentity(t *testing.T) {
	useFakeAzure(t, answer(http.StatusOK, imdsToken("tok-1", "https://ai.azure.com")))
	for _, id := range []Identity{
		{Type: "client-secret"},
		{Type: ManagedIdentity, ClientID: "my-app"},
		{Type: ContainerApps, TenantID: testTenantID},
	} {
		if cred, err := Get(id); err == nil {
			t.Errorf("Get(%+v) = %v, want an error", id, cred)
		}
	}
}

// Without the variables the AKS Workload Identity webhook sets, the SDK can't build the credential,
// and the error says what is missing.
func TestWorkloadIdentityNeedsItsEnvironment(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	unsetEnv(t, "AZURE_FEDERATED_TOKEN_FILE")
	useFakeAzure(t, answer(http.StatusOK, entraToken("tok")))

	_, err := Get(Identity{Type: WorkloadIdentity})
	if err == nil || !strings.Contains(err.Error(), "workload identity is not available") || !strings.Contains(err.Error(), "AZURE_FEDERATED_TOKEN_FILE") {
		t.Fatalf("Get() = %v, want workload identity not available, naming AZURE_FEDERATED_TOKEN_FILE", err)
	}
}

// Without its identity endpoint the SDK would fall back to IMDS, which Container Apps doesn't have,
// and every token request would wait out its timeout; Get refuses instead.
func TestContainerAppsNeedsItsEndpoint(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
		// managed-identity takes whatever endpoint the environment offers, IMDS included.
		managedIdentityBuilds bool
	}{
		{name: "no managed identity on the app", want: "IDENTITY_ENDPOINT and IDENTITY_HEADER are not set", managedIdentityBuilds: true},
		{name: "no IDENTITY_HEADER", env: map[string]string{"IDENTITY_ENDPOINT": testIdentityEndpoint}, want: "IDENTITY_HEADER is not set", managedIdentityBuilds: true},
		{name: "no IDENTITY_ENDPOINT", env: map[string]string{"IDENTITY_HEADER": testIdentityHeader}, want: "IDENTITY_ENDPOINT is not set", managedIdentityBuilds: true},
		{
			name: "a Service Fabric endpoint",
			env:  map[string]string{"IDENTITY_ENDPOINT": testIdentityEndpoint, "IDENTITY_HEADER": testIdentityHeader, "IDENTITY_SERVER_THUMBPRINT": "0123456789ABCDEF"},
			want: "IDENTITY_SERVER_THUMBPRINT is set",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFakeAzure(t, answer(http.StatusOK, imdsToken("tok-1", "https://ai.azure.com")))
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			_, err := Get(Identity{Type: ContainerApps})
			if err == nil || !strings.Contains(err.Error(), "Container Apps managed identity is not available") || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Get() = %v, want Container Apps managed identity not available: %s", err, tt.want)
			}
			if tt.managedIdentityBuilds {
				if _, err := Get(Identity{Type: ManagedIdentity}); err != nil {
					t.Fatalf("managed-identity: %v", err)
				}
			}
		})
	}
}

// Policies name the token service in their failure logs.
func TestServiceNamesTheTokenService(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	useFakeAzure(t, answer(http.StatusOK, entraToken("tok")))
	setContainerAppsEnv(t)

	for typ, want := range map[string]string{
		WorkloadIdentity: "Microsoft Entra ID",
		ManagedIdentity:  "the managed identity endpoint",
		ContainerApps:    "the Container Apps identity endpoint",
	} {
		if got := mustGet(t, Identity{Type: typ}).Service(); got != want {
			t.Errorf("%s: Service() = %q, want %q", typ, got, want)
		}
	}
}
