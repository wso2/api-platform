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

package requesttransformation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	matchTypeExact   = "EXACT"
	matchTypeRegex   = "REGEX"
	matchTypePresent = "PRESENT"

	pathReplacePrefix = "REPLACEPREFIXMATCH"
	pathReplaceFull   = "REPLACEFULLPATH"
	pathReplaceRegex  = "REPLACEREGEXMATCH"

	queryActionReplace      = "REPLACE"
	queryActionRemove       = "REMOVE"
	queryActionAdd          = "ADD"
	queryActionAppend       = "APPEND"
	queryActionReplaceRegex = "REPLACEREGEXMATCH"

	dynamicMetadataNamespace = "api_platform.policy_engine.envoy.filters.http.ext_proc"
)

var ins = &RequestTransformationPolicy{}

// RequestTransformationPolicy implements request transformation (path, query, method)
type RequestTransformationPolicy struct{}

// GetPolicy returns the policy instance
func GetPolicy(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
	slog.Debug("[Request Transformation]: GetPolicy called", "name", metadata.Name, "version", metadata.Version)
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *RequestTransformationPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Needs request headers for matching and rewriting
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

type policyConfig struct {
	Match         *matchConfig  `json:"match"`
	PathRewrite   *pathRewrite  `json:"pathRewrite"`
	QueryRewrite  *queryRewrite `json:"queryRewrite"`
	MethodRewrite string        `json:"methodRewrite"`
}

type matchConfig struct {
	Headers     []headerMatcher   `json:"headers"`
	QueryParams []queryParamMatch `json:"queryParams"`
}

type headerMatcher struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type queryParamMatch struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type pathRewrite struct {
	Type               string            `json:"type"`
	ReplacePrefixMatch string            `json:"replacePrefixMatch"`
	ReplaceFullPath    string            `json:"replaceFullPath"`
	ReplaceRegexMatch  *regexReplacement `json:"replaceRegexMatch"`
}

type regexReplacement struct {
	Pattern      string `json:"pattern"`
	Substitution string `json:"substitution"`
}

type queryRewrite struct {
	Rules []queryRule `json:"rules"`
}

type queryRule struct {
	Action       string `json:"action"`
	Name         string `json:"name"`
	Value        string `json:"value"`
	Separator    string `json:"separator"`
	Pattern      string `json:"pattern"`
	Substitution string `json:"substitution"`
}

// OnRequest applies request transformations based on policy configuration
func (p *RequestTransformationPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	cfg, err := parseConfig(params)
	if err != nil {
		return configErrorResponse("Invalid request-transformation configuration", err)
	}

	if cfg == nil {
		slog.Debug("[Request Transformation]: No configuration provided, passing through")
		return policy.UpstreamRequestModifications{}
	}

	if !matchesRequest(ctx, cfg.Match) {
		slog.Debug("[Request Transformation]: Match conditions not met, skipping transformations")
		return policy.UpstreamRequestModifications{}
	}

	originalPath := ctx.Path
	pathOnly, queryValues, _ := splitPathAndQuery(originalPath)
	updatedPath := pathOnly
	pathRewriteApplied := false
	queryRewriteConfigured := cfg.QueryRewrite != nil

	if cfg.PathRewrite != nil {
		updatedPath = applyPathRewrite(ctx, updatedPath, cfg.PathRewrite)
		pathRewriteApplied = updatedPath != pathOnly
	}

	if cfg.QueryRewrite != nil {
		if err := applyQueryRewrite(queryValues, cfg.QueryRewrite); err != nil {
			return configErrorResponse("Invalid queryRewrite configuration", err)
		}
	}

	finalPath := originalPath
	if pathRewriteApplied || queryRewriteConfigured {
		finalPath = buildPath(updatedPath, queryValues)
	}

	mods := policy.UpstreamRequestModifications{}
	if finalPath != originalPath {
		slog.Info("[Request Transformation]: Rewriting path", "from", originalPath, "to", finalPath)
		mods.SetHeaders = map[string]string{":path": finalPath}
	}

	method := strings.TrimSpace(cfg.MethodRewrite)
	if method != "" {
		method = strings.ToUpper(method)
		if !isAllowedMethod(method) {
			return configErrorResponse("Invalid methodRewrite value", fmt.Errorf("unsupported method: %s", method))
		}
		mods.DynamicMetadata = map[string]map[string]any{
			dynamicMetadataNamespace: {
				"request_transformation.target_method": method,
			},
		}
		slog.Info("[Request Transformation]: Scheduling method rewrite", "method", method)
	}

	return mods
}

// OnResponse is not used for this policy
func (p *RequestTransformationPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil
}

func parseConfig(params map[string]interface{}) (*policyConfig, error) {
	if params == nil || len(params) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize params: %w", err)
	}
	var cfg policyConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse params: %w", err)
	}
	return &cfg, nil
}

func configErrorResponse(message string, err error) policy.RequestAction {
	slog.Error("[Request Transformation]: Configuration error", "error", err)
	body, _ := json.Marshal(map[string]string{
		"error":   "Configuration Error",
		"message": fmt.Sprintf("%s: %s", message, err.Error()),
	})
	return policy.ImmediateResponse{
		StatusCode: 500,
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: body,
	}
}

