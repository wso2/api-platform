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
package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

const (
	storedCred = "Bearer sk-stored-credential"
	newCred    = "Bearer sk-rotated-credential"
)

func sp(s string) *string { return &s }

// storedProvider builds a persisted provider carrying storedCred.
func storedProvider() api.LLMProviderConfiguration {
	var cfg api.LLMProviderConfiguration
	cfg.Spec.Upstream.Url = sp("https://api.openai.com/v1")
	cfg.Spec.Upstream.Auth = &struct {
		Header        *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
		PolicyName    *string                                   `json:"policyName,omitempty" yaml:"policyName,omitempty"`
		PolicyParams  *map[string]interface{}                   `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
		PolicyVersion *string                                   `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
		Type          api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
		Value         *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
	}{Header: sp("Authorization"), Type: "api-key", Value: sp(storedCred)}
	return cfg
}

// storedOAuth2Provider builds a persisted provider whose credential lives in
// PolicyParams (oauth2/other auth), not Value.
func storedOAuth2Provider() api.LLMProviderConfiguration {
	var cfg api.LLMProviderConfiguration
	cfg.Spec.Upstream.Url = sp("https://api.openai.com/v1")
	params := map[string]interface{}{"tokenEndpoint": "https://idp.example.com/token", "clientSecret": storedCred}
	cfg.Spec.Upstream.Auth = &struct {
		Header        *string                                   `json:"header,omitempty" yaml:"header,omitempty"`
		PolicyName    *string                                   `json:"policyName,omitempty" yaml:"policyName,omitempty"`
		PolicyParams  *map[string]interface{}                   `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
		PolicyVersion *string                                   `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
		Type          api.LLMProviderConfigDataUpstreamAuthType `json:"type" yaml:"type"`
		Value         *string                                   `json:"value,omitempty" yaml:"value,omitempty"`
	}{Type: "oauth2", PolicyParams: &params}
	return cfg
}

