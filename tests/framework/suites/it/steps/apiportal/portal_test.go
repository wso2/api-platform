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
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/components"
	"github.com/wso2/api-platform/tests/framework/core/runtime"
	"github.com/wso2/api-platform/tests/framework/core/topology"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
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

func TestWithReplacedClaimKeepsHeaderAndSignature(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"u","name":"Ada"}`))
	token := "header." + payload + ".signature"

	forged, err := withReplacedClaim(token, "name", "Mallory")
	require.NoError(t, err)
	parts := strings.Split(forged, ".")
	require.Len(t, parts, 3)
	require.Equal(t, "header", parts[0])
	require.Equal(t, "signature", parts[2])
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(raw, &claims))
	require.Equal(t, map[string]any{"sub": "u", "name": "Mallory"}, claims)

	added, err := withReplacedClaim(token, "iss", "https://other")
	require.NoError(t, err)
	raw, err = base64.RawURLEncoding.DecodeString(strings.Split(added, ".")[1])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &claims))
	require.Equal(t, "https://other", claims["iss"], "a claim the token lacks is added")

	for name, malformed := range map[string]string{
		"not a JWS":       "only.two",
		"bad base64":      "h.!!!.s",
		"payload not map": "h." + base64.RawURLEncoding.EncodeToString([]byte(`[1]`)) + ".s",
		"null payload":    "h." + base64.RawURLEncoding.EncodeToString([]byte(`null`)) + ".s",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := withReplacedClaim(malformed, "name", "x")
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
	body := []byte(`{"id":"api-1","status":"DEPRECATED","name":"Renamed","type":"GRAPHQL"}`)

	require.NoError(t, apiHasField(body, "api-1", "status", "DEPRECATED"))
	require.NoError(t, apiHasField(body, "api-1", "name", "Renamed"))
	require.NoError(t, apiHasField(body, "api-1", "type", "GRAPHQL"))
	require.ErrorContains(t, apiHasField(body, "api-1", "status", "PUBLISHED"), `with status "DEPRECATED"`)
	require.ErrorContains(t, apiHasField(body, "api-1", "title", "x"), "want")
	require.ErrorContains(t, apiHasField([]byte(`not json`), "api-1", "status", "x"), "decoding")
}

func TestFetchListedAPIRejectsAnUnknownListing(t *testing.T) {
	_, _, err := (&Steps{}).fetchListedAPI(context.Background(), "LLM provider", "api-1")
	require.ErrorContains(t, err, "unsupported API Portal listing")
	require.Equal(t, "/apis/", listingCollections["API"])
	require.Equal(t, "/mcp-servers/", listingCollections["MCP server"])
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
	require.Equal(t, portalMCPServerKind, portalResourceKind("/mcp-servers"))
	require.Equal(t, portalOAuth2KeyKind, portalResourceKind("/oauth2-keys"))
	require.Equal(t, portalApplicationKind, portalResourceKind("/applications"))
	require.Equal(t, portalSubscriptionKind, portalResourceKind("/subscriptions"))
	// A subscription is removed before the API it is for, and before its plan.
	require.Less(t, portalResourceKind("/subscriptions").Order, portalResourceKind("/apis").Order)
	require.Less(t, portalResourceKind("/subscriptions").Order, portalSubscriptionPlanKind.Order)
	// An OAuth2 key is removed before the key manager that issued it.
	require.Less(t, portalOAuth2KeyKind.Order, portalResourceKind("/key-managers").Order)
	// Each view's workflows get a deleter of their own: the deletion path names the view.
	require.Equal(t, portalWorkflowKind.Order, portalResourceKind("/views/partners/api-workflows").Order)
	require.NotEqual(t, portalResourceKind("/views/partners/api-workflows"), portalResourceKind("/views/others/api-workflows"))
	require.Equal(t, "api-portal-view", portalResourceKind("/views").Name)
	// A workflow is removed before the view it belongs to.
	require.Less(t, portalWorkflowKind.Order, portalResourceKind("/views").Order)
	// An API goes before the plans it offers, and a plan before the meters it prices.
	require.Less(t, portalResourceKind("/apis").Order, portalSubscriptionPlanKind.Order)
	require.Less(t, portalSubscriptionPlanKind.Order, portalMeterKind.Order)
}

func TestAPIKeyRevokeCleanupRevokesTheKeyAsItsOwner(t *testing.T) {
	request, err := apiKeyRevokeCleanup("portal api/1", "developer")
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, request.method)
	require.Equal(t, "/apis/portal%20api%2F1/api-keys/revoke", request.path)
	require.Equal(t, "developer", request.role)
	require.Equal(t, map[string]any{"keyId": "key-1"}, request.payload("key-1"))

	_, err = apiKeyRevokeCleanup("", "developer")
	require.Error(t, err)
	_, err = apiKeyRevokeCleanup("   ", "developer")
	require.Error(t, err)
	_, err = apiKeyRevokeCleanup("api-1", "")
	require.Error(t, err)
	_, err = apiKeyRevokeCleanup("api-1", "intruder")
	require.Error(t, err)
}

func TestAPIKeyKindOfSeparatesTheKeysOfEachAPI(t *testing.T) {
	first, second := apiKeyKindOf("api-1"), apiKeyKindOf("api-2")
	require.NotEqual(t, first.Name, second.Name)
	require.Equal(t, apiKeyKindOf("api-1"), first)
	require.Equal(t, portalAPIKeyKind.Order, first.Order)
	require.NotEqual(t, portalAPIKeyKind.Name, first.Name)
}

func TestPortalCleanupRoleDeletesOwnerScopedResourcesAsTheirCreator(t *testing.T) {
	require.Equal(t, "developer", portalCleanupRole("/applications", "developer"))
	require.Equal(t, "developer", portalCleanupRole("/oauth2-keys", "developer"))
	require.Equal(t, "admin", portalCleanupRole("/applications", "admin"))
	require.Empty(t, portalCleanupRole("/key-managers", "developer"))
	require.Empty(t, portalCleanupRole("/webhook-subscribers", "admin"))
	require.Empty(t, portalCleanupRole("", ""))
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

func TestCreateApplicationWithTokenRemovesItWithTheSameToken(t *testing.T) {
	type call struct{ method, path, auth, contentType string }
	var calls []call
	created := http.StatusCreated
	pair := shared.APIPortalTLS()
	cert, err := tls.X509KeyPair(pair.CertPEM, pair.PrivateKeyPEM)
	require.NoError(t, err)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, call{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type")})
		if r.Method == http.MethodPost {
			w.WriteHeader(created)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	t.Cleanup(server.Close)
	address, err := url.Parse(server.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(address.Port())
	require.NoError(t, err)
	definition := &components.Definition{Name: portalService, Alias: portalService,
		Endpoints: []components.Endpoint{{Name: "https", Port: 9543, Scheme: "https"}}}
	inst, err := components.NewInstance(definition, 0, 1, "127.0.0.1", map[int]int{9543: port})
	require.NoError(t, err)
	set := components.NewSet()
	require.NoError(t, set.Add(inst))
	s := &Steps{topo: &runtime.Topology{Block: &topology.ResolvedBlock{Name: "apps"}, Instances: set}, client: newPortalClient()}

	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	reg := cleanup.NewRegistry(slog.Default())
	require.NoError(t, cleanup.Install(ctx, reg))
	require.NoError(t, tcontext.Set(ctx, "token", "user-token"))

	require.NoError(t, s.createApplicationWithToken(ctx, portalService, "token", "appId"))
	id, err := tcontext.ResolveString(ctx, "appId")
	require.NoError(t, err)
	require.Equal(t, []call{{http.MethodPost, apiPrefix + "/applications", "Bearer user-token", "application/json"}}, calls)
	require.Len(t, reg.Pending(), 1)

	require.NoError(t, reg.Sweep(ctx))
	require.Len(t, calls, 2)
	require.Equal(t, call{http.MethodDelete, apiPrefix + "/applications/" + id, "Bearer user-token", ""}, calls[1],
		"an application is removed by the user who created it")

	created = http.StatusForbidden
	require.ErrorContains(t, s.createApplicationWithToken(ctx, portalService, "token", "other"), "403")
	require.Empty(t, reg.Pending(), "a refused create registers nothing")
	require.ErrorContains(t, s.createApplicationWithToken(ctx, portalService, "missing", "other"), "unavailable")
}

// TestCleanupDeletesEachResourceAsItsOwnCreator guards the single deleter a kind keeps: it must
// take the role from each resource, not from whichever registration came last.
func TestCleanupDeletesEachResourceAsItsOwnCreator(t *testing.T) {
	serve := func(certPEM, keyPEM []byte, handler http.HandlerFunc) int {
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		require.NoError(t, err)
		server := httptest.NewUnstartedServer(handler)
		server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
		server.StartTLS()
		t.Cleanup(server.Close)
		address, err := url.Parse(server.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(address.Port())
		require.NoError(t, err)
		return port
	}
	developer, err := portalActor("developer")
	require.NoError(t, err)
	deleted := map[string]string{}
	var mu sync.Mutex
	portalPair := shared.APIPortalTLS()
	portalPort := serve(portalPair.CertPEM, portalPair.PrivateKeyPEM, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		deleted[strings.TrimPrefix(r.URL.Path, apiPrefix+"/applications/")] = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
	crypto := shared.ControlPlaneCrypto()
	controlPlanePort := serve(crypto["certs/cert.pem"], crypto["certs/key.pem"], func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		token := "other-token"
		if strings.Contains(string(body), developer.Username) {
			token = "developer-token"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"token": token})
	})
	set := components.NewSet()
	for name, port := range map[string]struct {
		internal, published int
	}{portalService: {9543, portalPort}, "platform-api": {9243, controlPlanePort}} {
		definition := &components.Definition{Name: name, Alias: name,
			Endpoints: []components.Endpoint{{Name: "https", Port: port.internal, Scheme: "https"}}}
		inst, err := components.NewInstance(definition, 0, 1, "127.0.0.1", map[int]int{port.internal: port.published})
		require.NoError(t, err)
		require.NoError(t, set.Add(inst))
	}
	s := &Steps{topo: &runtime.Topology{Block: &topology.ResolvedBlock{Name: "owners"}, Instances: set}, client: newPortalClient()}
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	reg := cleanup.NewRegistry(slog.Default())
	require.NoError(t, cleanup.Install(ctx, reg))

	for id, role := range map[string]string{"dev-app": "developer", "pub-app": "publisher"} {
		require.NoError(t, s.registerPortalResource(ctx, portalApplicationKind, id, cleanup.ScenarioScope,
			portalCleanupRequest{method: http.MethodDelete, path: "/applications", role: role}))
	}
	require.NoError(t, reg.Sweep(ctx))
	require.Equal(t, map[string]string{"dev-app": "Bearer developer-token", "pub-app": "Bearer other-token"}, deleted)
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

func TestRenderAPIMetadataFromTheCanonicalTemplate(t *testing.T) {
	s := &Steps{featureRoot: filepath.Join("..", "..")}
	ctx := context.Background()

	metadata, err := s.renderAPIMetadata(ctx, "resources/templates/api-portal-api.yaml", claimsTable(t,
		"id", "orders-api",
		"name", "Orders API",
		"tags", `["orders"]`,
		"endPoints.sandboxURL", "null",
	))
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"orders-api","name":"Orders API","version":"v1.0","type":"REST",
		"status":"PUBLISHED","tags":["orders"],
		"endPoints":{"productionURL":"https://backend.example.invalid"}}`, string(metadata))

	withoutType, err := s.renderAPIMetadata(ctx, "resources/templates/api-portal-api.yaml", claimsTable(t,
		"id", "orders-api", "name", "Orders API", "type", "null"))
	require.NoError(t, err)
	require.NotContains(t, string(withoutType), `"type"`, "a null value leaves the field out")

	_, err = s.renderAPIMetadata(ctx, "resources/templates/api-portal-api.yaml", claimsTable(t, "id", "orders-api"))
	require.Error(t, err, "the template's required name has no value")
	_, err = s.renderAPIMetadata(ctx, "resources/templates/absent.yaml", claimsTable(t, "id", "x", "name", "x"))
	require.Error(t, err)
}

