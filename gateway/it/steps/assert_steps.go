/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package steps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// ResponseProvider provides access to the last HTTP response
type ResponseProvider interface {
	LastResponse() *http.Response
	LastBody() []byte
}

// AssertSteps provides common response assertion step definitions
type AssertSteps struct {
	provider ResponseProvider
}

// NewAssertSteps creates a new AssertSteps instance
func NewAssertSteps(provider ResponseProvider) *AssertSteps {
	return &AssertSteps{
		provider: provider,
	}
}

// Register registers all assertion step definitions
func (a *AssertSteps) Register(ctx *godog.ScenarioContext) {
	// Status code assertions
	ctx.Step(`^the response status code should be (\d+)$`, a.statusCodeShouldBe)
	ctx.Step(`^the response status should be (\d+)$`, a.statusCodeShouldBe) // Alias
	ctx.Step(`^the response status should be "([^"]*)"$`, a.statusShouldBe)
	ctx.Step(`^the response should be successful$`, a.responseShouldBeSuccessful)
	ctx.Step(`^the response should be a client error$`, a.responseShouldBeClientError)
	ctx.Step(`^the response should be a server error$`, a.responseShouldBeServerError)

	// Header assertions
	ctx.Step(`^the response header "([^"]*)" should be "([^"]*)"$`, a.headerShouldBe)
	ctx.Step(`^the response header "([^"]*)" should contain "([^"]*)"$`, a.headerShouldContain)
	ctx.Step(`^the response header "([^"]*)" should exist$`, a.headerShouldExist)
	ctx.Step(`^the response header "([^"]*)" should not exist$`, a.headerShouldNotExist)
	ctx.Step(`^the response Content-Type should be "([^"]*)"$`, a.contentTypeShouldBe)

	// Body assertions
	ctx.Step(`^the response body should contain "([^"]*)"$`, a.bodyShouldContain)
	ctx.Step(`^the response body should not contain "([^"]*)"$`, a.bodyShouldNotContain)
	ctx.Step(`^the response body should be empty$`, a.bodyShouldBeEmpty)
	ctx.Step(`^the response body should not be empty$`, a.bodyShouldNotBeEmpty)
	ctx.Step(`^the response body should match pattern "([^"]*)"$`, a.bodyShouldMatchPattern)
	ctx.Step(`^the response body should be:$`, a.bodyShouldBe)

	// JSON assertions
	ctx.Step(`^the response should be valid JSON$`, a.shouldBeValidJSON)
	ctx.Step(`^the JSON response should have field "([^"]*)"$`, a.jsonShouldHaveField)
	ctx.Step(`^the JSON response field "([^"]*)" should be "([^"]*)"$`, a.jsonFieldShouldBe)
	ctx.Step(`^the JSON response field "([^"]*)" should contain "([^"]*)"$`, a.jsonFieldShouldContain)
	ctx.Step(`^the JSON response field "([^"]*)" should be (\d+)$`, a.jsonFieldShouldBeInt)
	ctx.Step(`^the JSON response field "([^"]*)" should be (true|false)$`, a.jsonFieldShouldBeBool)
	ctx.Step(`^the JSON response should have (\d+) items$`, a.jsonShouldHaveItems)

	// Echoed header assertions (for sample-backend /echo endpoint)
	ctx.Step(`^the response should contain echoed header "([^"]*)" with value "([^"]*)"$`, a.echoedHeaderShouldBe)

	// Debug helper
	ctx.Step(`^I print the response body$`, a.printResponseBody)
	ctx.Step(`^the response should contain echoed header "([^"]*)" with both values "([^"]*)" and "([^"]*)"$`, a.echoedHeaderShouldHaveBothValues)
	ctx.Step(`^the response should not contain echoed header "([^"]*)"$`, a.echoedHeaderShouldNotExist)
	ctx.Step(`^the response should contain echoed header "([^"]*)" containing "([^"]*)"$`, a.echoedHeaderShouldContain)

	// Response header assertions (alternative to existing headerShouldBe)
	ctx.Step(`^the response should have header "([^"]*)" with value "([^"]*)"$`, a.headerShouldBe)
	ctx.Step(`^the response should have header "([^"]*)" with values "([^"]*)" and "([^"]*)"$`, a.headerShouldHaveBothValues)
	ctx.Step(`^the response should not have header "([^"]*)"$`, a.headerShouldNotExist)
	ctx.Step(`^the response should have header "([^"]*)" containing "([^"]*)"$`, a.headerShouldContain)
}

// statusCodeShouldBe asserts the response status code
func (a *AssertSteps) statusCodeShouldBe(expected int) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.StatusCode != expected {
		return fmt.Errorf("expected status code %d, got %d", expected, resp.StatusCode)
	}
	return nil
}

