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

package xmltojson

import (
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// Helper function to create test headers
func createTestHeaders(key, value string) *policy.Headers {
	headers := make(map[string][]string)
	headers[key] = []string{value}
	return policy.NewHeaders(headers)
}

func TestXMLToJSONPolicy_Mode(t *testing.T) {
	p := &XMLToJSONPolicy{}
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

func TestXMLToJSONPolicy_OnRequest_DisabledByDefault(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`<root><name>test</name></root>`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/xml"),
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

func TestXMLToJSONPolicy_OnRequest_EnabledWithParameter(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`<root><name>John Doe</name><age>30</age></root>`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/xml"),
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

	// Parse the JSON result to verify structure
	var jsonResult map[string]interface{}
	if err := json.Unmarshal(mods.Body, &jsonResult); err != nil {
		t.Fatalf("Failed to parse transformed JSON: %v", err)
	}

	// Check that root element exists
	if root, ok := jsonResult["root"]; !ok {
		t.Errorf("Expected 'root' element in JSON result, got: %v", jsonResult)
	} else if rootMap, ok := root.(map[string]interface{}); !ok {
		t.Errorf("Expected root to be an object, got: %T", root)
	} else {
		if rootMap["name"] != "John Doe" {
			t.Errorf("Expected name to be 'John Doe', got: %v", rootMap["name"])
		}
		if rootMap["age"] != float64(30) {
			t.Errorf("Expected age to be 30, got: %v", rootMap["age"])
		}
	}

	// Check headers were updated
	if mods.SetHeaders["content-type"] != "application/json" {
		t.Errorf("Expected content-type to be application/json, got: %s", mods.SetHeaders["content-type"])
	}
}

func TestXMLToJSONPolicy_OnRequest_WrongContentType(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`<root><name>test</name></root>`),
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return internal server error
	if resp, ok := result.(policy.ImmediateResponse); !ok {
		t.Errorf("Expected ImmediateResponse for wrong content type, got %T", result)
	} else {
		if resp.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", resp.StatusCode)
		}
	}
}

func TestXMLToJSONPolicy_OnRequest_EmptyBody(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte{},
			Present: false,
		},
		Headers: createTestHeaders("content-type", "application/xml"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return empty modifications (no transformation)
	mods, ok := result.(policy.UpstreamRequestModifications)
	if !ok {
		t.Errorf("Expected UpstreamRequestModifications for empty body, got %T", result)
	}

	if mods.Body != nil {
		t.Errorf("Expected no body modification for empty body, got body: %s", string(mods.Body))
	}
}

func TestXMLToJSONPolicy_OnRequest_InvalidXML(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.RequestContext{
		Body: &policy.Body{
			Content: []byte(`<root><name>test</name>`), // Missing closing tag
			Present: true,
		},
		Headers: createTestHeaders("content-type", "application/xml"),
	}

	params := map[string]interface{}{
		"onRequestFlow": true,
	}

	result := p.OnRequest(ctx, params)

	// Should return internal server error
	if resp, ok := result.(policy.ImmediateResponse); !ok {
		t.Errorf("Expected ImmediateResponse for invalid XML, got %T", result)
	} else {
		if resp.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", resp.StatusCode)
		}
	}
}

func TestXMLToJSONPolicy_OnResponse_DisabledByDefault(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`<root><name>test</name></root>`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/xml"),
	}

	// No parameters - should be disabled by default
	result := p.OnResponse(ctx, map[string]interface{}{})

	// Should return empty modifications (no transformation)
	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	if mods.Body != nil {
		t.Errorf("Expected no body modification when disabled, got body: %s", string(mods.Body))
	}
}