// TestWithoutNullFields covers removing null fields at each depth and refusing a non-object.
func TestWithoutNullFields(t *testing.T) {
	got, err := withoutNullFields([]byte(`{"a":null,"b":{"c":null,"d":1},"e":[{"f":null,"g":"h"}],"i":""}`))
	require.NoError(t, err)
	require.JSONEq(t, `{"b":{"d":1},"e":[{"g":"h"}],"i":""}`, string(got), "only null values are removed")

	for _, document := range []string{``, `[]`, `"text"`, `{`} {
		_, err := withoutNullFields([]byte(document))
		require.Error(t, err, document)
	}
}

// TestAPIDefinitionPart covers the default, named and omitted definition choices.
func TestAPIDefinitionPart(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		definition, wantFile string
		wantContent          bool
	}{
		{definition: "", wantFile: "definition.json", wantContent: true},
		{definition: "rest", wantFile: "definition.json", wantContent: true},
		{definition: "GraphQL", wantFile: "schema.graphql", wantContent: true},
		{definition: "not an OpenAPI document", wantFile: "definition.json", wantContent: true},
		{definition: " None ", wantFile: "", wantContent: false},
	} {
		t.Run(tc.definition, func(t *testing.T) {
			content, filename, _, err := apiDefinitionPart(ctx, tc.definition)
			require.NoError(t, err)
			require.Equal(t, tc.wantFile, filename)
			require.Equal(t, tc.wantContent, len(content) > 0)
		})
	}
}

