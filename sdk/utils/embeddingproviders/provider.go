package embeddingproviders

import "fmt"

const (
	DefaultRequestTimeout = 30 // DefaultRequestTimeout is the default timeout for requests in seconds (30 seconds)
)

// EmbeddingProvider defines the interface for services that provide text embedding
type EmbeddingProvider interface {
	Init(config EmbeddingProviderConfig) error
	GetType() string
	GetEmbedding(input string) ([]float32, error)
	GetEmbeddings(inputs []string) ([][]float32, error)
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

// ValidateEmbeddingProviderConfigProps validates the properties of the embedding provider configuration.
func ValidateEmbeddingProviderConfigProps(config EmbeddingProviderConfig) error {
	if config.AuthHeaderName == "" {
		return fmt.Errorf("missing auth header name in the embedding provider configuration")
	}
	if config.APIKey == "" {
		return fmt.Errorf("missing API key in the embedding provider configuration")
	}
	if config.EmbeddingEndpoint == "" {
		return fmt.Errorf("missing embedding endpoint in the embedding provider configuration")
	}
	if config.EmbeddingProvider != "MISTRAL" && config.EmbeddingProvider != "AZURE_OPENAI" && config.EmbeddingProvider != "OPENAI" {
		return fmt.Errorf("missing/Invalid embedding provider found in the embedding provider configuration")
	}
	if config.EmbeddingModel == "" && config.EmbeddingProvider != "AZURE_OPENAI" {
		return fmt.Errorf("missing embedding model in the embedding provider configuration")
	}
	return nil
}
