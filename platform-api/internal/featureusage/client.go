/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package featureusage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// eventsPath is Moesif's API-call event endpoint, which the collector proxy
// exposes verbatim under its base URL. A backend feature occurrence is
// reported as the REST API call it happened in; /v1/actions is for UI
// interactions, which the console reports itself.
const eventsPath = "/v1/events"

// maxDrainBytes bounds how much of a response body is read (and discarded) so
// the connection can be reused. Bodies are never logged: a rejection can quote
// the identifiers that were sent.
const maxDrainBytes = 4 << 10

// eventBody is the request body of Moesif's Events API (one API call). Only
// what identifies the call's route and outcome is carried: no caller IP
// address, header values or bodies, which can hold personal data or
// credentials.
type eventBody struct {
	Request       eventRequest   `json:"request"`
	Response      eventResponse  `json:"response"`
	UserID        string         `json:"user_id,omitempty"`
	CompanyID     string         `json:"company_id"`
	TransactionID string         `json:"transaction_id,omitempty"`
	Metadata      map[string]any `json:"metadata"`
}

// Moesif's Events API requires request.headers and response.headers. They are
// always sent empty: no header of the call is reported.
type eventRequest struct {
	Time    string            `json:"time"`
	URI     string            `json:"uri"`
	Verb    string            `json:"verb"`
	Headers map[string]string `json:"headers"`
}

type eventResponse struct {
	Time    string            `json:"time"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// statusError is a non-2xx answer from the collector proxy or Moesif.
type statusError struct{ status int }

func (e *statusError) Error() string { return fmt.Sprintf("collector answered status %d", e.status) }

// collectorClient posts one event to the collector proxy, retrying as the
// proxy's integration guide prescribes: 2xx is done; 401 fetches a new token
// and retries once; 429, 5xx, timeouts and connection errors are retried with
// a linear backoff; any other 4xx is a verdict and is not retried.
type collectorClient struct {
	httpClient     *http.Client
	eventsURL      string
	tokens         TokenSource
	maxAttempts    int
	retryBackoff   time.Duration
	attemptTimeout time.Duration
}

// send posts payload and returns how many requests it made — every one,
// including the extra request a 401 triggers, so a drop log reports what was
// actually sent. The retry budget (maxAttempts) is counted separately.
func (c *collectorClient) send(ctx context.Context, payload []byte) (int, error) {
	requests := 0
	attempts := 0
	retriedUnauthorized := false
	for {
		requests++
		attempts++
		err := c.attempt(ctx, payload)
		if err == nil {
			return requests, nil
		}

		var se *statusError
		if errors.As(err, &se) && se.status == http.StatusUnauthorized && !retriedUnauthorized {
			// A token the IdP considers valid can still be refused (rotated
			// keys, clock skew). One fresh token settles it either way; this
			// retry does not count against maxAttempts.
			retriedUnauthorized = true
			attempts--
			continue
		}
		if !retryable(err) || attempts >= c.maxAttempts || ctx.Err() != nil {
			return requests, err
		}
		if !sleep(ctx, c.retryBackoff*time.Duration(attempts)) {
			return requests, err
		}
	}
}

// attempt makes one delivery attempt. Fetching a token (when the cached one
// has expired) and posting the event each get their own attemptTimeout, so a
// slow token endpoint cannot eat the collector request's budget; both stay
// within ctx, the delivery's overall deadline.
func (c *collectorClient) attempt(ctx context.Context, payload []byte) error {
	tokenCtx, cancelToken := context.WithTimeout(ctx, c.attemptTimeout)
	token, err := c.tokens.Token(tokenCtx)
	cancelToken()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, c.attemptTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.eventsURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build collector request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &transportError{kind: transportErrorKind(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		c.tokens.Invalidate(token)
	}
	return &statusError{status: resp.StatusCode}
}

// transportError is a failure to get any HTTP answer. It is always retryable.
type transportError struct{ kind string }

func (e *transportError) Error() string { return "collector request failed: " + e.kind }

func retryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.status >= 500 || se.status == http.StatusTooManyRequests
	}
	var te *tokenError
	if errors.As(err, &te) {
		return te.retryable
	}
	var tre *transportError
	if errors.As(err, &tre) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// transportErrorKind classifies a transport failure without echoing it: an
// *url.Error carries the full request URL, which has no place in a drop log.
func transportErrorKind(err error) string {
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.As(err, &netErr) && netErr.Timeout():
		return "timeout"
	default:
		return "connection error"
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