// TestCreatedAPIID covers the reported and requested id sources and the case with neither.
func TestCreatedAPIID(t *testing.T) {
	metadata := []byte(`{"id":"requested"}`)
	for _, tc := range []struct {
		name, body, metadata, want string
		wantErr                    bool
	}{
		{name: "reported id", body: `{"id":"reported"}`, metadata: string(metadata), want: "reported"},
		{name: "no reported id", body: `{}`, metadata: string(metadata), want: "requested"},
		{name: "empty body", body: ``, metadata: string(metadata), want: "requested"},
		{name: "no id anywhere", body: `{}`, metadata: `{}`, wantErr: true},
		{name: "no id in malformed body or metadata", body: `{`, metadata: `{}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := createdAPIID([]byte(tc.body), []byte(tc.metadata))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCreatedAPIIDKeepsTheRequestedIDWhenTheResponseCannotBeDecoded keeps an accepted API
// registrable when its response is unusable.
func TestCreatedAPIIDKeepsTheRequestedIDWhenTheResponseCannotBeDecoded(t *testing.T) {
	for name, body := range map[string]string{"malformed": `{`, "non-string id": `{"id":7}`} {
		t.Run(name, func(t *testing.T) {
			id, err := createdAPIID([]byte(body), []byte(`{"id":"requested"}`))
			require.Error(t, err)
			require.Equal(t, "requested", id)
		})
	}
}

func TestPublishedResponse(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	_, err := publishedResponse(ctx)
	require.Error(t, err, "no response has been published yet")

	require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, &httpx.Response{StatusCode: http.StatusCreated}))
	response, err := publishedResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, response.StatusCode)

	require.NoError(t, tcontext.Set(ctx, httpx.ResponseKey, "not a response"))
	_, err = publishedResponse(ctx)
	require.Error(t, err)
}

// TestTableJSONValues covers the typed values and malformed rows of a values table.
func TestTableJSONValues(t *testing.T) {
	values, err := tableJSONValues(context.Background(), claimsTable(t,
		"displayName", "Orders",
		"labels", `["a", "b"]`,
		"pricing", `{"currency": "USD"}`,
		"note", "[not json",
		"padded", `  {"k": 1}  `,
	))
	require.NoError(t, err)
	require.Equal(t, "Orders", values["displayName"])
	require.Equal(t, []any{"a", "b"}, values["labels"])
	require.Equal(t, map[string]any{"currency": "USD"}, values["pricing"])
	require.Equal(t, "[not json", values["note"], "text that only looks like JSON stays a string")
	require.Equal(t, map[string]any{"k": float64(1)}, values["padded"])

	for name, table := range map[string]*godog.Table{
		"nil table":     nil,
		"empty table":   {},
		"empty key":     claimsTable(t, " ", "value"),
		"duplicate key": claimsTable(t, "id", "a", "id", "b"),
		"wrong columns": func() *godog.Table {
			table := claimsTable(t, "id", "a")
			table.Rows[0].Cells = table.Rows[0].Cells[:1]
			return table
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tableJSONValues(context.Background(), table)
			require.Error(t, err)
		})
	}
}

// TestAPICollection covers the collections a template step may name and refuses any other.
func TestAPICollection(t *testing.T) {
	for _, tc := range []struct {
		given, want string
		kind        cleanup.Kind
	}{
		{given: "", want: "/apis", kind: portalAPIKind},
		{given: "/apis", want: "/apis", kind: portalAPIKind},
		{given: " /mcp-servers ", want: "/mcp-servers", kind: portalMCPServerKind},
	} {
		collection, kind, err := apiCollection(tc.given)
		require.NoError(t, err, tc.given)
		require.Equal(t, tc.want, collection)
		require.Equal(t, tc.kind, kind)
	}
	for _, refused := range []string{"/labels", "/apis/x", "apis", "/../admin"} {
		_, _, err := apiCollection(refused)
		require.Error(t, err, refused)
	}
	require.NotEqual(t, portalAPIKind.Name, portalMCPServerKind.Name,
		"a kind keeps one deleter, so REST APIs and MCP servers each need their own")
}

func artifactContext(t *testing.T) context.Context {
	t.Helper()
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("scenario"))
	require.NoError(t, tcontext.Set(ctx, "apiId", "artifact-api"))
	return ctx
}

// artifactEntries returns the entries of a generated artifact archive by name.
func artifactEntries(t *testing.T, archive []byte) map[string]*zip.File {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	entries := map[string]*zip.File{}
	for _, file := range reader.File {
		entries[file.Name] = file
	}
	return entries
}

func artifactEntryText(t *testing.T, file *zip.File) string {
	t.Helper()
	reader, err := file.Open()
	require.NoError(t, err)
	defer reader.Close()
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(content)
}

func TestArtifactZipRequiresAnAPIID(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("scenario"))
	_, err := artifactZip(ctx, "valid")
	require.Error(t, err)
}

// TestArtifactZipUnsafeAndInvalidKinds covers the archive shapes the portal must refuse.
func TestArtifactZipUnsafeAndInvalidKinds(t *testing.T) {
	ctx := artifactContext(t)
	for _, tc := range []struct {
		kind, entry, contains string
	}{
		{kind: "absolute-path", entry: "/tmp/apip-absolute-entry.txt"},
		{kind: "mcp-type", entry: "api.yaml", contains: "type: MCP"},
		{kind: "invalid-status", entry: "api.yaml", contains: "status: BOGUS"},
		{kind: "valid", entry: "api.yaml", contains: "status: PUBLISHED"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			archive, err := artifactZip(ctx, tc.kind)
			require.NoError(t, err)
			entries := artifactEntries(t, archive)
			require.Contains(t, entries, "definition.json")
			require.Contains(t, entries, tc.entry)
			if tc.contains != "" {
				require.Contains(t, artifactEntryText(t, entries[tc.entry]), tc.contains)
			}
		})
	}
}

func TestArtifactZipValidKindIsARESTAPI(t *testing.T) {
	archive, err := artifactZip(artifactContext(t), "valid")
	require.NoError(t, err)
	metadata := artifactEntryText(t, artifactEntries(t, archive)["api.yaml"])
	require.Contains(t, metadata, "type: REST")
	require.Contains(t, metadata, "name: artifact-api")
}

func TestArtifactZipNotAZipIsNotAnArchive(t *testing.T) {
	archive, err := artifactZip(artifactContext(t), "not-a-zip")
	require.NoError(t, err)
	require.NotEmpty(t, archive)
	_, err = zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.Error(t, err)
}

func TestPortalDefinitionTitledREST(t *testing.T) {
	content, filename, contentType, err := portalDefinition("rest titled Kumquat Ledger")
	require.NoError(t, err)
	require.Equal(t, "definition.json", filename)
	require.Equal(t, "application/json", contentType)
	var document struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title string `json:"title"`
		} `json:"info"`
		Paths map[string]any `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(content, &document))
	require.Equal(t, "3.0.3", document.OpenAPI)
	require.Equal(t, "Kumquat Ledger", document.Info.Title)
	require.Contains(t, document.Paths, "/ping")

	_, _, _, err = portalDefinition("rest titled   ")
	require.Error(t, err)

	content, _, _, err = portalDefinition("rest")
	require.NoError(t, err)
	require.Equal(t, portalRESTDefinition, string(content))
}

// TestPortalPublicURLKeepsThePathAsWritten ensures a route under test reaches the portal without
// normalization.
func TestPortalPublicURLKeepsThePathAsWritten(t *testing.T) {
	base := "https://portal.test"
	for _, path := range []string{"/health", "/api-portal/health", "/nope", "/api/v0.9/apis", "/api-portal/default/views/default"} {
		require.Equal(t, base+path, portalPublicURL(base, path))
	}
	steps := &Steps{}
	require.Equal(t, base+apiPrefix+"/apis", steps.portalRequestURL(base, "/apis"))
	require.Equal(t, base+"/health", steps.portalRequestURL(base, "/health"))
}

// zipEntryNames returns the entry names of a generated archive.
func zipEntryNames(t *testing.T, archive []byte) []string {
	t.Helper()
	names := make([]string, 0)
	for name := range artifactEntries(t, archive) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestContentZipVariants covers the entries each content archive variant holds.
func TestContentZipVariants(t *testing.T) {
	for _, tc := range []struct {
		variant string
		want    []string
	}{
		{"default", []string{"bundle/web/api-content.hbs", "bundle/web/api-icon.png"}},
		{"image-metadata", []string{"bundle/web/api-content.hbs", "bundle/web/api-icon.png"}},
		{"alternate", []string{"bundle/web/api-content.hbs", "bundle/web/api-icon.png"}},
		{"with-docs", []string{"bundle/docs/HowTo/setup.md", "bundle/docs/overview.md", "bundle/web/api-content.hbs", "bundle/web/api-icon.png"}},
		{"skips", []string{"bundle/web/api-content.hbs", "bundle/web/api-icon.png", "bundle/web/nested/deep.md", "bundle/web/run.sh"}},
		{"no-content-dirs", []string{"bundle/readme.txt"}},
		{"replace-and-add", []string{"bundle/docs/HowTo/extra.md", "bundle/web/api-icon.png"}},
	} {
		t.Run(tc.variant, func(t *testing.T) {
			archive, err := contentZip(tc.variant)
			require.NoError(t, err)
			require.Equal(t, tc.want, zipEntryNames(t, archive))
		})
	}
}

func TestContentZipIconsDifferBetweenDefaultAndAlternate(t *testing.T) {
	read := func(variant string) string {
		archive, err := contentZip(variant)
		require.NoError(t, err)
		return artifactEntryText(t, artifactEntries(t, archive)["bundle/web/api-icon.png"])
	}
	want, err := hex.DecodeString(contentIconPNG)
	require.NoError(t, err)
	require.Equal(t, string(want), read("default"))
	alternate, err := hex.DecodeString(contentAlternateIconPNG)
	require.NoError(t, err)
	require.Equal(t, string(alternate), read("alternate"))
	require.Equal(t, string(alternate), read("replace-and-add"))
	require.NotEqual(t, read("default"), read("alternate"))
}

func TestContentZipDocsKeepTheirText(t *testing.T) {
	archive, err := contentZip("with-docs")
	require.NoError(t, err)
	entries := artifactEntries(t, archive)
	require.Equal(t, contentHowToDoc, artifactEntryText(t, entries["bundle/docs/HowTo/setup.md"]))
	require.Equal(t, contentOverviewDoc, artifactEntryText(t, entries["bundle/docs/overview.md"]))
	require.Equal(t, contentLandingPage, artifactEntryText(t, entries["bundle/web/api-content.hbs"]))
}

func TestContentZipRejectsAnUnknownVariant(t *testing.T) {
	_, err := contentZip("no-such-variant")
	require.Error(t, err)
	_, err = contentZip("")
	require.Error(t, err)
}

// TestThemeZipVariants covers the entries each theme archive variant holds.
func TestThemeZipVariants(t *testing.T) {
	themed := []string{"theme/images/brand-mark.png", "theme/styles/main.css"}
	for _, tc := range []struct {
		variant string
		want    []string
	}{
		{"default", themed},
		{"favicon", []string{"theme/images/favicon.ico", "theme/styles/main.css"}},
		{"new", []string{"theme/images/new.png", "theme/styles/main.css"}},
		{"old", []string{"theme/images/old.png", "theme/styles/main.css"}},
		{"unsupported", []string{"theme/evil.exe"}},
		{"scripts-only", []string{"theme/scripts/app.js"}},
		{"with-script", append(append([]string{}, themed...), "theme/scripts/app.js")},
		{"script-in-template", []string{"theme/images/brand-mark.png", "theme/partials/header.hbs", "theme/styles/main.css"}},
		{"zip-slip", []string{"../outside.css", "theme/images/brand-mark.png", "theme/styles/main.css"}},
		{"oversized", []string{"theme/images/brand-mark.png", "theme/images/large.png", "theme/styles/main.css"}},
	} {
		t.Run(tc.variant, func(t *testing.T) {
			archive, err := themeZip(tc.variant)
			require.NoError(t, err)
			want := append([]string{}, tc.want...)
			sort.Strings(want)
			require.Equal(t, want, zipEntryNames(t, archive))
		})
	}
}

func TestThemeZipOversizedIsPastTheUploadLimitAndScriptIsInTheTemplate(t *testing.T) {
	archive, err := themeZip("oversized")
	require.NoError(t, err)
	require.Greater(t, len(archive), 10<<20)
	require.EqualValues(t, themeOversizedBytes, artifactEntries(t, archive)["theme/images/large.png"].UncompressedSize64)

	archive, err = themeZip("script-in-template")
	require.NoError(t, err)
	require.Contains(t, artifactEntryText(t, artifactEntries(t, archive)["theme/partials/header.hbs"]), "<script>")
}

func TestThemeZipRejectsAnUnknownVariant(t *testing.T) {
	_, err := themeZip("no-such-theme")
	require.Error(t, err)
}

// TestCSRFToken covers reading the token from a Cookie header, including missing and malformed ones.
func TestCSRFToken(t *testing.T) {
	token, err := csrfToken("connect.sid=s%3Aabc; XSRF-TOKEN=a%2Bb%3Dc")
	require.NoError(t, err)
	require.Equal(t, "a+b=c", token)

	token, err = csrfToken("XSRF-TOKEN=plain")
	require.NoError(t, err)
	require.Equal(t, "plain", token)

	for _, cookie := range []string{"", "connect.sid=abc", "XSRF-TOKEN=", "XSRF-TOKEN=%zz"} {
		_, err := csrfToken(cookie)
		require.Error(t, err, cookie)
	}
}

func TestOrganizationRestorePayloadKeepsOnlyStoredSettings(t *testing.T) {
	payload := organizationRestorePayload(map[string]any{
		"id": "default", "idpRefId": "default", "displayName": "Default", "cpRefId": "default",
		"businessOwner": nil, "businessOwnerEmail": "", "businessOwnerContact": "+1 555",
		"configuration": map[string]any{"theme": "dark"}, "createdAt": "2026-01-01T00:00:00Z",
	})
	require.Equal(t, map[string]any{
		"id": "default", "idpRefId": "default", "displayName": "Default", "cpRefId": "default",
		"businessOwnerContact": "+1 555", "configuration": map[string]any{"theme": "dark"},
	}, payload)
}

func TestMultipartFileBodyCarriesOneNamedFilePart(t *testing.T) {
	body, contentType, err := multipartFileBody("organization", "organization.yaml", "kind: Organization\n")
	require.NoError(t, err)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/organizations/default", bytes.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	require.NoError(t, request.ParseMultipartForm(1<<20))
	files := request.MultipartForm.File["organization"]
	require.Len(t, files, 1)
	require.Equal(t, "organization.yaml", files[0].Filename)
	require.Equal(t, "application/yaml", files[0].Header.Get("Content-Type"))
	file, err := files[0].Open()
	require.NoError(t, err)
	defer file.Close()
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "kind: Organization\n", string(content))

	_, _, err = multipartFileBody(" ", "organization.yaml", "")
	require.Error(t, err)
}

func TestOrganizationTemplateRendersTheManifestEnvelope(t *testing.T) {
	root := filepath.Join("..", "..")
	path, err := stepscommon.ResourceTemplatePath(root, "resources/templates/api-portal-organization.yaml")
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	ctx := context.Background()

	rendered, err := stepscommon.RenderResourceTemplate(ctx, "resources/templates/api-portal-organization.yaml", content,
		claimsTable(t,
			"name", "default",
			"displayName", "Acme",
			"spec.cpRefId", "default",
			"spec.labels", `[{"id":"gold","displayName":"Gold"}]`,
		))
	require.NoError(t, err)
	document, err := stepscommon.YAMLToJSON([]byte(rendered))
	require.NoError(t, err)
	require.JSONEq(t, `{"apiVersion":"api-portal.api-platform.wso2.com/v1","kind":"Organization",
		"metadata":{"name":"default"},
		"spec":{"displayName":"Acme","cpRefId":"default","labels":[{"id":"gold","displayName":"Gold"}]}}`,
		string(document))

	withoutName, err := stepscommon.RenderResourceTemplate(ctx, "resources/templates/api-portal-organization.yaml", content,
		claimsTable(t, "name", "default", "displayName", "Acme", "spec", `{"idpRefId":"default"}`))
	require.NoError(t, err)
	require.NotContains(t, withoutName, "displayName", "a complete spec override replaces the template spec")

	_, err = stepscommon.RenderResourceTemplate(ctx, "resources/templates/api-portal-organization.yaml", content,
		claimsTable(t, "name", "default"))
	require.Error(t, err, "the template's display name has no value")
}

func testPlatformToken(t *testing.T) string {
	t.Helper()
	token, err := signRS256(map[string]any{"typ": "JWT", "kid": "cp"},
		map[string]any{"sub": "admin", "org_handle": "default", "exp": float64(4102444800)},
		shared.ControlPlaneCrypto()["keys/jwt_private.pem"])
	require.NoError(t, err)
	return token
}

func decodeTestSegment(t *testing.T, segment string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, decodeSegment(segment, &out))
	return out
}

func TestForgeTokenVariants(t *testing.T) {
	token := testPlatformToken(t)
	now := time.Unix(1_800_000_000, 0)
	public := shared.ControlPlaneCrypto()["keys/jwt_public.pem"]

	valid, err := forgeToken(token, "valid", now)
	require.NoError(t, err)
	require.Equal(t, token, valid)

	resigned, err := forgeToken(token, "re-signed", now)
	require.NoError(t, err)
	require.True(t, verifyTestRS256(t, resigned, public))
	require.Equal(t, decodeTestSegment(t, strings.Split(token, ".")[1]), decodeTestSegment(t, strings.Split(resigned, ".")[1]))

	expired, err := forgeToken(token, "expired", now)
	require.NoError(t, err)
	require.True(t, verifyTestRS256(t, expired, public))
	require.Less(t, decodeTestSegment(t, strings.Split(expired, ".")[1])["exp"].(float64), float64(now.Unix()))

	tampered, err := forgeToken(token, "tampered", now)
	require.NoError(t, err)
	require.Equal(t, strings.Split(token, ".")[2], strings.Split(tampered, ".")[2])
	require.Equal(t, "admin-tampered", decodeTestSegment(t, strings.Split(tampered, ".")[1])["sub"])
	require.False(t, verifyTestRS256(t, tampered, public))

	none, err := forgeToken(token, "alg-none", now)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(none, "."))
	require.Equal(t, "none", decodeTestSegment(t, strings.Split(none, ".")[0])["alg"])

	hs, err := forgeToken(token, "hs256-public-key", now)
	require.NoError(t, err)
	parts := strings.Split(hs, ".")
	require.Equal(t, "HS256", decodeTestSegment(t, parts[0])["alg"])
	mac := hmac.New(sha256.New, public)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	require.Equal(t, base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), parts[2])

	foreign, err := forgeToken(token, "foreign-key", now)
	require.NoError(t, err)
	require.Len(t, strings.Split(foreign, "."), 3)
	require.False(t, verifyTestRS256(t, foreign, public))
}

