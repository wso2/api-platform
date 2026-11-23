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

package interceptor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2/api-platform/tests/framework/testbench"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// payload builds a contract-shaped call for the phase with the given request headers.
func payload(t *testing.T, ph phase, headers map[string]string) string {
	t.Helper()
	body := map[string]any{"invocationContext": map[string]any{"requestId": "req-1", "path": "/api/v1/items"}}
	if headers != nil {
		body["requestHeaders"] = headers
	}
	if ph == phaseResponse {
		body["responseCode"] = 200
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func call(h http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func post(h http.Handler, path, body string) *httptest.ResponseRecorder {
	return call(h, http.MethodPost, path, jsonContentType, body)
}

// controlName builds a control header name as a client would write it.
func controlName(ph phase, control string) string {
	return "X-Testbench-Interceptor-" + map[phase]string{phaseRequest: "Request", phaseResponse: "Response"}[ph] + "-" + control
}

func hookPath(ph phase) string { return "/scripted/case-1/handle-" + string(ph) }

func TestServiceMetadata(t *testing.T) {
	s := New()
	if s.Name() != "interceptor" || s.Port() != 3003 || s.Stateful() {
		t.Fatalf("got name %q port %d stateful %v", s.Name(), s.Port(), s.Stateful())
	}
}

func TestFixedRepliesAreUnchanged(t *testing.T) {
	h := New().Handler()
	ctx := func(path string) string { return `{"invocationContext":{"path":"` + path + `"}}` }
	cases := []struct {
		name, path, body, want string
	}{
		{"block", "/handle-request", ctx("/api/block"),
			`{"directRespond":true,"responseCode":403,"headersToAdd":{"Content-Type":"application/json","X-Interceptor-Decision":"blocked","X-Interceptor-RequestHook":"true"},"body":"` + b64(`{"error":"blocked by interceptor"}`) + `"}`},
		{"mutate", "/handle-request", ctx("/api/mutate"),
			`{"headersToAdd":{"X-Interceptor-Request":"true"},"pathToRewrite":"/anything/intercepted","body":"` + b64(`{"message":"mutated-by-interceptor"}`) + `","interceptorContext":{"trace":"request-phase"}}`},
		{"response-rewrite request", "/handle-request", ctx("/api/response-rewrite"),
			`{"interceptorContext":{"trace":"request-phase"}}`},
		{"response-rewrite response", "/handle-response",
			`{"invocationContext":{"path":"/api/response-rewrite"},"interceptorContext":{"trace":"request-phase"}}`,
			`{"responseCode":202,"headersToAdd":{"Content-Type":"application/json","X-Interceptor-Response":"true","X-Interceptor-Trace":"request-phase"},"body":"` + b64(`{"message":"response-overridden"}`) + `"}`},
		{"response-rewrite without context", "/handle-response", ctx("/api/response-rewrite"),
			`{"responseCode":202,"headersToAdd":{"Content-Type":"application/json","X-Interceptor-Response":"true","X-Interceptor-Trace":"missing"},"body":"` + b64(`{"message":"response-overridden"}`) + `"}`},
		{"default request", "/handle-request", ctx("/api/other"), `{}`},
		{"default response", "/handle-response", ctx("/api/other"), `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := post(h, tc.path, tc.body)
			if rec.Code != http.StatusOK || rec.Body.String() != tc.want {
				t.Fatalf("got %d %s, want 200 %s", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}
	if rec := call(h, http.MethodGet, "/handle-request", "", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /handle-request: got %d", rec.Code)
	}
}

func TestScriptedDefaultReplyIsAnEmptyObject(t *testing.T) {
	h := New().Handler()
	for _, ph := range []phase{phaseRequest, phaseResponse} {
		rec := post(h, hookPath(ph), payload(t, ph, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "{}" || rec.Header().Get("Content-Type") != jsonContentType {
			t.Fatalf("%s: got %d %q %q", ph, rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
		}
	}
}

func TestScriptedReplyBytesAreSentAsGiven(t *testing.T) {
	h := New().Handler()
	for _, want := range []string{"not json", "[]", `{"headersToAdd":["a"]}`, `{"headersToAdd":{"x":`, `{"headersToAdd":{"X-A":"1"}}`} {
		for _, ph := range []phase{phaseRequest, phaseResponse} {
			hdr := map[string]string{controlName(ph, "Reply"): b64(want)}
			rec := post(h, hookPath(ph), payload(t, ph, hdr))
			if rec.Code != http.StatusOK || rec.Body.String() != want {
				t.Fatalf("%s %q: got %d %q", ph, want, rec.Code, rec.Body.String())
			}
		}
	}
}

func TestScriptedStatus(t *testing.T) {
	h := New().Handler()
	reply := `{"headersToAdd":{"X-Should-Not":"apply"}}`
	for _, tc := range []struct {
		ph     phase
		status string
		want   int
	}{
		{phaseRequest, "403", 403}, {phaseRequest, "503", 503}, {phaseRequest, "200", 200},
		{phaseResponse, "400", 400}, {phaseResponse, "502", 502}, {phaseResponse, "599", 599},
		{phaseRequest, "199", 400}, {phaseRequest, "600", 400}, {phaseRequest, "abc", 400}, {phaseRequest, "", 400},
	} {
		hdr := map[string]string{controlName(tc.ph, "Status"): tc.status, controlName(tc.ph, "Reply"): b64(reply)}
		rec := post(h, hookPath(tc.ph), payload(t, tc.ph, hdr))
		if rec.Code != tc.want {
			t.Fatalf("%s status %q: got %d, want %d (%s)", tc.ph, tc.status, rec.Code, tc.want, rec.Body.String())
		}
		if tc.want < 400 || tc.status == "400" || tc.status == "403" || tc.status == "502" || tc.status == "503" {
			if rec.Body.String() != reply {
				t.Fatalf("%s status %q: body %q, want the reply", tc.ph, tc.status, rec.Body.String())
			}
		}
	}
}

func TestScriptedDelay(t *testing.T) {
	h := New().Handler()
	hdr := map[string]string{"X-Testbench-Interceptor-Request-Delay-Ms": "60"}
	started := time.Now()
	rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr))
	if elapsed := time.Since(started); rec.Code != http.StatusOK || elapsed < 60*time.Millisecond {
		t.Fatalf("got %d after %s, want 200 after at least 60ms", rec.Code, elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, hookPath(phaseRequest),
		strings.NewReader(payload(t, phaseRequest, map[string]string{"X-Testbench-Interceptor-Request-Delay-Ms": "10000"}))).WithContext(ctx)
	req.Header.Set("Content-Type", jsonContentType)
	rec = httptest.NewRecorder()
	started = time.Now()
	h.ServeHTTP(rec, req)
	if time.Since(started) > time.Second || rec.Body.Len() != 0 {
		t.Fatalf("a cancelled call should return at once without a reply, got %q", rec.Body.String())
	}

	for value, ok := range map[string]bool{"0": true, "10000": true, "-1": false, "10001": false, "1.5": false} {
		_, err := parseControls(phaseRequest, map[string]string{"x-testbench-interceptor-request-delay-ms": value}, false)
		if (err == nil) != ok {
			t.Fatalf("delay %q: err %v, want ok=%v", value, err, ok)
		}
	}
}

func TestScriptedReflect(t *testing.T) {
	h := New().Handler()
	decode := func(t *testing.T, rec *httptest.ResponseRecorder) reply {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d %s", rec.Code, rec.Body.String())
		}
		var r reply
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}

	reqBody := payload(t, phaseRequest, map[string]string{"X-Testbench-Interceptor-Request-Mode": "REFLECT", "X-Custom": "v"})
	r := decode(t, post(h, hookPath(phaseRequest), reqBody))
	if !r.DirectRespond || r.ResponseCode != 200 || r.HeadersToAdd["Content-Type"] != jsonContentType || r.Body != b64(reqBody) || r.HeadersToReplace != nil {
		t.Fatalf("request reflect: got %+v", r)
	}

	respBody := payload(t, phaseResponse, map[string]string{"x-testbench-interceptor-response-mode": "reflect"})
	r = decode(t, post(h, hookPath(phaseResponse), respBody))
	if r.DirectRespond || r.ResponseCode != 0 || r.HeadersToReplace["Content-Type"] != jsonContentType || r.Body != b64(respBody) || r.HeadersToAdd != nil {
		t.Fatalf("response reflect: got %+v", r)
	}

	plain := payload(t, phaseResponse, nil)
	r = decode(t, post(h, "/scripted/case-1/reflect/handle-response", plain))
	if r.Body != b64(plain) {
		t.Fatalf("path reflect: got %+v", r)
	}
	plain = payload(t, phaseRequest, nil)
	r = decode(t, post(h, "/scripted/case-1/reflect/handle-request", plain))
	if !r.DirectRespond || r.Body != b64(plain) {
		t.Fatalf("path reflect request: got %+v", r)
	}

	for name, hdr := range map[string]map[string]string{
		"with reply":   {"X-Testbench-Interceptor-Request-Mode": "reflect", "X-Testbench-Interceptor-Request-Reply": b64("{}")},
		"with status":  {"X-Testbench-Interceptor-Request-Mode": "reflect", "X-Testbench-Interceptor-Request-Status": "200"},
		"unknown mode": {"X-Testbench-Interceptor-Request-Mode": "echo"},
	} {
		if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr)); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d", name, rec.Code)
		}
	}
	hdr := map[string]string{"X-Testbench-Interceptor-Request-Reply": b64("{}")}
	if rec := post(h, "/scripted/case-1/reflect/handle-request", payload(t, phaseRequest, hdr)); rec.Code != http.StatusBadRequest {
		t.Fatalf("path reflect with reply: got %d", rec.Code)
	}
}

func TestScriptedControlsMatchWithoutRegardToCaseAndPerPhase(t *testing.T) {
	h := New().Handler()
	hdr := map[string]string{
		"x-TESTBENCH-interceptor-request-reply":   b64(`{"a":1}`),
		"X-Testbench-Interceptor-Response-Status": "503",
	}
	if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr)); rec.Code != 200 || rec.Body.String() != `{"a":1}` {
		t.Fatalf("request phase: got %d %q", rec.Code, rec.Body.String())
	}
	if rec := post(h, hookPath(phaseResponse), payload(t, phaseResponse, hdr)); rec.Code != 503 || rec.Body.String() != "{}" {
		t.Fatalf("response phase: got %d %q", rec.Code, rec.Body.String())
	}

	// A control repeated in two cases is checked only in its own phase.
	hdr = map[string]string{
		"X-Testbench-Interceptor-Response-Status": "200",
		"x-testbench-interceptor-response-status": "503",
	}
	if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr)); rec.Code != 200 {
		t.Fatalf("request phase with a repeated response control: got %d %q", rec.Code, rec.Body.String())
	}
}

func TestScriptedLocation(t *testing.T) {
	h := New().Handler()
	for _, ph := range []phase{phaseRequest, phaseResponse} {
		hdr := map[string]string{
			controlName(ph, "Status"):   "307",
			controlName(ph, "Location"): "https://elsewhere.test/hook?x=1",
		}
		rec := post(h, hookPath(ph), payload(t, ph, hdr))
		if rec.Code != http.StatusTemporaryRedirect || rec.Header().Get("Location") != "https://elsewhere.test/hook?x=1" || rec.Body.String() != "{}" {
			t.Fatalf("%s: got %d Location %q body %q", ph, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
	}
	if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, nil)); rec.Header().Get("Location") != "" {
		t.Fatalf("no Location control must send no Location header, got %q", rec.Header().Get("Location"))
	}
	other := map[string]string{controlName(phaseResponse, "Location"): "http://elsewhere.test"}
	if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, other)); rec.Header().Get("Location") != "" {
		t.Fatalf("the other phase's Location must be ignored, got %q", rec.Header().Get("Location"))
	}
	for _, value := range []string{"/relative", "ftp://elsewhere.test", "http://", "not a url", "http://elsewhere.test/%zz"} {
		hdr := map[string]string{controlName(phaseRequest, "Location"): value}
		if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr)); rec.Code != http.StatusBadRequest {
			t.Fatalf("Location %q: got %d, want 400", value, rec.Code)
		}
	}
	hdr := map[string]string{controlName(phaseRequest, "Mode"): "reflect", controlName(phaseRequest, "Location"): "http://elsewhere.test"}
	if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr)); rec.Code != http.StatusBadRequest {
		t.Fatalf("reflect with a location: got %d, want 400", rec.Code)
	}
}

func TestScriptedRejectsBadControls(t *testing.T) {
	h := New().Handler()
	for name, hdr := range map[string]map[string]string{
		"unknown control":   {"X-Testbench-Interceptor-Request-Colour": "red"},
		"url-safe base64":   {"X-Testbench-Interceptor-Request-Reply": "-_-_"},
		"not base64":        {"X-Testbench-Interceptor-Request-Reply": "%%%"},
		"negative delay":    {"X-Testbench-Interceptor-Request-Delay-Ms": "-5"},
		"non-numeric delay": {"X-Testbench-Interceptor-Request-Delay-Ms": "soon"},
		"same control in two cases": {
			"X-Testbench-Interceptor-Request-Status": "200",
			"x-testbench-interceptor-request-status": "503",
		},
	} {
		rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid interceptor control") {
			t.Fatalf("%s: got %d %q", name, rec.Code, rec.Body.String())
		}
	}
}

func TestScriptedRejectsBadIDsMethodsAndContentTypes(t *testing.T) {
	h := New().Handler()
	body := payload(t, phaseRequest, nil)
	if rec := post(h, "/scripted/"+strings.Repeat("a", 128)+"/handle-request", body); rec.Code != http.StatusOK {
		t.Fatalf("128-character id: got %d", rec.Code)
	}
	for _, id := range []string{strings.Repeat("a", 129), "a.b", "a~b"} {
		if rec := post(h, "/scripted/"+id+"/handle-request", body); rec.Code != http.StatusBadRequest {
			t.Fatalf("id %q: got %d", id, rec.Code)
		}
	}
	if rec := call(h, http.MethodGet, hookPath(phaseRequest), "", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: got %d", rec.Code)
	}
	for ct, want := range map[string]int{
		"text/plain": http.StatusUnsupportedMediaType, "": http.StatusUnsupportedMediaType,
		"application/json; charset=utf-8": http.StatusOK, "Application/JSON": http.StatusOK,
	} {
		if rec := call(h, http.MethodPost, hookPath(phaseRequest), ct, body); rec.Code != want {
			t.Fatalf("content type %q: got %d, want %d", ct, rec.Code, want)
		}
	}
}

func TestScriptedRejectsPayloadsOutsideTheContract(t *testing.T) {
	h := New().Handler()
	for name, tc := range map[string]struct {
		ph   phase
		body string
	}{
		"not JSON":                       {phaseRequest, "nope"},
		"array":                          {phaseRequest, "[]"},
		"null":                           {phaseRequest, "null"},
		"trailing data":                  {phaseRequest, `{} {}`},
		"requestHeaders array":           {phaseRequest, `{"requestHeaders":["a"]}`},
		"requestHeaders number value":    {phaseRequest, `{"requestHeaders":{"a":1}}`},
		"requestHeaders null member":     {phaseRequest, `{"requestHeaders":{"X-Testbench-Interceptor-Request-Reply":null}}`},
		"responseHeaders null member":    {phaseResponse, `{"responseCode":200,"responseHeaders":{"a":null}}`},
		"interceptorContext null member": {phaseRequest, `{"interceptorContext":{"k":null}}`},
		"requestBody number":             {phaseRequest, `{"requestBody":5}`},
		"responseBody object":            {phaseResponse, `{"responseCode":200,"responseBody":{}}`},
		"responseHeaders string":         {phaseResponse, `{"responseCode":200,"responseHeaders":"a"}`},
		"invocationContext string":       {phaseRequest, `{"invocationContext":"x"}`},
		"interceptorContext number":      {phaseResponse, `{"responseCode":200,"interceptorContext":3}`},
		"responseCode missing":           {phaseResponse, `{"invocationContext":{}}`},
		"responseCode string":            {phaseResponse, `{"responseCode":"200"}`},
		"responseCode fraction":          {phaseResponse, `{"responseCode":200.5}`},
	} {
		rec := post(h, hookPath(tc.ph), tc.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid interceptor payload") {
			t.Fatalf("%s: got %d %q", name, rec.Code, rec.Body.String())
		}
	}
	for name, tc := range map[string]struct {
		ph   phase
		body string
	}{
		"empty object":         {phaseRequest, `{}`},
		"null fields":          {phaseRequest, `{"requestHeaders":null,"requestBody":null,"invocationContext":null,"interceptorContext":null}`},
		"empty string members": {phaseResponse, `{"responseCode":200,"requestHeaders":{"a":""},"responseHeaders":{"b":""},"interceptorContext":{"k":""}}`},
		"unknown field":        {phaseRequest, `{"somethingNew":[1,2]}`},
		"full response call":   {phaseResponse, `{"responseCode":201,"requestHeaders":{"a":"b"},"requestBody":"eA==","responseHeaders":{"c":"d"},"responseBody":"eQ==","invocationContext":{"requestId":"r"},"interceptorContext":{"k":"v"}}`},
	} {
		if rec := post(h, hookPath(tc.ph), tc.body); rec.Code != http.StatusOK {
			t.Fatalf("%s: got %d %q", name, rec.Code, rec.Body.String())
		}
	}
}

func TestScriptedRejectsAnOversizedBody(t *testing.T) {
	h := New().Handler()
	req := httptest.NewRequest(http.MethodPost, hookPath(phaseRequest), strings.NewReader(`{"requestBody":"`+strings.Repeat("x", 64)+`"}`))
	req.Header.Set("Content-Type", jsonContentType)
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 16)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestScriptedConcurrentCallsGetTheirOwnReplies(t *testing.T) {
	h := New().Handler()
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			want := fmt.Sprintf(`{"directRespond":true,"responseCode":%d}`, 200+i)
			hdr := map[string]string{"X-Testbench-Interceptor-Request-Reply": b64(want)}
			if rec := post(h, hookPath(phaseRequest), payload(t, phaseRequest, hdr)); rec.Body.String() != want {
				errs <- fmt.Errorf("call %d got %q", i, rec.Body.String())
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestOverTLSServesTheSameRepliesOverHTTPS(t *testing.T) {
	certPEM, keyPEM := selfSignedPEM(t)
	secure, err := New().OverTLS(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if secure.Name() != "interceptor-tls" || secure.Port() != TLSPort || secure.Stateful() {
		t.Fatalf("got name %q port %d stateful %v", secure.Name(), secure.Port(), secure.Stateful())
	}
	tlsService, ok := secure.(testbench.TLSService)
	if !ok || len(tlsService.TLSConfig().Certificates) != 1 {
		t.Fatal("the HTTPS service must declare its TLS configuration with one certificate")
	}

	srv := httptest.NewUnstartedServer(secure.Handler())
	srv.TLS = tlsService.TLSConfig()
	srv.StartTLS()
	defer srv.Close()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(certPEM) {
		t.Fatal("could not add the test certificate to the pool")
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "testbench"}}}

	hdr := map[string]string{controlName(phaseRequest, "Reply"): b64(`{"headersToAdd":{"X-A":"1"}}`)}
	resp, err := client.Post(srv.URL+hookPath(phaseRequest), jsonContentType, strings.NewReader(payload(t, phaseRequest, hdr)))
	if err != nil {
		t.Fatalf("a client trusting the certificate must connect: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != `{"headersToAdd":{"X-A":"1"}}` {
		t.Fatalf("scripted reply over HTTPS: got %d %s", resp.StatusCode, body)
	}

	resp, err = client.Post(srv.URL+"/handle-request", jsonContentType, strings.NewReader(`{"invocationContext":{"path":"/api/other"}}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "{}" {
		t.Fatalf("fixed reply over HTTPS: got %d %s", resp.StatusCode, body)
	}

	untrusting := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{ServerName: "testbench"}}}
	if _, err := untrusting.Post(srv.URL+"/handle-request", jsonContentType, strings.NewReader(`{}`)); err == nil {
		t.Fatal("a client that does not trust the certificate must fail the handshake")
	}
}

func TestOverTLSRejectsAnInvalidKeyPair(t *testing.T) {
	if _, err := New().OverTLS([]byte("not a certificate"), []byte("not a key")); err == nil {
		t.Fatal("want an error for an invalid key pair")
	}
}

func selfSignedPEM(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "testbench"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"testbench"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}
