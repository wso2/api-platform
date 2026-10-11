/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/proxy"
)

const (
	docsTestJWT      = "test-jwt"
	docsTestAPIKey   = "test-docs-api-key"
	docsTestQuestion = "How do I create an LLM proxy?"
)

// docsUpstreamCall is what the fake assistant received.
type docsUpstreamCall struct {
	path    string
	header  http.Header
	payload map[string]any
}

// fakeDocsAssistant answers /chat with the given status and body, after delay, and
// records each call.
func fakeDocsAssistant(t *testing.T, status int, body string, delay time.Duration) (*httptest.Server, *[]docsUpstreamCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []docsUpstreamCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)
		mu.Lock()
		calls = append(calls, docsUpstreamCall{path: r.URL.Path, header: r.Header.Clone(), payload: payload})
		mu.Unlock()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// fakeSessionCheck stands in for the Platform API's organization list, the read used
// to confirm the session. It answers with status.
func fakeSessionCheck(t *testing.T, status int) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	return fakeControlPlane(t, map[string]struct {
		status int
		body   string
	}{
		"GET /api/v0.9/organizations": {status, `{"list":[]}`},
	})
}

// docsTestServer wires a Server with the docs assistant pointed at assistantURL and
// the Platform API at platformURL. timeout bounds the assistant call.
func docsTestServer(t *testing.T, platformURL, assistantURL string, timeout time.Duration) *Server {
	t.Helper()
	transport, err := proxy.NewTransport(
		config.HTTPClientConfig{Timeouts: config.HTTPClientTimeoutsConfig{MaxResponseBytes: -1}},
		proxy.TLSClientOptions{SkipVerify: true},
	)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	cfg := &config.Config{
		ControlPlane: config.ControlPlaneConfig{URL: platformURL},
		Cookie:       config.CookieConfig{Name1: "_ai_workspace_session_1", Name2: "_ai_workspace_session_2"},
		DocsAssistant: config.DocsAssistantConfig{
			Endpoint: assistantURL + "/",
			APIKey:   docsTestAPIKey,
			Product:  "bijira",
			Branch:   "main",
			Timeout:  timeout,
		},
	}
	client, err := newDocsAssistantClient(
		config.HTTPClientConfig{Timeouts: config.HTTPClientTimeoutsConfig{MaxResponseBytes: -1}}, timeout)
	if err != nil {
		t.Fatalf("newDocsAssistantClient: %v", err)
	}
	return &Server{
		cfg:           cfg,
		proxy:         proxy.ReverseProxy(mustParseURL(platformURL), paths.Proxy, transport),
		docsAssistant: client,
		refreshLocks:  make(map[string]*refreshLock),
	}
}

// postChat sends body to the handler, with a session cookie unless jwt is empty.
func postChat(t *testing.T, s *Server, jwt, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/ai-workspace/api/docs-assistant/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if jwt != "" {
		addSessionCookies(req, s.cfg.Cookie, jwt)
	}
	rec := httptest.NewRecorder()
	s.handleDocsAssistantChat(rec, req)
	return rec
}

func chatBody(question, questionContext string) string {
	b, _ := json.Marshal(map[string]string{"question": question, "questionContext": questionContext})
	return string(b)
}

func decodeChatResponse(t *testing.T, rec *httptest.ResponseRecorder) docsAssistantChatResponse {
	t.Helper()
	var out docsAssistantChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v (%q)", err, rec.Body.String())
	}
	return out
}

