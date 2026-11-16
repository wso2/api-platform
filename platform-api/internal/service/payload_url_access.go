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
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
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
//   - a URL whose scheme or host is a template placeholder ({{ secret }}, {{ env }}):
//     only the gateway can resolve it. A placeholder elsewhere in the URL does not
//     exempt its host (see destinationOf);
//   - artifacts a gateway imports, which the gateway already checked.
//
// The checks do no network I/O: scheme, hostname rule and IP literal (see
// utils.CheckUserURL).
func CheckRequestURLs(ctx context.Context, orgID string, payload any) error {
	return checkRequestURLs(ctx, orgID, payload, nil)
}

// CheckRequestURLsForUpdate is CheckRequestURLs for an update of a stored artifact of
// the given origin. A DP-originated artifact's runtime configuration belongs to its
// gateway, which applied its own access control when it accepted it: an update keeps
// the stored copy, or must leave it unchanged, so the request's upstream URLs are not
// what the artifact will use and are not checked. Its per-gateway endpoint overrides
// are still stored by the control plane, so those are.
func CheckRequestURLsForUpdate(ctx context.Context, orgID, origin string, payload any) error {
	if origin != constants.OriginDP {
		return CheckRequestURLs(ctx, orgID, payload)
	}
	return checkRequestURLs(ctx, orgID, payload, func(path string) bool {
		return strings.HasPrefix(path, associatedGatewaysField)
	})
}

// checkRequestURLs checks the URL fields of payload that keep accepts (all of them when
// keep is nil).
func checkRequestURLs(ctx context.Context, orgID string, payload any, keep func(path string) bool) error {
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
		if keep != nil && !keep(t.path) {
			continue
		}
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
	// CheckUserURL returns nil or a refusal, never another error: it does no I/O.
	err := utils.CheckUserURL(ctx, rawURL)
	if err == nil {
		return nil
	}
	slog.Warn("Request URL refused by network access control",
		slog.String("organization_id", orgID),
		slog.String("field", field),
		slog.String("host", hostOf(rawURL)))
	return apperror.DestinationNotAllowed.Wrap(err).
		WithFieldErrors([]apperror.FieldError{{Field: field, Message: refusedFieldMessage}})
}

// associatedGatewaysField prefixes the path of a per-gateway endpoint override.
const associatedGatewaysField = "associatedGateways["

// urlTarget is one URL found in a request, with the JSON path of its field.
type urlTarget struct {
	path string
	url  string
}

// addURL records value at path when it is a non-empty string URL whose destination can
// be judged here (see destinationOf).
func addURL(out *[]urlTarget, path string, value any) {
	s, ok := value.(string)
	if !ok {
		return
	}
	if dest, ok := destinationOf(strings.TrimSpace(s)); ok {
		*out = append(*out, urlTarget{path: path, url: dest})
	}
}

// destinationOf returns the URL to check for raw, and false when there is nothing to
// check. A URL without a template placeholder is checked as it is. One with a
// placeholder ({{ secret }}, {{ env }} or any other expression the gateway renders) is
// skipped only when the placeholder is in its scheme or host, which only the gateway can
// resolve; otherwise its literal scheme and host are checked as scheme://host[:port], so
// a placeholder in the userinfo, port, path or query neither hides a refused host nor
// stops the URL from parsing (a templated port is left out).
func destinationOf(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if !strings.Contains(raw, "{{") {
		return raw, true
	}
	scheme, rest, found := strings.Cut(raw, "://")
	if !found || strings.Contains(scheme, "{{") {
		return "", false
	}
	authority := rest
	if end := strings.IndexAny(authority, "/?#"); end >= 0 {
		authority = authority[:end]
	}
	// The host follows any userinfo, which may itself be a placeholder.
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	host, port := splitHostPort(authority)
	if host == "" || strings.Contains(host, "{{") {
		return "", false
	}
	if strings.Contains(port, "{{") {
		// The port is the gateway's to render; the literal host is still checked.
		port = ""
	}
	return scheme + "://" + host + port, true
}

// splitHostPort splits a URL authority into its host and its ":port" suffix (empty when
// there is none), keeping a bracketed IPv6 host whole. It does not validate either part.
func splitHostPort(authority string) (host, port string) {
	if strings.HasPrefix(authority, "[") {
		if end := strings.Index(authority, "]"); end >= 0 {
			return authority[:end+1], authority[end+1:]
		}
		return authority, ""
	}
	if i := strings.Index(authority, ":"); i >= 0 {
		return authority[:i], authority[i:]
	}
	return authority, ""
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
				addURL(&out, fmt.Sprintf("%s%d].configurations.endpointUrl", associatedGatewaysField, i), conf["endpointUrl"])
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
