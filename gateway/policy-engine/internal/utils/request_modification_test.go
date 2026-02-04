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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// AddQueryParametersToPath Tests
// =============================================================================

func TestAddQueryParametersToPath_EmptyParameters(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{}

	result := AddQueryParametersToPath(path, params)

	assert.Equal(t, "/api/users", result)
}

func TestAddQueryParametersToPath_SingleParameter(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{
		"id": {"123"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "id=123")
	assert.Contains(t, result, "/api/users?")
}

func TestAddQueryParametersToPath_MultipleParameters(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{
		"id":   {"123"},
		"name": {"john"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "id=123")
	assert.Contains(t, result, "name=john")
	assert.Contains(t, result, "/api/users?")
}

func TestAddQueryParametersToPath_MultipleValuesForSameKey(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{
		"tag": {"go", "rust", "python"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "tag=go")
	assert.Contains(t, result, "tag=rust")
	assert.Contains(t, result, "tag=python")
}

func TestAddQueryParametersToPath_ExistingQueryParams(t *testing.T) {
	path := "/api/users?existing=value"
	params := map[string][]string{
		"new": {"param"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "existing=value")
	assert.Contains(t, result, "new=param")
}

func TestAddQueryParametersToPath_SpecialCharacters(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{
		"query": {"hello world"},
		"email": {"user@example.com"},
	}

	result := AddQueryParametersToPath(path, params)

	// Values should be URL encoded
	assert.Contains(t, result, "query=hello+world") // space encoded as +
	assert.Contains(t, result, "email=user%40example.com") // @ encoded
}

func TestAddQueryParametersToPath_EmptyPath(t *testing.T) {
	path := ""
	params := map[string][]string{
		"id": {"123"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "id=123")
}

func TestAddQueryParametersToPath_PathWithFragment(t *testing.T) {
	path := "/api/users#section"
	params := map[string][]string{
		"id": {"123"},
	}

	result := AddQueryParametersToPath(path, params)

	// Query params should come before fragment
	assert.Contains(t, result, "id=123")
	assert.Contains(t, result, "#section")
}

func TestAddQueryParametersToPath_RootPath(t *testing.T) {
	path := "/"
	params := map[string][]string{
		"key": {"value"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "/?key=value")
}

func TestAddQueryParametersToPath_ComplexPath(t *testing.T) {
	path := "/api/v1/users/123/orders"
	params := map[string][]string{
		"status":  {"pending"},
		"limit":   {"10"},
		"offset":  {"0"},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "/api/v1/users/123/orders?")
	assert.Contains(t, result, "status=pending")
	assert.Contains(t, result, "limit=10")
	assert.Contains(t, result, "offset=0")
}

// =============================================================================
// RemoveQueryParametersFromPath Tests
// =============================================================================

func TestRemoveQueryParametersFromPath_NoExistingParams(t *testing.T) {
	path := "/api/users"
	params := []string{"id"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.Equal(t, "/api/users", result)
}

func TestRemoveQueryParametersFromPath_RemoveSingleParam(t *testing.T) {
	path := "/api/users?id=123&name=john"
	params := []string{"id"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.NotContains(t, result, "id=123")
	assert.Contains(t, result, "name=john")
}

func TestRemoveQueryParametersFromPath_RemoveMultipleParams(t *testing.T) {
	path := "/api/users?id=123&name=john&age=30"
	params := []string{"id", "age"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.NotContains(t, result, "id=123")
	assert.NotContains(t, result, "age=30")
	assert.Contains(t, result, "name=john")
}

func TestRemoveQueryParametersFromPath_RemoveAllParams(t *testing.T) {
	path := "/api/users?id=123&name=john"
	params := []string{"id", "name"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.NotContains(t, result, "id=123")
	assert.NotContains(t, result, "name=john")
	assert.Equal(t, "/api/users", result)
}

func TestRemoveQueryParametersFromPath_RemoveNonExistentParam(t *testing.T) {
	path := "/api/users?id=123"
	params := []string{"nonexistent"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.Contains(t, result, "id=123")
}

func TestRemoveQueryParametersFromPath_EmptyParamsToRemove(t *testing.T) {
	path := "/api/users?id=123&name=john"
	params := []string{}

	result := RemoveQueryParametersFromPath(path, params)

	assert.Contains(t, result, "id=123")
	assert.Contains(t, result, "name=john")
}

func TestRemoveQueryParametersFromPath_RemoveMultiValueParam(t *testing.T) {
	path := "/api/users?tag=go&tag=rust&name=john"
	params := []string{"tag"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.NotContains(t, result, "tag=go")
	assert.NotContains(t, result, "tag=rust")
	assert.Contains(t, result, "name=john")
}

func TestRemoveQueryParametersFromPath_PreservePathFragment(t *testing.T) {
	path := "/api/users?id=123#section"
	params := []string{"id"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.NotContains(t, result, "id=123")
	assert.Contains(t, result, "#section")
}

func TestRemoveQueryParametersFromPath_EncodedParams(t *testing.T) {
	path := "/api/users?query=hello%20world&name=john"
	params := []string{"query"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.NotContains(t, result, "query=")
	assert.Contains(t, result, "name=john")
}

func TestRemoveQueryParametersFromPath_CaseSensitive(t *testing.T) {
	path := "/api/users?ID=123&id=456"
	params := []string{"id"}

	result := RemoveQueryParametersFromPath(path, params)

	// Should only remove lowercase 'id', not 'ID'
	assert.Contains(t, result, "ID=123")
	assert.NotContains(t, result, "id=456")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestAddQueryParametersToPath_UnicodeCharacters(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{
		"name": {"日本語"},
	}

	result := AddQueryParametersToPath(path, params)

	// Unicode should be URL encoded
	assert.Contains(t, result, "name=")
	assert.Contains(t, result, "/api/users?")
}

func TestAddQueryParametersToPath_EmptyValue(t *testing.T) {
	path := "/api/users"
	params := map[string][]string{
		"key": {""},
	}

	result := AddQueryParametersToPath(path, params)

	assert.Contains(t, result, "key=")
}

func TestRemoveQueryParametersFromPath_EmptyPath(t *testing.T) {
	path := ""
	params := []string{"id"}

	result := RemoveQueryParametersFromPath(path, params)

	assert.Equal(t, "", result)
}

// =============================================================================
// AddQueryParametersToPath URL Parsing Failure Fallback Tests
// =============================================================================

func TestAddQueryParametersToPath_InvalidURLFallback_NoExistingQuery(t *testing.T) {
	// Use a control character in the URL to make url.Parse fail
	path := "http://example.com\x00/path"
	params := map[string][]string{
		"key": {"value"},
	}

	result := AddQueryParametersToPath(path, params)

	// Should fallback to simple append with "?"
	assert.Contains(t, result, "?key=value")
}

func TestAddQueryParametersToPath_InvalidURLFallback_WithExistingQuery(t *testing.T) {
	// Use a control character in the URL with existing "?" to make url.Parse fail
	path := "http://example.com\x00/path?existing=param"
	params := map[string][]string{
		"key": {"value"},
	}

	result := AddQueryParametersToPath(path, params)

	// Should fallback to simple append with "&" since "?" already exists
	assert.Contains(t, result, "&key=value")
	assert.Contains(t, result, "?existing=param")
}

func TestAddQueryParametersToPath_InvalidURLFallback_MultipleParams(t *testing.T) {
	// Use invalid URL escape to make url.Parse fail
	path := "%zzinvalid/path"
	params := map[string][]string{
		"a": {"1"},
		"b": {"2"},
	}

	result := AddQueryParametersToPath(path, params)

	// Should contain both parameters with proper separators
	assert.Contains(t, result, "a=1")
	assert.Contains(t, result, "b=2")
	assert.Contains(t, result, "?")
}

func TestAddQueryParametersToPath_InvalidURLFallback_MultipleValues(t *testing.T) {
	// Use control character to make url.Parse fail
	path := "/path\x00here"
	params := map[string][]string{
		"tags": {"go", "rust"},
	}

	result := AddQueryParametersToPath(path, params)

	// Should contain both values for the same key
	assert.Contains(t, result, "tags=go")
	assert.Contains(t, result, "tags=rust")
}

func TestAddQueryParametersToPath_InvalidURLFallback_SpecialCharsEncoded(t *testing.T) {
	// Use control character to trigger fallback
	path := "/api\x00/users"
	params := map[string][]string{
		"name": {"John Doe"},
		"email": {"test@example.com"},
	}

	result := AddQueryParametersToPath(path, params)

	// Values should be URL encoded in fallback path
	assert.Contains(t, result, "name=John+Doe")
	assert.Contains(t, result, "email=test%40example.com")
}

// =============================================================================
// RemoveQueryParametersFromPath URL Parsing Failure Tests
// =============================================================================

func TestRemoveQueryParametersFromPath_InvalidURLReturnsOriginal(t *testing.T) {
	// Use a control character in the URL to make url.Parse fail
	path := "http://example.com\x00/path?id=123"
	params := []string{"id"}

	result := RemoveQueryParametersFromPath(path, params)

	// Should return the original path when parsing fails
	assert.Equal(t, path, result)
}

func TestRemoveQueryParametersFromPath_InvalidURLEscapeReturnsOriginal(t *testing.T) {
	// Use invalid URL escape to make url.Parse fail
	path := "%zzinvalid/path?key=value"
	params := []string{"key"}

	result := RemoveQueryParametersFromPath(path, params)

	// Should return the original path when parsing fails
	assert.Equal(t, path, result)
}

