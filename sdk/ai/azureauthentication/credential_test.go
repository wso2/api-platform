/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package azureauthentication

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testTenantID  = "72f988bf-86f1-41af-91ab-2d7cd011db47"
	testClientID  = "11111111-2222-3333-4444-555555555555"
	testAuthority = "https://login.test"

	// What Container Apps sets in a container app with a managed identity, shaped as a probe of a
	// real container app recorded them.
	testIdentityEndpoint = "http://localhost:12356/msi/token"
	testIdentityHeader   = "4f8c2b6e-9d31-4a7e-b5c0-identity-hdr"
)

// tokenRequest is a token request the SDK sent: to Entra ID's token endpoint, to IMDS or to the
// Container Apps identity endpoint.
type tokenRequest struct {
	method string
	url    *url.URL
	header http.Header
	form   url.Values // the POST form; empty for a managed identity
}

// rawBody is a response body sent as it is, without a Content-Type, rather than as JSON.
type rawBody string

// fakeAzure stands in for Microsoft Entra ID, IMDS and the Container Apps identity endpoint as the
// SDK's HTTP transport. It serves the OpenID configuration MSAL reads before its first token
// request, and answers each token request with respond. A status of 0 drops the connection instead.
type fakeAzure struct {
	respond func(ctx context.Context, r tokenRequest) (status int, body any)

	mu        sync.Mutex
	tokens    []tokenRequest
	discovery int
	otherURLs []string
}

func (f *fakeAzure) Do(req *http.Request) (*http.Response, error) {
	switch {
	case strings.HasSuffix(req.URL.Path, "/v2.0/.well-known/openid-configuration"):
		f.mu.Lock()
		f.discovery++
		f.mu.Unlock()
		tenant := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")[0]
		base := req.URL.Scheme + "://" + req.URL.Host + "/" + tenant
		return jsonResponse(req, http.StatusOK, map[string]string{
			"issuer":                 base + "/v2.0",
			"token_endpoint":         base + "/oauth2/v2.0/token",
			"authorization_endpoint": base + "/oauth2/v2.0/authorize",
		}), nil
	case strings.HasSuffix(req.URL.Path, "/oauth2/v2.0/token"), req.URL.Path == "/metadata/identity/oauth2/token",
		req.URL.Path == "/msi/token":
	default:
		f.mu.Lock()
		f.otherURLs = append(f.otherURLs, req.URL.String())
		f.mu.Unlock()
		return jsonResponse(req, http.StatusNotFound, map[string]string{"error": "not_found"}), nil
	}

	r := tokenRequest{method: req.Method, url: req.URL, header: req.Header.Clone()}
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		r.form, _ = url.ParseQuery(string(body))
	}
	f.mu.Lock()
	f.tokens = append(f.tokens, r)
	f.mu.Unlock()

	status, body := f.respond(req.Context(), r)
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	if status == 0 {
		return nil, errors.New("connection refused")
	}
	return jsonResponse(req, status, body), nil
}

// tokenRequests returns the token requests sent so far, oldest first.
func (f *fakeAzure) tokenRequests() []tokenRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]tokenRequest(nil), f.tokens...)
}

func jsonResponse(req *http.Request, status int, body any) *http.Response {
	header := http.Header{}
	var data []byte
	if raw, ok := body.(rawBody); ok {
		data = []byte(raw)
	} else {
		data, _ = json.Marshal(body)
		header.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status) + " " + http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(data)),
		Request:    req,
	}
}

// entraToken is a Microsoft Entra ID token response.
func entraToken(accessToken string) map[string]any {
	return map[string]any{"token_type": "Bearer", "access_token": accessToken, "expires_in": 3599, "ext_expires_in": 3599}
}

// imdsToken is an IMDS token response, with every number as a string, as IMDS sends it.
func imdsToken(accessToken, resource string) map[string]any {
	return map[string]any{
		"token_type":   "Bearer",
		"access_token": accessToken,
		"expires_in":   "3599",
		"expires_on":   strconv.FormatInt(time.Now().Add(3599*time.Second).Unix(), 10),
		"resource":     resource,
	}
}

// containerAppsToken is a token response from the Container Apps identity endpoint, as the probe
// recorded it: expires_on is Unix seconds as a string, 24 hours ahead.
func containerAppsToken(accessToken, resource, clientID string) map[string]any {
	return map[string]any{
		"access_token": accessToken,
		"expires_on":   strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10),
		"resource":     resource,
		"token_type":   "Bearer",
		"client_id":    clientID,
	}
}

// containerAppsError is an error the Container Apps identity endpoint answers itself, as the probe
// recorded it.
func containerAppsError(status int, message string) map[string]any {
	return map[string]any{"statusCode": status, "message": message, "correlationId": "6b8a85dc-9ab3-4bad-a96c-2f91ac8a4198"}
}

// oauthError is an error response from Entra ID or IMDS.
func oauthError(code, description string, errorCodes ...int) map[string]any {
	body := map[string]any{"error": code, "error_description": description}
	if len(errorCodes) > 0 {
		body["error_codes"] = errorCodes
	}
	return body
}

