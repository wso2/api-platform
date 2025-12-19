package semanticcache

// EmbeddingProvider defines the interface for services that provide text embedding
type EmbeddingProvider interface {
	Init(config EmbeddingProviderConfig) error
	GetType() string
	GetEmbedding(input string) ([]float32, error)
	GetEmbeddings(inputs []string) ([][]float32, error)
}

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
	EmbeddingDimention  string
	DistanceMetric      string
	Threshold           string
	DBHost              string
	DBPort              int
	Username            string
	Password            string
	DatabaseName        string
	TTL                 string
}

// EmbeddingProviderConfig defines the properties required for initializing an embedding provider
type EmbeddingProviderConfig struct {
	AuthHeaderName    string
	EmbeddingProvider string
	EmbeddingEndpoint string
	APIKey            string
	EmbeddingModel    string
	TimeOut           string
}
