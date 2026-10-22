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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	corecatalog "github.com/wso2/api-platform/tests/framework/core/catalog/cloudconsole"
	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

const (
	keySecretID            = "cloudSecretID"
	keySecretValue         = "cloudSecretValue"
	keySecretPreviousValue = "cloudSecretPreviousValue"
	keySecretReference     = "cloudSecretReference"

	secretsCollection = "/secrets"
)

// kindSecret orders secret cleanup after the APIs that reference a secret, which the platform
// refuses to delete a secret while they exist.
var kindSecret = cleanup.Kind{Name: "secret", Order: 70}

// Echo states a data-plane response can be in with respect to the secret a header should carry.
const (
	echoCurrent  = "current"
	echoPrevious = "previous"
	echoAbsent   = "absent"
	echoOther    = "other"
)

// secretReference is the placeholder a REST API definition uses to name a secret. The quotes are
// escaped because scenarios place it inside a JSON string of a values table.
func secretReference(handle string) string {
	return `{{ secret \"` + handle + `\" }}`
}

// sampleSecretValue returns a unique, obviously fake secret value.
func sampleSecretValue(ctx context.Context, base string) (string, error) {
	value, err := unique.Unique(ctx, base)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.ReplaceAll(value, "_", "-")), nil
}

// secretForm encodes the multipart body the secrets API accepts.
func secretForm(fields [][2]string) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return nil, "", fmt.Errorf("encoding secret form field %q: %w", field[0], err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("encoding the secret form: %w", err)
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

// doSecretForm sends a multipart secrets request and publishes the response.
func (s *Steps) doSecretForm(ctx context.Context, method, path string, fields [][2]string) (*httpx.Response, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return nil, err
	}
	base, err := s.topo.URL(corecatalog.Name, corecatalog.EndpointAPIPPlatformAPI)
	if err != nil {
		return nil, err
	}
	body, contentType, err := secretForm(fields)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: method, URL: cloudAPIURL(base, path), ContentType: contentType, Body: body,
		Headers: map[string]string{"Authorization": "Bearer " + token},
	}, 0, 0)
	if err != nil {
		return nil, err
	}
	if s.funnel != nil {
		if err := s.funnel.Publish(ctx, resp); err != nil {
			return nil, err
		}
	}
	return resp, nil
}

// chooseSecretHandle picks a secret handle and its placeholder without creating the secret.
func (s *Steps) chooseSecretHandle(ctx context.Context) error {
	if err := generateResourceAndStore(ctx, "syn-sec", keySecretID); err != nil {
		return err
	}
	handle, err := requiredValue(ctx, keySecretID)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keySecretReference, secretReference(handle))
}