func TestInheritLLMProviderCredential(t *testing.T) {
	t.Run("auth omitted entirely inherits the stored block", func(t *testing.T) {
		var incoming api.LLMProviderConfiguration
		incoming.Spec.Upstream.Url = sp("https://api.openai.com/v1")

		inheritLLMProviderCredential(&incoming, storedProvider())

		require.NotNil(t, incoming.Spec.Upstream.Auth, "stored auth block should be inherited")
		require.NotNil(t, incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, "Authorization", *incoming.Spec.Upstream.Auth.Header)
	})

	t.Run("auth present with no value inherits the stored value", func(t *testing.T) {
		incoming := storedProvider()
		incoming.Spec.Upstream.Auth.Value = nil // what a redacted GET returns

		inheritLLMProviderCredential(&incoming, storedProvider())

		require.NotNil(t, incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
	})

	t.Run("empty-string value is treated as absent and inherits", func(t *testing.T) {
		incoming := storedProvider()
		incoming.Spec.Upstream.Auth.Value = sp("")

		inheritLLMProviderCredential(&incoming, storedProvider())

		require.NotNil(t, incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
	})

	t.Run("supplied value wins so rotation still works", func(t *testing.T) {
		incoming := storedProvider()
		incoming.Spec.Upstream.Auth.Value = sp(newCred)

		inheritLLMProviderCredential(&incoming, storedProvider())

		assert.Equal(t, newCred, *incoming.Spec.Upstream.Auth.Value, "must not clobber a rotated credential")
	})

	t.Run("type none removes auth and inherits nothing", func(t *testing.T) {
		incoming := storedProvider()
		incoming.Spec.Upstream.Auth.Type = "none"
		incoming.Spec.Upstream.Auth.Value = nil

		inheritLLMProviderCredential(&incoming, storedProvider())

		assert.Nil(t, incoming.Spec.Upstream.Auth.Value, "type: none is the explicit removal signal")
	})

	t.Run("nothing stored leaves the incoming config untouched", func(t *testing.T) {
		var incoming api.LLMProviderConfiguration
		inheritLLMProviderCredential(&incoming, api.LLMProviderConfiguration{})
		assert.Nil(t, incoming.Spec.Upstream.Auth)
	})

	// The stored artifact is a plain map once it has round-tripped through the
	// database, so inheritance must work from that shape too.
	t.Run("inherits from a map-shaped stored configuration", func(t *testing.T) {
		storedAsMap := map[string]any{
			"spec": map[string]any{
				"upstream": map[string]any{
					"url":  "https://api.openai.com/v1",
					"auth": map[string]any{"type": "api-key", "header": "Authorization", "value": storedCred},
				},
			},
		}
		var incoming api.LLMProviderConfiguration

		inheritLLMProviderCredential(&incoming, storedAsMap)

		require.NotNil(t, incoming.Spec.Upstream.Auth)
		require.NotNil(t, incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
	})

	// A credential held as a secret expression must be carried forward
	// unresolved, so inheritance never materialises a plaintext secret.
	t.Run("carries a secret expression forward unresolved", func(t *testing.T) {
		const expr = `{{ secret "openai-prod-key" }}`
		stored := storedProvider()
		stored.Spec.Upstream.Auth.Value = sp(expr)

		var incoming api.LLMProviderConfiguration
		inheritLLMProviderCredential(&incoming, stored)

		require.NotNil(t, incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, expr, *incoming.Spec.Upstream.Auth.Value)
	})

	// A credential belongs to the scheme it was stored under: changing the auth
	// type must not silently carry the old value into the new scheme.
	t.Run("changed auth type inherits nothing", func(t *testing.T) {
		incoming := storedProvider()
		incoming.Spec.Upstream.Auth.Type = "other"
		incoming.Spec.Upstream.Auth.Value = nil

		inheritLLMProviderCredential(&incoming, storedProvider())

		assert.Nil(t, incoming.Spec.Upstream.Auth.Value,
			"a changed auth type must supply its own credential")
	})

	t.Run("unchanged auth type still inherits", func(t *testing.T) {
		incoming := storedProvider()
		incoming.Spec.Upstream.Auth.Type = "api-key" // same as stored
		incoming.Spec.Upstream.Auth.Value = nil

		inheritLLMProviderCredential(&incoming, storedProvider())

		require.NotNil(t, incoming.Spec.Upstream.Auth.Value)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
	})

	t.Run("nil incoming does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() { inheritLLMProviderCredential(nil, storedProvider()) })
	})

	// Regression: oauth2/other stores its credential in PolicyParams, not Value.
	t.Run("oauth2 auth omitted entirely inherits the stored policyParams", func(t *testing.T) {
		var incoming api.LLMProviderConfiguration
		incoming.Spec.Upstream.Url = sp("https://api.openai.com/v1")

		inheritLLMProviderCredential(&incoming, storedOAuth2Provider())

		require.NotNil(t, incoming.Spec.Upstream.Auth, "stored oauth2 auth block should be inherited")
		require.NotNil(t, incoming.Spec.Upstream.Auth.PolicyParams)
		assert.Equal(t, storedCred, (*incoming.Spec.Upstream.Auth.PolicyParams)["clientSecret"])
	})

	t.Run("oauth2 auth present with no policyParams inherits the stored policyParams", func(t *testing.T) {
		incoming := storedOAuth2Provider()
		incoming.Spec.Upstream.Auth.PolicyParams = nil // what a redacted GET would return

		inheritLLMProviderCredential(&incoming, storedOAuth2Provider())

		require.NotNil(t, incoming.Spec.Upstream.Auth.PolicyParams)
		assert.Equal(t, storedCred, (*incoming.Spec.Upstream.Auth.PolicyParams)["clientSecret"])
	})

	t.Run("supplied policyParams wins so rotation still works", func(t *testing.T) {
		incoming := storedOAuth2Provider()
		rotated := map[string]interface{}{"clientSecret": newCred}
		incoming.Spec.Upstream.Auth.PolicyParams = &rotated

		inheritLLMProviderCredential(&incoming, storedOAuth2Provider())

		assert.Equal(t, newCred, (*incoming.Spec.Upstream.Auth.PolicyParams)["clientSecret"],
			"must not clobber rotated policyParams")
	})

	// Regression: a client that always serializes policyParams as `{}` must still inherit.
	t.Run("empty-but-present policyParams inherits the stored policyParams", func(t *testing.T) {
		incoming := storedOAuth2Provider()
		empty := map[string]interface{}{}
		incoming.Spec.Upstream.Auth.PolicyParams = &empty

		inheritLLMProviderCredential(&incoming, storedOAuth2Provider())

		require.NotNil(t, incoming.Spec.Upstream.Auth.PolicyParams)
		assert.Equal(t, storedCred, (*incoming.Spec.Upstream.Auth.PolicyParams)["clientSecret"],
			"an empty policyParams map must not be treated as a supplied credential")
	})
}

func TestInheritLLMProxyCredentials(t *testing.T) {
	stored := func() api.LLMProxyConfiguration {
		var cfg api.LLMProxyConfiguration
		cfg.Spec.Provider = api.LLMProxyProvider{
			Id:   "openai-provider",
			Auth: &api.LLMUpstreamAuth{Type: "api-key", Header: sp("Authorization"), Value: sp(storedCred)},
		}
		additional := []api.LLMProxyAdditionalProvider{
			{Id: "anthropic-provider", Auth: &api.LLMUpstreamAuth{Type: "api-key", Value: sp("Bearer sk-anthropic-stored")}},
			{Id: "gemini-provider", Auth: &api.LLMUpstreamAuth{Type: "api-key", Value: sp("Bearer sk-gemini-stored")}},
		}
		cfg.Spec.AdditionalProviders = &additional
		return cfg
	}

	t.Run("primary provider auth omitted is inherited", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{Id: "openai-provider"}

		inheritLLMProxyCredentials(&incoming, stored())

		require.NotNil(t, incoming.Spec.Provider.Auth)
		assert.Equal(t, storedCred, *incoming.Spec.Provider.Auth.Value)
	})

	// Matching is by provider id, not list position, so a reordered
	// additionalProviders list still inherits the right credential.
	t.Run("additional providers inherit by id, not position", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{Id: "openai-provider"}
		reordered := []api.LLMProxyAdditionalProvider{
			{Id: "gemini-provider"},
			{Id: "anthropic-provider"},
		}
		incoming.Spec.AdditionalProviders = &reordered

		inheritLLMProxyCredentials(&incoming, stored())

		got := *incoming.Spec.AdditionalProviders
		require.NotNil(t, got[0].Auth)
		require.NotNil(t, got[1].Auth)
		assert.Equal(t, "Bearer sk-gemini-stored", *got[0].Auth.Value)
		assert.Equal(t, "Bearer sk-anthropic-stored", *got[1].Auth.Value)
	})

	t.Run("an unknown additional provider id inherits nothing", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{Id: "openai-provider"}
		fresh := []api.LLMProxyAdditionalProvider{{Id: "brand-new-provider"}}
		incoming.Spec.AdditionalProviders = &fresh

		inheritLLMProxyCredentials(&incoming, stored())

		assert.Nil(t, (*incoming.Spec.AdditionalProviders)[0].Auth)
	})

	t.Run("changed auth type on the primary provider inherits nothing", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{
			Id:   "openai-provider",
			Auth: &api.LLMUpstreamAuth{Type: "other"},
		}

		inheritLLMProxyCredentials(&incoming, stored())

		assert.Nil(t, incoming.Spec.Provider.Auth.Value)
	})

	t.Run("type none on the primary provider removes auth", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{
			Id:   "openai-provider",
			Auth: &api.LLMUpstreamAuth{Type: "none"},
		}

		inheritLLMProxyCredentials(&incoming, stored())

		assert.Nil(t, incoming.Spec.Provider.Auth.Value)
	})

	// An auth block belongs to a specific provider id, so repointing the primary
	// provider must not carry the previous provider's credential across.
	t.Run("repointed primary provider inherits nothing", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{Id: "a-different-provider"}

		inheritLLMProxyCredentials(&incoming, stored())

		assert.Nil(t, incoming.Spec.Provider.Auth,
			"a repointed provider must supply its own credential")
	})

	t.Run("repointed primary provider with an empty value inherits nothing", func(t *testing.T) {
		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{
			Id:   "a-different-provider",
			Auth: &api.LLMUpstreamAuth{Type: "api-key"},
		}

		inheritLLMProxyCredentials(&incoming, stored())

		assert.Nil(t, incoming.Spec.Provider.Auth.Value)
	})

	// Regression: see the equivalent oauth2 case in TestInheritLLMProviderCredential.
	t.Run("oauth2 primary provider auth omitted entirely inherits the stored policyParams", func(t *testing.T) {
		params := map[string]interface{}{"tokenEndpoint": "https://idp.example.com/token", "clientSecret": storedCred}
		storedOAuth2 := func() api.LLMProxyConfiguration {
			var cfg api.LLMProxyConfiguration
			cfg.Spec.Provider = api.LLMProxyProvider{
				Id:   "openai-provider",
				Auth: &api.LLMUpstreamAuth{Type: "oauth2", PolicyParams: &params},
			}
			return cfg
		}

		var incoming api.LLMProxyConfiguration
		incoming.Spec.Provider = api.LLMProxyProvider{Id: "openai-provider"}

		inheritLLMProxyCredentials(&incoming, storedOAuth2())

		require.NotNil(t, incoming.Spec.Provider.Auth)
		require.NotNil(t, incoming.Spec.Provider.Auth.PolicyParams)
		assert.Equal(t, storedCred, (*incoming.Spec.Provider.Auth.PolicyParams)["clientSecret"])
	})
}

