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

package platformapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

const (
	serviceAccountsPath     = "/service-accounts"
	serviceAccountTokenPath = serviceAccountsPath + "/token"
	formContentType         = "application/x-www-form-urlencoded"
	clientCredentialsGrant  = "client_credentials"
	jwtHeaderSegment        = 0
	jwtPayloadSegment       = 1
)

// RegisterServiceAccount binds the service-account steps: creating and updating an account from a
// template, the client-credentials exchange, requests made without signing in, and claim checks on
// an issued token. Every request goes through the control-plane funnel, so the shared assertions
// read its response.
func RegisterServiceAccount(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^I create a service account via the control plane from "([^"]*)"(?: as "([^"]*)")? with values:$`,
		s.createServiceAccountFromTemplate)
	sc.Step(`^I update the service account "([^"]*)" via the control plane from "([^"]*)"(?: as "([^"]*)")? with values:$`,
		s.updateServiceAccountFromTemplate)
	sc.Step(`^I exchange service account credentials( using HTTP Basic as "([^"]*)" with secret "([^"]*)")? with values:$`,
		s.exchangeServiceAccountCredentials)
	sc.Step(`^I send a "([^"]*)" request to the control plane at "([^"]*)" without signing in$`,
		s.sendControlPlaneUnauthenticated)
	sc.Step(`^I send a "([^"]*)" request to the control plane root at "([^"]*)"(?: with header "([^"]*)" set to "([^"]*)")?$`,
		s.sendControlPlaneRoot)
	sc.Step(`^the JWT in the JSON response field "([^"]*)" should have (claim|header) "([^"]*)" (equal to|matching) "([^"]*)"$`,
		s.jwtEntryIs)
}

func (s *Steps) createServiceAccountFromTemplate(ctx context.Context, templateName, who string, table *godog.Table) error {
	payload, err := s.renderJSONTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{method: http.MethodPost, path: serviceAccountsPath, who: who, body: payload})
	return err
}

func (s *Steps) updateServiceAccountFromTemplate(ctx context.Context, id, templateName, who string, table *godog.Table) error {
	resolved, err := stepscommon.Expand(ctx, id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolved) == "" {
		return fmt.Errorf("updating a service account needs its id")
	}
	payload, err := s.renderJSONTemplate(ctx, templateName, table)
	if err != nil {
		return err
	}
	_, err = s.sendControlPlane(ctx, controlPlaneCall{
		method: http.MethodPut, path: serviceAccountsPath + "/" + url.PathEscape(resolved), who: who, body: payload,
	})
	return err
}

// anonymousCall is one control-plane request sent without signing in.
type anonymousCall struct {
	method      string
	path        string
	header      string
	value       string
	body        []byte
	contentType string
	// root resolves path against the server root rather than the versioned management API.
	root bool
}

// sendAnonymous is sendControlPlane without the sign-in: it sends through the funnel, publishes
// the response and records any resource the response reports creating or deleting.
func (s *Steps) sendAnonymous(ctx context.Context, call anonymousCall) (*httpx.Response, error) {
	method := strings.ToUpper(strings.TrimSpace(call.method))
	if method == "" {
		return nil, fmt.Errorf("a control-plane request needs a method")
	}
	path, err := stepscommon.Expand(ctx, call.path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("control-plane path %q must start with /", path)
	}
	base, err := s.baseURL()
	if err != nil {
		return nil, err
	}
	headers := map[string]string{}
	if call.body != nil {
		headers["Content-Type"] = call.contentType
	}
	if name := strings.TrimSpace(call.header); name != "" {
		value, expandErr := stepscommon.Expand(ctx, call.value)
		if expandErr != nil {
			return nil, expandErr
		}
		headers[name] = value
	}
	prefix := apiBase
	if call.root {
		prefix = ""
	}
	resp, err := s.funnel.Send(ctx, httpx.Request{
		Method: method, URL: base + prefix + path, Headers: headers, Body: call.body,
	})
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if err := s.trackControlPlaneResource(ctx, method, call.body, resp); err != nil {
		return resp, err
	}
	return resp, nil
}

// exchangeServiceAccountCredentials posts a client-credentials request to the public token
// endpoint. The table rows are the form, sent exactly as given, empty values included, and
// grant_type defaults to client_credentials. With HTTP Basic the table still supplies the form,
// so a scenario can send both client authentication methods at once.
func (s *Steps) exchangeServiceAccountCredentials(
	ctx context.Context, basic, clientID, clientSecret string, table *godog.Table,
) error {
	form, err := tokenForm(ctx, table)
	if err != nil {
		return err
	}
	call := anonymousCall{
		method: http.MethodPost, path: serviceAccountTokenPath,
		body: []byte(form.Encode()), contentType: formContentType,
	}
	if basic != "" {
		id, err := stepscommon.Expand(ctx, clientID)
		if err != nil {
			return err
		}
		secret, err := stepscommon.Expand(ctx, clientSecret)
		if err != nil {
			return err
		}
		call.header, call.value = "Authorization", basicClientCredentials(id, secret)
	}
	_, err = s.sendAnonymous(ctx, call)
	return err
}

