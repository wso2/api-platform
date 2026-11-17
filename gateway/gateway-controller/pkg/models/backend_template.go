/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package models

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// EndpointKeyPlaceholder marks where a templated Backend's endpoint host takes the key a
// request carries. Must match the SDK's policy.BackendKeyPlaceholder.
const EndpointKeyPlaceholder = "{key}"

// endpointTemplateSampleKey fills the placeholder when a template is checked as a plain URL.
const endpointTemplateSampleKey = "key"

// DNS limits an expanded templated host must meet.
const (
	maxTemplateLabelLength = 63
	maxTemplateHostLength  = 253
)

var (
	templateHostLabel  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	templateLabelAffix = regexp.MustCompile(`^[a-z0-9-]*$`)
)

// EndpointTemplate is a templated Backend endpoint: scheme://host[:port], whose host has
// EndpointKeyPlaceholder inside its first label and at least one fixed label after it. Every
// host a request can reach through it therefore ends in the template's fixed domain. The SDK's
// policy.ParseBackendURLTemplate accepts exactly the templates ParseEndpointTemplate accepts.
type EndpointTemplate struct {
	// Scheme is http or https.
	Scheme string
	prefix string // first-label text before the placeholder
	suffix string // first-label text after the placeholder
	domain string // the fixed labels after the first, without the leading "."
	// Port is the explicit port, or "" for the scheme's default.
	Port string
}

// IsEndpointTemplate reports whether an endpoint URL is meant as a template: it has a brace,
// which no plain URL host can contain.
func IsEndpointTemplate(raw string) bool {
	return strings.ContainsAny(raw, "{}")
}

// ParseEndpointTemplate parses a templated endpoint URL. The error names the rule it breaks.
func ParseEndpointTemplate(raw string) (EndpointTemplate, error) {
	var t EndpointTemplate
	rest := ""
	switch {
	case strings.HasPrefix(raw, "https://"):
		t.Scheme, rest = "https", strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		t.Scheme, rest = "http", strings.TrimPrefix(raw, "http://")
	default:
		return EndpointTemplate{}, errors.New("a templated endpoint URL must start with http:// or https://")
	}
	if strings.Count(raw, EndpointKeyPlaceholder) != 1 || strings.Count(raw, "{") != 1 || strings.Count(raw, "}") != 1 {
		return EndpointTemplate{}, errors.New("a templated endpoint URL must contain " + EndpointKeyPlaceholder +
			" exactly once and no other braces")
	}
	rest = strings.TrimSuffix(rest, "/")
	if strings.ContainsAny(rest, "/?#@[]") {
		return EndpointTemplate{}, errors.New("a templated endpoint URL can have only a scheme, a host and a port: " +
			"no path, query, fragment or user info (set the path in spec.basePath)")
	}
	host := rest
	if i := strings.LastIndexByte(rest, ':'); i >= 0 {
		host, t.Port = rest[:i], rest[i+1:]
		if n, err := strconv.Atoi(t.Port); err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != t.Port {
			return EndpointTemplate{}, errors.New("the port of a templated endpoint URL must be a number from 1 to 65535")
		}
	}
	first, domain, ok := strings.Cut(host, ".")
	if !ok || !strings.Contains(first, EndpointKeyPlaceholder) {
		return EndpointTemplate{}, errors.New(EndpointKeyPlaceholder + " must be in the first label of the host, " +
			"followed by at least one fixed label, for example https://api-" + EndpointKeyPlaceholder + ".example.com")
	}
	prefix, suffix, _ := strings.Cut(first, EndpointKeyPlaceholder)
	if !templateLabelAffix.MatchString(prefix) || !templateLabelAffix.MatchString(suffix) ||
		strings.HasPrefix(prefix, "-") || strings.HasSuffix(suffix, "-") {
		return EndpointTemplate{}, errors.New("the text around " + EndpointKeyPlaceholder +
			" must be lowercase letters, digits and hyphens, and the label must not start or end with a hyphen")
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if !templateHostLabel.MatchString(label) {
			return EndpointTemplate{}, errors.New("the host labels after " + EndpointKeyPlaceholder +
				" must be lowercase letters, digits and inner hyphens, 1 to 63 characters each")
		}
	}
	if !strings.ContainsAny(labels[len(labels)-1], "abcdefghijklmnopqrstuvwxyz") {
		return EndpointTemplate{}, errors.New("the last label of a templated host must contain a letter, " +
			"so the host can never be an IP address")
	}
	// The shortest key is one character. A template too long even for it can never expand.
	if label := len(prefix) + 1 + len(suffix); label > maxTemplateLabelLength || label+1+len(domain) > maxTemplateHostLength {
		return EndpointTemplate{}, errors.New("the fixed text of a templated host leaves no room for a key: " +
			"with a one-character key, the first label must fit in 63 characters and the host in 253")
	}
	t.prefix, t.suffix, t.domain = prefix, suffix, domain
	return t, nil
}

// String is the template as published to the policy engine: scheme://host[:port].
func (t EndpointTemplate) String() string {
	return t.Scheme + "://" + t.authority(EndpointKeyPlaceholder)
}

// Sample is the template with a sample key, a URL the plain-URL checks can parse.
func (t EndpointTemplate) Sample() string {
	return t.Scheme + "://" + t.authority(endpointTemplateSampleKey)
}

// Domain is the fixed part of the host after the first label, for example "url.fr" in
// https://url-{key}.url.fr.
func (t EndpointTemplate) Domain() string {
	return t.domain
}

func (t EndpointTemplate) authority(key string) string {
	authority := t.prefix + key + t.suffix + "." + t.domain
	if t.Port != "" {
		authority += ":" + t.Port
	}
	return authority
}

// BackendEndpointTemplate returns the template of a Backend whose single endpoint is templated.
// ok is false for a plain Backend; err reports a templated endpoint that breaks the rules,
// including a template that is not the Backend's only endpoint.
func BackendEndpointTemplate(backend api.BackendRequest) (t EndpointTemplate, ok bool, err error) {
	endpoints := backend.Spec.Endpoints
	templated := false
	for _, endpoint := range endpoints {
		if IsEndpointTemplate(endpoint.Url) {
			templated = true
		}
	}
	if !templated {
		return EndpointTemplate{}, false, nil
	}
	if len(endpoints) != 1 {
		return EndpointTemplate{}, true, errors.New("a templated endpoint must be the Backend's only endpoint")
	}
	t, err = ParseEndpointTemplate(strings.TrimSpace(endpoints[0].Url))
	return t, true, err
}

// EndpointTemplate returns the Backend's endpoint template when it is templated and valid.
func (e *StoredBackend) EndpointTemplate() (EndpointTemplate, bool) {
	t, ok, err := BackendEndpointTemplate(e.Configuration)
	return t, ok && err == nil
}

// WithSampleEndpoint returns a copy of backend whose templated endpoint URL is replaced by the
// template's sample, so the checks written for plain URLs (scheme, port, TLS) apply to it. A
// Backend that is not validly templated is returned unchanged.
func WithSampleEndpoint(backend api.BackendRequest) api.BackendRequest {
	t, ok, err := BackendEndpointTemplate(backend)
	if !ok || err != nil {
		return backend
	}
	endpoints := append(backend.Spec.Endpoints[:0:0], backend.Spec.Endpoints...)
	endpoints[0].Url = t.Sample()
	backend.Spec.Endpoints = endpoints
	return backend
}