func TestDocsAssistantChat_ForwardsQuestionAndReturnsContent(t *testing.T) {
	platform, platformCalls := fakeSessionCheck(t, http.StatusOK)
	assistant, calls := fakeDocsAssistant(t, http.StatusOK,
		`{"content":"Create it from LLM Proxies.","usage":{"prompt_tokens":10}}`, 0)
	s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

	rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, "User: hi\nAssistant: hello"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := decodeChatResponse(t, rec); got.Content != "Create it from LLM Proxies." || got.ErrorCode != "" {
		t.Errorf("response = %+v, want only the assistant's content", got)
	}
	if strings.Contains(rec.Body.String(), "usage") {
		t.Errorf("response passed through upstream fields: %s", rec.Body.String())
	}

	if len(*platformCalls) != 1 || (*platformCalls)[0].auth != "Bearer "+docsTestJWT {
		t.Fatalf("session check calls = %+v, want one call with the user's token", *platformCalls)
	}
	if len(*calls) != 1 {
		t.Fatalf("assistant calls = %d, want 1", len(*calls))
	}
	call := (*calls)[0]
	if call.path != "/chat" {
		t.Errorf("path = %q, want /chat", call.path)
	}
	for header, want := range map[string]string{
		"Api-Key":      docsTestAPIKey,
		"Product":      "bijira",
		"Branch":       "main",
		"Content-Type": "application/json",
	} {
		if got := call.header.Get(header); got != want {
			t.Errorf("header %s = %q, want %q", header, got, want)
		}
	}
	if call.header.Get("Authorization") != "" || call.header.Get("Cookie") != "" {
		t.Errorf("user credentials reached the assistant: %v", call.header)
	}
	questions, _ := call.payload["questions"].([]any)
	if len(questions) != 1 || questions[0] != docsTestQuestion {
		t.Errorf("questions = %v, want [%q]", call.payload["questions"], docsTestQuestion)
	}
	if call.payload["question_context"] != "User: hi\nAssistant: hello" {
		t.Errorf("question_context = %v", call.payload["question_context"])
	}
}

func TestDocsAssistantChat_OmitsEmptyContext(t *testing.T) {
	platform, _ := fakeSessionCheck(t, http.StatusOK)
	assistant, calls := fakeDocsAssistant(t, http.StatusOK, `{"content":"ok"}`, 0)
	s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

	rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, "   "))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if _, present := (*calls)[0].payload["question_context"]; present {
		t.Errorf("question_context was sent for an empty context: %v", (*calls)[0].payload)
	}
}

func TestDocsAssistantChat_PassesContentFilterCode(t *testing.T) {
	platform, _ := fakeSessionCheck(t, http.StatusOK)
	assistant, _ := fakeDocsAssistant(t, http.StatusOK,
		`{"content":"Please rephrase.","error":{"code":"content_filtered","message":"Please rephrase."}}`, 0)
	s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

	rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, ""))
	got := decodeChatResponse(t, rec)
	if rec.Code != http.StatusOK || got.Content != "Please rephrase." || got.ErrorCode != "content_filtered" {
		t.Errorf("status %d, response %+v; want 200 with the message and content_filtered", rec.Code, got)
	}
}

func TestDocsAssistantChat_RequiresSession(t *testing.T) {
	cases := []struct {
		name        string
		jwt         string
		checkStatus int
	}{
		{"no cookie", "", http.StatusOK},
		{"token the Platform API rejects", "forged-token", http.StatusUnauthorized},
		{"token without access", "other-token", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			platform, _ := fakeSessionCheck(t, tc.checkStatus)
			assistant, calls := fakeDocsAssistant(t, http.StatusOK, `{"content":"ok"}`, 0)
			s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

			rec := postChat(t, s, tc.jwt, chatBody(docsTestQuestion, ""))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if len(*calls) != 0 {
				t.Errorf("assistant was called %d time(s) without a valid session", len(*calls))
			}
		})
	}
}

func TestDocsAssistantChat_SessionCheckFailureIs502(t *testing.T) {
	platform, _ := fakeSessionCheck(t, http.StatusInternalServerError)
	assistant, calls := fakeDocsAssistant(t, http.StatusOK, `{"content":"ok"}`, 0)
	s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

	rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, ""))
	if rec.Code != http.StatusBadGateway || len(*calls) != 0 {
		t.Errorf("status = %d, assistant calls = %d; want 502 and no call", rec.Code, len(*calls))
	}
}

func TestDocsAssistantChat_RejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"not JSON", "not json", http.StatusBadRequest},
		{"empty question", chatBody("   ", ""), http.StatusBadRequest},
		{"question too long", chatBody(strings.Repeat("a", maxDocsAssistantQuestionChars+1), ""), http.StatusBadRequest},
		{"context too long", chatBody(docsTestQuestion, strings.Repeat("a", maxDocsAssistantContextChars+1)), http.StatusBadRequest},
		{"body too large", chatBody(docsTestQuestion, strings.Repeat("a", maxDocsAssistantRequestBytes)), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			platform, _ := fakeSessionCheck(t, http.StatusOK)
			assistant, calls := fakeDocsAssistant(t, http.StatusOK, `{"content":"ok"}`, 0)
			s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

			rec := postChat(t, s, docsTestJWT, tc.body)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if len(*calls) != 0 {
				t.Errorf("assistant was called for invalid input")
			}
		})
	}
}