func TestForgeTokenRejectsMalformedInput(t *testing.T) {
	_, err := forgeToken("not-a-jwt", "valid", time.Now())
	require.Error(t, err)
	_, err = forgeToken("a.b.c", "valid", time.Now())
	require.Error(t, err)
	_, err = forgeToken(testPlatformToken(t), "unknown", time.Now())
	require.Error(t, err)
	_, err = signRS256(map[string]any{}, map[string]any{}, []byte("not pem"))
	require.Error(t, err)
}

func verifyTestRS256(t *testing.T, token string, publicPEM []byte) bool {
	t.Helper()
	block, _ := pem.Decode(publicPEM)
	require.NotNil(t, block)
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err)
	parts := strings.Split(token, ".")
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	return rsa.VerifyPKCS1v15(parsed.(*rsa.PublicKey), crypto.SHA256, digest[:], signature) == nil
}

func TestCookieHasAttribute(t *testing.T) {
	const setCookie = "connect.sid=s%3Aabc; Path=/api-portal; Expires=Thu, 01 Jan 2099 00:00:00 GMT; HttpOnly; Secure"
	require.True(t, cookieHasAttribute(setCookie, "HttpOnly"))
	require.True(t, cookieHasAttribute(setCookie, "httponly"))
	require.True(t, cookieHasAttribute(setCookie, "Secure"))
	require.True(t, cookieHasAttribute(setCookie, "Path=/api-portal"))
	require.False(t, cookieHasAttribute(setCookie, "Path=/"))
	require.False(t, cookieHasAttribute(setCookie, "SameSite"))
	require.False(t, cookieHasAttribute("connect.sid=HttpOnly", "HttpOnly"))
	require.False(t, cookieHasAttribute("", "HttpOnly"))
}

