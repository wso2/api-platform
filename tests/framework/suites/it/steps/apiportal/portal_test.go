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

package apiportal

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

func TestNewPortalClientIsBuiltOnce(t *testing.T) {
	require.Same(t, newPortalClient(), newPortalClient())
}

func TestTopLevelJSONArrayLength(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		want    int
		wantErr string
	}{
		{name: "empty array", body: `{"labels":[]}`, field: "labels", want: 0},
		{name: "populated array", body: `{"labels":["one","two"]}`, field: "labels", want: 2},
		{name: "missing field", body: `{}`, field: "labels", wantErr: `is absent`},
		{name: "non array", body: `{"labels":null}`, field: "labels", wantErr: `is not an array`},
		{name: "invalid JSON", body: `{`, field: "labels", wantErr: `not a JSON object`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := topLevelJSONArrayLength([]byte(tt.body), tt.field)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDevportalAPIHandle(t *testing.T) {
	tests := []struct {
		name              string
		platformAPIHandle string
		want              string
	}{
		{name: "typical handle", platformAPIHandle: "devportal-api-ab12cd34", want: "dp-devportal-api-ab12cd34"},
		{name: "empty handle", platformAPIHandle: "", want: "dp-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, devportalAPIHandle(tt.platformAPIHandle))
		})
	}
}

func TestDevportalAPIHandleDistinctFromPlatformAPIHandle(t *testing.T) {
	handle := "shared-name"
	require.NotEqual(t, handle, devportalAPIHandle(handle))
}

func TestValidWebhookSignature(t *testing.T) {
	secret := "test-signing-secret"
	body := []byte(`{"id":"application-123","name":"signed"}`)
	timestamp := "1710000000"
	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(append([]byte(timestamp+"."), body...))
	require.NoError(t, err)
	header := "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name   string
		secret string
		body   []byte
		header string
		want   bool
	}{
		{name: "valid raw payload", secret: secret, body: body, header: header, want: true},
		{name: "wrong secret", secret: "other-secret", body: body, header: header, want: false},
		{name: "altered raw payload", secret: secret, body: []byte(`{"id":"application-123","name":"changed"}`), header: header, want: false},
		{name: "malformed header", secret: secret, body: body, header: "v1=invalid", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, validWebhookSignature(tt.secret, tt.body, tt.header))
		})
	}
}

func TestDecryptWebhookField(t *testing.T) {
	secret := "test-webhook-secret"
	plaintext := "generated-api-key-value"
	key, err := hkdf.Key(sha3.New256, []byte(secret), nil, webhookFieldKeyInfo, 32)
	require.NoError(t, err)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	iv := []byte("012345678901")
	sealed := gcm.Seal(nil, iv, []byte(plaintext), nil)
	tagOffset := len(sealed) - gcm.Overhead()
	envelope := map[string]any{
		"iv":         base64.StdEncoding.EncodeToString(iv),
		"tag":        base64.StdEncoding.EncodeToString(sealed[tagOffset:]),
		"ciphertext": base64.StdEncoding.EncodeToString(sealed[:tagOffset]),
	}

	decrypted, err := decryptWebhookField(secret, envelope)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

// claimsTable builds a two-column table from name/value pairs.
func claimsTable(t *testing.T, pairs ...string) *godog.Table {
	t.Helper()
	type cell struct {
		Value string `json:"value"`
	}
	type row struct {
		Cells []cell `json:"cells"`
	}
	var rows []row
	for i := 0; i+1 < len(pairs); i += 2 {
		rows = append(rows, row{Cells: []cell{{pairs[i]}, {pairs[i+1]}}})
	}
	raw, err := json.Marshal(map[string]any{"rows": rows})
	require.NoError(t, err)
	var table godog.Table
	require.NoError(t, json.Unmarshal(raw, &table))
	return &table
}

func TestClaimsFromTableDecodesListsMapsAndQuotedStrings(t *testing.T) {
	claims, err := claimsFromTable(context.Background(), claimsTable(t,
		"sub", "ivy",
		"org_id", `["acme"]`,
		"org_map", `{"acme": {"id": "1"}}`,
		"padded", `"default "`,
		"org_name", "Acme Corp",
	))
	require.NoError(t, err)
	require.Equal(t, "ivy", claims["sub"])
	require.Equal(t, []any{"acme"}, claims["org_id"])
	require.Equal(t, map[string]any{"acme": map[string]any{"id": "1"}}, claims["org_map"])
	require.Equal(t, "default ", claims["padded"], "a quoted value keeps the space Gherkin would trim")
	require.Equal(t, "Acme Corp", claims["org_name"])
}

func TestClaimsFromTableRejectsMalformedTables(t *testing.T) {
	cases := map[string]*godog.Table{
		"nil table":    nil,
		"missing sub":  claimsTable(t, "org_id", "acme"),
		"empty sub":    claimsTable(t, "sub", ""),
		"empty name":   claimsTable(t, "sub", "u", " ", "value"),
		"duplicate":    claimsTable(t, "sub", "u", "sub", "v"),
		"invalid JSON": claimsTable(t, "sub", "u", "roles", "[unclosed"),
		"wrong columns": func() *godog.Table {
			table := claimsTable(t, "sub", "u")
			table.Rows[0].Cells = table.Rows[0].Cells[:1]
			return table
		}(),
	}
	for name, table := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := claimsFromTable(context.Background(), table)
			require.Error(t, err)
		})
	}
}

