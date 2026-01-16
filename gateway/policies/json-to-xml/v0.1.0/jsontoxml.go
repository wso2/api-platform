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
	"encoding/xml"
	"fmt"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// JSONToXMLPolicy implements transforming a request/response with a JSON payload to a request/response with an XML payload
type JSONToXMLPolicy struct{}

var ins = &JSONToXMLPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *JSONToXMLPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers for content type
		RequestBodyMode:    policy.BodyModeBuffer,    // Need request body for transformation
		ResponseHeaderMode: policy.HeaderModeProcess, // Process response headers for content type
		ResponseBodyMode:   policy.BodyModeBuffer,    // Need response body for transformation
	}
}

// OnRequest transforms JSON request body to XML
func (p *JSONToXMLPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check onRequestFlow parameter to determine if request transformation should be applied
	onRequestFlow := false // default value
	if onRequestFlowParam, ok := params["onRequestFlow"].(bool); ok {
		onRequestFlow = onRequestFlowParam
	}

	// Skip transformation if onRequestFlow is false
	if !onRequestFlow {
		return policy.UpstreamRequestModifications{}
	}

	if ctx.Body == nil || !ctx.Body.Present || len(ctx.Body.Content) == 0 {
		return policy.UpstreamRequestModifications{}
	}

	// Check content type to ensure it is JSON
	contentType := ""
	if contentTypeHeaders := ctx.Headers.Get("content-type"); len(contentTypeHeaders) > 0 {
		contentType = strings.ToLower(contentTypeHeaders[0])
	}

	if !strings.Contains(contentType, "application/json") {
		return p.handleInternalServerError("Content-Type must be application/json for JSON to XML transformation")
	}

	// Parse JSON
	var jsonData interface{}
	if err := json.Unmarshal(ctx.Body.Content, &jsonData); err != nil {
		return p.handleInternalServerError("Invalid JSON format in request body")
	}

	// Convert to XML
	xmlData, err := p.convertJSONToXML(jsonData)
	if err != nil {
		return p.handleInternalServerError("Failed to convert JSON to XML format")
	}

	// Return modified request with XML body and updated content type
	return policy.UpstreamRequestModifications{
		Body: xmlData,
		SetHeaders: map[string]string{
			"content-type":   "application/xml",
			"content-length": fmt.Sprintf("%d", len(xmlData)),
		},
	}
}

// OnResponse transforms JSON response body to XML
func (p *JSONToXMLPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Check onResponseFlow parameter to determine if response transformation should be applied
	onResponseFlow := false // default value
	if onResponseFlowParam, ok := params["onResponseFlow"].(bool); ok {
		onResponseFlow = onResponseFlowParam
	}

	// Skip transformation if onResponseFlow is false
	if !onResponseFlow {
		return policy.UpstreamResponseModifications{}
	}

	if ctx.ResponseBody == nil || !ctx.ResponseBody.Present || len(ctx.ResponseBody.Content) == 0 {
		return policy.UpstreamResponseModifications{}
	}

	// Check content type to ensure it is JSON
	contentType := ""
	if contentTypeHeaders := ctx.ResponseHeaders.Get("content-type"); len(contentTypeHeaders) > 0 {
		contentType = strings.ToLower(contentTypeHeaders[0])
	}

	if !strings.Contains(contentType, "application/json") {
		return p.handleInternalServerErrorResponse("Content-Type must be application/json for JSON to XML transformation")
	}

	// Parse JSON
	var jsonData interface{}
	if err := json.Unmarshal(ctx.ResponseBody.Content, &jsonData); err != nil {
		return p.handleInternalServerErrorResponse("Invalid JSON format in response body")
	}

	// Convert to XML
	xmlData, err := p.convertJSONToXML(jsonData)
	if err != nil {
		return p.handleInternalServerErrorResponse("Failed to convert JSON to XML format")
	}

	// Return modified response with XML body and updated content type
	return policy.UpstreamResponseModifications{
		Body: xmlData,
		SetHeaders: map[string]string{
			"content-type":   "application/xml",
			"content-length": fmt.Sprintf("%d", len(xmlData)),
		},
	}
}

// handleInternalServerError returns a 500 internal server error response for request flow
func (p *JSONToXMLPolicy) handleInternalServerError(message string) policy.RequestAction {
	errorResponse := map[string]interface{}{
		"error":   "Internal Server Error",
		"message": message,
	}
	bodyBytes, _ := json.Marshal(errorResponse)

	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(bodyBytes)),
		},
		Body: bodyBytes,
	}
}

// handleInternalServerErrorResponse returns a 500 internal server error response for response flow
func (p *JSONToXMLPolicy) handleInternalServerErrorResponse(message string) policy.ResponseAction {
	errorResponse := map[string]interface{}{
		"error":   "Internal Server Error",
		"message": message,
	}
	bodyBytes, _ := json.Marshal(errorResponse)

	return policy.UpstreamResponseModifications{
		StatusCode: &[]int{500}[0],
		Body:       bodyBytes,
		SetHeaders: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(bodyBytes)),
		},
	}
}

// ConvertJSONToXML converts JSON data to XML format (public for testing)
func (p *JSONToXMLPolicy) ConvertJSONToXML(jsonData interface{}) ([]byte, error) {
	return p.convertJSONToXML(jsonData)
}

// convertJSONToXML converts JSON data to XML format
func (p *JSONToXMLPolicy) convertJSONToXML(jsonData interface{}) ([]byte, error) {
	// Convert JSON data to a map structure that can be marshaled to XML
	xmlStruct := p.buildXMLStruct(jsonData, "root")

	// Marshal to XML with proper formatting (no XML declaration)
	xmlData, err := xml.MarshalIndent(xmlStruct, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to XML: %w", err)
	}

	// Return XML without declaration
	return xmlData, nil
}

// buildXMLStruct recursively builds an XML-compatible structure from JSON data
func (p *JSONToXMLPolicy) buildXMLStruct(data interface{}, tagName string) XMLElement {
	element := XMLElement{XMLName: xml.Name{Local: tagName}}

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if arr, isArray := value.([]interface{}); isArray {
				// For arrays, add each item as a direct child with the array key name
				for _, item := range arr {
					childElement := p.buildXMLStruct(item, key)
					element.Children = append(element.Children, childElement)
				}
			} else {
				// For non-arrays, process normally
				childElement := p.buildXMLStruct(value, key)
				element.Children = append(element.Children, childElement)
			}
		}
	case []interface{}:
		// This handles root-level arrays
		for _, item := range v {
			itemTagName := tagName
			if tagName == "root" {
				// For root arrays, use indexed names
				itemTagName = fmt.Sprintf("item%d", len(element.Children))
			}
			childElement := p.buildXMLStruct(item, itemTagName)
			element.Children = append(element.Children, childElement)
		}
	case string:
		element.Content = v
	case float64:
		element.Content = fmt.Sprintf("%g", v)
	case bool:
		element.Content = fmt.Sprintf("%t", v)
	case nil:
		element.Content = ""
	default:
		// Handle other types by converting to string
		element.Content = fmt.Sprintf("%v", v)
	}

	return element
}

// XMLElement represents a generic XML element for marshaling
type XMLElement struct {
	XMLName  xml.Name     `xml:""`
	Content  string       `xml:",chardata"`
	Children []XMLElement `xml:",any"`
}
