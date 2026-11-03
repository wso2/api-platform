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

package aiworkspace

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keyStubbedSupportedVersions holds the protocol versions the stubbed MCP server reports.
const keyStubbedSupportedVersions = "uiStubbedSupportedVersions"

// parseVersionList splits the comma-separated list a step carries.
func parseVersionList(list string) []string {
	versions := []string{}
	for _, part := range strings.Split(list, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			versions = append(versions, trimmed)
		}
	}
	return versions
}

// theMCPServerReportsTheProtocolVersions sets what every later fetch-server-info stub
// answers with. The stub reads this per request rather than at registration, so a scenario
// can change it between the create and a later refetch — which is the only way to make a
// refetch discover something the proxy has not already recorded.
func (u *Steps) theMCPServerReportsTheProtocolVersions(ctx context.Context, list string) error {
	return tcontext.Set(ctx, keyStubbedSupportedVersions, parseVersionList(list))
}

// theMCPServerReportsNoProtocolVersions is the legacy-server case. It renders the same stub
// body as never setting one, because platform-api omits supportedVersions both when the
// server named none and when nothing could be determined — the two are indistinguishable on
// the wire. The step exists so a scenario states which case it means.
func (u *Steps) theMCPServerReportsNoProtocolVersions(ctx context.Context) error {
	return tcontext.Set(ctx, keyStubbedSupportedVersions, []string{})
}

// stubbedSupportedVersionsField renders the supportedVersions member for the stub body,
// including its trailing comma, or "" when there is nothing to report.
func stubbedSupportedVersionsField(ctx context.Context) string {
	v, ok := tcontext.Get(ctx, keyStubbedSupportedVersions)
	if !ok {
		return ""
	}
	versions, ok := v.([]string)
	if !ok || len(versions) == 0 {
		return ""
	}
	encoded, err := json.Marshal(versions)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`"supportedVersions":%s,`, encoded)
}

// upstreamVersionsFromCall decodes the upstreamMcpSpecVersions member of a recorded
// /mcp-proxies request body. It insists on a present JSON array rather than decoding
// straight into a []string field: an omitted key and an explicit null both decode to a nil
// slice, which slices.Equal cannot tell from an explicit [] — so the clearing assertion
// below would pass against a body that records nothing at all.
func upstreamVersionsFromCall(call recordedCall) ([]string, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return nil, fmt.Errorf("parsing the request body: %w", err)
	}
	raw, found := body["upstreamMcpSpecVersions"]
	if !found {
		return nil, fmt.Errorf("the request body records no upstreamMcpSpecVersions at all")
	}
	versions := []string{}
	if err := json.Unmarshal(raw, &versions); err != nil {
		return nil, fmt.Errorf("upstreamMcpSpecVersions is not a list of strings: %s", raw)
	}
	if versions == nil {
		return nil, fmt.Errorf("upstreamMcpSpecVersions is null, not a list")
	}
	return versions, nil
}

// mcpProxyCallCarriesUpstreamVersions asserts the most recent request of the given method
// records exactly the versions the probe reported, in the order it reported them. Order is
// asserted deliberately: the workspace relays the discovered set verbatim, and sorting is a
// display concern the payload must not pick up.
func (u *Steps) mcpProxyCallCarriesUpstreamVersions(ctx context.Context, method, list string) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, method)
	if err != nil {
		return fmt.Errorf("no %s request to /mcp-proxies was recorded: %w", method, err)
	}
	got, err := upstreamVersionsFromCall(call)
	if err != nil {
		return err
	}
	if want := parseVersionList(list); !slices.Equal(got, want) {
		return fmt.Errorf("upstreamMcpSpecVersions = %v, want %v", got, want)
	}
	return nil
}

func (u *Steps) theMCPProxyWasCreatedWithUpstreamProtocolVersions(ctx context.Context, list string) error {
	return u.mcpProxyCallCarriesUpstreamVersions(ctx, "POST", list)
}

// theMCPProxyUpdateCarriesTheUpstreamProtocolVersions guards the one silent regression this
// field can suffer: the API's update is a full replace, so a PUT that omits the versions
// clears the stored set with no error and nothing visible in the UI.
func (u *Steps) theMCPProxyUpdateCarriesTheUpstreamProtocolVersions(ctx context.Context, list string) error {
	return u.mcpProxyCallCarriesUpstreamVersions(ctx, "PUT", list)
}

// theMCPProxyCreateRecordsNoUpstreamProtocolVersions asserts the create body omits the key
// entirely rather than sending an empty list. The two are not interchangeable: the API stores
// what it is given, so an explicit [] records "this server was asked and named none" where the
// absent key records nothing at all.
func (u *Steps) theMCPProxyCreateRecordsNoUpstreamProtocolVersions(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, "POST")
	if err != nil {
		return fmt.Errorf("no POST request to /mcp-proxies was recorded: %w", err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return fmt.Errorf("parsing the request body: %w", err)
	}
	if raw, found := body["upstreamMcpSpecVersions"]; found {
		return fmt.Errorf("the create body unexpectedly records upstreamMcpSpecVersions as %s", raw)
	}
	return nil
}

// theMCPProxyCreateDeclaresNoMCPSpecVersion asserts the create body carries neither the
// deprecated mcpSpecVersion scalar nor an mcpSpecVersions list: the workspace records what
// the upstream reported and declares nothing on the proxy's own behalf, leaving the gateway
// to apply its own oldest supported version.
//
// It decodes to a key set rather than searching the raw body, because "mcpSpecVersion" is a
// prefix of "mcpSpecVersions" and a substring of "upstreamMcpSpecVersions" — a text search
// cannot tell the three apart.
func (u *Steps) theMCPProxyCreateDeclaresNoMCPSpecVersion(ctx context.Context) error {
	t, err := u.tracker(ctx)
	if err != nil {
		return err
	}
	call, err := t.awaitMCPProxyCall(ctx, "POST")
	if err != nil {
		return fmt.Errorf("no POST request to /mcp-proxies was recorded: %w", err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call.body), &body); err != nil {
		return fmt.Errorf("parsing the request body: %w", err)
	}
	for _, key := range []string{"mcpSpecVersion", "mcpSpecVersions"} {
		if _, found := body[key]; found {
			return fmt.Errorf("the create body unexpectedly declares %q", key)
		}
	}
	return nil
}