// createSecret creates a secret holding a generated sample value and keeps the value in scenario
// state only.
func (s *Steps) createSecret(ctx context.Context) error {
	if err := s.chooseSecretHandle(ctx); err != nil {
		return err
	}
	handle, err := requiredValue(ctx, keySecretID)
	if err != nil {
		return err
	}
	value, err := sampleSecretValue(ctx, "sample-secret")
	if err != nil {
		return err
	}
	if err := cleanup.Register(ctx, cleanup.Resource{
		Kind: kindSecret, ID: handle, Actor: resourceActor, Description: "secret " + handle,
	}); err != nil {
		return fmt.Errorf("registering secret %q for cleanup: %w", handle, err)
	}
	resp, err := s.doSecretForm(ctx, http.MethodPost, secretsCollection, [][2]string{
		{"id", handle}, {"displayName", "Synthetic " + handle},
		{"description", "Created by the APIP cloud integration test with a sample value. Safe to delete."},
		{"value", value}, {"type", "GENERIC"},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		deregister(ctx, kindSecret, handle)
		return responseError(resp)
	}
	createdID, err := stringField(resp, "id")
	if err != nil {
		return fmt.Errorf("creating APIP secret: %w", err)
	}
	if createdID != handle {
		deregister(ctx, kindSecret, handle)
		if err := cleanup.Register(ctx, cleanup.Resource{
			Kind: kindSecret, ID: createdID, Actor: resourceActor, Description: "secret " + createdID,
		}); err != nil {
			return fmt.Errorf("registering secret %q for cleanup: %w", createdID, err)
		}
		if err := tcontext.Set(ctx, keySecretID, createdID); err != nil {
			return err
		}
		if err := tcontext.Set(ctx, keySecretReference, secretReference(createdID)); err != nil {
			return err
		}
	}
	return tcontext.Set(ctx, keySecretValue, value)
}

// rotateSecret replaces the synthetic secret's value with a new sample value.
func (s *Steps) rotateSecret(ctx context.Context) error {
	handle, err := requiredValue(ctx, keySecretID)
	if err != nil {
		return err
	}
	previous, err := requiredValue(ctx, keySecretValue)
	if err != nil {
		return err
	}
	value, err := sampleSecretValue(ctx, "sample-rotated")
	if err != nil {
		return err
	}
	resp, err := s.doSecretForm(ctx, http.MethodPut, secretsCollection+"/"+url.PathEscape(handle), [][2]string{
		{"displayName", "Synthetic " + handle}, {"value", value},
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	if err := tcontext.Set(ctx, keySecretPreviousValue, previous); err != nil {
		return err
	}
	return tcontext.Set(ctx, keySecretValue, value)
}

// readSecret reads the synthetic secret's metadata.
func (s *Steps) readSecret(ctx context.Context) error {
	handle, err := requiredValue(ctx, keySecretID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodGet, secretsCollection+"/"+url.PathEscape(handle), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return nil
}

// listSecrets reads the first page of the organisation's secrets.
func (s *Steps) listSecrets(ctx context.Context) error {
	resp, err := s.doAuthenticated(ctx, http.MethodGet, secretsCollection, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return nil
}

// deleteSecret deletes the synthetic secret and publishes the answer, so a scenario can assert on
// a refusal. A deleted secret is deregistered so cleanup does not repeat the delete.
func (s *Steps) deleteSecret(ctx context.Context) error {
	handle, err := requiredValue(ctx, keySecretID)
	if err != nil {
		return err
	}
	resp, err := s.doAuthenticated(ctx, http.MethodDelete, secretsCollection+"/"+url.PathEscape(handle), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNoContent {
		deregister(ctx, kindSecret, handle)
	}
	return nil
}

// carriesValueField reports whether a decoded JSON document has a field named "value" anywhere.
func carriesValueField(document any) bool {
	switch typed := document.(type) {
	case map[string]any:
		for key, nested := range typed {
			if key == "value" || carriesValueField(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if carriesValueField(nested) {
				return true
			}
		}
	}
	return false
}

// assertSecretValueHidden checks the published response neither contains the secret's value nor
// has a value field. The failure names the request, never the response, so a leak does not repeat
// the value into a report.
func (s *Steps) assertSecretValueHidden(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	value, err := requiredValue(ctx, keySecretValue)
	if err != nil {
		return err
	}
	if bytes.Contains(response.Body, []byte(value)) {
		return fmt.Errorf("the response to %s %s contains the secret's value", response.Method, response.URL)
	}
	var document any
	if json.Unmarshal(response.Body, &document) == nil && carriesValueField(document) {
		return fmt.Errorf("the response to %s %s has a \"value\" field", response.Method, response.URL)
	}
	return nil
}

// assertSecretHash checks the published secret metadata identifies the secret by a hash that is
// not the value.
func (s *Steps) assertSecretHash(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	value, err := requiredValue(ctx, keySecretValue)
	if err != nil {
		return err
	}
	hash, err := stringField(response, "hash")
	if err != nil {
		return fmt.Errorf("the secret metadata has no hash: %w", err)
	}
	if hash == value || strings.Contains(hash, value) {
		return fmt.Errorf("the secret's hash reveals its value")
	}
	return nil
}

// echoedHeader reads a request header from the JSON an echo backend returns, ignoring case.
func echoedHeader(body []byte, name string) (string, bool) {
	var echoed struct {
		Headers map[string]any `json:"headers"`
	}
	if json.Unmarshal(body, &echoed) != nil {
		return "", false
	}
	for key, raw := range echoed.Headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		switch typed := raw.(type) {
		case string:
			return typed, true
		case []any:
			parts := make([]string, 0, len(typed))
			for _, part := range typed {
				if text, ok := part.(string); ok {
					parts = append(parts, text)
				}
			}
			return strings.Join(parts, ","), true
		}
	}
	return "", false
}

// classifyEcho says which secret value an echoed header carries: the current one, the previous
// one, none, or something else.
func classifyEcho(response *httpx.Response, header, current, previous string) string {
	if response == nil || response.StatusCode != http.StatusOK {
		return echoOther
	}
	got, ok := echoedHeader(response.Body, header)
	switch {
	case !ok:
		return echoAbsent
	case got == current:
		return echoCurrent
	case previous != "" && got == previous:
		return echoPrevious
	default:
		return echoOther
	}
}

// assertEchoedSecret checks the published echo carries the synthetic secret's value in a header.
func (s *Steps) assertEchoedSecret(ctx context.Context, header string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	value, err := requiredValue(ctx, keySecretValue)
	if err != nil {
		return err
	}
	switch classifyEcho(response, header, value, "") {
	case echoCurrent:
		return nil
	case echoAbsent:
		return fmt.Errorf("the echoed request has no %q header: %s", header, response.Describe())
	default:
		return fmt.Errorf("the request reached the backend with a %q header that is not the secret's value", header)
	}
}

// sendUntilEchoesRotated polls a data-plane URL, with no redeployment, until the echoed header
// carries the rotated secret value.
func (s *Steps) sendUntilEchoesRotated(ctx context.Context, method, target, header string) error {
	current, err := requiredValue(ctx, keySecretValue)
	if err != nil {
		return err
	}
	previous, err := requiredValue(ctx, keySecretPreviousValue)
	if err != nil {
		return fmt.Errorf("the secret has not been rotated: %w", err)
	}
	request, err := s.prepareDataPlaneRequest(ctx, httpx.Request{Method: strings.ToUpper(strings.TrimSpace(method)), URL: target})
	if err != nil {
		return err
	}
	last, err := retry.Until(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			response, sendErr := s.funnel.Send(ctx, request)
			if sendErr != nil {
				return "", retry.Transient(sendErr)
			}
			return classifyEcho(response, header, current, previous), nil
		},
		func(state string) bool { return state == echoCurrent })
	if err != nil {
		return fmt.Errorf("waiting for the gateway to send the rotated secret in %q: %w", header, err)
	}
	switch last {
	case echoCurrent:
		return nil
	case echoPrevious:
		return fmt.Errorf("the gateway still sends the previous secret value in %q after the rotation, with no redeployment", header)
	default:
		return fmt.Errorf("the gateway never echoed the rotated secret in %q (last state: %s)", header, last)
	}
}