func TestPortalDatabaseBindsPlaceholdersForItsEngine(t *testing.T) {
	query := "SELECT uuid FROM organizations WHERE idp_ref_id = ? AND portal_id = ?"
	require.Equal(t, "SELECT uuid FROM organizations WHERE idp_ref_id = $1 AND portal_id = $2",
		(&portalDatabase{engine: components.Postgres}).bind(query))
	require.Equal(t, "SELECT uuid FROM organizations WHERE idp_ref_id = @p1 AND portal_id = @p2",
		(&portalDatabase{engine: components.SQLServer}).bind(query))
	require.Equal(t, query, (&portalDatabase{engine: components.SQLite}).bind(query))
	require.Equal(t, "SELECT 1", (&portalDatabase{engine: components.Postgres}).bind("SELECT 1"))
}

func TestBrowserRedirectsAreRoutedToTheirTarget(t *testing.T) {
	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		return u
	}
	require.True(t, isIdentityProvider(parse("https://testbench:3015/oauth2/authorize?org=a")))
	require.False(t, isIdentityProvider(parse("/api-portal/default/views/default")))
	require.False(t, isIdentityProvider(parse("http://api-portal-multi-organization:9543/api-portal/default/callback")))
	require.False(t, isIdentityProvider(parse("https://testbench.evil.example/oauth2/authorize")))

	require.Equal(t, "/api-portal/default/callback?code=c&state=s",
		portalTarget(parse("http://api-portal-multi-organization:9543/api-portal/default/callback?code=c&state=s")))
	require.Equal(t, "/api-portal/a%2Fb/views/default", portalTarget(parse("/api-portal/a%2Fb/views/default")))

	for status, want := range map[int]bool{
		http.StatusFound: true, http.StatusSeeOther: true, http.StatusMovedPermanently: true,
		http.StatusTemporaryRedirect: true, http.StatusOK: false, http.StatusNotModified: false,
	} {
		require.Equal(t, want, isRedirect(status), status)
	}
}

func withScenarioPartition(t *testing.T, partition string) context.Context {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	require.NoError(t, tcontext.Set(ctx, "apiPortalRunnerPartition", partition))
	return ctx
}

func TestBrowserStateIsScopedToTheScenario(t *testing.T) {
	s := &Steps{}
	ctx := withScenarioPartition(t, "scenario-one")
	require.NoError(t, s.setBrowserValue(ctx, browserCookieKey, "sam", "sid=1"))
	require.Equal(t, "sid=1", s.browserValue(ctx, browserCookieKey, "sam"))
	require.Empty(t, s.browserValue(ctx, browserCookieKey, "ada"), "each browser has its own state")
	require.Empty(t, s.browserValue(ctx, browserSessionKey, "sam"), "each kind of state has its own key")

	require.NoError(t, tcontext.Set(ctx, "apiPortalRunnerPartition", "scenario-two"))
	require.Empty(t, s.browserValue(ctx, browserCookieKey, "sam"), "a later scenario starts with a fresh browser")
}

func TestSinkPartitionsAreScenarioScopedPathSegments(t *testing.T) {
	s := &Steps{}
	ctx := withScenarioPartition(t, "api-portal-testbench-abc-1")
	partition, err := s.sinkPartition(ctx, "here")
	require.NoError(t, err)
	require.Equal(t, "api-portal-testbench-abc-1-here", partition)
	for _, bad := range []string{"", " ", "a/b", "a?b", "a#b"} {
		_, err := s.sinkPartition(ctx, bad)
		require.Error(t, err, bad)
	}
}

func TestRESTAPIMetadataIsPublishedJSON(t *testing.T) {
	var metadata map[string]any
	require.NoError(t, json.Unmarshal([]byte(restAPIMetadata("mt-api-1")), &metadata))
	require.Equal(t, "mt-api-1", metadata["id"])
	require.Equal(t, "PUBLISHED", metadata["status"])
	require.Equal(t, "REST", metadata["type"])
}

