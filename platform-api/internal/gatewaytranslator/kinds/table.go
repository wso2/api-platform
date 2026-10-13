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

// Package kinds defines, one file per artifact kind, how that kind's
// deployment artifact is adapted for the gateway it is deployed to.
//
// Reading rule: open the file named after the kind. It is the complete list
// of what the kind needs — the first gateway release that has the kind, how an
// older stored shape is brought up to date, and the ordered steps an older
// gateway needs, each tagged with the gwversion.Min* release it applies below.
// No version number is ever written in a kind file.
//
// upstreamauth.go and llmpolicies.go are helpers the kind files call by name;
// they are not entry points and export nothing.
package kinds

import (
	"fmt"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
)

// All is every kind the translator knows, keyed by the kind as the gateway
// names it (the artifact's "kind:" field). Agent proxies therefore appear
// under Agent, not AgentProxy. The table is explicit rather than built by
// init() so that adding a kind is one row here, and so a test can range over
// it and compare it with the control plane's kind map.
var All = map[string]translate.Kind{
	constants.RestApi:          RestAPI,
	constants.MCPProxy:         MCP,
	constants.LLMProvider:      LLMProvider,
	constants.LLMProxy:         LLMProxy,
	constants.WebSubApi:        WebSub,
	constants.WebBrokerApi:     WebBroker,
	constants.GraphQLApi:       GraphQL,
	constants.GatewayKindAgent: Agent,
}

// Lookup returns the definition for a gateway kind.
func Lookup(gatewayKind string) (translate.Kind, bool) {
	k, ok := All[gatewayKind]
	return k, ok
}

// expect asserts that artifact is the deployment struct a kind works on. Every
// kind function starts with it, so a caller that pairs a kind with another
// kind's artifact fails here instead of shipping an untranslated document.
func expect[T any](artifact any) (T, error) {
	v, ok := artifact.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("expected %T, got %T", zero, artifact)
	}
	return v, nil
}

// onlyType is the Normalize of a kind that has only ever had one stored
// shape: there is nothing to bring up to date, but the payload type is still
// checked.
func onlyType[T any]() func(sourceDataVersion string, artifact any) error {
	return func(_ string, artifact any) error {
		_, err := expect[T](artifact)
		return err
	}
}
