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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2/api-platform/common/agentproto"
	versionutil "github.com/wso2/api-platform/common/version"
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

// agentCardUpstreamPath is where a passthrough card is fetched from, relative to
// the Agent's upstream base path.
//
// It is fixed by the protocol, not by configuration. agentCard.public.path
// chooses where the *gateway* serves the card and says nothing about the
// upstream: an Agent serving its card at /card still has it proxied from the
// upstream's standard well-known location. A custom or legacy upstream card path
// is not supported.
const agentCardUpstreamPath = config.DefaultAgentCardPath

// agentPreflightMethod is the method of the CORS preflight routes generated
// beside the real ones. Envoy needs a route to match the preflight against
// before any policy — including the cors policy that answers it — ever runs.
const agentPreflightMethod = "OPTIONS"

// corsPolicyName is the policy whose presence in a scope makes that scope's
// paths need an OPTIONS route. Compared case-insensitively, as the MCP
// transformer does.
const corsPolicyName = "cors"

// maxAgentResolvedRequestBodyBytes bounds the request body the policy engine
// will accept on an Agent route that is resolved from its body.
//
// Two kinds of route are: the single JSON-RPC endpoint, which carries its
// operation in the body, and the two HTTP+JSON message-sending routes, whose
// operation is fixed by the route but whose message identifiers exist nowhere
// else. On both, the body is buffered and parsed before any policy in the
// chain — authentication included — has run, so this ceiling is what bounds the
// unauthenticated parsing work an anonymous caller can ask for. It is an
// acceptance ceiling rather than a buffering one: Envoy has already buffered
// the body by the time the engine checks it, and the memory a caller can make
// the gateway hold is bounded listener-wide by
// per_connection_buffer_limit_bytes.
//
// The value is the same boundary the managed Agent Card is capped at — 1 MiB,
// the largest single object Kubernetes stores by default. It is deliberately
// not an operator knob: raising it would admit a request the platform around
// the gateway cannot carry anyway, and lowering it would only move a rejection
// earlier without changing what an attacker can attempt.
//
// Setting it explicitly on every body-resolved route is required, not
// decorative: a route that resolves from its body and carries no limit falls
// back to the engine's DefaultMaxResolverRequestBodyBytes, which is 64 KiB —
// small enough to reject a legitimate A2A message carrying a file part.
//
// The other nine HTTP+JSON routes are unaffected. They identify their operation
// and their task from the path, and buffer nothing for resolution.
const maxAgentResolvedRequestBodyBytes = 1024 * 1024

