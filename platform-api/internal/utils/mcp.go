/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"

	"gopkg.in/yaml.v3"
)

// MCP JSON-RPC constants
const (
	JsonRpcVersion           = "2.0"
	ProtocolVersion          = "2025-06-18"
	MethodInitialize         = "initialize"
	MethodInitialized        = "notifications/initialized"
	MethodServerDiscover     = "server/discover"
	MethodToolsList          = "tools/list"
	MethodPromptsList        = "prompts/list"
	MethodResourcesList      = "resources/list"
	ClientName               = "api-platform-mcp-client"
	ClientVersion            = "1.0.0"
	McpSessionHeader         = "mcp-session-id"
	McpProtocolVersionHeader = "MCP-Protocol-Version"
	McpMethodHeader          = "Mcp-Method"
)

// Keys of the params._meta envelope 2026-07-28 MCP spec requires on every request.
const (
	metaProtocolVersionKey    = "io.modelcontextprotocol/protocolVersion"
	metaClientInfoKey         = "io.modelcontextprotocol/clientInfo"
	metaClientCapabilitiesKey = "io.modelcontextprotocol/clientCapabilities"
)

// jsonRPCMethodNotFound is the JSON-RPC 2.0 "Method not found" code. Returned for
// server/discover it identifies a server implementing a revision earlier than 2026-07-28, which
// made that method mandatory.
const jsonRPCMethodNotFound = -32601

// metaServerInfoKey is where 2026-07-28 places the server's identity in a server/discover
// result.
const metaServerInfoKey = "io.modelcontextprotocol/serverInfo"

// SpecVersion20260728 is the revision that introduced server/discover, sent as
// MCP-Protocol-Version on requests made under it.
//
// A literal rather than a generated schema constant: upstreamMcpSpecVersions records whichever
// revisions a server reports, so the schema constrains the shape of a revision date and not its
// value, and there are no generated constants left to name. What this client can speak is a
// separate, closed set - the one below.
const SpecVersion20260728 = "2026-07-28"

// mcpRequestTimeout bounds each outbound MCP JSON-RPC call (DNS + connect + TLS + read).
const mcpRequestTimeout = 10 * time.Second