func TestXMLToJSONPolicy_OnResponse_EnabledWithParameter(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`<users><user><id>1</id><name>Alice</name></user><user><id>2</id><name>Bob</name></user></users>`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/xml"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications, got %T", result)
	}

	// Check body was transformed
	if mods.Body == nil {
		t.Fatal("Expected body to be transformed, got nil")
	}

	// Parse the JSON result to verify structure
	var jsonResult map[string]interface{}
	if err := json.Unmarshal(mods.Body, &jsonResult); err != nil {
		t.Fatalf("Failed to parse transformed JSON: %v", err)
	}

	// Check that users element exists and contains array
	if users, ok := jsonResult["users"]; !ok {
		t.Errorf("Expected 'users' element in JSON result, got: %v", jsonResult)
	} else if usersMap, ok := users.(map[string]interface{}); !ok {
		t.Errorf("Expected users to be an object, got: %T", users)
	} else if userArray, ok := usersMap["user"].([]interface{}); !ok {
		t.Errorf("Expected users.user to be an array, got: %T", usersMap["user"])
	} else {
		if len(userArray) != 2 {
			t.Errorf("Expected 2 users in array, got %d", len(userArray))
		}
	}

	// Check headers were updated
	if mods.SetHeaders["content-type"] != "application/json" {
		t.Errorf("Expected content-type to be application/json, got: %s", mods.SetHeaders["content-type"])
	}
}

func TestXMLToJSONPolicy_OnResponse_WrongContentType(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`<root><name>test</name></root>`),
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/json"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	// Should return 500 error for wrong content type in response
	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications for wrong content type in response, got %T", result)
	}

	if mods.StatusCode == nil || *mods.StatusCode != 500 {
		t.Errorf("Expected status code 500 for wrong content type, got: %v", mods.StatusCode)
	}
}

func TestXMLToJSONPolicy_OnResponse_InvalidXML(t *testing.T) {
	p := &XMLToJSONPolicy{}
	ctx := &policy.ResponseContext{
		ResponseBody: &policy.Body{
			Content: []byte(`<root><name>test</name>`), // Missing closing tag
			Present: true,
		},
		ResponseHeaders: createTestHeaders("content-type", "application/xml"),
	}

	params := map[string]interface{}{
		"onResponseFlow": true,
	}

	result := p.OnResponse(ctx, params)

	// Should return 500 error for invalid XML in response
	mods, ok := result.(policy.UpstreamResponseModifications)
	if !ok {
		t.Errorf("Expected UpstreamResponseModifications for invalid XML in response, got %T", result)
	}

	if mods.StatusCode == nil || *mods.StatusCode != 500 {
		t.Errorf("Expected status code 500 for invalid XML, got: %v", mods.StatusCode)
	}
}

func TestXMLToJSONPolicy_ConvertXMLToJSON_SimpleObject(t *testing.T) {
	p := &XMLToJSONPolicy{}
	xmlData := []byte(`<person><name>John</name><age>30</age></person>`)

	result, err := p.ConvertXMLToJSON(xmlData)
	if err != nil {
		t.Fatalf("ConvertXMLToJSON failed: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(result, &jsonResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	person, ok := jsonResult["person"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected person to be an object, got: %T", jsonResult["person"])
	}

	if person["name"] != "John" {
		t.Errorf("Expected name to be 'John', got: %v", person["name"])
	}

	if person["age"] != float64(30) {
		t.Errorf("Expected age to be 30, got: %v", person["age"])
	}
}

func TestXMLToJSONPolicy_ConvertXMLToJSON_WithAttributes(t *testing.T) {
	p := &XMLToJSONPolicy{}
	xmlData := []byte(`<book id="123" isbn="978-0123456789"><title>Go Programming</title><author>John Doe</author></book>`)

	result, err := p.ConvertXMLToJSON(xmlData)
	if err != nil {
		t.Fatalf("ConvertXMLToJSON failed: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(result, &jsonResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	book, ok := jsonResult["book"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected book to be an object, got: %T", jsonResult["book"])
	}

	if book["@id"] != "123" {
		t.Errorf("Expected @id to be '123', got: %v", book["@id"])
	}

	if book["@isbn"] != "978-0123456789" {
		t.Errorf("Expected @isbn to be '978-0123456789', got: %v", book["@isbn"])
	}

	if book["title"] != "Go Programming" {
		t.Errorf("Expected title to be 'Go Programming', got: %v", book["title"])
	}
}