// operationResolvesFromRequestBody reports whether an HTTP+JSON route for this
// operation is resolved from its body, and therefore needs the ceiling above.
//
// It mirrors carriesMessageInBody in the policy engine's a2a resolver: the
// engine decides which routes buffer, and this decides which routes get a
// bound, so the two must name the same operations. They cannot share a
// constant — separate modules, and the engine's copy is internal/ — so the
// coupling is asserted by a test instead.
func operationResolvesFromRequestBody(operation agentproto.Operation) bool {
	return operation == agentproto.SendMessage || operation == agentproto.SendStreamingMessage
}

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

	// The same operation-common policies without the gateway's own upstream
	// credential. Used by the one chain that answers locally and never forwards:
	// a managed protected Agent Card. Injecting a credential into a request that
	// stops at the gateway would put the Agent's upstream secret into a header
	// mutation nothing consumes. The author's own policies are untouched — only
	// the instance the transformer synthesised is left out.
	//
	// Resolved unconditionally rather than behind the mode check because
	// resolvePolicyInstances logs what it drops, and having that log depend on the
	// card mode would make an unrelated policy problem appear and disappear with
	// it. It is a slice of already-resolved instances; building it costs nothing.
	commonPoliciesWithoutUpstreamAuth := commonOperationPolicies
	if upstreamAuth != nil {
		commonPoliciesWithoutUpstreamAuth = resolvePolicyInstances(
			t.policyDefinitions, t.latestVersions, a2a.OperationConfigs.Policies, policyv1alpha.LevelAPI)
	}

	// The internal instance an explicitly configured protected Agent Card adds to
	// the canonical GetExtendedAgentCard chain, and to nothing else.
	protectedCard, protectedCardManaged, err := t.protectedCardPolicyInstance(a2a.AgentCard.Protected)
	if err != nil {
		return nil, err
	}

	perOperationPolicies := make(map[agentproto.Operation][]policyenginev1.PolicyInstance)
	perOperationCORS := make(map[agentproto.Operation][]policyenginev1.PolicyInstance)
	perOperationResilience := make(map[agentproto.Operation]*api.Resilience)
	if a2a.OperationConfigs.Operations != nil {
		for i := range *a2a.OperationConfigs.Operations {
			opCfg := (*a2a.OperationConfigs.Operations)[i]
			operation := agentproto.Operation(opCfg.Name)
			resolved := resolvePolicyInstances(
				t.policyDefinitions, t.latestVersions, opCfg.Policies, policyv1alpha.LevelRoute)
			perOperationPolicies[operation] = resolved
			perOperationResilience[operation] = opCfg.Resilience
			// A preflight borrows an operation's cors policy and nothing else of
			// its chain. The rest — authentication above all — must not run
			// against a preflight, which carries no credentials and would be
			// rejected.
			perOperationCORS[operation] = corsInstances(resolved)
		}
	}

	cardPolicySource := a2a.AgentCard.Public.Policies
	if passthroughCard {
		cardPolicySource = withPolicy(cardPolicySource, upstreamAuth)
	}
	cardPolicies := resolvePolicyInstances(
		t.policyDefinitions, t.latestVersions, cardPolicySource, policyv1alpha.LevelAPI)

	// The policy that answers a managed card, appended after the author's own
	// card policies so anything they attached — a rate limit, an IP filter —
	// runs before the request is answered. It short-circuits the chain, so a
	// policy placed after it would never run at all.
	//
	// It is deliberately kept out of cardPolicies: the CORS preflight for this
	// same path is built from those, and a preflight that answered with the card
	// body would be worse than no preflight.
	cardChain := cardPolicies
	if passthroughCard {
		// L4: the gateway proxies the upstream's card unparsed, so none of the
		// card/policy consistency checks a managed card gets can run against it.
		// There is no deployment-status surface on the management API to carry
		// this yet, so it is recorded here, where it names the artifact.
		slog.Warn("agent card consistency cannot be verified in passthrough mode; "+
			"the upstream is responsible for advertising interfaces and security "+
			"requirements consistent with gateway enforcement",
			"agent_id", cfg.UUID, "handle", cfg.Handle)
	} else {
		cardPolicy, err := t.agentCardPolicyInstance(a2a.AgentCard.Public.Content)
		if err != nil {
			return nil, err
		}
		cardChain = append(append([]policyenginev1.PolicyInstance{}, cardPolicies...), cardPolicy)
	}

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

			// Exactly one operation's chain differs, and only when the Agent
			// explicitly configures a protected card. Everything else — including
			// GetExtendedAgentCard on an Agent that omits the block — is the plain
			// concatenation above.
			protectedChain := protectedCard != nil && operation == agentproto.GetExtendedAgentCard

			common := commonOperationPolicies
			if protectedChain && protectedCardManaged {
				common = commonPoliciesWithoutUpstreamAuth
			}

			chain := make([]policyenginev1.PolicyInstance, 0,
				len(common)+len(perOperationPolicies[operation])+1)
			chain = append(chain, common...)
			chain = append(chain, perOperationPolicies[operation]...)
			if protectedChain {
				// At the tail, after everything the author attached at either
				// scope. Which of their policies authenticates, and in which
				// scope, is theirs to decide — this only asks whether one of them
				// did. In managed mode it also short-circuits the chain, so
				// anything placed after it would never run.
				chain = append(chain, *protectedCard)
			}
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

		// preflights collects the routes that need an OPTIONS sibling, in
		// generation order, merged by path: two operations may share one (POST
		// and GET on pushNotificationConfigs, and every operation on the
		// JSON-RPC endpoint), and a single preflight answers for all of them.
		//
		// A preflight is generated **only if its own chain contains a cors
		// policy**. That is the whole rule, and it is what keeps the trigger and
		// the answer from disagreeing: a route that matches an OPTIONS request
		// but whose chain cannot respond to it is worse than no route at all —
		// Envoy stops 404ing the preflight and starts proxying it upstream
		// instead, so the browser's failure now depends on what the backend does
		// with an OPTIONS it never expected.
		//
		// The operation path is carried from the route that asked for the
		// preflight rather than re-derived from the path, so the preflight
		// strips exactly what its sibling strips.
		type preflightRoute struct {
			path, operationPath string
			// policies is the chain, before system policies are prepended: the
			// scope's own policies, then the cors instances contributed by each
			// operation reachable at this path.
			policies []policyenginev1.PolicyInstance
		}
		var preflights []preflightRoute
		preflightByPath := make(map[string]int)

		// addPreflight registers, or extends, the preflight for one path. base is
		// the governing scope's policies and is contributed once; operationCORS is
		// appended per contributing operation.
		addPreflight := func(path, operationPath string, base, operationCORS []policyenginev1.PolicyInstance) {
			if index, exists := preflightByPath[path]; exists {
				preflights[index].policies = append(preflights[index].policies, operationCORS...)
				return
			}
			policies := make([]policyenginev1.PolicyInstance, 0, len(base)+len(operationCORS))
			policies = append(policies, base...)
			policies = append(policies, operationCORS...)
			if !containsCORS(policies) {
				return
			}
			preflightByPath[path] = len(preflights)
			preflights = append(preflights, preflightRoute{
				path:          path,
				operationPath: operationPath,
				policies:      policies,
			})
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
					Timeout:             agentRouteTimeout(nil, agentTimeout, agentIdleTimeout, true),
					ResolverName:        agentproto.ResolverName,
					ResolverConfig:      resolverConfig,
					MaxRequestBodyBytes: maxAgentResolvedRequestBodyBytes,
				}); err != nil {
					return nil, err
				}
				// Every operation is reachable at this one endpoint, so each
				// contributes its cors policy to the single preflight that
				// covers them all.
				jsonrpcCORS := make([]policyenginev1.PolicyInstance, 0, len(operations))
				for _, operation := range operations {
					jsonrpcCORS = append(jsonrpcCORS, perOperationCORS[operation]...)
				}
				addPreflight(transport.basePath, transport.relativePath,
					commonOperationPolicies, jsonrpcCORS)

			case api.HTTPJSON:
				for _, operation := range operations {
					bindings, ok := agentproto.HTTPJSONBindings(protocolVersion, operation)
					if !ok {
						return nil, fmt.Errorf("A2A %s defines no HTTP+JSON binding for operation %q",
							protocolVersion, operation)
					}
					// The operation is known from the route, so this route
					// carries no canonical chain key. Most such routes resolve
					// statically; the two message-sending ones read the body for
					// the message identifiers it alone carries.
					resolverConfig, err := agentResolverConfig(protocolVersion, agentproto.TransportHTTPJSON, operation)
					if err != nil {
						return nil, err
					}
					timeout := agentRouteTimeout(
						perOperationResilience[operation], agentTimeout, agentIdleTimeout, isStreamingOperation(operation))
					// A body-resolved route with no explicit ceiling inherits the
					// engine's 64 KiB default, which would reject a legitimate
					// message carrying a file part.
					var maxRequestBodyBytes int64
					if operationResolvesFromRequestBody(operation) {
						maxRequestBodyBytes = maxAgentResolvedRequestBodyBytes
					}
					for _, binding := range bindings {
						path := config.JoinAgentPath(transport.basePath, binding.PathTemplate)
						// The transport prefix plus the protocol's own path.
						// Both reach the upstream; only spec.context does not.
						operationPath := config.JoinAgentPath(transport.relativePath, binding.PathTemplate)
						if _, err := addRoute(&models.Route{
							Method:              binding.Method,
							Path:                path,
							OperationPath:       operationPath,
							PathMatchType:       "Exact",
							Timeout:             timeout,
							ResolverName:        agentproto.ResolverName,
							ResolverConfig:      resolverConfig,
							MaxRequestBodyBytes: maxRequestBodyBytes,
						}); err != nil {
							return nil, err
						}
						addPreflight(path, operationPath,
							commonOperationPolicies, perOperationCORS[operation])
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
			Timeout:       agentRouteTimeout(nil, agentTimeout, agentIdleTimeout, false),
			// Where the card lives upstream is fixed by the protocol, so the
			// route says so regardless of mode; a managed card is answered by
			// its chain and never gets as far as using it. Without this a
			// custom gateway-facing path would be forwarded verbatim to an
			// upstream that only serves the well-known one.
			UpstreamPathOverride: agentCardUpstreamPath,
		})
		if err != nil {
			return nil, err
		}
		rdc.PolicyChains[cardRouteKey] = sdkChainToModel(
			utils.InjectSystemPolicies(cardChain, t.systemConfig, nil))
		// The card is not an operation, so only its own scope can answer its
		// preflight.
		addPreflight(cardPath, cardRelativePath, cardPolicies, nil)

		for _, preflight := range preflights {
			preflightKey, err := addRoute(&models.Route{
				Method:        agentPreflightMethod,
				Path:          preflight.path,
				OperationPath: preflight.operationPath,
				PathMatchType: "Exact",
			})
			if err != nil {
				return nil, err
			}
			rdc.PolicyChains[preflightKey] = sdkChainToModel(
				utils.InjectSystemPolicies(preflight.policies, t.systemConfig, nil))
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

// agentCardPolicyInstance builds the policy that serves a managed card.
//
// The policy is attached by name, so its version is resolved from the loaded
// definitions the same way an author-attached policy's is. Unlike an
// author-attached policy, an unresolvable one is a hard failure rather than a
// dropped chain entry: the chain is what makes a managed card a managed card.
// Dropping it would leave a route that proxies the card request to the upstream,
// and the gateway would then serve the upstream's own unvalidated, unsigned card
// under a configuration that says the gateway owns it — a silent substitution
// with no error anywhere.
func (t *AgentTransformer) agentCardPolicyInstance(
	content *api.A2AAgentCardDocument,
) (policyenginev1.PolicyInstance, error) {
	if content == nil || len(*content) == 0 {
		// Validation rejects a managed card with no content, so this is a
		// defensive check on a path that should be unreachable.
		return policyenginev1.PolicyInstance{},
			fmt.Errorf("managed agent card has no content to serve")
	}

	body, etag, err := agentCardBody(*content)
	if err != nil {
		return policyenginev1.PolicyInstance{}, err
	}

	// The card configuration is a nested block because the policy is the A2A
	// system policy, not the Agent Card policy: every other thing it answers
	// brings its own block alongside this one.
	instance, err := t.a2aSystemPolicyInstance(map[string]any{
		constants.A2A_POLICY_PARAM_AGENT_CARD: map[string]any{
			constants.A2A_POLICY_PARAM_CONTENT: string(body),
			constants.A2A_POLICY_PARAM_ETAG:    etag,
		},
	})
	if err != nil {
		return policyenginev1.PolicyInstance{}, fmt.Errorf(
			"cannot serve a managed agent card: %w", err)
	}
	return instance, nil
}

// protectedCardPolicyInstance builds the internal instance an explicitly
// configured protected Agent Card adds to the canonical GetExtendedAgentCard
// chain, and reports whether that card is managed.
//
// It goes at the tail of the chain, after every policy the author attached at
// either scope. Where authentication sits among those is the author's decision;
// this instance only asks whether the request that reached it was authenticated
// by something. That question is answered from SharedContext.AuthContext, which
// every supported auth policy populates identically, so a custom authentication
// policy protects this surface exactly as well as a built-in one — and no
// deployment-time allowlist of policy names is needed, which could not have
// proved that a policy bearing an approved name authenticated *this* request
// anyway.
//
// The instance is returned for both modes and its check is unconditional. An
// Agent that opts into a protected card gets a card that is protected; whether
// the author also remembered to attach an authentication policy is not a
// question the answer may depend on, because "no auth policy" would otherwise
// mean "public extended card" — the exact failure the feature exists to prevent.
//
// A nil instance means the Agent has no explicit protected block at all. That is
// deliberately not the same as passthrough: it is the behaviour that already
// shipped, where GetExtendedAgentCard is an ordinary proxied operation with no
// gateway-added guard.
func (t *AgentTransformer) protectedCardPolicyInstance(
	protected *api.A2AProtectedAgentCard,
) (*policyenginev1.PolicyInstance, bool, error) {
	if protected == nil {
		return nil, false, nil
	}

	// Passthrough carries no content: require an authenticated request, then
	// forward the operation and let the upstream answer it.
	parameters := map[string]any{}
	managed := protected.Mode == api.A2AProtectedAgentCardModeManaged

	if managed {
		if protected.Content == nil || len(*protected.Content) == 0 {
			// Validation rejects a managed card with no content, so this is a
			// defensive check on a path that should be unreachable. Failing here
			// rather than degrading to passthrough matters: degrading would proxy
			// the upstream's own unvalidated extended card under a configuration
			// that says the gateway owns it.
			return nil, false, fmt.Errorf("managed protected agent card has no content to serve")
		}
		// The same encoder the public card uses. A second one would produce
		// different bytes for the same document, and the bytes are what card
		// signing will sign. The ETag is deliberately discarded: this response is
		// authenticated and carries Cache-Control: no-store, and the JSON-RPC
		// binding is a POST that cannot take part in the conditional-GET contract
		// the public route has.
		body, _, err := agentCardBody(*protected.Content)
		if err != nil {
			return nil, false, err
		}
		parameters[constants.A2A_POLICY_PARAM_CONTENT] = string(body)
	}

	instance, err := t.a2aSystemPolicyInstance(map[string]any{
		constants.A2A_POLICY_PARAM_PROTECTED_AGENT_CARD: parameters,
	})
	if err != nil {
		return nil, false, fmt.Errorf("cannot serve the protected agent card: %w", err)
	}
	return &instance, managed, nil
}

// a2aSystemPolicyInstance builds one instance of the in-repo A2A system policy
// carrying the given parameter block.
//
// The policy is attached by name, so its version is resolved from the loaded
// definitions the same way an author-attached policy's is — and, as there, an
// unresolvable one is a hard failure rather than a dropped chain entry, because
// every job this policy does is one the chain cannot be correct without.
func (t *AgentTransformer) a2aSystemPolicyInstance(
	parameters map[string]any,
) (policyenginev1.PolicyInstance, error) {
	resolved, err := config.ResolvePolicyVersion(
		t.policyDefinitions, t.latestVersions, constants.A2A_SYSTEM_POLICY_NAME, "")
	if err != nil {
		return policyenginev1.PolicyInstance{}, err
	}
	// No attachedTo parameter, matching how the injected system policies are
	// built: the attachment level is a hint for author-attached policies about
	// which scope configured them, and this one was configured by no scope.
	return policyenginev1.PolicyInstance{
		Name:       constants.A2A_SYSTEM_POLICY_NAME,
		Version:    versionutil.MajorVersion(resolved),
		Enabled:    true,
		Parameters: parameters,
	}, nil
}

// agentCardBody serializes a managed Agent Card and derives its entity tag.
//
// This is the one place the served bytes are produced. Card signing (a later
// section) signs the bytes this returns, and the runtime serves them verbatim,
// so a second encoding of the same document anywhere would be a signature that
// does not verify against the card a client received. The document itself is
// never rewritten — extension fields and unknown members survive, because it is
// carried as a free-form map rather than a typed struct.
//
// The tag is a strong ETag: a hash over exactly those bytes, so two cards
// compare equal if and only if their served representations are byte-identical.
// A deploy that does not change the card therefore does not invalidate a
// client's cached copy.
func agentCardBody(content api.A2AAgentCardDocument) ([]byte, string, error) {
	body, err := json.Marshal(content)
	if err != nil {
		// Unreachable for a document that arrived as JSON or YAML, and already
		// rejected by validation's own size check, which encodes it too.
		return nil, "", fmt.Errorf("failed to encode agent card content: %w", err)
	}
	digest := sha256.Sum256(body)
	return body, `"` + hex.EncodeToString(digest[:]) + `"`, nil
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

// agentRouteTimeout resolves one route's timeouts.
//
// Precedence is resolved **per field** — operation over agent, timeout and idle
// timeout independently — before the streaming default is considered. Choosing
// one resilience block wholesale instead would let an operation that sets only
// idleTimeout discard the Agent's explicit timeout, and on a streaming route it
// would then be replaced by the disabled default: the Agent asks for 45s, the
// operation says nothing about timeouts, and the route ends up with none.
//
// streaming makes the *default* a disabled route timeout, because a finite one
// would cut a healthy stream at an arbitrary point. It is only a default: an
// explicit timeout at either level still wins, and the idle timeout is left
// alone, since that is the liveness guard a disabled route timeout relies on.
//
// operationResilience is nil for the routes that are not one operation — the
// JSON-RPC endpoint, which serves all of them, and the public Agent Card.
func agentRouteTimeout(
	operationResilience *api.Resilience,
	agentTimeout, agentIdleTimeout *time.Duration,
	streaming bool,
) *models.RouteTimeout {
	opTimeout, opIdleTimeout, err := xds.ResolveResilience(operationResilience)
	if err != nil {
		// Unreachable in practice: validation rejects a malformed duration
		// before an Agent is stored. Dropping to the agent-level values keeps a
		// transform that somehow got here from silently widening the route.
		opTimeout, opIdleTimeout = nil, nil
	}

	timeout := buildRouteTimeout(opTimeout, agentTimeout, opIdleTimeout, agentIdleTimeout)
	if !streaming {
		return timeout
	}
	if timeout == nil {
		timeout = &models.RouteTimeout{}
	}
	if timeout.Timeout == nil {
		disabled := time.Duration(0)
		timeout.Timeout = &disabled
	}
	return timeout
}

// isStreamingOperation reports whether an operation's response is a long-lived
// event stream rather than a single reply.
func isStreamingOperation(operation agentproto.Operation) bool {
	return operation == agentproto.SendStreamingMessage || operation == agentproto.SubscribeToTask
}

// agentUpstreamAuthPolicy builds the policy that injects the Agent's upstream
// credential, or nil when the Agent configures none. It mirrors what the MCP
// transformer does with the identically shaped block.
// The struct shape below must stay identical (field names, types, tags and
// order) to AgentConfigData_Upstream.Auth in the generated management models:
// it is an inline anonymous struct there, so Go type identity is the only thing
// that links them. It widened when the shared UpstreamAuth schema component
// gained the policyName/policyParams/policyVersion fields.
func agentUpstreamAuthPolicy(auth *struct {
	Header        *string                             `json:"header,omitempty" yaml:"header,omitempty"`
	PolicyName    *string                             `json:"policyName,omitempty" yaml:"policyName,omitempty"`
	PolicyParams  *map[string]interface{}             `json:"policyParams,omitempty" yaml:"policyParams,omitempty"`
	PolicyVersion *string                             `json:"policyVersion,omitempty" yaml:"policyVersion,omitempty"`
	Type          api.AgentConfigDataUpstreamAuthType `json:"type" yaml:"type"`
	Value         *string                             `json:"value,omitempty" yaml:"value,omitempty"`
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

// corsInstances returns the cors policies of a resolved chain.
//
// It filters *resolved* instances rather than the configured api.Policy list on
// purpose: a policy whose version cannot be resolved is dropped from the chain
// (with a log line), and a preflight must be judged on the chain that will
// actually run, not on what was asked for. A cors attachment naming an unknown
// policy therefore produces no preflight route, rather than one nothing answers.
func corsInstances(chain []policyenginev1.PolicyInstance) []policyenginev1.PolicyInstance {
	var cors []policyenginev1.PolicyInstance
	for _, instance := range chain {
		if strings.EqualFold(instance.Name, corsPolicyName) {
			cors = append(cors, instance)
		}
	}
	return cors
}

// containsCORS reports whether a chain can answer a CORS preflight.
func containsCORS(chain []policyenginev1.PolicyInstance) bool {
	return len(corsInstances(chain)) > 0
}
