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

package config

import (
	"net"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// SplitVhosts parses a vhosts.main value into its individual production
// hostnames. Multiple hostnames may be provided separated by ";" (each serves
// the main upstream); surrounding whitespace is trimmed, empty entries are
// dropped, and duplicates are removed while preserving order. A single
// hostname (the common case) returns a one-element slice.
func SplitVhosts(raw string) []string {
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// RestAPIHasSandbox reports whether a REST API has a sandbox upstream, set
// through either url or ref.
func RestAPIHasSandbox(spec api.APIConfigData) bool {
	sb := spec.Upstream.Sandbox
	return sb != nil &&
		((sb.Url != nil && strings.TrimSpace(*sb.Url) != "") ||
			(sb.Ref != nil && strings.TrimSpace(*sb.Ref) != ""))
}

// RestAPIVhosts resolves the vhosts a REST API's routes are served on. main
// holds every vhosts.main entry, or the main default when none is set; the
// first is the primary vhost. sandbox is vhosts.sandbox as written, or the
// sandbox default when it is blank, and applies only when hasSandbox is true.
func RestAPIVhosts(spec api.APIConfigData, vhosts VHostsConfig) (main []string, sandbox string, hasSandbox bool) {
	var rawMain string
	var rawSandbox *string
	if spec.Vhosts != nil {
		rawMain, rawSandbox = spec.Vhosts.Main, spec.Vhosts.Sandbox
	}
	main, sandbox = resolveVhosts(rawMain, rawSandbox, vhosts)
	return main, sandbox, RestAPIHasSandbox(spec)
}

// WebSocketAPIHasSandbox reports whether a WebSocket API has a sandbox
// upstream, set through either url or ref.
func WebSocketAPIHasSandbox(spec api.WebSocketAPIConfigData) bool {
	sb := spec.Upstream.Sandbox
	return sb != nil &&
		((sb.Url != nil && strings.TrimSpace(*sb.Url) != "") ||
			(sb.Ref != nil && strings.TrimSpace(*sb.Ref) != ""))
}

// WebSocketAPIVhosts resolves the vhosts a WebSocket API's channel routes are
// served on, with the same rules as RestAPIVhosts.
func WebSocketAPIVhosts(spec api.WebSocketAPIConfigData, vhosts VHostsConfig) (main []string, sandbox string, hasSandbox bool) {
	var rawMain string
	var rawSandbox *string
	if spec.Vhosts != nil {
		rawMain, rawSandbox = spec.Vhosts.Main, spec.Vhosts.Sandbox
	}
	main, sandbox = resolveVhosts(rawMain, rawSandbox, vhosts)
	return main, sandbox, WebSocketAPIHasSandbox(spec)
}

// resolveVhosts applies an API's vhosts.main and vhosts.sandbox over the
// gateway defaults: every main entry, or the main default when none is set, and
// the sandbox value, or the sandbox default when it is blank.
func resolveVhosts(rawMain string, rawSandbox *string, vhosts VHostsConfig) (main []string, sandbox string) {
	main = []string{vhosts.Main.Default}
	sandbox = vhosts.Sandbox.Default
	if parsed := SplitVhosts(rawMain); len(parsed) > 0 {
		main = parsed
	}
	if rawSandbox != nil && strings.TrimSpace(*rawSandbox) != "" {
		sandbox = *rawSandbox
	}
	return main, sandbox
}

// Domains returns the routing domains of a resolved vhost: the configured
// domains when vhost is the main or sandbox default and that default lists
// any, otherwise vhost itself.
func (v VHostsConfig) Domains(vhost string) []string {
	for _, entry := range []VHostEntry{v.Main, v.Sandbox} {
		if vhost != entry.Default {
			continue
		}
		var out []string
		for _, d := range entry.Domains {
			if d = strings.TrimSpace(d); d != "" {
				out = append(out, d)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{vhost}
}

// ServerName returns the TLS server name (SNI) a client reaching a
// dedicated vhost sends, lower-cased and without a port. ok is false when the
// vhost cannot be told apart on SNI: a main or sandbox default, which every
// API without its own hostname shares; the gateway-default sentinel, which
// stands for a default; an IP address, which clients never send as SNI; and
// anything but an exact DNS name or one with a single leading "*." label.
//
// Envoy lower-cases both filter_chain_match.server_names and the client's
// SNI before matching, so a lower-cased name matches a client that sends it
// in any case.
func (v VHostsConfig) ServerName(vhost string) (name string, ok bool) {
	vhost = strings.TrimSpace(vhost)
	// Hostnames match without regard to case, as in Envoy, so API.example.com
	// is the default api.example.com.
	for _, d := range []string{constants.VHostGatewayDefault, strings.TrimSpace(v.Main.Default), strings.TrimSpace(v.Sandbox.Default)} {
		if strings.EqualFold(vhost, d) {
			return "", false
		}
	}
	return sniName(vhost)
}

// sniName converts a hostname, optionally with a port, to the server name a
// client sends. A trailing dot is not scopable: clients send SNI without it,
// so the vhost is left to the listener that asks every connection.
func sniName(domain string) (string, bool) {
	host := strings.ToLower(strings.TrimSpace(domain))
	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err != nil {
			return "", false
		}
		host = h
	}
	if host == "" || net.ParseIP(host) != nil {
		return "", false
	}
	for i, label := range strings.Split(host, ".") {
		if i == 0 && label == "*" {
			continue
		}
		if !isDNSLabel(label) {
			return "", false
		}
	}
	if host == "*" {
		return "", false
	}
	return host, true
}

// isDNSLabel reports whether label is a non-empty run of a-z, 0-9 and "-".
func isDNSLabel(label string) bool {
	if label == "" {
		return false
	}
	for _, c := range label {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
