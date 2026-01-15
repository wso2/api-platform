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

package modelroundrobin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

const (
	// Metadata keys for context storage
	MetadataKeySelectedModel = "model_roundrobin.selected_model"
	MetadataKeyOriginalModel = "model_roundrobin.original_model"
)

// ModelRoundRobinPolicyParams holds the parsed policy parameters
type ModelRoundRobinPolicyParams struct {
	Models          []ModelConfig
	SuspendDuration int
	RequestModel    RequestModelConfig
}

// ModelConfig represents a single model configuration
type ModelConfig struct {
	Model string
}

// RequestModelConfig holds the requestModel configuration
type RequestModelConfig struct {
	Location   string
	Identifier string
}

// ModelRoundRobinPolicy implements round-robin load balancing for AI models
type ModelRoundRobinPolicy struct {
	currentIndex    int
	mu              sync.Mutex
	suspendedModels map[string]time.Time // Track suspended models
	params          ModelRoundRobinPolicyParams
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	// Parse and validate parameters
	policyParams, err := parseParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	p := &ModelRoundRobinPolicy{
		currentIndex:    0,
		suspendedModels: make(map[string]time.Time),
		params:          policyParams,
	}

	return p, nil
}

// parseParams parses and validates parameters from map to struct
func parseParams(params map[string]interface{}) (ModelRoundRobinPolicyParams, error) {
	var result ModelRoundRobinPolicyParams

	// Parse models parameter (required)
	modelsRaw, ok := params["models"]
	if !ok {
		return result, fmt.Errorf("'models' parameter is required")
	}

	modelList, ok := modelsRaw.([]interface{})
	if !ok {
		return result, fmt.Errorf("'models' must be an array")
	}

	if len(modelList) == 0 {
		return result, fmt.Errorf("'models' array must contain at least one model")
	}

	// Parse each model in the array
	result.Models = make([]ModelConfig, 0, len(modelList))
	for i, item := range modelList {
		modelMap, ok := item.(map[string]interface{})
		if !ok {
			return result, fmt.Errorf("'models[%d]' must be an object", i)
		}

		var modelConfig ModelConfig

		// Parse model name (required)
		modelName, ok := modelMap["model"]
		if !ok {
			return result, fmt.Errorf("'models[%d].model' is required", i)
		}

		modelNameStr, ok := modelName.(string)
		if !ok {
			return result, fmt.Errorf("'models[%d].model' must be a string", i)
		}

		if len(modelNameStr) == 0 {
			return result, fmt.Errorf("'models[%d].model' must have a minimum length of 1", i)
		}
		modelConfig.Model = modelNameStr

		result.Models = append(result.Models, modelConfig)
	}

	// Parse suspendDuration if provided (optional)
	if suspendDuration, ok := params["suspendDuration"]; ok {
		suspendDurationInt, err := extractInt(suspendDuration)
		if err != nil {
			return result, fmt.Errorf("'suspendDuration' must be an integer: %w", err)
		}

		if suspendDurationInt < 0 {
			return result, fmt.Errorf("'suspendDuration' must be >= 0")
		}
		result.SuspendDuration = suspendDurationInt
	}

	// Parse requestModel configuration (required, comes from systemParameters)
	requestModel, ok := params["requestModel"]
	if !ok {
		return result, fmt.Errorf("'requestModel' configuration is required")
	}

	requestModelMap, ok := requestModel.(map[string]interface{})
	if !ok {
		return result, fmt.Errorf("'requestModel' must be an object")
	}

	// Parse location (required)
	location, ok := requestModelMap["location"]
	if !ok {
		return result, fmt.Errorf("'requestModel.location' is required")
	}

	locationStr, ok := location.(string)
	if !ok {
		return result, fmt.Errorf("'requestModel.location' must be a string")
	}

	// Validate location value
	validLocations := map[string]bool{
		"payload":    true,
		"header":     true,
		"queryParam": true,
		"pathParam":  true,
	}
	if !validLocations[locationStr] {
		return result, fmt.Errorf("'requestModel.location' must be one of: payload, header, queryParam, pathParam")
	}
	result.RequestModel.Location = locationStr

	// Parse identifier (required)
	identifier, ok := requestModelMap["identifier"]
	if !ok {
		return result, fmt.Errorf("'requestModel.identifier' is required")
	}

	identifierStr, ok := identifier.(string)
	if !ok {
		return result, fmt.Errorf("'requestModel.identifier' must be a string")
	}

	if len(identifierStr) == 0 {
		return result, fmt.Errorf("'requestModel.identifier' must have a minimum length of 1")
	}
	result.RequestModel.Identifier = identifierStr

	return result, nil
}