func TestCookieHasAttributeComparesPathCaseSensitively(t *testing.T) {
	require.True(t, cookieHasAttribute("sid=1; Path=/api-portal", "path=/api-portal"))
	require.False(t, cookieHasAttribute("sid=1; Path=/API-PORTAL", "Path=/api-portal"))
	require.True(t, cookieHasAttribute("sid=1; SameSite=lax", "SameSite=Lax"))
}

// TestForgeTokenRejectsNullSegments guards against a panic on a JWT segment holding JSON null.
func TestForgeTokenRejectsNullSegments(t *testing.T) {
	null := base64.RawURLEncoding.EncodeToString([]byte("null"))
	object := base64.RawURLEncoding.EncodeToString([]byte("{}"))
	for name, token := range map[string]string{
		"header": null + "." + object + ".sig",
		"claims": object + "." + null + ".sig",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := forgeToken(token, "expired", time.Now())
			require.ErrorContains(t, err, "must be a JSON object")
		})
	}
}

func TestOrganizationSnapshotHeldKeepsTheFirstSnapshot(t *testing.T) {
	pending := []cleanup.Resource{{Kind: portalOrganizationKind, ID: "default"}, {Kind: portalAPIKind, ID: "other"}}
	require.True(t, organizationSnapshotHeld(pending, "default"))
	require.False(t, organizationSnapshotHeld(pending, "other"))
	require.False(t, organizationSnapshotHeld(nil, "default"))
}

