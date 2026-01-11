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

package cors

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

type CorsPolicy struct {
	AllowedOrigins         []string
	CompiledAllowedOrigins []*regexp.Regexp
	AllowedMethods         []string
	AllowedHeaders         []string
	ExposedHeaders         []string
	MaxAge                 *int
	AllowCredentials       *bool
	ForwardPreflight       bool
}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]any,
) (policy.Policy, error) {
	slog.Debug("Cors Policy: GetPolicy called")
	p := &CorsPolicy{}
	p.AllowedOrigins = getStringArrayParam(params, "allowedOrigins", []string{"*"})
	if len(p.AllowedOrigins) > 1 && strings.Contains(strings.Join(p.AllowedOrigins, ","), "*") {
		slog.Debug("Ignoring other origins as wildcard is included")
		p.AllowedOrigins = []string{"*"}
	}
	p.AllowedMethods = getStringArrayParam(params, "allowedMethods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	p.AllowedHeaders = getStringArrayParam(params, "allowedHeaders", []string{})
	if len(p.AllowedHeaders) > 1 && strings.Contains(strings.Join(p.AllowedHeaders, ","), "*") {
		slog.Debug("Ignoring other headers as wildcard is included")
		p.AllowedHeaders = []string{"*"}
	}
	p.ExposedHeaders = getStringArrayParam(params, "exposedHeaders", nil)

	if v, ok := params["maxAge"]; ok {
		if i, ok := v.(int); ok {
			val := i
			p.MaxAge = &val
		} else if f, ok := v.(float64); ok {
			val := int(f)
			p.MaxAge = &val
		}
	}

	val, ok := params["allowCredentials"]
	if ok {
		if b, ok := val.(bool); ok {
			p.AllowCredentials = &b
		}
	}

	if p.AllowCredentials != nil && *p.AllowCredentials {
		// As per CORS spec, wildcard cannot be used with allow-credentials
		if len(p.AllowedOrigins) == 1 && p.AllowedOrigins[0] == "*" {
			slog.Debug("Cannot have wildcard origin with allowCredentials set to true")
			return nil, fmt.Errorf("cannot have wildcard origin with allowCredentials set to true")
		} else if len(p.AllowedHeaders) == 1 && p.AllowedHeaders[0] == "*" {
			slog.Debug("Cannot have wildcard headers with allowCredentials set to true")
			return nil, fmt.Errorf("cannot have wildcard headers with allowCredentials set to true")
		} else if len(p.AllowedMethods) == 1 && p.AllowedMethods[0] == "*" {
			slog.Debug("Cannot have wildcard methods with allowCredentials set to true")
			return nil, fmt.Errorf("cannot have wildcard methods with allowCredentials set to true")
		} else if len(p.ExposedHeaders) == 1 && p.ExposedHeaders[0] == "*" {
			slog.Debug("Cannot have wildcard exposed headers with allowCredentials set to true")
			return nil, fmt.Errorf("cannot have wildcard exposed headers with allowCredentials set to true")
		}
	}

	val, ok = params["forwardPreflight"]
	if ok {
		if b, ok := val.(bool); ok {
			p.ForwardPreflight = b
		}
	} else {
		p.ForwardPreflight = false
	}

	for _, origin := range p.AllowedOrigins {
		if origin == "*" {
			continue
		}
		regex, err := regexp.Compile(origin)
		if err != nil {
			slog.Debug("Invalid origin regex", "origin", origin, "error", err)
			return nil, fmt.Errorf("invalid origin regex: %s", origin)
		}
		p.CompiledAllowedOrigins = append(p.CompiledAllowedOrigins, regex)
	}

	return p, nil
}

func (p *CorsPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

func (p *CorsPolicy) OnRequest(ctx *policy.RequestContext, params map[string]any) policy.RequestAction {
	if !strings.EqualFold(ctx.Method, "options") {
		slog.Debug("Not a CORS preflight request, skipping CORS policy")
		return nil
	}
	slog.Debug("Handling CORS preflight request")
	isCorsFailure := false
	requestHeaders := ctx.Headers
	headers := make(map[string]string)

	origin := requestHeaders.Get("Origin")
	requestedMethod := requestHeaders.Get("Access-Control-Request-Method")
	requestedHeaders := requestHeaders.Get("Access-Control-Request-Headers")

	// handle origins
	originAllowed := false
	if p.AllowedOrigins[0] == "*" {
		slog.Debug("CORS: Allowing all origins")
		headers["Access-Control-Allow-Origin"] = "*"
		originAllowed = true
	} else if len(origin) > 0 {
		for _, regex := range p.CompiledAllowedOrigins {
			if regex.MatchString(origin[0]) {
				slog.Debug("CORS: Adding allowed origins")
				headers["Access-Control-Allow-Origin"] = origin[0]
				originAllowed = true
				break
			}
		}
	}
	if !originAllowed {
		slog.Debug("CORS: Origin not allowed")
		isCorsFailure = true
	}

	// handle methods
	methodAllowed := false
	if p.AllowedMethods[0] == "*" {
		slog.Debug("CORS: Allowing all methods")
		headers["Access-Control-Allow-Methods"] = "*"
		methodAllowed = true
	} else if len(requestedMethod) > 0 {
		if slices.Contains(p.AllowedMethods, requestedMethod[0]) {
			slog.Debug("CORS: Adding allowed methods")
			headers["Access-Control-Allow-Methods"] = strings.Join(p.AllowedMethods, ",")
			methodAllowed = true
		}
	}
	if !methodAllowed {
		slog.Debug("CORS: Method not allowed")
		isCorsFailure = true
	}

	// handle headers
	headersAllowed := false
	if len(p.AllowedHeaders) > 0 {
		if p.AllowedHeaders[0] == "*" {
			slog.Debug("CORS: Allowing all headers")
			headers["Access-Control-Allow-Headers"] = requestedHeaders[0]
			headersAllowed = true
		} else if len(requestedHeaders) > 0 {
			var checkedHeaders []string
			requestedList := strings.Split(requestedHeaders[0], ",")
			allowedCount := 0
			for _, allowedHeader := range p.AllowedHeaders {
				for _, requestedHeader := range requestedList {
					if strings.EqualFold(allowedHeader, requestedHeader) {
						checkedHeaders = append(checkedHeaders, allowedHeader)
						allowedCount++
					}
				}
			}
			if len(requestedList) == allowedCount {
				slog.Debug("CORS: Adding allowed headers")
				headers["Access-Control-Allow-Headers"] = strings.Join(checkedHeaders, ",")
				headersAllowed = true
			}
		}
	}

	if !headersAllowed && len(requestedHeaders) > 0 {
		slog.Debug("CORS: Headers not allowed")
		isCorsFailure = true
	}

	if len(p.ExposedHeaders) > 0 {
		slog.Debug("CORS: Adding expose headers")
		headers["Access-Control-Expose-Headers"] = strings.Join(p.ExposedHeaders, ",")
	}

	if p.MaxAge != nil {
		slog.Debug("CORS: Adding max age header")
		headers["Access-Control-Max-Age"] = fmt.Sprintf("%d", *p.MaxAge)
	}

	if p.AllowCredentials != nil {
		slog.Debug("CORS: Adding allow credentials header")
		headers["Access-Control-Allow-Credentials"] = fmt.Sprintf("%t", *p.AllowCredentials)
	}

	if isCorsFailure {
		slog.Debug("CORS: Preflight request did not pass the policy")
		headers = make(map[string]string)
		if p.ForwardPreflight {
			slog.Debug("CORS: Forwarding preflight request to upstream.")
			return policy.UpstreamRequestModifications{}
		}
	}

	return policy.ImmediateResponse{
		StatusCode: 204,
		Headers:    headers,
		Body:       nil,
	}
}

func (p *CorsPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]any) policy.ResponseAction {
	return nil
}

func getStringArrayParam(params map[string]any, key string, defaultValue []string) []string {
	if v, ok := params[key]; ok {
		if arr, ok := v.([]any); ok {
			var result []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
	}
	return defaultValue
}