// extractInt safely extracts an integer from various types
func extractInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("expected an integer but got %v", v)
		}
		return int(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// Mode returns the processing mode for this policy
func (p *ModelRoundRobinPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest processes the request and selects the next model in round-robin order
func (p *ModelRoundRobinPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Extract original model from request
	originalModel, err := p.extractModelFromRequest(ctx)
	if err != nil {
		slog.Debug("ModelRoundRobin: Could not extract original model", "error", err)
	}

	// Store original model in metadata
	if originalModel != "" {
		ctx.Metadata[MetadataKeyOriginalModel] = originalModel
	}

	// Select next available model in round-robin fashion
	selectedModel := p.selectNextAvailableModel(p.params.Models)

	if selectedModel == nil {
		return policy.ImmediateResponse{
			StatusCode: 503,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"error": "All models are currently unavailable"}`),
		}
	}

	ctx.Metadata[MetadataKeySelectedModel] = selectedModel.Model

	slog.Debug("ModelRoundRobin: Selected model", "model", selectedModel.Model, "index", p.currentIndex)

	return p.modifyRequestModel(ctx, selectedModel.Model)
}

// OnResponse handles response processing and suspension on error
func (p *ModelRoundRobinPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Check if response indicates an error that should trigger suspension
	if ctx.ResponseStatus >= 500 || ctx.ResponseStatus == 429 {
		selectedModel := ""
		if model, ok := ctx.Metadata[MetadataKeySelectedModel]; ok {
			if modelStr, ok := model.(string); ok {
				selectedModel = modelStr
			}
		}

		if p.params.SuspendDuration > 0 && selectedModel != "" {
			// Suspend this model
			p.mu.Lock()
			p.suspendedModels[selectedModel] = time.Now().Add(time.Duration(p.params.SuspendDuration) * time.Second)
			p.mu.Unlock()
			slog.Debug("ModelRoundRobin: Suspended model", "model", selectedModel, "duration", p.params.SuspendDuration)
		}
	}

	return policy.UpstreamResponseModifications{}
}

// selectNextAvailableModel selects the next available model in round-robin fashion
func (p *ModelRoundRobinPolicy) selectNextAvailableModel(models []ModelConfig) *ModelConfig {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	attemptCount := 0
	totalModels := len(models)

	for attemptCount < totalModels {
		// Get current model (copy to avoid returning pointer to slice element)
		selectedModel := models[p.currentIndex]
		modelName := selectedModel.Model

		// Move to next index for next call
		p.currentIndex = (p.currentIndex + 1) % totalModels

		// Check if model is suspended
		if suspendedUntil, ok := p.suspendedModels[modelName]; ok {
			if now.Before(suspendedUntil) {
				// This model is still suspended, try next
				attemptCount++
				continue
			}
			// Suspension period has expired, remove from suspended list
			delete(p.suspendedModels, modelName)
		}

		return &selectedModel
	}

	return nil
}

// extractModelFromRequest extracts the model identifier from the request using requestModel config
// Note: requestModel is validated in GetPolicy, so it will always be present
func (p *ModelRoundRobinPolicy) extractModelFromRequest(ctx *policy.RequestContext) (string, error) {
	location := p.params.RequestModel.Location
	identifier := p.params.RequestModel.Identifier

	switch location {
	case "payload":
		return p.extractModelFromBody(ctx, identifier)
	case "header":
		if ctx.Headers != nil {
			values := ctx.Headers.Get(identifier)
			if len(values) > 0 && values[0] != "" {
				return values[0], nil
			}
		}
		return "", fmt.Errorf("header %s not found", identifier)
	case "queryParam":
		return p.extractModelFromQuery(ctx, identifier)
	case "pathParam":
		return p.extractModelFromPath(ctx, identifier)
	default:
		return "", fmt.Errorf("unsupported location: %s", location)
	}
}

// extractModelFromBody extracts model from request body using JSONPath
func (p *ModelRoundRobinPolicy) extractModelFromBody(ctx *policy.RequestContext, jsonPath string) (string, error) {
	if ctx.Body == nil || ctx.Body.Content == nil {
		return "", fmt.Errorf("request body is empty")
	}

	value, err := utils.ExtractStringValueFromJsonpath(ctx.Body.Content, jsonPath)
	if err != nil {
		return "", fmt.Errorf("failed to extract model from JSONPath %s: %w", jsonPath, err)
	}

	return value, nil
}

// extractModelFromQuery extracts model from query parameter
func (p *ModelRoundRobinPolicy) extractModelFromQuery(ctx *policy.RequestContext, paramName string) (string, error) {
	if ctx.Path == "" {
		return "", fmt.Errorf("request path is empty")
	}

	// Parse the URL-encoded path
	decodedPath, err := url.PathUnescape(ctx.Path)
	if err != nil {
		return "", fmt.Errorf("failed to decode path: %w", err)
	}

	// Split the path into components
	parts := strings.Split(decodedPath, "?")
	if len(parts) != 2 {
		return "", fmt.Errorf("query parameter %s not found in path", paramName)
	}

	// Parse the query string
	queryString := parts[1]
	values, err := url.ParseQuery(queryString)
	if err != nil {
		return "", fmt.Errorf("failed to parse query string: %w", err)
	}

	// Get the first value of the specified parameter
	if value, ok := values[paramName]; ok && len(value) > 0 && value[0] != "" {
		return value[0], nil
	}

	return "", fmt.Errorf("query parameter %s not found", paramName)
}

// extractModelFromPath extracts model from path using regex pattern
func (p *ModelRoundRobinPolicy) extractModelFromPath(ctx *policy.RequestContext, regexPattern string) (string, error) {
	if ctx.Path == "" {
		return "", fmt.Errorf("request path is empty")
	}
	// Parse the URL-encoded path (remove query string for path matching)
	decodedPath, err := url.PathUnescape(ctx.Path)
	if err != nil {
		return "", fmt.Errorf("failed to decode path: %w", err)
	}

	// Remove query string from path for regex matching
	pathWithoutQuery := strings.Split(decodedPath, "?")[0]

	// Compile regex pattern
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern %s: %w", regexPattern, err)
	}

	// Find matches
	matches := re.FindStringSubmatch(pathWithoutQuery)
	if len(matches) < 2 {
		return "", fmt.Errorf("regex pattern %s did not match path %s", regexPattern, pathWithoutQuery)
	}

	// Return the first capture group (index 1, index 0 is the full match)
	if matches[1] != "" {
		return matches[1], nil
	}

	return "", fmt.Errorf("regex pattern %s matched but no capture group found", regexPattern)
}

// modifyRequestModel modifies the request to replace the model field based on location
func (p *ModelRoundRobinPolicy) modifyRequestModel(ctx *policy.RequestContext, newModel string) policy.RequestAction {
	location := p.params.RequestModel.Location
	identifier := p.params.RequestModel.Identifier

	switch location {
	case "payload":
		return p.modifyModelInPayload(ctx, newModel, identifier)
	case "header":
		return p.modifyModelInHeader(ctx, newModel, identifier)
	case "queryParam":
		return p.modifyModelInQueryParam(ctx, newModel, identifier)
	case "pathParam":
		return p.modifyModelInPathParam(ctx, newModel, identifier)
	default:
		slog.Debug("ModelRoundRobin: Unsupported location", "location", location)
		return policy.UpstreamRequestModifications{}
	}
}

// modifyModelInPayload modifies the model in request body using JSONPath
func (p *ModelRoundRobinPolicy) modifyModelInPayload(ctx *policy.RequestContext, newModel string, jsonPath string) policy.RequestAction {
	if ctx.Body == nil || ctx.Body.Content == nil {
		return policy.ImmediateResponse{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"error":"Request body is empty."}`),
		}
	}

	// Parse request body
	var payloadData map[string]interface{}
	if err := json.Unmarshal(ctx.Body.Content, &payloadData); err != nil {
		slog.Error("ModelRoundRobin: Error unmarshaling request body", "error", err)
		return policy.ImmediateResponse{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(fmt.Sprintf(`{"error":"Invalid JSON in request body: %s"}`, err.Error())),
		}
	}

	// Update model in payload
	if err := utils.SetValueAtJSONPath(payloadData, jsonPath, newModel); err != nil {
		slog.Error("ModelRoundRobin: Error setting model in request body", "jsonPath", jsonPath, "error", err)
		return policy.ImmediateResponse{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(fmt.Sprintf(`{"error":"Invalid or missing model at '%s': %s"}`, jsonPath, err.Error())),
		}
	}

	// Marshal back to JSON
	updatedPayload, err := json.Marshal(payloadData)
	if err != nil {
		slog.Error("ModelRoundRobin: Error marshaling updated request body", "error", err)
		return policy.ImmediateResponse{
			StatusCode: 500,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(fmt.Sprintf(`{"error":"Failed to serialize updated request body: %s"}`, err.Error())),
		}
	}

	slog.Debug("ModelRoundRobin: Modified request model in payload", "originalModel", ctx.Metadata[MetadataKeyOriginalModel], "newModel", newModel, "jsonPath", jsonPath)

	return policy.UpstreamRequestModifications{
		Body: updatedPayload,
	}
}

