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

// Package upstreamguard applies controller.server.network_security.access_control
// to the URLs an artifact asks the gateway to reach: its upstream URLs (and a
// gateway-wide Backend's endpoint URLs), which the router dials, and the URL-typed parameters of the policies attached to
// it, which the policy engine dials. It runs on the rendered artifact (see
// templateengine.SetPostRenderCheck), so a URL produced by {{ env }} or
// {{ secret }} is checked as resolved.
//
// Only URL fields of the artifact spec are checked. Policy system parameters
// come from config.toml and are never checked, and neither are the clusters the
// translator generates for the gateway's own components.
package upstreamguard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine/funcs"
	"github.com/wso2/api-platform/httpkit/netguard"
)

// refusedMessage is the only description of a refusal that leaves the
// controller: it never says which rule refused a URL or what its host
// resolved to.
const refusedMessage = "an upstream or policy URL is not permitted by the gateway's network access control"

// RefusedError reports an artifact URL that access control refuses. Error()
// is the fixed refusedMessage; FieldPath names the refused field (for example
// "spec.upstream.main.url") so the publisher knows what to change.
type RefusedError struct {
	FieldPath string
}

func (e *RefusedError) Error() string { return refusedMessage }

// Is lets errors.Is match a refusal against netguard.ErrDestinationNotAllowed.
func (e *RefusedError) Is(target error) bool { return target == netguard.ErrDestinationNotAllowed }

// IsRefused reports whether err, or any error it wraps, is a *RefusedError.
func IsRefused(err error) bool {
	var refused *RefusedError
	return errors.As(err, &refused)
}

// RefusedFieldPath returns the refused field of a *RefusedError in err's
// chain, or "".
func RefusedFieldPath(err error) string {
	var refused *RefusedError
	if errors.As(err, &refused) {
		return refused.FieldPath
	}
	return ""
}

// IsArtifactRefusal reports whether err means the artifact may not be
// deployed under this gateway's configuration: it reads an environment
// variable outside policy_engine.env_access, or names a URL access control
// refuses. Both are handled the same way: rejected on deploy, undeployed when
// a stored artifact meets them after a restart.
func IsArtifactRefusal(err error) bool {
	return funcs.IsEnvAccessError(err) || IsRefused(err)
}

// RefusalReason names the configuration that refused an artifact, for logs.
func RefusalReason(err error) string {
	if IsRefused(err) {
		return "controller.server.network_security.access_control"
	}
	return "policy_engine.env_access.allow_list"
}

// defaultLookupTimeout bounds the DNS lookup made for each upstream hostname.
const defaultLookupTimeout = 2 * time.Second

// Guard checks rendered artifacts against an access-control policy.
type Guard struct {
	policy        netguard.Policy
	addrPolicy    netguard.Policy
	definitions   map[string]models.PolicyDefinition
	latest        map[string]string
	logger        *slog.Logger
	lookupTimeout time.Duration
}

// New builds a Guard. definitions are the loaded policy definitions, used to
// find which parameters of an attached policy are URLs (`format: uri`).
func New(policy netguard.Policy, definitions map[string]models.PolicyDefinition, logger *slog.Logger) *Guard {
	if logger == nil {
		logger = slog.Default()
	}
	addr := policy
	addr.Hosts = nil // resolved addresses are checked without the host rule
	return &Guard{
		policy:        policy,
		addrPolicy:    addr,
		definitions:   definitions,
		latest:        config.BuildLatestVersionIndex(definitions),
		logger:        logger,
		lookupTimeout: defaultLookupTimeout,
	}
}

// Check returns a *RefusedError for the first refused URL in rendered, and nil
// otherwise. It is CheckArtifact for an artifact whose kind adds no exemption.
func (g *Guard) Check(rendered any) error {
	return g.CheckArtifact("", rendered)
}