// statusShouldBe asserts the response status text
func (a *AssertSteps) statusShouldBe(expected string) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.Status != expected && !strings.Contains(resp.Status, expected) {
		return fmt.Errorf("expected status %q, got %q", expected, resp.Status)
	}
	return nil
}

// responseShouldBeSuccessful asserts 2xx status
func (a *AssertSteps) responseShouldBeSuccessful() error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("expected successful response (2xx), got %d", resp.StatusCode)
	}
	return nil
}

// responseShouldBeClientError asserts 4xx status
func (a *AssertSteps) responseShouldBeClientError() error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		return fmt.Errorf("expected client error (4xx), got %d", resp.StatusCode)
	}
	return nil
}

// responseShouldBeServerError asserts 5xx status
func (a *AssertSteps) responseShouldBeServerError() error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.StatusCode < 500 || resp.StatusCode >= 600 {
		return fmt.Errorf("expected server error (5xx), got %d", resp.StatusCode)
	}
	return nil
}

// headerShouldBe asserts a header value
func (a *AssertSteps) headerShouldBe(name, expected string) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	actual := resp.Header.Get(name)
	if actual != expected {
		return fmt.Errorf("expected header %q to be %q, got %q", name, expected, actual)
	}
	return nil
}

// headerShouldContain asserts a header contains a value
func (a *AssertSteps) headerShouldContain(name, expected string) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	actual := resp.Header.Get(name)
	if !strings.Contains(actual, expected) {
		return fmt.Errorf("expected header %q to contain %q, got %q", name, expected, actual)
	}
	return nil
}

// headerShouldExist asserts a header exists
func (a *AssertSteps) headerShouldExist(name string) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.Header.Get(name) == "" {
		return fmt.Errorf("expected header %q to exist", name)
	}
	return nil
}

// headerShouldNotExist asserts a header does not exist
func (a *AssertSteps) headerShouldNotExist(name string) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}
	if resp.Header.Get(name) != "" {
		return fmt.Errorf("expected header %q to not exist, but got %q", name, resp.Header.Get(name))
	}
	return nil
}

// contentTypeShouldBe asserts the Content-Type header
func (a *AssertSteps) contentTypeShouldBe(expected string) error {
	return a.headerShouldContain("Content-Type", expected)
}

// bodyShouldContain asserts body contains text
func (a *AssertSteps) bodyShouldContain(expected string) error {
	body := string(a.provider.LastBody())
	if !strings.Contains(body, expected) {
		return fmt.Errorf("expected body to contain %q, got: %s", expected, truncate(body, 200))
	}
	return nil
}

// bodyShouldNotContain asserts body does not contain text
func (a *AssertSteps) bodyShouldNotContain(unexpected string) error {
	body := string(a.provider.LastBody())
	if strings.Contains(body, unexpected) {
		return fmt.Errorf("expected body to not contain %q, but it does", unexpected)
	}
	return nil
}

// bodyShouldBeEmpty asserts body is empty
func (a *AssertSteps) bodyShouldBeEmpty() error {
	body := a.provider.LastBody()
	if len(body) > 0 {
		return fmt.Errorf("expected empty body, got %d bytes", len(body))
	}
	return nil
}

// bodyShouldNotBeEmpty asserts body is not empty
func (a *AssertSteps) bodyShouldNotBeEmpty() error {
	body := a.provider.LastBody()
	if len(body) == 0 {
		return fmt.Errorf("expected non-empty body")
	}
	return nil
}

// bodyShouldMatchPattern asserts body matches regex
func (a *AssertSteps) bodyShouldMatchPattern(pattern string) error {
	body := string(a.provider.LastBody())
	matched, err := regexp.MatchString(pattern, body)
	if err != nil {
		return fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	if !matched {
		return fmt.Errorf("expected body to match pattern %q, got: %s", pattern, truncate(body, 200))
	}
	return nil
}

// bodyShouldBe asserts body equals exact content
func (a *AssertSteps) bodyShouldBe(expected *godog.DocString) error {
	body := string(a.provider.LastBody())
	if strings.TrimSpace(body) != strings.TrimSpace(expected.Content) {
		return fmt.Errorf("expected body:\n%s\ngot:\n%s", expected.Content, body)
	}
	return nil
}

// shouldBeValidJSON asserts body is valid JSON
func (a *AssertSteps) shouldBeValidJSON() error {
	body := a.provider.LastBody()
	var js interface{}
	if err := json.Unmarshal(body, &js); err != nil {
		return fmt.Errorf("expected valid JSON: %w", err)
	}
	return nil
}

// jsonShouldHaveField asserts JSON has a field
func (a *AssertSteps) jsonShouldHaveField(field string) error {
	value, err := a.getJSONField(field)
	if err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("expected JSON field %q to exist", field)
	}
	return nil
}

