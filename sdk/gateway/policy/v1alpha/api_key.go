package policyv1alpha

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type APIKey struct {
	// ID of the API Key
	ID string `json:"id" yaml:"id"`
	// Name of the API key
	Name string `json:"name" yaml:"name"`
	// ApiKey API key with apip_ prefix
	APIKey string `json:"api_key" yaml:"api_key"`
	// Handle Unique public identifier of the API that the key is associated with
	Handle string `json:"handle" yaml:"handle"`
	// Operations List of API operations the key will have access to
	Operations string `json:"operations" yaml:"operations"`
	// Status of the API key
	Status APIKeyStatus `json:"status" yaml:"status"`
	// CreatedAt Timestamp when the API key was generated
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	// UpdatedAt Timestamp when the API key was last updated
	UpdatedAt *time.Time `json:"updated_at" yaml:"updated_at"`
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
	apiKeys      map[string]*APIKey   // Key: API key value → Value: APIKey
	apiKeysByAPI map[string][]*APIKey // Key: "name:version" → Value: slice of APIKeys
}

// NewAPIkeyStore creates a new in-memory API key store
func NewAPIkeyStore() *APIkeyStore {
	return &APIkeyStore{
		apiKeys:      make(map[string]*APIKey),
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
func (aks *APIkeyStore) StoreAPIKey(name, version string, apiKey *APIKey) error {
	if apiKey == nil {
		return fmt.Errorf("API key cannot be nil")
	}

	aks.mu.Lock()
	defer aks.mu.Unlock()

	handleKey := compositeKey(name, version)

	// Check if an API key with the same handleKey and name already exists
	existingKeys, handleExists := aks.apiKeysByAPI[handleKey]
	var existingKeyIndex = -1
	var oldAPIKeyValue string

	if handleExists {
		for i, existingKey := range existingKeys {
			if existingKey.Name == apiKey.Name {
				existingKeyIndex = i
				oldAPIKeyValue = existingKey.APIKey
				break
			}
		}
	}

	// Check if the new API key value already exists (but with different handleKey/name)
	if _, keyExists := aks.apiKeys[apiKey.APIKey]; keyExists && oldAPIKeyValue != apiKey.APIKey {
		return ErrConflict
	}

	if existingKeyIndex >= 0 {
		// Update existing API key
		// Remove old API key value from apiKeys map if it's different
		if oldAPIKeyValue != apiKey.APIKey {
			delete(aks.apiKeys, oldAPIKeyValue)
		}

		// Update the existing entry in apiKeysByAPI
		aks.apiKeysByAPI[handleKey][existingKeyIndex] = apiKey

		// Store by new API key value
		aks.apiKeys[apiKey.APIKey] = apiKey
	} else {
		// Insert new API key
		// Check if API key value already exists
		if _, exists := aks.apiKeys[apiKey.APIKey]; exists {
			return ErrConflict
		}

		// Store by API key value
		aks.apiKeys[apiKey.APIKey] = apiKey

		// Store by API handleKey
		aks.apiKeysByAPI[handleKey] = append(aks.apiKeysByAPI[handleKey], apiKey)
	}

	return nil
}

// ValidateAPIKey validates the provided API key against the internal APIkey store
func (aks *APIkeyStore) ValidateAPIKey(apiName, apiVersion, apiOperation, operationMethod, apiKey string) (bool, error) {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	handleKey := compositeKey(apiName, apiVersion)

	// Get API keys for the handleKey
	apiKeys, exists := aks.apiKeysByAPI[handleKey]
	if !exists {
		return false, ErrNotFound
	}

	// Find the API key with the matching key value
	var targetAPIKey *APIKey

	for _, ak := range apiKeys {
		if ak.APIKey == apiKey {
			targetAPIKey = ak
			break
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

// RevokeAPIKey revokes a specific API key by API key value
func (aks *APIkeyStore) RevokeAPIKey(apiName, apiVersion, apiKeyValue string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	handleKey := compositeKey(apiName, apiVersion)

	// Get API keys for the handleKey
	apiKeys, exists := aks.apiKeysByAPI[handleKey]
	if !exists {
		// If the API doesn't exist in our store, we treat revocation as successful
		// since the key is effectively "not active" anyway
		return nil
	}

	// Find the API key with the matching key value
	var targetAPIKey *APIKey
	var targetIndex = -1

	for i, apiKey := range apiKeys {
		if apiKey.APIKey == apiKeyValue {
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

	// Remove from main apiKeys map
	delete(aks.apiKeys, apiKeyValue)

	// Remove from apiKeysByAPI slice
	aks.apiKeysByAPI[handleKey] = append(apiKeys[:targetIndex], apiKeys[targetIndex+1:]...)

	// Clean up empty slices
	if len(aks.apiKeysByAPI[handleKey]) == 0 {
		delete(aks.apiKeysByAPI, handleKey)
	}

	return nil
}

// RemoveAPIKeysByAPI removes all API keys for a specific API
func (aks *APIkeyStore) RemoveAPIKeysByAPI(name, version string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	handleKey := compositeKey(name, version)

	apiKeys, exists := aks.apiKeysByAPI[handleKey]
	if !exists {
		return nil // No keys to remove
	}

	// Remove from main map
	for _, key := range apiKeys {
		delete(aks.apiKeys, key.APIKey)
	}

	// Remove from API-specific map
	delete(aks.apiKeysByAPI, handleKey)

	return nil
}

// compositeKey creates a composite key from name and version
func compositeKey(name, version string) string {
	return fmt.Sprintf("%s:%s", name, version)
}