// CheckArtifact is a templateengine.PostRenderCheck. kind is the stored
// artifact's kind. For an LlmProxy, the upstream fields are not checked: a
// user's LlmProxy spec has none, and the RestAPI derived from it routes to its
// providers through the gateway's own listener (http://127.0.0.1:<port>),
// hops the controller generates rather than destinations a user chose. The
// providers' own upstream URLs are checked when the providers are deployed.
// The proxy's policy parameters are still checked.
func (g *Guard) CheckArtifact(kind string, rendered any) error {
	raw, err := json.Marshal(rendered)
	if err != nil {
		return fmt.Errorf("failed to inspect rendered configuration: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("failed to inspect rendered configuration: %w", err)
	}
	spec, ok := doc["spec"].(map[string]any)
	if !ok {
		return nil
	}

	targets := g.collect(spec, kind != string(models.KindLlmProxy))
	if kind == string(models.KindBackend) {
		targets = append(collectBackendEndpoints(spec), targets...)
	}
	for _, target := range targets {
		if err := g.checkURL(target.path, target.url); err != nil {
			return err
		}
	}
	return nil
}

// target is one URL found in a spec, with the field it came from.
type target struct {
	path string
	url  string
}

// collect gathers every URL-typed policy parameter in spec, and every upstream
// URL when upstreams is true.
func (g *Guard) collect(spec map[string]any, upstreams bool) []target {
	var out []target
	add := func(path string, v any) {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, target{path: path, url: strings.TrimSpace(s)})
		}
	}

	if upstreams {
		g.collectUpstreams(spec, add)
	}
	g.collectPolicyURLs("spec", spec, &out)
	return out
}

// collectUpstreams passes add every upstream URL in spec.
func (g *Guard) collectUpstreams(spec map[string]any, add func(path string, v any)) {
	// spec.upstream: {url} (Mcp, LlmProvider, Agent) or {main: {url}, sandbox: {url}}
	// (RestApi, GraphQLApi).
	if upstream, ok := spec["upstream"].(map[string]any); ok {
		add("spec.upstream.url", upstream["url"])
		for _, env := range []string{"main", "sandbox"} {
			if branch, ok := upstream[env].(map[string]any); ok {
				add("spec.upstream."+env+".url", branch["url"])
			}
		}
	}
	// spec.upstreamDefinitions[].upstreams[].url
	if defs, ok := spec["upstreamDefinitions"].([]any); ok {
		for i, d := range defs {
			def, ok := d.(map[string]any)
			if !ok {
				continue
			}
			ups, _ := def["upstreams"].([]any)
			for j, u := range ups {
				if up, ok := u.(map[string]any); ok {
					add(fmt.Sprintf("spec.upstreamDefinitions[%d].upstreams[%d].url", i, j), up["url"])
				}
			}
		}
	}
}

// collectBackendEndpoints returns the endpoint URLs of a gateway-wide Backend,
// spec.endpoints[].url, which the router dials like any upstream URL.
func collectBackendEndpoints(spec map[string]any) []target {
	var out []target
	endpoints, _ := spec["endpoints"].([]any)
	for i, e := range endpoints {
		endpoint, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if url, ok := endpoint["url"].(string); ok && strings.TrimSpace(url) != "" {
			out = append(out, target{path: fmt.Sprintf("spec.endpoints[%d].url", i), url: strings.TrimSpace(url)})
		}
	}
	return out
}

// collectPolicyURLs walks v for policy attachments — objects carrying a
// policy "name" and a "params" object — wherever the artifact kind places
// them (API level, per operation, per route), and adds every parameter whose
// definition declares it a URL.
func (g *Guard) collectPolicyURLs(path string, v any, out *[]target) {
	switch node := v.(type) {
	case map[string]any:
		if name, ok := node["name"].(string); ok {
			if params, ok := node["params"].(map[string]any); ok {
				version, _ := node["version"].(string)
				if schema := g.parameterSchema(name, version); schema != nil {
					collectURLParams(path+".params", params, schema, out)
				}
			}
		}
		// Sorted so the first refused field reported is stable.
		for _, key := range slices.Sorted(maps.Keys(node)) {
			g.collectPolicyURLs(path+"."+key, node[key], out)
		}
	case []any:
		for i, child := range node {
			g.collectPolicyURLs(fmt.Sprintf("%s[%d]", path, i), child, out)
		}
	}
}

// parameterSchema returns the parameters schema of the named policy, or nil
// when the policy is unknown (the policy validator reports that) or takes no
// parameters.
func (g *Guard) parameterSchema(name, version string) map[string]any {
	resolved, err := config.ResolvePolicyVersion(g.definitions, g.latest, name, version)
	if err != nil {
		return nil
	}
	def, ok := g.definitions[name+"|"+resolved]
	if !ok || def.Parameters == nil {
		return nil
	}
	return *def.Parameters
}