// jsonFieldShouldBe asserts JSON field equals string
func (a *AssertSteps) jsonFieldShouldBe(field, expected string) error {
	value, err := a.getJSONField(field)
	if err != nil {
		return err
	}
	actual := fmt.Sprintf("%v", value)
	if actual != expected {
		return fmt.Errorf("expected JSON field %q to be %q, got %q", field, expected, actual)
	}
	return nil
}

// jsonFieldShouldContain asserts JSON field contains string
func (a *AssertSteps) jsonFieldShouldContain(field, expected string) error {
	value, err := a.getJSONField(field)
	if err != nil {
		return err
	}
	actual := fmt.Sprintf("%v", value)
	if !strings.Contains(actual, expected) {
		return fmt.Errorf("expected JSON field %q to contain %q, got %q", field, expected, actual)
	}
	return nil
}

// jsonFieldShouldBeInt asserts JSON field equals int
func (a *AssertSteps) jsonFieldShouldBeInt(field string, expected int) error {
	value, err := a.getJSONField(field)
	if err != nil {
		return err
	}
	// JSON numbers are float64
	switch v := value.(type) {
	case float64:
		if int(v) != expected {
			return fmt.Errorf("expected JSON field %q to be %d, got %v", field, expected, v)
		}
	case int:
		if v != expected {
			return fmt.Errorf("expected JSON field %q to be %d, got %d", field, expected, v)
		}
	default:
		return fmt.Errorf("expected JSON field %q to be int, got %T", field, value)
	}
	return nil
}

// jsonFieldShouldBeBool asserts JSON field equals bool
func (a *AssertSteps) jsonFieldShouldBeBool(field, expected string) error {
	value, err := a.getJSONField(field)
	if err != nil {
		return err
	}
	expectedBool, _ := strconv.ParseBool(expected)
	actual, ok := value.(bool)
	if !ok {
		return fmt.Errorf("expected JSON field %q to be bool, got %T", field, value)
	}
	if actual != expectedBool {
		return fmt.Errorf("expected JSON field %q to be %v, got %v", field, expectedBool, actual)
	}
	return nil
}

// jsonShouldHaveItems asserts JSON array has N items
func (a *AssertSteps) jsonShouldHaveItems(expected int) error {
	body := a.provider.LastBody()
	var arr []interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		return fmt.Errorf("expected JSON array: %w", err)
	}
	if len(arr) != expected {
		return fmt.Errorf("expected %d items, got %d", expected, len(arr))
	}
	return nil
}

// getJSONField extracts a field from JSON body (supports dot notation)
// getJSONField extracts a field from JSON body (supports dot notation and array indices)
func (a *AssertSteps) getJSONField(field string) (interface{}, error) {
	body := a.provider.LastBody()
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	parts := strings.Split(field, ".")
	current := data

	for _, part := range parts {
		// Handle array index like "items[0]"
		if i := strings.Index(part, "["); i != -1 && strings.HasSuffix(part, "]") {
			key := part[:i]
			idxStr := part[i+1 : len(part)-1]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index in path %q: %v", part, err)
			}

			if key != "" {
				m, ok := current.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("expected map at %q but got %T", key, current)
				}
				v, exists := m[key]
				if !exists {
					return nil, fmt.Errorf("key %q does not exist in JSON", key)
				}
				current = v
			}

			l, ok := current.([]interface{})
			if !ok {
				return nil, fmt.Errorf("expected array at %q but got %T", part, current)
			}
			if idx < 0 || idx >= len(l) {
				return nil, fmt.Errorf("index out of bounds at %q: %d", part, idx)
			}
			current = l[idx]
		} else {
			m, ok := current.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected map at %q but got %T", part, current)
			}
			v, exists := m[part]
			if !exists {
				return nil, fmt.Errorf("key %q does not exist in JSON", part)
			}
			current = v
		}
	}

	return current, nil
}

// truncate truncates a string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// echoedHeaderShouldBe asserts an echoed header value from sample-backend JSON response
// Sample-backend returns headers in the JSON response under the "headers" field
func (a *AssertSteps) echoedHeaderShouldBe(headerName, expected string) error {
	body := a.provider.LastBody()
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	headers, ok := data["headers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("response does not contain headers field or is not a map")
	}

	// Headers are case-insensitive, normalize to lowercase
	normalizedName := strings.ToLower(headerName)
	var actualValue interface{}
	var found bool

	// Find header with case-insensitive matching
	for key, value := range headers {
		if strings.ToLower(key) == normalizedName {
			actualValue = value
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("expected echoed header %q to exist in response", headerName)
	}

	// Handle both string and array values
	switch v := actualValue.(type) {
	case string:
		if v != expected {
			return fmt.Errorf("expected echoed header %q to be %q, got %q", headerName, expected, v)
		}
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("expected echoed header %q to be %q, got empty array", headerName, expected)
		}
		firstVal := fmt.Sprintf("%v", v[0])
		if firstVal != expected {
			return fmt.Errorf("expected echoed header %q to be %q, got %q", headerName, expected, firstVal)
		}
	default:
		return fmt.Errorf("expected echoed header %q to be string or array, got %T", headerName, actualValue)
	}

	return nil
}

