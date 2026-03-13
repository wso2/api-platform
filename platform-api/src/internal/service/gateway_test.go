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
	"platform-api/src/internal/constants"
	"testing"
)

// TestValidateGatewayInput tests input validation logic
func TestValidateGatewayInput(t *testing.T) {
	service := &GatewayService{}

	tests := []struct {
		name              string
		orgID             string
		gatewayName       string
		displayName       string
		vhost             string
		functionalityType string
		version           string
		wantErr           bool
		errContains       string
	}{
		{
			name:              "valid input",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           false,
		},
		{
			name:              "empty organization ID",
			orgID:             "",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "organization ID is required",
		},
		{
			name:              "invalid organization ID format",
			orgID:             "not-a-uuid",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "invalid organization ID format",
		},
		{
			name:              "empty gateway name",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "gateway name is required",
		},
		{
			name:              "gateway name too short",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "ab",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "at least 3 characters",
		},
		{
			name:              "gateway name too long",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "this-is-a-very-long-gateway-name-that-exceeds-the-maximum-length-of-64-characters",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "must not exceed 64 characters",
		},
		{
			name:              "gateway name with uppercase",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "Prod-Gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "lowercase letters, numbers, and hyphens",
		},
		{
			name:              "gateway name with special characters",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod_gateway_01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "lowercase letters, numbers, and hyphens",
		},
		{
			name:              "gateway name with leading hyphen",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "-prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "cannot start or end with a hyphen",
		},
		{
			name:              "gateway name with trailing hyphen",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01-",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "cannot start or end with a hyphen",
		},
		{
			name:              "empty display name",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "display name is required",
		},
		{
			name:              "display name too long",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "This is a very long display name that exceeds the maximum allowed length of 128 characters which should trigger a validation error in the system",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "must not exceed 128 characters",
		},
		{
			name:              "empty vhost",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "vhost is required",
		},
		{
			name:              "empty version",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "",
			wantErr:           true,
			errContains:       "version is required",
		},
		{
			name:              "display name with spaces (valid)",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01 - Main",
			vhost:             "api.example.com",
			functionalityType: constants.GatewayFunctionalityTypeRegular,
			version:           "1.0.0",
			wantErr:           false,
		},
		{
			name:              "empty functionality type",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: "",
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "functionality type is required",
		},
		{
			name:              "whitespace-only functionality type",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: "   ",
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "functionality type is required",
		},
		{
			name:              "invalid functionality type",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "prod-gateway-01",
			displayName:       "Production Gateway 01",
			vhost:             "api.example.com",
			functionalityType: "invalid-type",
			version:           "1.0.0",
			wantErr:           true,
			errContains:       "gateway type must be one of: regular, ai, event",
		},
		{
			name:              "valid functionality type - ai",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "ai-gateway-01",
			displayName:       "AI Gateway 01",
			vhost:             "ai.example.com",
			functionalityType: "ai",
			version:           "1.0.0",
			wantErr:           false,
		},
		{
			name:              "valid functionality type - event",
			orgID:             "123e4567-e89b-12d3-a456-426614174000",
			gatewayName:       "event-gateway-01",
			displayName:       "Event Gateway 01",
			vhost:             "events.example.com",
			functionalityType: "event",
			version:           "1.0.0",
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateGatewayInput(tt.orgID, tt.gatewayName, tt.displayName, tt.vhost, tt.functionalityType, tt.version)
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

// TestHashToken tests token hashing without salt
func TestHashToken(t *testing.T) {
	token := "test-token-12345"

	// Generate hash
	hash1 := hashToken(token)

	// Hash should not be empty
	if hash1 == "" {
		t.Error("hashToken() returned empty hash")
	}

	// Hash should be deterministic (same input = same output)
	hash2 := hashToken(token)
	if hash1 != hash2 {
		t.Error("hashToken() not deterministic")
	}

	// Different token should produce different hash
	differentToken := "different-token-12345"
	hash3 := hashToken(differentToken)
	if hash1 == hash3 {
		t.Error("hashToken() same hash for different tokens")
	}

	// Hash should be hex-encoded SHA-256 (64 characters)
	if len(hash1) != 64 {
		t.Errorf("hashToken() hash length = %d, want 64 (SHA-256 hex)", len(hash1))
	}
}

// TestHashTokenMatchesProduction tests that hashToken produces consistent results for token lookup
func TestHashTokenMatchesProduction(t *testing.T) {
	token := "test-token-12345"
	hash := hashToken(token)

	// Same token should produce same hash (used for DB lookup in VerifyToken)
	if hashToken(token) != hash {
		t.Error("hashToken() not deterministic")
	}

	// Wrong token should produce different hash
	if hashToken("wrong-token-12345") == hash {
		t.Error("hashToken() same hash for different tokens")
	}

	// Empty token should produce different hash
	if hashToken("") == hash {
		t.Error("hashToken() same hash for empty and non-empty token")
	}
}

// TestTokenHashingRoundTrip tests full token generation and hash lookup cycle
func TestTokenHashingRoundTrip(t *testing.T) {
	// Generate token
	token, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	// Hash token (simulates what RotateToken stores in DB)
	storedHash := hashToken(token)

	// Re-hash same token (simulates what VerifyToken computes for lookup)
	lookupHash := hashToken(token)

	if storedHash != lookupHash {
		t.Error("hashToken() round-trip failed: stored hash != lookup hash")
	}

	// Different token should not match
	differentToken, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	if hashToken(differentToken) == storedHash {
		t.Error("hashToken() different token produced same hash")
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
