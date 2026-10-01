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

package certstore

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// identityRows serves ListCertificatesByUsage only.
type identityRows struct {
	storage.Storage
	certs []*models.StoredCertificate
	err   error
}

func (r *identityRows) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out []*models.StoredCertificate
	for _, cert := range r.certs {
		if cert.EffectiveUsage() == usage {
			out = append(out, cert)
		}
	}
	return out, nil
}

func TestGetDefaultGatewayIdentity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identity := func(name, role string) *models.StoredCertificate {
		return &models.StoredCertificate{Name: name, Usage: models.CertificateUsageIdentity, Role: role}
	}

	t.Run("no database", func(t *testing.T) {
		got, err := NewCertStore(logger, nil, "", "").GetDefaultGatewayIdentity()
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("none has role default", func(t *testing.T) {
		db := &identityRows{certs: []*models.StoredCertificate{
			identity("partner", models.CertificateRoleClient),
			{Name: "authority", Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleDefault},
		}}
		got, err := NewCertStore(logger, db, "", "").GetDefaultGatewayIdentity()
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("the default identity", func(t *testing.T) {
		db := &identityRows{certs: []*models.StoredCertificate{
			identity("partner", models.CertificateRoleClient),
			identity("gateway-default", models.CertificateRoleDefault),
		}}
		got, err := NewCertStore(logger, db, "", "").GetDefaultGatewayIdentity()
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "gateway-default", got.Name)
	})

	t.Run("two defaults resolve to the first by name", func(t *testing.T) {
		db := &identityRows{certs: []*models.StoredCertificate{
			identity("zeta", models.CertificateRoleDefault),
			identity("alpha", models.CertificateRoleDefault),
		}}
		got, err := NewCertStore(logger, db, "", "").GetDefaultGatewayIdentity()
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "alpha", got.Name)
	})

	t.Run("read failure", func(t *testing.T) {
		db := &identityRows{err: errors.New("database unavailable")}
		_, err := NewCertStore(logger, db, "", "").GetDefaultGatewayIdentity()
		require.Error(t, err)
	})
}

// The duplicate-defaults warning is logged once per set of default
// identities, not on every lookup.
func TestGetDefaultGatewayIdentity_DuplicateWarningOncePerChange(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	identity := func(name string) *models.StoredCertificate {
		return &models.StoredCertificate{Name: name, Usage: models.CertificateUsageIdentity, Role: models.CertificateRoleDefault}
	}
	db := &identityRows{certs: []*models.StoredCertificate{identity("zeta"), identity("alpha")}}
	cs := NewCertStore(logger, db, "", "")
	const line = "More than one gateway identity has role: default"
	lookup := func() {
		t.Helper()
		_, err := cs.GetDefaultGatewayIdentity()
		require.NoError(t, err)
	}

	lookup()
	lookup()
	assert.Equal(t, 1, strings.Count(logs.String(), line), "an unchanged set is logged once")

	db.certs = append(db.certs, identity("beta"))
	lookup()
	lookup()
	assert.Equal(t, 2, strings.Count(logs.String(), line), "a changed set is logged again")

	db.certs = []*models.StoredCertificate{identity("alpha")}
	lookup()
	db.certs = []*models.StoredCertificate{identity("zeta"), identity("alpha"), identity("beta")}
	lookup()
	assert.Equal(t, 3, strings.Count(logs.String(), line), "the set returning after a single default is logged again")
}
