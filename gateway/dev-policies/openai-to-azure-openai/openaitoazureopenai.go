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

// Package openaitoazureopenai rewrites an OpenAI Chat Completions request path
// into the Azure OpenAI form
// /openai/deployments/{deployment}/{pathSuffix}?api-version=<apiVersion>.
// The body passes through unchanged; Azure already emits OpenAI-shaped
// responses, so response translation is intentionally not implemented.
package openaitoazureopenai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	PolicyName                  = "openai-to-azure-openai"
	DefaultPathSuffix           = "/chat/completions"
	MetadataKeySelectedProvider = "selected_provider"
)

type PolicyParams struct {
	APIVersion string
	Model      string
	PathSuffix string
	// Id is the upstream provider this translator targets. It is both the
	// upstream cluster the request is routed to and the key matched
	// (case-insensitive) against SharedContext.Metadata["selected_provider"]
	// in multi-provider mode.
	Id string
}

type TranslatorPolicy struct {
	params PolicyParams
}

func GetPolicy(_ policy.PolicyMetadata, rawParams map[string]interface{}) (policy.Policy, error) {
	parsed, err := parseParams(rawParams)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid params: %w", PolicyName, err)
	}
	return &TranslatorPolicy{params: parsed}, nil
}

// Mode buffers the request body even though we don't modify it — the
// deployment id may have to be read from the body's "model" field when the
// operator hasn't pinned one.
func (p *TranslatorPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *TranslatorPolicy) OnRequestBody(
	_ context.Context,
	reqCtx *policy.RequestContext,
	_ map[string]interface{},
) policy.RequestAction {
	if !p.shouldRun(reqCtx) {
		return policy.UpstreamRequestModifications{}
	}

	deployment := p.params.Model
	if deployment == "" {
		deployment = readModelFromBody(reqCtx)
	}
	if deployment == "" {
		return errResponse(400,
			"'model' is required in the request body (or as a policy parameter) "+
				"to derive the Azure deployment id.")
	}

	newPath := buildAzurePath(deployment, p.params.PathSuffix, p.params.APIVersion)
	slog.Debug(PolicyName+": rewriting request path",
		"id", p.params.Id, "deployment", deployment, "path", newPath)

	mods := policy.UpstreamRequestModifications{Path: &newPath}
	if p.params.Id != "" {
		upstream := p.params.Id
		mods.UpstreamName = &upstream
	}
	return mods
}

// shouldRun reports whether the request should be rewritten. When no upstream
// router (e.g. openai-header-router) has published a selected provider into the
// metadata, the proxy is in single-provider mode and the translator always
// runs. When a provider has been selected, the translator runs only if that
// selection matches its own "id".
func (p *TranslatorPolicy) shouldRun(reqCtx *policy.RequestContext) bool {
	selected := selectedProvider(reqCtx)
	if selected == "" {
		// Single-provider mode: no router selected a provider, so run.
		return true
	}
	return strings.EqualFold(selected, p.params.Id)
}

func selectedProvider(reqCtx *policy.RequestContext) string {
	if reqCtx == nil || reqCtx.SharedContext == nil || reqCtx.Metadata == nil {
		return ""
	}
	raw, ok := reqCtx.Metadata[MetadataKeySelectedProvider]
	if !ok {
		return ""
	}
	v, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func readModelFromBody(reqCtx *policy.RequestContext) string {
	if reqCtx.Body == nil || !reqCtx.Body.Present || len(reqCtx.Body.Content) == 0 {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(reqCtx.Body.Content, &payload); err != nil {
		return ""
	}
	model, _ := payload["model"].(string)
	return strings.TrimSpace(model)
}

func buildAzurePath(deployment, pathSuffix, apiVersion string) string {
	return fmt.Sprintf("/openai/deployments/%s%s?api-version=%s",
		deployment, pathSuffix, apiVersion)
}

func parseParams(params map[string]interface{}) (PolicyParams, error) {
	result := PolicyParams{PathSuffix: DefaultPathSuffix}

	apiVersion, err := optionalString(params, "apiVersion")
	if err != nil {
		return result, err
	}
	if apiVersion == "" {
		return result, fmt.Errorf("'apiVersion' is required")
	}
	result.APIVersion = apiVersion

	if v, err := optionalString(params, "model"); err != nil {
		return result, err
	} else {
		result.Model = v
	}

	if v, err := optionalString(params, "pathSuffix"); err != nil {
		return result, err
	} else if v != "" {
		// PathSuffix must start with '/' so buildAzurePath can concatenate
		// without inspecting the operator-supplied string.
		if !strings.HasPrefix(v, "/") {
			v = "/" + v
		}
		result.PathSuffix = v
	}

	if v, err := optionalString(params, "id"); err != nil {
		return result, err
	} else {
		result.Id = v
	}

	return result, nil
}

func optionalString(params map[string]interface{}, key string) (string, error) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return "", nil
	}
	v, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	return strings.TrimSpace(v), nil
}

func errResponse(statusCode int, message string) policy.ImmediateResponse {
	body, _ := json.Marshal(map[string]string{"error": message})
	return policy.ImmediateResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}
