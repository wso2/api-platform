package policyv1alpha

import "testing"

func TestAuthContext_ZeroValue(t *testing.T) {
	var ac AuthContext
	if ac.Authenticated {
		t.Error("zero-value AuthContext should have Authenticated=false")
	}
	if ac.AuthType != "" {
		t.Error("zero-value AuthContext should have empty AuthType")
	}
	if ac.PolicyName != "" {
		t.Error("zero-value AuthContext should have empty PolicyName")
	}
	if ac.Subject != "" {
		t.Error("zero-value AuthContext should have empty Subject")
	}
	if ac.Issuer != "" {
		t.Error("zero-value AuthContext should have empty Issuer")
	}
	if ac.Audience != nil {
		t.Error("zero-value AuthContext should have nil Audience")
	}
	if ac.Scopes != nil {
		t.Error("zero-value AuthContext should have nil Scopes")
	}
	if ac.CredentialID != "" {
		t.Error("zero-value AuthContext should have empty CredentialID")
	}
	if ac.Properties != nil {
		t.Error("zero-value AuthContext should have nil Properties")
	}
	if ac.Previous != nil {
		t.Error("zero-value AuthContext should have nil Previous")
	}
}

func TestAuthContext_ScopeLookup(t *testing.T) {
	ac := &AuthContext{
		Scopes: map[string]bool{
			"read":  true,
			"write": true,
		},
	}

	if !ac.Scopes["read"] {
		t.Error("expected Scopes[\"read\"] to be true")
	}
	if !ac.Scopes["write"] {
		t.Error("expected Scopes[\"write\"] to be true")
	}
	if ac.Scopes["admin"] {
		t.Error("expected Scopes[\"admin\"] to be false (not present)")
	}
}

func TestAuthContext_NilScopes(t *testing.T) {
	ac := &AuthContext{}
	// Accessing a nil map returns false, not a panic
	if ac.Scopes["read"] {
		t.Error("expected false for scope lookup on nil map")
	}
}

func TestAuthContext_Audience(t *testing.T) {
	ac := &AuthContext{
		Audience: []string{"api-v1", "api-v2"},
	}

	if len(ac.Audience) != 2 {
		t.Errorf("expected 2 audience entries, got %d", len(ac.Audience))
	}
	if ac.Audience[0] != "api-v1" {
		t.Errorf("expected first audience 'api-v1', got %q", ac.Audience[0])
	}
}

func TestAuthContext_NilAudience(t *testing.T) {
	ac := &AuthContext{}
	if ac.Audience != nil {
		t.Error("expected nil Audience")
	}
	if len(ac.Audience) != 0 {
		t.Error("expected len(nil) == 0")
	}
}

func TestAuthContext_PreviousChain(t *testing.T) {
	first := &AuthContext{
		Authenticated: true,
		AuthType:      "basic",
		PolicyName:    "basic-auth",
		Subject:       "alice",
	}
	second := &AuthContext{
		Authenticated: true,
		AuthType:      "jwt",
		PolicyName:    "jwt-auth",
		Subject:       "alice@example.com",
		Previous:      first,
	}

	if second.Previous == nil {
		t.Fatal("expected non-nil Previous")
	}
	if second.Previous.Subject != "alice" {
		t.Errorf("expected Previous.Subject='alice', got %q", second.Previous.Subject)
	}
	if second.Previous.Previous != nil {
		t.Error("expected Previous.Previous to be nil")
	}

	// Walk the chain
	count := 0
	for ac := second; ac != nil; ac = ac.Previous {
		count++
	}
	if count != 2 {
		t.Errorf("expected chain length 2, got %d", count)
	}
}

func TestAuthContext_Properties(t *testing.T) {
	ac := &AuthContext{
		Properties: map[string]string{
			"custom_claim": "custom_value",
			"tenant":       "acme",
		},
	}

	if ac.Properties["custom_claim"] != "custom_value" {
		t.Errorf("expected Properties[\"custom_claim\"]='custom_value', got %q", ac.Properties["custom_claim"])
	}
	if ac.Properties["tenant"] != "acme" {
		t.Errorf("expected Properties[\"tenant\"]='acme', got %q", ac.Properties["tenant"])
	}
	if _, ok := ac.Properties["missing"]; ok {
		t.Error("expected missing key to be absent")
	}
}
