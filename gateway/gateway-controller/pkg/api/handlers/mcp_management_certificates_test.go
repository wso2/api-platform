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

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/encryption"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
)

// The expected route keys below are written by hand rather than read from
// certActions, so this test compares the dispatch table against the management
// routes as spelled in cmd/controller/main.go rather than against itself.
func TestResolveCertAction(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		wantAction   string
		wantRouteKey string
		wantMutating bool
		wantConfirm  bool
	}{
		{"list", "list", certActionList, "GET /certificates", false, false},
		{"apply", "apply", certActionApply, "POST /certificates", true, false},
		{"delete", "delete", certActionDelete, "DELETE /certificates/{id}", true, true},
		{"reload", "reload", certActionReload, "POST /certificates/reload", true, true},
		{"update", "update", certActionUpdate, "PUT /certificates/{id}", true, true},

		// Alias and spelling tolerance: a model reaching for a different verb,
		// or a different casing, must land on the same operation.
		{"upload aliases to apply", "upload", certActionApply, "POST /certificates", true, false},
		{"create aliases to apply", "create", certActionApply, "POST /certificates", true, false},
		{"remove aliases to delete", "remove", certActionDelete, "DELETE /certificates/{id}", true, true},
		{"refresh aliases to reload", "refresh", certActionReload, "POST /certificates/reload", true, true},
		{"rotate aliases to update", "rotate", certActionUpdate, "PUT /certificates/{id}", true, true},
		{"replace aliases to update", "replace", certActionUpdate, "PUT /certificates/{id}", true, true},
		{"upper case", "LIST", certActionList, "GET /certificates", false, false},
		{"surrounding space", "  Apply  ", certActionApply, "POST /certificates", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := resolveCertAction(tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAction, op.Action)
			assert.Equal(t, tt.wantRouteKey, op.RouteKey)
			assert.Equal(t, tt.wantMutating, op.Mutating)
			assert.Equal(t, tt.wantConfirm, op.NeedsConfirm)
		})
	}
}

func TestResolveCertActionRejectsUnknown(t *testing.T) {
	for _, action := range []string{"", "get", "patch", "drop"} {
		_, err := resolveCertAction(action)
		require.Error(t, err, "action %q should not resolve", action)
		assert.Contains(t, err.Error(), "list, apply, update, delete, reload",
			"the rejection should name the actions that would have worked")
	}
}

// The authorization gate and the tool must derive the same route key from the
// same arguments; if they can disagree, the gate authorizes one operation while
// the handler performs another.
func TestCertificateGateAgreesWithHandler(t *testing.T) {
	h := &McpHandler{}

	for _, action := range []string{"list", "apply", "update", "delete", "reload", "UPLOAD", "Rotate"} {
		t.Run(action, func(t *testing.T) {
			args, err := json.Marshal(manageCertificatesInput{Action: action, ID: "cert-1"})
			require.NoError(t, err)

			gateKeys, ok := h.routeKeysForCall("wso2_apip_gw_manage_certificates", args)
			require.True(t, ok, "the gate must be able to map a resolvable call")
			require.Len(t, gateKeys, 1, "a certificate call maps to exactly one route key")

			op, err := resolveCertAction(action)
			require.NoError(t, err)
			assert.Equal(t, op.RouteKey, gateKeys[0])
		})
	}
}

// An unresolvable action is passed through rather than authorized, so the tool
// itself can explain what was wrong — the same contract an unknown kind has.
func TestCertificateGatePassesThroughUnresolvableAction(t *testing.T) {
	h := &McpHandler{}
	args, err := json.Marshal(manageCertificatesInput{Action: "sideways"})
	require.NoError(t, err)

	_, ok := h.routeKeysForCall("wso2_apip_gw_manage_certificates", args)
	assert.False(t, ok)
}

