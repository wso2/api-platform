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

package encryption

import (
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockEncryptionProvider is a test double for EncryptionProvider
type MockEncryptionProvider struct {
	name             string
	encryptErr       error
	decryptErr       error
	healthCheckErr   error
	encryptedPayload *EncryptedPayload
	decryptedData    []byte
}

func (m *MockEncryptionProvider) Name() string {
	return m.name
}

func (m *MockEncryptionProvider) Encrypt(plaintext []byte) (*EncryptedPayload, error) {
	if m.encryptErr != nil {
		return nil, m.encryptErr
	}
	if m.encryptedPayload != nil {
		return m.encryptedPayload, nil
	}
	return &EncryptedPayload{
		Provider:   m.name,
		KeyVersion: "test-key-v1",
		Ciphertext: append([]byte("encrypted:"), plaintext...),
	}, nil
}

func (m *MockEncryptionProvider) Decrypt(payload *EncryptedPayload) ([]byte, error) {
	if m.decryptErr != nil {
		return nil, m.decryptErr
	}
	if m.decryptedData != nil {
		return m.decryptedData, nil
	}
	if len(payload.Ciphertext) > 10 {
		return payload.Ciphertext[10:], nil
	}
	return payload.Ciphertext, nil
}

func (m *MockEncryptionProvider) HealthCheck() error {
	return m.healthCheckErr
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewProviderManager(t *testing.T) {
	tests := []struct {
		name        string
		providers   []EncryptionProvider
		wantErr     bool
		errContains string
	}{
		{
			name: "single healthy provider",
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "test-provider"},
			},
			wantErr: false,
		},
		{
			name: "multiple healthy providers",
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "provider-1"},
				&MockEncryptionProvider{name: "provider-2"},
			},
			wantErr: false,
		},
		{
			name:        "no providers",
			providers:   []EncryptionProvider{},
			wantErr:     true,
			errContains: "at least one encryption provider is required",
		},
		{
			name:        "nil providers slice",
			providers:   nil,
			wantErr:     true,
			errContains: "at least one encryption provider is required",
		},
		{
			name: "unhealthy provider",
			providers: []EncryptionProvider{
				&MockEncryptionProvider{
					name:           "unhealthy-provider",
					healthCheckErr: errors.New("health check failed"),
				},
			},
			wantErr:     true,
			errContains: "failed health check",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewProviderManager(tt.providers, testLogger())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, manager)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, manager)
			}
		})
	}
}

func TestProviderManager_Encrypt(t *testing.T) {
	tests := []struct {
		name        string
		plaintext   []byte
		provider    *MockEncryptionProvider
		wantErr     bool
		errContains string
	}{
		{
			name:      "successful encryption",
			plaintext: []byte("secret data"),
			provider:  &MockEncryptionProvider{name: "test-provider"},
			wantErr:   false,
		},
		{
			name:      "empty plaintext",
			plaintext: []byte{},
			provider:  &MockEncryptionProvider{name: "test-provider"},
			wantErr:   false,
		},
		{
			name:      "encryption failure",
			plaintext: []byte("secret data"),
			provider: &MockEncryptionProvider{
				name:       "failing-provider",
				encryptErr: errors.New("encryption failed"),
			},
			wantErr:     true,
			errContains: "encryption failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewProviderManager([]EncryptionProvider{tt.provider}, testLogger())
			require.NoError(t, err)

			payload, err := manager.Encrypt(tt.plaintext)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, payload)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, payload)
				assert.Equal(t, tt.provider.name, payload.Provider)
			}
		})
	}
}

func TestProviderManager_Decrypt(t *testing.T) {
	tests := []struct {
		name        string
		payload     *EncryptedPayload
		providers   []EncryptionProvider
		wantErr     bool
		errContains string
	}{
		{
			name: "successful decryption - matching provider",
			payload: &EncryptedPayload{
				Provider:   "provider-1",
				KeyVersion: "v1",
				Ciphertext: []byte("encrypted:secret data"),
			},
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "provider-1"},
			},
			wantErr: false,
		},
		{
			name: "decryption with multiple providers - second matches",
			payload: &EncryptedPayload{
				Provider:   "provider-2",
				KeyVersion: "v1",
				Ciphertext: []byte("encrypted:secret data"),
			},
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "provider-1"},
				&MockEncryptionProvider{name: "provider-2"},
			},
			wantErr: false,
		},
		{
			name:    "nil payload",
			payload: nil,
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "provider-1"},
			},
			wantErr:     true,
			errContains: "encrypted payload is nil",
		},
		{
			name: "no matching provider",
			payload: &EncryptedPayload{
				Provider:   "unknown-provider",
				KeyVersion: "v1",
				Ciphertext: []byte("encrypted:secret data"),
			},
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "provider-1"},
			},
			wantErr:     true,
			errContains: "unknown-provider",
		},
		{
			name: "decryption failure",
			payload: &EncryptedPayload{
				Provider:   "failing-provider",
				KeyVersion: "v1",
				Ciphertext: []byte("encrypted:secret data"),
			},
			providers: []EncryptionProvider{
				&MockEncryptionProvider{
					name:       "failing-provider",
					decryptErr: errors.New("decryption error"),
				},
			},
			wantErr:     true,
			errContains: "decryption failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewProviderManager(tt.providers, testLogger())
			require.NoError(t, err)

			plaintext, err := manager.Decrypt(tt.payload)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, plaintext)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, plaintext)
			}
		})
	}
}

