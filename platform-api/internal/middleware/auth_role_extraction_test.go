package middleware

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// TestExtractClaimByPath verifies dot-notation path traversal for all IDP layouts.
func TestExtractClaimByPath(t *testing.T) {
	tests := []struct {
		name   string
		claims jwt.MapClaims
		path   string
		want   []string
	}{
		{
			name:   "flat array - Asgardeo / Entra ID style",
			claims: jwt.MapClaims{"roles": []interface{}{"admin", "viewer"}},
			path:   "roles",
			want:   []string{"admin", "viewer"},
		},
		{
			name:   "flat single-element array",
			claims: jwt.MapClaims{"roles": []interface{}{"admin"}},
			path:   "roles",
			want:   []string{"admin"},
		},
		{
			name:   "flat string - space-separated",
			claims: jwt.MapClaims{"roles": "admin viewer"},
			path:   "roles",
			want:   []string{"admin", "viewer"},
		},
		{
			name:   "flat string - single role",
			claims: jwt.MapClaims{"roles": "admin"},
			path:   "roles",
			want:   []string{"admin"},
		},
		{
			name: "nested two levels - Keycloak realm_access.roles",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"developer", "viewer"},
				},
			},
			path: "realm_access.roles",
			want: []string{"developer", "viewer"},
		},
		{
			name: "nested three levels - Keycloak resource_access.<client>.roles",
			claims: jwt.MapClaims{
				"resource_access": map[string]interface{}{
					"my-client": map[string]interface{}{
						"roles": []interface{}{"platform-admin", "platform-developer"},
					},
				},
			},
			path: "resource_access.my-client.roles",
			want: []string{"platform-admin", "platform-developer"},
		},
		{
			name: "nested three levels - single role in array",
			claims: jwt.MapClaims{
				"resource_access": map[string]interface{}{
					"my-client": map[string]interface{}{
						"roles": []interface{}{"platform-admin"},
					},
				},
			},
			path: "resource_access.my-client.roles",
			want: []string{"platform-admin"},
		},
		{
			name:   "missing claim returns nil",
			claims: jwt.MapClaims{"other": "value"},
			path:   "roles",
			want:   nil,
		},
		{
			name: "missing nested key returns nil",
			claims: jwt.MapClaims{
				"resource_access": map[string]interface{}{
					"other-client": map[string]interface{}{
						"roles": []interface{}{"admin"},
					},
				},
			},
			path: "resource_access.my-client.roles",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractClaimByPath(tc.claims, tc.path)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestResolvePlatformRoles verifies that multiple IDP roles produce a union of scopes
// with no duplicates, and that nil map is a passthrough.
func TestResolvePlatformRoles(t *testing.T) {
	roleScopeMap := map[string][]string{
		"admin":     {"ap:project:create", "ap:project:manage", "ap:gateway:manage"},
		"developer": {"ap:project:create", "ap:rest_api:create"},
		"viewer":    {"ap:project:read"},
	}

	tests := []struct {
		name         string
		claims       jwt.MapClaims
		claimPath    string
		roleScopeMap map[string][]string
		want         []string
	}{
		{
			name: "single role maps to multiple scopes",
			claims: jwt.MapClaims{
				"roles": []interface{}{"admin"},
			},
			claimPath:    "roles",
			roleScopeMap: roleScopeMap,
			want:         []string{"ap:project:create", "ap:project:manage", "ap:gateway:manage"},
		},
		{
			name: "two roles produce union without duplicates",
			claims: jwt.MapClaims{
				"roles": []interface{}{"admin", "developer"},
			},
			claimPath:    "roles",
			roleScopeMap: roleScopeMap,
			// admin contributes 3, developer adds ap:rest_api:create (ap:project:create deduped)
			want: []string{"ap:project:create", "ap:project:manage", "ap:gateway:manage", "ap:rest_api:create"},
		},
		{
			name: "unmapped role grants no scopes",
			claims: jwt.MapClaims{
				"roles": []interface{}{"unknown-role"},
			},
			claimPath:    "roles",
			roleScopeMap: roleScopeMap,
			want:         nil,
		},
		{
			name: "nil map is passthrough - returns raw role names",
			claims: jwt.MapClaims{
				"roles": []interface{}{"admin", "viewer"},
			},
			claimPath:    "roles",
			roleScopeMap: nil,
			want:         []string{"admin", "viewer"},
		},
		{
			name: "keycloak resource_access path with role map",
			claims: jwt.MapClaims{
				"resource_access": map[string]interface{}{
					"platform": map[string]interface{}{
						"roles": []interface{}{"viewer"},
					},
				},
			},
			claimPath:    "resource_access.platform.roles",
			roleScopeMap: roleScopeMap,
			want:         []string{"ap:project:read"},
		},
		{
			name: "empty claimPath returns nil",
			claims: jwt.MapClaims{
				"roles": []interface{}{"admin"},
			},
			claimPath:    "",
			roleScopeMap: roleScopeMap,
			want:         nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolvePlatformRoles(tc.claims, tc.claimPath, tc.roleScopeMap)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