func TestInheritMCPProxyCredential(t *testing.T) {
	stored := func() api.MCPProxyConfiguration {
		var cfg api.MCPProxyConfiguration
		cfg.Spec.Upstream.Auth = &struct {
			Header        *string                                `json:"header,omitempty" yaml:"header,omitempty"`
			PolicyName    *string                                `json:"policyName,omitempty" yaml:"policyName,omitempty"`
			PolicyParams  *map[string]interface{}                `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
			PolicyVersion *string                                `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
			Type          api.MCPProxyConfigDataUpstreamAuthType `json:"type" yaml:"type"`
			Value         *string                                `json:"value,omitempty" yaml:"value,omitempty"`
		}{Header: sp("Authorization"), Type: "api-key", Value: sp(storedCred)}
		return cfg
	}

	t.Run("auth omitted is inherited", func(t *testing.T) {
		var incoming api.MCPProxyConfiguration
		inheritMCPProxyCredential(&incoming, stored())
		require.NotNil(t, incoming.Spec.Upstream.Auth)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
	})

	t.Run("supplied value wins", func(t *testing.T) {
		incoming := stored()
		incoming.Spec.Upstream.Auth.Value = sp(newCred)
		inheritMCPProxyCredential(&incoming, stored())
		assert.Equal(t, newCred, *incoming.Spec.Upstream.Auth.Value)
	})

	t.Run("changed auth type inherits nothing", func(t *testing.T) {
		incoming := stored()
		incoming.Spec.Upstream.Auth.Type = "other"
		incoming.Spec.Upstream.Auth.Value = nil
		inheritMCPProxyCredential(&incoming, stored())
		assert.Nil(t, incoming.Spec.Upstream.Auth.Value)
	})

	t.Run("type none removes auth", func(t *testing.T) {
		incoming := stored()
		incoming.Spec.Upstream.Auth.Type = "none"
		incoming.Spec.Upstream.Auth.Value = nil
		inheritMCPProxyCredential(&incoming, stored())
		assert.Nil(t, incoming.Spec.Upstream.Auth.Value)
	})

	// Regression: see the equivalent oauth2 case in TestInheritLLMProviderCredential.
	t.Run("oauth2 auth omitted entirely inherits the stored policyParams", func(t *testing.T) {
		storedOAuth2 := func() api.MCPProxyConfiguration {
			var cfg api.MCPProxyConfiguration
			params := map[string]interface{}{"tokenEndpoint": "https://idp.example.com/token", "clientSecret": storedCred}
			cfg.Spec.Upstream.Auth = &struct {
				Header        *string                                `json:"header,omitempty" yaml:"header,omitempty"`
				PolicyName    *string                                `json:"policyName,omitempty" yaml:"policyName,omitempty"`
				PolicyParams  *map[string]interface{}                `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
				PolicyVersion *string                                `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
				Type          api.MCPProxyConfigDataUpstreamAuthType `json:"type" yaml:"type"`
				Value         *string                                `json:"value,omitempty" yaml:"value,omitempty"`
			}{Type: "oauth2", PolicyParams: &params}
			return cfg
		}

		var incoming api.MCPProxyConfiguration
		inheritMCPProxyCredential(&incoming, storedOAuth2())

		require.NotNil(t, incoming.Spec.Upstream.Auth)
		require.NotNil(t, incoming.Spec.Upstream.Auth.PolicyParams)
		assert.Equal(t, storedCred, (*incoming.Spec.Upstream.Auth.PolicyParams)["clientSecret"])
	})
}

