package vectordbproviders

import "fmt"

const (
	VectorIndexPrefix = "api_platform_semantic_cache_" // VectorIndexPrefix is the prefix for vector index keys in the cache
	DefaultTTL        = 3600                           // DefaultTTL is the default time-to-live for cache entries in seconds (1 hour)
)

// VectorDBProvider defines the interface for vector database providers
type VectorDBProvider interface {
	Init(config VectorDBProviderConfig) error
	GetType() string
	CreateIndex() error
	Store(embeddings []float32, response CacheResponse, filter map[string]interface{}) error
	Retrieve(embeddings []float32, filter map[string]interface{}) (CacheResponse, error)
	Close() error
}

// VectorDBProviderConfig defines the properties required for initializing a vector DB provider
type VectorDBProviderConfig struct {
	VectorStoreProvider string
	EmbeddingDimension  string
	DistanceMetric      string
	Threshold           string
	DBHost              string
	DBPort              int
	Username            string
	Password            string
	DatabaseName        string
	TTL                 string
}

// ValidateVectorStoreConfigProps validates the properties of the vector store configuration.
func ValidateVectorStoreConfigProps(config VectorDBProviderConfig) error {
	if config.VectorStoreProvider != "REDIS" && config.VectorStoreProvider != "MILVUS" {
		return fmt.Errorf("invalid vector store provider found in the vector store configuration")
	}
	if config.EmbeddingDimension == "" {
		return fmt.Errorf("missing embedding dimension in the vector store configuration")
	}
	if config.Threshold == "" {
		return fmt.Errorf("missing threshold in the vector store configuration")
	}
	if config.DBHost == "" {
		return fmt.Errorf("missing database host in the vector store configuration")
	}
	if config.DBPort == 0 || config.DBPort < 0 {
		return fmt.Errorf("missing/invalid database port in the vector store configuration")
	}
	if config.Username == "" {
		return fmt.Errorf("missing DB username in the vector store configuration")
	}
	if config.Password == "" {
		return fmt.Errorf("missing DB password in the vector store configuration")
	}
	if config.DatabaseName == "" {
		return fmt.Errorf("missing database name in the vector store configuration")
	}
	return nil
}
