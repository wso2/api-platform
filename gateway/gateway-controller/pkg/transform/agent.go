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

package transform

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/agentproto"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/xds"
	policyv1alpha "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	policyenginev1 "github.com/wso2/api-platform/sdk/core/policyengine"
)

// agentCardRouteMethod is the HTTP method the public Agent Card is served under.
// Card discovery is a plain read.
const agentCardRouteMethod = "GET"

// agentPreflightMethod is the method of the CORS preflight routes generated
// beside the real ones. Envoy needs a route to match the preflight against
// before any policy — including the cors policy that answers it — ever runs.
const agentPreflightMethod = "OPTIONS"

// corsPolicyName is the policy whose presence in a scope makes that scope's
// paths need an OPTIONS route. Compared case-insensitively, as the MCP
// transformer does.
const corsPolicyName = "cors"

// maxAgentJSONRPCRequestBodyBytes bounds the JSON-RPC request body the policy
// engine will accept when resolving which A2A operation a request is for.
//
// The single JSON-RPC endpoint carries its operation in the body, so that body
// is buffered and parsed before any policy in the chain — authentication
// included — has run. This ceiling is what bounds the unauthenticated parsing
// work an anonymous caller can ask for on that one route. It is an acceptance
// ceiling rather than a buffering one: Envoy has already buffered the body by
// the time the engine checks it, and the memory a caller can make the gateway
// hold is bounded listener-wide by per_connection_buffer_limit_bytes.
//
// The value is the same boundary the managed Agent Card is capped at — 1 MiB,
// the largest single object Kubernetes stores by default. It is deliberately
// not an operator knob: raising it would admit a request the platform around
// the gateway cannot carry anyway, and lowering it would only move a rejection
// earlier without changing what an attacker can attempt.
//
// HTTP+JSON routes are unaffected. They identify their operation from the path
// and buffer nothing for resolution.
const maxAgentJSONRPCRequestBodyBytes = 1024 * 1024

// disabledRouteTimeout is the resilience value that switches the Envoy route
// timeout off, as opposed to leaving it unset (which selects the gateway
// default). Spelled as the string the resilience block takes so it goes through
// the same parser as a user-supplied value.
const disabledRouteTimeout = "0s"

// AgentTransformer turns a stored Agent (A2A) artifact into a RuntimeDeployConfig.
//
// It builds the RDC directly rather than desugaring an Agent into a synthetic
// api.RestAPI the way MCP does. The reason is the shape of A2A itself: two
// transports address the *same* eleven operations, one of them from a single
// endpoint that names its operation in the request body. A REST API's route and
// its policy chain are the same thing, so a REST-shaped intermediate cannot
// express "eleven chains, one route" — building one and then patching it would
// mean maintaining the patch, not reusing the model.
//
// What it does reuse is every helper that decides a value some other component
// also computes: upstream cluster construction, resilience parsing, policy
// version resolution, system-policy injection, route naming, and the Agent path
// arithmetic the validator checked for collisions.
//
// # Paths
//
// Every route sets Path (gateway-facing, absolute) and OperationPath (the same
// path with spec.context removed). The translator rewrites Path minus
// OperationPath — the context — onto the upstream's base path, so OperationPath
// is exactly the part that reaches the upstream.
//
// spec.context is the only gateway-local segment. A transport's pathPrefix is
// not: it describes where the agent serves that protocol binding, and the
// gateway mirrors that layout, so it belongs to OperationPath and travels
// upstream with it. An Agent at context /weather with a JSONRPC prefix of /rpc
// and an upstream of https://host/a2a/v1 therefore answers POST /weather/rpc and
// forwards it to https://host/a2a/v1/rpc.
type AgentTransformer struct {
	routerConfig      *config.RouterConfig
	systemConfig      *config.Config
	policyDefinitions map[string]models.PolicyDefinition
	latestVersions    map[string]string // pre-computed policyName -> latest full semver
}

