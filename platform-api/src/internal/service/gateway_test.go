/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package service

import (
	"testing"
)

// TestValidateGatewayInput tests input validation logic
func TestValidateGatewayInput(t *testing.T) {
	service := &GatewayService{}

	tests := []struct {
		name        string
		orgID       string
		gatewayName string
		displayName string
		vhost       string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid input",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod-gateway-01",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     false,
		},
		{
			name:        "empty organization ID",
			orgID:       "",
			gatewayName: "prod-gateway-01",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "organization ID is required",
		},
		{
			name:        "invalid organization ID format",
			orgID:       "not-a-uuid",
			gatewayName: "prod-gateway-01",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "invalid organization ID format",
		},
		{
			name:        "empty gateway name",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "gateway name is required",
		},
		{
			name:        "gateway name too short",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "ab",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "at least 3 characters",
		},
		{
			name:        "gateway name too long",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "this-is-a-very-long-gateway-name-that-exceeds-the-maximum-length-of-64-characters",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "must not exceed 64 characters",
		},
		{
			name:        "gateway name with uppercase",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "Prod-Gateway-01",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "lowercase letters, numbers, and hyphens",
		},
		{
			name:        "gateway name with special characters",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod_gateway_01",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "lowercase letters, numbers, and hyphens",
		},
		{
			name:        "gateway name with leading hyphen",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "-prod-gateway-01",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "cannot start or end with a hyphen",
		},
		{
			name:        "gateway name with trailing hyphen",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod-gateway-01-",
			displayName: "Production Gateway 01",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "cannot start or end with a hyphen",
		},
		{
			name:        "empty display name",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod-gateway-01",
			displayName: "",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "display name is required",
		},
		{
			name:        "display name too long",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod-gateway-01",
			displayName: "This is a very long display name that exceeds the maximum allowed length of 128 characters which should trigger a validation error in the system",
			vhost:       "api.example.com",
			wantErr:     true,
			errContains: "must not exceed 128 characters",
		},
		{
			name:        "empty display name",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod-gateway-01",
			displayName: "Production Gateway 01",
			vhost:       "",
			wantErr:     true,
			errContains: "vhost is required",
		},
		{
			name:        "display name with spaces (valid)",
			orgID:       "123e4567-e89b-12d3-a456-426614174000",
			gatewayName: "prod-gateway-01",
			displayName: "Production Gateway 01 - Main",
			vhost:       "api.example.com",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateGatewayInput(tt.orgID, tt.gatewayName, tt.displayName, tt.vhost)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGatewayInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("validateGatewayInput() error = %v, should contain %v", err, tt.errContains)
				}
			}
		})
	}
}

// TestGenerateToken tests token generation
func TestGenerateToken(t *testing.T) {
	// Generate multiple tokens to verify randomness
	token1, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	token2, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	// Tokens should not be empty
	if token1 == "" {
		t.Error("generateToken() returned empty token")
	}
	if token2 == "" {
		t.Error("generateToken() returned empty token")
	}

	// Tokens should be different (cryptographically random)
	if token1 == token2 {
		t.Error("generateToken() generated identical tokens (not random)")
	}

	// Token length should be reasonable (32 bytes base64-encoded is ~43 characters)
	if len(token1) < 40 {
		t.Errorf("generateToken() token too short: %d characters", len(token1))
	}
}

// TestGenerateSalt tests salt generation
func TestGenerateSalt(t *testing.T) {
	// Generate multiple salts to verify randomness
	salt1, err := generateSalt()
	if err != nil {
		t.Fatalf("generateSalt() error = %v", err)
	}

	salt2, err := generateSalt()
	if err != nil {
		t.Fatalf("generateSalt() error = %v", err)
	}

	// Salts should be 32 bytes
	if len(salt1) != 32 {
		t.Errorf("generateSalt() salt1 length = %d, want 32", len(salt1))
	}
	if len(salt2) != 32 {
		t.Errorf("generateSalt() salt2 length = %d, want 32", len(salt2))
	}

	// Salts should be different
	if string(salt1) == string(salt2) {
		t.Error("generateSalt() generated identical salts (not random)")
	}
}

// TestHashToken tests token hashing
func TestHashToken(t *testing.T) {
	token := "test-token-12345"
	salt := []byte("test-salt-32-bytes-for-hashing!!")

	// Generate hash
	hash1 := hashToken(token, salt)

	// Hash should not be empty
	if hash1 == "" {
		t.Error("hashToken() returned empty hash")
	}

	// Hash should be deterministic (same input = same output)
	hash2 := hashToken(token, salt)
	if hash1 != hash2 {
		t.Error("hashToken() not deterministic")
	}

	// Different token should produce different hash
	differentToken := "different-token-12345"
	hash3 := hashToken(differentToken, salt)
	if hash1 == hash3 {
		t.Error("hashToken() same hash for different tokens")
	}

	// Different salt should produce different hash
	differentSalt := []byte("different-salt-32-bytes-hashing!")
	hash4 := hashToken(token, differentSalt)
	if hash1 == hash4 {
		t.Error("hashToken() same hash for different salts")
	}

	// Hash should be hex-encoded SHA-256 (64 characters)
	if len(hash1) != 64 {
		t.Errorf("hashToken() hash length = %d, want 64 (SHA-256 hex)", len(hash1))
	}
}

// TestVerifyToken tests token verification
func TestVerifyToken(t *testing.T) {
	token := "test-token-12345"
	salt := []byte("test-salt-32-bytes-for-hashing!!")
	hash := hashToken(token, salt)
	saltHex := "746573742d73616c742d33322d62797465732d666f722d68617368696e672121" // hex encoding

	tests := []struct {
		name       string
		plainToken string
		storedHash string
		storedSalt string
		wantValid  bool
	}{
		{
			name:       "valid token",
			plainToken: token,
			storedHash: hash,
			storedSalt: saltHex,
			wantValid:  true,
		},
		{
			name:       "wrong token",
			plainToken: "wrong-token-12345",
			storedHash: hash,
			storedSalt: saltHex,
			wantValid:  false,
		},
		{
			name:       "empty token",
			plainToken: "",
			storedHash: hash,
			storedSalt: saltHex,
			wantValid:  false,
		},
		{
			name:       "invalid hash hex",
			plainToken: token,
			storedHash: "not-hex",
			storedSalt: saltHex,
			wantValid:  false,
		},
		{
			name:       "invalid salt hex",
			plainToken: token,
			storedHash: hash,
			storedSalt: "not-hex",
			wantValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := verifyToken(tt.plainToken, tt.storedHash, tt.storedSalt)
			if valid != tt.wantValid {
				t.Errorf("verifyToken() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

// TestTokenHashingRoundTrip tests full token hashing and verification cycle
func TestTokenHashingRoundTrip(t *testing.T) {
	// Generate token and salt
	token, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	salt, err := generateSalt()
	if err != nil {
		t.Fatalf("generateSalt() error = %v", err)
	}

	// Hash token
	hash := hashToken(token, salt)

	// Convert salt to hex for storage simulation
	saltHex := ""
	for _, b := range salt {
		saltHex += string("0123456789abcdef"[b>>4])
		saltHex += string("0123456789abcdef"[b&0x0f])
	}

	// Verify token
	if !verifyToken(token, hash, saltHex) {
		t.Error("verifyToken() failed for valid token in round-trip test")
	}

	// Generate different token and verify it fails
	differentToken, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	if verifyToken(differentToken, hash, saltHex) {
		t.Error("verifyToken() succeeded for invalid token")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
