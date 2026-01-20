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

package jsontoxml

import (
	"encoding/json"
	"strings"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// Helper function to create test headers
func createTestHeaders(key, value string) *policy.Headers {
	headers := make(map[string][]string)
	headers[key] = []string{value}
	return policy.NewHeaders(headers)
}

func TestJSONToXMLPolicy_Mode(t *testing.T) {
	p := &JSONToXMLPolicy{}
	mode := p.Mode()

	expectedMode := policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}

	if mode != expectedMode {
		t.Errorf("Expected mode %+v, got %+v", expectedMode, mode)
	}
}

func TestJSONToXMLPolicy_OnRequest_DisabledByDefault(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"name": "test"}`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	// No parameters - should be disabled by default
	result := p.OnRequest(ctx, map[string]interface{}{})

	// Should return empty modifications (no transformation)
	if _, ok := result.(policy.UpstreamRequestModifications); !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification when disabled, got body: %s", string(mods.Body))
	}
}

func TestJSONToXMLPolicy_OnRequest_EnabledWithParameter(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"name": "John Doe", "age": 30}`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications, got %T", result)
	}

	// Check body was transformed
	if mods.Body == nil {
		t.Fatal("Expected body to be transformed, got nil")
	}

	xmlStr := string(mods.Body)
	if strings.Contains(xmlStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Errorf("Expected no XML declaration, got: %s", xmlStr)
	}

	if !strings.Contains(xmlStr, "<name>John Doe</name>") {
		t.Errorf("Expected name element, got: %s", xmlStr)
	}

	if !strings.Contains(xmlStr, "<age>30</age>") {
		t.Errorf("Expected age element, got: %s", xmlStr)
	}

	// Check headers were updated
	if mods.SetHeaders["content-type"] != "application/xml" {
		t.Errorf("Expected content-type to be application/xml, got: %s", mods.SetHeaders["content-type"])
	}

	if mods.SetHeaders["content-length"] == "" {
		t.Errorf("Expected content-length to be set")
	}
}

func TestJSONToXMLPolicy_OnRequest_NoBody(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte{},
			Present: false,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification for empty body, got: %s", string(mods.Body))
	}
}

func TestJSONToXMLPolicy_OnRequest_WrongContentType(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"name": "test"}`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/xml"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return internal server error for wrong content type
	if immediate, ok := result.(policy.ImmediateResponse); ok {
		if immediate.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", immediate.StatusCode)
		}

		var errorResp map[string]interface{}
		if err := json.Unmarshal(immediate.Body, &errorResp); err != nil {
			t.Errorf("Failed to unmarshal error response: %v", err)
		}

		if errorResp["error"] != "Internal Server Error" {
			t.Errorf("Expected error 'Internal Server Error', got %v", errorResp["error"])
		}
	} else {
		t.Errorf("Expected ImmediateResponse for wrong content type, got %T", result)
	}
}

func TestJSONToXMLPolicy_OnRequest_InvalidJSON(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"name": "test", invalid json}`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return internal server error for invalid JSON
	if immediate, ok := result.(policy.ImmediateResponse); ok {
		if immediate.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", immediate.StatusCode)
		}
	} else {
		t.Errorf("Expected ImmediateResponse for invalid JSON, got %T", result)
	}
}

func TestJSONToXMLPolicy_OnResponse_DisabledByDefault(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"status": "success"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/json"),
	}

	// No parameters - should be disabled by default
	result := p.OnResponse(ctx, map[string]interface{}{})

	mods := result.(policy.UpstreamResponseModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification when disabled, got body: %s", string(mods.Body))
	}
}

func TestJSONToXMLPolicy_OnResponse_EnabledWithParameter(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"status": "success", "data": {"id": 123}}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	mods := result.(policy.UpstreamResponseModifications)

	// Check body was transformed
	if mods.Body == nil {
		t.Fatal("Expected body to be transformed, got nil")
	}

	xmlStr := string(mods.Body)
	if !strings.Contains(xmlStr, "<status>success</status>") {
		t.Errorf("Expected status element, got: %s", xmlStr)
	}

	if !strings.Contains(xmlStr, "<id>123</id>") {
		t.Errorf("Expected id element, got: %s", xmlStr)
	}
}