// answer returns a respond function that always answers status and body.
func answer(status int, body any) func(context.Context, tokenRequest) (int, any) {
	return func(context.Context, tokenRequest) (int, any) { return status, body }
}

// useFakeAzure points the SDK's credentials at a fake Entra ID and IMDS for one test, with an
// empty credential registry, retries that don't wait, and an environment that selects IMDS for
// managed identity.
func useFakeAzure(t *testing.T, respond func(ctx context.Context, r tokenRequest) (int, any)) *fakeAzure {
	t.Helper()
	for _, name := range []string{"IDENTITY_ENDPOINT", "IDENTITY_HEADER", "IDENTITY_SERVER_THUMBPRINT", "IMDS_ENDPOINT", "MSI_ENDPOINT", "MSI_SECRET"} {
		unsetEnv(t, name)
	}
	f := &fakeAzure{respond: respond}
	prevTransport, prevCredentials := transport, credentials
	prevEntra, prevManaged, prevContainerApps := entraRetry, managedRetry, containerAppsRetry
	transport, credentials = f, newCredentialRegistry()
	entraRetry.RetryDelay, entraRetry.MaxRetryDelay = time.Millisecond, time.Millisecond
	managedRetry.RetryDelay, managedRetry.MaxRetryDelay = time.Millisecond, time.Millisecond
	containerAppsRetry.RetryDelay, containerAppsRetry.MaxRetryDelay = time.Millisecond, time.Millisecond
	t.Cleanup(func() {
		transport, credentials = prevTransport, prevCredentials
		entraRetry, managedRetry, containerAppsRetry = prevEntra, prevManaged, prevContainerApps
	})
	return f
}

// setContainerAppsEnv sets what Container Apps injects into a container app with a managed
// identity, as the probe recorded it: MSI_ENDPOINT and MSI_SECRET too. Call it after useFakeAzure,
// which clears these variables.
func setContainerAppsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("IDENTITY_ENDPOINT", testIdentityEndpoint)
	t.Setenv("IDENTITY_HEADER", testIdentityHeader)
	t.Setenv("MSI_ENDPOINT", testIdentityEndpoint)
	t.Setenv("MSI_SECRET", testIdentityHeader)
}

// setWorkloadIdentityEnv sets what the AKS Workload Identity webhook injects, with the authority
// host pointing at the fake Entra ID, and writes the federated token to a file.
func setWorkloadIdentityEnv(t *testing.T, federatedToken string) {
	t.Helper()
	tokenFile := filepath.Join(t.TempDir(), "azure-identity-token")
	if err := os.WriteFile(tokenFile, []byte(federatedToken), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AZURE_TENANT_ID", testTenantID)
	t.Setenv("AZURE_CLIENT_ID", testClientID)
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", tokenFile)
	t.Setenv("AZURE_AUTHORITY_HOST", testAuthority)
}

var scopeCount atomic.Int64

// uniqueScope returns a scope no other test, or run of a test, uses. MSAL caches managed identity
// tokens for the whole process, by identity and resource, so tests that shared a resource would
// share tokens.
func uniqueScope(t *testing.T) string {
	name := strings.NewReplacer("/", "-", " ", "-", "'", "").Replace(strings.ToLower(t.Name()))
	return fmt.Sprintf("api://%s-%d/.default", name, scopeCount.Add(1))
}

// unsetEnv removes name for one test and restores it afterwards.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	os.Unsetenv(name)
}

// mustGet returns the credential for id, failing the test if there is none.
func mustGet(t *testing.T, id Identity) *Credential {
	t.Helper()
	cred, err := Get(id)
	if err != nil {
		t.Fatalf("Get(%+v): %v", id, err)
	}
	return cred
}

// asTokenError returns err as a *TokenError, failing the test if it is some other error.
func asTokenError(t *testing.T, err error) *TokenError {
	t.Helper()
	if err == nil {
		return nil
	}
	var terr *TokenError
	if !errors.As(err, &terr) {
		t.Fatalf("error = %v (%T), want a *TokenError", err, err)
	}
	return terr
}

// mustFail asks cred for a token for scope and returns the failure, failing the test if it got one.
func mustFail(t *testing.T, cred *Credential, scope string) *TokenError {
	t.Helper()
	_, err := cred.Token(context.Background(), scope, 5*time.Second)
	if err == nil {
		t.Fatal("expected an error")
	}
	return asTokenError(t, err)
}