func TestCheckStagedRoleGrants(t *testing.T) {
	platform := map[string][]string{"admin": {"a"}, "narrow": {"x"}}
	portal := map[string][]string{"admin": {"a", "b"}, "narrow": {"y"}}
	require.NoError(t, checkStagedRoleGrants(platform, portal, "narrow"))
	require.ErrorContains(t, checkStagedRoleGrants(platform, portal, "admin"), "does not grant")
	require.ErrorContains(t, checkStagedRoleGrants(platform, portal, ""), "exception is required")
	require.ErrorContains(t, checkStagedRoleGrants(
		map[string][]string{"admin": {"a"}, "narrow": {"x"}, "extra": {"z"}}, portal, "narrow"), "missing role")
}

func TestNextListingOffset(t *testing.T) {
	next, more, err := nextListingOffset([]byte(`{"list":[{},{}],"pagination":{"offset":100,"total":103}}`))
	require.NoError(t, err)
	require.True(t, more)
	require.Equal(t, 102, next)
	_, more, err = nextListingOffset([]byte(`{"list":[{}],"pagination":{"offset":2,"total":3}}`))
	require.NoError(t, err)
	require.False(t, more)
	_, more, err = nextListingOffset([]byte(`{"list":[],"pagination":{"offset":0,"total":3}}`))
	require.NoError(t, err)
	require.False(t, more)
	_, _, err = nextListingOffset([]byte(`not json`))
	require.Error(t, err)
}

