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

package steps

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

const incompleteRequestTimeout = 20 * time.Second

func (b *Base) registerRawHTTPSteps(sc *godog.ScenarioContext) {
	sc.Step(`^I send an incomplete HTTP request to "([^"]*)"$`, b.sendIncompleteHTTP)
}

// sendIncompleteHTTP leaves the header block unterminated and records the gateway response.
func (b *Base) sendIncompleteHTTP(ctx context.Context, path string) error {
	resolved, err := stepscommon.Expand(ctx, path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resolved, "/") {
		resolved = "/" + resolved
	}
	base, err := b.topo.URL("platform-gateway", "http")
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

	host := b.requestHost(ctx)
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
	if err := b.funnel.Publish(ctx, response); err != nil {
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