func TestWorkloadIdentityTokenRequest(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	f := useFakeAzure(t, answer(http.StatusOK, entraToken("tok-1")))

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	tok, err := cred.Token(context.Background(), "https://ai.azure.com/.default", time.Second)
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "tok-1" {
		t.Fatalf("token = %q, want tok-1", tok)
	}

	reqs := f.tokenRequests()
	if len(reqs) != 1 {
		t.Fatalf("token requests = %d, want 1", len(reqs))
	}
	r := reqs[0]
	if got, want := r.url.String(), testAuthority+"/"+testTenantID+"/oauth2/v2.0/token"; got != want {
		t.Errorf("token URL = %s, want %s", got, want)
	}
	for field, want := range map[string]string{
		"grant_type":            "client_credentials",
		"client_id":             testClientID,
		"client_assertion_type": "urn:ietf:params:oauth:client-assertion-type:jwt-bearer",
		"client_assertion":      "federated-token",
	} {
		if got := r.form.Get(field); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	if scopes := strings.Fields(r.form.Get("scope")); len(scopes) == 0 || scopes[0] != "https://ai.azure.com/.default" {
		t.Errorf("scope = %q, want it to start with the requested scope", r.form.Get("scope"))
	}
}

// The authority host comes only from the gateway's environment, so the credential must not ask the
// public cloud to validate it: an air-gapped or sovereign gateway can't reach it.
func TestWorkloadIdentitySkipsInstanceDiscovery(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	f := useFakeAzure(t, answer(http.StatusOK, entraToken("tok-1")))

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	if _, err := cred.Token(context.Background(), "https://ai.azure.com/.default", time.Second); err != nil {
		t.Fatalf("Token: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.otherURLs) != 0 {
		t.Fatalf("unexpected requests: %v", f.otherURLs)
	}
	if f.discovery != 1 {
		t.Fatalf("OpenID configuration requests = %d, want 1", f.discovery)
	}
}

func TestManagedIdentityTokenRequest(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		wantClientID string
	}{
		{name: "user-assigned identity", clientID: testClientID, wantClientID: testClientID},
		{name: "host's default identity", clientID: "", wantClientID: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AZURE_CLIENT_ID names the federated identity, which need not be assigned to the host.
			t.Setenv("AZURE_CLIENT_ID", "99999999-8888-7777-6666-555555555555")
			scope := uniqueScope(t)
			resource := strings.TrimSuffix(scope, "/.default")
			f := useFakeAzure(t, answer(http.StatusOK, imdsToken("tok-1", resource)))

			cred := mustGet(t, Identity{Type: ManagedIdentity, ClientID: tt.clientID})
			tok, err := cred.Token(context.Background(), scope, time.Second)
			if err != nil || tok != "tok-1" {
				t.Fatalf("token = %q, %v; want tok-1", tok, err)
			}

			reqs := f.tokenRequests()
			if len(reqs) != 1 {
				t.Fatalf("token requests = %d, want 1", len(reqs))
			}
			r := reqs[0]
			if r.url.Host != "169.254.169.254" || r.header.Get("Metadata") != "true" {
				t.Errorf("request = %s with Metadata %q, want IMDS with Metadata: true", r.url, r.header.Get("Metadata"))
			}
			q := r.url.Query()
			if q.Get("resource") != resource || q.Get("api-version") != "2018-02-01" || q.Get("client_id") != tt.wantClientID {
				t.Errorf("query = %v, want resource %s, api-version 2018-02-01, client_id %q", q, resource, tt.wantClientID)
			}
		})
	}
}

// A container app's token comes from the endpoint Container Apps sets, with its header, and from
// nowhere else.
func TestContainerAppsTokenRequest(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
	}{
		{name: "user-assigned identity", clientID: testClientID},
		{name: "system-assigned identity", clientID: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// AZURE_CLIENT_ID names a federated identity; it must not select the app's identity.
			setWorkloadIdentityEnv(t, "federated-token")
			scope := uniqueScope(t)
			resource := strings.TrimSuffix(scope, "/.default")
			f := useFakeAzure(t, answer(http.StatusOK, containerAppsToken("tok-1", resource, testClientID)))
			setContainerAppsEnv(t)

			cred := mustGet(t, Identity{Type: ContainerApps, ClientID: tt.clientID})
			tok, err := cred.Token(context.Background(), scope, time.Second)
			if err != nil || tok != "tok-1" {
				t.Fatalf("token = %q, %v; want tok-1", tok, err)
			}

			reqs := f.tokenRequests()
			if len(reqs) != 1 || len(f.otherURLs) != 0 {
				t.Fatalf("token requests = %d, other requests = %v; want one token request only", len(reqs), f.otherURLs)
			}
			r := reqs[0]
			endpoint := r.url.Scheme + "://" + r.url.Host + r.url.Path
			if r.method != http.MethodGet || endpoint != testIdentityEndpoint || r.header.Get("X-IDENTITY-HEADER") != testIdentityHeader {
				t.Errorf("request = %s %s with X-IDENTITY-HEADER %q, want GET %s with the identity header",
					r.method, endpoint, r.header.Get("X-IDENTITY-HEADER"), testIdentityEndpoint)
			}
			q := r.url.Query()
			if q.Get("resource") != resource || q.Get("api-version") != "2019-08-01" || q.Get("client_id") != tt.clientID {
				t.Errorf("query = %v, want resource %s, api-version 2019-08-01, client_id %q", q, resource, tt.clientID)
			}
			if _, sent := q["client_id"]; tt.clientID == "" && sent {
				t.Errorf("query = %v, want no client_id for the system-assigned identity", q)
			}
		})
	}
}

func TestContainerAppsAndManagedIdentityDoNotShareACredential(t *testing.T) {
	useFakeAzure(t, answer(http.StatusOK, containerAppsToken("tok-1", "https://ai.azure.com", testClientID)))
	setContainerAppsEnv(t)

	containerApps := mustGet(t, Identity{Type: ContainerApps, ClientID: testClientID})
	managed := mustGet(t, Identity{Type: ManagedIdentity, ClientID: testClientID})
	if containerApps == managed {
		t.Fatal("container-apps and managed-identity share a credential")
	}
}