// NewAgentTransformer creates a new AgentTransformer.
func NewAgentTransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) *AgentTransformer {
	return &AgentTransformer{
		routerConfig:      routerConfig,
		systemConfig:      systemConfig,
		policyDefinitions: policyDefinitions,
		latestVersions:    config.BuildLatestVersionIndex(policyDefinitions),
	}
}

// agentTransport is one declared transport after its path prefix has been
// resolved against the Agent's context.
type agentTransport struct {
	binding api.A2AProtocolBinding
	// basePath is the gateway-facing base: spec.context joined with the prefix.
	basePath string
	// relativePath is the same base with spec.context removed — the prefix,
	// normalised. It is the operation-path root every route under this transport
	// hangs off, and therefore the part that survives onto the upstream.
	relativePath string
}

// Transform converts a StoredConfig holding an Agent into a RuntimeDeployConfig.
func (t *AgentTransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	agentCfg, err := agentConfiguration(cfg)
	if err != nil {
		return nil, err
	}
	spec := agentCfg.Spec
	a2a := spec.A2a

	// The protocol version selects the operation set every chain below is keyed
	// by. An unregistered one is refused outright rather than defaulted: an
	// Agent silently deployed against another version's operations would
	// enforce policies its own Agent Card does not advertise.
	protocolVersion := agentproto.ProtocolVersion(a2a.ProtocolVersion)
	operations, ok := agentproto.Operations(protocolVersion)
	if !ok {
		return nil, fmt.Errorf("unsupported A2A protocol version %q", a2a.ProtocolVersion)
	}

	agentContext := config.AgentContextPath(spec.Context)

	rdc := &models.RuntimeDeployConfig{
		Metadata: models.Metadata{
			UUID:        cfg.UUID,
			Kind:        models.KindAgent,
			Handle:      cfg.Handle,
			Version:     spec.Version,
			DisplayName: spec.DisplayName,
			ProjectID:   extractProjectID(cfg),
		},
		Context: agentContext,
		// The RDC-level default covers the routes that are not A2A operations:
		// the public Agent Card and any CORS preflight. Operation routes name
		// the a2a resolver themselves.
		PolicyChainResolver: models.RouteKeyResolverName,
		Routes:              make(map[string]*models.Route),
		PolicyChains:        make(map[string]*models.PolicyChain),
		UpstreamClusters:    make(map[string]*models.UpstreamCluster),
		SensitiveValues:     cfg.SensitiveValues,
	}

	upstream, err := addUpstreamCluster(rdc, "main", agentUpstream(&spec.Upstream), spec.UpstreamDefinitions)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve upstream: %w", err)
	}
	upstreamInfo := upstream.UpstreamInfo()

	// An Agent has no sandbox slot, so cluster-header routing is needed only when
	// named upstream definitions exist for a policy to select between. This must
	// agree with pkg/xds/translator.go's own computation or Envoy waits on a
	// header the policy engine never sets.
	useClusterHeader := spec.UpstreamDefinitions != nil && len(*spec.UpstreamDefinitions) > 0
	defaultCluster := ""
	if useClusterHeader {
		// ClusterKey, not EnvoyClusterName: the translator names clusters by the
		// UpstreamClusters map key, so anything else points at no cluster.
		defaultCluster = upstream.ClusterKey
	}

	autoHostRewrite := spec.Upstream.HostRewrite == nil ||
		*spec.Upstream.HostRewrite != api.AgentConfigDataUpstreamHostRewriteManual

	agentTimeout, agentIdleTimeout, err := xds.ResolveResilience(spec.Resilience)
	if err != nil {
		return nil, fmt.Errorf("invalid agent-level resilience: %w", err)
	}

	transports, err := resolveAgentTransports(agentContext, a2a.OperationConfigs.Transports)
	if err != nil {
		return nil, err
	}

	passthroughCard := a2a.AgentCard.Public.Mode == api.A2APublicAgentCardModePassthrough

	// Upstream credential injection. It rides in every chain whose requests
	// actually reach the upstream — all operation chains, and the card chain
	// only when the card is proxied — rather than being attached once at the
	// artifact level, because a managed card is answered by the gateway and the
	// credential has no business travelling in a chain that never forwards.
	upstreamAuth, err := agentUpstreamAuthPolicy(spec.Upstream.Auth)
	if err != nil {
		return nil, err
	}

	// Operation policies, resolved once and shared by every partition. The chain
	// order is system policies, then the operation-common policies, then the
	// selected operation's own: plain concatenation, no dedup and no override,
	// so a policy attached at both levels runs twice as two instances.
	commonOperationPolicies := resolvePolicyInstances(
		t.policyDefinitions, t.latestVersions, withPolicy(a2a.OperationConfigs.Policies, upstreamAuth), policyv1alpha.LevelAPI)
	perOperationPolicies := make(map[agentproto.Operation][]policyenginev1.PolicyInstance)
	perOperationResilience := make(map[agentproto.Operation]*api.Resilience)
	if a2a.OperationConfigs.Operations != nil {
		for i := range *a2a.OperationConfigs.Operations {
			opCfg := (*a2a.OperationConfigs.Operations)[i]
			operation := agentproto.Operation(opCfg.Name)
			perOperationPolicies[operation] = resolvePolicyInstances(
				t.policyDefinitions, t.latestVersions, opCfg.Policies, policyv1alpha.LevelRoute)
			perOperationResilience[operation] = opCfg.Resilience
		}
	}

	cardPolicySource := a2a.AgentCard.Public.Policies
	if passthroughCard {
		cardPolicySource = withPolicy(cardPolicySource, upstreamAuth)
	}
	cardPolicies := resolvePolicyInstances(
		t.policyDefinitions, t.latestVersions, cardPolicySource, policyv1alpha.LevelAPI)

	// A preflight is a property of a path, not of an operation: the browser
	// sends OPTIONS before it knows which operation the real request will
	// resolve to, and on the JSON-RPC endpoint every operation shares one path.
	// So the trigger is "cors is attached anywhere in this scope", and the
	// preflight chain carries the scope's common policies only — running a
	// single operation's authentication against a preflight would reject it.
	operationCORS := hasCORSPolicy(a2a.OperationConfigs.Policies) ||
		hasPerOperationCORSPolicy(a2a.OperationConfigs.Operations)
	cardCORS := hasCORSPolicy(a2a.AgentCard.Public.Policies)

	vhosts := agentVhosts(spec.Vhost, t.routerConfig.VHosts.Main.Default)

	routeOrder := 0
	nextOrder := func() int {
		order := routeOrder
		routeOrder++
		return order
	}

	for _, vhost := range vhosts {
		// Canonical chains are built here — inside the partition loop and
		// outside the transport loop — because a chain belongs to an operation
		// in a routing partition, not to the route that happened to carry the
		// request. Building them per transport would have the second transport
		// overwrite the first's chains, silently, with the last one to be
		// generated winning.
		for _, operation := range operations {
			chainKey := rdc.ChainKeyFor(vhost, string(operation))
			if _, exists := rdc.PolicyChains[chainKey]; exists {
				return nil, fmt.Errorf("policy chain %q for operation %q would be built twice", chainKey, operation)
			}
			chain := make([]policyenginev1.PolicyInstance, 0,
				len(commonOperationPolicies)+len(perOperationPolicies[operation]))
			chain = append(chain, commonOperationPolicies...)
			chain = append(chain, perOperationPolicies[operation]...)
			rdc.PolicyChains[chainKey] = sdkChainToModel(utils.InjectSystemPolicies(chain, t.systemConfig, nil))
		}

		routeUpstream := models.RouteUpstream{
			ClusterKey:       upstream.ClusterKey,
			UseClusterHeader: useClusterHeader,
			DefaultCluster:   defaultCluster,
			Default:          &upstreamInfo,
		}

		// addRoute registers a route under its generated key and hands the key
		// back, so a caller that also needs to key a chain by it cannot spell
		// the key a second, subtly different way.
		addRoute := func(route *models.Route) (string, error) {
			routeKey := agentRouteKey(route.Method, route.Path, vhost)
			if _, exists := rdc.Routes[routeKey]; exists {
				return "", fmt.Errorf("route %q is generated twice", routeKey)
			}
			route.Vhost = vhost
			route.AutoHostRewrite = autoHostRewrite
			route.Upstream = routeUpstream
			route.Order = nextOrder()
			rdc.Routes[routeKey] = route
			return routeKey, nil
		}

		// preflightPaths collects the paths that need an OPTIONS route, in
		// generation order, deduplicated: two operations may share a path (POST
		// and GET on pushNotificationConfigs), and one preflight route answers
		// for both.
		var preflightPaths []string
		seenPreflight := make(map[string]struct{})
		addPreflight := func(path string) {
			if _, seen := seenPreflight[path]; seen {
				return
			}
			seenPreflight[path] = struct{}{}
			preflightPaths = append(preflightPaths, path)
		}

		for _, transport := range transports {
			switch transport.binding {
			case api.JSONRPC:
				// One route for every operation. The operation is in the body,
				// so this route names the resolver and carries no canonical
				// chain key — the resolver composes one per request.
				resolverConfig, err := agentResolverConfig(protocolVersion, agentproto.TransportJSONRPC, "")
				if err != nil {
					return nil, err
				}
				if _, err := addRoute(&models.Route{
					Method: "POST",
					Path:   transport.basePath,
					// The transport prefix is the whole of this route's operation
					// path: the JSON-RPC endpoint sits at the prefix and names its
					// operation in the body. Only spec.context is gateway-local, so
					// only spec.context is stripped before the upstream base path
					// is prepended.
					OperationPath: transport.relativePath,
					PathMatchType: "Exact",
					// Every operation shares this route, including the streaming
					// ones, so a finite route timeout would sever a healthy
					// stream. Disabled unless the Agent asks for one; idleTimeout
					// stays the liveness guard.
					Timeout:             agentRouteTimeout(disabledTimeoutResilience(spec.Resilience), agentTimeout, agentIdleTimeout),
					ResolverName:        agentproto.ResolverName,
					ResolverConfig:      resolverConfig,
					MaxRequestBodyBytes: maxAgentJSONRPCRequestBodyBytes,
				}); err != nil {
					return nil, err
				}
				if operationCORS {
					addPreflight(transport.basePath)
				}

			case api.HTTPJSON:
				for _, operation := range operations {
					bindings, ok := agentproto.HTTPJSONBindings(protocolVersion, operation)
					if !ok {
						return nil, fmt.Errorf("A2A %s defines no HTTP+JSON binding for operation %q",
							protocolVersion, operation)
					}
					// The operation is known from the route, so the resolver
					// answers statically and this route also carries no
					// canonical chain key.
					resolverConfig, err := agentResolverConfig(protocolVersion, agentproto.TransportHTTPJSON, operation)
					if err != nil {
						return nil, err
					}
					timeout := t.httpJSONRouteTimeout(
						operation, perOperationResilience[operation], spec.Resilience, agentTimeout, agentIdleTimeout)
					for _, binding := range bindings {
						path := config.JoinAgentPath(transport.basePath, binding.PathTemplate)
						if _, err := addRoute(&models.Route{
							Method: binding.Method,
							Path:   path,
							// The transport prefix plus the protocol's own path.
							// Both reach the upstream; only spec.context does not.
							OperationPath:  config.JoinAgentPath(transport.relativePath, binding.PathTemplate),
							PathMatchType:  "Exact",
							Timeout:        timeout,
							ResolverName:   agentproto.ResolverName,
							ResolverConfig: resolverConfig,
						}); err != nil {
							return nil, err
						}
						if operationCORS {
							addPreflight(path)
						}
					}
				}
			}
		}

		// The public Agent Card is not an A2A operation: it is a document at a
		// known path, resolved by route identity like any other direct route.
		// What serves it — a proxied fetch from the upstream or a gateway-held
		// document — is decided by the card policy, not here.
		cardRelativePath := agentCardPath(a2a.AgentCard.Public.Path)
		cardPath := config.JoinAgentPath(agentContext, cardRelativePath)
		cardRouteKey, err := addRoute(&models.Route{
			Method:        agentCardRouteMethod,
			Path:          cardPath,
			OperationPath: cardRelativePath,
			PathMatchType: "Exact",
			Timeout:       agentRouteTimeout(spec.Resilience, agentTimeout, agentIdleTimeout),
		})
		if err != nil {
			return nil, err
		}
		rdc.PolicyChains[cardRouteKey] = sdkChainToModel(
			utils.InjectSystemPolicies(cardPolicies, t.systemConfig, nil))
		if cardCORS {
			addPreflight(cardPath)
		}

		for _, path := range preflightPaths {
			preflightKey, err := addRoute(&models.Route{
				Method:        agentPreflightMethod,
				Path:          path,
				OperationPath: strings.TrimPrefix(path, agentContext),
				PathMatchType: "Exact",
			})
			if err != nil {
				return nil, err
			}
			scopePolicies := commonOperationPolicies
			if path == cardPath {
				scopePolicies = cardPolicies
			}
			rdc.PolicyChains[preflightKey] = sdkChainToModel(
				utils.InjectSystemPolicies(scopePolicies, t.systemConfig, nil))
		}
	}

	// Both xDS streams are fed from this RDC independently, so a construction
	// mistake here surfaces as a request-time mystery rather than a deployment
	// error. Catching it at the end of the transform names the route.
	if err := rdc.ValidateResolution(); err != nil {
		return nil, fmt.Errorf("agent %s: %w", cfg.UUID, err)
	}

	return rdc, nil
}

