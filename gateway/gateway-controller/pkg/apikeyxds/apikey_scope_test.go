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

package apikeyxds

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func translatedKeys(t *testing.T, translator *APIKeyTranslator, keys []*models.APIKey) []APIKeyData {
	t.Helper()
	resources, err := translator.TranslateAPIKeys(keys)
	require.NoError(t, err)
	require.Len(t, resources[APIKeyStateTypeURL], 1)
	anyRes, ok := resources[APIKeyStateTypeURL][0].(*anypb.Any)
	require.True(t, ok)
	// The type URL is the gateway's own; the payload is a Struct.
	st := &structpb.Struct{}
	require.NoError(t, proto.Unmarshal(anyRes.GetValue(), st))
	raw, err := st.MarshalJSON()
	require.NoError(t, err)
	var state APIKeyStateResource
	require.NoError(t, json.Unmarshal(raw, &state))
	return state.APIKeys
}

func key(uuid, name, hash, artifact string) *models.APIKey {
	now := time.Now()
	return &models.APIKey{UUID: uuid, Name: name, APIKey: hash, ArtifactUUID: artifact,
		Status: models.APIKeyStatusActive, CreatedAt: now, UpdatedAt: now}
}

// A Model's key is sent once under each endpoint serving it, scoped to the Model,
// named apart from the endpoint's own keys; every other key is sent as before.
func TestAPIKeyTranslator_ScopedKeys(t *testing.T) {
	translator := NewAPIKeyTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)))
	translator.scopeResolver = func(artifact string) ([]KeyScope, bool) {
		if artifact != "model-uuid" {
			return nil, false
		}
		return []KeyScope{
			{APIId: "endpoint-a", Operations: []string{"claude-haiku", "GET /models"}, Owner: "claude-haiku"},
			{APIId: "endpoint-b", Operations: []string{"claude-haiku", "GET /v1/models"}, Owner: "claude-haiku"},
		}, true
	}

	keys := translatedKeys(t, translator, []*models.APIKey{
		key("k1", "team-a", "hash-endpoint", "endpoint-a"),
		key("k2", "team-a", "hash-model", "model-uuid"),
	})
	require.Len(t, keys, 3)

	byAPI := map[string]APIKeyData{}
	for _, k := range keys {
		byAPI[k.APIId+"|"+k.ID] = k
	}
	endpointKey := byAPI["endpoint-a|k1"]
	assert.Equal(t, AllOperations, endpointKey.Operations)
	assert.Equal(t, "team-a", endpointKey.Name)

	onA := byAPI["endpoint-a|k2"]
	assert.Equal(t, `["claude-haiku","GET /models"]`, onA.Operations)
	assert.Equal(t, "claude-haiku/team-a", onA.Name, "never the name of one of the endpoint's own keys")
	onB := byAPI["endpoint-b|k2"]
	assert.Equal(t, `["claude-haiku","GET /v1/models"]`, onB.Operations)
	_, sentUnderModel := byAPI["model-uuid|k2"]
	assert.False(t, sentUnderModel, "a Model key is not sent under the Model's own id")
}

// The same key value registered on a Model and on an endpoint serving it would put
// two equal keys under one API id, which the policy engine refuses as a whole; the
// endpoint's key is kept and the Model's copy left out.
func TestAPIKeyTranslator_ScopedKeyCollisionKeepsTheEndpointKey(t *testing.T) {
	translator := NewAPIKeyTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)))
	translator.scopeResolver = func(artifact string) ([]KeyScope, bool) {
		if artifact != "model-uuid" {
			return nil, false
		}
		return []KeyScope{{APIId: "endpoint-a", Operations: []string{"m"}, Owner: "m"}}, true
	}
	keys := translatedKeys(t, translator, []*models.APIKey{
		key("k-model", "x", "same-hash", "model-uuid"),
		key("k-endpoint", "y", "same-hash", "endpoint-a"),
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "k-endpoint", keys[0].ID)
	assert.Equal(t, AllOperations, keys[0].Operations)
}

// A Model on no endpoint the gateway has sends its keys nowhere.
func TestAPIKeyTranslator_ModelOnNoEndpoint(t *testing.T) {
	translator := NewAPIKeyTranslator(slog.New(slog.NewTextHandler(io.Discard, nil)))
	translator.scopeResolver = func(string) ([]KeyScope, bool) { return nil, true }
	assert.Empty(t, translatedKeys(t, translator, []*models.APIKey{key("k", "n", "h", "model-uuid")}))
}