func TestManageCertificatesGuards(t *testing.T) {
	// Skipped exempts the role check, which is what lets these cases run
	// without a role map; every guard asserted here sits outside that check.
	authorized := withMcpCaller(context.Background(), mcpCaller{Skipped: true})

	t.Run("immutable mode refuses every mutating action", func(t *testing.T) {
		h := newAuthzTestHandler(true)
		for _, action := range []string{"apply", "update", "delete", "reload"} {
			_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
				Action: action, Confirm: true, ID: "cert-1", Name: "n", Certificate: "pem",
			})
			require.Errorf(t, err, "%s should be refused in immutable mode", action)
			assert.Contains(t, err.Error(), "immutable mode")
		}
	})

	t.Run("update, delete and reload require confirm", func(t *testing.T) {
		h := newAuthzTestHandler(false)
		for _, action := range []string{"update", "delete", "reload"} {
			_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
				Action: action, ID: "cert-1",
			})
			require.Errorf(t, err, "%s should require confirm", action)
			assert.Contains(t, err.Error(), "confirm=true")
		}
	})

	t.Run("apply rejects an id rather than creating a duplicate", func(t *testing.T) {
		h := newAuthzTestHandler(false)
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "apply", ID: "cert-1", Name: "n", Certificate: "pem",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action=update")
	})

	// No pre-check: the service reports the missing field, as REST does.
	t.Run("apply reports a missing certificate field by field", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "apply", Name: "n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1. certificate: both name and certificate are required")
	})

	t.Run("update requires an id", func(t *testing.T) {
		h := newAuthzTestHandler(false)
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "update", Confirm: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action=list")
	})

	t.Run("delete requires an id", func(t *testing.T) {
		h := newAuthzTestHandler(false)
		_, _, err := h.manageCertificates(authorized, nil, manageCertificatesInput{
			Action: "delete", Confirm: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "action=list")
	})
}

// A tool handler reached without an authorization decision on the context must
// deny: its absence is what proves the gate ran.
func TestManageCertificatesDeniesWithoutGate(t *testing.T) {
	h := newAuthzTestHandler(false)
	_, _, err := h.manageCertificates(context.Background(), nil, manageCertificatesInput{Action: "list"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

// Upload saves the row before it finds there is no cert store, so apply must
// report that state changed; the other actions changed nothing.
func TestMcpCertErrorWithoutCertStore(t *testing.T) {
	err := mcpCertError(certActionApply, "upstream", certificate.ErrCertStoreNotConfigured)
	assert.Contains(t, err.Error(), `"upstream" was stored`)
	assert.Contains(t, err.Error(), "Do not repeat this call")

	for _, action := range []string{certActionList, certActionDelete, certActionReload} {
		err := mcpCertError(action, "", certificate.ErrCertStoreNotConfigured)
		assert.NotContains(t, err.Error(), "was stored", action)
		assert.Contains(t, err.Error(), "unavailable", action)
	}

	// Update also writes before it finds there is no cert store; the error
	// arrives wrapped in the SyncError of the failed push.
	err = mcpCertError(certActionUpdate, "cert-1", &certificate.SyncError{
		Stage: certificate.StageSnapshot, Persisted: true, Cause: certificate.ErrCertStoreNotConfigured,
	})
	assert.Contains(t, err.Error(), `"cert-1" were stored`)
	assert.Contains(t, err.Error(), "Do not repeat this call")
}

// stubCertStore and stubSnapshot stand in for the router-side collaborators the
// certificate service reaches through its XDSResolver, so the MCP certificate
// tool's success paths are reachable without a real *xds.SnapshotManager.
type stubCertStore struct {
	reloadErr error
	combined  []byte
}

func (c *stubCertStore) Reload() error                   { return c.reloadErr }
func (c *stubCertStore) GetCombinedCertificates() []byte { return c.combined }

type stubSnapshot struct {
	err error
}

func (s *stubSnapshot) UpdateSnapshot(_ context.Context, _ string) error { return s.err }

func toolsCertService(db storage.Storage, store *stubCertStore, snapshot *stubSnapshot) *certificate.CertificateService {
	return certificate.NewCertificateService(certificate.Dependencies{
		DB: db,
		ResolveXDS: func() *certificate.XDSTargets {
			return &certificate.XDSTargets{Store: store, Snapshot: snapshot}
		},
		Logger: toolsDiscard,
	})
}

func certToolsHandler(db storage.Storage, store *stubCertStore, snapshot *stubSnapshot) *McpHandler {
	h := newAuthzTestHandler(false)
	h.logger = toolsDiscard
	h.certificateService = toolsCertService(db, store, snapshot)
	return h
}

// certToolsHandlerWithIdentity is certToolsHandler with an encryption
// provider, so usage: identity certificates can be stored. log receives the
// handler's and the service's log lines.
func certToolsHandlerWithIdentity(t *testing.T, db storage.Storage, log *slog.Logger) *McpHandler {
	t.Helper()
	mgr := testEncryptionManager(t)
	h := newAuthzTestHandler(false)
	h.logger = log
	h.certificateService = certificate.NewCertificateService(certificate.Dependencies{
		DB: db,
		ResolveXDS: func() *certificate.XDSTargets {
			return &certificate.XDSTargets{Store: &stubCertStore{}, Snapshot: &stubSnapshot{}}
		},
		EncryptionManager: func() *encryption.ProviderManager { return mgr },
		Logger:            log,
	})
	return h
}

func TestManageCertificatesLifecycle(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{combined: []byte("0123456789")}, &stubSnapshot{})

	_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
	})
	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "create", res["operation"])
	assert.Equal(t, "Certificate stored and pushed to the router.", res["message"])
	created := toolsAs[CertificateResponse](t, res["certificate"])
	assert.Equal(t, "upstream-ca", created.Name)
	require.Len(t, db.certs, 1)

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list"})
	require.NoError(t, err)
	res = resultMap(t, out)
	assert.Equal(t, 1, res["count"])
	assert.Equal(t, len(validTestCert), res["totalBytes"])
	items := toolsAs[[]CertificateResponse](t, res["items"])
	require.Len(t, items, 1)
	assert.Equal(t, created.ID, items[0].ID)
	encoded, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "BEGIN CERTIFICATE", "list must never put stored PEM into the transcript")

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "reload", Confirm: true})
	require.NoError(t, err)
	assert.Equal(t, 10, resultMap(t, out)["totalBytes"])

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "delete", ID: created.ID, Confirm: true,
	})
	require.NoError(t, err)
	assert.Equal(t, created.ID, resultMap(t, out)["id"])
	assert.Empty(t, db.certs)
}