func TestTheIdentityProviderClientVerifiesTheProviderCertificate(t *testing.T) {
	pair := shared.IdentityProviderTLS()
	certificate, err := tls.X509KeyPair(pair.CertPEM, pair.PrivateKeyPEM)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	defer server.Close()

	client, err := newIdentityProviderClient()
	require.NoError(t, err)
	response, err := client.Do(context.Background(), httpx.Request{URL: server.URL}, 0, 0)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.StatusCode)

	other := httptest.NewTLSServer(http.NotFoundHandler())
	defer other.Close()
	_, err = client.Do(context.Background(), httpx.Request{URL: other.URL}, 0, 0)
	require.Error(t, err, "a certificate other than the provider's must not be trusted")
}

func TestAPIOffersPlan(t *testing.T) {
	body := []byte(`{"id":"api-1","status":"PUBLISHED","subscriptionPlans":[{"id":"gold"},{"id":"silver"}]}`)

	require.NoError(t, apiOffersPlan(body, "api-1", "silver"))
	require.ErrorContains(t, apiOffersPlan(body, "api-1", "bronze"), "without plan")
	require.ErrorContains(t, apiOffersPlan(body, "api-2", "gold"), "returned")
	require.NoError(t, apiOffersPlan([]byte(`{"id":"api-1","status":"DEPRECATED","subscriptionPlans":[{"id":"gold"}]}`),
		"api-1", "gold"))
	require.ErrorContains(t, apiOffersPlan([]byte(`not json`), "api-1", "gold"), "decoding")
}

func TestAPIHasField(t *testing.T) {
	body := []byte(`{"id":"api-1","status":"DEPRECATED","name":"Renamed"}`)

	require.NoError(t, apiHasField(body, "api-1", "status", "DEPRECATED"))
	require.NoError(t, apiHasField(body, "api-1", "name", "Renamed"))
	require.ErrorContains(t, apiHasField(body, "api-1", "status", "PUBLISHED"), `with status "DEPRECATED"`)
	require.ErrorContains(t, apiHasField(body, "api-1", "title", "x"), "want")
	require.ErrorContains(t, apiHasField([]byte(`not json`), "api-1", "status", "x"), "decoding")
}

func TestTrackPublishedPlanRemovesThePlanFromThePortalAtCleanup(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	reg := cleanup.NewRegistry(slog.Default())
	require.NoError(t, cleanup.Install(ctx, reg))
	s := &Steps{}

	require.NoError(t, s.trackPublishedPlan(ctx, "gold"))
	require.NoError(t, s.trackPublishedPlan(ctx, "silver"))

	var ids []string
	for _, res := range reg.Pending() {
		require.Equal(t, portalSubscriptionPlanKind.Name, res.Kind.Name)
		ids = append(ids, res.ID)
	}
	require.ElementsMatch(t, []string{"gold", "silver"}, ids)

	require.Error(t, s.trackPublishedPlan(context.Background(), "gold"), "without a cleanup registry nothing can be tracked")
}