// echoedHeaderShouldHaveBothValues asserts an echoed header has both values (multi-value header)
func (a *AssertSteps) echoedHeaderShouldHaveBothValues(headerName, value1, value2 string) error {
	body := a.provider.LastBody()
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	headers, ok := data["headers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("response does not contain headers field or is not a map")
	}

	normalizedName := strings.ToLower(headerName)
	var actualValue interface{}
	var found bool

	for key, value := range headers {
		if strings.ToLower(key) == normalizedName {
			actualValue = value
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("expected echoed header %q to exist in response", headerName)
	}

	// Check if it's an array with both values
	arr, ok := actualValue.([]interface{})
	if !ok {
		// Could be a single comma-separated string
		if strVal, ok := actualValue.(string); ok {
			if !strings.Contains(strVal, value1) || !strings.Contains(strVal, value2) {
				return fmt.Errorf("expected echoed header %q to contain both %q and %q, got %q", headerName, value1, value2, strVal)
			}
			return nil
		}
		return fmt.Errorf("expected echoed header %q to be an array, got %T", headerName, actualValue)
	}

	foundValue1 := false
	foundValue2 := false

	for _, item := range arr {
		itemStr := fmt.Sprintf("%v", item)
		if itemStr == value1 {
			foundValue1 = true
		}
		if itemStr == value2 {
			foundValue2 = true
		}
	}

	if !foundValue1 {
		return fmt.Errorf("expected echoed header %q to contain value %q", headerName, value1)
	}
	if !foundValue2 {
		return fmt.Errorf("expected echoed header %q to contain value %q", headerName, value2)
	}

	return nil
}

// echoedHeaderShouldNotExist asserts an echoed header does not exist
func (a *AssertSteps) echoedHeaderShouldNotExist(headerName string) error {
	body := a.provider.LastBody()
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	headers, ok := data["headers"].(map[string]interface{})
	if !ok {
		// No headers field means header doesn't exist
		return nil
	}

	normalizedName := strings.ToLower(headerName)

	for key := range headers {
		if strings.ToLower(key) == normalizedName {
			return fmt.Errorf("expected echoed header %q to not exist, but it does", headerName)
		}
	}

	return nil
}

// echoedHeaderShouldContain asserts an echoed header contains a value
func (a *AssertSteps) echoedHeaderShouldContain(headerName, expected string) error {
	body := a.provider.LastBody()
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	headers, ok := data["headers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("response does not contain headers field or is not a map")
	}

	normalizedName := strings.ToLower(headerName)
	var actualValue interface{}
	var found bool

	for key, value := range headers {
		if strings.ToLower(key) == normalizedName {
			actualValue = value
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("expected echoed header %q to exist in response", headerName)
	}

	actualStr := fmt.Sprintf("%v", actualValue)
	if !strings.Contains(actualStr, expected) {
		return fmt.Errorf("expected echoed header %q to contain %q, got %q", headerName, expected, actualStr)
	}

	return nil
}

// headerShouldHaveBothValues asserts a response header has both values (multi-value header)
func (a *AssertSteps) headerShouldHaveBothValues(headerName, value1, value2 string) error {
	resp := a.provider.LastResponse()
	if resp == nil {
		return fmt.Errorf("no response received")
	}

	values := resp.Header.Values(headerName)
	if len(values) == 0 {
		return fmt.Errorf("expected header %q to exist", headerName)
	}

	foundValue1 := false
	foundValue2 := false

	for _, val := range values {
		if val == value1 {
			foundValue1 = true
		}
		if val == value2 {
			foundValue2 = true
		}
	}

	if !foundValue1 {
		return fmt.Errorf("expected header %q to contain value %q, got %v", headerName, value1, values)
	}
	if !foundValue2 {
		return fmt.Errorf("expected header %q to contain value %q, got %v", headerName, value2, values)
	}

	return nil
}

// printResponseBody prints the response body for debugging
func (a *AssertSteps) printResponseBody() error {
	body := string(a.provider.LastBody())
	fmt.Printf("\n═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("DEBUG: Response Status: %d\n", a.provider.LastResponse().StatusCode)
	fmt.Printf("DEBUG: Response Body:\n%s\n", body)
	fmt.Printf("═══════════════════════════════════════════════════════════════\n\n")
	return nil
}
