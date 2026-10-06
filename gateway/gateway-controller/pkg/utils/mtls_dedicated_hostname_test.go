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
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// clientAuthorityStore holds one client authority for the mtls-auth
// validator.
type clientAuthorityStore struct{}

func (clientAuthorityStore) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	return nil, fmt.Errorf("no certificate named %q", name)
}

func (clientAuthorityStore) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	if usage != models.CertificateUsageDownstream {
		return nil, nil
	}
	return []*models.StoredCertificate{{Name: "partner-a", Usage: usage, Role: models.CertificateRoleClient}}, nil
}

func controlPlaneMtlsAuthYAML(vhosts string) []byte {
	return []byte(`apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: cp-mtls
spec:
  displayName: cp-mtls
  version: v1.0.0
  context: /cp-mtls
` + vhosts + `  upstream:
    main:
      url: http://backend:8080
  policies:
    - name: mtls-auth
      version: v1
  operations:
    - method: GET
      path: /
`)
}

// A control-plane deployment goes through the same deploy validator as the
// management REST API, so the setting refuses it there too.
func TestCreateAPIFromYAML_MtlsRequiresDedicatedHostname(t *testing.T) {
	routerConfig := &config.RouterConfig{
		HTTPSEnabled: true,
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "*"},
			Sandbox: config.VHostEntry{Default: "sandbox-*"},
		},
	}
	deploy := func(t *testing.T, required bool, vhosts string) error {
		t.Helper()
		defs := map[string]models.PolicyDefinition{
			"mtls-auth|v1.0.0": {Name: config.MtlsAuthPolicyName, Version: "v1.0.0"},
		}
		mtlsAuthValidator := config.NewMtlsAuthValidator(clientAuthorityStore{}, true, false, nil).
			WithVHosts(routerConfig.VHosts).
			WithDedicatedHostnameRequired(required)
		validator := config.NewAPIValidator()
		validator.SetPolicyValidator(config.NewPolicyValidator(defs, mtlsAuthValidator))

		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		service := newTestAPIDeploymentService(storage.NewConfigStore(), newTestMockDB(), nil, validator, routerConfig)
		utilsService := NewAPIUtilsService(PlatformAPIConfig{}, testHTTPClient(), logger)
		deployedAt := time.Now()
		_, err := utilsService.CreateAPIFromYAML(controlPlaneMtlsAuthYAML(vhosts),
			"0000-cp-mtls-0000-000000000000", "deployment-1", &deployedAt, "corr-1", service)
		return err
	}

	t.Run("refused without its own hostname", func(t *testing.T) {
		err := deploy(t, true, "")
		var listErr *ValidationErrorListError
		require.True(t, errors.As(err, &listErr), "error = %v", err)
		assert.Equal(t, []config.ValidationError{{
			Field: "spec.vhosts.main",
			Message: "this gateway requires every mtls-auth API to have its own hostname " +
				"(an exact name or a leading *.); set vhosts.main",
		}}, listErr.Errors)
	})
	t.Run("accepted with its own hostname", func(t *testing.T) {
		assert.NoError(t, deploy(t, true, "  vhosts:\n    main: pay.example.com\n"))
	})
	t.Run("accepted when the setting is off", func(t *testing.T) {
		assert.NoError(t, deploy(t, false, ""))
	})
}
