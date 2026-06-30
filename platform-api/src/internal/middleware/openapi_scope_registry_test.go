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
  /projects/{id}:
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
		{"GET", "/api/v0.9/projects/{id}", true, []string{"ap:project:read", "ap:project:manage"}},
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
