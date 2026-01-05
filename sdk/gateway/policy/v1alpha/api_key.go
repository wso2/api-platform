package policyv1alpha

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

type APIKey struct {
	// ID of the API Key
	ID string `json:"id" yaml:"id"`
	// Name of the API key
	Name string `json:"name" yaml:"name"`
	// ApiKey API key with apip_ prefix
	APIKey string `json:"api_key" yaml:"api_key"`
	// APIId Unique identifier of the API that the key is associated with
	APIId string `json:"apiId" yaml:"apiId"`
	// Operations List of API operations the key will have access to
	Operations string `json:"operations" yaml:"operations"`
	// Status of the API key
	Status APIKeyStatus `json:"status" yaml:"status"`
	// CreatedAt Timestamp when the API key was generated
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// CreatedBy User who created the API key
	CreatedBy string `json:"created_by" yaml:"created_by"`
	// UpdatedAt Timestamp when the API key was last updated
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	// ExpiresAt Expiration timestamp (null if no expiration)
	ExpiresAt *time.Time `json:"expires_at" yaml:"expires_at"`
}

// APIKeyStatus Status of the API key
type APIKeyStatus string

// Defines values for APIKeyStatus.
const (
	Active  APIKeyStatus = "active"
	Expired APIKeyStatus = "expired"
	Revoked APIKeyStatus = "revoked"
)

// Common storage errors - implementation agnostic
var (
	// ErrNotFound is returned when an API key is not found
	ErrNotFound = errors.New("API key not found")

	// ErrConflict is returned when an API Key with the same name/version or key value already exists
	ErrConflict = errors.New("API key already exists")
)

// Singleton instance
var (
	instance *APIkeyStore
	once     sync.Once
)

// APIkeyStore holds all API keys in memory for fast access
type APIkeyStore struct {
	mu sync.RWMutex // Protects concurrent access
	// API Keys storage
	apiKeysByAPI map[string][]*APIKey // Key: "API ID" → Value: slice of APIKeys
}

// NewAPIkeyStore creates a new in-memory API key store
func NewAPIkeyStore() *APIkeyStore {
	return &APIkeyStore{
		apiKeysByAPI: make(map[string][]*APIKey),
	}
}

// GetAPIkeyStoreInstance provides a shared instance of APIkeyStore
func GetAPIkeyStoreInstance() *APIkeyStore {
	once.Do(func() {
		instance = NewAPIkeyStore()
	})
	return instance
}

// StoreAPIKey stores an API key in the in-memory cache
func (aks *APIkeyStore) StoreAPIKey(apiId string, apiKey *APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("API key cannot be nil")
	}

	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Check if an API key with the same apiId and name already exists
	existingKeys, apiIdExists := aks.apiKeysByAPI[apiId]
	var existingKeyIndex = -1

	if apiIdExists {
		for i, existingKey := range existingKeys {
			if existingKey.Name == apiKey.Name {
				existingKeyIndex = i
				break
			}
		}
	}

	// Note: We no longer check for API key value collisions because:
	// 1. API key values are now hashed, making direct comparison impossible
	// 2. API keys use 256-bit cryptographic random generation, making collisions highly unlikely
	// 3. Any extremely rare hash collisions will be handled at the database level with constraints

	if existingKeyIndex >= 0 {
		// Update the existing entry in apiKeysByAPI
		aks.apiKeysByAPI[apiId][existingKeyIndex] = apiKey
	} else {
		// Insert new API key
		aks.apiKeysByAPI[apiId] = append(aks.apiKeysByAPI[apiId], apiKey)
	}

	return nil
}

