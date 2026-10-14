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
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/lazyresourcexds"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

// fakeCertificateRows is a config.MtlsAuthCertificateStore over a fixed row
// list, standing in for the database.
type fakeCertificateRows struct {
	rows    []*models.StoredCertificate
	listErr error
}

func (f *fakeCertificateRows) GetCertificateByName(name string) (*models.StoredCertificate, error) {
	for _, row := range f.rows {
		if row.Name == name {
			return row, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeCertificateRows) ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []*models.StoredCertificate
	for _, row := range f.rows {
		if row.Usage == usage {
			out = append(out, row)
		}
	}
	return out, nil
}

func newTestClientAuthorityPublisher(t *testing.T, rows *fakeCertificateRows) (*ClientAuthorityPublisher, *lazyresourcexds.LazyResourceStateManager, *storage.LazyResourceStore) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewLazyResourceStore(logger)
	manager := lazyresourcexds.NewLazyResourceStateManager(store, lazyresourcexds.NewLazyResourceSnapshotManager(store, logger), logger)
	return NewClientAuthorityPublisher(rows, manager), manager, store
}

func certificateRow(t *testing.T, name, usage, role string, pemCount int) *models.StoredCertificate {
	t.Helper()
	var bundle strings.Builder
	for i := 0; i < pemCount; i++ {
		bundle.Write(pki.NewRootCA(t, name).PEM())
	}
	return &models.StoredCertificate{Name: name, Usage: usage, Role: role, Certificate: []byte(bundle.String())}
}

func TestClientAuthorityPublisher_PublishesOneResourcePerClientRow(t *testing.T) {
	client := certificateRow(t, "partner-root", models.CertificateUsageDownstream, "", 2)
	relayWithMatch := certificateRow(t, "edge-lb", models.CertificateUsageDownstream, models.CertificateRoleRelay, 1)
	relayWithMatch.Match = &models.CertificateMatch{DNSSANs: []string{"lb.corp.test"}}
	relayWithoutMatch := certificateRow(t, "edge-proxy", models.CertificateUsageDownstream, models.CertificateRoleRelay, 1)
	rows := &fakeCertificateRows{rows: []*models.StoredCertificate{
		client, relayWithMatch, relayWithoutMatch,
		certificateRow(t, "backend-ca", models.CertificateUsageUpstream, "", 1),
		certificateRow(t, "gateway-identity", models.CertificateUsageIdentity, "", 1),
	}}
	publisher, manager, _ := newTestClientAuthorityPublisher(t, rows)

	require.NoError(t, publisher.Publish("corr"))

	published := manager.GetResourcesByType(LazyResourceTypeClientCertificateAuthority)
	require.Len(t, published, 3, "only usage: downstream rows are published")

	root := published["partner-root"]
	require.NotNil(t, root)
	assert.Equal(t, "partner-root", root.ID)
	assert.Equal(t, models.CertificateRoleClient, root.Resource["role"], "an empty stored role publishes as client")
	certs, ok := root.Resource["certificates"].([]string)
	require.True(t, ok)
	require.Len(t, certs, 2, "one PEM string per certificate in the row")
	for _, c := range certs {
		assert.Equal(t, 1, strings.Count(c, "BEGIN CERTIFICATE"))
	}
	_, hasMatch := root.Resource["match"]
	assert.False(t, hasMatch)

	lb := published["edge-lb"]
	require.NotNil(t, lb)
	assert.Equal(t, models.CertificateRoleRelay, lb.Resource["role"])
	assert.Equal(t, map[string]interface{}{"dnsSANs": []string{"lb.corp.test"}}, lb.Resource["match"],
		"match carries only the lists the relay row has")

	proxy := published["edge-proxy"]
	require.NotNil(t, proxy)
	_, hasMatch = proxy.Resource["match"]
	assert.False(t, hasMatch, "a relay row stored without narrowing carries no match")
}

func TestClientAuthorityPublisher_RemovesStaleAndSkipsUnchanged(t *testing.T) {
	kept := certificateRow(t, "kept", models.CertificateUsageDownstream, "", 1)
	removed := certificateRow(t, "removed", models.CertificateUsageDownstream, "", 1)
	rows := &fakeCertificateRows{rows: []*models.StoredCertificate{kept, removed}}
	publisher, manager, store := newTestClientAuthorityPublisher(t, rows)
	require.NoError(t, publisher.Publish(""))
	require.Len(t, manager.GetResourcesByType(LazyResourceTypeClientCertificateAuthority), 2)

	// Another resource type sharing the store is never touched.
	require.NoError(t, manager.StoreResource(&storage.LazyResource{
		ID: "removed", ResourceType: LazyResourceTypeLLMProviderTemplate, Resource: map[string]interface{}{},
	}, ""))

	rows.rows = []*models.StoredCertificate{kept}
	require.NoError(t, publisher.Publish(""))
	published := manager.GetResourcesByType(LazyResourceTypeClientCertificateAuthority)
	require.Len(t, published, 1)
	assert.Contains(t, published, "kept")
	_, stillThere := manager.GetResourceByIDAndType("removed", LazyResourceTypeLLMProviderTemplate)
	assert.True(t, stillThere)

	before := store.GetResourceVersion()
	require.NoError(t, publisher.Publish(""))
	assert.Equal(t, before, store.GetResourceVersion(), "publishing an unchanged pool pushes no snapshot")
}

func TestClientAuthorityPublisher_ListFailureLeavesPublishedSetAlone(t *testing.T) {
	rows := &fakeCertificateRows{rows: []*models.StoredCertificate{certificateRow(t, "kept", models.CertificateUsageDownstream, "", 1)}}
	publisher, manager, _ := newTestClientAuthorityPublisher(t, rows)
	require.NoError(t, publisher.Publish(""))

	rows.listErr = errors.New("database unavailable")
	require.Error(t, publisher.Publish(""))
	assert.Len(t, manager.GetResourcesByType(LazyResourceTypeClientCertificateAuthority), 1,
		"a failed read must never unpublish the pool")
}