func TestDocsAssistantChat_UpstreamErrorIsHidden(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"5xx", http.StatusInternalServerError, `{"detail":"secret internal detail"}`},
		{"empty answer", http.StatusOK, `{"content":""}`},
		{"not JSON", http.StatusOK, `<html>secret internal detail</html>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			platform, _ := fakeSessionCheck(t, http.StatusOK)
			assistant, _ := fakeDocsAssistant(t, tc.status, tc.body, 0)
			s := docsTestServer(t, platform.URL, assistant.URL, 5*time.Second)

			rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, ""))
			if rec.Code != http.StatusBadGateway {
				t.Errorf("status = %d, want 502", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "DOCS_ASSISTANT_UPSTREAM_ERROR") {
				t.Errorf("body = %s, want the DOCS_ASSISTANT_UPSTREAM_ERROR code", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "secret internal detail") {
				t.Errorf("upstream body leaked to the browser: %s", rec.Body.String())
			}
		})
	}
}

func TestDocsAssistantChat_TimeoutIs504(t *testing.T) {
	platform, _ := fakeSessionCheck(t, http.StatusOK)
	assistant, _ := fakeDocsAssistant(t, http.StatusOK, `{"content":"late"}`, 2*time.Second)
	s := docsTestServer(t, platform.URL, assistant.URL, 200*time.Millisecond)

	rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, ""))
	if rec.Code != http.StatusGatewayTimeout || !strings.Contains(rec.Body.String(), "DOCS_ASSISTANT_TIMEOUT") {
		t.Errorf("status = %d, body = %s; want 504 DOCS_ASSISTANT_TIMEOUT", rec.Code, rec.Body.String())
	}
}

// A redirect is never followed: the API key is a custom header, which net/http would
// copy onto a redirect to another host.
func TestDocsAssistantChat_DoesNotFollowRedirects(t *testing.T) {
	var gotKey string
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"content":"from elsewhere"}`)
	}))
	t.Cleanup(elsewhere.Close)
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/chat", http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redirecting.Close)
	platform, _ := fakeSessionCheck(t, http.StatusOK)
	s := docsTestServer(t, platform.URL, redirecting.URL, 5*time.Second)

	rec := postChat(t, s, docsTestJWT, chatBody(docsTestQuestion, ""))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 for a redirect", rec.Code)
	}
	if gotKey != "" {
		t.Errorf("the API key was sent to the redirect target")
	}
}

// Questions, context, answers and the API key are never logged, on success or failure.
func TestDocsAssistantChat_DoesNotLogUserContentOrKey(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	platform, _ := fakeSessionCheck(t, http.StatusOK)
	ok, _ := fakeDocsAssistant(t, http.StatusOK, `{"content":"the private answer"}`, 0)
	failing, _ := fakeDocsAssistant(t, http.StatusInternalServerError, `{"detail":"the private answer"}`, 0)
	for _, upstream := range []string{ok.URL, failing.URL} {
		s := docsTestServer(t, platform.URL, upstream, 5*time.Second)
		postChat(t, s, docsTestJWT, chatBody("my private question", "my private context"))
	}

	for _, secret := range []string{"my private question", "my private context", "the private answer", docsTestAPIKey} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("logs contain %q:\n%s", secret, logs.String())
		}
	}
}

// Through the full route chain: unconfigured answers 503 (not the SPA fallback), and a
// POST without the CSRF header is refused before the handler runs.
func TestRoutesDocsAssistant(t *testing.T) {
	s := routesTestServer(t) // no [docs_assistant], so s.docsAssistant stays nil

	t.Run("not configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ai-workspace/api/docs-assistant/chat",
			strings.NewReader(chatBody(docsTestQuestion, "")))
		req.Header.Set(config.CSRFHeaderName, "ai-workspace")
		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "DOCS_ASSISTANT_NOT_CONFIGURED") {
			t.Errorf("status = %d, body = %s; want 503 DOCS_ASSISTANT_NOT_CONFIGURED", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing CSRF header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/ai-workspace/api/docs-assistant/chat",
			strings.NewReader(chatBody(docsTestQuestion, "")))
		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}
