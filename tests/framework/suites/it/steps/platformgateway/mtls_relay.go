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

package platformgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/testpki"
)

func mtlsFixtureNamed(name string) (*testpki.Fixture, error) {
	fixtures, err := testpki.Default()
	if err != nil {
		return nil, err
	}
	return fixtures.Get(name)
}

func (g *Gateway) registerMTLSRelaySteps(sc *godog.ScenarioContext) {
	sc.Step(`^the backend's X-Forwarded-Client-Cert should (name|not name) certificate "([^"]*)"$`, g.forwardedCertificateNames)
}

// xfccHash reads each Hash element of an X-Forwarded-Client-Cert value.
var xfccHash = regexp.MustCompile(`(?i)(?:^|[;,])Hash=([0-9a-f]{64})(?:[;,]|$)`)

// forwardedCertificateNames asserts what the backend's X-Forwarded-Client-Cert describes. It
// names a certificate when its first element's Hash is the fixture's thumbprint and its Subject
// carries the fixture's common name; it does not name one when no element's Hash is that
// thumbprint.
func (g *Gateway) forwardedCertificateNames(ctx context.Context, mode, fixtureName string) error {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	fixture, err := mtlsFixtureNamed(fixtureName)
	if err != nil {
		return err
	}
	xfcc, err := echoedHeaderValues(resp, "x-forwarded-client-cert")
	if err != nil {
		return err
	}
	hashes := xfccHash.FindAllStringSubmatch(xfcc, -1)
	if mode == "not name" {
		for _, h := range hashes {
			if strings.EqualFold(h[1], fixture.Thumbprint) {
				return fmt.Errorf("expected X-Forwarded-Client-Cert not to name %q, got %q", fixtureName, xfcc)
			}
		}
		return nil
	}
	if xfcc == "" {
		return fmt.Errorf("expected the backend to receive X-Forwarded-Client-Cert naming %q, got none: %s", fixtureName, resp.Describe())
	}
	if len(hashes) == 0 || !strings.EqualFold(hashes[0][1], fixture.Thumbprint) {
		return fmt.Errorf("expected X-Forwarded-Client-Cert Hash to be the thumbprint of %q (%s), got %q", fixtureName, fixture.Thumbprint, xfcc)
	}
	if cn := "CN=" + fixture.Certificate.Subject.CommonName; !strings.Contains(xfcc, cn) {
		return fmt.Errorf("expected X-Forwarded-Client-Cert Subject to name %s, got %q", cn, xfcc)
	}
	return nil
}

// echoedHeaderValues returns every value the echo backend received for a header, joined by
// commas, or "" when it received none.
func echoedHeaderValues(resp *httpx.Response, name string) (string, error) {
	var echo struct {
		Headers map[string]any `json:"headers"`
	}
	if err := json.Unmarshal(resp.Body, &echo); err != nil || echo.Headers == nil {
		return "", fmt.Errorf("the response carries no echoed headers: %s", resp.Describe())
	}
	for key, value := range echo.Headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		switch v := value.(type) {
		case string:
			return v, nil
		case []any:
			parts := make([]string, 0, len(v))
			for _, part := range v {
				parts = append(parts, fmt.Sprint(part))
			}
			return strings.Join(parts, ","), nil
		default:
			return "", fmt.Errorf("echoed header %q is %T, not a string or an array", name, value)
		}
	}
	return "", nil
}
