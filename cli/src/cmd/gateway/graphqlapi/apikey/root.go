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

package apikey

import (
	"github.com/wso2/api-platform/cli/cmd/gateway/apikeycmd"
	"github.com/wso2/api-platform/cli/utils"
)

// apiKeyConfig parameterizes the shared api-key request logic and root-group
// builder (see cmd/gateway/apikeycmd) for GraphQL APIs. create.go/update.go
// are not templated - see apikeycmd's package doc for why.
var apiKeyConfig = apikeycmd.Config{
	KindLabel:       "GraphQL API",
	KindPathSegment: "graphql-api",
	ExampleAPIID:    "countries-graphql-api",
	KeysPath:        utils.GatewayGraphQLAPIKeysPath,
	KeyByNamePath:   utils.GatewayGraphQLAPIKeyByNamePath,
	RegeneratePath:  utils.GatewayGraphQLAPIKeyRegeneratePath,
	CreateExample:   "# Generate a new API key with an auto-generated name\nap gateway graphql-api api-key create --id countries-graphql-api",
}

// APIKeyCmd represents the gateway GraphQL API api-key command group. API keys
// are scoped to a GraphQL API via the /graphql-apis/{id}/api-keys management
// endpoints.
var APIKeyCmd = apikeycmd.NewRootCmd(apiKeyConfig, createCmd, listCmd, regenerateCmd, updateCmd, revokeCmd)
