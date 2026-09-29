/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package platformgateway

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

const incompleteRequestTimeout = 20 * time.Second

// maxRawResponseBodyBytes bounds a raw response body read, mirroring the funnel client's own
// cap so a raw-connection scenario cannot buffer an unbounded body into memory.
const maxRawResponseBodyBytes = 10 << 20

func (g *Gateway) registerRawHTTPSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send an incomplete HTTP request to "([^"]*)"$`, g.sendIncompleteHTTP)
	sc.Step(`^I send a raw "([^"]*)" request to "([^"]*)" with headers:$`, g.sendRawRequestWithHeaders)
}

// sendRawRequestWithHeaders writes an HTTP/1.1 request directly onto the wire, one header line
// per table row in the exact order and casing given. It exists for requests the standard HTTP
// client cannot express - a header name whose casing must survive untouched, a value the client
// would otherwise normalize, or duplicate header lines - and publishes the parsed response so
// every generic response assertion works unchanged.
func (g *Gateway) sendRawRequestWithHeaders(
	ctx context.Context, method, path string, table *godog.Table,
) error {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	method = strings.ToUpper(method)

	headers, err := rawHeaderLines(ctx, table)
	if err != nil {
		return err
	}

	base, err := g.topo.URL("platform-gateway", "http")
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse gateway endpoint %q: %w", base, err)
	}
	if endpoint.Host == "" {
		return fmt.Errorf("gateway endpoint %q has no host", base)
	}

	host := g.requestHost(ctx)
	if host == "" {
		host = endpoint.Hostname()
	}

	started := time.Now()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		return fmt.Errorf("connect to gateway endpoint %q: %w", endpoint.Host, err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(incompleteRequestTimeout))

	var request strings.Builder
	fmt.Fprintf(&request, "%s %s HTTP/1.1\r\n", method, resolved)
	fmt.Fprintf(&request, "Host: %s\r\n", host)
	for _, header := range headers {
		fmt.Fprintf(&request, "%s: %s\r\n", header.name, header.value)
	}
	request.WriteString("Connection: close\r\n\r\n")

	if _, err := conn.Write([]byte(request.String())); err != nil {
		return fmt.Errorf("send raw request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: method})
	if err != nil {
		return fmt.Errorf("read raw response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRawResponseBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read raw response body: %w", err)
	}
	if len(body) > maxRawResponseBodyBytes {
		return fmt.Errorf("raw response body of %s %s exceeds the %d-byte limit", method, resolved, maxRawResponseBodyBytes)
	}

	response := &httpx.Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
		Method:     method,
		URL:        base + resolved,
		Elapsed:    time.Since(started),
	}
	return g.funnel.Publish(ctx, response)
}

// rawHeader is one header line to write verbatim, in table order.
type rawHeader struct {
	name  string
	value string
}

// rawHeaderLines expands and decodes each table row into a header line, preserving row order and
// duplicate names so a scenario can exercise ambiguous or repeated headers on the wire.
func rawHeaderLines(ctx context.Context, table *godog.Table) ([]rawHeader, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("raw request headers table must not be empty")
	}
	headers := make([]rawHeader, 0, len(table.Rows))
	for i, row := range table.Rows {
		if row == nil || len(row.Cells) != 2 {
			return nil, fmt.Errorf("raw request headers row %d must contain exactly two cells", i+1)
		}
		name := strings.TrimSpace(row.Cells[0].Value)
		if name == "" {
			return nil, fmt.Errorf("raw request headers row %d has an empty header name", i+1)
		}
		expanded, err := stepscommon.Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		headers = append(headers, rawHeader{name: name, value: decodeRawHeaderValue(expanded)})
	}
	return headers, nil
}

// decodeRawHeaderValue resolves the small escape vocabulary a raw header value needs to express
// bytes a Gherkin table cell cannot hold literally: \t, \r, \n, a literal backslash, and \xHH for
// an arbitrary byte. Any other escape, or a truncated \x sequence, is left as written.
func decodeRawHeaderValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		switch value[i+1] {
		case 't':
			b.WriteByte('\t')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case 'n':
			b.WriteByte('\n')
			i++
		case '\\':
			b.WriteByte('\\')
			i++
		case 'x':
			if i+3 < len(value) {
				if n, err := strconv.ParseUint(value[i+2:i+4], 16, 8); err == nil {
					b.WriteByte(byte(n))
					i += 3
					continue
				}
			}
			b.WriteByte(value[i])
		default:
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

// sendIncompleteHTTP leaves the header block unterminated and records the gateway response.
func (g *Gateway) sendIncompleteHTTP(ctx context.Context, path string) error {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	base, err := g.topo.URL("platform-gateway", "http")
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("parse gateway endpoint %q: %w", base, err)
	}
	if endpoint.Host == "" {
		return fmt.Errorf("gateway endpoint %q has no host", base)
	}

	started := time.Now()
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", endpoint.Host)
	if err != nil {
		return fmt.Errorf("connect to gateway endpoint %q: %w", endpoint.Host, err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(incompleteRequestTimeout))

	host := g.base.RequestHost(ctx)
	if host == "" {
		host = endpoint.Hostname()
	}
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\n", resolved, host); err != nil {
		return fmt.Errorf("send incomplete request: %w", err)
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read incomplete-request response: %w", err)
	}
	status, err := parseHTTPStatusLine(line)
	if err != nil {
		return err
	}
	response := &httpx.Response{
		StatusCode: status,
		Method:     "GET",
		URL:        base + resolved,
		Elapsed:    time.Since(started),
	}
	if err := g.funnel.Publish(ctx, response); err != nil {
		return err
	}
	return nil
}

func parseHTTPStatusLine(line string) (int, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 || !strings.HasPrefix(fields[0], "HTTP/") {
		return 0, fmt.Errorf("invalid HTTP status line %q", strings.TrimSpace(line))
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil || status < 100 || status > 999 {
		return 0, fmt.Errorf("invalid HTTP status code in %q", strings.TrimSpace(line))
	}
	return status, nil
}