func TestPortalProberTrustsOnlyThePortalCertificateAndFollowsRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/signin", http.StatusFound) })
	mux.HandleFunc("/signin", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	serve := func(pair *shared.KeyPairPEM) *httptest.Server {
		cert, err := tls.X509KeyPair(pair.CertPEM, pair.PrivateKeyPEM)
		require.NoError(t, err)
		server := httptest.NewUnstartedServer(mux)
		server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
		server.StartTLS()
		t.Cleanup(server.Close)
		return server
	}
	prober := newPortalProber()

	portal := serve(shared.APIPortalTLS())
	status, err := prober.Probe(context.Background(), portal.URL+"/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "the redirect to the sign-in page is followed")

	stranger, err := shared.SelfSignedCert(shared.APIPortalHost, []string{shared.APIPortalHost})
	require.NoError(t, err)
	_, err = prober.Probe(context.Background(), serve(stranger).URL+"/")
	require.Error(t, err, "a certificate that is not the portal's is refused even when it carries the portal's name")

	portal.Close()
	status, err = prober.Probe(context.Background(), portal.URL+"/")
	require.Error(t, err)
	require.Zero(t, status)
}

func TestPortalResourceKindGivesEachCollectionItsOwnDeleter(t *testing.T) {
	require.Equal(t, portalSubscriptionPlanKind, portalResourceKind("/subscription-plans"))
	require.Equal(t, portalMeterKind, portalResourceKind("/meters"))
	require.Equal(t, portalWebhookKind, portalResourceKind("/webhook-subscribers"))
	require.Equal(t, "api-portal-resource", portalResourceKind("/apis").Name)
	// An API goes before the plans it offers, and a plan before the meters it prices.
	require.Less(t, portalResourceKind("/apis").Order, portalSubscriptionPlanKind.Order)
	require.Less(t, portalSubscriptionPlanKind.Order, portalMeterKind.Order)
}

// billingFakes returns steps whose topology holds fakes of the testbench Stripe service, the API
// Portal and the control plane's sign-in. Paying checkout "cs-1" delivers one event, which the
// portal answers with deliveryStatus; reading the checkout finds it completed with subscription
// "sub-1", as the portal reports once it has confirmed the payment with Stripe.
func billingFakes(t *testing.T, deliveryStatus int) *Steps {
	t.Helper()
	serveTLS := func(certPEM, keyPEM []byte, handler http.HandlerFunc) *httptest.Server {
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		server := httptest.NewUnstartedServer(handler)
		server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
		server.StartTLS()
		t.Cleanup(server.Close)
		return server
	}
	writeJSON := func(w http.ResponseWriter, body any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}

	stripe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/billing/test/checkout-sessions/cs-1/complete" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"deliveries": []map[string]any{{
			"url":    "https://api-portal.6f1c2b9e-net:9543/api-portal/billing/stripe/webhooks/org-1",
			"status": deliveryStatus,
		}}})
	}))
	t.Cleanup(stripe.Close)
	portalPair := shared.APIPortalTLS()
	portal := serveTLS(portalPair.CertPEM, portalPair.PrivateKeyPEM, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != apiPrefix+"/checkout-sessions/cs-1" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"status": "COMPLETED", "subscriptionId": "sub-1"})
	})
	crypto := shared.ControlPlaneCrypto()
	controlPlane := serveTLS(crypto["certs/cert.pem"], crypto["certs/key.pem"], func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/portal/v0.9/auth/login" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]any{"token": "token"})
	})

	set := components.NewSet()
	add := func(name string, endpoint components.Endpoint, server *httptest.Server) {
		address, err := url.Parse(server.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(address.Port())
		require.NoError(t, err)
		definition := &components.Definition{Name: name, Alias: name, Endpoints: []components.Endpoint{endpoint}}
		inst, err := components.NewInstance(definition, 0, 1, "127.0.0.1", map[int]int{endpoint.Port: port})
		require.NoError(t, err)
		require.NoError(t, set.Add(inst))
	}
	add("testbench", components.Endpoint{Name: "stripe", Port: 3022, Scheme: "http"}, stripe)
	add(portalService, components.Endpoint{Name: "https", Port: 9543, Scheme: "https"}, portal)
	add("platform-api", components.Endpoint{Name: "https", Port: 9243, Scheme: "https"}, controlPlane)

	return &Steps{
		topo:   &runtime.Topology{Block: &topology.ResolvedBlock{Name: "billing"}, Instances: set},
		client: newPortalClient(),
	}
}

func TestPayCheckoutEndsTheSubscriptionAtCleanupEvenWhenStripesEventIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		wantErr string
	}{
		{name: "every event is accepted", status: http.StatusOK},
		{name: "an event is refused", status: http.StatusNotFound,
			wantErr: "was not accepted by https://api-portal.6f1c2b9e-net:9543/api-portal/billing/stripe/webhooks/org-1: status 404"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := billingFakes(t, tc.status)
			ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
			reg := cleanup.NewRegistry(slog.Default())
			require.NoError(t, cleanup.Install(ctx, reg))

			err := s.payCheckout(ctx, "cs-1", "developer", "subscriptionId")

			pending := reg.Pending()
			require.Len(t, pending, 1, "the subscription is ended when the scenario ends")
			require.Equal(t, paidSubscriptionKind("developer").Name, pending[0].Kind.Name)
			require.Equal(t, "sub-1", pending[0].ID)
			stored, ok := tcontext.Get(ctx, "subscriptionId")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.False(t, ok, "a refused event fails the step before the subscription is stored")
				return
			}
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, "sub-1", stored)
		})
	}
}

func TestPayCheckoutNeedsACleanupRegistry(t *testing.T) {
	s := billingFakes(t, http.StatusOK)
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))

	err := s.payCheckout(ctx, "cs-1", "developer", "subscriptionId")

	require.Error(t, err, "a subscription that cannot be ended at cleanup fails the step")
	_, ok := tcontext.Get(ctx, "subscriptionId")
	require.False(t, ok)
}
