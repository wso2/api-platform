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
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// XMLToJSONPolicy implements transforming a request/response with a XML payload to a request/response with a JSON payload
type XMLToJSONPolicy struct{}

var ins = &XMLToJSONPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *XMLToJSONPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers for content type
		RequestBodyMode:    policy.BodyModeBuffer,    // Need request body for transformation
		ResponseHeaderMode: policy.HeaderModeProcess, // Process response headers for content type
		ResponseBodyMode:   policy.BodyModeBuffer,    // Need response body for transformation
	}
}

// OnRequest transforms XML request body to JSON
func (p *XMLToJSONPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
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

	// Check content type to ensure it is XML
	contentType := ""
	if contentTypeHeaders := ctx.Headers.Get("content-type"); len(contentTypeHeaders) > 0 {
		contentType = strings.ToLower(contentTypeHeaders[0])
	}

	if !strings.Contains(contentType, "application/xml") && !strings.Contains(contentType, "text/xml") {
		return p.handleInternalServerError("Content-Type must be application/xml or text/xml for XML to JSON transformation")
	}

	// Parse XML and convert to JSON
	jsonData, err := p.convertXMLToJSON(ctx.Body.Content)
	if err != nil {
		return p.handleInternalServerError("Failed to convert XML to JSON format: " + err.Error())
	}

	// Return modified request with JSON body and updated content type
	return policy.UpstreamRequestModifications{
		Body: jsonData,
		SetHeaders: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(jsonData)),
		},
	}
}

// OnResponse transforms XML response body to JSON
func (p *XMLToJSONPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
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

	// Check content type to ensure it is XML
	contentType := ""
	if contentTypeHeaders := ctx.ResponseHeaders.Get("content-type"); len(contentTypeHeaders) > 0 {
		contentType = strings.ToLower(contentTypeHeaders[0])
	}

	if !strings.Contains(contentType, "application/xml") && !strings.Contains(contentType, "text/xml") {
		return p.handleInternalServerErrorResponse("Content-Type must be application/xml or text/xml for XML to JSON transformation")
	}

	// Parse XML and convert to JSON
	jsonData, err := p.convertXMLToJSON(ctx.ResponseBody.Content)
	if err != nil {
		return p.handleInternalServerErrorResponse("Failed to convert XML to JSON format: " + err.Error())
	}

	// Return modified response with JSON body and updated content type
	return policy.UpstreamResponseModifications{
		Body: jsonData,
		SetHeaders: map[string]string{
			"content-type":   "application/json",
			"content-length": fmt.Sprintf("%d", len(jsonData)),
		},
	}
}

// handleInternalServerError returns a 500 internal server error response for request flow
func (p *XMLToJSONPolicy) handleInternalServerError(message string) policy.RequestAction {
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
func (p *XMLToJSONPolicy) handleInternalServerErrorResponse(message string) policy.ResponseAction {
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

// ConvertXMLToJSON converts XML data to JSON format (public for testing)
func (p *XMLToJSONPolicy) ConvertXMLToJSON(xmlData []byte) ([]byte, error) {
	return p.convertXMLToJSON(xmlData)
}

// convertXMLToJSON converts XML data to JSON format
func (p *XMLToJSONPolicy) convertXMLToJSON(xmlData []byte) ([]byte, error) {
	// Parse XML into XMLNode structure
	var node XMLNode
	err := xml.Unmarshal(xmlData, &node)
	if err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	// Convert XMLNode to JSON-friendly structure
	jsonData := p.nodeToMap(node)

	// Marshal to JSON with proper formatting
	result, err := json.MarshalIndent(jsonData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return result, nil
}

// nodeToMap converts XMLNode to a JSON-friendly map structure
func (p *XMLToJSONPolicy) nodeToMap(node XMLNode) interface{} {
	// Start with the root element name
	result := make(map[string]interface{})

	// Process the node
	nodeData := p.processXMLNode(node)
	result[node.XMLName.Local] = nodeData

	return result
}

// processXMLNode processes a single XML node and returns its JSON representation
func (p *XMLToJSONPolicy) processXMLNode(node XMLNode) interface{} {
	// If node has no children and no attributes, return the content
	if len(node.Nodes) == 0 && len(node.Attrs) == 0 {
		content := strings.TrimSpace(node.Content)
		if content == "" {
			return nil
		}
		return p.parseValue(content)
	}

	// Create a map to hold the result
	result := make(map[string]interface{})

	// Add attributes with @ prefix
	for _, attr := range node.Attrs {
		result["@"+attr.Name.Local] = p.parseAttributeValue(attr.Value)
	}

	// Group child nodes by name to handle arrays
	childGroups := make(map[string][]XMLNode)

	for _, child := range node.Nodes {
		name := child.XMLName.Local
		childGroups[name] = append(childGroups[name], child)
	}

	// Process child groups
	for name, children := range childGroups {
		if len(children) == 1 {
			// Single child - add directly
			result[name] = p.processXMLNode(children[0])
		} else {
			// Multiple children with same name - create array
			array := make([]interface{}, len(children))
			for i, child := range children {
				array[i] = p.processXMLNode(child)
			}
			result[name] = array
		}
	}

	// Add text content if present and we have other content
	content := strings.TrimSpace(node.Content)
	if content != "" && len(result) > 0 {
		result["#text"] = p.parseValue(content)
	} else if content != "" && len(result) == 0 {
		// If only text content, return it directly
		return p.parseValue(content)
	}

	// If result is empty, return null
	if len(result) == 0 {
		return nil
	}

	return result
}

// parseAttributeValue is more conservative for attribute values, preserving strings unless clearly boolean or decimal
func (p *XMLToJSONPolicy) parseAttributeValue(value string) interface{} {
	value = strings.TrimSpace(value)

	if value == "" {
		return ""
	}

	// Only parse as boolean if it's exactly "true" or "false"
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Only parse as float if it contains a decimal point
	if strings.Contains(value, ".") {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}

	// For attributes, preserve as string to avoid converting IDs, codes, etc.
	return value
}

// parseValue attempts to parse a string value into appropriate JSON types
func (p *XMLToJSONPolicy) parseValue(value string) interface{} {
	value = strings.TrimSpace(value)

	if value == "" {
		return ""
	}

	// Try to parse as boolean
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Try to parse as number only if it looks like a pure number
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		// Only convert if it contains a decimal point
		if strings.Contains(value, ".") {
			return floatVal
		}
	}

	// Return as string
	return value
}

// XMLNode represents a generic XML node for parsing
type XMLNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
	Nodes   []XMLNode  `xml:",any"`
}