func TestNewestDeliveryMatching(t *testing.T) {
	list := []byte(`[
		{"headers":{},"body":{"event_type":"apikey.updated","id":"key-1","state":"associated"}},
		{"headers":{},"body":{"event_type":"apikey.updated","id":"key-1","state":"dissociated"}},
		{"headers":{},"body":{"event_type":"apikey.updated","id":"key-1","state":"associated","late":true}},
		{"headers":{},"body":{"event_type":"apikey.updated","id":"key-2","state":"dissociated"}}
	]`)

	body, found, err := newestDeliveryMatching(list, "key-1", "associated\"")
	require.NoError(t, err)
	require.True(t, found)
	require.Contains(t, string(body), `"dissociated"`, "a late copy of the excluded event must be skipped")

	body, found, err = newestDeliveryMatching(list, "key-1", "")
	require.NoError(t, err)
	require.True(t, found)
	require.Contains(t, string(body), `"late":true`, "without an exclusion the newest match wins")

	_, found, err = newestDeliveryMatching(list, "key-3", "")
	require.NoError(t, err)
	require.False(t, found)

	_, found, err = newestDeliveryMatching([]byte(`[]`), "key-1", "")
	require.NoError(t, err)
	require.False(t, found)

	_, _, err = newestDeliveryMatching([]byte(`{"not":"a list"}`), "key-1", "")
	require.Error(t, err)
}

func TestCheckValuesOnExactlyOnePage(t *testing.T) {
	pages := []string{"one two", "three"}
	require.NoError(t, checkValuesOnExactlyOnePage([]string{"one", " two", "three"}, pages))
	require.ErrorContains(t, checkValuesOnExactlyOnePage([]string{"four"}, pages), "on 0 of 2")
	require.ErrorContains(t, checkValuesOnExactlyOnePage([]string{"t"}, []string{"t", "t"}), "on 2 of 2")
}

func TestCheckExportedTheme(t *testing.T) {
	uploaded, err := buildZip([]zipEntry{{name: "theme/styles/main.css", content: []byte("a")}})
	require.NoError(t, err)
	same, err := buildZip([]zipEntry{{name: "styles/main.css", content: []byte("a")}})
	require.NoError(t, err)
	require.NoError(t, checkExportedTheme(same, uploaded))
	changed, err := buildZip([]zipEntry{{name: "styles/main.css", content: []byte("b")}})
	require.NoError(t, err)
	require.ErrorContains(t, checkExportedTheme(changed, uploaded), "differs")
	missing, err := buildZip([]zipEntry{{name: "styles/other.css", content: []byte("a")}})
	require.NoError(t, err)
	require.ErrorContains(t, checkExportedTheme(missing, uploaded), "has no file")
	require.ErrorContains(t, checkExportedTheme([]byte("not a zip"), uploaded), "not a usable archive")
	escaping, err := buildZip([]zipEntry{{name: "../evil", content: []byte("a")}})
	require.NoError(t, err)
	require.ErrorContains(t, checkExportedTheme(escaping, uploaded), "outside the archive root")
}

func TestCookieValue(t *testing.T) {
	require.Equal(t, "s%3Aabc", cookieValue("XSRF-TOKEN=x; connect.sid=s%3Aabc", "connect.sid"))
	require.Equal(t, "", cookieValue("XSRF-TOKEN=x", "connect.sid"))
	require.Equal(t, "", cookieValue("", "connect.sid"))
}

// TestForgedCSRFTokenDiffersInEveryPosition ensures the forged token is wrong everywhere, not only
// in part.
func TestForgedCSRFTokenDiffersInEveryPosition(t *testing.T) {
	issued := "AbAcA-_9"
	forged := forgedCSRFToken(issued)
	require.Len(t, forged, len(issued))
	for i := range issued {
		require.NotEqual(t, issued[i], forged[i])
	}
	require.NotEmpty(t, forgedCSRFToken(""))
}

// TestUploadBoundaryFixturesStraddleTheLimit keeps the near-limit and oversized fixtures on either
// side of the limit.
func TestUploadBoundaryFixturesStraddleTheLimit(t *testing.T) {
	const limit = 10 << 20
	near, _, _, err := portalDefinition("rest near-limit")
	require.NoError(t, err)
	require.Len(t, near, uploadNearLimitBytes)
	require.Less(t, len(near), limit)
	require.True(t, json.Valid(near))
	over, _, _, err := portalDefinition("rest oversized")
	require.NoError(t, err)
	require.Len(t, over, uploadOversizedBytes)
	require.Greater(t, len(over), limit)
	_, _, _, err = paddedRESTDefinition(1)
	require.Error(t, err)

	for _, build := range []func(string) ([]byte, error){contentZip, themeZip} {
		nearZip, err := build("near-limit")
		require.NoError(t, err)
		require.Less(t, len(nearZip), limit)
		require.Greater(t, len(nearZip), uploadNearLimitBytes)
		overZip, err := build("oversized")
		require.NoError(t, err)
		require.Greater(t, len(overZip), limit)
	}
}