// modifyModelInHeader modifies the model in request header
func (p *ModelRoundRobinPolicy) modifyModelInHeader(ctx *policy.RequestContext, newModel string, headerName string) policy.RequestAction {
	slog.Debug("ModelRoundRobin: Modified request model in header", "originalModel", ctx.Metadata[MetadataKeyOriginalModel], "newModel", newModel, "header", headerName)

	return policy.UpstreamRequestModifications{
		SetHeaders: map[string]string{headerName: newModel},
	}
}

// modifyModelInQueryParam modifies the model in query parameter by updating the path
func (p *ModelRoundRobinPolicy) modifyModelInQueryParam(ctx *policy.RequestContext, newModel string, paramName string) policy.RequestAction {
	if ctx.Path == "" {
		slog.Debug("ModelRoundRobin: Cannot modify query param, path is empty")
		return policy.UpstreamRequestModifications{}
	}

	// Parse the URL-encoded path
	decodedPath, err := url.PathUnescape(ctx.Path)
	if err != nil {
		slog.Debug("ModelRoundRobin: Error decoding path", "error", err)
		return policy.UpstreamRequestModifications{}
	}

	// Split path and query string
	parts := strings.Split(decodedPath, "?")
	pathBase := parts[0]

	var queryValues url.Values
	if len(parts) == 2 {
		// Parse existing query string
		queryValues, err = url.ParseQuery(parts[1])
		if err != nil {
			slog.Debug("ModelRoundRobin: Error parsing query string", "error", err)
			return policy.UpstreamRequestModifications{}
		}
	} else {
		// No existing query string, create new
		queryValues = make(url.Values)
	}

	// Update the query parameter
	queryValues.Set(paramName, newModel)

	// Reconstruct path with updated query string
	updatedPath := pathBase
	if len(queryValues) > 0 {
		updatedPath = pathBase + "?" + queryValues.Encode()
	}

	slog.Debug("ModelRoundRobin: Modified request model in query param", "originalModel", ctx.Metadata[MetadataKeyOriginalModel], "newModel", newModel, "param", paramName, "newPath", updatedPath)

	// Set the :path pseudo-header to modify the path and query string
	// Envoy ext_proc requires path modifications via the :path header
	return policy.UpstreamRequestModifications{
		SetHeaders: map[string]string{
			":path": updatedPath,
		},
	}
}