// Every policy, API and route with the same identity shares one credential, and so one token cache.
func TestOneCredentialPerIdentity(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	f := useFakeAzure(t, answer(http.StatusOK, entraToken("tok-1")))
	id := Identity{Type: WorkloadIdentity}

	var wg sync.WaitGroup
	creds := make([]*Credential, 20)
	for i := range creds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			creds[i], _ = Get(id)
		}()
	}
	wg.Wait()
	if creds[0] == nil {
		t.Fatal("Get returned no credential")
	}
	for _, c := range creds[1:] {
		if c != creds[0] {
			t.Fatal("the same identity got two credentials")
		}
	}

	for range 3 {
		if _, err := creds[0].Token(context.Background(), "https://ai.azure.com/.default", time.Second); err != nil {
			t.Fatalf("Token: %v", err)
		}
	}
	if _, err := creds[0].Token(context.Background(), "https://cognitiveservices.azure.com/.default", time.Second); err != nil {
		t.Fatalf("Token: %v", err)
	}
	if n := len(f.tokenRequests()); n != 2 {
		t.Fatalf("token requests = %d, want 2 (one per scope)", n)
	}

	other := mustGet(t, Identity{Type: ManagedIdentity})
	if other == creds[0] {
		t.Fatal("workload identity and managed identity share a credential")
	}
}

func TestFaultReasons(t *testing.T) {
	tests := []struct {
		name         string
		typ          string
		clientID     string
		status       int
		body         any
		wantReason   string
		wantRejected bool
	}{
		{
			name: "Entra ID rejects the client", typ: WorkloadIdentity,
			status: http.StatusUnauthorized, body: oauthError("invalid_client", "AADSTS700213: No matching federated identity record found.", 700213),
			wantReason: "Microsoft Entra ID rejected the token request (HTTP 401, invalid_client)", wantRejected: true,
		},
		{
			name: "Entra ID rejects without an error code", typ: WorkloadIdentity,
			status: http.StatusBadRequest, body: "not JSON",
			wantReason: "Microsoft Entra ID rejected the token request (HTTP 400)", wantRejected: true,
		},
		{
			name: "Entra ID keeps failing", typ: WorkloadIdentity,
			status: http.StatusServiceUnavailable, body: oauthError("temporarily_unavailable", "The service is unavailable."),
			wantReason: "Microsoft Entra ID returned HTTP 503 (temporarily_unavailable)",
		},
		{
			name: "Entra ID keeps throttling", typ: WorkloadIdentity,
			status: http.StatusTooManyRequests, body: oauthError("temporarily_unavailable", "The request was throttled."),
			wantReason: "Microsoft Entra ID returned HTTP 429 (temporarily_unavailable)",
		},
		{
			name: "Entra ID keeps timing out", typ: WorkloadIdentity,
			status: http.StatusRequestTimeout, body: rawBody(""),
			wantReason: "Microsoft Entra ID returned HTTP 408",
		},
		{
			name: "Entra ID can't be reached", typ: WorkloadIdentity,
			status:     0,
			wantReason: "could not obtain an access token from Microsoft Entra ID",
		},
		{
			name: "IMDS doesn't know the identity", typ: ManagedIdentity, clientID: testClientID,
			status: http.StatusBadRequest, body: oauthError("invalid_request", "Identity not found"),
			wantReason: "the managed identity endpoint rejected the token request (HTTP 400, invalid_request)", wantRejected: true,
		},
		{
			name: "IMDS keeps failing", typ: ManagedIdentity, clientID: testClientID,
			status: http.StatusGone, body: oauthError("unavailable", "IMDS is updating."),
			wantReason: "the managed identity endpoint returned HTTP 410 (unavailable)",
		},
		// The Container Apps rows answer as a probe of a real container app recorded the endpoint
		// answering.
		{
			name: "Container Apps doesn't know the identity", typ: ContainerApps, clientID: testClientID,
			status: http.StatusBadRequest, body: containerAppsError(http.StatusBadRequest,
				"No User Assigned or Delegated Managed Identity found for specified ClientId/ResourceId/PrincipalId."),
			wantReason: "the Container Apps identity endpoint rejected the token request (HTTP 400)", wantRejected: true,
		},
		{
			name: "Container Apps has no system-assigned identity", typ: ContainerApps,
			status: http.StatusBadRequest, body: containerAppsError(http.StatusBadRequest, "Unable to load the proper Managed Identity."),
			wantReason: "the Container Apps identity endpoint rejected the token request (HTTP 400)", wantRejected: true,
		},
		{
			name: "Entra ID refuses through Container Apps", typ: ContainerApps, clientID: testClientID,
			status: http.StatusBadRequest, body: map[string]any{
				"exceptionMessage": "AADSTS500011: The resource principal named https://nonexistent.invalid was not found in the tenant named " + testTenantID + ".",
				"errorCode":        "invalid_resource", "innerException": nil, "statusCode": 400, "message": nil,
				"correlationId": "0685330a-bb44-4f22-8172-4032ed6aa32b",
			},
			wantReason: "the Container Apps identity endpoint rejected the token request (HTTP 400, invalid_resource)", wantRejected: true,
		},
		{
			name: "Container Apps refuses the identity header", typ: ContainerApps, clientID: testClientID,
			status: http.StatusForbidden, body: rawBody(""),
			wantReason: "the Container Apps identity endpoint rejected the token request (HTTP 403)", wantRejected: true,
		},
		{
			// Unlike IMDS, the endpoint doesn't answer 404 while it starts, so a 404 is a refusal.
			name: "Container Apps answers 404", typ: ContainerApps, clientID: testClientID,
			status: http.StatusNotFound, body: rawBody(""),
			wantReason: "the Container Apps identity endpoint rejected the token request (HTTP 404)", wantRejected: true,
		},
		{
			name: "Container Apps keeps failing", typ: ContainerApps, clientID: testClientID,
			status: http.StatusInternalServerError, body: containerAppsError(http.StatusInternalServerError, "An error occurred."),
			wantReason: "the Container Apps identity endpoint returned HTTP 500",
		},
		{
			name: "Container Apps keeps timing out", typ: ContainerApps, clientID: testClientID,
			status: http.StatusRequestTimeout, body: rawBody(""),
			wantReason: "the Container Apps identity endpoint returned HTTP 408",
		},
		{
			name: "Container Apps can't be reached", typ: ContainerApps, clientID: testClientID,
			status:     0,
			wantReason: "could not obtain an access token from the Container Apps identity endpoint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWorkloadIdentityEnv(t, "federated-token")
			useFakeAzure(t, answer(tt.status, tt.body))
			if tt.typ == ContainerApps {
				setContainerAppsEnv(t)
			}

			cred := mustGet(t, Identity{Type: tt.typ, ClientID: tt.clientID})
			terr := mustFail(t, cred, uniqueScope(t))
			if terr.Reason != tt.wantReason || terr.Rejected != tt.wantRejected {
				t.Fatalf("reason = %q (rejected %v), want %q (rejected %v)", terr.Reason, terr.Rejected, tt.wantReason, tt.wantRejected)
			}
		})
	}
}

