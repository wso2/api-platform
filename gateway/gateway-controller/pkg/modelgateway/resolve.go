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

// Package modelgateway resolves a Model against the artifacts it refers to.
//
// A Model names its targets by provider handle and leaves everything else to the
// gateway: how each provider is reached, which credential is presented, and how
// the routing policy's parameters name the targets. Resolving that is the same
// work whether a Model is being saved (where a target the gateway cannot serve
// must be refused with a reason) or compiled into an endpoint (where a Model that
// no longer resolves is left out). Both callers use Resolve, so the two cannot
// disagree about what a valid Model is.
//
// Which transformer a provider gets is not part of the Model: it depends on the
// endpoint serving it, whose inbound format the request arrives in. Bind decides
// that, once per endpoint, from the endpoint's own transformer entries.
//
// The package holds no state and builds no policy instances; it decides, and the
// compiler (pkg/transform) builds from the decision.
package modelgateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ErrProviderNotFound reports that a target names a provider the gateway does not
// have. Lookup implementations wrap it so a caller can tell a missing provider
// from a failed lookup.
var ErrProviderNotFound = errors.New("provider not found")

// Provider is one provider as a Model target sees it.
type Provider struct {
	UUID         string
	DesiredState models.DesiredState

	// Config is the provider with its template expressions rendered: its upstream
	// credential is the real value, so a direct target can present it.
	Config api.LLMProviderConfiguration

	// SensitiveValues are the plaintext secrets rendering produced, for log
	// redaction wherever the credential travels.
	SensitiveValues []string
}

// Lookup supplies the artifacts a Model refers to.
type Lookup interface {
	// Provider returns the provider with the given handle, or an error wrapping
	// ErrProviderNotFound.
	Provider(handle string) (*Provider, error)
	// Template returns the provider template with the given handle.
	Template(handle string) (*models.StoredLLMProviderTemplate, error)
	// PolicyDefinition returns the loaded definition of a policy at a major
	// version ("v1").
	PolicyDefinition(name, majorVersion string) (*models.PolicyDefinition, error)
}

// Options carries the gateway configuration resolution depends on.
type Options struct {
	// DefaultVhost is the router's default main vhost: the vhost a provider that
	// names none of its own is served on.
	DefaultVhost string

	// Strict refuses configuration that is harmless but misleading. A save is
	// strict, so an author is told; the compiler is not, so a Model that was valid
	// when saved keeps serving after a change elsewhere made part of it moot — a
	// target credential on a provider whose exposure has since become `none`, which
	// the target no longer presents.
	Strict bool
}

// TargetMode is how a model endpoint reaches a target's provider.
type TargetMode string

const (
	// ModeDirect calls the provider's upstream itself, with the provider's own
	// upstream credential: one hop. Chosen for a provider with `exposure: none`.
	ModeDirect TargetMode = "direct"
	// ModeRoute calls the provider's own route over the gateway's loopback, so the
	// provider's policies apply: two hops, with the target's credential presented
	// to the provider's route. Chosen for a provider with `exposure: route`.
	ModeRoute TargetMode = "route"
)

// Target is one resolved Model target.
type Target struct {
	Name          string
	UpstreamModel string

	ProviderHandle string
	Provider       *Provider
	Template       *models.StoredLLMProviderTemplate

	Mode TargetMode

	// RouteAuth is the credential presented to a routed target's provider route —
	// the target's own auth block, rendered. Nil for a direct target, and for a
	// routed one whose provider route needs none.
	RouteAuth *api.LLMUpstreamAuth

	// RouteValuePrefix is the valuePrefix the provider route's api-key-auth strips,
	// which the route credential must carry.
	RouteValuePrefix string

	// RouteContext is the routed provider's context, the base path of the
	// loopback upstream.
	RouteContext string

	// RouteHostname is the Host the loopback request must carry to reach the
	// provider's route: its vhost, or the gateway's default vhost. Empty when that
	// vhost is "*", so any Host reaches it.
	RouteHostname string

	// HeaderPhaseCredential reports that the credential this target presents is
	// applied in the request-header phase, before a body-phase router has chosen.
	HeaderPhaseCredential bool
}

