package policyv1alpha

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
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

// ParsedAPIKey represents a parsed API key with its components
type ParsedAPIKey struct {
	APIKey string
	ID     string
}

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
	apiKeysByAPI map[string]map[string]*APIKey // Key: "API ID" → Value: map[API key ID]*APIKey
}

// NewAPIkeyStore creates a new in-memory API key store
func NewAPIkeyStore() *APIkeyStore {
	return &APIkeyStore{
		apiKeysByAPI: make(map[string]map[string]*APIKey),
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
	var existingKeyID = ""

	if apiIdExists {
		for id, existingKey := range existingKeys {
			if existingKey.Name == apiKey.Name {
				existingKeyID = id
				break
			}
		}
	}

	if existingKeyID != "" {
		// Update the existing entry in apiKeysByAPI
		aks.apiKeysByAPI[apiId][existingKeyID] = apiKey
	} else {
		// Insert new API key
		// Check if API key ID already exists
		if _, exists := aks.apiKeysByAPI[apiId][apiKey.ID]; exists {
			return ErrConflict
		}

		// Initialize the map for this API ID if it doesn't exist
		if aks.apiKeysByAPI[apiId] == nil {
			aks.apiKeysByAPI[apiId] = make(map[string]*APIKey)
		}

		// Store by API key value
		aks.apiKeysByAPI[apiId][apiKey.ID] = apiKey
	}

	return nil
}

// ValidateAPIKey validates the provided API key against the internal APIkey store
func (aks *APIkeyStore) ValidateAPIKey(apiId, apiOperation, operationMethod, providedAPIKey string) (bool, error) {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	parsedAPIkey, ok := parseAPIKey(providedAPIKey)
	if !ok {
		return false, ErrNotFound
	}

	var targetAPIKey *APIKey

	apiKey, exists := aks.apiKeysByAPI[apiId][parsedAPIkey.ID]
	if !exists {
		return false, ErrNotFound
	}

	// Find the API key that matches the provided plain text key (by comparing against hashed values)
	if apiKey != nil {
		if compareAPIKeys(parsedAPIkey.APIKey, apiKey.APIKey) {
			targetAPIKey = apiKey
		}
	}

	if targetAPIKey == nil {
		return false, ErrNotFound
	}

	// Check if the API key belongs to the specified API
	if targetAPIKey.APIId != apiId {
		return false, nil
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
func (aks *APIkeyStore) RevokeAPIKey(apiId, providedAPIKey string) error {
	aks.mu.Lock()
	defer aks.mu.Unlock()

	parsedAPIkey, ok := parseAPIKey(providedAPIKey)
	if !ok {
		return nil
	}

	var matchedKey *APIKey

	apiKey, exists := aks.apiKeysByAPI[apiId][parsedAPIkey.ID]
	if !exists {
		return nil
	}

	// Find the API key that matches the provided plain text key
	if apiKey != nil {
		if compareAPIKeys(parsedAPIkey.APIKey, apiKey.APIKey) {
			matchedKey = apiKey
		}
	}

	// If the API key doesn't exist, treat revocation as successful (idempotent operation)
	if matchedKey == nil {
		return nil
	}

	// Set status to revoked
	matchedKey.Status = Revoked

	aks.removeFromAPIMapping(apiKey)

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
	aks.apiKeysByAPI = make(map[string]map[string]*APIKey)

	return nil
}

// compareAPIKeys compares API keys for external use
// Returns true if the plain API key matches the hash, false otherwise
// If hashing is disabled, performs plain text comparison
func compareAPIKeys(providedAPIKey, storedAPIKey string) bool {
	if providedAPIKey == "" || storedAPIKey == "" {
		return false
	}

	// Check if it's an SHA-256 hash (format: $sha256$<salt_hex>$<hash_hex>)
	if strings.HasPrefix(storedAPIKey, "$sha256$") {
		return compareSHA256Hash(providedAPIKey, storedAPIKey)
	}

	// Check if it's a bcrypt hash (starts with $2a$, $2b$, or $2y$)
	if strings.HasPrefix(storedAPIKey, "$2a$") ||
		strings.HasPrefix(storedAPIKey, "$2b$") ||
		strings.HasPrefix(storedAPIKey, "$2y$") {
		return compareBcryptHash(providedAPIKey, storedAPIKey)
	}

	// Check if it's an Argon2id hash
	if strings.HasPrefix(storedAPIKey, "$argon2id$") {
		err := compareArgon2id(providedAPIKey, storedAPIKey)
		return err == nil
	}

	// If no hash format is detected and hashing is enabled, try plain text comparison as fallback
	// This handles migration scenarios where some keys might still be stored as plain text
	return subtle.ConstantTimeCompare([]byte(providedAPIKey), []byte(storedAPIKey)) == 1
}

// compareSHA256Hash validates an encoded SHA-256 hash and compares it to the provided password.
// Expected format: $sha256$<salt_hex>$<hash_hex>
// Returns true if the plain API key matches the hash, false otherwise
func compareSHA256Hash(apiKey, encoded string) bool {
	if apiKey == "" || encoded == "" {
		return false
	}

	// Parse the hash format: $sha256$<salt_hex>$<hash_hex>
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[1] != "sha256" {
		return false
	}

	// Decode salt and hash from hex
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}

	storedHash, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	// Compute hash of the provided key with the stored salt
	hasher := sha256.New()
	hasher.Write([]byte(apiKey))
	hasher.Write(salt)
	computedHash := hasher.Sum(nil)

	// Constant-time comparison
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

// compareBcryptHash validates an encoded bcrypt hash and compares it to the provided password.
// Returns true if the plain API key matches the hash, false otherwise
func compareBcryptHash(apiKey, encoded string) bool {
	if apiKey == "" || encoded == "" {
		return false
	}

	// Compare the provided key with the stored bcrypt hash
	err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(apiKey))
	return err == nil
}

// compareArgon2id parses an encoded Argon2id hash and compares it to the provided password.
// Expected format: $argon2id$v=19$m=65536,t=3,p=4$<salt_b64>$<hash_b64>
func compareArgon2id(apiKey, encoded string) error {
	parts := strings.Split(encoded, "$")
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

// parseAPIKey splits an API key value into its key and ID components
func parseAPIKey(value string) (ParsedAPIKey, bool) {
	idx := strings.LastIndex(value, ".")
	if idx <= 0 || idx == len(value)-1 {
		return ParsedAPIKey{}, false
	}

	apiKey := value[:idx]
	hexEncodedID := value[idx+1:]

	// Decode the hex encoded ID back to the raw ID
	decodedIDBytes, err := hex.DecodeString(hexEncodedID)
	if err != nil {
		// If decoding fails, return the hex value as-is for backward compatibility
		return ParsedAPIKey{
			APIKey: apiKey,
			ID:     hexEncodedID,
		}, true
	}

	return ParsedAPIKey{
		APIKey: apiKey,
		ID:     string(decodedIDBytes),
	}, true
}

// removeFromAPIMapping removes an API key from the API mapping
func (aks *APIkeyStore) removeFromAPIMapping(apiKey *APIKey) {
	apiKeys, apiIdExists := aks.apiKeysByAPI[apiKey.APIId]
	if apiIdExists {
		delete(apiKeys, apiKey.ID)
		// clean up empty maps
		if len(aks.apiKeysByAPI[apiKey.APIId]) == 0 {
			delete(aks.apiKeysByAPI, apiKey.APIId)
		}
	}
}