// JsonRPCRequest represents a JSON-RPC request
type JsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// JsonRPCError represents a JSON-RPC error
type JsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolsResult represents the result of tools/list request
type ToolsResult struct {
	Result struct {
		Tools []map[string]interface{} `json:"tools"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// PromptsResult represents the result of prompts/list request
type PromptsResult struct {
	Result struct {
		Prompts []map[string]interface{} `json:"prompts"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// ResourcesResult represents the result of resources/list request
type ResourcesResult struct {
	Result struct {
		Resources []map[string]interface{} `json:"resources"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// DiscoverResult is the subset of a server/discover result this probe consumes. Members it does
// not declare - instructions, and the CacheableResult hints ttlMs and cacheScope - are discarded
// by encoding/json rather than causing a decode error.
type DiscoverResult struct {
	Result struct {
		SupportedVersions []string               `json:"supportedVersions"`
		Capabilities      map[string]interface{} `json:"capabilities"`
		Meta              map[string]interface{} `json:"_meta"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

// ServerInfo returns the server identity the result carries, or nil when the server omitted it.
func (d DiscoverResult) ServerInfo() map[string]interface{} {
	info, ok := d.Result.Meta[metaServerInfoKey].(map[string]interface{})
	if !ok {
		return nil
	}
	return info
}

// InitializeResult represents the result of initialize request
type InitializeResult struct {
	Result struct {
		ProtocolVersion string                 `json:"protocolVersion"`
		ServerInfo      map[string]interface{} `json:"serverInfo"`
		Capabilities    map[string]interface{} `json:"capabilities"`
	} `json:"result"`
	Error *JsonRPCError `json:"error"`
}

type MCPUtils struct{}

// BuildMCPDeploymentYAML builds the deployment YAML struct without marshalling.
func (u *MCPUtils) BuildMCPDeploymentYAML(proxy *model.MCPProxy) (*model.MCPProxyDeploymentYAML, error) {

	contextValue := "/"
	if proxy.Configuration.Context != nil && *proxy.Configuration.Context != "" {
		contextValue = *proxy.Configuration.Context
	}
	var vhostValue *string
	if proxy.Configuration.Vhost != nil {
		vhostValue = proxy.Configuration.Vhost
	}

	mcpDeploymentYaml := model.MCPProxyDeploymentYAML{
		ApiVersion: constants.GatewayApiVersion,
		Kind:       constants.MCPProxy,
		Metadata: model.DeploymentMetadata{
			Name: proxy.Handle,
		},
		Spec: model.MCPProxyDeploymentSpec{
			DisplayName:  proxy.Name,
			Version:      proxy.Version,
			Context:      contextValue,
			Vhost:        vhostValue,
			SpecVersions: proxy.Configuration.EffectiveSpecVersions(),
			Policies:     proxy.Configuration.Policies,
		},
	}

	// Considering upstream main only as the sandbox is not supported in the gateway side currently.
	var upstream model.MCPProxyUpstream
	if proxy.Configuration.Upstream.Main != nil {
		upstream.URL = proxy.Configuration.Upstream.Main.URL
		if proxy.Configuration.Upstream.Main.Auth != nil {
			upstream.Auth = proxy.Configuration.Upstream.Main.Auth
		}

	}
	mcpDeploymentYaml.Spec.Upstream = upstream

	if proxy.ProjectUUID != nil {
		mcpDeploymentYaml.Metadata.Labels = map[string]string{
			"projectId": *proxy.ProjectUUID,
		}
	}

	return &mcpDeploymentYaml, nil
}

// GenerateMCPDeploymentYAML creates the deployment YAML string.
func (u *MCPUtils) GenerateMCPDeploymentYAML(proxy *model.MCPProxy) (string, error) {
	d, err := u.BuildMCPDeploymentYAML(proxy)
	if err != nil {
		return "", err
	}
	yamlBytes, err := yaml.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("failed to marshal MCP proxy to YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// modernRequestParams is the params a request carries under 2026-07-28. All three envelope keys
// are required: a server rejects the call with -32602 when any is absent.
func modernRequestParams() map[string]any {
	return map[string]any{
		"_meta": map[string]any{
			metaProtocolVersionKey: SpecVersion20260728,
			metaClientInfoKey: map[string]any{
				"name":    ClientName,
				"version": ClientVersion,
			},
			metaClientCapabilitiesKey: map[string]any{},
		},
	}
}

// implementedSpecVersions are the revisions this client knows how to address a server under.
// 2026-07-28 is the only modern one, so a negotiated modern version is always the revision
// modernRequestParams states in _meta.
var implementedSpecVersions = []string{
	"2025-06-18",
	"2025-11-25",
	SpecVersion20260728,
}

// errNoSharedSpecVersion reports that the server implements no revision this client does.
var errNoSharedSpecVersion = errors.New("no MCP specification version in common with the server")

var specVersionPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// reportedSpecVersions keeps the revisions a server named, dropping only values that are not
// revision dates at all.
//
// Deliberately not filtered by what this platform or a gateway serves: the response records
// what the server said, and hiding a revision it named would make it a claim about us rather
// than about the server. Which revisions a given gateway serves is that gateway's property, and
// it is reported where a gateway is chosen. Not implementedSpecVersions either: that is the
// narrower set this client speaks when it addresses the server.
func reportedSpecVersions(reported []string) []string {
	kept := make([]string, 0, len(reported))
	for _, version := range reported {
		if specVersionPattern.MatchString(version) {
			kept = append(kept, version)
		}
	}
	return kept
}

// negotiateVersion picks the highest revision both sides implement, for the calls that follow a
// server/discover. Taking the server's highest outright would address it under a revision this
// client cannot speak: the version rides in the MCP-Protocol-Version header while _meta still
// declared 2026-07-28, so one request would state two revisions and a conformant server would
// reject it.
func negotiateVersion(reported []string) (string, error) {
	if len(reported) == 0 {
		// Reporting nothing is not evidence of nothing shared: supportedVersions is filled
		// from the modern set only in some SDKs, so a dual-era server under-reports.
		return SpecVersion20260728, nil
	}

	negotiated := ""
	for _, version := range reported {
		if slices.Contains(implementedSpecVersions, version) && version > negotiated {
			negotiated = version
		}
	}
	if negotiated == "" {
		return "", fmt.Errorf("%w: it reports %s, this platform implements %s",
			errNoSharedSpecVersion, strings.Join(reported, ", "), strings.Join(implementedSpecVersions, ", "))
	}
	return negotiated, nil
}

// FetchMCPServerInfo fetches server information from an MCP backend including tools, prompts,
// resources, server info and the protocol versions the server reports.
//
// server/discover is tried first: mandatory from 2026-07-28, it returns supportedVersions,
// capabilities and serverInfo in a single call and establishes no session. A refusal falls back to
// the initialize handshake; a transport failure propagates. The handshake also runs when discovery
// succeeded but the negotiated revision is legacy, since those revisions require a session.
func FetchMCPServerInfo(url string, headerName string, headerValue string) (*api.MCPServerInfoFetchResponse, error) {
	resp := &api.MCPServerInfoFetchResponse{}

	// Headers for the tools/list, prompts/list and resources/list calls below, completed by
	// whichever handshake answered: a session id on the legacy path, a version on the modern one.
	catalogue := mcpRequestHeaders{headerName: headerName, headerValue: headerValue}

	versions, serverInfo, err := discoverMCPServer(url, headerName, headerValue)
	switch {
	case err == nil:
		if catalogue.protocolVersion, err = negotiateVersion(versions); err != nil {
			return nil, err
		}
		// A server that answered server/discover can still report a set whose highest shared
		// revision predates 2026-07-28, and those revisions make the initialize lifecycle
		// mandatory: a stateful one rejects every later call for want of a session. Run the
		// handshake for that session, but keep what discovery reported - initialize yields only
		// the single revision it negotiated, which is a narrower answer than the server gave.
		if !catalogue.isModern() {
			handshakeVersions, legacyServerInfo, sessionID, initErr := initializeMCPServerLegacy(url, headerName, headerValue)
			if initErr != nil {
				return nil, initErr
			}
			catalogue.sessionID = sessionID
			// The handshake, not discovery, settles the revision for this session: the client
			// offers one version and the server answers with the one it will serve, which can
			// be lower than discovery advertised. Later calls must carry that answer, since a
			// strict server rejects a header naming a revision it did not negotiate.
			if len(handshakeVersions) > 0 {
				catalogue.protocolVersion = handshakeVersions[0]
			}
			if serverInfo == nil {
				serverInfo = legacyServerInfo
			}
		}
	case errors.Is(err, errServerDiscoverUnsupported):
		var sessionID string
		versions, serverInfo, sessionID, err = initializeMCPServerLegacy(url, headerName, headerValue)
		if err != nil {
			return nil, err
		}
		catalogue.sessionID = sessionID
		// Same rule with no discovery to fall back on: the handshake's answer is the only
		// revision this session has, so the catalogue calls must name it.
		if len(versions) > 0 {
			catalogue.protocolVersion = versions[0]
		}
	default:
		return nil, fmt.Errorf("failed to discover MCP server: %w", err)
	}

	if serverInfo != nil {
		resp.ServerInfo = &serverInfo
	}
	if reportedVersions := reportedSpecVersions(versions); len(reportedVersions) > 0 {
		resp.SupportedVersions = &reportedVersions
	}

	// Step 3: Fetch tools
	toolsReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      2,
		Method:  MethodToolsList,
	}
	if catalogue.isModern() {
		toolsReq.Params = modernRequestParams()
	}
	toolsResp, err := postJSONRPC(url, toolsReq, catalogue.forMethod(MethodToolsList))
	if err != nil {
		slog.Default().Warn("Failed to fetch MCP tools, continuing with available info", "error", err)
	} else {
		var toolsResult ToolsResult
		if err := json.Unmarshal(toolsResp, &toolsResult); err != nil {
			slog.Default().Warn("Failed to parse MCP tools response, continuing with available info", "error", err)
		} else if toolsResult.Error != nil {
			slog.Default().Warn("tools/list returned an error, continuing with available info", "error", toolsResult.Error.Message)
		} else if len(toolsResult.Result.Tools) > 0 {
			resp.Tools = &toolsResult.Result.Tools
		}
	}

	// Step 4: Fetch prompts
	promptsReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      3,
		Method:  MethodPromptsList,
	}
	if catalogue.isModern() {
		promptsReq.Params = modernRequestParams()
	}
	promptsResp, err := postJSONRPC(url, promptsReq, catalogue.forMethod(MethodPromptsList))
	if err != nil {
		slog.Default().Warn("Failed to fetch MCP prompts, continuing with available info", "error", err)
	} else {
		var promptsResult PromptsResult
		if err := json.Unmarshal(promptsResp, &promptsResult); err != nil {
			slog.Default().Warn("Failed to parse MCP prompts response, continuing with available info", "error", err)
		} else if promptsResult.Error != nil {
			slog.Default().Warn("prompts/list returned an error, continuing with available info", "error", promptsResult.Error.Message)
		} else if len(promptsResult.Result.Prompts) > 0 {
			resp.Prompts = &promptsResult.Result.Prompts
		}
	}

	// Step 5: Fetch resources
	resourcesReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      4,
		Method:  MethodResourcesList,
	}
	if catalogue.isModern() {
		resourcesReq.Params = modernRequestParams()
	}
	resourcesResp, err := postJSONRPC(url, resourcesReq, catalogue.forMethod(MethodResourcesList))
	if err != nil {
		slog.Default().Warn("Failed to fetch MCP resources, continuing with available info", "error", err)
	} else {
		var resourcesResult ResourcesResult
		if err := json.Unmarshal(resourcesResp, &resourcesResult); err != nil {
			slog.Default().Warn("Failed to parse MCP resources response, continuing with available info", "error", err)
		} else if resourcesResult.Error != nil {
			slog.Default().Warn("resources/list returned an error, continuing with available info", "error", resourcesResult.Error.Message)
		} else if len(resourcesResult.Result.Resources) > 0 {
			resp.Resources = &resourcesResult.Result.Resources
		}
	}

	return resp, nil
}

// initializeMCPServerLegacy runs the initialize/notifications-initialized handshake and reports
// what it yields: the single protocol version negotiated for this connection, the server's
// identity, and the mcp-session-id every subsequent call must carry.
//
// The negotiated version is not the server's full set - initialize returns one version, chosen
// against the one the client asked for.
func initializeMCPServerLegacy(url string, headerName string, headerValue string) ([]string, map[string]any, string, error) {
	sessionID, protocolVersion, serverInfo, err := initializeMCPServer(url, headerName, headerValue)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	notifyReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		Method:  MethodInitialized,
	}
	if _, err := postJSONRPC(url, notifyReq, mcpRequestHeaders{
		headerName:      headerName,
		headerValue:     headerValue,
		sessionID:       sessionID,
		protocolVersion: protocolVersion,
	}); err != nil {
		return nil, nil, "", fmt.Errorf("failed to send notification: %w", err)
	}

	var versions []string
	if protocolVersion != "" {
		versions = []string{protocolVersion}
	}
	return versions, serverInfo, sessionID, nil
}

// errServerDiscoverUnsupported reports that the server refused server/discover.
var errServerDiscoverUnsupported = errors.New("server does not implement server/discover")

// discoverMCPServer issues server/discover and returns the protocol versions the server reports
// alongside its identity. It takes no params and needs no session.
//
// errServerDiscoverUnsupported is returned for any refusal: a JSON-RPC error at any status below
// 500, or HTTP 404/405 from a router with no handler. A timeout, transport error or 5xx is
// returned as itself, since none of them establishes that the method is absent.
func discoverMCPServer(url string, headerName string, headerValue string) ([]string, map[string]any, error) {
	req := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      1,
		Method:  MethodServerDiscover,
		Params:  modernRequestParams(),
	}

	body, err := postJSONRPC(url, req, mcpRequestHeaders{
		headerName:      headerName,
		headerValue:     headerValue,
		protocolVersion: SpecVersion20260728,
		method:          MethodServerDiscover,
	})
	if err != nil {
		// A router with no handler for the method answers 404 or 405 before any JSON-RPC
		// layer sees the request.
		if isMethodUnsupportedStatus(err) {
			return nil, nil, errServerDiscoverUnsupported
		}
		// A non-2xx status can still carry a JSON-RPC error body.
		if rpcErr, ok := jsonRPCErrorFrom(statusErrorBody(err)); ok {
			logServerDiscoverRefusal(url, rpcErr)
			return nil, nil, errServerDiscoverUnsupported
		}
		return nil, nil, err
	}

	var result DiscoverResult
	if err := json.Unmarshal(body, &result); err != nil {
		// A body that is not JSON-RPC is no evidence about the method, so it propagates rather
		// than triggering the fallback.
		return nil, nil, fmt.Errorf("failed to parse server/discover response: %w, body: %s", err, string(body))
	}
	if result.Error != nil {
		logServerDiscoverRefusal(url, result.Error)
		return nil, nil, errServerDiscoverUnsupported
	}

	return result.Result.SupportedVersions, result.ServerInfo(), nil
}

// initializeMCPServer runs the handshake and returns the session ID, the protocol version the
// server negotiated, and its identity.
func initializeMCPServer(url string, headerName string, headerValue string) (string, string, map[string]any, error) {
	initReq := JsonRPCRequest{
		JSONRPC: JsonRpcVersion,
		ID:      1,
		Method:  MethodInitialize,
		Params: map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"roots": map[string]bool{"listChanged": true}},
			"clientInfo":      map[string]string{"name": ClientName, "version": ClientVersion},
		},
	}
	data, err := json.Marshal(initReq)
	if err != nil {
		return "", "", nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mcpRequestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create init request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if headerName != "" {
		lower := strings.ToLower(headerName)
		if lower != McpSessionHeader && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headerName, headerValue)
		}
	}
	// The MCP endpoint URL is user/tenant-supplied — dial through the SSRF-guarded client
	// so scheme, resolved-IP and redirect restrictions apply to this call too.
	client, err := NewUpstreamFetchClient(mcpRequestTimeout)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to reach MCP server for initialize: %w", err)
	}
	defer resp.Body.Close()
	// Bound the body: the shared client's own MaxResponseBytes is disabled (see
	// InitSharedHTTPClient's doc comment), so this call site applies its own configured
	// ceiling — read one extra byte so an over-limit response can be detected explicitly
	// rather than silently truncated.
	maxBytes := mcpResponseMaxBytes()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return "", "", nil, fmt.Errorf("MCP server response exceeds the maximum allowed size")
	}

	// Check HTTP status code. The 401 is mapped as it is in postJSONRPC, for the same reason.
	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", nil, apperror.MCPProxyUpstreamUnauthorized.New().
			WithLogMessage("MCP server returned 401 Unauthorized to the initialize request")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", nil, fmt.Errorf("initialize request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check if response is event stream and parse it
	isEventStreamResp := isEventStream(resp)
	if isEventStreamResp {
		data, err := parseEventStream(body)
		if err == nil {
			body = data
		}
	}

	// Parse initialize response for server info
	var initResult InitializeResult
	if err := json.Unmarshal(body, &initResult); err != nil {
		// Only ignore unmarshal error if this was a valid event stream (parsed above)
		if !isEventStreamResp {
			return "", "", nil, fmt.Errorf("failed to parse initialize response: %w, body: %s", err, string(body))
		}
		// For event stream, if unmarshal fails after successful parsing, that's still an error
		return "", "", nil, fmt.Errorf("failed to parse initialize response from event stream: %w, body: %s", err, string(body))
	}

	if initResult.Error != nil {
		return "", "", nil, fmt.Errorf("initialize request returned an error: %s", initResult.Error.Message)
	}

	var serverInfo map[string]any
	if initResult.Result.ServerInfo != nil {
		serverInfo = initResult.Result.ServerInfo
	}

	sessionID := getSessionIDFromResponse(resp)
	return sessionID, initResult.Result.ProtocolVersion, serverInfo, nil
}