func TestJSONToXMLPolicy_OnResponse_InvalidJSON_Error(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`invalid json`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	// Should return 500 error for invalid JSON in response
	mods := result.(policy.UpstreamResponseModifications)
	if mods.StatusCode == nil || *mods.StatusCode != 500 {
		t.Errorf("Expected status code 500 for invalid JSON response, got: %v", mods.StatusCode)
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_SimpleObject(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{
		"name":   "John",
		"age":    30,
		"active": true,
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)

	// No XML declaration expected
	if strings.Contains(xmlStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Errorf("Expected no XML declaration, got: %s", xmlStr)
	}

	if !strings.Contains(xmlStr, "<name>John</name>") {
		t.Errorf("Expected name element")
	}

	if !strings.Contains(xmlStr, "<age>30</age>") {
		t.Errorf("Expected age element")
	}

	if !strings.Contains(xmlStr, "<active>true</active>") {
		t.Errorf("Expected active element")
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_Array(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{
		"skills": []interface{}{"Java", "Python", "Go"},
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)

	// Should NOT singularize "skills" - use "skills" for each item
	if !strings.Contains(xmlStr, "<skills>Java</skills>") {
		t.Errorf("Expected skills element for Java, got: %s", xmlStr)
	}

	if !strings.Contains(xmlStr, "<skills>Python</skills>") {
		t.Errorf("Expected skills element for Python")
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_NestedObject(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Jane",
			"address": map[string]interface{}{
				"city": "New York",
				"zip":  "10001",
			},
		},
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)

	if !strings.Contains(xmlStr, "<name>Jane</name>") {
		t.Errorf("Expected nested name element")
	}

	if !strings.Contains(xmlStr, "<city>New York</city>") {
		t.Errorf("Expected nested city element")
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_NullValue(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{
		"name":   "John",
		"middle": nil,
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)

	if !strings.Contains(xmlStr, "<middle></middle>") {
		t.Errorf("Expected empty middle element for null value")
	}
}

func TestJSONToXMLPolicy_ComplexJSONConversion(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// Complex JSON with mixed data types
	jsonStr := `{
		"user": {
			"id": 123,
			"name": "Alice Smith",
			"active": true,
			"balance": 45.67,
			"tags": ["premium", "verified"],
			"preferences": {
				"theme": "dark",
				"notifications": false
			},
			"metadata": null
		},
		"timestamp": 1642781234
	}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse test JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)

	// Verify structure
	if !strings.Contains(xmlStr, "<id>123</id>") {
		t.Errorf("Expected id element")
	}

	if !strings.Contains(xmlStr, "<name>Alice Smith</name>") {
		t.Errorf("Expected name element")
	}

	if !strings.Contains(xmlStr, "<active>true</active>") {
		t.Errorf("Expected active element")
	}

	if !strings.Contains(xmlStr, "<balance>45.67</balance>") {
		t.Errorf("Expected balance element")
	}

	if !strings.Contains(xmlStr, "<tags>premium</tags>") {
		t.Errorf("Expected tags element (not singularized)")
	}

	if !strings.Contains(xmlStr, "<theme>dark</theme>") {
		t.Errorf("Expected nested theme element")
	}

	if !strings.Contains(xmlStr, "<metadata></metadata>") {
		t.Errorf("Expected empty metadata element")
	}
}

func TestJSONToXMLPolicy_BothFlowsEnabled(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// Test request flow
	reqCtx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"action": "create"}`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onRequestFlow":  true,
		"onResponseFlow": true,
	}

	reqResult := p.OnRequest(reqCtx, params)
	reqMods := reqResult.(policy.UpstreamRequestModifications)

	if reqMods.Body == nil {
		t.Errorf("Expected request body to be transformed")
	}

	// Test response flow
	respCtx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`{"status": "created"}`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/json"),
	}

	respResult := p.OnResponse(respCtx, params)
	respMods := respResult.(policy.UpstreamResponseModifications)

	if respMods.Body == nil {
		t.Errorf("Expected response body to be transformed")
	}
}

func TestJSONToXMLPolicy_ParameterTypes(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"test": "data"}`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	// Test with string parameter (should not work)
	params := map[string]interface{}{
		"onRequestFlow": "true", // string instead of bool
	}

	result := p.OnRequest(ctx, params)
	mods := result.(policy.UpstreamRequestModifications)

	if mods.Body != nil {
		t.Errorf("Expected no transformation with string parameter, got body: %s", string(mods.Body))
	}
}

// Additional edge case tests

func TestJSONToXMLPolicy_OnRequest_NilBody(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body:    nil, // nil body
		Headers: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	mods := result.(policy.UpstreamRequestModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification for nil body, got: %s", string(mods.Body))
	}
}

func TestJSONToXMLPolicy_OnResponse_NilBody(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody:    nil, // nil body
		ResponseHeaders: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	mods := result.(policy.UpstreamResponseModifications)
	if mods.Body != nil {
		t.Errorf("Expected no body modification for nil response body, got: %s", string(mods.Body))
	}
}

func TestJSONToXMLPolicy_OnRequest_NoContentTypeHeader(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`{"test": "data"}`),
			Present: true,
		},
		Headers: createTestHeaders("other-header", "value"), // no content-type
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return internal server error for missing JSON content type
	if immediate, ok := result.(policy.ImmediateResponse); ok {
		if immediate.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", immediate.StatusCode)
		}
	} else {
		t.Errorf("Expected ImmediateResponse for missing content type, got %T", result)
	}
}

func TestJSONToXMLPolicy_OnResponse_WrongContentType_Error(t *testing.T) {
	p := &JSONToXMLPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`<xml>data</xml>`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/xml"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	// Should return 500 error for wrong content type in response
	mods := result.(policy.UpstreamResponseModifications)
	if mods.StatusCode == nil || *mods.StatusCode != 500 {
		t.Errorf("Expected status code 500 for wrong content type response, got: %v", mods.StatusCode)
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_EmptyObject(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, "<root>") || !strings.Contains(xmlStr, "</root>") {
		t.Errorf("Expected root element for empty object")
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_RootArray(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := []interface{}{"item1", "item2", "item3"}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, "<item0>item1</item0>") {
		t.Errorf("Expected item0 element for root array")
	}
	if !strings.Contains(xmlStr, "<item1>item2</item1>") {
		t.Errorf("Expected item1 element for root array")
	}
}

func TestJSONToXMLPolicy_ConvertJSONToXML_NumberTypes(t *testing.T) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{
		"integer": 42,
		"float":   3.14159,
		"zero":    0,
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	if !strings.Contains(xmlStr, "<integer>42</integer>") {
		t.Errorf("Expected integer element")
	}
	if !strings.Contains(xmlStr, "<float>3.14159</float>") {
		t.Errorf("Expected float element")
	}
	if !strings.Contains(xmlStr, "<zero>0</zero>") {
		t.Errorf("Expected zero element")
	}
}

func TestGetPolicy(t *testing.T) {
	policy, err := GetPolicy(policy.PolicyMetadata{}, map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected no error from GetPolicy, got: %v", err)
	}
	if policy == nil {
		t.Error("Expected policy instance, got nil")
	}
	if _, ok := policy.(*JSONToXMLPolicy); !ok {
		t.Errorf("Expected JSONToXMLPolicy, got %T", policy)
	}
}

// Benchmark tests
func BenchmarkJSONToXMLConversion(b *testing.B) {
	p := &JSONToXMLPolicy{}
	jsonData := map[string]interface{}{
		"user": map[string]interface{}{
			"id":   123,
			"name": "Test User",
			"tags": []interface{}{"tag1", "tag2", "tag3"},
		},
		"active": true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ConvertJSONToXML(jsonData)
		if err != nil {
			b.Fatalf("Conversion failed: %v", err)
		}
	}
}

// Test the specific books example transformation
func TestJSONToXMLPolicy_BooksExample(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// Your example JSON
	jsonStr := `{
  "books": [
    {
      "author": "J. R. R. Tolkien",
      "id": "fe2594d0-ccea-42a2-97ac-0487458b5642",
      "status": "to_read",
      "title": "The Lord of the Rings"
    },
    {
      "author": "J. R. R. Tolkien", 
      "id": "fe2594d0-ccea-42a2-97ac-0487458b5643",
      "status": "to_read", 
      "title": "The Hobbit"
    },
    {
      "author": "George Orwell",
      "id": "fe2594d0-ccea-42a2-97ac-0487458b5644", 
      "status": "read",
      "title": "1984"
    }
  ]
}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse test JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	t.Logf("Generated XML:\n%s", xmlStr)

	// Verify structure
	if !strings.Contains(xmlStr, "<books>") {
		t.Errorf("Expected books elements")
	}

	// Verify each book has the expected structure
	if !strings.Contains(xmlStr, "<author>J. R. R. Tolkien</author>") {
		t.Errorf("Expected author element")
	}

	if !strings.Contains(xmlStr, "<title>The Lord of the Rings</title>") {
		t.Errorf("Expected title element")
	}

	// Count occurrences of <books> to ensure we have 3 books
	booksCount := strings.Count(xmlStr, "<books>")
	if booksCount != 3 {
		t.Errorf("Expected 3 books elements, got %d", booksCount)
	}
}

// Complex JSON test cases for thorough validation

func TestJSONToXMLPolicy_ComplexNestedStructures(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// Deeply nested JSON with mixed data types
	jsonStr := `{
		"organization": {
			"name": "Tech Corp",
			"id": 12345,
			"active": true,
			"departments": [
				{
					"name": "Engineering",
					"employees": [
						{
							"id": 101,
							"name": "Alice Johnson",
							"skills": ["Go", "Python", "Kubernetes"],
							"contact": {
								"email": "alice@techcorp.com",
								"phone": "+1-555-0101",
								"address": {
									"street": "123 Tech St",
									"city": "San Francisco",
									"state": "CA",
									"zipcode": "94105",
									"coordinates": {
										"lat": 37.7749,
										"lng": -122.4194
									}
								}
							},
							"projects": [
								{
									"name": "Project Alpha",
									"status": "active",
									"tags": ["critical", "backend", "api"]
								},
								{
									"name": "Project Beta", 
									"status": "completed",
									"tags": ["frontend", "ui"]
								}
							]
						},
						{
							"id": 102,
							"name": "Bob Smith",
							"skills": ["JavaScript", "React", "Node.js"],
							"contact": {
								"email": "bob@techcorp.com",
								"phone": "+1-555-0102",
								"address": null
							},
							"projects": []
						}
					]
				},
				{
					"name": "Marketing",
					"employees": [
						{
							"id": 201,
							"name": "Carol Davis",
							"skills": ["SEO", "Content Marketing"],
							"contact": {
								"email": "carol@techcorp.com",
								"phone": null,
								"address": {
									"street": "456 Market Ave",
									"city": "New York",
									"state": "NY",
									"zipcode": "10001",
									"coordinates": null
								}
							},
							"projects": [
								{
									"name": "Brand Campaign",
									"status": "planning",
									"tags": ["branding", "social-media"]
								}
							]
						}
					]
				}
			],
			"metadata": {
				"created": "2024-01-01T00:00:00Z",
				"updated": "2024-12-31T23:59:59Z",
				"version": "1.0.0",
				"settings": {
					"notifications": {
						"email": true,
						"sms": false,
						"push": true
					},
					"features": {
						"analytics": true,
						"reporting": true,
						"backup": false
					}
				}
			}
		}
	}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse complex JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	t.Logf("Complex nested XML:\n%s", xmlStr)

	// Verify deep nesting
	if !strings.Contains(xmlStr, "<organization>") {
		t.Errorf("Expected organization element")
	}

	// Verify arrays are not singularized
	if !strings.Contains(xmlStr, "<departments>") {
		t.Errorf("Expected departments elements")
	}
	if !strings.Contains(xmlStr, "<employees>") {
		t.Errorf("Expected employees elements")
	}
	if !strings.Contains(xmlStr, "<skills>") {
		t.Errorf("Expected skills elements")
	}

	// Verify nested object structure
	if !strings.Contains(xmlStr, "<coordinates>") {
		t.Errorf("Expected coordinates element")
	}
	if !strings.Contains(xmlStr, "<lat>37.7749</lat>") {
		t.Errorf("Expected lat coordinate")
	}

	// Verify null values
	if !strings.Contains(xmlStr, "<address></address>") {
		t.Errorf("Expected empty address element for null value")
	}

	// Verify empty arrays are handled
	projectsCount := strings.Count(xmlStr, "<projects>")
	if projectsCount < 2 {
		t.Errorf("Expected multiple projects elements")
	}
}

func TestJSONToXMLPolicy_MixedArrayTypes(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// JSON with arrays containing different data types
	jsonStr := `{
		"data": {
			"numbers": [1, 2.5, -3, 0, 999.999],
			"strings": ["hello", "world", "", "test with spaces", "special@chars!"],
			"booleans": [true, false, true],
			"nulls": [null, null, null],
			"mixed": [1, "text", true, null, 42.5],
			"objects": [
				{"type": "A", "value": 100},
				{"type": "B", "value": 200},
				{"type": "C", "value": null}
			],
			"nested_arrays": [
				["a", "b", "c"],
				[1, 2, 3],
				[true, false]
			]
		}
	}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse mixed array JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	t.Logf("Mixed arrays XML:\n%s", xmlStr)

	// Verify different number formats
	if !strings.Contains(xmlStr, "<numbers>1</numbers>") {
		t.Errorf("Expected integer number")
	}
	if !strings.Contains(xmlStr, "<numbers>2.5</numbers>") {
		t.Errorf("Expected decimal number")
	}
	if !strings.Contains(xmlStr, "<numbers>-3</numbers>") {
		t.Errorf("Expected negative number")
	}

	// Verify string handling
	if !strings.Contains(xmlStr, "<strings>hello</strings>") {
		t.Errorf("Expected string element")
	}
	if !strings.Contains(xmlStr, "<strings></strings>") {
		t.Errorf("Expected empty string element")
	}

	// Verify boolean handling
	if !strings.Contains(xmlStr, "<booleans>true</booleans>") {
		t.Errorf("Expected true boolean")
	}
	if !strings.Contains(xmlStr, "<booleans>false</booleans>") {
		t.Errorf("Expected false boolean")
	}

	// Verify null handling
	nullCount := strings.Count(xmlStr, "<nulls></nulls>")
	if nullCount != 3 {
		t.Errorf("Expected 3 null elements, got %d", nullCount)
	}

	// Verify mixed types
	if !strings.Contains(xmlStr, "<mixed>1</mixed>") {
		t.Errorf("Expected mixed number")
	}
	if !strings.Contains(xmlStr, "<mixed>text</mixed>") {
		t.Errorf("Expected mixed string")
	}
}

func TestJSONToXMLPolicy_UnicodeAndSpecialCharacters(t *testing.T) {
	p := &JSONToXMLPolicy{}

	jsonStr := `{
		"unicode": {
			"emoji": "🎉🚀💻",
			"chinese": "你好世界",
			"arabic": "مرحبا بالعالم",
			"japanese": "こんにちは世界",
			"russian": "Привет мир",
			"mathematical": "∑∆∇∞≠≤≥"
		},
		"special_chars": {
			"html_entities": "<>&\"'",
			"xml_chars": "<?xml version=\"1.0\"?>",
			"json_chars": "{\"key\": \"value\"}",
			"whitespace": "line1\nline2\tline3\r\nline4",
			"symbols": "!@#$%^&*()[]{}|\\:;\"'<>?,./` + "`" + `~"
		},
		"edge_cases": {
			"very_long_string": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.",
			"numbers_as_strings": ["0001", "0.0000", "1e10", "-999"],
			"empty_values": ["", null, 0, false]
		}
	}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse unicode JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	t.Logf("Unicode XML:\n%s", xmlStr)

	// Verify unicode characters are preserved
	if !strings.Contains(xmlStr, "🎉🚀💻") {
		t.Errorf("Expected emoji characters")
	}
	if !strings.Contains(xmlStr, "你好世界") {
		t.Errorf("Expected Chinese characters")
	}

	// Verify special characters are handled
	if !strings.Contains(xmlStr, "<html_entities>") {
		t.Errorf("Expected html_entities element")
	}

	// Verify long strings are handled
	if !strings.Contains(xmlStr, "Lorem ipsum") {
		t.Errorf("Expected long string content")
	}
}

func TestJSONToXMLPolicy_APIResponseExamples(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// Real-world API response example
	jsonStr := `{
		"status": "success",
		"code": 200,
		"message": "Data retrieved successfully",
		"timestamp": "2024-01-15T10:30:45Z",
		"data": {
			"users": [
				{
					"id": "usr_123456789",
					"username": "john_doe",
					"email": "john@example.com",
					"profile": {
						"firstName": "John",
						"lastName": "Doe",
						"avatar": "https://example.com/avatars/john.jpg",
						"bio": null,
						"preferences": {
							"theme": "dark",
							"language": "en-US",
							"timezone": "America/New_York",
							"notifications": {
								"email": true,
								"push": false,
								"sms": true
							}
						}
					},
					"roles": ["user", "premium"],
					"permissions": [
						{
							"resource": "posts",
							"actions": ["read", "write", "delete"]
						},
						{
							"resource": "comments",
							"actions": ["read", "write"]
						}
					],
					"metadata": {
						"createdAt": "2023-06-15T09:20:00Z",
						"updatedAt": "2024-01-10T14:30:22Z",
						"lastLogin": "2024-01-15T08:45:12Z",
						"loginCount": 1247,
						"isActive": true,
						"tags": ["beta-tester", "power-user"]
					}
				}
			],
			"pagination": {
				"page": 1,
				"limit": 10,
				"total": 1,
				"totalPages": 1,
				"hasNext": false,
				"hasPrev": false
			}
		},
		"links": {
			"self": "https://api.example.com/v1/users?page=1&limit=10",
			"next": null,
			"prev": null
		}
	}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse API response JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	t.Logf("API response XML:\n%s", xmlStr)

	// Verify top-level structure
	if !strings.Contains(xmlStr, "<status>success</status>") {
		t.Errorf("Expected status element")
	}
	if !strings.Contains(xmlStr, "<code>200</code>") {
		t.Errorf("Expected code element")
	}

	// Verify user array structure
	if !strings.Contains(xmlStr, "<users>") {
		t.Errorf("Expected users elements")
	}

	// Verify nested permissions array
	if !strings.Contains(xmlStr, "<permissions>") {
		t.Errorf("Expected permissions elements")
	}
	if !strings.Contains(xmlStr, "<actions>read</actions>") {
		t.Errorf("Expected actions elements")
	}

	// Verify URL handling
	if !strings.Contains(xmlStr, "https://api.example.com") {
		t.Errorf("Expected URL in XML")
	}

	// Verify boolean and numeric values
	if !strings.Contains(xmlStr, "<isActive>true</isActive>") {
		t.Errorf("Expected boolean value")
	}
	if !strings.Contains(xmlStr, "<loginCount>1247</loginCount>") {
		t.Errorf("Expected numeric value")
	}
}

func TestJSONToXMLPolicy_ECommerceExample(t *testing.T) {
	p := &JSONToXMLPolicy{}

	// E-commerce order example with complex nested structure
	jsonStr := `{
		"order": {
			"orderId": "ORD-2024-001234",
			"orderNumber": "24001234",
			"status": "processing",
			"customer": {
				"customerId": "CUST-789123",
				"email": "customer@example.com",
				"name": {
					"first": "Jane",
					"last": "Smith",
					"middle": "M"
				},
				"addresses": [
					{
						"type": "billing",
						"street1": "123 Billing St",
						"street2": "Apt 4B",
						"city": "New York",
						"state": "NY",
						"zipCode": "10001",
						"country": "US",
						"isDefault": true
					},
					{
						"type": "shipping",
						"street1": "456 Shipping Ave",
						"street2": null,
						"city": "Brooklyn",
						"state": "NY",
						"zipCode": "11201",
						"country": "US",
						"isDefault": false
					}
				]
			},
			"items": [
				{
					"sku": "LAPTOP-001",
					"name": "Gaming Laptop Pro",
					"category": "Electronics",
					"price": 1299.99,
					"quantity": 1,
					"discount": {
						"type": "percentage",
						"value": 10,
						"amount": 129.99
					},
					"attributes": [
						{"name": "Color", "value": "Black"},
						{"name": "RAM", "value": "16GB"},
						{"name": "Storage", "value": "512GB SSD"}
					]
				},
				{
					"sku": "MOUSE-002",
					"name": "Wireless Gaming Mouse",
					"category": "Electronics",
					"price": 79.99,
					"quantity": 2,
					"discount": null,
					"attributes": [
						{"name": "Color", "value": "RGB"},
						{"name": "DPI", "value": "16000"}
					]
				}
			],
			"totals": {
				"subtotal": 1459.97,
				"discounts": -129.99,
				"tax": {
					"rate": 0.0875,
					"amount": 116.37
				},
				"shipping": {
					"method": "standard",
					"cost": 9.99,
					"estimated": "2024-01-20"
				},
				"total": 1456.34
			},
			"payments": [
				{
					"method": "credit_card",
					"provider": "visa",
					"last4": "1234",
					"amount": 1456.34,
					"status": "authorized",
					"transactionId": "TXN-987654321"
				}
			],
			"tracking": {
				"carrier": "UPS",
				"trackingNumber": "1Z999AA1012345675",
				"status": "processing",
				"events": [
					{
						"timestamp": "2024-01-15T10:00:00Z",
						"status": "order_received",
						"description": "Order received and processing"
					},
					{
						"timestamp": "2024-01-15T14:30:00Z",
						"status": "payment_confirmed",
						"description": "Payment authorization successful"
					}
				]
			}
		}
	}`

	var jsonData interface{}
	err := json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		t.Fatalf("Failed to parse e-commerce JSON: %v", err)
	}

	xmlBytes, err := p.ConvertJSONToXML(jsonData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	xmlStr := string(xmlBytes)
	t.Logf("E-commerce XML:\n%s", xmlStr)

	// Verify order structure
	if !strings.Contains(xmlStr, "<orderId>ORD-2024-001234</orderId>") {
		t.Errorf("Expected order ID")
	}

	// Verify customer addresses array
	addressCount := strings.Count(xmlStr, "<addresses>")
	if addressCount != 2 {
		t.Errorf("Expected 2 address elements, got %d", addressCount)
	}

	// Verify items array with attributes
	if !strings.Contains(xmlStr, "<items>") {
		t.Errorf("Expected items elements")
	}
	if !strings.Contains(xmlStr, "<attributes>") {
		t.Errorf("Expected attributes elements")
	}

	// Verify price handling
	if !strings.Contains(xmlStr, "<price>1299.99</price>") {
		t.Errorf("Expected price element")
	}

	// Verify tracking events array
	if !strings.Contains(xmlStr, "<events>") {
		t.Errorf("Expected events elements")
	}
	if !strings.Contains(xmlStr, "<status>order_received</status>") {
		t.Errorf("Expected status in events")
	}
}

func TestJSONToXMLPolicy_EdgeCasesAndLimits(t *testing.T) {
	p := &JSONToXMLPolicy{}

	tests := []struct {
		name     string
		jsonStr  string
		validate func(t *testing.T, xmlStr string)
	}{
		{
			name:    "Empty arrays and objects",
			jsonStr: `{"empty_object": {}, "empty_array": [], "nested": {"inner_empty": {}}}`,
			validate: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "<empty_object>") {
					t.Errorf("Expected empty_object element")
				}
			},
		},
		{
			name:    "Very large numbers",
			jsonStr: `{"big_int": 9223372036854775807, "big_float": 1.7976931348623157e+308, "small_float": 2.2250738585072014e-308}`,
			validate: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "<big_int>") {
					t.Errorf("Expected big_int element")
				}
			},
		},
		{
			name:    "Deeply nested arrays",
			jsonStr: `{"level1": [{"level2": [{"level3": [{"level4": ["deep_value"]}]}]}]}`,
			validate: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "deep_value") {
					t.Errorf("Expected deep nested value")
				}
			},
		},
		{
			name:    "Mixed null and valid values",
			jsonStr: `{"data": [null, "valid", null, 42, null, true, null]}`,
			validate: func(t *testing.T, xmlStr string) {
				if !strings.Contains(xmlStr, "<data></data>") {
					t.Errorf("Expected empty data elements for null")
				}
				if !strings.Contains(xmlStr, "<data>valid</data>") {
					t.Errorf("Expected valid data element")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var jsonData interface{}
			err := json.Unmarshal([]byte(test.jsonStr), &jsonData)
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			xmlBytes, err := p.ConvertJSONToXML(jsonData)
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			xmlStr := string(xmlBytes)
			t.Logf("%s XML:\n%s", test.name, xmlStr)
			test.validate(t, xmlStr)
		})
	}
}
