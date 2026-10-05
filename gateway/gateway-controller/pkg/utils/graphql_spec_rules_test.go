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
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// graphQLAPIYAML renders a minimal deployable GraphQLApi.
func graphQLAPIYAML(handle, displayName, version, context string) []byte {
	return []byte(fmt.Sprintf(`
apiVersion: gateway.api-platform.wso2.com/v1
kind: GraphQLApi
metadata:
  name: %s
spec:
  displayName: %q
  version: %q
  context: %q
  upstream:
    main:
      url: https://backend.example.com/graphql
`, handle, displayName, version, context))
}

// validateGraphQLYAML runs the GraphQLApi parse + config-validation pair the
// deploy path uses, returning only the validation errors.
func validateGraphQLYAML(t *testing.T, data []byte) []config.ValidationError {
	t.Helper()
	cfg, _, _, _, err := parseGraphQLAPIDeployment(config.NewParser(), data, "application/yaml")
	require.NoError(t, err)
	_, _, errs := validateGraphQLAPIConfig(cfg)
	return errs
}

func fieldMessages(errs []config.ValidationError, field string) []string {
	var messages []string
	for _, e := range errs {
		if e.Field == field {
			messages = append(messages, e.Message)
		}
	}
	return messages
}

// TestValidateGraphQLAPIConfig_SpecRulesMatchRestApi pins GraphQLApi's
// displayName/version/context rules to RestApi's own: each case is checked
// against what config.APIValidator reports for the same value, so a rule
// changed for one kind can't silently stop applying to the other.
func TestValidateGraphQLAPIConfig_SpecRulesMatchRestApi(t *testing.T) {
	rest := config.NewAPIValidator()

	cases := []struct {
		name                          string
		displayName, version, context string
		field                         string
		restRule                      []config.ValidationError
	}{
		{
			name:        "display name longer than 100 characters",
			displayName: strings.Repeat("a", 101), version: "v1.0", context: "/countries",
			field: "spec.displayName", restRule: rest.ValidateDisplayName(strings.Repeat("a", 101)),
		},
		{
			name:        "display name with characters that are not URL-friendly",
			displayName: "Countries/API", version: "v1.0", context: "/countries",
			field: "spec.displayName", restRule: rest.ValidateDisplayName("Countries/API"),
		},
		{
			name:        "version that is not a semantic version",
			displayName: "Countries API", version: "1.0-beta", context: "/countries",
			field: "spec.version", restRule: rest.ValidateVersion("1.0-beta"),
		},
		{
			name:        "version with a free-text label",
			displayName: "Countries API", version: "latest", context: "/countries",
			field: "spec.version", restRule: rest.ValidateVersion("latest"),
		},
		{
			name:        "context longer than 200 characters",
			displayName: "Countries API", version: "v1.0", context: "/" + strings.Repeat("c", 200),
			field: "spec.context", restRule: rest.ValidateContext("/" + strings.Repeat("c", 200)),
		},
		{
			name:        "context with a trailing slash",
			displayName: "Countries API", version: "v1.0", context: "/countries/",
			field: "spec.context", restRule: rest.ValidateContext("/countries/"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotEmpty(t, tc.restRule, "RestApi must reject this value too, or the case proves nothing")

			errs := validateGraphQLYAML(t, graphQLAPIYAML("countries", tc.displayName, tc.version, tc.context))

			assert.Equal(t, fieldMessages(tc.restRule, tc.field), fieldMessages(errs, tc.field))
		})
	}

	t.Run("accepts the values RestApi accepts", func(t *testing.T) {
		for _, version := range []string{"v1", "v1.0", "v2.1.3", "1.0.0"} {
			errs := validateGraphQLYAML(t, graphQLAPIYAML("countries", "Countries API v2_beta.1", version, "/countries"))

			assert.Empty(t, fieldMessages(errs, "spec.displayName"), "displayName")
			assert.Empty(t, fieldMessages(errs, "spec.version"), "version %q", version)
			assert.Empty(t, fieldMessages(errs, "spec.context"), "context")
		}
	})
}

// TestDeployGraphQLAPI_DBConflictValidation mirrors RestApi's
// TestDeployAPIConfiguration_DBConflictValidation: a GraphQLApi's display
// name + version, and its handle, must be unique among GraphQLApis.
func TestDeployGraphQLAPI_DBConflictValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kind := string(api.GraphQLAPIKindGraphQLApi)

	existing := func(uuid, handle, displayName, version string) *models.StoredConfig {
		return &models.StoredConfig{
			UUID: uuid, Kind: kind, Handle: handle, DisplayName: displayName, Version: version,
		}
	}
	deploy := func(service *APIDeploymentService, apiID string, data []byte) error {
		_, err := service.DeployAPIConfiguration(APIDeploymentParams{
			APIID:         apiID,
			Data:          data,
			ContentType:   "application/yaml",
			CorrelationID: "test-corr",
			Kind:          kind,
			Origin:        models.OriginGatewayAPI,
			Logger:        logger,
		})
		return err
	}

	t.Run("rejects a new API reusing another's display name and version", func(t *testing.T) {
		db := newTestMockDB()
		service := newTestAPIDeploymentService(storage.NewConfigStore(), db, nil, config.NewAPIValidator(), nil)
		require.NoError(t, db.SaveConfig(existing("gql-existing-1", "countries", "Countries API", "v1.0")))

		err := deploy(service, "", graphQLAPIYAML("countries-copy", "Countries API", "v1.0", "/countries-copy"))

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "name 'Countries API' and version 'v1.0' already exists")
	})

	t.Run("rejects a new API reusing another's handle", func(t *testing.T) {
		db := newTestMockDB()
		service := newTestAPIDeploymentService(storage.NewConfigStore(), db, nil, config.NewAPIValidator(), nil)
		require.NoError(t, db.SaveConfig(existing("gql-existing-2", "countries", "Countries API", "v1.0")))

		err := deploy(service, "", graphQLAPIYAML("countries", "Another API", "v2.0", "/another"))

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "handle 'countries' already exists")
	})

	t.Run("rejects an update that renames one API onto another's name and version", func(t *testing.T) {
		db := newTestMockDB()
		service := newTestAPIDeploymentService(storage.NewConfigStore(), db, nil, config.NewAPIValidator(), nil)
		require.NoError(t, db.SaveConfig(existing("gql-a", "countries", "Countries API", "v1.0")))
		require.NoError(t, db.SaveConfig(existing("gql-b", "cities", "Cities API", "v1.0")))

		err := deploy(service, "gql-b", graphQLAPIYAML("cities", "Countries API", "v1.0", "/cities"))

		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrConflict)
		assert.Contains(t, err.Error(), "name 'Countries API' and version 'v1.0' already exists")
	})

	t.Run("does not count an API as conflicting with itself on update", func(t *testing.T) {
		db := newTestMockDB()
		service := newTestAPIDeploymentService(storage.NewConfigStore(), db, nil, config.NewAPIValidator(), nil)
		require.NoError(t, db.SaveConfig(existing("gql-self", "countries", "Countries API", "v1.0")))

		err := deploy(service, "gql-self", graphQLAPIYAML("countries", "Countries API", "v1.0", "/countries"))

		assert.False(t, storage.IsConflictError(err), "unexpected conflict: %v", err)
	})
}
