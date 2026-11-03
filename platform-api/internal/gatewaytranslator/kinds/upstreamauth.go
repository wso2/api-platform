/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package kinds

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Upstream auth type values as the gateway spells them.
const (
	upstreamAuthTypeAPIKey = "api-key"
	upstreamAuthTypeNone   = "none"
	upstreamAuthTypeBearer = "bearer"
)

// upstreamAuthRule is what one kind's older gateways accept as an upstream
// auth type. The LLM and MCP validators gained "none" and "other" in
// different releases and treat an unknown type differently, so each kind
// carries its own rule.
type upstreamAuthRule struct {
	// minVersion is the first LTS release that accepts "none" and "other"
	// for this kind (gwversion.NoLTSRelease when none does); the step that
	// applies the rule runs below it.
	minVersion string
	// known lists the types besides api-key that older gateways already
	// handle, so they pass without a warning.
	known map[string]bool
	// unsupported says what an older gateway does with any other type; it
	// ends the warning.
	unsupported string
}

// llmUpstreamAuthRule: LLM validators below the minimum reject every type but
// api-key, so the artifact fails visibly on the gateway.
var llmUpstreamAuthRule = upstreamAuthRule{
	minVersion:  gwversion.MinLLMUpstreamAuthTypeNoneOtherVersion,
	unsupported: "shipped unchanged so the gateway reports it",
}

// mcpUpstreamAuthRule: no LTS MCP validator accepts "none" without a header and
// value; they accept any type that has both and send that header upstream, so
// "other" behaves like api-key there. "bearer" has had its own MCP validation
// since 1.0.0.
var mcpUpstreamAuthRule = upstreamAuthRule{
	minVersion:  gwversion.MinMCPUpstreamAuthTypeNoneOtherVersion,
	known:       map[string]bool{upstreamAuthTypeBearer: true},
	unsupported: "shipped unchanged; the gateway applies it like api-key with the configured header and value",
}

// decide is the one rule for an upstream auth type on a gateway below
// rule.minVersion:
//
//   - empty, api-key or a type in rule.known: nothing to do;
//   - none: drop the auth block, which means exactly the same thing to the
//     gateway, so no warning;
//   - anything else (other, and the basic/bearer values platform-api's enum
//     allows): keep it and warn. Stripping a credential would silently send
//     unauthenticated traffic upstream.
func (rule upstreamAuthRule) decide(authType string) (drop bool, warning string) {
	switch {
	case authType == "" || authType == upstreamAuthTypeAPIKey || rule.known[authType]:
		return false, ""
	case authType == upstreamAuthTypeNone:
		return true, ""
	default:
		return false, fmt.Sprintf("upstream auth type %q is not accepted by %s; %s",
			authType, gwversion.Gateways(rule.minVersion), rule.unsupported)
	}
}

// downConvertAPIUpstreamAuth applies rule to an *api.UpstreamAuth (the
// generated type, pointer fields), as the LLM provider and proxy artifacts
// carry it.
func downConvertAPIUpstreamAuth(rule upstreamAuthRule, auth **api.UpstreamAuth, kind, field string, r *translate.Report) {
	if *auth == nil || (*auth).Type == nil {
		return
	}
	drop, warning := rule.decide(string(*(*auth).Type))
	if drop {
		*auth = nil
		return
	}
	if warning != "" {
		r.Warn(kind, field, "%s", warning)
	}
}

// downConvertModelUpstreamAuth applies rule to a *model.UpstreamAuth (plain
// string fields), as the MCP artifact carries it.
func downConvertModelUpstreamAuth(rule upstreamAuthRule, auth **model.UpstreamAuth, kind, field string, r *translate.Report) {
	if *auth == nil {
		return
	}
	drop, warning := rule.decide((*auth).Type)
	if drop {
		*auth = nil
		return
	}
	if warning != "" {
		r.Warn(kind, field, "%s", warning)
	}
}