// errOnGetConfigDB wraps the shared test double to simulate a lookup failure
// that is not "not found".
type errOnGetConfigDB struct {
	storage.Storage
	err error
}

func (d errOnGetConfigDB) GetConfig(string) (*models.StoredConfig, error) {
	return nil, d.err
}

func TestStoredSourceForUpdate(t *testing.T) {
	t.Run("no id means a create, so nothing to inherit", func(t *testing.T) {
		source, err := storedSourceForUpdate(newTestMockDB(), "")
		require.NoError(t, err)
		assert.Nil(t, source)
	})

	t.Run("nil db is tolerated", func(t *testing.T) {
		source, err := storedSourceForUpdate(nil, "some-id")
		require.NoError(t, err)
		assert.Nil(t, source)
	})

	t.Run("unknown id is not an error", func(t *testing.T) {
		source, err := storedSourceForUpdate(newTestMockDB(), "no-such-id")
		require.NoError(t, err)
		assert.Nil(t, source)
	})

	t.Run("returns the stored source configuration", func(t *testing.T) {
		db := newTestMockDB()
		require.NoError(t, db.SaveConfig(&models.StoredConfig{
			UUID:                "known-id",
			SourceConfiguration: storedProvider(),
		}))

		source, err := storedSourceForUpdate(db, "known-id")
		require.NoError(t, err)
		require.NotNil(t, source)

		var incoming api.LLMProviderConfiguration
		inheritLLMProviderCredential(&incoming, source)
		require.NotNil(t, incoming.Spec.Upstream.Auth)
		assert.Equal(t, storedCred, *incoming.Spec.Upstream.Auth.Value)
	})

	// A lookup failure must surface rather than be reported as "nothing stored":
	// the validators only inspect an auth block that is present, so a silently
	// skipped inheritance would deploy with no upstream auth attached.
	t.Run("a lookup failure is returned as an error", func(t *testing.T) {
		db := errOnGetConfigDB{Storage: newTestMockDB(), err: errors.New("connection refused")}

		source, err := storedSourceForUpdate(db, "known-id")

		require.Error(t, err)
		assert.Nil(t, source)
		assert.Contains(t, err.Error(), "credential inheritance")
	})
}