// The SDK reports IMDS refusing the host's default identity without the response, such as when the
// host has no system-assigned identity and several user-assigned ones. It is still a refusal, so
// it is cached like one.
func TestARefusedHostDefaultIdentityIsCached(t *testing.T) {
	f := useFakeAzure(t, answer(http.StatusBadRequest, oauthError("invalid_request",
		"Multiple user assigned identities exist, please specify the clientId / resourceId of the identity in the token request")))
	scope := uniqueScope(t)

	cred := mustGet(t, Identity{Type: ManagedIdentity})
	first := mustFail(t, cred, scope)
	if want := "the managed identity endpoint rejected the token request"; first.Reason != want || !first.Rejected {
		t.Fatalf("error = %+v, want %q, rejected", first, want)
	}
	if again := mustFail(t, cred, scope); !again.Cached {
		t.Fatal("the refusal was not cached")
	}
	if n := len(f.tokenRequests()); n != 1 {
		t.Fatalf("token requests = %d, want 1", n)
	}
}

// Container Apps refuses an app's default identity with its response, unlike IMDS, so the refusal
// carries its status and is cached like any other.
func TestARefusedContainerAppsDefaultIdentityIsCached(t *testing.T) {
	f := useFakeAzure(t, answer(http.StatusBadRequest, containerAppsError(http.StatusBadRequest, "Unable to load the proper Managed Identity.")))
	setContainerAppsEnv(t)
	scope := uniqueScope(t)

	cred := mustGet(t, Identity{Type: ContainerApps})
	first := mustFail(t, cred, scope)
	if want := "the Container Apps identity endpoint rejected the token request (HTTP 400)"; first.Reason != want || !first.Rejected {
		t.Fatalf("error = %+v, want %q, rejected", first, want)
	}
	if again := mustFail(t, cred, scope); !again.Cached {
		t.Fatal("the refusal was not cached")
	}
	if n := len(f.tokenRequests()); n != 1 {
		t.Fatalf("token requests = %d, want 1", n)
	}
}

// Calls that arrive together, before the first refusal is cached, share it rather than each asking
// the service again.
func TestConcurrentCallsShareARejection(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	f := useFakeAzure(t, func(context.Context, tokenRequest) (int, any) {
		time.Sleep(50 * time.Millisecond) // let every caller arrive while the first request is out
		return http.StatusUnauthorized, oauthError("invalid_client", "AADSTS700213", 700213)
	})

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := cred.Token(context.Background(), "https://ai.azure.com/.default", 5*time.Second)
			var terr *TokenError
			if !errors.As(err, &terr) || !terr.Rejected {
				t.Errorf("error = %v, want the rejection", err)
			}
		}()
	}
	wg.Wait()
	if n := len(f.tokenRequests()); n != 1 {
		t.Fatalf("token requests = %d, want 1", n)
	}
}