// Routing is a Model's resolved routing policy.
type Routing struct {
	PolicyName    string
	PolicyVersion string // major, as authored
	Router        RouterInfo

	// Params are the router's parameters, as the Model gives them.
	Params map[string]interface{}
}

// Model is a resolved Model.
type Model struct {
	Handle      string
	ModelName   string
	DisplayName string
	Targets     []Target
	Routing     *Routing // nil for a single-target Model without one
}

// ProviderHandles returns the distinct providers the Model's targets are on, in
// target order.
func (m *Model) ProviderHandles() []string {
	seen := make(map[string]bool, len(m.Targets))
	out := make([]string, 0, len(m.Targets))
	for _, t := range m.Targets {
		if !seen[t.ProviderHandle] {
			seen[t.ProviderHandle] = true
			out = append(out, t.ProviderHandle)
		}
	}
	return out
}

// Errors is every reason a Model did not resolve.
type Errors []config.ValidationError

func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for i, ve := range e {
		parts = append(parts, fmt.Sprintf("%d. %s: %s", i+1, ve.Field, ve.Message))
	}
	return fmt.Sprintf("%d error(s): %s", len(e), strings.Join(parts, "; "))
}

// Resolve resolves a Model against the artifacts it refers to. The Model must
// already have passed the stateless Model validator; Resolve checks only what
// needs the other artifacts.
//
// On failure it returns Errors listing every problem found, so a save reports all
// of them at once.
func Resolve(cfg *api.ModelConfiguration, lookup Lookup, opts Options) (*Model, error) {
	var errs Errors
	add := func(field, format string, args ...interface{}) {
		errs = append(errs, config.ValidationError{Field: field, Message: fmt.Sprintf(format, args...)})
	}

	model := &Model{
		Handle:      cfg.Metadata.Name,
		ModelName:   cfg.Spec.ModelName,
		DisplayName: cfg.Spec.DisplayName,
	}

	for i, spec := range cfg.Spec.Targets {
		field := fmt.Sprintf("spec.targets[%d]", i)
		target, targetErrs := resolveTarget(field, spec, lookup, opts)
		errs = append(errs, targetErrs...)
		if target == nil {
			continue
		}
		model.Targets = append(model.Targets, *target)
	}

	// One provider is one group of provider-specific policies in the Model's chain,
	// so it presents one credential. Two targets on one provider route that carry
	// different credentials would leave the group to pick one of them silently.
	credentialByProvider := make(map[string]string, len(model.Targets))
	for i, t := range model.Targets {
		if t.Mode != ModeRoute {
			continue
		}
		key := routeCredentialKey(t.RouteAuth)
		if first, seen := credentialByProvider[t.ProviderHandle]; seen && first != key {
			add(fmt.Sprintf("spec.targets[%d].auth", i), "every target on provider '%s' must present the same "+
				"credential: the provider's route is called with one", t.ProviderHandle)
			continue
		}
		credentialByProvider[t.ProviderHandle] = key
	}

	if cfg.Spec.Routing != nil {
		routing, err := resolveRouting(cfg.Spec.Routing, lookup)
		if err != nil {
			add("spec.routing", "%s", err.Error())
		} else {
			model.Routing = routing
		}
	} else if len(cfg.Spec.Targets) > 1 {
		add("spec.routing", "a Model with more than one target needs a routing policy to choose between them")
	}

	// A credential applied in the request-header phase is gated on the provider
	// already chosen at that point. When the provider is only known after a
	// body-phase router has run, the gate is evaluated against no choice at all and
	// the credential is never applied — so the combination is refused here rather
	// than discovered as a 401 from the provider.
	if model.Routing != nil && model.Routing.Router.Phase == PhaseBody && len(model.ProviderHandles()) > 1 {
		for i, t := range model.Targets {
			if t.HeaderPhaseCredential {
				add(fmt.Sprintf("spec.targets[%d].provider.id", i),
					"provider '%s' authenticates upstream in the request-header phase (oauth2), but routing policy "+
						"'%s' chooses between providers from the request body, after that phase: the credential could "+
						"not be applied. Use a header-phase router, or put this Model's targets on one provider",
					t.ProviderHandle, model.Routing.PolicyName)
			}
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return model, nil
}

func resolveTarget(field string, spec api.ModelTarget, lookup Lookup, opts Options) (*Target, Errors) {
	var errs Errors
	add := func(f, format string, args ...interface{}) {
		errs = append(errs, config.ValidationError{Field: f, Message: fmt.Sprintf(format, args...)})
	}

	provider, err := lookup.Provider(spec.Provider.Id)
	if err != nil {
		if errors.Is(err, ErrProviderNotFound) {
			add(field+".provider.id", "provider '%s' not found", spec.Provider.Id)
		} else {
			add(field+".provider.id", "provider '%s' could not be read", spec.Provider.Id)
		}
		return nil, errs
	}
	if provider.DesiredState != models.StateDeployed {
		add(field+".provider.id", "provider '%s' is not deployed", spec.Provider.Id)
		return nil, errs
	}
	pcfg := &provider.Config

	template, err := lookup.Template(pcfg.Spec.Template)
	if err != nil || template == nil {
		add(field+".provider.id", "provider '%s' uses template '%s', which could not be found",
			spec.Provider.Id, pcfg.Spec.Template)
		return nil, errs
	}
	target := &Target{
		Name:           spec.Name,
		UpstreamModel:  spec.UpstreamModel,
		ProviderHandle: spec.Provider.Id,
		Provider:       provider,
		Template:       template,
	}

	if !models.IsLLMProviderExposed(pcfg.Spec) {
		target.Mode = ModeDirect
		// The target is called with the provider's own upstream credential; a second
		// credential on the target would either be ignored or shadow it, and both
		// read as configuration that works when it does not.
		if spec.Auth != nil && opts.Strict {
			add(field+".auth", "a target on provider '%s' (exposure: none) is called with the provider's own "+
				"upstream credential and must not set auth", spec.Provider.Id)
		}
		if auth := pcfg.Spec.Upstream.Auth; auth != nil {
			target.HeaderPhaseCredential = isHeaderPhaseCredential(string(auth.Type), auth.PolicyName)
		}
		return target, errs
	}

	target.Mode = ModeRoute
	ctx := "/"
	if pcfg.Spec.Context != nil && strings.TrimSpace(*pcfg.Spec.Context) != "" {
		ctx = strings.ReplaceAll(strings.TrimSpace(*pcfg.Spec.Context), "$version", pcfg.Spec.Version)
	}
	target.RouteContext = ctx

	hostname, err := routeHostname(pcfg, opts.DefaultVhost)
	if err != nil {
		add(field+".provider.id", "%s", err.Error())
	}
	target.RouteHostname = hostname

	// The provider route's own inbound authentication decides what the target must
	// present on the loopback hop.
	inbound := inboundAuthOf(pcfg)
	switch {
	case inbound.unsupported != "":
		add(field+".provider.id", "provider '%s' authenticates its route with '%s', which a Model cannot present a "+
			"credential for; give the provider api-key-auth, or set its exposure to 'none'", spec.Provider.Id, inbound.unsupported)
	case inbound.apiKey:
		target.RouteValuePrefix = inbound.valuePrefix
		if spec.Auth == nil || spec.Auth.Type == api.LLMUpstreamAuthTypeNone {
			add(field+".auth", "provider '%s' authenticates its route with api-key-auth, so the target must set auth "+
				"with type 'api-key' carrying a key issued for that provider", spec.Provider.Id)
		} else if spec.Auth.Type != api.LLMUpstreamAuthTypeApiKey {
			add(field+".auth.type", "provider '%s' authenticates its route with api-key-auth, so the target's auth type "+
				"must be 'api-key'", spec.Provider.Id)
		} else {
			target.RouteAuth = spec.Auth
		}
	default:
		// No inbound authentication: a credential is optional, and `none` is the
		// same as leaving it out.
		if spec.Auth != nil && spec.Auth.Type != api.LLMUpstreamAuthTypeNone {
			target.RouteAuth = spec.Auth
		}
	}
	if target.RouteAuth != nil {
		target.HeaderPhaseCredential = isHeaderPhaseCredential(string(target.RouteAuth.Type), target.RouteAuth.PolicyName)
	}

	// The paths a routed target is called on depend on the endpoint's transformer
	// for this provider, so they are checked against the route's access control when
	// the Model is bound to an endpoint (Bind), not here.
	return target, errs
}

// routeCredentialKey identifies the credential a routed target presents.
func routeCredentialKey(a *api.LLMUpstreamAuth) string {
	if a == nil {
		return ""
	}
	parts := []string{string(a.Type)}
	for _, p := range []*string{a.Header, a.Value, a.PolicyName, a.PolicyVersion} {
		if p == nil {
			parts = append(parts, "")
		} else {
			parts = append(parts, *p)
		}
	}
	if a.PolicyParams != nil {
		raw, _ := json.Marshal(*a.PolicyParams) // map keys marshal sorted
		parts = append(parts, string(raw))
	}
	return strings.Join(parts, "\x00")
}

// isHeaderPhaseCredential reports whether a credential is applied in the request
// header phase only. oauth2-generator is: it acquires a token from the request
// headers alone. The default api-key credential (set-headers) is moved to the body
// phase by the compiler, and aws-authentication signs the body.
func isHeaderPhaseCredential(authType string, policyName *string) bool {
	if policyName != nil && strings.TrimSpace(*policyName) == oauth2GeneratorPolicyName {
		return true
	}
	return authType == string(api.LLMUpstreamAuthTypeOauth2) && (policyName == nil || strings.TrimSpace(*policyName) == "")
}

const (
	oauth2GeneratorPolicyName = "oauth2-generator"
	apiKeyAuthPolicyName      = "api-key-auth"
)

// unsupportedInboundAuthPolicies authenticate a provider route with a credential
// a model endpoint has no way to produce on the loopback hop.
var unsupportedInboundAuthPolicies = map[string]bool{
	"jwt-auth":                true,
	"basic-auth":              true,
	"mtls-auth":               true,
	"opaque-token-auth":       true,
	"subscription-validation": true,
}

type inboundAuth struct {
	apiKey      bool
	valuePrefix string
	unsupported string
}

// inboundAuthOf reports how a provider's route authenticates, from its global and
// operation policies.
func inboundAuthOf(p *api.LLMProviderConfiguration) inboundAuth {
	var out inboundAuth
	check := func(name string, params *map[string]interface{}) {
		switch {
		case name == apiKeyAuthPolicyName:
			out.apiKey = true
			if params != nil {
				if v, ok := (*params)["valuePrefix"].(string); ok && out.valuePrefix == "" {
					out.valuePrefix = strings.TrimSpace(v)
				}
			}
		case unsupportedInboundAuthPolicies[name] && out.unsupported == "":
			out.unsupported = name
		}
	}
	if p.Spec.GlobalPolicies != nil {
		for _, pol := range *p.Spec.GlobalPolicies {
			check(pol.Name, pol.Params)
		}
	}
	if p.Spec.OperationPolicies != nil {
		for _, pol := range *p.Spec.OperationPolicies {
			for _, path := range pol.Paths {
				params := path.Params
				check(pol.Name, &params)
			}
		}
	}
	if p.Spec.Policies != nil {
		for _, pol := range *p.Spec.Policies {
			for _, path := range pol.Paths {
				params := path.Params
				check(pol.Name, &params)
			}
		}
	}
	return out
}

// routeHostname returns the Host a loopback request to a provider's route must
// carry. A provider's route is served on its vhost, or the gateway's default one;
// the loopback endpoint names that host so the router rewrites Host to it.
//
// "*" needs no name: any Host reaches it. A wildcard pattern ("*.example.com")
// names no single host the loopback could carry, so such a provider is refused as
// a routed target rather than sent a Host it might not serve.
func routeHostname(p *api.LLMProviderConfiguration, defaultVhost string) (string, error) {
	vhost := strings.TrimSpace(defaultVhost)
	if p.Spec.Vhost != nil && strings.TrimSpace(*p.Spec.Vhost) != "" {
		vhost = strings.TrimSpace(*p.Spec.Vhost)
	}
	switch {
	case vhost == "" || vhost == "*":
		return "", nil
	case strings.Contains(vhost, "*"):
		return "", fmt.Errorf("provider '%s' is served on the wildcard vhost '%s', which names no single host a "+
			"Model can reach it on; give it a concrete vhost, or set its exposure to 'none'", p.Metadata.Name, vhost)
	default:
		return vhost, nil
	}
}

// accessControlAllows reports whether a provider route's access control lets a
// method and path through, by the same rules its routes are generated from:
// allow_all forwards everything but its exceptions, deny_all only its exceptions.
// A path exception may use `{param}` for one segment and a trailing `*` for
// anything below a prefix.
func accessControlAllows(ac *api.LLMAccessControl, method, path string) bool {
	if ac == nil {
		return false
	}
	matched := false
	if ac.Exceptions != nil {
		for _, ex := range *ac.Exceptions {
			if exceptionMatches(ex, method, path) {
				matched = true
				break
			}
		}
	}
	switch ac.Mode {
	case api.AllowAll:
		return !matched
	case api.DenyAll:
		return matched
	default:
		return false
	}
}

func exceptionMatches(ex api.RouteException, method, path string) bool {
	methodOK := false
	for _, m := range ex.Methods {
		if strings.EqualFold(string(m), method) || string(m) == "*" {
			methodOK = true
			break
		}
	}
	if !methodOK {
		return false
	}
	re, err := exceptionPathPattern(ex.Path)
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

var paramSegment = regexp.MustCompile(`\{[^}/]+\}`)

// exceptionPathPattern compiles an access-control path into an anchored pattern.
func exceptionPathPattern(p string) (*regexp.Regexp, error) {
	if p == "/*" || p == "*" {
		return regexp.Compile(`^/.*$`)
	}
	var b strings.Builder
	b.WriteString("^")
	rest := p
	wildcard := strings.HasSuffix(rest, "*")
	if wildcard {
		rest = strings.TrimSuffix(rest, "*")
	}
	last := 0
	for _, loc := range paramSegment.FindAllStringIndex(rest, -1) {
		b.WriteString(regexp.QuoteMeta(rest[last:loc[0]]))
		b.WriteString(`[^/]+`)
		last = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(rest[last:]))
	if wildcard {
		b.WriteString(".*")
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func resolveRouting(spec *api.ModelRouting, lookup Lookup) (*Routing, error) {
	name := strings.TrimSpace(spec.Policy.Name)
	version := strings.TrimSpace(spec.Policy.Version)
	def, err := lookup.PolicyDefinition(name, version)
	if err != nil || def == nil {
		return nil, fmt.Errorf("routing policy '%s' %s is not loaded in this gateway", name, version)
	}
	info, ok := RouterInfoFor(name, def)
	if !ok {
		return nil, fmt.Errorf("policy '%s' is not a model routing policy: it is not one of the gateway's routers "+
			"and declares no %s extension", name, ModelRoutingSchemaExtension)
	}

	// Copied, so the compiled chain never aliases the stored Model's map.
	params := map[string]interface{}{}
	if spec.Params != nil {
		for k, v := range *spec.Params {
			params[k] = v
		}
	}
	return &Routing{PolicyName: name, PolicyVersion: version, Router: info, Params: params}, nil
}

// TargetNames returns the Model's target names, sorted.
func (m *Model) TargetNames() []string {
	names := make([]string, 0, len(m.Targets))
	for _, t := range m.Targets {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names
}
