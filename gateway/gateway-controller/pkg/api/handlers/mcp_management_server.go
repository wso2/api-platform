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

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/agent"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/certificate"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/graphqlapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/subscription"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/version"
)

// McpHandler serves the Model Context Protocol endpoint. Built once at startup and reused for every request.
type McpHandler struct {
	// protected is the SDK's Streamable HTTP handler wrapped in the per-tool
	// authorization gate. Everything the mux serves goes through this field.
	protected http.Handler

	restAPIService       *restapi.RestAPIService
	mcpDeploymentService *utils.MCPDeploymentService
	llmDeploymentService *utils.LLMDeploymentService
	agentService         *agent.AgentService
	graphQLAPIService    *graphqlapi.GraphQLAPIService
	secretService        *secrets.SecretService
	apiKeyService        *utils.APIKeyService
	kinds                map[string]*kindOps

	// Certificates and subscriptions are deliberately NOT in the kind registry.
	// They have no manifest, are identified by a server-generated UUID
	// rather than a metadata.name handle, and have verbs kindOps cannot express
	// (a certificate "reload" with no CRUD analogue). Each gets one
	// action-dispatched tool instead, holding its service directly.
	certificateService  *certificate.CertificateService
	subscriptionService *subscription.SubscriptionService

	// authz is the shared authorization core: the HTTP-layer gate, the
	// tool-layer check, and the RFC 6750 step-up challenge
	authz *mcpAuthz

	// immutable mirrors ImmutableGateway.Enabled. The MCP route is deliberately
	// NOT wrapped in the immutable middleware (that middleware rejects every
	// POST, which would disable reads too, since every MCP call is a POST), so
	// the write tools enforce the invariant themselves.
	immutable bool

	// MaxCertificateUploadBytes is the body limit POST and PUT /certificates
	// enforce, applied to the certificate tool. Mirrors server.max_certificate_upload_bytes
	maxCertificateUploadBytes int64

	maxRequestBytes int64
	logger          *slog.Logger
}

// McpHandlerParams collects newMcpHandler's dependencies.
type McpHandlerParams struct {
	RestAPIService            *restapi.RestAPIService
	MCPDeploymentService      *utils.MCPDeploymentService
	LLMDeploymentService      *utils.LLMDeploymentService
	AgentService              *agent.AgentService
	GraphQLAPIService         *graphqlapi.GraphQLAPIService
	SecretService             *secrets.SecretService
	APIKeyService             *utils.APIKeyService
	CertificateService        *certificate.CertificateService
	SubscriptionService       *subscription.SubscriptionService
	ResourceRoles             map[string][]string
	RoleMapping               map[string][]string
	ResourceMetadataURL       string
	Immutable                 bool
	MaxRequestBytes           int64
	MaxCertificateUploadBytes int64
	Logger                    *slog.Logger
}

// newMcpHandler builds the MCP server, registers the tools, and wraps the
// SDK handler in the authorization gate.
func newMcpHandler(p McpHandlerParams) *McpHandler {
	h := &McpHandler{
		restAPIService:            p.RestAPIService,
		mcpDeploymentService:      p.MCPDeploymentService,
		llmDeploymentService:      p.LLMDeploymentService,
		agentService:              p.AgentService,
		graphQLAPIService:         p.GraphQLAPIService,
		secretService:             p.SecretService,
		apiKeyService:             p.APIKeyService,
		certificateService:        p.CertificateService,
		subscriptionService:       p.SubscriptionService,
		immutable:                 p.Immutable,
		logger:                    p.Logger,
		maxCertificateUploadBytes: p.MaxCertificateUploadBytes,
		authz: newMcpAuthz(mcpAuthzParams{
			ResourceRoles:       p.ResourceRoles,
			RoleMapping:         p.RoleMapping,
			ResourceMetadataURL: p.ResourceMetadataURL,
			MaxRequestBytes:     p.MaxRequestBytes,
			Logger:              p.Logger,
		}),
	}

	// Built after h exists because each kindOps closes over h's services.
	h.kinds = h.buildKindRegistry()

	// Assigned after the registry exists, because routeKeysForCall resolves a
	// call's kind argument through h.kinds. Until this line the gate's default
	// resolver maps nothing, so a construction path that skipped it would fail
	// closed rather than authorize by accident.
	h.authz.routeKeysForCall = h.routeKeysForCall

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "wso2-api-platform-gateway-controller-management",
		Version: version.Version,
	}, nil)
	h.registerTools(server)

	streamHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			// MCP 2026-07-28 removed protocol-level sessions and the GET
			// stream. Stateless mode implements exactly that: no
			// Mcp-Session-Id is read or minted, and GET/DELETE answer 405.
			Stateless: true,
			// The gate reads the body first; keep the SDK's ceiling identical
			// so the two layers reject oversized bodies at the same threshold.
			MaxRequestBodyBytes: h.authz.maxRequestBytes,
			// A closed HTTP request means the response can no longer be
			// delivered, so cancelling the in-flight handler is safe and stops
			// work on an abandoned deploy.
			PropagateRequestCancellation: true,
			Logger:                       p.Logger,
		},
	)

	h.protected = h.authz.ScopeGate(streamHandler)
	return h
}

// ServeHTTP implements http.Handler. Registered on the mux in main.go behind
// the authentication and baseline authorization middleware.
func (h *McpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.protected.ServeHTTP(w, r)
}
