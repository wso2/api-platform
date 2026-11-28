/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// requestIDHeader is the header the gateway carries a request's correlation ID in.
const requestIDHeader = "X-Request-Id"

// generatedRequestID matches the UUID the gateway generates for a request that brings none.
var generatedRequestID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// registerRuntimeSteps binds the steps that send one data-plane request and check what the
// gateway answers: its headers, its body and what the echo backend received.
func (s *Steps) registerRuntimeSteps(sc stepRegistrar) {
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)"$`, s.sendOnce)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with the header "([^"]*)" set to "([^"]*)"$`, s.sendOnceWithHeader)
	sc.Step(`^I send a "([^"]*)" request to "([^"]*)" with a JSON body of (\d+) bytes$`, s.sendOnceWithJSONBody)
	sc.Step(`^the response header "([^"]*)" should (be|not be|contain) "([^"]*)"$`, s.assertResponseHeader)
	sc.Step(`^I store the response header "([^"]*)" as "([^"]*)"$`, s.storeResponseHeader)
	sc.Step(`^the response should carry a request ID the gateway generated$`, s.assertGeneratedRequestID)
	sc.Step(`^the response body should be empty$`, s.assertEmptyBody)
	sc.Step(`^the cloud API response should not report "([^"]*)"$`, s.assertResponseOmits)
	sc.Step(`^the echoed request header "([^"]*)" should be "([^"]*)"$`, s.assertEchoedHeader)
}

// sendOnce sends one data-plane request and publishes the answer, whatever its status.
func (s *Steps) sendOnce(ctx context.Context, method, target string) error {
	return s.sendDataPlaneOnce(ctx, httpx.Request{Method: method, URL: target})
}

func (s *Steps) sendOnceWithHeader(ctx context.Context, method, target, name, value string) error {
	resolved, err := expandCloudValue(ctx, value)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("sending %s request: a header name is required", method)
	}
	return s.sendDataPlaneOnce(ctx, httpx.Request{Method: method, URL: target, Headers: map[string]string{name: resolved}})
}

// sendOnceWithJSONBody sends a JSON document of exactly size bytes: a string field padded to fill it.
func (s *Steps) sendOnceWithJSONBody(ctx context.Context, method, target string, size int) error {
	body, err := jsonBodyOfSize(size)
	if err != nil {
		return err
	}
	return s.sendDataPlaneOnce(ctx, httpx.Request{Method: method, URL: target, Body: body, ContentType: "application/json"})
}

// jsonBodyOfSize returns a JSON object of exactly size bytes.
func jsonBodyOfSize(size int) ([]byte, error) {
	const frame = `{"padding":""}`
	if size < len(frame) {
		return nil, fmt.Errorf("a JSON body needs at least %d bytes, asked for %d", len(frame), size)
	}
	return []byte(`{"padding":"` + strings.Repeat("x", size-len(frame)) + `"}`), nil
}

func (s *Steps) sendDataPlaneOnce(ctx context.Context, request httpx.Request) error {
	httpx.ClearPublished(ctx)
	request, err := s.prepareDataPlaneRequest(ctx, request)
	if err != nil {
		return err
	}
	_, err = s.funnel.Send(ctx, request)
	return err
}

// publishedHeader returns the published response's header value and whether it was present.
func publishedHeader(ctx context.Context, name string) (string, bool, *httpx.Response, error) {
	response, err := httpx.Published(ctx)
	if err != nil {
		return "", false, nil, err
	}
	values := response.Headers.Values(name)
	return strings.Join(values, ", "), len(values) > 0, response, nil
}

func (s *Steps) assertResponseHeader(ctx context.Context, name, relation, expected string) error {
	want, err := expandCloudValue(ctx, expected)
	if err != nil {
		return err
	}
	got, present, response, err := publishedHeader(ctx, name)
	if err != nil {
		return err
	}
	switch relation {
	case "be":
		if !present || got != want {
			return fmt.Errorf("header %q is %q, want %q (%s)", name, got, want, response.Describe())
		}
	case "not be":
		if present && got == want {
			return fmt.Errorf("header %q is %q, want anything else (%s)", name, got, response.Describe())
		}
	case "contain":
		if !present || !strings.Contains(got, want) {
			return fmt.Errorf("header %q is %q, want it to contain %q (%s)", name, got, want, response.Describe())
		}
	default:
		return fmt.Errorf("unsupported header relation %q", relation)
	}
	return nil
}

func (s *Steps) storeResponseHeader(ctx context.Context, name, key string) error {
	got, present, response, err := publishedHeader(ctx, name)
	if err != nil {
		return err
	}
	if !present || got == "" {
		return fmt.Errorf("the response has no %q header to store (%s)", name, response.Describe())
	}
	return tcontext.Set(ctx, key, got)
}

func (s *Steps) assertGeneratedRequestID(ctx context.Context) error {
	got, _, response, err := publishedHeader(ctx, requestIDHeader)
	if err != nil {
		return err
	}
	if !generatedRequestID.MatchString(got) {
		return fmt.Errorf("the response's request ID %q is not one the gateway generated (%s)", got, response.Describe())
	}
	return nil
}

func (s *Steps) assertEmptyBody(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if len(response.Body) != 0 {
		return fmt.Errorf("the response body has %d bytes, want none (%s)", len(response.Body), response.Describe())
	}
	return nil
}

// assertResponseOmits checks that a dotted path is absent from the published JSON response.
func (s *Steps) assertResponseOmits(ctx context.Context, path string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return fmt.Errorf("the response is not JSON: %w (%s)", err, response.Describe())
	}
	if got, present := documentPath(document, path); present {
		return fmt.Errorf("the response reports %s as %v, want it absent (%s)", path, got, response.Describe())
	}
	return nil
}

func (s *Steps) assertEchoedHeader(ctx context.Context, name, expected string) error {
	want, err := expandCloudValue(ctx, expected)
	if err != nil {
		return err
	}
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	got, present := echoedHeader(response.Body, name)
	if !present {
		return fmt.Errorf("the echoed request has no %q header: %s", name, response.Describe())
	}
	if got != want {
		return fmt.Errorf("the echoed request's %q header is %q, want %q", name, got, want)
	}
	return nil
}