// urlFormats are the JSON Schema string formats that mark a parameter as a
// URL the policy will dial.
var urlFormats = map[string]bool{"uri": true, "url": true, "iri": true}

// urlMarker marks a parameter as a URL the policy will dial without changing
// how its value is validated. It suits an optional parameter whose empty value
// means "not set" (format: uri would reject the empty string).
const urlMarker = "x-wso2-url"

// isURLSchema reports whether a parameter schema declares a URL the policy
// dials, directly or through one of its allOf, anyOf or oneOf subschemas.
func isURLSchema(schema map[string]any) bool {
	if marked, _ := schema[urlMarker].(bool); marked {
		return true
	}
	format, _ := schema["format"].(string)
	if urlFormats[strings.ToLower(format)] {
		return true
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		subschemas, _ := schema[keyword].([]any)
		for _, s := range subschemas {
			if sub, ok := s.(map[string]any); ok && isURLSchema(sub) {
				return true
			}
		}
	}
	return false
}

// collectURLParams walks a parameter value alongside its JSON Schema and adds
// every string whose schema declares a URL (a URL format or x-wso2-url: true),
// through nested objects and arrays.
func collectURLParams(path string, value any, schema map[string]any, out *[]target) {
	switch v := value.(type) {
	case string:
		if isURLSchema(schema) && strings.TrimSpace(v) != "" {
			*out = append(*out, target{path: path, url: strings.TrimSpace(v)})
		}
	case map[string]any:
		props, _ := schema["properties"].(map[string]any)
		additional, _ := schema["additionalProperties"].(map[string]any)
		for _, key := range slices.Sorted(maps.Keys(v)) {
			child := v[key]
			childSchema, _ := props[key].(map[string]any)
			if childSchema == nil {
				childSchema = additional
			}
			if childSchema != nil {
				collectURLParams(path+"."+key, child, childSchema, out)
			}
		}
	case []any:
		items, _ := schema["items"].(map[string]any)
		if items == nil {
			return
		}
		for i, child := range v {
			collectURLParams(fmt.Sprintf("%s[%d]", path, i), child, items, out)
		}
	}
}

// dialableScheme reports whether Envoy or a policy's HTTP client would dial a
// URL with this scheme.
func dialableScheme(scheme string) bool {
	s := strings.ToLower(scheme)
	return s == "http" || s == "https"
}

// checkURL refuses a URL whose scheme, hostname or address access control does
// not permit. A hostname that does not resolve here is not refused: the
// controller's DNS view need not match the router's, and a backend often comes
// up after the artifact that points at it. Envoy re-resolves it on its own, so
// only its addresses at deploy (and restart) time are checked.
func (g *Guard) checkURL(path, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		// Malformed URLs are the validators' to report.
		return nil
	}
	if !dialableScheme(parsed.Scheme) {
		// Nothing dials a scheme other than http or https, and the validators
		// refuse it with a message that names the problem. allowed_schemes can
		// still narrow http and https below.
		return nil
	}
	host := parsed.Hostname()

	// raw has parsed with a host, so any error CheckURL returns is a refusal.
	if err := g.policy.CheckURL(context.Background(), raw, false); errors.Is(err, netguard.ErrDestinationNotAllowed) {
		g.logger.Warn("Artifact URL refused by network access control",
			slog.String("field", path), slog.String("host", host), slog.String("reason", err.Error()))
		return &RefusedError{FieldPath: path}
	}

	ctx, cancel := context.WithTimeout(context.Background(), g.lookupTimeout)
	defer cancel()
	if err := netguard.Validate(ctx, g.addrPolicy, host); err != nil {
		if errors.Is(err, netguard.ErrDestinationNotAllowed) {
			g.logger.Warn("Artifact URL refused by network access control: host resolves to a refused address",
				slog.String("field", path), slog.String("host", host))
			return &RefusedError{FieldPath: path}
		}
		g.logger.Warn("Artifact URL host did not resolve at deploy time; its addresses were not checked",
			slog.String("field", path), slog.String("host", host))
	}
	return nil
}