func TestManageCertificatesFailures(t *testing.T) {
	t.Run("invalid PEM", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "bad", Certificate: "not a certificate",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the Gateway refused the certificate: certificate upload is invalid:\n  1. certificate:")
	})

	// As on REST, a private key pasted into "certificate" is refused rather
	// than silently skipped and the chain stored.
	t.Run("a pasted private key is refused", func(t *testing.T) {
		db := NewMockStorage()
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
		withKey := validTestCert + "\n-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "upstream-ca", Certificate: withKey,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the private key belongs in privateKey")
		assert.Empty(t, db.certs, "nothing may be stored")
	})

	t.Run("name already taken", func(t *testing.T) {
		db := NewMockStorage()
		db.saveErr = fmt.Errorf("%w: certificate name", storage.ErrConflict)
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `a certificate named "upstream-ca" already exists`)
	})

	// The row is committed before the push, so the model is told not to repeat
	// the write and to retry only the push.
	t.Run("stored but not pushed", func(t *testing.T) {
		db := NewMockStorage()
		h := certToolsHandler(db, &stubCertStore{reloadErr: errors.New("boom")}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the change was stored but could not be pushed to the router")
		assert.Contains(t, err.Error(), "action=reload")
		assert.Len(t, db.certs, 1)
	})

	t.Run("reload push fails with nothing stored", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{err: errors.New("xds down")})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "reload", Confirm: true})
		require.Error(t, err)
		assert.Equal(t, "nothing changed: the trust store could not be pushed to the router (it failed at the snapshot stage)",
			err.Error())
	})

	t.Run("delete of an unknown id", func(t *testing.T) {
		h := certToolsHandler(NewMockStorage(), &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "delete", ID: "no-such-cert", Confirm: true,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `cannot delete certificate "no-such-cert": no certificate with that id`)
	})

	// As on REST, a certificate a deployed API trusts cannot be deleted, and
	// the error names that API so the model can remove the reference first.
	t.Run("delete refused while a deployed API trusts it", func(t *testing.T) {
		db := NewMockStorage()
		db.certs = []*models.StoredCertificate{{
			UUID: "cert-1", Name: "upstream-ca", Usage: models.CertificateUsageUpstream,
			Certificate: []byte(validTestCert),
		}}
		tls := map[string]interface{}{"trustedCAs": []interface{}{"upstream-ca"}}
		defs := []api.UpstreamDefinition{{Tls: &tls}}
		db.configs["cfg-orders"] = &models.StoredConfig{
			UUID: "cfg-orders", Handle: "orders", Kind: models.KindRestApi, DesiredState: models.StateDeployed,
			Configuration: api.RestAPI{Spec: api.APIConfigData{UpstreamDefinitions: &defs}},
		}
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})

		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "delete", ID: "cert-1", Confirm: true,
		})

		require.Error(t, err)
		assert.Equal(t, `cannot delete certificate "cert-1": certificate 'upstream-ca' is named by 1 deployed API; remove those references first:`+"\n"+
			`  1. API "orders" (spec.upstreamDefinitions[0].tls.trustedCAs[0])`, err.Error())
		assert.Len(t, db.certs, 1, "nothing is deleted")
	})

	t.Run("list failure is generic", func(t *testing.T) {
		db := NewMockStorage()
		db.getErr = errors.New("database is locked")
		h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list"})
		require.Error(t, err)
		assert.Equal(t, "the Gateway could not list the certificate", err.Error())
	})
}

