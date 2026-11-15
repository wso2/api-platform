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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// refusedFieldMessage describes a refused field without naming the rule or the host.
const refusedFieldMessage = "The URL's destination is not permitted."

// CheckRequestURLs applies server.network_security.access_control to the URL fields
// of an artifact's create or update request, before anything is stored: its upstream
// URLs, a GraphQL SDL URL, per-gateway endpoint overrides
// (associatedGateways[].configurations.endpointUrl), and an LLM provider template's
// endpoint and spec URLs. It returns apperror.DestinationNotAllowed, naming the first
// refused field in its field errors, and nil otherwise. payload is any value that
// marshals to the request's JSON shape.
//
// Not checked, by design:
//   - vhosts, and the addresses platform-api uses to reach portals and gateways:
//     deployment configuration, not tenant destinations;
//   - URLs that are only shown to people: a template's logo, a publication's
//     endpoints, agent card content;
//   - policy parameters;
//   - a URL built from a {{ secret }} or {{ env }} placeholder: the gateway checks
//     the resolved value when it deploys the artifact;
//   - artifacts a gateway imports, which the gateway already checked.
//
// The checks do no network I/O: scheme, hostname rule and IP literal (see
// utils.CheckUserURL).
func CheckRequestURLs(ctx context.Context, orgID string, payload any) error {
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to inspect request URLs: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || doc == nil {
		// Not an object (a nil pointer marshals to null): no URL fields.
		return nil
	}

	for _, t := range collectRequestURLs(doc) {
		if err := checkURLField(ctx, orgID, t.path, t.url); err != nil {
			return err
		}
	}
	return nil
}

// checkURLField applies server.network_security.access_control to one URL taken
// from field of a request. A refusal is apperror.DestinationNotAllowed naming field;
// the host and the rule that refused it are logged, never returned.
func checkURLField(ctx context.Context, orgID, field, rawURL string) error {
	err := utils.CheckUserURL(ctx, rawURL)
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if !apperror.DestinationNotAllowed.Is(err) || !errors.As(err, &appErr) {
		return err
	}
	slog.Warn("Request URL refused by network access control",
		slog.String("organization_id", orgID),
		slog.String("field", field),
		slog.String("host", hostOf(rawURL)))
	return appErr.WithFieldErrors([]apperror.FieldError{{Field: field, Message: refusedFieldMessage}})
}

// urlTarget is one URL found in a request, with the JSON path of its field.
type urlTarget struct {
	path string
	url  string
}

// addURL records value at path when it is a URL that can be judged here: a non-empty
// string not built from a template placeholder.
func addURL(out *[]urlTarget, path string, value any) {
	s, ok := value.(string)
	if !ok {
		return
	}
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "{{") {
		return
	}
	*out = append(*out, urlTarget{path: path, url: s})
}

// collectRequestURLs gathers the URL fields of a request, in a stable order.
func collectRequestURLs(doc map[string]any) []urlTarget {
	var out []urlTarget

	// upstream: {url} or {main: {url}, sandbox: {url}}. A ref names an upstream
	// definition and carries no URL.
	if upstream, ok := doc["upstream"].(map[string]any); ok {
		addURL(&out, "upstream.url", upstream["url"])
		for _, env := range []string{"main", "sandbox"} {
			if branch, ok := upstream[env].(map[string]any); ok {
				addURL(&out, "upstream."+env+".url", branch["url"])
			}
		}
	}

	// GraphQL: the SDL document platform-api fetches.
	addURL(&out, "sdlUrl", doc["sdlUrl"])

	// A per-gateway upstream override, applied when the artifact is deployed there.
	if gateways, ok := doc["associatedGateways"].([]any); ok {
		for i, gw := range gateways {
			entry, ok := gw.(map[string]any)
			if !ok {
				continue
			}
			if conf, ok := entry["configurations"].(map[string]any); ok {
				addURL(&out, fmt.Sprintf("associatedGateways[%d].configurations.endpointUrl", i), conf["endpointUrl"])
			}
		}
	}

	// LLM provider template: the provider endpoint it defaults to and its spec.
	// logoUrl is only displayed.
	if metadata, ok := doc["metadata"].(map[string]any); ok {
		addURL(&out, "metadata.endpointUrl", metadata["endpointUrl"])
		addURL(&out, "metadata.openapiSpecUrl", metadata["openapiSpecUrl"])
	}
	return out
}

// hostOf returns raw's hostname for internal logs, or "" when it does not parse.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