func TestProviderManager_HealthCheck(t *testing.T) {
	tests := []struct {
		name      string
		providers []EncryptionProvider
		wantErr   bool
	}{
		{
			name: "all providers healthy",
			providers: []EncryptionProvider{
				&MockEncryptionProvider{name: "provider-1"},
				&MockEncryptionProvider{name: "provider-2"},
			},
			wantErr: false,
		},
		{
			name: "first provider unhealthy",
			providers: []EncryptionProvider{
				&MockEncryptionProvider{
					name:           "unhealthy-provider",
					healthCheckErr: errors.New("health check failed"),
				},
				&MockEncryptionProvider{name: "provider-2"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allHealthy := make([]EncryptionProvider, len(tt.providers))
			for i, p := range tt.providers {
				allHealthy[i] = &MockEncryptionProvider{name: p.Name()}
			}
			manager, err := NewProviderManager(allHealthy, testLogger())
			require.NoError(t, err)

			manager.providers = tt.providers

			err = manager.HealthCheck()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProviderManager_GetPrimaryProvider(t *testing.T) {
	provider1 := &MockEncryptionProvider{name: "provider-1"}
	provider2 := &MockEncryptionProvider{name: "provider-2"}

	manager, err := NewProviderManager([]EncryptionProvider{provider1, provider2}, testLogger())
	require.NoError(t, err)

	primary := manager.GetPrimaryProvider()
	assert.Equal(t, "provider-1", primary.Name())
}

func TestProviderManager_GetProviders(t *testing.T) {
	provider1 := &MockEncryptionProvider{name: "provider-1"}
	provider2 := &MockEncryptionProvider{name: "provider-2"}

	manager, err := NewProviderManager([]EncryptionProvider{provider1, provider2}, testLogger())
	require.NoError(t, err)

	providers := manager.GetProviders()
	assert.Len(t, providers, 2)
	assert.Equal(t, "provider-1", providers[0].Name())
	assert.Equal(t, "provider-2", providers[1].Name())
}

func TestMarshalPayload(t *testing.T) {
	tests := []struct {
		name     string
		payload  *EncryptedPayload
		expected string
	}{
		{
			name:     "nil payload",
			payload:  nil,
			expected: "",
		},
		{
			name: "valid payload",
			payload: &EncryptedPayload{
				Provider:   "aesgcm",
				KeyVersion: "key-v1",
				Ciphertext: []byte("test cipher"),
			},
			expected: "enc:aesgcm:v1:key-v1:dGVzdCBjaXBoZXI=",
		},
		{
			name: "empty ciphertext",
			payload: &EncryptedPayload{
				Provider:   "aesgcm",
				KeyVersion: "key-v1",
				Ciphertext: []byte{},
			},
			expected: "enc:aesgcm:v1:key-v1:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarshalPayload(tt.payload)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnmarshalPayload(t *testing.T) {
	tests := []struct {
		name        string
		stored      string
		wantErr     bool
		errContains string
		expected    *EncryptedPayload
	}{
		{
			name:    "valid payload",
			stored:  "enc:aesgcm:v1:key-v1:dGVzdCBjaXBoZXI=",
			wantErr: false,
			expected: &EncryptedPayload{
				Provider:   "aesgcm",
				KeyVersion: "key-v1",
				Ciphertext: []byte("test cipher"),
			},
		},
		{
			name:        "invalid format - too few parts",
			stored:      "enc:aesgcm:v1",
			wantErr:     true,
			errContains: "expected 5 parts",
		},
		{
			name:        "invalid prefix",
			stored:      "encrypted:aesgcm:v1:key-v1:dGVzdA==",
			wantErr:     true,
			errContains: "invalid payload prefix",
		},
		{
			name:        "unsupported version",
			stored:      "enc:aesgcm:v2:key-v1:dGVzdA==",
			wantErr:     true,
			errContains: "unsupported payload version",
		},
		{
			name:        "invalid base64",
			stored:      "enc:aesgcm:v1:key-v1:invalid!!!base64",
			wantErr:     true,
			errContains: "failed to decode ciphertext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalPayload(tt.stored)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.Provider, result.Provider)
				assert.Equal(t, tt.expected.KeyVersion, result.KeyVersion)
				assert.Equal(t, tt.expected.Ciphertext, result.Ciphertext)
			}
		})
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	original := &EncryptedPayload{
		Provider:   "aesgcm",
		KeyVersion: "production-key-v2",
		Ciphertext: []byte("some encrypted binary data \x00\x01\x02\xff"),
	}

	marshaled := MarshalPayload(original)
	unmarshaled, err := UnmarshalPayload(marshaled)

	require.NoError(t, err)
	assert.Equal(t, original.Provider, unmarshaled.Provider)
	assert.Equal(t, original.KeyVersion, unmarshaled.KeyVersion)
	assert.Equal(t, original.Ciphertext, unmarshaled.Ciphertext)
}