// A caller waiting for another caller's fetch on the same credential gives up at its own timeout,
// even when the other fetch has a longer one.
func TestWaitingForAnotherFetchStopsAtTheTimeout(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	started, release := make(chan struct{}, 1), make(chan struct{})
	useFakeAzure(t, func(ctx context.Context, _ tokenRequest) (int, any) {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return http.StatusOK, entraToken("tok-1")
	})
	defer close(release)

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	go cred.Token(context.Background(), "https://ai.azure.com/.default", 5*time.Second)
	<-started

	start := time.Now()
	_, err := cred.Token(context.Background(), "https://cognitiveservices.azure.com/.default", 100*time.Millisecond)
	if terr := asTokenError(t, err); terr == nil || terr.Reason != "timed out obtaining an access token from Microsoft Entra ID" {
		t.Fatalf("error = %v, want a timeout", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("the caller waited %v, want about 100ms", elapsed)
	}
}

// The error text keeps the SDK's error up to the response body MSAL quotes, or up to the first token
// field, and withholds everything from there on, however the body around the token is cut off or
// escaped. A JWT before it is redacted too, and text without either is kept as it is.
func TestErrorWithholdsEverythingFromATokenFieldOn(t *testing.T) {
	const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJhdWQiOiJhcGk6Ly94In0.c2lnbmF0dXJl"
	tests := []struct {
		name, text, keep string
	}{
		{"the body MSAL quotes for a token request", "json decode error: bad\nraw message was: " + `{"\u0061ccess_token":"secret-1"}`, "json decode error: bad\nraw message was: "},
		{"the body MSAL quotes for another request", "json decode error: bad\njson message bytes were: " + `{"token":"secret-1"}`, "json decode error: bad\njson message bytes were: "},
		{"an upper-case field name", `body: {"ACCESS_TOKEN":"secret-1"}`, `body: {`},
		{"a complete field", `decode error; body: {"access_token":"secret-1","expires_in":"x"}`, `decode error; body: {`},
		{"a field cut off inside its value", `unexpected end of JSON input; body: {"token_type":"Bearer","access_token":"secret-1`, `body: {"token_type":"Bearer",`},
		{"a body with escaped quotes", `body: "{\"token_type\":\"Bearer\",\"access_token\":\"secret-1\"}"`, `body: "{\"token_type\":\"Bearer\",`},
		{"a body escaped twice", `body: "{\\\"access_token\\\":\\\"secret-1\\\"}"`, `body: "{`},
		{"a refresh token", `body: {"refresh_token":"secret-1`, `body: {`},
		{"an ID token", `body: {"id_token": "secret-1"}`, `body: {`},
		{"a JWT before the field", `assertion ` + jwt + ` rejected; body: {"access_token":"secret-1"}`, `assertion [REDACTED] rejected; body: {`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&TokenError{Reason: "r", cause: errors.New(tt.text)}).Error()
			if strings.Contains(got, "secret-1") || strings.Contains(got, jwt) {
				t.Fatalf("Error() = %q, which still has the token", got)
			}
			if !strings.HasPrefix(got, "r: ") || !strings.Contains(got, tt.keep) {
				t.Errorf("Error() = %q, want the reason, then %q", got, tt.keep)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("Error() = %q, want it to say what it withheld", got)
			}
		})
	}

	const plain = `AADSTS700016: Application with identifier 'x' was not found in the directory 'y'.`
	if got := (&TokenError{Reason: "r", cause: errors.New(plain)}).Error(); got != "r: "+plain {
		t.Errorf("Error() = %q, want the reason and the text without a token field unchanged", got)
	}
}