func TestManageCertificatesDownstream(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})

	t.Run("a client authority defaults to role client", func(t *testing.T) {
		_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "client-ca", Usage: "downstream",
			Certificate: string(pki.NewRootCA(t, "MCP Client CA").PEM()),
		})
		require.NoError(t, err)
		created := toolsAs[CertificateResponse](t, resultMap(t, out)["certificate"])
		assert.Equal(t, models.CertificateUsageDownstream, created.Usage)
		assert.Equal(t, models.CertificateRoleClient, created.Role)
	})

	t.Run("a relay entry keeps its match", func(t *testing.T) {
		_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "relay-ca", Usage: "downstream", Role: "relay",
			Certificate: string(pki.NewRootCA(t, "MCP Relay CA").PEM()),
			Match:       &certificateMatchInput{URISANs: []string{"spiffe://example.org/proxy"}},
		})
		require.NoError(t, err)
		created := toolsAs[CertificateResponse](t, resultMap(t, out)["certificate"])
		assert.Equal(t, models.CertificateRoleRelay, created.Role)
		require.NotNil(t, created.Match)
		assert.Equal(t, []string{"spiffe://example.org/proxy"}, created.Match.URISANs)
	})

	// REST refuses an empty match object; the tool must too, rather than store
	// a relay entry its caller believed was narrowed.
	t.Run("an empty match is refused", func(t *testing.T) {
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
			Action: "apply", Name: "relay-empty", Usage: "downstream", Role: "relay",
			Certificate: string(pki.NewRootCA(t, "MCP Empty CA").PEM()),
			Match:       &certificateMatchInput{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "match: match must list uriSANs or dnsSANs, or be omitted")
	})

	t.Run("list filters by usage and reports references", func(t *testing.T) {
		require.NoError(t, db.SaveCertificate(&models.StoredCertificate{
			UUID: "up-1", Name: "upstream-only", Usage: models.CertificateUsageUpstream, Certificate: []byte(validTestCert),
		}))
		_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list", Usage: "downstream"})
		require.NoError(t, err)
		items := toolsAs[[]CertificateResponse](t, resultMap(t, out)["items"])
		require.Len(t, items, 2)
		for _, item := range items {
			assert.Equal(t, models.CertificateUsageDownstream, item.Usage)
			assert.NotNil(t, item.ReferencedByApis)
		}
	})

	t.Run("an unknown usage filter is refused", func(t *testing.T) {
		_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list", Usage: "sideways"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "the Gateway refused the listing: invalid usage filter")
	})
}

func TestManageCertificatesIdentity(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	db := NewMockStorage()
	h := certToolsHandlerWithIdentity(t, db, log)

	first := gatewayIdentityLeaf(t, "mcp-identity-v1")
	second := gatewayIdentityLeaf(t, "mcp-identity-v2")
	var transcript []string // every output and error the model would see
	record := func(out any, err error) {
		if out != nil {
			encoded, mErr := json.Marshal(out)
			require.NoError(t, mErr)
			transcript = append(transcript, string(encoded))
		}
		if err != nil {
			transcript = append(transcript, err.Error())
		}
	}

	_, out, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "gw-identity", Usage: "identity", Certificate: string(first.PEM()),
	})
	record(out, err)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "privateKey: both certificate and privateKey are required for usage: identity")

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "gw-identity", Usage: "identity", Role: "default",
		Certificate: string(first.PEM()), PrivateKey: string(first.KeyPEM()),
	})
	record(out, err)
	require.NoError(t, err)
	created := toolsAs[CertificateResponse](t, resultMap(t, out)["certificate"])
	assert.Equal(t, models.CertificateUsageIdentity, created.Usage)
	assert.Equal(t, models.CertificateRoleDefault, created.Role)
	assert.NotEmpty(t, created.KeyAlgorithm)
	assert.Equal(t, 1, created.ChainLength)

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: created.ID, Confirm: true,
		Certificate: string(second.PEM()), PrivateKey: string(second.KeyPEM()),
	})
	record(out, err)
	require.NoError(t, err)
	res := resultMap(t, out)
	assert.Equal(t, "update", res["operation"])
	rotated := toolsAs[CertificateResponse](t, res["certificate"])
	assert.Equal(t, created.ID, rotated.ID, "rotation keeps the id")
	assert.Equal(t, "gw-identity", rotated.Name, "rotation keeps the name")
	assert.Equal(t, models.CertificateRoleDefault, rotated.Role, "rotation keeps the role")
	assert.Contains(t, rotated.Subject, "mcp-identity-v2")

	_, out, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{Action: "list", Usage: "identity"})
	record(out, err)
	require.NoError(t, err)

	// The key must never reach the model or the log, on success or failure.
	keyLine := strings.Split(string(second.KeyPEM()), "\n")[1]
	for _, text := range append(transcript, logs.String()) {
		assert.NotContains(t, text, "PRIVATE KEY")
		assert.NotContains(t, text, keyLine)
	}
}

