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

// Upstream auth type values as the gateway spells them. Gateways below
// gwversion.MinUpstreamAuthTypeNoneOtherVersion accept only api-key.
const (
	upstreamAuthTypeAPIKey = "api-key"
	upstreamAuthTypeNone   = "none"
)

// upstreamAuthDecision is the one rule for an upstream auth type on a gateway
// that accepts only api-key:
//
//   - empty or api-key: nothing to do;
//   - none: drop the auth block, which means exactly the same thing to the
//     gateway, so no warning;
//   - anything else (other, and the basic/bearer values platform-api's enum
//     allows): keep it and warn. Stripping a credential would silently send
//     unauthenticated traffic upstream; shipping it lets the gateway reject
//     the artifact visibly.
func upstreamAuthDecision(authType string) (drop bool, warning string) {
	switch authType {
	case "", upstreamAuthTypeAPIKey:
		return false, ""
	case upstreamAuthTypeNone:
		return true, ""
	default:
		return false, fmt.Sprintf(
			"upstream auth type %q is not accepted by gateways below %s; shipped unchanged so the gateway reports it",
			authType, gwversion.MinUpstreamAuthTypeNoneOtherVersion)
	}
}

// downConvertAPIUpstreamAuth applies upstreamAuthDecision to an *api.UpstreamAuth
// (the generated type, pointer fields), as the LLM provider and proxy artifacts
// carry it.
func downConvertAPIUpstreamAuth(auth **api.UpstreamAuth, kind, field string, r *translate.Report) {
	if *auth == nil || (*auth).Type == nil {
		return
	}
	drop, warning := upstreamAuthDecision(string(*(*auth).Type))
	if drop {
		*auth = nil
		return
	}
	if warning != "" {
		r.Warn(kind, field, "%s", warning)
	}
}

// downConvertModelUpstreamAuth applies upstreamAuthDecision to a
// *model.UpstreamAuth (plain string fields), as the MCP artifact carries it.
func downConvertModelUpstreamAuth(auth **model.UpstreamAuth, kind, field string, r *translate.Report) {
	if *auth == nil {
		return
	}
	drop, warning := upstreamAuthDecision((*auth).Type)
	if drop {
		*auth = nil
		return
	}
	if warning != "" {
		r.Warn(kind, field, "%s", warning)
	}
}