// agentRouteKey names one Agent route.
//
// Every Agent route key goes through here, and here alone. The Envoy route name
// and the policy-xDS resource key must be byte-identical or a matched request
// finds no chain and 500s, and the vhost is parsed back out of the key by
// position — so this is a place where hand-formatting the string once would be
// enough to break routing in a way nothing else reports.
//
// The path is already absolute by the time it arrives: an Agent's context is
// folded into it by the same JoinAgentPath arithmetic the validator used, so
// the context argument is empty and no version placeholder is substituted.
func agentRouteKey(method, path, vhost string) string {
	return xds.GenerateRouteNameWithDiscriminator(method, "", "", path, vhost, "")
}

// agentConfiguration extracts the Agent resource from a StoredConfig, accepting
// the value form the storage layer produces and the pointer form a caller
// holding a freshly parsed configuration may pass.
func agentConfiguration(cfg *models.StoredConfig) (*api.AgentConfiguration, error) {
	switch agentCfg := cfg.Configuration.(type) {
	case api.AgentConfiguration:
		return &agentCfg, nil
	case *api.AgentConfiguration:
		if agentCfg == nil {
			return nil, fmt.Errorf("configuration is a nil Agent")
		}
		return agentCfg, nil
	default:
		return nil, fmt.Errorf("configuration is not an Agent")
	}
}