func getSessionIDFromResponse(resp *http.Response) string {
	return resp.Header.Get(McpSessionHeader)
}

// httpStatusError carries the HTTP status of a non-2xx MCP response so callers can distinguish
// 404/405, which establish that a method is not served, from 5xx and transport failures, which
// establish nothing.
type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("request failed with status %d: %s", e.StatusCode, e.Body)
}

// statusErrorBody returns the body carried by a 4xx error, nil for any other failure. A 5xx is
// excluded because it can originate from an intermediary that never reached the server, so its
// body is no evidence of the server's era whatever it contains.
func statusErrorBody(err error) []byte {
	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode >= http.StatusInternalServerError {
		return nil
	}
	return []byte(statusErr.Body)
}

// jsonRPCErrorFrom extracts a JSON-RPC error from a body, unwrapping an event-stream frame first.
func jsonRPCErrorFrom(body []byte) (*JsonRPCError, bool) {
	if len(body) == 0 {
		return nil, false
	}
	if data, err := parseEventStream(body); err == nil {
		body = data
	}
	var envelope struct {
		Error *JsonRPCError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error == nil {
		return nil, false
	}
	return envelope.Error, true
}

// No error code is matched on: a legacy server may refuse with -32601 or an implementation-
// defined code such as -32000. The code is logged so that -32602 or -32020, which indicate a
// malformed envelope on this side, stay distinguishable from a genuine legacy refusal.
func logServerDiscoverRefusal(url string, rpcErr *JsonRPCError) {
	slog.Default().Debug("server/discover refused, falling back to the initialize handshake",
		"url", url, "code", rpcErr.Code, "message", rpcErr.Message)
}

// isMethodUnsupportedStatus reports whether err is a status that means the server does not serve
// this method.
func isMethodUnsupportedStatus(err error) bool {
	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode == http.StatusNotFound || statusErr.StatusCode == http.StatusMethodNotAllowed
}

// mcpRequestHeaders are the per-call headers layered onto the fixed Content-Type and Accept:
// the caller's upstream auth header, mcp-session-id on the legacy path, and MCP-Protocol-Version
// on the modern one, where there is no handshake to negotiate a version.
type mcpRequestHeaders struct {
	headerName      string
	headerValue     string
	sessionID       string
	protocolVersion string
	method          string
}

// forMethod copies the headers with the JSON-RPC method that the next call mirrors, so one
// prepared set can serve every call without the method leaking between them.
func (h mcpRequestHeaders) forMethod(method string) mcpRequestHeaders {
	h.method = method
	return h
}

// isModern reports whether the server is being addressed under a revision that mandates the
// mirrored headers and the per-request envelope.
func (h mcpRequestHeaders) isModern() bool {
	return h.protocolVersion >= SpecVersion20260728
}

// postJSONRPC sends a JSON-RPC request to an MCP server and returns the decoded body.
func postJSONRPC(url string, req any, headers mcpRequestHeaders) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mcpRequestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if headers.headerName != "" {
		lower := strings.ToLower(headers.headerName)
		if lower != McpSessionHeader && lower != "content-type" && lower != "accept" {
			httpReq.Header.Set(headers.headerName, headers.headerValue)
		}
	}
	if headers.sessionID != "" {
		httpReq.Header.Set(McpSessionHeader, headers.sessionID)
	}
	if headers.protocolVersion != "" {
		httpReq.Header.Set(McpProtocolVersionHeader, headers.protocolVersion)
	}
	if headers.isModern() && headers.method != "" {
		httpReq.Header.Set(McpMethodHeader, headers.method)
	}
	// Same SSRF-guarded client as the initialize call: the session/tools/prompts/resources
	// requests target the same user-supplied URL and must be dialed under the same guard.
	client, err := NewUpstreamFetchClient(mcpRequestTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	// Bound the body — see the matching comment in initializeMCPServer.
	maxBytes := mcpResponseMaxBytes()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("MCP server response exceeds the maximum allowed size")
	}

	// Check HTTP status code.
	// The upstream's 401 is reported as MCP_PROXY_UPSTREAM_UNAUTHORIZED (400), never as our own
	// Unauthorized: clients treat a 401 from this API as their session expiring and force a
	// logout, so relaying a remote peer's 401 verbatim would let any auth-requiring MCP server
	// sign the user out.
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, apperror.MCPProxyUpstreamUnauthorized.New().
			WithLogMessage("MCP server returned 401 Unauthorized to a JSON-RPC request")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	// Check if response is event stream
	if isEventStream(resp) {
		// Extract JSON data from event stream
		data, err := parseEventStream(body)
		if err != nil {
			return nil, fmt.Errorf("failed to parse event stream: %w, body: %s", err, string(body))
		}
		return data, nil
	}

	return body, nil
}

// parseEventStream extracts JSON data from event stream response
func parseEventStream(body []byte) ([]byte, error) {
	lines := bytes.Split(body, []byte("\n"))
	for _, line := range lines {
		if after, ok := bytes.CutPrefix(line, []byte("data: ")); ok {
			data := after
			data = bytes.TrimSpace(data)
			if len(data) > 0 && !bytes.Equal(data, []byte("{}")) {
				return data, nil
			}
		}
	}
	return nil, fmt.Errorf("no data found in event stream")
}

// isEventStream checks if the response is an event stream
func isEventStream(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return bytes.Contains([]byte(contentType), []byte("text/event-stream"))
}
