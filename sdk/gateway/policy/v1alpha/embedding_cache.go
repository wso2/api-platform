package policyv1alpha

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// EmbeddingEntry stores the tool name and its embedding vector
type EmbeddingEntry struct {
	Name      string
	Embedding []float32
}

// APIEmbeddingCache stores embeddings for a specific API
// Key: SHA-256 hash of tool description → Value: EmbeddingEntry
type APIEmbeddingCache map[string]*EmbeddingEntry

// EmbeddingCacheStore is a global singleton for storing embeddings per API
type EmbeddingCacheStore struct {
	mu    sync.RWMutex
	cache map[string]APIEmbeddingCache // Key: API ID → Value: APIEmbeddingCache
}

// Singleton instance for EmbeddingCacheStore
var (
	embeddingCacheInstance *EmbeddingCacheStore
	embeddingCacheOnce     sync.Once
)

// GetEmbeddingCacheStoreInstance returns the global singleton instance
func GetEmbeddingCacheStoreInstance() *EmbeddingCacheStore {
	embeddingCacheOnce.Do(func() {
		embeddingCacheInstance = &EmbeddingCacheStore{
			cache: make(map[string]APIEmbeddingCache),
		}
	})
	return embeddingCacheInstance
}

// HashDescription computes SHA-256 hash of the tool description
func HashDescription(description string) string {
	hash := sha256.Sum256([]byte(description))
	return hex.EncodeToString(hash[:])
}

// HasAPI checks if there is a cache entry for the given API ID
func (ecs *EmbeddingCacheStore) HasAPI(apiId string) bool {
	ecs.mu.RLock()
	defer ecs.mu.RUnlock()

	_, exists := ecs.cache[apiId]
	return exists
}

// GetAPICache returns the embedding cache for a specific API ID
// Returns nil if the API ID doesn't exist in the cache
func (ecs *EmbeddingCacheStore) GetAPICache(apiId string) APIEmbeddingCache {
	ecs.mu.RLock()
	defer ecs.mu.RUnlock()

	if apiCache, exists := ecs.cache[apiId]; exists {
		// Return a shallow copy to prevent data races
		copyCache := make(APIEmbeddingCache, len(apiCache))
		for k, v := range apiCache {
			copyCache[k] = v
		}
		return copyCache
	}
	return nil
}

// AddAPICache creates a new empty cache for the given API ID
// If a cache already exists for this API ID, it does nothing
func (ecs *EmbeddingCacheStore) AddAPICache(apiId string) {
	ecs.mu.Lock()
	defer ecs.mu.Unlock()

	if _, exists := ecs.cache[apiId]; !exists {
		ecs.cache[apiId] = make(APIEmbeddingCache)
	}
}

// GetEntry retrieves an embedding entry for a specific API and hash key
// Returns nil if not found
func (ecs *EmbeddingCacheStore) GetEntry(apiId, hashKey string) *EmbeddingEntry {
	ecs.mu.RLock()
	defer ecs.mu.RUnlock()

	if apiCache, exists := ecs.cache[apiId]; exists {
		if entry, found := apiCache[hashKey]; found {
			return entry
		}
	}
	return nil
}

// GetEntryByDescription retrieves an embedding entry by API ID and tool description
// Automatically hashes the description to find the entry
func (ecs *EmbeddingCacheStore) GetEntryByDescription(apiId, description string) *EmbeddingEntry {
	hashKey := HashDescription(description)
	return ecs.GetEntry(apiId, hashKey)
}

// AddEntry adds or updates an embedding entry for a specific API
// If an entry with the same name exists in this API's cache, it removes the old one first
// The hashKey should be SHA-256 hash of the tool description
func (ecs *EmbeddingCacheStore) AddEntry(apiId, hashKey, name string, embedding []float32) {
	ecs.mu.Lock()
	defer ecs.mu.Unlock()

	// Ensure API cache exists
	if _, exists := ecs.cache[apiId]; !exists {
		ecs.cache[apiId] = make(APIEmbeddingCache)
	}

	apiCache := ecs.cache[apiId]

	// Check if there's an existing entry with the same name and remove it
	for key, entry := range apiCache {
		if entry.Name == name {
			delete(apiCache, key)
			break
		}
	}

	// Add new entry
	apiCache[hashKey] = &EmbeddingEntry{
		Name:      name,
		Embedding: embedding,
	}
}

// AddEntryByDescription adds or updates an embedding entry using the description to generate the hash key
func (ecs *EmbeddingCacheStore) AddEntryByDescription(apiId, description, name string, embedding []float32) {
	hashKey := HashDescription(description)
	ecs.AddEntry(apiId, hashKey, name, embedding)
}

// RemoveAPI removes all cached embeddings for a specific API
func (ecs *EmbeddingCacheStore) RemoveAPI(apiId string) {
	ecs.mu.Lock()
	defer ecs.mu.Unlock()

	delete(ecs.cache, apiId)
}

// ClearAll removes all cached embeddings
func (ecs *EmbeddingCacheStore) ClearAll() {
	ecs.mu.Lock()
	defer ecs.mu.Unlock()

	ecs.cache = make(map[string]APIEmbeddingCache)
}

// GetCacheStats returns statistics about the cache
func (ecs *EmbeddingCacheStore) GetCacheStats() (apiCount int, totalEntries int) {
	ecs.mu.RLock()
	defer ecs.mu.RUnlock()

	apiCount = len(ecs.cache)
	for _, apiCache := range ecs.cache {
		totalEntries += len(apiCache)
	}
	return
}