func webhookDeliveryFixture(eventType, eventID, deliveryID string) recordedWebhookDelivery {
	body, _ := json.Marshal(map[string]any{
		"event_id":         eventID,
		"event_type":       eventType,
		"occurred_at":      "2026-11-06T10:00:00.123Z",
		"org":              map[string]string{"ref_id": "default"},
		"encrypted_fields": []string{},
		"data":             map[string]string{"handle": "app-1"},
	})
	return recordedWebhookDelivery{
		Headers: map[string][]string{
			"Content-Type":             {"application/json"},
			"X-Api-Portal-Event":       {eventType},
			"X-Api-Portal-Event-Id":    {eventID},
			"X-Api-Portal-Delivery-Id": {deliveryID},
		},
		Body: body,
	}
}

// TestCheckWebhookDeliveryEnvelopes covers each way a delivery envelope or its headers can be
// invalid.
func TestCheckWebhookDeliveryEnvelopes(t *testing.T) {
	valid := []recordedWebhookDelivery{
		webhookDeliveryFixture("application.created", "evt-1", "dlv-1"),
		webhookDeliveryFixture("application.updated", "evt-2", "dlv-2"),
	}
	require.NoError(t, checkWebhookDeliveryEnvelopes(valid))
	require.ErrorContains(t, checkWebhookDeliveryEnvelopes(nil), "no API Portal webhook delivery")

	mutate := func(change func(*recordedWebhookDelivery)) []recordedWebhookDelivery {
		first := webhookDeliveryFixture("application.created", "evt-1", "dlv-1")
		first.Headers = map[string][]string{}
		for name, values := range valid[0].Headers {
			first.Headers[name] = append([]string(nil), values...)
		}
		change(&first)
		return []recordedWebhookDelivery{first}
	}
	withBody := func(field string, value any) []recordedWebhookDelivery {
		return mutate(func(d *recordedWebhookDelivery) {
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(d.Body, &envelope))
			if value == nil {
				delete(envelope, field)
			} else {
				envelope[field] = value
			}
			d.Body, _ = json.Marshal(envelope)
		})
	}

	cases := map[string]struct {
		deliveries []recordedWebhookDelivery
		want       string
	}{
		"not JSON":              {mutate(func(d *recordedWebhookDelivery) { d.Body = []byte("nope") }), "not an event envelope"},
		"no event id":           {withBody("event_id", nil), "event_id and event_type are required"},
		"wrong content type":    {mutate(func(d *recordedWebhookDelivery) { d.Headers["Content-Type"] = []string{"text/plain"} }), "Content-Type"},
		"event header mismatch": {mutate(func(d *recordedWebhookDelivery) { d.Headers["X-Api-Portal-Event"] = []string{"apikey.generated"} }), "X-Api-Portal-Event is"},
		"event id mismatch":     {mutate(func(d *recordedWebhookDelivery) { d.Headers["X-Api-Portal-Event-Id"] = []string{"evt-9"} }), "X-Api-Portal-Event-Id"},
		"no delivery id":        {mutate(func(d *recordedWebhookDelivery) { delete(d.Headers, "X-Api-Portal-Delivery-Id") }), "Delivery-Id is missing"},
		"bad occurred_at":       {withBody("occurred_at", "yesterday"), "not RFC 3339"},
		"no org":                {withBody("org", map[string]string{}), "org.ref_id is missing"},
		"no encrypted fields":   {withBody("encrypted_fields", nil), "encrypted_fields is not a list"},
		"data not an object":    {withBody("data", "text"), "data is not an object"},
		"repeated delivery id": {[]recordedWebhookDelivery{
			webhookDeliveryFixture("application.created", "evt-1", "dlv-1"),
			webhookDeliveryFixture("application.updated", "evt-2", "dlv-1"),
		}, "repeats another delivery's"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			require.ErrorContains(t, checkWebhookDeliveryEnvelopes(tc.deliveries), tc.want)
		})
	}

	lower := mutate(func(d *recordedWebhookDelivery) {
		d.Headers = map[string][]string{
			"content-type":             {"application/json; charset=utf-8"},
			"x-api-portal-event":       {"application.created"},
			"x-api-portal-event-id":    {"evt-1"},
			"x-api-portal-delivery-id": {"dlv-1"},
		}
	})
	require.NoError(t, checkWebhookDeliveryEnvelopes(lower))
}

func TestCheckWebhookDeliveryEnvelopesAcceptsFanOutOfOneEvent(t *testing.T) {
	require.NoError(t, checkWebhookDeliveryEnvelopes([]recordedWebhookDelivery{
		webhookDeliveryFixture("application.created", "evt-1", "dlv-1"),
		webhookDeliveryFixture("application.created", "evt-1", "dlv-2"),
	}))
}

func TestDeliveriesContainingFiltersByMarker(t *testing.T) {
	deliveries := []recordedWebhookDelivery{
		webhookDeliveryFixture("application.created", "evt-1", "dlv-1"),
		{Body: json.RawMessage(`{"data":{"handle":"other"}}`)},
	}
	require.Len(t, deliveriesContaining(deliveries, "app-1"), 1)
	require.Empty(t, deliveriesContaining(deliveries, "missing"))
	require.Empty(t, deliveriesContaining(nil, "app-1"))
}

func TestAssertWebhookDeliveryEnvelopesRefusesMissingInput(t *testing.T) {
	ctx := tcontext.WithLocal(context.Background(), tcontext.NewLocal("runner"))
	s := &Steps{}
	require.ErrorContains(t, s.assertWebhookDeliveryEnvelopes(ctx, " ", 1), "requires a marker")
	require.ErrorContains(t, s.assertWebhookDeliveryEnvelopes(ctx, "app-1", 0), "positive count")
}
