package embeddings

import (
	"fmt"
	"slices"

	"github.com/wso2/api-platform/sdk/ai/azureauthentication"
)

const (
	DefaultRequestTimeout = 30 // DefaultRequestTimeout is the default timeout for requests in seconds (30 seconds)

	// AuthenticationTypeAPIKey authenticates with a key sent in the auth header. It is the default.
	AuthenticationTypeAPIKey = "api-key"

	// DefaultTokenScope is the scope requested for Azure OpenAI access tokens.
	DefaultTokenScope = "https://cognitiveservices.azure.com/.default"
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

	// AuthenticationType is "api-key" (or empty) to send APIKey in AuthHeaderName, or one of
	// azureauthentication.Types() to send a Microsoft Entra ID token for the gateway's Azure
	// identity. Identity types are supported for AZURE_OPENAI only.
	AuthenticationType string

	// ClientID names a user-assigned identity (an application client ID GUID). Empty means the
	// host's default identity. Used with identity types only.
	ClientID string

	// TokenScope is the token scope. Empty means DefaultTokenScope. Used with identity types only.
	TokenScope string
}

// UsesIdentity reports whether the provider authenticates with an Azure identity token rather than
// an API key.
func (c EmbeddingProviderConfig) UsesIdentity() bool {
	return c.AuthenticationType != "" && c.AuthenticationType != AuthenticationTypeAPIKey
}

// ValidateEmbeddingProviderConfigProps validates the properties of the embedding provider configuration.
func ValidateEmbeddingProviderConfigProps(config EmbeddingProviderConfig) error {
	if config.UsesIdentity() {
		if !slices.Contains(azureauthentication.Types(), config.AuthenticationType) {
			return fmt.Errorf("authentication type %q is not supported: use %q or one of %v",
				config.AuthenticationType, AuthenticationTypeAPIKey, azureauthentication.Types())
		}
		if config.EmbeddingProvider != "AZURE_OPENAI" {
			return fmt.Errorf("%s authentication is only supported for the AZURE_OPENAI embedding provider", config.AuthenticationType)
		}
		id := azureauthentication.Identity{Type: config.AuthenticationType, ClientID: config.ClientID}
		if err := id.Validate(); err != nil {
			return fmt.Errorf("invalid client ID: %w", err)
		}
		if config.TokenScope != "" {
			if err := azureauthentication.ValidateScope(config.TokenScope); err != nil {
				return fmt.Errorf("invalid token scope: %w", err)
			}
		}
	} else {
		if config.AuthHeaderName == "" {
			return fmt.Errorf("missing auth header name in the embedding provider configuration")
		}
		if config.APIKey == "" {
			return fmt.Errorf("missing API key in the embedding provider configuration")
		}
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