func matchesRequest(ctx *policy.RequestContext, match *matchConfig) bool {
	if match == nil {
		return true
	}

	if len(match.Headers) == 0 && len(match.QueryParams) == 0 {
		return true
	}

	for _, matcher := range match.Headers {
		if !matchHeader(ctx, matcher) {
			return false
		}
	}

	if len(match.QueryParams) > 0 {
		_, queryValues, _ := splitPathAndQuery(ctx.Path)
		for _, matcher := range match.QueryParams {
			if !matchQueryParam(queryValues, matcher) {
				return false
			}
		}
	}

	return true
}

func matchHeader(ctx *policy.RequestContext, matcher headerMatcher) bool {
	name := strings.TrimSpace(matcher.Name)
	if name == "" {
		return false
	}

	matchType := strings.ToUpper(strings.TrimSpace(matcher.Type))
	values := ctx.Headers.Get(name)

	switch matchType {
	case matchTypePresent:
		return len(values) > 0
	case matchTypeExact:
		for _, v := range values {
			if v == matcher.Value {
				return true
			}
		}
		return false
	case matchTypeRegex:
		regex, err := regexp.Compile(matcher.Value)
		if err != nil {
			slog.Warn("[Request Transformation]: Invalid header regex", "name", name, "pattern", matcher.Value, "error", err)
			return false
		}
		for _, v := range values {
			if regex.MatchString(v) {
				return true
			}
		}
		return false
	default:
		slog.Warn("[Request Transformation]: Unsupported header match type", "type", matcher.Type)
		return false
	}
}

func matchQueryParam(values url.Values, matcher queryParamMatch) bool {
	name := strings.TrimSpace(matcher.Name)
	if name == "" {
		return false
	}
	matchType := strings.ToUpper(strings.TrimSpace(matcher.Type))
	vals, exists := values[name]

	switch matchType {
	case matchTypePresent:
		return exists && len(vals) > 0
	case matchTypeExact:
		for _, v := range vals {
			if v == matcher.Value {
				return true
			}
		}
		return false
	case matchTypeRegex:
		regex, err := regexp.Compile(matcher.Value)
		if err != nil {
			slog.Warn("[Request Transformation]: Invalid query regex", "name", name, "pattern", matcher.Value, "error", err)
			return false
		}
		for _, v := range vals {
			if regex.MatchString(v) {
				return true
			}
		}
		return false
	default:
		slog.Warn("[Request Transformation]: Unsupported query match type", "type", matcher.Type)
		return false
	}
}

func applyPathRewrite(ctx *policy.RequestContext, currentPath string, cfg *pathRewrite) string {
	rewriteType := strings.ToUpper(strings.TrimSpace(cfg.Type))

	switch rewriteType {
	case pathReplacePrefix:
		operationPath := strings.TrimSpace(ctx.OperationPath)
		if operationPath == "" {
			slog.Warn("[Request Transformation]: Operation path is empty, skipping prefix rewrite")
			return currentPath
		}
		if !strings.HasPrefix(currentPath, operationPath) {
			return currentPath
		}
		remainder := strings.TrimPrefix(currentPath, operationPath)
		return cfg.ReplacePrefixMatch + remainder
	case pathReplaceFull:
		if cfg.ReplaceFullPath == "" {
			return currentPath
		}
		return cfg.ReplaceFullPath
	case pathReplaceRegex:
		if cfg.ReplaceRegexMatch == nil {
			return currentPath
		}
		regex, err := regexp.Compile(cfg.ReplaceRegexMatch.Pattern)
		if err != nil {
			slog.Warn("[Request Transformation]: Invalid path regex", "pattern", cfg.ReplaceRegexMatch.Pattern, "error", err)
			return currentPath
		}
		return regex.ReplaceAllString(currentPath, cfg.ReplaceRegexMatch.Substitution)
	default:
		slog.Warn("[Request Transformation]: Unsupported path rewrite type", "type", cfg.Type)
		return currentPath
	}
}

func applyQueryRewrite(values url.Values, cfg *queryRewrite) error {
	for _, rule := range cfg.Rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return fmt.Errorf("query rule name cannot be empty")
		}

		action := strings.ToUpper(strings.TrimSpace(rule.Action))
		switch action {
		case queryActionReplace:
			values.Set(name, rule.Value)
		case queryActionRemove:
			values.Del(name)
		case queryActionAdd:
			values.Add(name, rule.Value)
		case queryActionAppend:
			separator := rule.Separator
			vals, exists := values[name]
			if !exists || len(vals) == 0 {
				values.Set(name, rule.Value)
				break
			}
			updated := make([]string, 0, len(vals))
			for _, v := range vals {
				updated = append(updated, v+separator+rule.Value)
			}
			values[name] = updated
		case queryActionReplaceRegex:
			regex, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return fmt.Errorf("invalid query regex pattern: %w", err)
			}
			vals, exists := values[name]
			if !exists || len(vals) == 0 {
				break
			}
			updated := make([]string, 0, len(vals))
			for _, v := range vals {
				updated = append(updated, regex.ReplaceAllString(v, rule.Substitution))
			}
			values[name] = updated
		default:
			return fmt.Errorf("unsupported query rewrite action: %s", rule.Action)
		}
	}
	return nil
}

func splitPathAndQuery(rawPath string) (string, url.Values, error) {
	parsed, err := url.ParseRequestURI(rawPath)
	if err != nil {
		// Best-effort fallback: treat as path only
		return rawPath, url.Values{}, err
	}
	return parsed.Path, parsed.Query(), nil
}

func buildPath(path string, values url.Values) string {
	encoded := values.Encode()
	if encoded == "" {
		return path
	}
	return path + "?" + encoded
}

func isAllowedMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}