// A token response that can't be decoded reaches the error text with its tokens withheld, from a
// real failed fetch rather than a hand-made error. That holds for a token that isn't a JWT under a
// field name the SDK decodes as access_token but that isn't spelled that way.
func TestAnUndecodableResponseDoesNotLeakItsTokens(t *testing.T) {
	const jwt = "eyJhbGciOiJSUzI1NiJ9.eyJhdWQiOiJodHRwczovL2FpLmF6dXJlLmNvbSJ9.c2lnbmF0dXJl"
	tests := []struct {
		name string
		body any
	}{
		{"fields named as Entra ID names them", map[string]any{
			"token_type":   "Bearer",
			"access_token": "access-token-secret",
			"id_token":     jwt,
			"expires_in":   map[string]any{"not": "a number"},
		}},
		{"an upper-case field name", rawBody(`{"token_type":"Bearer","ACCESS_TOKEN":"access-token-secret","expires_in":{}}`)},
		{"an escaped field name", rawBody(`{"token_type":"Bearer","\u0061ccess_token":"access-token-secret","expires_in":{}}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWorkloadIdentityEnv(t, "federated-token")
			useFakeAzure(t, answer(http.StatusOK, tt.body))

			terr := mustFail(t, mustGet(t, Identity{Type: WorkloadIdentity}), "https://ai.azure.com/.default")
			text := terr.Error()
			for _, secret := range []string{"access-token-secret", jwt} {
				if strings.Contains(text, secret) || strings.Contains(terr.Reason, secret) {
					t.Errorf("the error has %q:\nreason: %s\nerror: %s", secret, terr.Reason, text)
				}
			}
			if !strings.Contains(text, "[REDACTED]") {
				t.Errorf("Error() = %q, want it to say what it withheld", text)
			}
		})
	}
}

// IDENTITY_HEADER keeps anything but the app itself from minting tokens at the identity endpoint,
// so no failure's reason or error text carries it.
func TestTheIdentityHeaderNeverReachesTheError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   any
	}{
		{"a 400 from the endpoint", http.StatusBadRequest, containerAppsError(http.StatusBadRequest, "Unable to load the proper Managed Identity.")},
		{"a 403 without a body", http.StatusForbidden, rawBody("")},
		{"a 500 from the endpoint", http.StatusInternalServerError, containerAppsError(http.StatusInternalServerError, "An error occurred.")},
		{"no response", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useFakeAzure(t, answer(tt.status, tt.body))
			setContainerAppsEnv(t)

			terr := mustFail(t, mustGet(t, Identity{Type: ContainerApps}), uniqueScope(t))
			if !strings.Contains(terr.Reason, "the Container Apps identity endpoint") {
				t.Fatalf("reason = %q, want one naming the Container Apps identity endpoint", terr.Reason)
			}
			for _, text := range []string{terr.Reason, terr.Error()} {
				if strings.Contains(text, testIdentityHeader) {
					t.Fatalf("the identity header appears in %q", text)
				}
			}
		})
	}
}

// Neither the federated token the gateway sends nor a token Entra ID returns reaches a failure's
// reason or error text, while Entra ID's own error code does.
func TestSecretsNeverReachTheError(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token-secret")
	useFakeAzure(t, func(_ context.Context, r tokenRequest) (int, any) {
		if strings.HasPrefix(r.form.Get("scope"), "https://cognitiveservices.azure.com/") {
			return http.StatusUnauthorized, oauthError("invalid_client", "AADSTS700213: No matching federated identity record found.", 700213)
		}
		return http.StatusOK, entraToken("access-token-secret")
	})
	cred := mustGet(t, Identity{Type: WorkloadIdentity})

	if tok, err := cred.Token(context.Background(), "https://ai.azure.com/.default", 5*time.Second); err != nil || tok != "access-token-secret" {
		t.Fatalf("Token = %q, %v; want the access token", tok, err)
	}
	terr := mustFail(t, cred, "https://cognitiveservices.azure.com/.default")
	text := terr.Error()
	if !strings.Contains(text, "700213") {
		t.Errorf("Error() = %q, want Entra ID's error code", text)
	}
	for _, secret := range []string{"access-token-secret", "federated-token-secret"} {
		if strings.Contains(text, secret) || strings.Contains(terr.Reason, secret) {
			t.Errorf("the error has %q:\nreason: %s\nerror: %s", secret, terr.Reason, text)
		}
	}
}

// The managed identity SDK fetches each scope on its own, so a scope with a cached token answers at
// once while another scope's fetch is slow.
func TestAManagedIdentityCacheHitDoesNotWaitForAnotherScope(t *testing.T) {
	cached, slow := uniqueScope(t), uniqueScope(t)
	slowResource := strings.TrimSuffix(slow, "/.default")
	started, release := make(chan struct{}, 1), make(chan struct{})
	useFakeAzure(t, func(ctx context.Context, r tokenRequest) (int, any) {
		if r.url.Query().Get("resource") == slowResource {
			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
		return http.StatusOK, imdsToken("tok-1", r.url.Query().Get("resource"))
	})
	defer close(release)

	cred := mustGet(t, Identity{Type: ManagedIdentity, ClientID: testClientID})
	if _, err := cred.Token(context.Background(), cached, time.Second); err != nil {
		t.Fatalf("Token: %v", err)
	}
	go cred.Token(context.Background(), slow, 5*time.Second)
	<-started

	if tok, err := cred.Token(context.Background(), cached, 100*time.Millisecond); err != nil || tok != "tok-1" {
		t.Fatalf("token = %q, error = %v; want the cached token", tok, err)
	}
}

// A token response without a token is a failure, not an empty token.
func TestAnEmptyTokenIsAFailure(t *testing.T) {
	scope := uniqueScope(t)
	body := imdsToken("", strings.TrimSuffix(scope, "/.default"))
	delete(body, "access_token")
	useFakeAzure(t, answer(http.StatusOK, body))

	cred := mustGet(t, Identity{Type: ManagedIdentity, ClientID: testClientID})
	tok, err := cred.Token(context.Background(), scope, time.Second)
	if terr := asTokenError(t, err); terr == nil || terr.Reason != "the managed identity endpoint returned an unusable response" {
		t.Fatalf("token = %q, error = %v; want an unusable response", tok, err)
	}
}

func TestAFetchThatOutlivesTheTimeoutTimesOut(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	useFakeAzure(t, func(ctx context.Context, _ tokenRequest) (int, any) {
		<-ctx.Done()
		return http.StatusOK, entraToken("too-late")
	})

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	start := time.Now()
	_, err := cred.Token(context.Background(), "https://ai.azure.com/.default", 50*time.Millisecond)
	if terr := asTokenError(t, err); terr == nil || terr.Reason != "timed out obtaining an access token from Microsoft Entra ID" {
		t.Fatalf("error = %v, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the fetch took %v, want about 50ms", elapsed)
	}
}

// A caller that passes no timeout gets DefaultTokenTimeout rather than a fetch that has already
// timed out.
func TestNoTimeoutMeansTheDefault(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	useFakeAzure(t, answer(http.StatusOK, entraToken("tok-1")))

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	if tok, err := cred.Token(context.Background(), "https://ai.azure.com/.default", 0); err != nil || tok != "tok-1" {
		t.Fatalf("token = %q, %v; want tok-1", tok, err)
	}
}

// The SDK retries throttling and server errors: Entra ID twice and IMDS four times.
func TestRetries(t *testing.T) {
	tests := []struct {
		name         string
		typ          string
		wantRequests int
	}{
		{name: "Entra ID", typ: WorkloadIdentity, wantRequests: 3},
		{name: "IMDS", typ: ManagedIdentity, wantRequests: 5},
		{name: "Container Apps", typ: ContainerApps, wantRequests: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWorkloadIdentityEnv(t, "federated-token")
			f := useFakeAzure(t, answer(http.StatusServiceUnavailable, oauthError("temporarily_unavailable", "")))
			if tt.typ == ContainerApps {
				setContainerAppsEnv(t)
			}

			mustFail(t, mustGet(t, Identity{Type: tt.typ, ClientID: testClientID}), uniqueScope(t))
			if n := len(f.tokenRequests()); n != tt.wantRequests {
				t.Fatalf("token requests = %d, want %d", n, tt.wantRequests)
			}
		})
	}

	// IMDS answers 404 while it starts, so its retries include 404; Container Apps' don't.
	t.Run("Container Apps doesn't retry a 404", func(t *testing.T) {
		f := useFakeAzure(t, answer(http.StatusNotFound, rawBody("")))
		setContainerAppsEnv(t)

		mustFail(t, mustGet(t, Identity{Type: ContainerApps, ClientID: testClientID}), uniqueScope(t))
		if n := len(f.tokenRequests()); n != 1 {
			t.Fatalf("token requests = %d, want 1", n)
		}
	})

	t.Run("a one-time throttle", func(t *testing.T) {
		setWorkloadIdentityEnv(t, "federated-token")
		var f *fakeAzure
		f = useFakeAzure(t, func(context.Context, tokenRequest) (int, any) {
			if len(f.tokenRequests()) == 1 {
				return http.StatusTooManyRequests, oauthError("temporarily_unavailable", "")
			}
			return http.StatusOK, entraToken("tok-1")
		})

		cred := mustGet(t, Identity{Type: WorkloadIdentity})
		if tok, err := cred.Token(context.Background(), "https://ai.azure.com/.default", 5*time.Second); err != nil || tok != "tok-1" {
			t.Fatalf("token = %q, %v; want tok-1 after one retry", tok, err)
		}
		if n := len(f.tokenRequests()); n != 2 {
			t.Fatalf("token requests = %d, want 2", n)
		}
	})
}

// A refused token request is answered again for about two seconds without asking the service, so a
// misconfigured identity doesn't send a token request for every API request.
func TestARejectionIsCachedBriefly(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	f := useFakeAzure(t, answer(http.StatusUnauthorized, oauthError("invalid_client", "AADSTS700213")))
	const scope = "https://ai.azure.com/.default"

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	first := mustFail(t, cred, scope)
	if first.Cached {
		t.Fatal("the first failure was served from the cache")
	}
	for range 2 {
		again := mustFail(t, cred, scope)
		if !again.Cached || again.Reason != first.Reason {
			t.Fatalf("error = %+v, want the cached %q", again, first.Reason)
		}
	}
	if n := len(f.tokenRequests()); n != 1 {
		t.Fatalf("token requests = %d, want 1", n)
	}

	// Another scope of the same identity is not affected.
	mustFail(t, cred, "https://cognitiveservices.azure.com/.default")
	if n := len(f.tokenRequests()); n != 2 {
		t.Fatalf("token requests = %d, want 2", n)
	}

	// Once the rejection expires, the next call asks again.
	cred.mu.Lock()
	cred.rejections[scope] = rejection{err: first, until: time.Now().Add(-time.Millisecond)}
	cred.mu.Unlock()
	if again := mustFail(t, cred, scope); again.Cached {
		t.Fatal("an expired rejection was served from the cache")
	}
	if n := len(f.tokenRequests()); n != 3 {
		t.Fatalf("token requests = %d, want 3", n)
	}
}

// Throttling and server errors can clear on their own, so they are not cached.
func TestTransientFailuresAreNotCached(t *testing.T) {
	setWorkloadIdentityEnv(t, "federated-token")
	f := useFakeAzure(t, answer(http.StatusTooManyRequests, oauthError("temporarily_unavailable", "")))

	cred := mustGet(t, Identity{Type: WorkloadIdentity})
	mustFail(t, cred, "https://ai.azure.com/.default")
	if again := mustFail(t, cred, "https://ai.azure.com/.default"); again.Cached {
		t.Fatal("a throttled request was cached")
	}
	if n := len(f.tokenRequests()); n != 6 {
		t.Fatalf("token requests = %d, want 6 (two calls of three tries)", n)
	}
}