// tokenForm turns a two-column values table into a token request form.
func tokenForm(ctx context.Context, table *godog.Table) (url.Values, error) {
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("the token request needs a values table")
	}
	form := url.Values{}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("a token request row needs a key and a value, found %d cells", len(row.Cells))
		}
		key := strings.TrimSpace(row.Cells[0].Value)
		if key == "" {
			return nil, fmt.Errorf("a token request row needs a key")
		}
		if form.Has(key) {
			return nil, fmt.Errorf("the token request names %q twice", key)
		}
		value, err := stepscommon.Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		form.Set(key, value)
	}
	if !form.Has("grant_type") {
		form.Set("grant_type", clientCredentialsGrant)
	}
	return form, nil
}

// basicClientCredentials builds the Authorization header of RFC 6749 section 2.3.1, which
// form-encodes the id and the secret before joining them.
func basicClientCredentials(id, secret string) string {
	pair := url.QueryEscape(id) + ":" + url.QueryEscape(secret)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(pair))
}

func (s *Steps) sendControlPlaneUnauthenticated(ctx context.Context, method, path string) error {
	_, err := s.sendAnonymous(ctx, anonymousCall{method: method, path: path})
	return err
}

func (s *Steps) sendControlPlaneRoot(ctx context.Context, method, path, header, value string) error {
	_, err := s.sendAnonymous(ctx, anonymousCall{method: method, path: path, header: header, value: value, root: true})
	return err
}

// jwtEntryIs checks one claim or header of the JWT held in a field of the published response.
// It decodes without checking the signature: the control plane is the verifier under test.
func (s *Steps) jwtEntryIs(ctx context.Context, field, part, name, comparison, want string) error {
	token, err := publishedJWT(ctx, field)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("the JWT check needs a %s name", part)
	}
	segment := jwtPayloadSegment
	if part == "header" {
		segment = jwtHeaderSegment
	}
	entries, err := jwtSegment(token, segment)
	if err != nil {
		return err
	}
	value, ok := traverse(entries, name)
	if !ok {
		return fmt.Errorf("the JWT in JSON field %q has no %s %q", field, part, name)
	}
	expected, err := stepscommon.Expand(ctx, want)
	if err != nil {
		return err
	}
	return compareJWTEntry(part, name, renderValue(value), comparison, expected)
}

// publishedJWT reads the token held in a string field of the published response. Its errors name
// the status and the top-level keys only, because the body of a 200 may hold the token.
func publishedJWT(ctx context.Context, field string) (string, error) {
	resp, err := httpx.Published(ctx)
	if err != nil {
		return "", err
	}
	var doc any
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return "", fmt.Errorf("the published response (status %d) is not JSON, so it holds no token in field %q",
			resp.StatusCode, field)
	}
	value, _ := traverse(doc, field)
	if token, ok := value.(string); ok && strings.TrimSpace(token) != "" {
		return token, nil
	}
	return "", fmt.Errorf("the published response (status %d, top-level keys %v) holds no token in JSON field %q",
		resp.StatusCode, topLevelKeys(doc), field)
}

// topLevelKeys lists the keys of a JSON object in order, or nothing for any other value.
func topLevelKeys(doc any) []string {
	object, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// jwtSegment decodes the header (0) or payload (1) of a compact JWT into a JSON object.
func jwtSegment(token string, index int) (map[string]any, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("a JWT has 3 dot-separated segments, this value has %d", len(parts))
	}
	if index != jwtHeaderSegment && index != jwtPayloadSegment {
		return nil, fmt.Errorf("JWT segment %d is not a header or a payload", index)
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[index])
	if err != nil {
		return nil, fmt.Errorf("JWT segment %d is not base64url", index)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var entries map[string]any
	if err := decoder.Decode(&entries); err != nil || entries == nil || decoder.More() {
		return nil, fmt.Errorf("JWT segment %d is not a JSON object", index)
	}
	return entries, nil
}

// compareJWTEntry compares a rendered claim or header with the expectation. The messages never
// carry the token itself.
func compareJWTEntry(part, name, got, comparison, want string) error {
	switch comparison {
	case "equal to":
		if got != want {
			return fmt.Errorf("JWT %s %q: expected %q, found %q", part, name, want, got)
		}
	case "matching":
		pattern, err := regexp.Compile(want)
		if err != nil {
			return fmt.Errorf("JWT %s %q: invalid pattern %q: %w", part, name, want, err)
		}
		if !pattern.MatchString(got) {
			return fmt.Errorf("JWT %s %q: expected a match for %q, found %q", part, name, want, got)
		}
	default:
		return fmt.Errorf("unknown JWT comparison %q", comparison)
	}
	return nil
}
