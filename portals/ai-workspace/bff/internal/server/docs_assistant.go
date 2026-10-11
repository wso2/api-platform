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

// Ask AI: the BFF calls the documentation assistant server-side with an API key from
// [docs_assistant], so the key never reaches the browser.
//
// Questions, conversation context and answers are user content and are never logged.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"ai-workspace-bff/internal/config"
	"ai-workspace-bff/internal/paths"
	"ai-workspace-bff/internal/proxy"
)

const (
	// maxDocsAssistantRequestBytes caps the browser's request body: a question plus
	// the recent conversation the SPA sends as context.
	maxDocsAssistantRequestBytes = 32 << 10
	// maxDocsAssistantQuestionChars and maxDocsAssistantContextChars bound the two
	// fields in characters, inside the byte cap above.
	maxDocsAssistantQuestionChars = 2000
	maxDocsAssistantContextChars  = 20000
	// maxDocsAssistantResponseBytes caps what is read back from the assistant.
	maxDocsAssistantResponseBytes = 1 << 20
	// sessionCheckPath is a cheap authenticated Platform API read, used to confirm the
	// session before calling the assistant (see verifySession).
	sessionCheckPath = paths.PlatformAPI + "/organizations?limit=1&offset=0"
)

type docsAssistantChatRequest struct {
	Question        string `json:"question"`
	QuestionContext string `json:"questionContext"`
}

// docsAssistantUpstreamRequest is the assistant's /chat contract.
type docsAssistantUpstreamRequest struct {
	Questions       []string `json:"questions"`
	QuestionContext string   `json:"question_context,omitempty"`
}

type docsAssistantUpstreamResponse struct {
	Content string `json:"content"`
	Error   *struct {
		Code string `json:"code"`
	} `json:"error"`
}

type docsAssistantChatResponse struct {
	Content string `json:"content"`
	// ErrorCode is set when the assistant answered with a handled error, such as
	// "content_filtered"; Content then holds the user-facing message.
	ErrorCode string `json:"errorCode,omitempty"`
}

// newDocsAssistantClient builds the client for the assistant. Its own transport, so the
// response-header timeout can cover answer generation (the shared default is 10s), and
// so no TLS setting of another upstream applies to this one.
func newDocsAssistantClient(hc config.HTTPClientConfig, timeout time.Duration) (*http.Client, error) {
	hc.Timeouts.Overall = timeout
	hc.Timeouts.ResponseHeader = timeout
	transport, err := proxy.NewTransport(hc, proxy.TLSClientOptions{})
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// Every request carries the API key, so never follow a redirect that drops TLS.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect from https to %s://%s (would send credentials in cleartext)",
					req.URL.Scheme, req.URL.Host)
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}, nil
}

// handleDocsAssistantChat (POST <base>/api/docs-assistant/chat) answers one Ask AI
// question. Always registered: when [docs_assistant] is not configured it says so
// with a 503 instead of falling through to the SPA handler.
func (s *Server) handleDocsAssistantChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	trackingID := w.Header().Get("X-Request-Id")

	if s.docsAssistant == nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "DOCS_ASSISTANT_NOT_CONFIGURED", "Ask AI is not available")
		return
	}

	if !s.verifySession(w, r) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxDocsAssistantRequestBytes)
	var req docsAssistantChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeErrorJSON(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "request body is too large")
			return
		}
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	question := strings.TrimSpace(req.Question)
	questionContext := strings.TrimSpace(req.QuestionContext)
	if question == "" {
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "question is required")
		return
	}
	if utf8.RuneCountInString(question) > maxDocsAssistantQuestionChars ||
		utf8.RuneCountInString(questionContext) > maxDocsAssistantContextChars {
		writeErrorJSON(w, http.StatusBadRequest, "INVALID_REQUEST", "question or context is too long")
		return
	}

	start := time.Now()
	resp, status, err := s.callDocsAssistant(r.Context(), question, questionContext, trackingID)
	dur := time.Since(start).Milliseconds()
	switch {
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		slog.Warn("docs assistant timed out", "request_id", trackingID, "dur_ms", dur)
		writeServerErrorJSON(w, http.StatusGatewayTimeout, "DOCS_ASSISTANT_TIMEOUT",
			"Ask AI took too long to answer", trackingID)
	case err != nil:
		slog.Warn("docs assistant call failed", "request_id", trackingID, "status", status, "dur_ms", dur, "err", err)
		writeServerErrorJSON(w, http.StatusBadGateway, "DOCS_ASSISTANT_UPSTREAM_ERROR",
			"Ask AI could not answer right now", trackingID)
	default:
		slog.Info("docs assistant answered", "request_id", trackingID, "dur_ms", dur)
		writeJSON(w, http.StatusOK, resp)
	}
}

// verifySession confirms the caller has a session the Platform API accepts. The BFF
// never verifies tokens itself, and this route forwards no user token to anyone, so
// without this check a cookie that merely looks like a session would be enough to use
// the assistant. Asking the Platform API is authoritative in every auth mode. On
// failure it has already written the response.
func (s *Server) verifySession(w http.ResponseWriter, r *http.Request) bool {
	jwt, ok := s.sessionToken(w, r)
	if !ok {
		return false
	}
	upstream, err := s.upstreamToken(r.Context(), jwt)
	if err != nil {
		slog.Warn("token exchange failed for an Ask AI request", "err", err)
		s.writeExchangeError(w, r, err)
		return false
	}
	resp, err := s.platformDo(r.Context(), upstream, http.MethodGet, sessionCheckPath, nil, nil)
	if err != nil {
		slog.Warn("could not confirm the session for an Ask AI request", "err", err)
		writeErrorJSON(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream temporarily unavailable")
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDocsAssistantResponseBytes))
	_ = resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		return true
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		writeErrorJSON(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired credentials.")
		return false
	default:
		slog.Warn("unexpected status confirming the session for an Ask AI request", "status", resp.StatusCode)
		writeErrorJSON(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream temporarily unavailable")
		return false
	}
}

// callDocsAssistant sends one question to the assistant and returns its answer. The
// returned status is the upstream HTTP status (0 when no response arrived). Errors never
// carry the upstream body, which could echo user content.
func (s *Server) callDocsAssistant(ctx context.Context, question, questionContext, requestID string) (*docsAssistantChatResponse, int, error) {
	cfg := s.cfg.DocsAssistant
	body, err := json.Marshal(docsAssistantUpstreamRequest{
		Questions:       []string{question},
		QuestionContext: questionContext,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("encoding the request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint+"chat", bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", cfg.APIKey)
	req.Header.Set("product", cfg.Product)
	req.Header.Set("branch", cfg.Branch)
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}

	resp, err := s.docsAssistant.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxDocsAssistantResponseBytes))
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading the response: %w", readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("assistant answered %d", resp.StatusCode)
	}
	var up docsAssistantUpstreamResponse
	if err := json.Unmarshal(raw, &up); err != nil {
		return nil, resp.StatusCode, errors.New("assistant returned a response that is not valid JSON")
	}
	if strings.TrimSpace(up.Content) == "" {
		return nil, resp.StatusCode, errors.New("assistant returned an empty answer")
	}
	out := &docsAssistantChatResponse{Content: up.Content}
	if up.Error != nil {
		out.ErrorCode = up.Error.Code
	}
	return out, resp.StatusCode, nil
}

// isTimeout reports whether err is a client timeout, which net/http reports through
// net.Error rather than context.DeadlineExceeded.
func isTimeout(err error) bool {
	var te interface{ Timeout() bool }
	return errors.As(err, &te) && te.Timeout()
}
