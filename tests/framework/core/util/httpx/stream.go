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

package httpx

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// StreamVisitor receives one line of a streamed response body, without its line terminator,
// and the time that elapsed between the response headers arriving and the line becoming
// readable. Returning false stops the read and closes the connection.
type StreamVisitor func(line string, sinceHeaders time.Duration) bool

// Stream issues one request and reads its body incrementally, handing every line to visit as
// it arrives.
//
// A fully buffered read collapses every arrival time onto the moment the body closed, which
// hides whether a server streamed at all. The returned Response carries the lines read as its
// body, so ordinary status, header and body assertions still apply to it. Transfer-Encoding
// and Content-Length are restored into the header map from the typed fields net/http moves
// them into, so a header assertion sees what the server actually sent.
//
// The client's whole-request timeout is not applied: a long-lived stream is bounded by ctx
// instead. Transient-response retries are not attempted because a stream is not replayable.
func (c *Client) Stream(ctx context.Context, req Request, visit StreamVisitor) (*Response, error) {
	if visit == nil {
		return nil, fmt.Errorf("httpx: a stream visitor is required")
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("httpx: request has no URL")
	}
	httpReq, err := c.build(ctx, req)
	if err != nil {
		return nil, err
	}
	streaming := &http.Client{Transport: c.http.Transport, CheckRedirect: c.http.CheckRedirect}

	started := time.Now()
	resp, err := streaming.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("httpx: %s %s: %w", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	headersAt := time.Now()

	var body bytes.Buffer
	reader := bufio.NewReader(resp.Body)
	stopped := false
	for !stopped {
		line, readErr := reader.ReadString('\n')
		if line != "" {
			if int64(body.Len()+len(line)) > maxResponseBodyBytes {
				return nil, fmt.Errorf("httpx: streamed body of %s %s exceeds the %d-byte limit",
					req.Method, req.URL, maxResponseBodyBytes)
			}
			body.WriteString(line)
			trimmed := strings.TrimRight(line, "\r\n")
			if !visit(trimmed, time.Since(headersAt)) {
				stopped = true
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || stopped {
				break
			}
			return nil, fmt.Errorf("httpx: reading the stream of %s %s: %w", req.Method, req.URL, readErr)
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body.Bytes(),
		Headers:    streamHeaders(resp),
		Method:     req.Method,
		URL:        req.URL,
		Elapsed:    time.Since(started),
	}, nil
}

// streamHeaders returns the response headers with the framing fields net/http strips from the
// header map restored from their typed equivalents.
func streamHeaders(resp *http.Response) http.Header {
	headers := resp.Header.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	if len(resp.TransferEncoding) > 0 && headers.Get("Transfer-Encoding") == "" {
		headers.Set("Transfer-Encoding", strings.Join(resp.TransferEncoding, ", "))
	}
	if resp.ContentLength >= 0 && headers.Get("Content-Length") == "" {
		headers.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	return headers
}

// Stream clears the published response, streams the request and publishes the result.
func (f *Funnel) Stream(ctx context.Context, req Request, visit StreamVisitor) (*Response, error) {
	ClearPublished(ctx)
	resp, err := f.client.Stream(ctx, req, visit)
	if err != nil {
		return nil, err
	}
	if err := f.Publish(ctx, resp); err != nil {
		return resp, err
	}
	return resp, nil
}