// modifyModelInPathParam modifies the model in path parameter using regex replacement
func (p *ModelRoundRobinPolicy) modifyModelInPathParam(ctx *policy.RequestContext, newModel string, regexPattern string) policy.RequestAction {
	if ctx.Path == "" {
		slog.Debug("ModelRoundRobin: Cannot modify path param, path is empty")
		return policy.UpstreamRequestModifications{}
	}

	// Parse the URL-encoded path
	decodedPath, err := url.PathUnescape(ctx.Path)
	if err != nil {
		slog.Debug("ModelRoundRobin: Error decoding path", "error", err)
		return policy.UpstreamRequestModifications{}
	}

	// Split path and query string (we need to preserve query string)
	parts := strings.Split(decodedPath, "?")
	pathWithoutQuery := parts[0]
	queryString := ""
	if len(parts) == 2 {
		queryString = parts[1]
	}

	// Compile regex pattern
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		slog.Debug("ModelRoundRobin: Invalid regex pattern", "pattern", regexPattern, "error", err)
		return policy.UpstreamRequestModifications{}
	}

	// Find the match to verify it exists and get the match indices
	matchIndices := re.FindStringSubmatchIndex(pathWithoutQuery)
	if len(matchIndices) < 4 {
		slog.Debug("ModelRoundRobin: Regex pattern did not match path or no capture group", "pattern", regexPattern, "path", pathWithoutQuery)
		return policy.UpstreamRequestModifications{}
	}

	// Replace the first capture group (indices 2-3) with the new model
	// matchIndices[0:2] = full match start/end
	// matchIndices[2:4] = first capture group start/end
	captureStart := matchIndices[2]
	captureEnd := matchIndices[3]

	if captureStart == -1 || captureEnd == -1 {
		slog.Debug("ModelRoundRobin: No capture group found in regex pattern", "pattern", regexPattern)
		return policy.UpstreamRequestModifications{}
	}

	// Replace the captured portion with the new model
	updatedPath := pathWithoutQuery[:captureStart] + newModel + pathWithoutQuery[captureEnd:]

	// Reconstruct full path with query string if it existed
	updatedFullPath := updatedPath
	if queryString != "" {
		updatedFullPath = updatedPath + "?" + queryString
	}

	slog.Debug("ModelRoundRobin: Modified request model in path param", "originalModel", ctx.Metadata[MetadataKeyOriginalModel], "newModel", newModel, "pattern", regexPattern, "originalPath", pathWithoutQuery, "newPath", updatedPath)

	// Set the :path pseudo-header to modify the path
	// Envoy ext_proc requires path modifications via the :path header
	return policy.UpstreamRequestModifications{
		SetHeaders: map[string]string{
			":path": updatedFullPath,
		},
	}
}
