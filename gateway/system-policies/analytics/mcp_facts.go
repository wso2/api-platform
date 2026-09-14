package analytics

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Request headers MCP 2026-07-28 mirrors the operation into, and the attributes the gateway's
// MCP resolver publishes from the body.
const (
	headerProtocolVersion = "MCP-Protocol-Version"
	headerMcpMethod       = "Mcp-Method"
	headerMcpName         = "Mcp-Name"

	attrBodyMethod          = "mcp.body.method"
	attrBodyCapabilityName  = "mcp.body.capability.name"
	attrBodyProtocolVersion = "mcp.body.protocol.version"
	attrBodyClientName      = "mcp.body.client.name"
	attrBodyClientVersion   = "mcp.body.client.version"

	// SHA-256 of params.requestState, the value a client echoes when retrying.
	attrBodyRequestStateHash = "mcp.body.request.state.hash"

	// The first revision to mirror values into headers. Versions are ISO-8601 dates, so string
	// comparison is chronological.
	specVersionModern = "2026-07-28"

	sentinelPrefix = "=?base64?"
	sentinelSuffix = "?="
)

// errMalformedSentinel marks a value that announced itself as sentinel-encoded but whose payload
// is not decodable base64.
var errMalformedSentinel = errors.New("malformed base64 sentinel value")

// isBodyResolved reports whether the controller marked this route as carrying the MCP operation
// resolver. Absent means no resolver, and the body is parsed instead.
func isBodyResolved(params map[string]interface{}) bool {
	resolved, _ := params["bodyResolved"].(bool)
	return resolved
}

// mcpRequestPropsFromResolver builds the MCP request properties without reading the body: from
// the mirrored headers on a modern request, and from the resolver's attributes on a legacy one.
func mcpRequestPropsFromResolver(headers *policy.Headers, shared *policy.SharedContext) McpRequestAnalyticsProperties {
	attrs := resolutionAttributes(shared)

	var props McpRequestAnalyticsProperties
	var requestedVersion string

	if isModernRequest(headers) {
		props.JsonRpcMethod = firstHeader(headers, headerMcpMethod)
		if raw := firstHeader(headers, headerMcpName); raw != "" {
			if decoded, err := decodeSentinel(raw); err == nil {
				props.CapabilityName = decoded
			}
		}
		requestedVersion = firstHeader(headers, headerProtocolVersion)
	} else {
		props.JsonRpcMethod = attrs.Get(attrBodyMethod)
		props.CapabilityName = attrs.Get(attrBodyCapabilityName)
		requestedVersion = attrs.Get(attrBodyProtocolVersion)
	}

	props.Capability = deriveMCPCapability(props.JsonRpcMethod)
	props.RequestStateHash = attrs.Get(attrBodyRequestStateHash)

	// The resolver publishes these for both eras. Legacy carries them on initialize only.
	clientInfo := McpClientInfo{
		RequestedProtocolVersion: requestedVersion,
		Name:                     attrs.Get(attrBodyClientName),
		Version:                  attrs.Get(attrBodyClientVersion),
	}
	if clientInfo.RequestedProtocolVersion != "" || clientInfo.Name != "" || clientInfo.Version != "" {
		props.ClientInfo = &clientInfo
	}
	return props
}

// resolutionAttributes returns what the route's resolver published, or a zero value whose Get
// answers "" for every key.
func resolutionAttributes(shared *policy.SharedContext) policy.ResolutionAttributes {
	if shared == nil {
		return policy.ResolutionAttributes{}
	}
	return shared.ResolutionAttributes
}

// isModernRequest reports whether the request declares a revision that mirrors the operation into
// headers. The era is the header's value, not its presence: 2025-06-18 sends one too.
func isModernRequest(headers *policy.Headers) bool {
	return firstHeader(headers, headerProtocolVersion) >= specVersionModern
}

// firstHeader returns a header's first value, or "" when absent.
func firstHeader(headers *policy.Headers, name string) string {
	values := headers.Get(name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// decodeSentinel unwraps the "=?base64?<standard-base64>?=" form MCP defines for header values
// that cannot be written as visible ASCII, returning anything else unchanged.
func decodeSentinel(value string) (string, error) {
	if !strings.HasPrefix(value, sentinelPrefix) || !strings.HasSuffix(value, sentinelSuffix) {
		return value, nil
	}
	// "=?base64?=" passes both checks carrying no payload; the slice below would run out of range.
	if len(value) < len(sentinelPrefix)+len(sentinelSuffix) {
		return "", errMalformedSentinel
	}
	raw, err := base64.StdEncoding.DecodeString(value[len(sentinelPrefix) : len(value)-len(sentinelSuffix)])
	if err != nil {
		return "", errMalformedSentinel
	}
	return string(raw), nil
}

// hashRequestState returns the SHA-256 hex of a requestState. Must match the resolver's function
// of the same name, which fingerprints the request side.
func hashRequestState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}