// agentUpstream adapts the Agent's upstream block to the shared upstream shape,
// so cluster construction runs through the same code as every other kind.
func agentUpstream(up *api.AgentConfigData_Upstream) *api.Upstream {
	shared := &api.Upstream{Url: up.Url, Ref: up.Ref}
	if up.HostRewrite != nil {
		hostRewrite := api.UpstreamHostRewrite(*up.HostRewrite)
		shared.HostRewrite = &hostRewrite
	}
	return shared
}

// agentVhosts resolves the Agent's routing partitions. A vhost value may carry
// several hostnames separated by ";" — the shape a Gateway-API HTTPRoute
// attached to multiple listener hostnames produces — and each is its own
// partition with its own routes and its own operation chains. An unset value
// leaves the Agent on the gateway default.
func agentVhosts(vhost *string, defaultVhost string) []string {
	if vhost == nil {
		return []string{defaultVhost}
	}
	if parsed := splitVhosts(*vhost); len(parsed) > 0 {
		return parsed
	}
	return []string{defaultVhost}
}

// resolveAgentTransports joins each declared transport's path prefix onto the
// Agent's context, reproducing exactly the base paths the validator checked for
// collisions.
func resolveAgentTransports(agentContext string, declared []api.A2ATransport) ([]agentTransport, error) {
	if len(declared) == 0 {
		return nil, fmt.Errorf("agent declares no transports")
	}
	resolved := make([]agentTransport, 0, len(declared))
	for _, transport := range declared {
		switch transport.ProtocolBinding {
		case api.JSONRPC, api.HTTPJSON:
		default:
			return nil, fmt.Errorf("unsupported A2A protocolBinding %q", transport.ProtocolBinding)
		}
		prefix := "/"
		if transport.PathPrefix != nil {
			prefix = *transport.PathPrefix
		}
		resolved = append(resolved, agentTransport{
			binding:  transport.ProtocolBinding,
			basePath: config.JoinAgentPath(agentContext, prefix),
			// Joined against "" rather than taken raw, so the prefix is
			// normalised by the same arithmetic as the gateway-facing path and
			// the two cannot disagree about a trailing slash.
			relativePath: config.JoinAgentPath("", prefix),
		})
	}
	return resolved, nil
}