func TestManageCertificatesUpdateFailures(t *testing.T) {
	db := NewMockStorage()
	db.certs = []*models.StoredCertificate{{
		UUID: "up-1", Name: "upstream-ca", Usage: models.CertificateUsageUpstream, Certificate: []byte(validTestCert),
	}}
	h := certToolsHandlerWithIdentity(t, db, toolsDiscard)
	leaf := gatewayIdentityLeaf(t, "mcp-update-failures")

	_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: "up-1", Confirm: true, Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage: only usage: identity certificates can be updated")

	_, _, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: "no-such-cert", Confirm: true, Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})
	require.Error(t, err)
	assert.Equal(t, `no certificate with id "no-such-cert"; call this tool with action=list to find it`, err.Error())
}

// Without an encryption provider a gateway identity is never stored.
func TestManageCertificatesIdentityNeedsEncryption(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
	leaf := gatewayIdentityLeaf(t, "mcp-no-encryption")

	_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "gw-identity", Usage: "identity",
		Certificate: string(leaf.PEM()), PrivateKey: string(leaf.KeyPEM()),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no encryption provider configured")
	assert.Empty(t, db.certs)
}

// The certificate tool enforces the limit POST /certificates does, without
// stating it.
func TestManageCertificatesRequestSizeLimit(t *testing.T) {
	db := NewMockStorage()
	h := certToolsHandler(db, &stubCertStore{}, &stubSnapshot{})
	h.maxCertificateUploadBytes = 256

	_, _, err := h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "apply", Name: "upstream-ca", Certificate: validTestCert,
	})
	require.Error(t, err)
	assert.Equal(t, "the certificate request is too large for this Gateway; nothing was stored", err.Error())
	assert.Empty(t, db.certs)

	// As PUT /certificates/{id}, update checks the stored certificate before
	// the size, so an unknown id is reported as unknown, not as too large.
	_, _, err = h.manageCertificates(toolsSkippedCtx, nil, manageCertificatesInput{
		Action: "update", ID: "no-such-cert", Confirm: true, Certificate: validTestCert, PrivateKey: validTestCert,
	})
	require.Error(t, err)
	assert.Equal(t, `no certificate with id "no-such-cert"; call this tool with action=list to find it`, err.Error())
}

func TestMcpCertErrorMapsTheServiceErrors(t *testing.T) {
	err := mcpCertError(certActionApply, "ca", &certificate.ValidationError{
		Message: certificate.MsgUploadInvalid,
		Fields:  []certificate.FieldError{{Field: "name", Message: "name may contain only letters, digits, ., _ and -"}},
	})
	assert.Equal(t, "the Gateway refused the certificate: certificate upload is invalid:\n"+
		"  1. name: name may contain only letters, digits, ., _ and -", err.Error())

	err = mcpCertError(certActionDelete, "ca-id", &certificate.InUseError{
		Message:    "certificate 'ca' is named by 1 deployed API; remove those references first",
		References: []certificate.Reference{{Field: "spec.upstreamDefinitions[0].tls.trustedCAs[0]", API: "orders"}},
	})
	assert.Equal(t, `cannot delete certificate "ca-id": certificate 'ca' is named by 1 deployed API; remove those references first:`+"\n"+
		`  1. API "orders" (spec.upstreamDefinitions[0].tls.trustedCAs[0])`, err.Error())

	// The read failure behind a refused delete stays in the log.
	err = mcpCertError(certActionDelete, "ca-id", &certificate.ReferenceCheckError{
		Message: "Failed to verify certificate references", Cause: errors.New("database is locked"),
	})
	assert.Contains(t, err.Error(), "nothing was deleted")
	assert.NotContains(t, err.Error(), "database is locked")

	err = mcpCertError(certActionApply, "gw", &certificate.DefaultIdentityExistsError{Name: "first"})
	assert.Equal(t, "gateway identity first already has role: default; delete it before uploading another default identity", err.Error())

	err = mcpCertError(certActionApply, "gw", certificate.ErrIdentityStorageUnavailable)
	assert.Equal(t, "this Gateway has no encryption provider configured, so gateway identities cannot be stored", err.Error())

	err = mcpCertError(certActionApply, "gw", &certificate.EncryptionError{Cause: errors.New("kms down")})
	assert.NotContains(t, err.Error(), "kms down")
}
