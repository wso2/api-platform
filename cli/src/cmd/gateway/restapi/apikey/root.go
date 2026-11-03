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
// builder (see cmd/gateway/apikeycmd) for REST APIs. create.go/update.go are
// not templated - see apikeycmd's package doc for why.
var apiKeyConfig = apikeycmd.Config{
	KindLabel:       "REST API",
	KindPathSegment: "rest-api",
	ExampleAPIID:    "reading-list-api-v1.0",
	KeysPath:        utils.GatewayAPIKeysPath,
	KeyByNamePath:   utils.GatewayAPIKeyByNamePath,
	RegeneratePath:  utils.GatewayAPIKeyRegeneratePath,
	CreateExample:   "# Generate a new API key from a CR file\nap gateway rest-api api-key create --file api-key.yaml",
}

// APIKeyCmd represents the gateway REST API api-key command group. API keys are
// scoped to a REST API via the /rest-apis/{id}/api-keys management endpoints.
var APIKeyCmd = apikeycmd.NewRootCmd(apiKeyConfig, createCmd, listCmd, regenerateCmd, updateCmd, revokeCmd)