func TestXMLToJSONPolicy_ConvertXMLToJSON_Array(t *testing.T) {
	p := &XMLToJSONPolicy{}
	xmlData := []byte(`<users><user><id>1</id><name>Alice</name></user><user><id>2</id><name>Bob</name></user></users>`)

	result, err := p.ConvertXMLToJSON(xmlData)
	if err != nil {
		t.Fatalf("ConvertXMLToJSON failed: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(result, &jsonResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	users, ok := jsonResult["users"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected users to be an object, got: %T", jsonResult["users"])
	}

	userArray, ok := users["user"].([]interface{})
	if !ok {
		t.Fatalf("Expected user to be an array, got: %T", users["user"])
	}

	if len(userArray) != 2 {
		t.Errorf("Expected 2 users, got %d", len(userArray))
	}

	// Check first user
	user1, ok := userArray[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected first user to be an object, got: %T", userArray[0])
	}

	if user1["id"] != float64(1) {
		t.Errorf("Expected first user id to be 1, got: %v", user1["id"])
	}

	if user1["name"] != "Alice" {
		t.Errorf("Expected first user name to be 'Alice', got: %v", user1["name"])
	}
}

func TestXMLToJSONPolicy_ConvertXMLToJSON_EmptyElement(t *testing.T) {
	p := &XMLToJSONPolicy{}
	xmlData := []byte(`<root><empty></empty><selfclosed/></root>`)

	result, err := p.ConvertXMLToJSON(xmlData)
	if err != nil {
		t.Fatalf("ConvertXMLToJSON failed: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(result, &jsonResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	root, ok := jsonResult["root"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected root to be an object, got: %T", jsonResult["root"])
	}

	// Empty elements should be null
	if root["empty"] != nil {
		t.Errorf("Expected empty element to be null, got: %v", root["empty"])
	}

	if root["selfclosed"] != nil {
		t.Errorf("Expected selfclosed element to be null, got: %v", root["selfclosed"])
	}
}

func TestXMLToJSONPolicy_ConvertXMLToJSON_BooleanAndNumbers(t *testing.T) {
	p := &XMLToJSONPolicy{}
	xmlData := []byte(`<data><active>true</active><inactive>false</inactive><count>42</count><price>19.99</price></data>`)

	result, err := p.ConvertXMLToJSON(xmlData)
	if err != nil {
		t.Fatalf("ConvertXMLToJSON failed: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(result, &jsonResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	data, ok := jsonResult["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be an object, got: %T", jsonResult["data"])
	}

	if data["active"] != true {
		t.Errorf("Expected active to be true, got: %v", data["active"])
	}

	if data["inactive"] != false {
		t.Errorf("Expected inactive to be false, got: %v", data["inactive"])
	}

	if data["count"] != float64(42) {
		t.Errorf("Expected count to be 42, got: %v", data["count"])
	}

	if data["price"] != 19.99 {
		t.Errorf("Expected price to be 19.99, got: %v", data["price"])
	}
}

func TestXMLToJSONPolicy_ConvertXMLToJSON_TextContentValue(t *testing.T) {
	p := &XMLToJSONPolicy{}
	xmlData := []byte(`<message>Hello World</message>`)

	result, err := p.ConvertXMLToJSON(xmlData)
	if err != nil {
		t.Fatalf("ConvertXMLToJSON failed: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal(result, &jsonResult); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if jsonResult["message"] != "Hello World" {
		t.Errorf("Expected message to be 'Hello World', got: %v", jsonResult["message"])
	}
}

func TestGetPolicy(t *testing.T) {
	metadata := policy.PolicyMetadata{}
	params := map[string]interface{}{}

	policyInstance, err := GetPolicy(metadata, params)
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}

	if _, ok := policyInstance.(*XMLToJSONPolicy); !ok {
		t.Errorf("Expected *XMLToJSONPolicy, got %T", policyInstance)
	}
}