// ValidateAPIKey validates the provided API key against the internal APIkey store
func (aks *APIkeyStore) ValidateAPIKey(apiId, apiOperation, operationMethod, providedAPIKey string) (bool, error) {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Get API keys for the apiId
	apiKeys, exists := aks.apiKeysByAPI[apiId]
	if !exists {
		return false, ErrNotFound
	}

	// Find the API key that matches the provided plain text key (by comparing against hashed values)
	var targetAPIKey *APIKey

	for _, ak := range apiKeys {
		// Compare provided plain text key with stored hashed key using Argon2id
		if strings.HasPrefix(ak.APIKey, "$argon2id$") {
			err := compareArgon2id(providedAPIKey, ak.APIKey)
			if err == nil {
				// Hash matches - this is our target API key
				targetAPIKey = ak
				break
			}
		}
	}

	if targetAPIKey == nil {
		return false, ErrNotFound
	}

	// Check if the API key is active
	if targetAPIKey.Status != Active {
		return false, nil
	}

	// Check if the API key has expired
	if targetAPIKey.Status == Expired || targetAPIKey.ExpiresAt != nil && time.Now().After(*targetAPIKey.ExpiresAt) {
		targetAPIKey.Status = Expired
		return false, nil
	}

	// Check if the API key has access to the requested operation
	// Operations is a JSON string array of allowed operations in format "METHOD path"
	// Example: ["GET /{country_code}/{city}", "POST /data"], ["*"] for allow all operations
	var operations []string
	if err := json.Unmarshal([]byte(targetAPIKey.Operations), &operations); err != nil {
		return false, fmt.Errorf("invalid operations format: %w", err)
	}

	// Check if wildcard is present
	for _, op := range operations {
		if strings.TrimSpace(op) == "*" {
			return true, nil
		}
	}

	// Check if the requested operation is in the allowed operations list
	requestedOperation := fmt.Sprintf("%s %s", operationMethod, apiOperation)
	for _, op := range operations {
		if strings.TrimSpace(op) == requestedOperation {
			return true, nil
		}
	}

	// Operation not found in allowed list
	return false, nil
}

// RevokeAPIKey revokes a specific API key by plain text API key value
func (aks *APIkeyStore) RevokeAPIKey(apiId, plainAPIKeyValue string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Get API keys for the apiId
	apiKeys, exists := aks.apiKeysByAPI[apiId]
	if !exists {
		// If the API doesn't exist in our store, we treat revocation as successful
		// since the key is effectively "not active" anyway
		return nil
	}

	// Find the API key with the matching hashed key value
	var targetAPIKey *APIKey
	var targetIndex = -1

	for i, apiKey := range apiKeys {
		// Compare plain text key with stored hashed key using Argon2id
		err := compareArgon2id(plainAPIKeyValue, apiKey.APIKey)
		if err == nil {
			targetAPIKey = apiKey
			targetIndex = i
			break
		}
	}

	// If the API key doesn't exist, treat revocation as successful (idempotent operation)
	if targetAPIKey == nil {
		return nil
	}

	// Set status to revoked
	targetAPIKey.Status = Revoked

	// Remove from apiKeysByAPI slice
	aks.apiKeysByAPI[apiId] = append(apiKeys[:targetIndex], apiKeys[targetIndex+1:]...)

	// Clean up empty slices
	if len(aks.apiKeysByAPI[apiId]) == 0 {
		delete(aks.apiKeysByAPI, apiId)
	}

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for a specific API
func (aks *APIkeyStore) RemoveAPIKeysByAPI(apiId string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	_, exists := aks.apiKeysByAPI[apiId]
	if !exists {
		return nil // No keys to remove
	}

	// Remove from API-specific map
	delete(aks.apiKeysByAPI, apiId)

	return nil
}

// ClearAll removes all API keys from the store
func (aks *APIkeyStore) ClearAll() error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	// Clear the API-specific keys map
	aks.apiKeysByAPI = make(map[string][]*APIKey)

	return nil
}

// compareArgon2id validates a plain text key against an Argon2id hash
func compareArgon2id(apiKey, hashedAPIKey string) error {
	if apiKey == "" {
		return fmt.Errorf("plain API key cannot be empty")
	}
	if hashedAPIKey == "" {
		return fmt.Errorf("hashed API key cannot be empty")
	}

	parts := strings.Split(hashedAPIKey, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return fmt.Errorf("invalid argon2id hash format")
	}

	// parts[2] -> v=19
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return err
	}
	if version != argon2.Version {
		return fmt.Errorf("unsupported argon2 version: %d", version)
	}

	// parts[3] -> m=65536,t=3,p=4
	var mem uint32
	var iters uint32
	var threads uint8
	var t, m, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return err
	}
	mem = m
	iters = t
	threads = uint8(p)

	// decode salt and hash (try RawStd then Std)
	salt, err := decodeBase64(parts[4])
	if err != nil {
		return err
	}
	hash, err := decodeBase64(parts[5])
	if err != nil {
		return err
	}

	derived := argon2.IDKey([]byte(apiKey), salt, iters, mem, threads, uint32(len(hash)))
	if subtle.ConstantTimeCompare(derived, hash) == 1 {
		return nil
	}
	return errors.New("API key mismatch")
}

// decodeBase64 decodes a base64 string, trying RawStdEncoding first, then StdEncoding
func decodeBase64(s string) ([]byte, error) {
	b, err := base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return b, nil
	}
	// try StdEncoding as a fallback
	return base64.StdEncoding.DecodeString(s)
}
