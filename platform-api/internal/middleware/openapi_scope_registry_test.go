package middleware

import (
	"os"
	"testing"
)

const testSpec = `
openapi: 3.1.1
servers:
  - url: https://localhost:9243/api/v0.9
paths:
  /projects:
    post:
      operationId: CreateProject
      security:
        - OAuth2Security:
            - ap:project:create
            - ap:project:manage
  /projects/{projectId}:
    get:
      operationId: GetProject
      security:
        - OAuth2Security:
            - ap:project:read
            - ap:project:manage
  /organizations:
    post:
      operationId: RegisterOrganization
      security: []
components:
  securitySchemes:
    OAuth2Security:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://localhost:9243/oauth2/token
          scopes:
            ap:project:create: Create projects
            ap:project:read: Read projects
            ap:project:manage: Full access to projects
`

func TestLoadScopeRegistry(t *testing.T) {
	path := t.TempDir() + "/openapi.yaml"
	if err := os.WriteFile(path, []byte(testSpec), 0644); err != nil {
		t.Fatal(err)
	}
	reg, err := LoadScopeRegistry(path)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		method     string
		path       string
		wantFound  bool
		wantScopes []string
	}{
		{"POST", "/api/v0.9/projects", true, []string{"ap:project:create", "ap:project:manage"}},
		{"GET", "/api/v0.9/projects/{projectId}", true, []string{"ap:project:read", "ap:project:manage"}},
		{"POST", "/api/v0.9/organizations", false, nil},
	}
	for _, tc := range tests {
		scopes, found := reg.Lookup(tc.method, tc.path)
		if found != tc.wantFound {
			t.Errorf("%s %s: found=%v want=%v", tc.method, tc.path, found, tc.wantFound)
			continue
		}
		if tc.wantFound {
			if len(scopes) != len(tc.wantScopes) {
				t.Errorf("%s %s: got scopes %v, want %v", tc.method, tc.path, scopes, tc.wantScopes)
			}
			for i, s := range scopes {
				if s != tc.wantScopes[i] {
					t.Errorf("%s %s: scope[%d]=%q want %q", tc.method, tc.path, i, s, tc.wantScopes[i])
				}
			}
		}
	}
}

// TestLoadScopeRegistry_TolerantOfPathItemMetadata covers a path item that
// carries fields other than operations. "parameters" is a sequence, and
// decoding it as an operation used to fail the whole document — taking every
// scope in the spec with it, so nothing in that spec was ever enforced.
func TestLoadScopeRegistry_TolerantOfPathItemMetadata(t *testing.T) {
	const spec = `
openapi: 3.0.3
paths:
  /api/v0.9/websub-apis/{apiId}:
    summary: A WebSub API
    parameters:
      - name: apiId
        in: path
        required: true
        schema:
          type: string
    delete:
      security:
        - OAuth2Security:
            - ap:websub_api:manage
`
	registry, err := LoadScopeRegistryFromBytes([]byte(spec))
	if err != nil {
		t.Fatalf("LoadScopeRegistryFromBytes: %v", err)
	}

	scopes, found := registry.Lookup("DELETE", "/api/v0.9/websub-apis/{apiId}")
	if !found {
		t.Fatal("operation alongside path-item metadata was not registered")
	}
	if len(scopes) != 1 || scopes[0] != "ap:websub_api:manage" {
		t.Errorf("scopes = %v, want [ap:websub_api:manage]", scopes)
	}

	// "parameters" and "summary" are metadata, never operations.
	if _, found := registry.Lookup("PARAMETERS", "/api/v0.9/websub-apis/{apiId}"); found {
		t.Error("path-item metadata was registered as an operation")
	}
}

// TestScopeRegistryLookup_IgnoresPathParameterNames asserts a spec documenting
// one parameter name and a route registering another still resolve to the same
// entry — the names are not part of the route's identity.
func TestScopeRegistryLookup_IgnoresPathParameterNames(t *testing.T) {
	const spec = `
openapi: 3.0.3
paths:
  /api/v0.9/websub-apis/{apiId}/deployments/{deploymentId}:
    get:
      security:
        - OAuth2Security:
            - ap:websub_api:read
`
	registry, err := LoadScopeRegistryFromBytes([]byte(spec))
	if err != nil {
		t.Fatalf("LoadScopeRegistryFromBytes: %v", err)
	}

	for _, path := range []string{
		"/api/v0.9/websub-apis/{apiId}/deployments/{deploymentId}",
		"/api/v0.9/websub-apis/{webSubApiId}/deployments/{deploymentId}",
		"/api/v0.9/websub-apis/{id}/deployments/{depId}",
	} {
		if _, found := registry.Lookup("GET", path); !found {
			t.Errorf("Lookup(GET, %q) did not find the declared operation", path)
		}
	}

	// A structurally different path must still miss.
	if _, found := registry.Lookup("GET", "/api/v0.9/websub-apis/{apiId}/deployments"); found {
		t.Error("a structurally different path matched")
	}
}