// agentCardPath is the card's path relative to the Agent's context: the
// configured value, or the location A2A clients probe during discovery.
func agentCardPath(configured *string) string {
	if configured == nil || *configured == "" {
		return config.DefaultAgentCardPath
	}
	return *configured
}

// agentResolverConfig encodes one route's resolver configuration. Every
// resolver-bearing route carries the protocol version, because that is what
// selects the operation table the engine resolves against — a route without it
// would leave the engine guessing which Agent's operation set it belongs to.
func agentResolverConfig(
	version agentproto.ProtocolVersion,
	transport agentproto.Transport,
	operation agentproto.Operation,
) (json.RawMessage, error) {
	encoded, err := json.Marshal(agentproto.ResolverConfig{
		ProtocolVersion: version,
		Transport:       transport,
		Operation:       operation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode a2a resolver config: %w", err)
	}
	return encoded, nil
}

// httpJSONRouteTimeout resolves one HTTP+JSON route's timeouts.
//
// A streaming operation gets the same treatment as the JSON-RPC endpoint: the
// route timeout defaults to disabled, because a finite one would cut a healthy
// stream at an arbitrary point. Everything else falls back to the gateway's
// global default like any other route.
func (t *AgentTransformer) httpJSONRouteTimeout(
	operation agentproto.Operation,
	operationResilience, agentResilience *api.Resilience,
	agentTimeout, agentIdleTimeout *time.Duration,
) *models.RouteTimeout {
	effective := operationResilience
	if effective == nil {
		effective = agentResilience
	}
	if isStreamingOperation(operation) {
		effective = disabledTimeoutResilience(effective)
	}
	opTimeout, opIdleTimeout, err := xds.ResolveResilience(effective)
	if err != nil {
		// Unreachable in practice: validation rejects a malformed duration
		// before an Agent is stored. Falling back to the agent-level values
		// keeps a transform that somehow got here from producing a route with
		// no timeout policy at all.
		return buildRouteTimeout(nil, agentTimeout, nil, agentIdleTimeout)
	}
	return buildRouteTimeout(opTimeout, agentTimeout, opIdleTimeout, agentIdleTimeout)
}

// agentRouteTimeout resolves a non-operation route's timeouts from the
// agent-level resilience block alone.
func agentRouteTimeout(resilience *api.Resilience, agentTimeout, agentIdleTimeout *time.Duration) *models.RouteTimeout {
	timeout, idleTimeout, err := xds.ResolveResilience(resilience)
	if err != nil {
		return buildRouteTimeout(nil, agentTimeout, nil, agentIdleTimeout)
	}
	return buildRouteTimeout(timeout, agentTimeout, idleTimeout, agentIdleTimeout)
}

// disabledTimeoutResilience returns base with the route timeout defaulted to
// disabled. An explicitly configured timeout always wins; the idle timeout is
// carried through untouched, since that is the liveness guard a disabled route
// timeout leaves in place.
func disabledTimeoutResilience(base *api.Resilience) *api.Resilience {
	disabled := disabledRouteTimeout
	resilience := &api.Resilience{Timeout: &disabled}
	if base != nil {
		if base.Timeout != nil {
			resilience.Timeout = base.Timeout
		}
		resilience.IdleTimeout = base.IdleTimeout
	}
	return resilience
}

// isStreamingOperation reports whether an operation's response is a long-lived
// event stream rather than a single reply.
func isStreamingOperation(operation agentproto.Operation) bool {
	return operation == agentproto.SendStreamingMessage || operation == agentproto.SubscribeToTask
}

// agentUpstreamAuthPolicy builds the policy that injects the Agent's upstream
// credential, or nil when the Agent configures none. It mirrors what the MCP
// transformer does with the identically shaped block.
func agentUpstreamAuthPolicy(auth *struct {
	Header *string                             `json:"header,omitempty" yaml:"header,omitempty"`
	Type   api.AgentConfigDataUpstreamAuthType `json:"type" yaml:"type"`
	Value  *string                             `json:"value,omitempty" yaml:"value,omitempty"`
}) (*api.Policy, error) {
	if auth == nil || auth.Type == api.AgentConfigDataUpstreamAuthTypeNone {
		return nil, nil
	}
	if auth.Header == nil || *auth.Header == "" || auth.Value == nil || *auth.Value == "" {
		return nil, nil
	}
	params, err := utils.GetParamsOfPolicy(constants.SET_HEADERS_POLICY_PARAMS, *auth.Header, *auth.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to build upstream auth params: %w", err)
	}
	return &api.Policy{Name: constants.SET_HEADERS_POLICY_NAME, Params: &params}, nil
}

// withPolicy returns base with extra appended, without mutating base. A nil
// extra returns base unchanged, so the caller does not branch.
func withPolicy(base *[]api.Policy, extra *api.Policy) *[]api.Policy {
	if extra == nil {
		return base
	}
	combined := make([]api.Policy, 0, lenPolicies(base)+1)
	if base != nil {
		combined = append(combined, *base...)
	}
	combined = append(combined, *extra)
	return &combined
}

func lenPolicies(policies *[]api.Policy) int {
	if policies == nil {
		return 0
	}
	return len(*policies)
}

// hasCORSPolicy reports whether a scope attaches the cors policy, which is what
// makes that scope's paths need a preflight route.
func hasCORSPolicy(policies *[]api.Policy) bool {
	if policies == nil {
		return false
	}
	for _, policy := range *policies {
		if strings.EqualFold(policy.Name, corsPolicyName) {
			return true
		}
	}
	return false
}

func hasPerOperationCORSPolicy(operations *[]api.A2AOperationConfig) bool {
	if operations == nil {
		return false
	}
	for i := range *operations {
		if hasCORSPolicy((*operations)[i].Policies) {
			return true
		}
	}
	return false
}
